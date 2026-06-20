# AGENTS.md

面向本仓库的编码 agent 开发指南。目标是让后续开发少走弯路：先理解边界，再做小而准的改动，最后用合适的测试闭环。

本文档基于实际代码阅读，描述的是当前代码库的真实架构和设计意图。

---

## 项目概述

本项目是 Go 编写的 **Twitter/X Media Downloader (TMD)**，支持 CLI、API Server 和内置 Web 管理界面。

- 语言：Go 1.25.0
- 数据库：SQLite（`modernc.org/sqlite`，纯 Go 实现，`CGO_ENABLED=0`）
- HTTP 客户端：`go-resty/resty/v2`
- 日志：`sirupsen/logrus` + `natefinch/lumberjack`（轮转） + `rifflock/lfshook`（文件 hook）
- 任务队列：自定义 `DownloadQueue`（基于 `sync.Cond`） + `ants/v2` goroutine 池
- 计划任务：自定义 `Scheduler`（基于 cron-like 解析）
- 前 端：原生 HTML/CSS/JS（无构建步骤）

---

## 一、整体架构分层

```
┌──────────────┐
│   main.go    │  ← 进程入口：配置、登录、数据库、分流
└──────┬───────┘
       │
       ├─ CLI 模式 ───────→ internal/cli
       │                     （同步执行，直接返回）
       │
       └─ Server 模式 ────→ internal/api
                            （异步任务，SSE 推送）
                            │
                            ├─ Web UI (internal/api/web/)
                            ├─ TaskManager + DownloadQueue
                            ├─ EventBus → SSE 广播
                            └─ Scheduler（定时计划任务）
                                    │
                                    ▼
       ┌──────────────────────────────────────┐
       │        internal/service              │ ← 统一应用服务层
       │        DownloadService 接口           │
       │        11 种下载/操作方法的统一入口     │
       └──────────────────┬───────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
  internal/downloading  internal/twitter  internal/database
  （业务编排）          （Twitter API）    （SQLite 持久化）
          │
          ▼
  internal/downloader ← internal/entity
  （单文件下载）        （实体抽象）
          │
          ▼
  internal/naming （命名规则）
  internal/path   （路径管理）
  internal/utils  （通用工具）

  其他支撑层：
  internal/config      ← 配置管理（YAML + 环境变量）
  internal/consolelog  ← 控制台日志捕获（供 Web UI 实时显示）
  internal/scheduler   ← 计划任务系统
```

---

## 二、进程入口 — `main.go`

`main.go` 是唯一的进程入口。按严格顺序执行以下步骤：

### 2.1 全局预解析 (`parseBootstrapArgs`)
只识别**引导级参数**，不解析下载参数：
- `-server` → 启动 Server 模式
- `-port <n>` → 指定 HTTP 端口（默认 25556，支持 `$TMD_PORT` 环境变量）
- `-dbg` → 调试模式
- `-conf` → 交互式配置模式

其余参数原样保留为 `cliArgs` 传递给 CLI 模式。

### 2.2 初始化顺序
1. **确定应用根目录** (`resolveAppRootPath`)：优先 `$TMD_HOME`，其次 `$APPDATA/.tmd2`（Windows）或 `$HOME/.tmd2`
2. **初始化日志**：logrus + lumberjack 文件轮转 + consolelog 捕获
3. **加载配置**：`conf.yaml` + `TMD_*` 环境变量覆盖
4. **代理设置**：`conf.yaml` 中的 `proxy_url` 覆盖系统设置；无代理时自动同步 HTTP_PROXY ↔ HTTPS_PROXY
5. **分流**：`-server` 标志 → `runServer()`，否则 → `cli.Execute()`

### 2.3 两种模式的差异

| 方面 | CLI 模式 | Server 模式 |
|------|----------|-------------|
| 执行方式 | 同步，等待完成后退 | 异步，创建任务后立即返回 |
| 客户端日志 | O_TRUNC（每次截断重新写入） | O_APPEND（追加模式） |
| 信号处理 | `cancel()`（取消 context） | `server.GracefulShutdown()` |
| 资源清理 | `defer db.Close()` | `GracefulShutdown` 统一处理 |
| 进度报告 | `LogReporter`（日志输出） | `SSEProgressReporter`（推送至 Web UI） |

### 2.4 数据目录结构

```
{appRootPath}（默认 ~/.tmd2 或 %APPDATA%\.tmd2）
├── conf.yaml                  ← 主配置
├── additional_cookies.yaml    ← 额外账号 cookies
├── tmd2.log                   ← 程序日志（轮转）
├── client.log                 ← HTTP 客户端日志
└── schedules.yaml             ← 计划任务配置

{rootPath}（下载根目录，由配置 root_path 指定）
├── users/                     ← 用户下载目录
│   ├── {screen_name}/         ← 每个用户一个目录
│   │   ├── ... 媒体文件
│   │   └── .loongtweet/       ← 推文元数据 JSON/TXT
│   ├── {list_name}/           ← List 目录（通过软链接关联用户）
│   └── ...
└── .data/
    ├── foo.db                 ← SQLite 数据库
    ├── errors.json            ← 常规下载失败记录
    └── json_errors.json       ← JSON 导入下载失败记录
```

