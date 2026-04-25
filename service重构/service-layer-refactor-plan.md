# Service Layer 重构计划

## 1. 背景与目标

### 当前架构问题
- API 层通过调用 CLI 实现功能，导致架构耦合
- 参数需要序列化为字符串再解析，类型安全弱
- 任务进度无法准确追踪
- 错误信息丢失

### 重构目标
- 将核心下载逻辑抽象为 Service 层
- CLI 和 API 都直接调用 Service 层
- 提高代码可维护性、可测试性
- 支持更细粒度的任务控制和进度追踪

## 2. 目标架构

```
┌─────────────────────────────────────────────────────────────┐
│                        入口层                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  CLI Main   │  │  API Server │  │  Web UI (future)    │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
└─────────┼────────────────┼────────────────────┼─────────────┘
          │                │                    │
          └────────────────┴────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                      Service Layer                           │
│  ┌────────────────────────────────────────────────────────┐  │
│  │                 DownloadService                         │  │
│  │  - UserDownload(ctx, opts) error                       │  │
│  │  - ListDownload(ctx, opts) error                       │  │
│  │  - FollowingDownload(ctx, opts) error                  │  │
│  │  - ProfileDownload(ctx, opts) error                    │  │
│  │  - MarkDownloaded(ctx, opts) error                     │  │
│  │  - JsonDownload(ctx, opts) error                       │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │              ProgressReporter (interface)               │  │
│  │  - OnProgress(taskID string, p Progress)               │  │
│  │  - OnComplete(taskID string, result Result)            │  │
│  │  - OnError(taskID string, err error)                   │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────┬──────────────────────────────────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
┌─────────▼────────┐ ┌─────▼─────┐ ┌───────▼────────┐
│   Repository     │ │ Repository│ │   Repository   │
│   Layer (DB)     │ │Layer (FS) │ │ Layer (Twitter)│
│  ┌────────────┐  │ │ ┌───────┐ │ │  ┌──────────┐  │
│  │ UserRepo   │  │ │ │Naming │ │ │  │UserAPI   │  │
│  │ ListRepo   │  │ │ │Version│ │ │  │ListAPI   │  │
│  │ EntityRepo │  │ │ │Storage│ │ │  │TweetAPI  │  │
│  └────────────┘  │ │ └───────┘ │ │  └──────────┘  │
└──────────────────┘ └───────────┘ └────────────────┘
```

## 3. 详细设计

### 3.1 Service Layer 接口设计

```go
// internal/service/download_service.go

package service

import "context"

// DownloadOptions 下载选项
type DownloadOptions struct {
    AutoFollow  bool
    SkipProfile bool
    NoRetry     bool
    MarkTime    *time.Time
}

// Progress 进度信息
type Progress struct {
    Total     int
    Completed int
    Failed    int
    Current   string // 当前处理的项
}

// Result 执行结果
type Result struct {
    Downloaded int
    Failed     int
    Skipped    int
    Message    string
}

// ProgressReporter 进度报告接口
type ProgressReporter interface {
    OnProgress(taskID string, p Progress)
    OnComplete(taskID string, r Result)
    OnError(taskID string, err error)
}

// DownloadService 下载服务接口
type DownloadService interface {
    // UserDownload 下载用户推文
    UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error
    
    // ListDownload 下载列表推文
    ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error
    
    // FollowingDownload 下载关注列表
    FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error
    
    // ProfileDownload 下载用户资料
    ProfileDownload(ctx context.Context, taskID string, screenNames []string, reporter ProgressReporter) error
    
    // ListProfileDownload 下载列表用户资料
    ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter ProgressReporter) error
    
    // MarkDownloaded 标记已下载
    MarkDownloaded(ctx context.Context, taskID string, screenName string, markTime *time.Time, reporter ProgressReporter) error
    
    // JsonDownload 从JSON下载
    JsonDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error
}
```

### 3.2 依赖注入设计

```go
// internal/service/service.go

package service

import (
    "github.com/go-resty/resty/v2"
    "github.com/jmoiron/sqlx"
    "github.com/unkmonster/tmd/internal/config"
)

// Dependencies 服务依赖
type Dependencies struct {
    Client            *resty.Client
    AdditionalClients []*resty.Client
    DB                *sqlx.DB
    Config            *config.Config
    AppRootPath       string
}

// NewDownloadService 创建下载服务
func NewDownloadService(deps *Dependencies) DownloadService {
    return &downloadServiceImpl{
        deps: deps,
        // 初始化各 Repository
        userRepo:    repository.NewUserRepo(deps.DB),
        listRepo:    repository.NewListRepo(deps.DB),
        entityRepo:  repository.NewEntityRepo(deps.DB),
        namingSvc:   naming.NewService(deps.Config.MaxFileNameLen),
        downloader:  downloader.NewDownloader(...),
    }
}
```

## 4. 重构步骤

### Phase 1: 准备工作 (1-2天)

#### 4.1.1 创建 Repository 层接口
- [ ] 创建 `internal/repository/user_repo.go`
- [ ] 创建 `internal/repository/list_repo.go`
- [ ] 创建 `internal/repository/entity_repo.go`
- [ ] 将现有 database 包的函数封装为 Repository 接口

**困难点:**
- 现有 database 包的函数粒度不一，需要统一接口设计
- 事务处理需要在 Repository 层妥善处理

