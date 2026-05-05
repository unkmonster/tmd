# Service 层文档

> 本文档用于快速了解 `internal/service/` 层的设计、结构与实现细节，加速后续开发。

---

## 1. 概述

Service 层是 TMD 项目的**核心业务编排层**，位于 CLI / API 层与底层下载引擎之间。它的职责是：

- **统一入口**：为 CLI 和 Web API 提供相同的下载业务逻辑，避免重复实现
- **流程编排**：协调 `downloading`、`downloader`、`twitter`、`database` 等底层包完成完整下载流程
- **进度抽象**：通过 `ProgressReporter` 接口将进度上报与具体展示解耦
- **依赖注入**：通过 `Dependencies` 结构体集中管理外部依赖，便于测试和替换

### 架构位置

```
┌─────────────────────────────────────────────────┐
│  main.go                                        │
│    ├── CLI 模式 → cli.Execute()                 │
│    └── Server 模式 → api.Server                 │
└──────────────┬──────────────────────────────────┘
               │ 调用 DownloadService 接口
┌──────────────▼──────────────────────────────────┐
│           ★ Service 层 (本层) ★                  │
│  interfaces.go / deps.go / download_service.go  │
│  progress.go                                    │
└──────────────┬──────────────────────────────────┘
               │ 编排调用
┌──────────────▼──────────────────────────────────┐
│  downloading / downloader / profile             │
│  twitter / database / path / config / entity    │
└─────────────────────────────────────────────────┘
```

---

## 2. 文件清单与职责

| 文件 | 职责 | 核心导出 |
|------|------|----------|
| `interfaces.go` | 定义 `DownloadService` 接口和 `DownloadOptions` 结构体 | `DownloadService`, `DownloadOptions` |
| `deps.go` | 依赖注入结构体和工厂函数 | `Dependencies`, `NewDownloadService()` |
| `download_service.go` | `DownloadService` 的实现 `downloadServiceImpl` 及所有业务方法 | （私有实现，通过接口暴露） |
| `progress.go` | 进度报告相关类型和接口 | `Progress`, `Result`, `MainResult`, `ProfileResult`, `ProgressReporter`, `NopReporter`, `LogReporter` |
| `deps_test.go` | 依赖验证和工厂函数的单元测试 | — |
| `download_service_test.go` | 下载服务实现的单元测试 + Mock 工具 | `MockProgressReporter`, `createTestDependencies()` |
| `progress_test.go` | 进度报告类型的单元测试 | — |

---

## 3. 核心接口与类型

### 3.1 DownloadService 接口

定义于 `interfaces.go`，是 Service 层对外暴露的唯一业务接口：

```go
type DownloadService interface {
    UserDownload(ctx, taskID, screenName, opts, reporter) error
    ListDownload(ctx, taskID, listID, opts, reporter) error
    FollowingDownload(ctx, taskID, screenName, opts, reporter) error
    ProfileDownload(ctx, taskID, screenNames, reporter) error
    ListProfileDownload(ctx, taskID, listID, reporter) error
    MarkDownloaded(ctx, taskID, screenNames, listIDs, followingNames, markTime, reporter) error
    JsonFileDownload(ctx, taskID, paths, noRetry, reporter) error
    JsonFolderDownload(ctx, taskID, paths, noRetry, reporter) error
    BatchDownload(ctx, taskID, screenNames, listIDs, followingNames, opts, reporter) error
}
```

**方法分类**：

| 类别 | 方法 | 说明 |
|------|------|------|
| 推文下载 | `UserDownload` | 下载单个用户的推文媒体 |
| | `ListDownload` | 下载 Twitter 列表成员的推文媒体 |
| | `FollowingDownload` | 下载某用户关注列表的推文媒体 |
| | `BatchDownload` | 批量下载（用户+列表+关注组合） |
| 资料下载 | `ProfileDownload` | 下载指定用户的头像/横幅/metadata |
| | `ListProfileDownload` | 下载列表成员的资料 |
| 标记操作 | `MarkDownloaded` | 标记用户/列表为已下载（跳过下载） |
| JSON 导入 | `JsonFileDownload` | 从第三方工具导出的 JSON 文件下载媒体 |
| | `JsonFolderDownload` | 从 TMD 生成的 .loongtweet 文件夹下载媒体 |

### 3.2 DownloadOptions

```go
type DownloadOptions struct {
    AutoFollow  bool  // 自动关注列表中的用户
    SkipProfile bool  // 跳过资料（头像/横幅）下载
    NoRetry     bool  // 不重试失败的推文
}
```