---

## 三、两条主调用链

### 3.1 CLI 模式调用链

```text
main.go
  → cli.Execute(ctx, args, deps)
    → cli.ParseArgs(args)           # args.go：解析 -user / -list / -foll / -jsonfile 等
    → cliTaskSelection.primaryMode() # 判断首要任务类型，处理优先级
    → switch on primaryMode:
        case cliTaskModeJSONFile:
          → service.JsonFileDownload(ctx, "cli", paths, noRetry, reporter)
        case cliTaskModeJSONFolder:
          → service.JsonFolderDownload(ctx, "cli", paths, noRetry, reporter)
        case cliTaskModeMarkDownloaded:
          → service.MarkDownloaded(ctx, "cli", users, lists, following, markTime, reporter)
        case cliTaskModeBatch:
          → 单用户: service.UserDownload()
            单关注: service.FollowingDownload()
            多/混合: service.BatchDownload()
        case cliTaskModeProfile:
          → service.ProfileDownload() / service.ListProfileDownload()
```

**关键设计**：
- CLI 参数有严格优先级：`-jsonfile` > `-jsonfolder` > `-mark-downloaded` > `-user/-list/-foll`（可组合）> `-profile-user/-profile-list`（可组合）
- 高优先级参数独占执行，低优先级被忽略
- 批量下载与 Profile 下载可以同时执行

### 3.2 API Server 模式调用链

```text
HTTP 请求
  → server.buildHandler() 路由分发
    → handler 函数（如 handleUserDownload）
      → taskManager.CreateTask(type, data)    创建任务 → task_id
      → buildTaskRunFunc(task)                构建执行闭包
      → downloadQueue.Enqueue(task, runFunc)   入队
      → 返回 HTTP 202 (Accepted) + task_id

后台：
  → DownloadQueue.workerLoop()
    → nextJob() → 持有 target 锁（防止同名用户并发下载）
    → UpdateTaskStatus(id, TaskStatusRunning)
    → SSEProgressReporter.OnProgress()
    → goroutine 执行 runFunc(task.Ctx, taskID, reporter)
      → service.DownloadService.*  ← 同一个 service 层
      → reporter.OnComplete() / OnError()
    → 释放 target 锁
    → SSE 事件通过 EventBus → sse_tasks handler → 浏览器
```

**异步任务调度流程**：
```
HTTP Request → handler
  → TaskManager.CreateTask()       # 返回 task{ID, Status="queued"}
    → DownloadQueue.Enqueue(task)   # 入 pending 队列
      → workerLoop 消费
        → UpdateTaskStatus("running")
        → goroutine 执行 runFunc
          → service.DownloadService.*
          → reporter.OnProgress / OnComplete / OnError
```

**TMD API 路由表**（Go 1.22+ 的 `mux.HandleFunc("METHOD /path", handler)` 风格）：

```
# 下载
POST /api/v1/users/{screen_name}/download
POST /api/v1/users/{screen_name}/profile
POST /api/v1/users/{screen_name}/mark
POST /api/v1/users/{screen_name}/following/download
POST /api/v1/users/{screen_name}/following/mark
POST /api/v1/lists/{list_id}/download
POST /api/v1/lists/{list_id}/profile
POST /api/v1/lists/{list_id}/mark
POST /api/v1/json/file/download
POST /api/v1/json/folder/download
POST /api/v1/batch/download
POST /api/v1/batch/mark

# 任务管理
GET  /api/v1/tasks
GET  /api/v1/tasks/stats
GET  /api/v1/tasks/{task_id}
POST /api/v1/tasks/{task_id}/cancel
POST /api/v1/tasks/cancel-queued
POST /api/v1/tasks/{task_id}/retry
DELETE /api/v1/tasks/{task_id}

# 错误重试
GET  /api/v1/errors
POST /api/v1/errors/retry
DELETE /api/v1/errors

# 数据库管理
GET  /api/v1/db/users
GET  /api/v1/db/users/{id}
PATCH /api/v1/db/users/{id}
DELETE /api/v1/db/users/{id}
GET  /api/v1/db/users/{id}/previous-names
GET  /api/v1/db/users/{id}/entities
GET  /api/v1/db/users/{id}/links
GET  /api/v1/db/lists
GET  /api/v1/db/lists/{id}
PATCH /api/v1/db/lists/{id}
DELETE /api/v1/db/lists/{id}
GET  /api/v1/db/lists/{id}/entities
GET  /api/v1/db/user-entities
GET/PATCH/DELETE /api/v1/db/user-entities/{id}
GET  /api/v1/db/list-entities
GET/PATCH/DELETE /api/v1/db/list-entities/{id}
GET  /api/v1/db/user-links
GET/PATCH/DELETE /api/v1/db/user-links/{id}
GET  /api/v1/db/user-previous-names
GET  /api/v1/db/stats

# 配置
GET  /api/v1/config
GET  /api/v1/config/raw
PUT  /api/v1/config/raw
GET  /api/v1/config/fields
PUT  /api/v1/config/fields

# Cookies
GET  /api/v1/cookies
PUT  /api/v1/cookies
GET  /api/v1/cookies/raw
PUT  /api/v1/cookies/raw

# 计划任务
GET/PUT/POST /api/v1/schedules
GET /api/v1/schedules/raw
PUT /api/v1/schedules/raw
POST /api/v1/schedules/reload
POST /api/v1/schedules/validate
POST /api/v1/schedules/trigger-all
GET  /api/v1/schedules/stats
PUT/DELETE /api/v1/schedules/{id}
PATCH /api/v1/schedules/{id}/enabled
POST /api/v1/schedules/{id}/trigger

# 日志
GET  /api/v1/logs
GET  /api/v1/logs/stats
GET  /api/v1/logs/export
GET  /api/v1/logs/stream

# 系统
GET  /api/v1/health
GET  /api/v1/queue/status
POST /api/v1/server/shutdown

# SSE
GET  /api/v1/sse/tasks

# Web UI
GET  / → Web UI
GET  /static/{path...} → 静态文件
```

