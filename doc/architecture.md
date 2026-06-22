# 项目架构

> 本文档面向开发者，内容从 README 迁移至此。普通用户无需阅读。

## 主干调用关系

```text
main.go
  ├─ 读取配置 / 环境变量 / 日志 / 数据库 / Twitter 客户端
  ├─ CLI 模式 -> internal/cli
  └─ Server 模式 -> internal/api

internal/cli / internal/api
  └─ 统一调用 internal/service.DownloadService

internal/service
  └─ 编排 downloading / downloader / twitter / database / path
```

整体上采用"入口层 + 应用服务层 + 业务层 + 基础设施层"的结构，核心复用点是 **`internal/service`**：CLI 和 API Server 共用同一套下载编排逻辑，避免两边各自维护一份下载流程。

## 完整分层图

```
┌─────────────────────────────────────────────────────────────┐
│  main.go (应用入口层)                                         │
│  - 预解析全局参数                                             │
│  - 初始化配置、日志、数据库、Twitter 客户端                      │
│  - 模式选择：Server / CLI                                    │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────────┐
│  internal/config (配置层)                                    │
│  - config.go: 配置结构、读写、环境变量覆盖、交互式配置             │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼───────────────────┐  ┌─────────────────────▼─────────────────────────┐
│  internal/twitter            │  │  internal/database                            │
│  (API 客户端层)               │  │  (数据持久化层)                                  │
│                              │  │                                               │
│  - client.go: 登录与客户端管理 │  │  - connect/sqlite/schema                       │
│  - api.go: 请求封装与通用能力  │  │  - sqlite_schema/sqlite_migration              │
│  - user/tweet/list/timeline  │  │  - model/query/helpers                        │
│  - batch_login.go: 多账号     │  │  - user/lst/user_entity/lst_entity            │
│                              │  │  - user_link/user_sync                        │
│                              │  │  - parent_dir_migration/path_validation       │
│                              │  │  - tx/manager                                 │
└──────────┬───────────────────┘  └────────────────────────┬──────────────────────┘
           │                                               │
           └────────────────┬──────────────────────────────┘
                            │
          ┌─────────────────▼───────────────────────────┐
          │  🎯 internal/service (Service 层)           │
          │        ★ 核心业务编排层 ★                    │
          │                                             │
          │  - interfaces.go: DownloadService 接口       │
          │  - download_service.go: 用户/列表/关注/       │
          │    JSON/Profile/重试等统一入口                 │
          │  - deps.go: 依赖注入与构造                     │
          │  - progress.go: 进度上报                      │
          └─────────────────┬───────────────────────────┘
                            │
          ┌─────────────────┼───────────────────────────┐
          │                 │                           │
┌─────────▼────────┐   ┌────▼───────────┐     ┌─────────▼─────────┐
│  internal/api    │   │  internal/cli  │     │  internal/path    │
│  (Server 层)     │   │  (CLI 层)       │     │  (路径工具)        │
│                  │   │                │     │                   │
│  - server.go     │   │  - args.go     │     │  - store.go       │
│  - download_*    │   │  - executor.go │     │                   │
│  - task_manager  │   │                │     │                   │
│  - download_queue│   │                │     │                   │
│  - progress / sse│   │                │     │                   │
│  - db/config/    │   │                │     │                   │
│    cookie/log_*  │   │                │     │                   │
│  - scheduler_*   │   │                │     │                   │
│  - event_bus.go  │   │                │     │                   │
└────────┬─────────┘   └────────────────┘     └───────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│  internal/downloading (业务流程层)                           │
│                                                             │
│  - batch_any.go / batch_download.go                         │
│  - tweet_download.go / user_sync.go / list_sync.go          │
│  - list_download.go / json_file_download.go / json_folder_download.go │
│  - mark_downloaded.go / retry.go / dumper.go                │
│  - entity.go / types.go / tweet_json_converter.go           │
│  - 负责抓取、同步实体、组织批量下载、失败重试                       │
├─────────────────────────────────────────────────────────────┤
│  internal/downloading/profile (Profile 业务子包)             │
│  - downloader.go / storage.go / types.go                    │
│  - 负责头像、横幅、简介、profile.json 与版本备份                  │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────────┐
│  internal/entity (数据实体层)                                 │
│  - interface.go / user.go / list.go / sync.go               │
├─────────────────────────────────────────────────────────────┤
│  internal/downloader (基础设施层 - 通用下载)                   │
│  - downloader.go: 单文件下载、流式下载、大小校验                 │
│  - file_writer.go: 原子写入、跳过未变化文件、并发锁管理           │
│  - version_manager.go: 版本备份管理                           │
├─────────────────────────────────────────────────────────────┤
│  internal/naming (命名服务)                                  │
│  - base.go / tweet_naming.go / user_naming.go / list_naming.go │
├─────────────────────────────────────────────────────────────┤
│  internal/scheduler (定时任务调度器)                          │
│  - scheduler.go: 调度执行与状态维护                            │
│  - types.go: ScheduleEntry / ScheduleStatus / ParsedSchedule│
├─────────────────────────────────────────────────────────────┤
│  internal/consolelog (控制台日志)                             │
│  - hub.go: 日志捕获和分发中心，支持 SSE 实时推送                  │
├─────────────────────────────────────────────────────────────┤
│  internal/utils (工具层)                                     │
│  - fs.go / http.go / algo.go / time_range.go / recovery.go  │
│  - user.go / win32.go (Windows) / stub.go (!Windows)        │
└─────────────────────────────────────────────────────────────┘
```