适用于推文下载类方法（`UserDownload`、`ListDownload`、`FollowingDownload`、`BatchDownload`）。

### 3.3 Dependencies

```go
type Dependencies struct {
    Client            *resty.Client      // 主 HTTP 客户端（主账号）
    AdditionalClients []*resty.Client    // 附加账号客户端（负载均衡）
    DB                *sqlx.DB           // SQLite 数据库
    Config            *config.Config     // 应用配置（必须包含 RootPath）
}
```

**验证规则**（`Validate()` 方法）：
- `Client` 不能为 nil
- `DB` 不能为 nil
- `Config` 不能为 nil
- `Config.RootPath` 不能为空
- `AdditionalClients` 可以为空切片（可选依赖）

### 3.4 ProgressReporter 接口

```go
type ProgressReporter interface {
    OnProgress(taskID string, p Progress)    // 进度更新
    OnComplete(taskID string, r Result)      // 任务完成
    OnError(taskID string, err error)        // 任务错误
}
```

**重要设计决策**：`OnError` 仅用于最终任务状态上报。Service 层的 fatal error 应**直接返回 error**，由外层编排代码统一决定是否调用 `OnError`。

### 3.5 进度与结果类型

```go
type Progress struct {
    Stage     string  // 阶段标识
    Total     int     // 总数
    Completed int     // 已完成数
    Failed    int     // 失败数
    Current   string  // 当前处理项描述
}

type MainResult struct {
    Downloaded int     // 成功下载数
    Failed     int     // 失败数
}

type ProfileResult struct {
    Downloaded int     // 成功下载数
    Failed     int     // 失败数
    Versioned  int     // 版本化文件数（旧文件已备份到 .versions）
}

type Result struct {
    Main    *MainResult     // 推文下载结果（可选）
    Profile *ProfileResult  // 资料下载结果（可选）
    Message string          // 结果描述信息
}
```

**Progress.Stage 取值**：

| Stage | 含义 | 触发场景 |
|-------|------|----------|
| `"syncing"` | 同步列表成员 | ListDownload, FollowingDownload 开始时 |
| `"downloading"` | 下载中 | 推文媒体下载过程中 |
| `"retrying"` | 重试中 | 重试失败推文时 |
| `"profile"` | 资料下载中 | 头像/横幅下载时 |
| `"profile_warning"` | 资料下载警告 | 资料下载部分失败时 |
| `"marking"` | 标记中 | MarkDownloaded 执行时 |
| `"preparing"` | 准备中 | BatchDownload 准备阶段 |
| `"resolving"` | 解析中 | 解析用户名/列表ID时 |

---

## 4. 实现细节

### 4.1 downloadServiceImpl 结构

```go
type downloadServiceImpl struct {
    deps *Dependencies
}
```

私有实现，通过 `NewDownloadService()` 工厂函数创建，返回 `DownloadService` 接口。

### 4.2 通用下载流程（推文下载类方法）

以 `UserDownload` 为例，所有推文下载方法遵循相同流程：

```
1. getReporterOrDefault(reporter)           → 确保 reporter 非 nil
2. OnProgress(stage)                        → 上报初始阶段
3. path.NewStorePath(config.RootPath)       → 获取存储路径
4. downloading.NewDumper() + Load()         → 加载失败推文记录
5. defer saveDumper()                       → 确保退出时保存
6. twitter.GetUserByScreenName / GetLst     → 获取 Twitter 实体
7. initDownloader()                         → 初始化 VersionManager + FileWriter + Downloader
8. newBatchProgressCallback / newRetryProgressCallback → 创建进度回调
9. downloading.BatchDownloadAny()           → 执行批量下载
10. collectFailedTweetSet()                 → 收集失败推文
11. collectFailedTweets(dumper)             → 写入 Dumper
12. downloading.RetryFailedTweets()         → 重试失败推文（除非 NoRetry）
13. downloadProfile()                       → 下载资料（除非 SkipProfile）
14. completeTask()                          → 上报完成结果
```

### 4.3 内部辅助方法