---

## 四、核心服务层 — `internal/service`

### 4.1 DownloadService 接口

定义在 `interfaces.go`，提供 **11 种操作**：

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
    RetryAllFailed(ctx, taskID, reporter) error
    ClearErrors() error
}
```

### 4.2 DownloadServiceImpl 实现

- 结构体持有 `Dependencies`（Client, AdditionalClients, DB, Config, ListSyncManager）
- 所有下载方法通过 `executeDownloadTemplate()` 模板方法编排统一流程
- 内部用 `dumperMu sync.Mutex` 保护 `TweetDumper` 的并发读写

### 4.3 统一下载模板 `executeDownloadTemplate`

```text
1. 创建 ProgressReporter
2. 创建 StorePath（路径助手）
3. 加载 TweetDumper（失败推文记录器）
4. Prepare() — 解析用户/列表（不同方法提供不同的 Prepare 实现）
5. 创建 downloader / fileWriter / versionManager
6. BatchDownloadAny() 批量下载所有推文的媒体
7. 可选：FollowMembers（关注目标用户）
8. 收集失败推文到 dumper
9. 可选：RetryFailedTweets（重试失败项）
10. 可选：downloadProfile（下载头像/横幅）
11. completeTask（上报完成结果）
```

### 4.4 进度报告接口

```go
type ProgressReporter interface {
    OnProgress(taskID string, p Progress)
    OnComplete(taskID string, r Result)
    OnError(taskID string, err error)
}
```

两种实现：
- **`NopReporter`**：什么都不做（用于测试或不需要进度的场景）
- **`LogReporter`**：输出到日志（用于 CLI 模式）
- **`SSEProgressReporter`**：更新 TaskManager 中的进度/结果（用于 API 模式）

### 4.5 下载选项

```go
type DownloadOptions struct {
    AutoFollow    bool  // 自动关注未关注的受保护用户
    FollowMembers bool  // 下载 list 后关注所有成员
    SkipProfile   bool  // 跳过 Profile 下载
    NoRetry       bool  // 失败后不重试
}
```

---

## 五、下载业务流程层 — `internal/downloading`

这一层是下载业务的具体实现，不依赖 `internal/service`（反向依赖：service 依赖 downloading）。

### 5.1 核心类型

- **`PackagedTweet`** 接口 — 包装推文及其对应的实体路径
- **`TweetInEntity`** — 推文 + `entity.UserEntity`（用于常规下载）
- **`JsonPackagedTweet`** — 推文 + 用户目录（用于 JSON 导入下载）
- **`RuntimeOptions`** — 最大并发数、最大文件名长度
- **`BatchProgress` / `RetryProgress`** — 进度回调类型
- **`workerConfig`** — 推文下载 worker 配置（包含 context、downloader、client 等）

### 5.2 批量下载 — `batch_download.go`

**`BatchUserDownload`** 是核心批量下载函数。工作流程：

```
1. 预处理阶段：遍历每个用户
   - syncUserAndEntity() 同步用户到数据库和文件系统
   - 计算缺失推文数 → depthByEntity
   - 推入 UserEntity 优先队列（受保护已关注用户优先）
   - 创建软链接（List 成员 → List 目录）
   - 自动关注未关注的受保护用户（如果 AutoFollow=true）

2. 生产者-消费者模式：
   - 生产者：从优先队列弹出一个用户 → twitter API 拉取 → 推入 tweetChan
   - 消费者：N 个 worker 从 tweetChan 消费 → downloader.Download() → fileWriter.Write()
   - 使用 ants.Pool 控制生产者并发（max=35）
   - 使用 goroutine 池控制消费者并发（max=MaxDownloadRoutine）

3. 进度控制：
   - 每轮最多拉取 1500 条推文的 API 请求（userTweetRateLimit = 1500）
   - 受保护用户优先处理（只能主账号拉取，需提前占用配额）