## 核心设计原则

| 原则 | 实现 |
| --- | --- |
| **Service 复用** | CLI 和 API 统一走 `DownloadService`，避免重复维护下载逻辑 |
| **入口与业务分离** | `internal/cli` / `internal/api` 只负责参数、HTTP、任务编排和响应 |
| **业务与下载器分离** | `internal/downloading` 负责"下载什么、按什么流程下载"，`internal/downloader` 负责"文件怎么下、怎么写" |
| **数据库集中管理** | SQLite schema、迁移、查询和实体同步集中在 `internal/database` |
| **任务异步化** | Server 模式通过 TaskManager、DownloadQueue、SSE 推进长任务 |
| **增量与重试** | 基于数据库和 `.data/errors.json` 做增量抓取与失败重试 |
| **跨入口一致性** | 用户下载、列表下载、关注下载、JSON 导入、Profile 下载都通过 service 层统一暴露 |

---

## Service 层架构

重构后的代码引入了 **Service 层**，将核心下载逻辑从 CLI 和 API 中抽象出来，实现代码复用和统一业务逻辑。

### 设计目标

| 目标 | 实现 |
|------|------|
| **统一业务逻辑** | CLI 和 API 共享同一套下载实现 |
| **简化 CLI 层** | CLI 只负责参数解析和调用 Service |
| **简化 API 层** | API 只负责 HTTP 路由和 SSE 推送 |
| **便于测试** | Service 接口易于 Mock 和单元测试 |

### DownloadService 接口

```go
type DownloadService interface {
    // 用户下载（单用户）
    UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error
    
    // 列表下载（单列表）
    ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error
    
    // 关注列表下载
    FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error
    
    // 批量下载（多用户/多列表/多关注）
    BatchDownload(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, opts DownloadOptions, reporter ProgressReporter) error
    
    // Profile 下载（指定用户）
    ProfileDownload(ctx context.Context, taskID string, screenNames []string, reporter ProgressReporter) error
    
    // 列表 Profile 下载
    ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter ProgressReporter) error
    
    // JSON 文件下载（第三方工具导出的推文 JSON）
    JsonFileDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error

    // LoongTweet 文件夹下载（TMD 生成的 .loongtweet）
    JsonFolderDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error

    // 标记已下载
    MarkDownloaded(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, markTime *string, reporter ProgressReporter) error

    // 重试所有历史失败推文（常规下载和 JSON 导入）
    RetryAllFailed(ctx context.Context, taskID string, reporter ProgressReporter) error

    // 清除所有失败推文记录
    ClearErrors() error
}
```

### 调用关系

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   CLI 层    │────▶│  DownloadService│────▶│  downloading包   │
│  executor   │     │  (业务编排)      │     │  (具体实现)      │
└─────────────┘     └─────────────────┘     └─────────────────┘
                           ▲