#### 4.1.2 创建 Naming/Storage 服务
- [ ] 将 `internal/naming` 和文件操作封装为服务接口
- [ ] 统一路径管理和文件命名逻辑

### Phase 2: Service 层实现 (3-5天)

#### 4.2.1 实现 DownloadService 核心逻辑
- [ ] 实现 `UserDownload` 方法
- [ ] 实现 `ListDownload` 方法
- [ ] 实现 `FollowingDownload` 方法
- [ ] 实现 `ProfileDownload` 方法
- [ ] 实现 `MarkDownloaded` 方法
- [ ] 实现 `JsonDownload` 方法

**困难点:**
- 现有 CLI 逻辑分散在多个文件中，需要仔细梳理
- 错误处理需要统一，区分可恢复错误和致命错误
- 进度报告需要侵入现有下载逻辑，修改点较多

#### 4.2.2 集成 ProgressReporter
- [ ] 修改 `internal/downloading` 包支持进度回调
- [ ] 在关键节点插入进度报告调用

**困难点:**
- 现有下载逻辑是同步的，需要找到合适的切入点
- 批量下载时的进度计算需要精确

### Phase 3: CLI 适配 (2-3天)

#### 4.3.1 重构 CLI 层
- [ ] 修改 `internal/cli/executor.go` 调用 Service 层
- [ ] 移除参数解析后的重复逻辑
- [ ] 实现 CLI 的 ProgressReporter（日志输出）

**困难点:**
- 需要保持 CLI 的行为与现有版本一致
- 错误处理和退出码需要兼容

### Phase 4: API 适配 (2-3天)

#### 4.4.1 重构 API 层
- [ ] 修改 `internal/api/server.go` 直接调用 Service 层
- [ ] 移除 `AsyncExecutor` 和 `BuildArgs`
- [ ] 实现 API 的 ProgressReporter（SSE/任务状态更新）

**困难点:**
- 需要重新设计任务管理逻辑
- SSE 推送需要与 Service 层进度报告集成

### Phase 5: 测试与优化 (3-5天)

#### 4.5.1 单元测试
- [ ] 为 Repository 层编写单元测试
- [ ] 为 Service 层编写单元测试（使用 mock）

#### 4.5.2 集成测试
- [ ] 测试 CLI 模式的完整流程
- [ ] 测试 API 模式的完整流程
- [ ] 对比重构前后的行为一致性

#### 4.5.3 性能优化
- [ ] 检查是否有性能退化
- [ ] 优化数据库查询
- [ ] 优化并发处理

## 5. 风险评估与缓解措施

### 5.1 高风险项

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 重构引入 Bug | 功能不可用 | 1. 保持现有测试通过<br>2. 分阶段发布<br>3. 保留回滚方案 |
| 进度报告不准确 | 用户体验下降 | 1. 详细设计进度计算逻辑<br>2. 增加进度报告测试 |
| 性能退化 | 下载速度变慢 | 1. 重构前后性能对比测试<br>2. 优化关键路径 |

### 5.2 技术债务

- 现有代码缺乏单元测试，重构后需要补充
- 部分逻辑与 Twitter API 强耦合，抽象难度大

## 6. 文件变更清单

### 新增文件
```
internal/
├── service/
│   ├── download_service.go      # 服务接口
│   ├── service.go               # 服务实现
│   └── progress.go              # 进度相关
├── repository/
│   ├── user_repo.go             # 用户仓库
│   ├── list_repo.go             # 列表仓库
│   └── entity_repo.go           # 实体仓库
```

### 修改文件
```
internal/
├── cli/
│   └── executor.go              # 改为调用 Service
├── api/
│   ├── server.go                # 改为调用 Service
│   ├── async_executor.go        # 可能移除
│   └── task_manager.go          # 集成 Service 进度
├── downloading/
│   └── *.go                     # 支持进度回调
```

## 7. 时间估算

| 阶段 | 预计时间 | 备注 |
|------|----------|------|
| Phase 1: 准备工作 | 1-2天 | 风险较低 |
| Phase 2: Service 层实现 | 3-5天 | 核心工作，风险较高 |
| Phase 3: CLI 适配 | 2-3天 | 需要仔细测试 |
| Phase 4: API 适配 | 2-3天 | 需要重新设计任务管理 |
| Phase 5: 测试与优化 | 3-5天 | 不可忽视 |
| **总计** | **11-18天** | 建议预留缓冲 |

## 8. 建议的实施策略

### 方案 A: 大爆炸式重构（不推荐）
- 一次性完成所有重构
- 风险高，难以回滚

### 方案 B: 增量式重构（推荐）
- 逐个功能迁移到 Service 层
- 先实现 `UserDownload`，验证后再迁移其他功能
- 保持旧代码可用，通过特性开关切换

### 推荐的增量步骤
1. **第1周**: 实现 Repository 层 + UserDownload Service
2. **第2周**: 迁移 CLI 的 UserDownload 到 Service
3. **第3周**: 迁移 API 的 UserDownload 到 Service
4. **第4周**: 迁移其他功能（ListDownload, FollowingDownload 等）

## 9. 回滚方案

- 保留重构前的代码分支
- 每个 Phase 完成后打 tag
- 如果出现问题，可以快速回滚到上一个稳定版本

## 10. 成功标准

- [ ] 所有现有功能正常工作
- [ ] CLI 和 API 行为一致
- [ ] API 任务进度可准确追踪
- [ ] 代码覆盖率提升 20% 以上
- [ ] 性能不劣于重构前