| 方法 | 作用 |
|------|------|
| `getReporterOrDefault(reporter)` | nil reporter 替换为 `NopReporter` |
| `completeTask(taskID, reporter, message, stats, warning)` | 统一完成上报，合并 warning 到 message |
| `completeProfileTask(taskID, reporter, profileResult)` | 纯资料任务的完成上报 |
| `newBatchProgressCallback(taskID, reporter)` | 创建 `BatchProgressFunc` 回调，映射为 `Progress{Stage: "downloading"}` |
| `newRetryProgressCallback(taskID, reporter)` | 创建 `RetryProgressFunc` 回调，映射为 `Progress{Stage: "retrying"}` |
| `buildMainDownloadResult(summary, failed)` | 从 `BatchDownloadSummary` 构建 `MainResult` |
| `resolveUsers(ctx, screenNames)` | 批量解析用户名 → `[]*twitter.User`，失败则标记不可访问 |
| `resolveLists(ctx, listIDs)` | 批量解析列表ID → `[]twitter.ListBase` |
| `resolveFollowings(ctx, screenNames)` | 批量解析关注列表 → `[]twitter.ListBase` |
| `initDownloader()` | 创建 `VersionManager` + `FileWriter` + `Downloader` 三件套 |
| `saveDumper(dumper, path)` | 保存失败推文记录到文件，无记录则删除文件 |
| `collectFailedTweets(dumper, failedTweets)` | 将失败推文推入 Dumper |
| `downloadProfile(ctx, taskID, users, ...)` | 执行资料下载，创建 `ProfileDownloaderWithDB`，统计结果 |

### 4.4 失败推文追踪机制

```
BatchDownloadAny() 返回 failedTweets
    │
    ├── collectFailedTweetSet() → failedTweetSet (map[entityID]map[tweetID]struct{})
    │       用于后续统计"仍失败的实体数"
    │
    └── collectFailedTweets() → dumper.Push(entityID, tweet)
            写入 TweetDumper 用于重试

RetryFailedTweets() → 重试 dumper 中的推文
    │
    └── countRemainingFailedEntities() → 统计重试后仍失败的实体数
            用于构建最终的 MainResult.Failed
```

### 4.5 Profile 下载流程

```
downloadProfile(ctx, taskID, users, pathHelper, versionManager, fileWriter, dwn, reporter)
    │
    ├── profile.NewFileStorageManager(pathHelper.Users)  → 创建存储管理器
    ├── storage.SetVersionManager(versionManager)        → 设置版本管理器
    ├── profile.NewProfileDownloaderWithDB(...)           → 创建资料下载器
    │       参数: config, storage, clients(主+附加), DB, dwn, fileWriter
    ├── 构建 []profile.DownloadRequest                    → 从 User 构建请求
    ├── pd.DownloadMultiple(ctx, requests)                → 执行批量下载
    └── 统计 successCount / failCount / versionedFileCount → 返回 *ProfileResult
```

### 4.6 特殊方法说明

**JsonFileDownload** / **JsonFolderDownload**：
- `noRetry` 参数被显式忽略（`_ = noRetry`）
- 原因：第三方 JSON / loongtweet 文件夹下载不涉及 `TweetDumper` 机制，失败项不会进入 `error.json`，因此无需重试逻辑

**MarkDownloaded**：
- 不执行实际下载，仅标记用户为已下载状态
- 内部调用 `downloading.MarkUsersAsDownloaded()`
- 支持传入 `markTime` 参数指定标记时间

---

## 5. ProgressReporter 实现

### 5.1 NopReporter

空操作报告器，所有方法为空实现。用于不需要进度报告的场景（如测试、后台静默执行）。

### 5.2 LogReporter

日志报告器，用于 CLI 模式。通过注入的 `logger` 函数输出日志。

**特殊行为**：
- `downloading` 阶段的进度**被静默**（不输出），因为下载过程会产生大量进度更新
- 其他阶段正常输出

```go
reporter := NewLogReporter(log.Printf)
```

### 5.3 调用方的实现

- **API 层**：`SSEProgressReporter` — 通过 Server-Sent Events 实时推送到 Web 前端
- **CLI 层**：使用 `LogReporter` — 输出到终端日志
- **测试**：`MockProgressReporter` — 记录所有调用用于断言

---

## 6. 调用方接入

### 6.1 API Server 接入

`internal/api/server.go` 中：

```go
downloadService, err := service.NewDownloadService(&service.Dependencies{
    Client:            client,
    AdditionalClients: additionalClients,
    DB:                db,
    Config:            config,
})
s.downloadService = downloadService
```

路由处理器通过 `s.downloadService.XxxDownload()` 调用，配合 `TaskManager` 异步执行和 `SSEProgressReporter` 推送进度。

### 6.2 CLI 接入

`internal/cli/executor.go` 中：

```go
if deps.DownloadService == nil {
    downloadService, err := service.NewDownloadService(&deps.Dependencies)
    deps.DownloadService = downloadService
}
```

CLI 使用 `LogReporter` 输出进度。

---

## 7. 依赖关系图

