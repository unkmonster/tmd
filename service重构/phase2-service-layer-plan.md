# Phase 2: Service 层实现计划

## 1. 目标

创建 Service 层，将核心下载逻辑从 CLI 中抽象出来，为 CLI 和 API 提供统一的业务逻辑接口。

## 2. 设计原则

遵循 CLAUDE.md 准则：
- **简单优先**：只做必要的抽象，Service 方法直接对应业务操作
- **外科手术式修改**：复用现有 downloading 包的逻辑，不重复造轮子
- **目标驱动**：每个 Service 方法都有明确的输入输出和错误处理

## 3. 模块划分

```
internal/
├── service/                    # 新增目录
│   ├── interfaces.go           # Service 接口定义
│   ├── download_service.go     # 下载服务实现
│   ├── progress.go             # 进度报告相关
│   └── deps.go                 # 依赖注入
├── downloading/                # 现有目录（需要适配进度回调）
│   └── ...
├── repository/                 # Phase 1 创建
│   └── ...
```

## 4. 详细步骤

### 步骤 1: 分析现有 downloading 包

**需要阅读的代码**:
- `internal/downloading/batch_any.go` - BatchDownloadAny
- `internal/downloading/batch_download.go` - BatchUserDownload
- `internal/downloading/user_sync.go` - syncListAndGetMembers
- `internal/downloading/mark_downloaded.go` - MarkUsersAsDownloaded
- `internal/downloading/json_download.go` - DownloadJsonDir
- `internal/downloading/profile/` - Profile 下载相关

**关键发现**:
- 现有逻辑使用 `log` 包输出进度，需要改造为回调机制
- 错误处理分散在各处，需要统一
- 需要支持 context 取消

---

### 步骤 2: 创建 Progress 接口

**文件**: `internal/service/progress.go`

**新增代码**:
```go
package service

// Progress 下载进度
type Progress struct {
    Stage     string // "syncing", "downloading", "retrying"
    Total     int
    Completed int
    Failed    int
    Current   string // 当前处理的用户/列表
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

// NopReporter 空报告器（用于 CLI 模式）
type NopReporter struct{}

func (n *NopReporter) OnProgress(taskID string, p Progress) {}
func (n *NopReporter) OnComplete(taskID string, r Result)   {}
func (n *NopReporter) OnError(taskID string, err error)     {}
```

**风险评估**:
- 风险: 低
- 注意: 保持简单，Stage 字段用于前端展示

**测试要点**:
- 编译通过

---

### 步骤 3: 创建 Service 接口

**文件**: `internal/service/interfaces.go`

**新增代码**:
```go
package service

import "context"

// DownloadOptions 下载选项
type DownloadOptions struct {
    AutoFollow  bool
    SkipProfile bool
    NoRetry     bool
    MarkTime    *string // 格式: "2006-01-02T15:04:05"
}

// DownloadService 下载服务接口
type DownloadService interface {
    // UserDownload 下载用户推文
    // 对应 CLI: -user <screen_name>
    UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error
    
    // ListDownload 下载列表推文
    // 对应 CLI: -list <list_id>
    ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error
    
    // FollowingDownload 下载关注列表
    // 对应 CLI: -foll <screen_name>
    FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error
    
    // ProfileDownload 下载用户资料
    // 对应 CLI: -profile-user <screen_name>
    ProfileDownload(ctx context.Context, taskID string, screenNames []string, reporter ProgressReporter) error
    
    // ListProfileDownload 下载列表用户资料
    // 对应 CLI: -profile-list <list_id>
    ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter ProgressReporter) error
    
    // MarkDownloaded 标记已下载
    // 对应 CLI: -user <screen_name> -mark-downloaded
    MarkDownloaded(ctx context.Context, taskID string, screenName string, markTime *string, reporter ProgressReporter) error
    
    // JsonDownload 从JSON下载
    // 对应 CLI: -json <paths...>
    JsonDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error
    
    // BatchDownload 批量下载
    // 对应 CLI: -user <u1> -user <u2> -list <l1>
    BatchDownload(ctx context.Context, taskID string, users []string, lists []uint64, opts DownloadOptions, reporter ProgressReporter) error
}
```

**风险评估**:
- 风险: 低
- 注意: 方法名与 CLI 参数对应，便于理解

**测试要点**:
- 编译通过

---

### 步骤 4: 创建依赖注入

**文件**: `internal/service/deps.go`

**新增代码**:
```go
package service

import (
    "github.com/go-resty/resty/v2"
    "github.com/jmoiron/sqlx"
    "github.com/unkmonster/tmd/internal/config"
    "github.com/unkmonster/tmd/internal/repository"
)

// Dependencies Service 依赖
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
        // Repository 实例
        userRepo:   repository.NewUserRepository(deps.DB),
        listRepo:   repository.NewListRepository(deps.DB),
        entityRepo: repository.NewEntityRepository(deps.DB),
        linkRepo:   repository.NewLinkRepository(deps.DB),
    }
}
```