```

### 5.3 推文下载 — `tweet_download.go`

- **`tweetDownloader`** — worker goroutine，从 `tweetChan` 读取推文，调用 `downloadTweetMedia` 下载每条推文的媒体
- **`downloadTweetMedia`** — 对推文中的每个 URL：
  1. 判断媒体类型（photo/video/gif）
  2. 调用 `downloader.Download()`
  3. 下载成功后原地删除 `tweet.Urls` 中的该项
  4. 记录失败项（非 403/404 错误才加入重试队列）
- **`saveLoongTweet`** — 异步保存 `.loongtweet/TweetId.txt` 文本文件（fire-and-forget）
- **`saveTweetJson`** — 异步保存 `.loongtweet/TweetId.json` JSON 元数据（fire-and-forget）

**关键行为**：`downloadTweetMedia` 会**原地修改**传入的 `tweet.Urls` 切片——成功下载的 URL 被删除，只留下失败项。这使得外部调用方可以检查 `len(tweet.Urls)` 判断是否完全成功。

### 5.4 失败重试 — `retry.go`

- **`RetryFailedTweets`** — 从 `TweetDumper` 加载失败推文，重新调用 `BatchDownloadTweet` 下载
- **`RetryFailedJsonTweets`** — 同上，用于 JSON 导入失败
- 成功下载的推文会从 dumper 中 `Remove`；仍然失败的保留供下次重试

### 5.5 失败记录器 — `dumper.go`

- **`TweetDumper`** — 按 entity ID 分组存储失败推文，支持 Push/Remove/Load/Dump
- **`JsonTweetDumper`** — 按来源路径分组存储 JSON 导入失败推文
- 数据持久化到 `.data/errors.json` 和 `.data/json_errors.json`
- Dump 为 JSON 格式，Load 时反序列化

### 5.6 列表同步 — `list_sync.go`

- **`ListSyncManager`** — 管理 List 成员同步
- **`SyncListMembers`** — 用事务同步当前成员列表，移除不再属于该 list 的用户的软链接
- 通过 `tx.Manager` 管理事务

### 5.7 实体同步 — `entity.go`

- **`syncUserAndEntity`** — 核心函数：同步 Twitter user → 数据库 User / UserEntity → 文件系统目录
- 处理用户名变更（检测到变更后重命名目录）
- 创建 `user_links` 记录 user ↔ lst_entity 关系
- 从 Twitter 用户信息同步到数据库

### 5.8 JSON 导入下载

- **`JsonFileDownload`** — 从第三方工具导出的推文搜索结果 JSON 文件下载媒体
- **`JsonFolderDownload`** — 从 TMD 生成的 `.loongtweet` 文件夹下载媒体
- 两者都使用 `JsonTweetDumper` 记录失败，支持重试
- **`BatchDownloadTweet`** — 底层通用函数，对一组 `PackagedTweet` 调用 worker 并发下载

### 5.9 标记已下载 — `mark_downloaded.go`

- 将指定用户/列表/关注的 `latest_release_time` 设置为当前时间
- 使后续增量下载跳过该时间点之前的推文
- 支持自定义标记时间

---

## 六、单文件下载层 — `internal/downloader`

只负责**单文件下载和原子写入**，不关心上层业务。

### 6.1 接口定义

```go
type Downloader interface {
    Download(req DownloadRequest) (*DownloadResult, error)
}

type FileWriter interface {
    Write(req WriteRequest) (WriteResult, error)
}