```
service
  ├── importing
  │     ├── github.com/go-resty/resty/v2        → HTTP 客户端
  │     ├── github.com/jmoiron/sqlx              → 数据库
  │     ├── github.com/sirupsen/logrus           → 日志
  │     ├── github.com/unkmonster/tmd/internal/config
  │     ├── github.com/unkmonster/tmd/internal/database
  │     ├── github.com/unkmonster/tmd/internal/downloader
  │     ├── github.com/unkmonster/tmd/internal/downloading
  │     ├── github.com/unkmonster/tmd/internal/downloading/profile
  │     ├── github.com/unkmonster/tmd/internal/path
  │     └── github.com/unkmonster/tmd/internal/twitter
  │
  └── used by
        ├── internal/api/server.go               → API Server
        ├── internal/cli/executor.go             → CLI
        └── internal/service/*_test.go           → 单元测试
```

---

## 8. 测试指南

### 8.1 测试辅助工具

| 工具 | 位置 | 用途 |
|------|------|------|
| `MockProgressReporter` | `download_service_test.go` | 记录 OnProgress/OnComplete/OnError 调用 |
| `createTestDependencies(t)` | `download_service_test.go` | 创建内存 SQLite + resty 客户端的测试依赖 |
| `newFailedTweet(entityID, tweetID)` | `download_service_test.go` | 构造 `TweetInEntity` 测试数据 |

### 8.2 测试覆盖要点

- **依赖验证**：`Dependencies.Validate()` 的 nil/空值场景
- **工厂函数**：`NewDownloadService()` 的成功/失败路径
- **Reporter 处理**：nil reporter → NopReporter 替换
- **进度回调**：`BatchProgress` / `RetryProgress` → `Progress` 的映射
- **结果构建**：`buildMainDownloadResult` 的边界条件
- **失败追踪**：`collectFailedTweetSet` + `countRemainingFailedEntities`
- **完成上报**：`completeTask` / `completeProfileTask` 的消息格式
- **错误处理**：Service 层错误直接返回，不调用 `OnError`
- **接口合规**：`downloadServiceImpl` 满足 `DownloadService` 接口

### 8.3 添加新测试的模式

```go
func TestXxx(t *testing.T) {
    deps := createTestDependencies(t)
    deps.Config.RootPath = t.TempDir()  // 如需文件系统

    service, err := NewDownloadService(deps)
    require.NoError(t, err)
    impl := service.(*downloadServiceImpl)

    reporter := NewMockProgressReporter()
    // ... 测试逻辑 ...
}
```

---

## 9. 扩展指南

### 9.1 添加新的下载方法

1. 在 `interfaces.go` 的 `DownloadService` 接口中添加方法签名
2. 在 `download_service.go` 的 `downloadServiceImpl` 中实现
3. 遵循现有流程模式：`getReporterOrDefault` → `OnProgress` → 业务逻辑 → `completeTask`
4. 在 `download_service_test.go` 中添加测试
5. 在 `internal/api/download_handlers.go` 和 `internal/cli/executor.go` 中添加调用入口

### 9.2 添加新的 ProgressReporter 实现

实现 `ProgressReporter` 接口的三个方法即可：

```go
type MyReporter struct{}

func (r *MyReporter) OnProgress(taskID string, p Progress) { ... }
func (r *MyReporter) OnComplete(taskID string, r Result)   { ... }
func (r *MyReporter) OnError(taskID string, err error)     { ... }
```

### 9.3 添加新的依赖项

1. 在 `deps.go` 的 `Dependencies` 结构体中添加字段
2. 在 `Validate()` 方法中添加验证逻辑
3. 更新 `deps_test.go` 中的测试用例
4. 更新所有 `NewDownloadService()` 调用点

---

## 10. 设计决策记录

| 决策 | 原因 |
|------|------|
| Service 层错误直接返回 error，不调用 `OnError` | 让外层（API/CLI）统一决定错误处理策略 |
| `AdditionalClients` 可为空 | 单账号场景下无需附加客户端 |
| `JsonFileDownload`/`JsonFolderDownload` 忽略 `noRetry` | 这些场景不涉及 TweetDumper 机制，无重试基础 |
| `NopReporter` 替换 nil reporter | 避免每个方法都做 nil 检查 |
| `LogReporter` 静默 `downloading` 阶段 | 下载过程进度更新过于频繁，不适合日志输出 |
| Profile 下载复用 `BatchDownloadAny` 返回的 `listMembers` | 避免重复调用 `GetMembers` API |
| `downloadServiceImpl` 私有 | 强制通过接口使用，便于替换实现 |