**风险评估**:
- 风险: 低
- 依赖: 需要 Phase 1 的 repository 包

---

### 步骤 5: 实现 DownloadService

**文件**: `internal/service/download_service.go`

**新增代码**:
```go
package service

import (
    "context"
    "fmt"
    "path/filepath"
    
    "github.com/go-resty/resty/v2"
    log "github.com/sirupsen/logrus"
    
    "github.com/unkmonster/tmd/internal/cli"
    "github.com/unkmonster/tmd/internal/downloader"
    "github.com/unkmonster/tmd/internal/downloading"
    "github.com/unkmonster/tmd/internal/repository"
    "github.com/unkmonster/tmd/internal/twitter"
)

type downloadServiceImpl struct {
    deps *Dependencies
    
    // Repository
    userRepo   repository.UserRepository
    listRepo   repository.ListRepository
    entityRepo repository.EntityRepository
    linkRepo   repository.LinkRepository
}

// UserDownload 实现
func (s *downloadServiceImpl) UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error {
    reporter.OnProgress(taskID, Progress{Stage: "downloading", Current: screenName})
    
    // 获取存储路径
    pathHelper, err := cli.NewStorePath(s.deps.Config.RootPath)
    if err != nil {
        reporter.OnError(taskID, fmt.Errorf("failed to make store dir: %w", err))
        return err
    }
    
    // 获取用户信息
    user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
    if err != nil {
        s.userRepo.MarkInaccessible(ctx, uid, screenName)
        reporter.OnError(taskID, err)
        return err
    }
    
    // 初始化下载器
    versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
    fileWriter := downloader.NewFileWriter(versionManager)
    versionManager.SetFileWriter(fileWriter)
    dwn := downloader.NewDownloader(fileWriter)
    
    // 执行下载
    users := []downloading.UserInListEntity{{User: user, Leid: nil}}
    _, err = downloading.BatchUserDownload(ctx, s.deps.Client, s.deps.DB, users, pathHelper.Users, opts.AutoFollow, s.deps.AdditionalClients, dwn, fileWriter)
    
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    // Profile 下载
    if !opts.SkipProfile {
        reporter.OnProgress(taskID, Progress{Stage: "profile", Current: screenName})
        s.downloadProfile(ctx, []string{screenName}, pathHelper, versionManager, fileWriter, dwn)
    }
    
    reporter.OnComplete(taskID, Result{Message: "User download completed"})
    return nil
}

// ListDownload 实现
func (s *downloadServiceImpl) ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error {
    reporter.OnProgress(taskID, Progress{Stage: "syncing", Current: fmt.Sprintf("list:%d", listID)})
    
    pathHelper, err := cli.NewStorePath(s.deps.Config.RootPath)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    // 获取列表信息
    list, err := twitter.GetLst(ctx, s.deps.Client, listID)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    // 同步列表并获取成员
    members, err := downloading.SyncListAndGetMembers(ctx, s.deps.Client, s.deps.DB, list, pathHelper.Root)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(members)})
    
    // 初始化下载器
    versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
    fileWriter := downloader.NewFileWriter(versionManager)
    versionManager.SetFileWriter(fileWriter)
    dwn := downloader.NewDownloader(fileWriter)
    
    // 执行下载
    _, err = downloading.BatchUserDownload(ctx, s.deps.Client, s.deps.DB, members, pathHelper.Users, opts.AutoFollow, s.deps.AdditionalClients, dwn, fileWriter)
    
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    // Profile 下载
    if !opts.SkipProfile {
        reporter.OnProgress(taskID, Progress{Stage: "profile"})
        // 提取成员 screen names
        var screenNames []string
        for _, m := range members {
            screenNames = append(screenNames, m.User.ScreenName)
        }
        s.downloadProfile(ctx, screenNames, pathHelper, versionManager, fileWriter, dwn)
    }
    
    reporter.OnComplete(taskID, Result{Message: "List download completed"})
    return nil
}

// FollowingDownload 实现
func (s *downloadServiceImpl) FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error {
    reporter.OnProgress(taskID, Progress{Stage: "downloading", Current: screenName})
    
    pathHelper, err := cli.NewStorePath(s.deps.Config.RootPath)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    // 获取用户信息
    user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
    if err != nil {
        s.userRepo.MarkInaccessible(ctx, uid, screenName)
        reporter.OnError(taskID, err)
        return err
    }
    
    // 使用 Following() 方法获取关注列表
    following := user.Following()
    
    // 获取关注列表成员
    membersResult, err := following.GetMembers(ctx, s.deps.Client)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    // 转换为 UserInListEntity
    var members []downloading.UserInListEntity
    for _, u := range membersResult.Users {
        members = append(members, downloading.UserInListEntity{User: u, Leid: nil})
    }
    
    reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(members)})
    
    // 初始化下载器
    versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
    fileWriter := downloader.NewFileWriter(versionManager)
    versionManager.SetFileWriter(fileWriter)
    dwn := downloader.NewDownloader(fileWriter)
    
    // 执行下载
    _, err = downloading.BatchUserDownload(ctx, s.deps.Client, s.deps.DB, members, pathHelper.Users, opts.AutoFollow, s.deps.AdditionalClients, dwn, fileWriter)
    
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    reporter.OnComplete(taskID, Result{Message: "Following download completed"})
    return nil
}

// ProfileDownload 实现
func (s *downloadServiceImpl) ProfileDownload(ctx context.Context, taskID string, screenNames []string, reporter ProgressReporter) error {
    reporter.OnProgress(taskID, Progress{Stage: "profile", Total: len(screenNames)})
    
    pathHelper, err := cli.NewStorePath(s.deps.Config.RootPath)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
    fileWriter := downloader.NewFileWriter(versionManager)
    versionManager.SetFileWriter(fileWriter)
    dwn := downloader.NewDownloader(fileWriter)
    
    s.downloadProfile(ctx, screenNames, pathHelper, versionManager, fileWriter, dwn)
    
    reporter.OnComplete(taskID, Result{Message: "Profile download completed"})
    return nil
}

// ListProfileDownload 实现
func (s *downloadServiceImpl) ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter ProgressReporter) error {
    reporter.OnProgress(taskID, Progress{Stage: "syncing", Current: fmt.Sprintf("list:%d", listID)})
    
    // 获取列表成员
    list, err := twitter.GetLst(ctx, s.deps.Client, listID)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    membersResult, err := list.GetMembers(ctx, s.deps.Client)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    var screenNames []string
    for _, u := range membersResult.Users {
        screenNames = append(screenNames, u.ScreenName)
    }
    
    return s.ProfileDownload(ctx, taskID, screenNames, reporter)
}

// MarkDownloaded 实现
func (s *downloadServiceImpl) MarkDownloaded(ctx context.Context, taskID string, screenName string, markTime *string, reporter ProgressReporter) error {
    reporter.OnProgress(taskID, Progress{Stage: "marking", Current: screenName})
    
    // 获取用户信息
    user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
    if err != nil {
        s.userRepo.MarkInaccessible(ctx, uid, screenName)
        reporter.OnError(taskID, err)
        return err
    }
    
    // 构建参数
    var opts downloading.MarkDownloadedOptions
    if markTime != nil {
        opts.MarkTime = *markTime
    }
    
    // 执行标记
    pathHelper, _ := cli.NewStorePath(s.deps.Config.RootPath)
    _, err = downloading.MarkUsersAsDownloaded(ctx, s.deps.Client, s.deps.DB, nil, []*twitter.User{user}, pathHelper.Users, opts.MarkTime)
    
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    reporter.OnComplete(taskID, Result{Message: "Marked as downloaded"})
    return nil
}

// JsonDownload 实现
func (s *downloadServiceImpl) JsonDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error {
    reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths)})
    
    pathHelper, err := cli.NewStorePath(s.deps.Config.RootPath)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
    fileWriter := downloader.NewFileWriter(versionManager)
    versionManager.SetFileWriter(fileWriter)
    dwn := downloader.NewDownloader(fileWriter)
    
    results := downloading.DownloadJsonDir(ctx, s.deps.Client, pathHelper.Root, dwn, fileWriter, paths...)
    
    var successCount, failCount int
    for _, r := range results {
        if r.Success {
            successCount++
        } else {
            failCount++
        }
    }
    
    reporter.OnComplete(taskID, Result{
        Downloaded: successCount,
        Failed:     failCount,
        Message:    fmt.Sprintf("JSON download: %d success, %d failed", successCount, failCount),
    })
    return nil
}

// BatchDownload 实现
func (s *downloadServiceImpl) BatchDownload(ctx context.Context, taskID string, users []string, lists []uint64, opts DownloadOptions, reporter ProgressReporter) error {
    reporter.OnProgress(taskID, Progress{Stage: "preparing"})
    
    pathHelper, err := cli.NewStorePath(s.deps.Config.RootPath)
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    // 获取用户
    var twitterUsers []*twitter.User
    for _, screenName := range users {
        user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
        if err != nil {
            s.userRepo.MarkInaccessible(ctx, uid, screenName)
            continue
        }
        twitterUsers = append(twitterUsers, user)
    }
    
    // 获取列表
    var twitterLists []twitter.ListBase
    for _, listID := range lists {
        list, err := twitter.GetLst(ctx, s.deps.Client, listID)
        if err != nil {
            continue
        }
        twitterLists = append(twitterLists, list)
    }
    
    reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(twitterUsers) + len(twitterLists)})
    
    // 初始化下载器
    versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
    fileWriter := downloader.NewFileWriter(versionManager)
    versionManager.SetFileWriter(fileWriter)
    dwn := downloader.NewDownloader(fileWriter)
    
    // 执行批量下载
    _, err = downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, twitterLists, twitterUsers, pathHelper.Root, pathHelper.Users, opts.AutoFollow, s.deps.AdditionalClients, dwn, fileWriter)
    
    if err != nil {
        reporter.OnError(taskID, err)
        return err
    }
    
    // Profile 下载
    if !opts.SkipProfile {
        var screenNames []string
        for _, u := range twitterUsers {
            screenNames = append(screenNames, u.ScreenName)
        }
        s.downloadProfile(ctx, screenNames, pathHelper, versionManager, fileWriter, dwn)
    }
    
    reporter.OnComplete(taskID, Result{Message: "Batch download completed"})
    return nil
}

// 内部辅助方法：下载 Profile
func (s *downloadServiceImpl) downloadProfile(ctx context.Context, screenNames []string, pathHelper *cli.StorePath, versionManager downloader.VersionManager, fileWriter downloader.FileWriter, dwn downloader.Downloader) {
    // 这里需要调用 internal/downloading/profile 包
    // 由于该包较复杂，暂时保持简化实现
    log.Infof("Downloading profiles for %d users", len(screenNames))
}
```