┌─────────────┐            │
│   API 层    │────────────┘
│  handlers   │
└─────────────┘
```

### 与稳定版的区别

| 特性 | 稳定版 | 重构版 |
|------|--------|--------|
| CLI 实现 | 直接调用 downloading 包 | 通过 Service 层间接调用 |
| API 实现 | 调用 CLI Execute | 直接调用 Service 层 |
| 代码复用 | 低（CLI 和 API 各自实现） | 高（共享 Service） |
| 可测试性 | 低 | 高（接口化设计） |
| 实时进度 | 无 | SSE 推送 |

---

## 开发指南

### 项目结构

```
tmd/
├── main.go                      # 应用入口（命令行解析、模式选择）
├── start-server.bat              # Windows Server 模式启动脚本
├── internal/
│   ├── api/                     # API Server 模块
│   ├── cli/                     # CLI 命令模块
│   ├── config/                  # 配置管理
│   ├── service/                 # Service 层（核心业务编排）
│   ├── database/                # 数据持久化层
│   ├── downloading/             # 核心下载逻辑
│   ├── downloader/              # 通用下载基础设施
│   ├── twitter/                 # Twitter API 客户端
│   ├── naming/                  # 命名服务
│   ├── entity/                  # 数据实体层
│   ├── path/                    # 路径管理
│   ├── scheduler/               # 定时任务调度器
│   ├── consolelog/              # 控制台日志捕获与分发
│   └── utils/                   # 工具函数
├── doc/                         # 详细文档
├── tools/                       # 工具脚本（迁移工具、Tampermonkey脚本）
├── .github/workflows/           # CI/CD 配置
└── test/                        # 集成测试
```

### 运行测试

项目包含 **58 个测试文件**，覆盖核心业务逻辑：

```bash
# 运行所有测试（含竞态检测）
go test -race ./...

# 运行特定包的测试
go test -v ./internal/downloading/
go test -v ./internal/api/
go test -v ./internal/database/
go test -v ./internal/service/

# 运行单个测试函数
go test -v -run TestFunctionName ./internal/package/

# 生成覆盖率报告
go test -race -covermode atomic -coverprofile=covprofile ./...
go tool cover -html=covprofile -o coverage.html
```

### 代码风格

本项目遵循以下编码规范：

- **Go 标准**: 遵循 [Effective Go](https://go.dev/doc/effective_go) 和官方风格指南
- **格式化**: 使用 `gofmt` 格式化代码（IDE 自动格式化）
- **编码准则**: 参考 [CLAUDE.md](../CLAUDE.md) 中的 AI 编码准则
- **设计原则**:
  - 简单优先：只写解决问题所需的最少代码
  - 外科手术式修改：只改必须改的内容
  - 接口隔离：小接口设计，便于 Mock 和测试
  - 分层解耦：清晰的层次结构，避免循环依赖

### 关键设计模式

| 模式 | 应用位置 | 说明 |
|------|---------|------|
| **依赖注入** | `service/deps.go` | 通过构造函数注入依赖，支持测试 Mock |
| **策略模式** | `downloader/downloader.go` | 小文件 Buffer / 大文件流式两种策略 |
| **观察者模式** | `api/sse.go` | SSE 推送任务状态更新 |
| **观察者模式** | `consolelog/hub.go` | SSE 推送实时日志流 |
| **工厂模式** | `naming/` | TweetNaming / UserNaming / ListNaming 工厂 |
| **单例模式** | `database/connect.go` | 全局数据库连接（SQLite） |
| **调度器模式** | `scheduler/scheduler.go` | interval/daily 两种调度策略 |

### CI/CD 流程

项目配置了 GitHub Actions 自动化流程：

```yaml
触发条件:
  - push 到 master 分支
  - Pull Request 到 master
  - 创建版本标签 (v*)

执行步骤:
  1. 多平台构建 (Windows / Linux / macOS) + Docker 镜像构建
  2. 运行测试套件 (go test -race)
  3. 上报覆盖率到 Coveralls
  4. 发布版本时自动创建 Release