type VersionManager interface {
    CreateVersion(sourcePath string) (string, error)
}
```

### 6.2 DefaultDownloader

**下载策略选择**（根据 HEAD 请求得到的文件大小）：

| 文件大小 | 策略 | 说明 |
|---------|------|------|
| ≤ 10MB | Buffer 模式 | 一次 HTTP GET 读入内存 → fileWriter.Write |
| > 10MB | Stream 模式 | 流式响应 → 边读边写，支持重试（最多 2 次） |

- 使用**独立的 HTTP 客户端** `downloadClient`，不携带 Twitter 鉴权凭据
- 403/404 错误视为**不可重试**，直接返回
- 流模式下文件大小不匹配时删除不完整文件并重试

### 6.3 DefaultFileWriter

**原子写入流程**：
1. 创建临时文件（在目标目录下，`CreateTemp(dir, ".tmp_*")`）
2. 写入数据（Buffer 模式从 `[]byte`，Stream 模式从 `io.Reader`）
3. `os.Rename` 原子覆盖目标文件

**可选功能**：
- `SkipUnchanged`：通过大小+MD5 哈希跳过未变化的文件
- `CreateVersion`：在 `.versions/` 备份旧文件
- `SetModTime`：设置文件修改时间

**线程安全**：基于文件名哈希的 256 槽互斥锁（`getLock`），避免不同文件互锁，相同文件串行化。

### 6.4 DefaultVersionManager

- 在目标文件所在目录下创建 `.versions/` 子目录
- 备份文件名：`{original}.v{timestamp}`

---

## 七、Twitter API 层 — `internal/twitter`

封装 Twitter/X 私有接口访问，这是**改动风险最高的层**。

### 7.1 核心文件职责

| 文件 | 职责 |
|------|------|
| `client.go` | Resty 客户端配置、登录、多账号管理、限流、Bearer Token |
| `api.go` | GraphQL endpoint、请求构建、响应解析 |
| `user.go` | 用户查询（`GetUserByScreenName`）、用户媒体时间线参数、受保护用户判断 |
| `tweet.go` | 单条推文查询 |
| `timeline.go` | 用户时间线翻页拉取 |
| `list.go` | List 推文拉取 |
| `errors.go` | 错误类型（TwitterApiError, ErrWouldBlock, ErrAccountLocked 等）|
| `batch_login.go` | 多账号批量登录 |

### 7.2 多账号机制

- **主账号**：读取 `conf.yaml` 中的 `auth_token` + `ct0` 登录
- **附加账号**：读取 `additional_cookies.yaml` 批量登录
- **账号选择**：`SelectClientMFQ` 根据用户是否受保护选择合适的账号
  - 受保护用户 → 主账号（必须关注了该用户才能看到内容）
  - 非保护用户 → 优先使用附加账号分摊限流

### 7.3 限流

- `EnableRateLimit(client)` 为客户端启用限流
- 限流触发时会阻塞等待或抛出错误
- 账号级别错误（`SetClientError`）会使该账号被标记为不可用

---

## 八、数据库层 — `internal/database`

### 8.1 Schema（6 张表）

```
users (id, screen_name, name, protected, friends_count, is_accessible)
user_previous_names (id, user_id, screen_name, name, record_date)
lsts (id, name, owner_user_id)
lst_entities (id, lst_id, name, parent_dir)     -- UNIQUE(lst_id, parent_dir)
user_entities (id, user_id, name, latest_release_time, parent_dir, media_count)
user_links (id, user_id, name, parent_lst_entity_id)
```

### 8.2 关键设计

- **WAL 模式**，连接池最大打开 1 个连接（`sqlite.go`）
- **迁移**：`sqlite_migration.go` 中可重复执行（`ALTER TABLE ADD COLUMN`、`RENAME COLUMN`）
- **`tx.Manager`**：`internal/database/tx/manager.go` 提供事务管理
- **`parent_dir_migration.go`**：处理 `parent_dir` 路径变更的历史数据兼容
- **查询**：`query.go` 提供通用 CRUD，`helpers.go` 提供分页、排序等辅助
- **用户同步**：`user_sync.go` 检测用户名变更并记录历史
- **路径校验**：`path_validation.go` 确保存储路径安全

### 8.3 实体关系

```
users ──1:N── user_entities (同一用户在多个父目录下可以有不同的 entity)
users ──1:N── user_previous_names (用户名历史记录)
lsts ──1:N── lst_entities (同一 list 在多个父目录下可以有不同的 entity)
lst_entities ──1:N── user_links (list 成员关系，通过软链链接到相应用户目录)
user_entities ──1:N── user_links (用户实体通过软链被关联到 list 实体)
```

---

## 九、实体层 — `internal/entity`

### 9.1 Entity 接口

```go
type Entity interface {
    Path() (string, error)
    Create(name string) error
    Rename(string) error
    Remove() error
    Name() (string, error)
    Id() (int, error)
    Recorded() bool
}
```

### 9.2 UserEntity

封装 `database.UserEntity` 记录，提供：
- **路径管理**：`Path()` 基于 `parent_dir/name` 构造
- **生命周期**：`Create/Rename/Remove` 同时操作文件系统和数据库
- **时间跟踪**：`LatestReleaseTime / SetLatestReleaseTime / ClearLatestReleaseTime`
- **媒体统计**：`MediaCount / MediaCountValid`

---

## 十、任务管理系统 — `internal/api`

### 10.1 任务类型

```go
TaskTypeUserDownload       = "user_download"
TaskTypeListDownload       = "list_download"
TaskTypeFollowingDownload  = "following_download"
TaskTypeProfileDownload    = "profile_download"
TaskTypeMarkDownloaded     = "mark_downloaded"
TaskTypeJsonFileDownload   = "json_file_download"
TaskTypeJsonFolderDownload = "json_folder_download"
TaskTypeBatchDownload      = "batch_download"
TaskTypeListProfile        = "list_profile"
TaskTypeRetryAllFailed     = "retry_all_failed"
```

### 10.2 任务状态机

```
queued → running → completed
                → failed
                → cancelled