**风险评估**:
- 风险: **高**（核心业务逻辑）
- 注意: 
  - 需要确保与现有 CLI 行为一致
  - 错误处理需要完善
  - Profile 下载部分需要进一步实现

**测试要点**:
- 编译通过
- 各方法能正确执行
- 进度报告正常触发

---

### 步骤 6: 修改 downloading 包支持进度（可选）

如果需要更细粒度的进度报告，可以修改 `internal/downloading` 包：

**文件**: `internal/downloading/types.go`（新增）

**新增代码**:
```go
package downloading

// ProgressCallback 进度回调函数
type ProgressCallback func(stage string, current string, completed int, total int)

// ContextKey 用于在 context 中存储回调
var ContextKey = struct{}{}

// GetProgressCallback 从 context 获取回调
func GetProgressCallback(ctx context.Context) ProgressCallback {
    if cb, ok := ctx.Value(ContextKey).(ProgressCallback); ok {
        return cb
    }
    return nil
}
```

**注意**: 这是可选优化，Phase 2 可以先使用简单的 Stage 报告。

---

## 5. 与现有代码的关系

### 5.1 复用的代码
- `internal/downloading/*` - 核心下载逻辑
- `internal/downloader/*` - 下载器初始化
- `internal/twitter/*` - Twitter API 调用
- `internal/cli/paths.go` - 路径管理