```

### 独立工具

| 工具 | 位置 | 用途 |
|------|------|------|
| **tmd-db-migrate** | `tools/tmd-db-migrate/` | 跨平台数据库路径迁移，当下载目录从 Windows 迁移到 Linux 等场景时重写 `foo.db` 中的 `parent_dir` 路径。详见 [foo.db 跨平台迁移说明](foo.db%20跨平台迁移说明.md) |
| **convert_db_to_legacy.py** | 仓库根目录 | 将新格式数据库转换为旧格式的辅助脚本 |

---

## 项目目录结构

```
+-- main.go
+-- internal/
|   +-- api
|   |   +-- config_handlers.go
|   |   +-- cookie_handlers.go
|   |   +-- db_handlers.go
|   |   +-- download_handlers.go
|   |   +-- download_queue.go
|   |   +-- download_targets.go
|   |   +-- event_bus.go
|   |   +-- handlers.go
|   |   +-- log_handlers.go
|   |   +-- middleware.go
|   |   +-- pagination.go
|   |   +-- progress.go
|   |   +-- resource_handler.go
|   |   +-- scheduler_handlers.go
|   |   +-- server.go
|   |   +-- sse_logs.go
|   |   +-- sse_tasks.go
|   |   +-- string_uint64.go
|   |   +-- task_manager.go
|   |   +-- task_types.go
|   |   +-- types.go
|   |   `-- version.go
|   +-- cli
|   |   +-- args.go
|   |   `-- executor.go
|   +-- config
|   |   +-- backup.go
|   |   `-- config.go
|   +-- consolelog
|   |   `-- hub.go
|   +-- database
|   |   +-- connect.go
|   |   +-- helpers.go
|   |   +-- lst.go
|   |   +-- lst_entity.go
|   |   +-- model.go
|   |   +-- parent_dir_migration.go
|   |   +-- path_validation.go
|   |   +-- query.go
|   |   +-- schema.go
|   |   +-- sqlite.go
|   |   +-- sqlite_migration.go
|   |   +-- sqlite_schema.go
|   |   +-- user.go
|   |   +-- user_entity.go
|   |   +-- user_link.go
|   |   +-- user_sync.go
|   |   `-- tx
|   |       `-- manager.go
|   +-- downloader
|   |   +-- downloader.go
|   |   +-- file_writer.go
|   |   +-- types.go
|   |   `-- version_manager.go
|   +-- downloading
|   |   +-- batch_any.go
|   |   +-- batch_download.go
|   |   +-- dumper.go
|   |   +-- entity.go
|   |   +-- json_file_download.go
|   |   +-- json_folder_download.go
|   |   +-- list_download.go
|   |   +-- list_sync.go
|   |   +-- mark_downloaded.go
|   |   +-- retry.go
|   |   +-- test_helper.go
|   |   +-- tweet_download.go
|   |   +-- tweet_json_converter.go
|   |   +-- types.go
|   |   +-- user_sync.go
|   |   `-- profile
|   |       +-- downloader.go
|   |       +-- storage.go
|   |       `-- types.go
|   +-- entity
|   |   +-- interface.go
|   |   +-- list.go
|   |   +-- sync.go
|   |   `-- user.go
|   +-- naming
|   |   +-- base.go
|   |   +-- list_naming.go
|   |   +-- tweet_naming.go
|   |   `-- user_naming.go
|   +-- path
|   |   `-- store.go
|   +-- scheduler
|   |   +-- scheduler.go
|   |   +-- types.go
|   |   `-- validate.go
|   +-- service
|   |   +-- deps.go
|   |   +-- download_service.go
|   |   +-- interfaces.go
|   |   `-- progress.go
|   +-- twitter
|   |   +-- api.go
|   |   +-- batch_login.go
|   |   +-- client.go
|   |   +-- errors.go
|   |   +-- list.go
|   |   +-- timeline.go
|   |   +-- tweet.go
|   |   `-- user.go
|   `-- utils
|       +-- algo.go
|       +-- fs.go
|       +-- http.go
|       +-- recovery.go
|       +-- stub.go
|       +-- time_range.go
|       +-- twitter_media.go
|       +-- user.go
|       `-- win32.go
+-- .github/workflows/
+-- tools/
+-- doc/
+-- go.mod
+-- go.sum
+-- Dockerfile
+-- docker-compose.yml
+-- readme.md
+-- CHANGELOG.md
+-- LICENSE
+-- start-server.bat
+-- convert_db_to_legacy.py
`-- .gitignore
```