```
- 终态不可逆转
- 24 小时前的已完成任务会被后台清理 goroutine 自动删除
- 删除任务只能删除终态任务

### 10.3 TaskManager

- 内存管理所有任务（`map[string]*Task`）
- 读写锁保护并发访问
- 每次状态变化后重建只读快照 + 通过 EventBus 广播
- 每个 Task 持有独立的 `context.Context` 和 `CancelFunc`
- 生成 `task_<uuid>` 格式的任务 ID，保证创建时间单调递增

### 10.4 DownloadQueue

- 基于 `sync.Cond` 的生产者-消费者模式
- **target 锁机制**：同一用户同时只能有一个任务执行（`targetKey{scope, value}`）
- **detached 机制**：任务取消后，正在运行的 goroutine 有 10s 优雅退出时间；超时后以 detached 状态在后台继续执行
- 关闭时等待所有 worker 完成（最多 15s）

### 10.5 EventBus / SSE

- **EventBus**：发布/订阅模式，支持多个 SSE 订阅者
- **coalesced 事件**：`tasks`、`schedules` 等事件会合并（只保留最新状态，供定时轮询）
- **replayable 事件**：`notification`、`server_shutdown` 保留历史（最多 256 条），断线重连时回放
- **预序列化**：同一事件对所有订阅者共享一份 JSON 字节缓存
- **慢消费者保护**：队列超过 4096 条时自动关闭该订阅者
- SSE 心跳间隔：25s；写超时：10s

---

## 十一、计划任务系统 — `internal/scheduler`

### 11.1 调度类型

```go
ScheduleTypeList      = "list"      // 下载某个 list
ScheduleTypeUser      = "user"      // 下载某个用户
ScheduleTypeFollowing = "following" // 下载某人的关注
ScheduleTypeMixed     = "mixed"     // 混合下载（多个用户/列表/关注）
```

### 11.2 调度模式

```go
ScheduleModeInterval = "interval"   // 固定间隔（如 "1h30m"）
ScheduleModeDaily    = "daily"      // 每日指定时间（如 "08:00,20:00"）
```

### 11.3 配置存储

计划任务配置存储在 `schedules.yaml` 中，格式：

```yaml
schedules:
  - type: user
    target: elonmusk
    name: "Elon Musk"
    schedule: "daily 08:00,20:00"
    enabled: true
    run_on_start: false
  - type: list
    target: "1234567890"
    schedule: "interval 4h"
    enabled: true
```

### 11.4 调度执行

- `Scheduler` 后台 goroutine 定时检查
- 到期 → 调用 `DownloadFunc`（实际是 `server.scheduledDownload`）
- 创建任务 → 入 DownloadQueue → 按正常任务流执行
- 跟踪运行状态、连续失败次数、下次执行时间

---

## 十二、命名、路径、工具等支撑层

### 12.1 `internal/naming` — 文件命名规则

| 文件 | 内容 |
|------|------|
| `user_naming.go` | 用户目录命名：`screen_name` |
| `tweet_naming.go` | 推文文件命名：`{tweet_id}_{media_index}.{ext}`，支持 `UniquePathResolver` 去重 |
| `list_naming.go` | List 目录命名 |
| `base.go` | `MaxFileNameLen` 默认值 200 |

### 12.2 `internal/path` — 路径管理

`StorePath` 管理下载根目录下的所有子目录路径：

```go
type StorePath struct {
    Root           string  // 下载根目录
    Users          string  // root/users/
    Data           string  // root/.data/
    DB             string  // root/.data/foo.db
    ErrorsPath     string  // root/.data/errors.json
    JSONErrorsPath string  // root/.data/json_errors.json
}
```

### 12.3 `internal/utils` — 通用工具

| 文件 | 功能 |
|------|------|
| `algo.go` | 堆/优先队列实现 |
| `fs.go` | 文件系统工具 |
| `http.go` | HTTP 状态错误类型 |
| `recovery.go` | panic 恢复 |
| `stub.go` | 测试替身 |
| `time_range.go` | 时间范围 |
| `twitter_media.go` | Twitter 媒体 URL 处理 |
| `user.go` | ScreenName 规范化、校验、高质量图片 URL |
| `win32.go` | Windows 专属实现 |

---

## 十三、Web UI — `internal/api/web/`

纯静态前端，**无构建步骤**（无 npm/webpack/框架）。

| 文件 | 大小 | 说明 |
|------|------|------|
| `index.html` | ~15KB | 多页面 SPA 布局（任务/数据/计划/系统/日志页） |
| `app.js` | ~200KB | 全部前端逻辑 |
| `styles.css` | ~38KB | 样式表 |

**实现方式**：
- 通过 `fetch()` 调用 REST API
- 通过 `EventSource` 订阅 SSE 实时推送（任务状态、调度状态）
- 页面内路由：`/` → 下载任务页，`/tasks` → 任务列表，`/data` → 数据管理
- `/schedules` → 计划任务，`/system` → 系统设置，`/logs` → 日志查看

---

## 十四、配置管理 — `internal/config`

### 14.1 配置结构

```yaml
root_path: "D:/TwitterMedia"        # 下载根目录
cookie:                             # Twitter 登录凭据
  auth_token: "..."
  ct0: "..."