### 5.2 不修改的代码（Phase 2）
- `internal/cli/*` - Phase 3 再适配
- `internal/api/*` - Phase 4 再适配

### 5.3 依赖的代码（Phase 1）
- `internal/repository/*` - User/Entity 数据访问

---

## 6. 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| 与 CLI 行为不一致 | 高 | 高 | 1. 仔细对照 CLI 执行流程<br>2. 编写对比测试<br>3. 逐步迁移验证 |
| 进度报告不准确 | 中 | 中 | 1. 先使用粗粒度 Stage 报告<br>2. 后续优化细粒度进度 |
| 错误处理不完善 | 中 | 高 | 1. 统一错误包装<br>2. 区分可恢复/致命错误 |
| Profile 下载未完整实现 | 中 | 中 | Phase 2 先实现基本功能，Phase 3 完善 |

---

## 7. 成功标准

- [ ] Service 接口定义完成
- [ ] 所有 Service 方法实现完成
- [ ] 代码编译通过
- [ ] 各方法能正确调用 downloading 包
- [ ] 进度报告机制正常工作
- [ ] 代码符合 CLAUDE.md 准则

---

## 8. 预计时间

- 步骤 1 (分析): 1-2 小时
- 步骤 2-4 (基础代码): 2-3 小时
- 步骤 5 (核心实现): 4-6 小时
- 步骤 6 (可选优化): 2 小时
- 测试与验证: 2-3 小时
- **总计**: 11-16 小时

---

## 9. 后续工作（Phase 3 & 4）

### Phase 3: CLI 适配
- 修改 `internal/cli/executor.go` 调用 Service
- 实现 CLI 的 ProgressReporter（日志输出）
- 移除重复的初始化逻辑

### Phase 4: API 适配
- 修改 `internal/api/server.go` 调用 Service
- 实现 API 的 ProgressReporter（SSE 推送）
- 移除 `AsyncExecutor` 和 `BuildArgs`