max_download_routine: 20            # 最大下载并发数（默认 CPU*10，上限 100）
max_file_name_len: 200              # 最大文件名长度（范围 50-245）
proxy_url: "http://127.0.0.1:7897"  # 代理（可选，支持 http/https/socks5）
```

### 14.2 配置加载优先级

1. 自动检测 `{appRootPath}/conf.yaml`
2. 不存在时走交互式配置（`PromptConfig`）
3. 支持 `TMD_ROOT_PATH / TMD_AUTH_TOKEN / TMD_CT0 / TMD_MAX_DOWNLOAD_ROUTINE / TMD_MAX_FILE_NAME_LEN / TMD_PROXY_URL` 环境变量覆盖
4. `-conf` 参数强制进入交互式配置

### 14.3 附加配置

- `additional_cookies.yaml`：额外 Twitter 账号列表（`[{auth_token, ct0}]`）
- `schedules.yaml`：计划任务配置

---

## 十五、Profile 下载 — `internal/downloading/profile/`

### 15.1 下载内容

每个用户目录下的 `.loongtweet/.profile/` 中保存：
- 头像（avatar）
- 横幅（banner）
- `profile.json`（用户简介、位置、URL、创建时间等元数据）

### 15.2 版本备份

旧文件自动备份到 `.loongtweet/.profile/.versions/` 目录。

### 15.3 实现

- `profile.NewProfileDownloaderWithDB(config, storage, clients, db, dwn, fileWriter)` 创建 Profile 下载器
- `DownloadMultiple(ctx, requests)` 批量下载多个用户的 Profile
- 使用 `FileStorageManager` 管理文件存储

---

## 十六、运行时数据流（完整示例）

### 16.1 完整的一次用户下载（API 模式）

```text
HTTP POST /api/v1/users/elonmusk/download
  Body: {"auto_follow": true, "skip_profile": false, "no_retry": false}

  → handleUserDownload(w, r, "elonmusk")
    → taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{...})
      → 生成 task_xxx-xxx 任务，状态 queued
    → buildTaskRunFunc(task) → 返回闭包
    → downloadQueue.Enqueue(task, runFunc)
      → 入 pending 队列
    → 返回 HTTP 202 { task_id: "task_xxx-xxx", status: "queued" }

  后台 goroutine worker 消费：

  → nextJob() → 持锁 targetKey{user, elonmusk}
    → UpdateTaskStatus("task_xxx-xxx", TaskStatusRunning)
    → 执行 runFunc(ctx, "task_xxx-xxx", reporter)

      → service.UserDownload(ctx, "task_xxx-xxx", "elonmusk", opts, reporter)
        → executeDownloadTemplate(...)
          → Prepare: twitter.GetUserByScreenName(ctx, client, "elonmusk")
            → GraphQL API: UserByScreenName
            → 返回 twitter.User{Id, ScreenName, MediaCount, ...}
          → downloading.BatchDownloadAny(ctx, client, db, [], [elonmusk],
              rootPath, usersPath, false, additionalClients, dwn, fw, opts, progress)
            → syncListAndGetMembers: 空（没有 list）
            → BatchUserDownload:
              1. syncUserAndEntity:
                 → database.SyncUser / database.LocateUserEntity
                 → 创建或更新用户目录
                 → 检查用户名变更，重命名目录
              2. 生产者:
                 → twitter.User.GetMedias → 翻页拉取时间线
                 → 推文通过 tweetChan 发给 worker
              3. 消费者 (多个 goroutine):
                 → tweetDownloader → downloadTweetMedia
                   → downloader.Download() 下载每个媒体文件
                   → fileWriter.Write() 原子写入
                   → saveLoongTweet / saveTweetJson 异步保存元数据
          → RetryFailedTweets: 如果有失败项则重试
          → downloadProfile: 下载头像/横幅/profile.json

          → reporter.OnComplete("task_xxx-xxx", Result{...})
            → SSEProgressReporter → taskManager.CompleteTask()
              → 更新状态为 completed, 设置结果
              → 通过 EventBus → SSE 推送 → 浏览器更新

    → releaseTargets("task_xxx-xxx") 释放锁
```

### 16.2 CLI 模式下差异

- 没有 TaskManager、DownloadQueue、SSE
- 直接调用 `service.*Download()` 方法
- 使用 `LogReporter` 输出进度到控制台
- 同步等待完成后进程退出

---

## 十七、重要设计文档

`doc/` 目录下有多份详细设计说明：

| 文档 | 内容 |
|------|------|
| `API_DOCUMENTATION.md` | 完整 REST API 规格说明 |
| `SERVICE_LAYER.md` | 服务层设计说明 |
| `foo.db 技术文档.md` | 数据库 schema 和字段详解 |
| `foo.db 跨平台迁移说明.md` | 数据库跨平台迁移指南 |
| `用户名变更处理机制.md` | 用户名变更自动重命名文件夹 |
| `mark-downloaded详解.md` | 标记已下载功能的详细行为说明 |
| `help.md` | CLI 帮助文本 |
| `user-accessible-status-changelog.md` | 用户可访问状态变更日志 |
| `call_chain_analysis_report.md` | 调用链分析报告 |
| `TMD 版本对比分析报告.md` | 版本演进对比报告 |

---

## 十八、并发模型总结

| 组件 | 并发模型 | 说明 |
|------|---------|------|
| DownloadQueue | 单 worker goroutine | `workerLoop` 逐条消费 pending 队列 |
| 推文生产者 | `ants.Pool` (max=35) | 并行拉取多个用户的推文 |
| 推文消费者 | N 个 goroutine | N = MaxDownloadRoutine，从 tweetChan 消费 |
| 文件写入 | 256 槽哈希锁 | 相同文件路径串行写 |
| List 同步 | goroutine per list | `sync.WaitGroup` 等待所有 list 完成 |
| SSE 订阅 | 每个连接一个 goroutine | 独立的事件消费循环 |
| Scheduler | 单 goroutine | 定时 ticker 检查到期任务 |
| 任务清理 | 单 goroutine | 每小时检查并清理 24h 前终态任务 |
| TweetDumper | `sync.Mutex` 保护 | `dumperMu` 在 service 层 |

---

## 十九、运行与验证

### 本地验证

```bash
go test ./...
go test -race -covermode atomic -coverprofile=covprofile ./...
go build -o tmd.exe .
go vet ./...               # 静态分析，建议改完就跑
```

### CI 持续集成

CI 配置在 `.github/workflows/go.yml`，当前仅由 tag 推送（`v*`）触发，负责构建发布：

```bash
# Windows
go build -o bin/tmd-${{ runner.os }}-amd64.exe -v -ldflags "-w -s" .

# Unix (CGO_ENABLED=0)
GOARCH=amd64 CGO_ENABLED=0 go build -o bin/tmd-${{ runner.os }}-amd64 -v -ldflags "-w -s" .
```

> ⚠️ **当前 gap：`go.yml` 没有 PR/push 触发的测试 job。** 如果你加了 CI 测试，记得创建一个 `test.yml` 跑 `go test -race ./...` 并在本文件更新说明。

---

## 二十、常见任务入口速查

| 任务 | CLI 入口 | API 入口 | Service 方法 | 下载业务层 |
|------|---------|---------|-------------|-----------|
| 下载用户 | `cli/executor.go:196` | `download_handlers.go:60` | `UserDownload` | `batch_download.go` |
| 下载列表 | `cli/executor.go:206` | `download_handlers.go` | `ListDownload` | `list_download.go` |
| 下载关注 | `cli/executor.go:199` | `download_handlers.go` | `FollowingDownload` | `batch_any.go` |
| 批量下载 | `cli/executor.go:206` | `download_handlers.go` | `BatchDownload` | `batch_any.go` |
| Profile 下载 | `cli/executor.go:228` | `download_handlers.go` | `ProfileDownload` | `profile/downloader.go` |
| 标记已下载 | `cli/executor.go:179` | `download_handlers.go` | `MarkDownloaded` | `mark_downloaded.go` |
| JSON 文件导入 | `cli/executor.go:165` | `download_handlers.go` | `JsonFileDownload` | `json_file_download.go` |
| JSON 文件夹导入 | `cli/executor.go:171` | `download_handlers.go` | `JsonFolderDownload` | `json_folder_download.go` |
| 重试失败 | — | `download_handlers.go:1081` | `RetryAllFailed` | `retry.go` |

---

## 二十一、工作原则

1. **先读代码，再下判断**。不要仅凭文件名或 README 推断行为。
2. **优先复用现有分层**：CLI/API 只做输入输出和任务编排，下载业务放在 `internal/service` 或更下层。
3. **改动要能直接追溯到用户请求**。不要顺手格式化、重构、删除无关代码。
4. **保持现有风格**。这个仓库大量代码是直接、显式、偏过程化的 Go，不要为了"优雅"引入复杂抽象。
5. **发现需求歧义时先说清楚歧义和可选方案**；简单、低风险的问题可以按最保守假设继续推进。
6. **不要写预设性扩展**。只解决当前问题，不提前设计未来可能用不到的配置、接口或插件点。
7. **CLI 和 Server 两条路径需要同时更新时，要同步检查**——API 类型、handler、task、SSE、Web UI 是否保持一致。
8. **数据库改动必须有 migration 和测试**。改 schema 后补数据库测试。
9. **文件/目录命名改动必须考虑旧数据兼容**。
10. **改并发、任务状态、SSE、数据库写入、下载器时，优先跑 `go test -race ...`**。

### 推荐变更工作流

1. **确认起点**：`git status` 工作区干净，开特性分支
2. **了解全貌**：`codegraph explore "<主题>"` + `codegraph impact "<核心符号>"` 评估影响范围
3. **按需阅读**：`codegraph node` 定位符号 → `read_file -o 行号 -l 60` 精确读取（文件 <300 行才全读）
4. **列出发现**：bug / 矛盾 / 可优化点，标注是否涉及 `internal/twitter` 等高风险区域
5. **确认方向**：除非单测已覆盖的纯增量，否则先向用户确认
6. **（bug 修复）先写复现测试**，再修
7. **最小改动**：改完即 `go build ./internal/xxx/...` 确保编译
8. **验证**：
   - `go vet ./...`
   - `go test ./pkg/...`
   - `codegraph affected` → 跑关联包测试
   - `grep` 确认无死代码残留
   - `git diff` 自审查
   - 查 `doc/` 目录是否需要同步更新
   - 确认 CLI + API Server 两条路径一致
9. **提交**：`<package>: <简洁描述>`，正文写做了什么 + 为什么

## 二十二、不要做的事

- 不要提交真实 `auth_token`、`ct0`、cookies、日志或用户下载数据。
- 不要修改或删除用户本地 `.tmd2`、下载目录、数据库文件、日志文件，除非用户明确要求。
- 不要绕过 `service.DownloadService` 在 API 和 CLI 分别实现同一份下载逻辑。
- 不要随意改 Twitter bearer、GraphQL endpoint 或 features，除非正在修相关问题。
- 不要把 server 长任务改成同步 HTTP 请求。
- 不要把不可重试媒体错误（403/404）放回重试队列。
- 不要无依据改启动脚本退出码语义；`start-server.bat` 依赖正常退出码为 0。
- 不要做仓库级大格式化。
