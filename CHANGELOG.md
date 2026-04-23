# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

***

## [2.14.2] - 2026-04-15

### Added

#### 新增分页功能

| 文件 | 功能 |
|------|------|
| `internal/api/pagination.go` | 分页参数解析和响应封装 |
| `internal/database/query.go` | 数据库查询构建器 |

**特性：**
- 支持 `page`、`pageSize`、`sortBy`、`sortOrder` 参数
- 自动计算 `offset` 和 `totalPages`
- 统一的分页响应格式

#### 数据库查询优化

| 文件 | 功能 |
|------|------|
| `internal/database/query.go` | 通用查询构建器 |

**支持操作：**
- `Count()` - 获取总数
- `Query()` - 分页查询
- `QueryWithJoin()` - 关联查询
- `BuildWhere()` - 动态条件构建

### Changed

#### API 功能增强

| 文件 | 变更 |
|------|------|
| `internal/api/db_handlers.go` | +797 行，完整的数据库管理 API |
| `internal/api/server.go` | 新增分页中间件 |
| `internal/api/types.go` | 扩展响应类型 |
| `internal/api/web/index.html` | +973 行，增强 Web UI |

**新增 API 端点：**
- `GET /api/v1/db/users` - 分页获取用户列表
- `GET /api/v1/db/lists` - 分页获取列表
- `GET /api/v1/db/entities` - 分页获取实体
- `GET /api/v1/db/links` - 获取用户链接
- `GET /api/v1/db/previous-names` - 获取历史名称

#### 数据库层优化

| 文件 | 变更 |
|------|------|
| `internal/database/model.go` | 优化模型定义 |
| `internal/database/schema.go` | 优化表结构 |
| `internal/database/user_link.go` | 优化链接查询 |
| `internal/database/db_test.go` | 更新测试 |
| `internal/database/user_sync_test.go` | 更新测试 |

#### 下载模块优化

| 文件 | 变更 |
|------|------|
| `internal/downloading/batch_download.go` | 优化批处理 |
| `internal/downloading/entity.go` | 优化实体处理 |
| `internal/downloading/list_sync.go` | 优化列表同步 |
| `internal/downloading/download_test.go` | 更新测试 |

#### 文档更新

| 文件 | 变更 |
|------|------|
| `doc/API_DOCUMENTATION.md` | +599 行，完整的 API 文档 |
| `readme.md` | 更新功能说明 |

### Stats

- **17 个文件变更**
- **+2354 行 / -218 行**
- **新增文件：** 2 个（`pagination.go`, `query.go`）

***

## [2.14.1] - 2026-04-15

### Added

#### 新增 Web 管理界面

| 文件 | 功能 |
|------|------|
| `internal/api/handlers.go` | Web 页面处理器（`handleWeb`, `handleStatic`） |
| `internal/api/web/index.html` | Web 管理界面（嵌入式） |

**Web 界面功能：**
- 任务管理：查看、创建、取消下载任务
- 数据浏览：查看数据库中的 Users、Lists、User Entities
- 系统配置：显示当前配置信息（脱敏）
- 实时状态：SSE 推送任务状态更新

#### 新增 SSE 实时推送

| 文件 | 功能 |
|------|------|
| `internal/api/sse.go` | Server-Sent Events 实现 |

**特性：**
- 每 2 秒推送一次任务列表更新
- 支持浏览器实时订阅
- 自动重连机制

#### 新增数据库管理 API

| 文件 | 功能 |
|------|------|
| `internal/api/db_handlers.go` | 数据库查询处理器 |

**API 端点：**
- `GET /api/v1/db/users` - 获取用户列表
- `GET /api/v1/db/lists` - 获取列表数据
- `GET /api/v1/db/entities` - 获取用户实体
- `GET /api/v1/db/config` - 获取配置信息（脱敏）

#### 新增中间件

| 文件 | 功能 |
|------|------|
| `internal/api/middleware.go` | CORS、日志、恢复中间件 |

### Changed

#### API 功能增强

| 文件 | 变更 |
|------|------|
| `internal/api/server.go` | 新增 Web 路由、静态文件服务 |
| `internal/api/async_executor.go` | 增强任务执行器，支持更多任务类型 |
| `internal/api/task_manager.go` | 优化任务管理 |
| `internal/api/types.go` | 扩展类型定义 |
| `internal/downloading/batch_download.go` | 优化批量下载错误处理 |

#### 文档更新

| 文件 | 变更 |
|------|------|
| `doc/API_DOCUMENTATION.md` | 完整重写（+261 行），包含所有新 API |
| `readme.md` | 更新 API Server 模式说明（+73 行） |

### Stats

- **12 个文件变更**
- **+480 行 / -62 行**
- **新增文件：** 5 个（`handlers.go`, `db_handlers.go`, `middleware.go`, `sse.go`, `web/index.html`）

***

## [2.14.0] - 2026-04-15

### Added

#### 新增 CLI 模块 (`internal/cli/`)

将 main.go 中的命令行处理逻辑抽离为独立模块：

| 文件 | 功能 |
|------|------|
| `args.go` | 命令行参数定义和解析（UserArgs, ListArgs, ProfileUserArgs, ProfileListArgs） |
| `executor.go` | CLI 命令执行器（Dependencies, ExecuteCLI） |
| `helpers.go` | 辅助函数（createClients, getUserByScreenName） |
| `paths.go` | 路径处理函数 |

**核心改进：**
- 解耦 main.go 中的业务逻辑
- 清晰的依赖注入结构（Dependencies）
- 支持可重复参数解析

#### 新增 API 异步执行器 (`internal/api/async_executor.go`)

| 文件 | 功能 |
|------|------|
| `async_executor.go` | 异步下载任务执行器 |

### Changed

#### main.go 重构

**精简 462 行代码**，职责更清晰：

| 变化 | 说明 |
|------|------|
| 移除 CLI 逻辑 | 迁移到 `internal/cli/` 包 |
| 简化 main 函数 | 只保留模式选择和启动逻辑 |
| 统一错误处理 | 使用 `log.Fatal` 替代多处错误判断 |

**新的 main.go 结构：**
```go
func main() {
    // 1. 解析命令行参数
    // 2. 初始化日志
    // 3. 检查工作目录
    // 4. 选择模式：API Server / CLI
    // 5. 启动对应模式
}
```

#### 依赖更新

| 文件 | 变更 |
|------|------|
| `go.mod` / `go.sum` | 新增依赖 |
| `internal/database/connect.go` | 添加 `InitDB` 函数 |
| `internal/downloader/downloader.go` | 修复流式下载 |
| `internal/downloading/tweet_download.go` | 优化错误处理 |
| `doc/API_DOCUMENTATION.md` | 更新文档 |

### Stats

- **9 个文件变更**
- **+120 行 / -462 行**
- **净减少：342 行代码**
- **新增模块：** 2 个（`internal/cli/`, `internal/api/async_executor.go`）

***

## [2.12.3] - 2026-04-15

### Added

#### 新增流式下载功能（大文件支持）

优化下载器以支持大文件下载，自动根据文件大小选择下载策略：

| 文件 | 变更 |
|------|------|
| `internal/downloader/downloader.go` | 新增流式下载逻辑 |
| `internal/downloader/types.go` | 新增 `Reader` 和 `Size` 字段，支持流式模式 |
| `internal/downloader/file_writer.go` | 新增流式写入支持 |
| `internal/downloader/downloader_test.go` | 新增流式下载测试（+327 行） |

**核心功能：**

1. **智能下载策略**
   - 小文件（< 10MB）：Buffer 模式（支持 SkipUnchanged）
   - 大文件（≥ 10MB）：流式模式（节省内存）

2. **流式下载特性**
   - HEAD 请求预获取文件大小
   - 带重试机制（最多 3 次，间隔 2 秒递增）
   - 文件大小验证（下载完成后校验）
   - 自动清理不完整文件
   - 失败后回退到 Buffer 模式

3. **文件写入增强**
   - 支持 `io.Reader` 流式写入
   - 实时进度跟踪
   - 大文件分块处理

**配置常量：**
```go
streamThreshold    = 10MB    // 流式下载阈值
maxDownloadRetries = 3       // 最大重试次数
retryDelay         = 2秒     // 重试间隔
```

### Changed

#### 依赖更新

- `internal/downloading/types.go` - 适配新的下载器接口

### Stats

- **5 个文件变更**
- **+637 行 / -4 行**
- **新增测试：** 327 行

***

## [2.12.2] - 2026-04-15

### Changed

#### 默认配置调整

| 配置项 | 旧值 | 新值 | 说明 |
|--------|------|------|------|
| `max_download_routine` | 3 | 35 | 默认下载并发数提高 |
| `max_file_name_len` | 155 | 158 | 与代码默认值保持一致 |
| 日志 `MaxSize` | 5 MB | 2 MB | 单日志文件大小限制 |
| 日志 `MaxAge` | 7 天 | 14 天 | 日志保留时间延长 |

**影响文件：**
- `internal/config/partial_update.go` - 更新默认值
- `main.go` - 更新日志配置

***

## [2.12.1] - 2026-04-15

### Added

#### 新增部分配置更新功能

新增 `PromptPartialConfig()` 函数，支持更新现有配置而不重新创建：

| 文件                                  | 变更                      |
| ----------------------------------- | ----------------------- |
| `internal/config/partial_update.go` | 新增部分配置更新功能              |
| `internal/config/config.go`         | 新增 `MaxFileNameLen` 配置项 |
| `main.go`                           | 使用 `-c` 参数时调用部分更新       |

**功能：**

- 支持更新的字段：root\_path, auth\_token, ct0, max\_download\_routine, max\_file\_name\_len
- 空输入表示保持原值不变

### Changed

#### 默认文件名长度调整

| 文件                        | 变更                                |
| ------------------------- | --------------------------------- |
| `internal/utils/fs.go`    | `DefaultMaxFileNameLen` 155 → 158 |
| `internal/naming/base.go` | 同步更新                              |

**说明：** 为后缀预留更多空间（5 → 8 字节）

#### .gitignore 更新

- 添加 `tmd-2.4.4/` 目录忽略

***

## \[2.12.0] - 2026-04-15

### Added

#### 新增部分失败推文重试功能

支持部分媒体下载失败的推文进入重试队列，而不是全部重新下载：

| 文件                                       | 变更                          |
| ---------------------------------------- | --------------------------- |
| `internal/downloading/tweet_download.go` | `downloadTweetMedia()` 函数增强 |
| `internal/downloading/retry.go`          | `RetryFailedTweets()` 函数增强  |

**核心改进：**

1. **智能 URL 跟踪**
   - 收集下载失败的 URL (`failedUrls`)
   - 收集下载成功的 URL (`successUrls`)
   - 更新 `tweet.Urls` 只保留失败的 URL
2. **部分失败处理**
   - 只要有失败的 URL，就返回错误让推文进入重试队列
   - 重试时只下载失败的媒体，不重复下载已成功的
3. **增强日志输出**
   - 显示下载进度：`[成功数/总数]`
   - 重试时显示：`retrying N tweets with M total media(s)`
   - 成功时显示：`tweet X all media downloaded successfully on retry`
   - 失败时显示：`tweet X still has N media(s) to download`
4. **空队列优化**
   - 如果没有需要重试的推文，直接返回并清空队列

### Changed

#### 文档更新

- `CLAUDE.md` - 更新 AI 编码准则
- `.gitignore` - 更新忽略规则

***

## \[2.11.2] - 2026-04-15

### Changed

#### 优化未关注用户统计逻辑

| 文件                                       | 变更          |
| ---------------------------------------- | ----------- |
| `internal/downloading/batch_download.go` | 优化未关注用户统计逻辑 |

**改进：**

- 只显示未关注且受保护的账户（这些账户无法下载内容）
- 未关注但公开的账户可以正常下载，不再显示警告
- 日志信息更新为："未关注且受保护的账户 (N，无法下载内容)"

***

## \[2.11.1] - 2026-04-15

### Added

#### 新增未关注用户统计功能

在批量下载前显示未关注的用户列表：

| 文件                                       | 变更               |
| ---------------------------------------- | ---------------- |
| `internal/downloading/batch_download.go` | 新增未关注用户统计和日志输出   |
| `internal/downloading/batch_any.go`      | 简化 autoFollow 逻辑 |

**功能：**

- 下载开始前自动统计未关注用户数量
- 显示未关注用户的名称和用户名
- 简化 `autoFollow` 参数传递逻辑

### Changed

#### 文档更新

- `readme.md` - 优化文档结构和内容

***

## \[2.11.0] - 2026-04-15

### Added

#### 新增 CLAUDE.md - AI 编码准则

用于减少常见 LLM 编码错误的行为准则文档，包含：

- **编码前先思考** - 明确假设、呈现权衡、提出异议
- **简单优先** - 只写最少代码，不做预设性扩展
- **外科手术式修改** - 只改必须改的内容
- **目标驱动执行** - 先定义成功标准，再循环推进

### Changed

#### 大规模代码精简与优化

**核心原则：** 删除冗余代码，简化实现，遵循 "简单优先"

| 模块                      | 变化                  | 行数变化   |
| ----------------------- | ------------------- | ------ |
| `internal/downloader/`  | 删除冗余测试和未使用功能        | -353 行 |
| `internal/downloading/` | 简化 JSON 下载、列表下载、批处理 | -260 行 |
| `internal/profile/`     | 精简下载器、获取器、存储层       | -175 行 |
| `internal/twitter/`     | 简化推文和用户处理           | -22 行  |
| `internal/utils/`       | 删除冗余工具函数            | -148 行 |
| `internal/database/`    | 精简测试和列表操作           | -54 行  |
| `main.go`               | 简化主逻辑               | -110 行 |

**具体改进：**

- 删除 `internal/downloading/user_download.go`（功能合并）
- 简化 `json_download.go` - 减少 233 行，优化错误处理
- 精简 `downloader_test.go` - 删除 296 行冗余测试
- 优化 `file_writer.go` - 简化文件写入逻辑
- 清理 `utils/fs.go` - 删除 26 行未使用函数
- 移除 `utils/recovery.go` - 删除 34 行 panic 恢复代码

### Stats

- **29 个文件变更**
- **+173 行 / -1146 行**
- **净减少：973 行代码**
- **新增文件：** 1 个（`CLAUDE.md`）
- **删除文件：** 1 个（`internal/downloading/user_download.go`）

***

## \[2.10.0] - 2026-04-15

### Added

#### 新增数据库连接模块 (`internal/database/connect.go`)

统一的数据库连接管理：

- `Connect(path)` - 连接数据库并自动执行迁移
- 支持 WAL 模式 (`_journal_mode=WAL`)
- 自动创建表结构和迁移

#### 新增 Downloader 辅助函数 (`internal/downloader/helpers.go`)

- `ExtractImageExtFromURL(url)` - 从 URL 提取图片扩展名
- 支持 `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`
- 无效扩展名默认返回 `.jpg`

#### 命名服务重构 (`internal/naming/`)

将原来的 `naming.go` 拆分为职责更清晰的文件：

| 文件                | 功能                         |
| ----------------- | -------------------------- |
| `base.go`         | 基础命名结构，定义 `MaxFileNameLen` |
| `tweet_naming.go` | 推文命名（`TweetNaming`）        |
| `user_naming.go`  | 用户命名（`UserNaming`）         |
| `list_naming.go`  | 列表命名（`ListNaming`）         |

### Changed

#### 架构优化

| 模块                      | 变化                             |
| ----------------------- | ------------------------------ |
| `internal/downloader/`  | 增强测试覆盖（+358 行测试代码），优化文件写入和版本管理 |
| `internal/downloading/` | 优化实体处理、批处理逻辑，改进重试机制            |
| `internal/profile/`     | 简化下载器、获取器、存储层，减少重复代码           |
| `internal/twitter/`     | 优化客户端、列表、用户处理逻辑                |
| `internal/utils/`       | 清理冗余代码，优化算法实现                  |
| `main.go`               | 精简 43 行，使用新的数据库连接模块            |

#### .gitignore 完善

新增大量忽略规则：

- IDE/编辑器文件（.vscode/, .idea/）
- Go 构建产物（\*.exe, \*.test, bin/）
- 数据库文件（\*.db, \*.sqlite）
- 日志文件（\*.log, logs/）
- 环境配置（.env）
- 临时文件（tmp/, temp/）

### Stats

- **32 个文件变更**
- **+827 行 / -590 行**
- **新增文件：** 7 个
- **删除文件：** 1 个（`internal/naming/naming.go`）

***

## \[2.9.2] - 2026-04-15

### Fixed

#### 修复多媒体文件命名冲突

修复同一推文包含多个媒体文件时的文件名冲突问题：

| 文件                                       | 变更                        |
| ---------------------------------------- | ------------------------- |
| `internal/downloading/tweet_download.go` | `downloadTweetMedia()` 函数 |

**问题：** 同一推文的多个媒体文件使用相同的文件名，导致后下载的文件覆盖先下载的文件。

**修复：** 采用互斥锁保护文件名生成，使用 `utils.UniquePath` 自动处理文件名冲突：

- 首次下载：`{text}_{tweet_id}.jpg`, `{text}_{tweet_id}(1).jpg`, `{text}_{tweet_id}(2).jpg`...
- 文件已存在时：自动继续序号，避免覆盖

***

## \[2.9.0] - 2026-04-15

### Added

#### 新增 `internal/config/` 包 - 配置管理模块

将配置逻辑从 `main.go` 中抽离，独立管理：

| 文件          | 功能              |
| ----------- | --------------- |
| `config.go` | 配置结构定义、读写、交互式引导 |

**核心功能：**

- `ReadConf()` / `WriteConf()` - YAML 配置文件读写
- `PromptConfig()` - 交互式配置引导（支持默认值、自动备份）
- `ReadAdditionalCookies()` - 多账号 Cookie 读取
- 配置损坏时自动备份并重新创建

#### 数据库层拆分 (`internal/database/`)

将原来的单体 `crud.go`（496行）按职责拆分为多个文件：

| 文件                         | 功能                                             |
| -------------------------- | ---------------------------------------------- |
| `schema.go`                | 数据库表结构与迁移                                      |
| `helpers.go`               | 通用数据库辅助函数（handleGetResult, handleInsertWithId） |
| `user.go`                  | 用户表 CRUD 操作 + 用户可访问状态管理                        |
| `user_entity.go`           | 用户实体操作（CRUD + 最新发布时间）                          |
| `lst.go` / `lst_entity.go` | 列表及列表实体操作                                      |
| `user_link.go`             | 用户符号链接（symlink）管理                              |
| `user_sync.go`             | 共享用户同步逻辑（`SyncUser()`）                         |
| `user_sync_test.go`        | 同步功能测试（5 个用例）                                  |

#### 下载模块拆分 (`internal/downloading/`)

将原来的单体 `features.go`（1226行）按职责拆分为多个文件：

| 文件                   | 功能                                                        |
| -------------------- | --------------------------------------------------------- |
| `types.go`           | 类型定义（PackagedTweet, TweetInEntity, workerConfig 等）+ 全局状态  |
| `tweet_download.go`  | 单条推文下载、JSON/LoongTweet 保存、媒体清理                            |
| `user_sync.go`       | 下载过程中的用户同步（syncUser, syncUserAndEntity, shouldIgnoreUser） |
| `user_download.go`   | 单用户推文获取与预处理                                               |
| `batch_download.go`  | 批量用户下载核心逻辑（优先级队列、并发控制、ants 池）                             |
| `list_download.go`   | 列表下载流程（syncList, syncListAndGetMembers）                   |
| `batch_any.go`       | 通用批量下载入口                                                  |
| `mark_downloaded.go` | 标记用户为已下载（支持时间戳、全量重置、JSON 结果输出）                            |
| `retry.go`           | 失败推文重试机制（RetryFailedTweets）                               |

**批量下载架构：**

```
BatchDownloadAny()
    ├── syncLstAndGetMembers() - 同步列表成员
    ├── BatchUserDownload()
    │   ├── 预处理阶段：用户排序、symlink 创建、深度计算
    │   ├── 生产者池（ants goroutine pool）：并发获取用户推文
    │   ├── 消费者池（MaxDownloadRoutine）：并发下载媒体
    │   └── 错误收集与重试
    └── MarkUsersAsDownloaded() - 标记已下载
```

#### 新增 `internal/twitter/batch_login.go` - 批量登录

多账号并发登录：

```go
func BatchLogin(ctx context.Context, dbg bool, cookies []AccountCookie, master string) []*resty.Client
```

**特性：**

- 并发登录所有账号
- 自动去重（相同 screen\_name 只保留一个）
- 主账号优先保证
- 支持调试模式（请求计数）

#### Profile 模块 API 去重

| 文件                                 | 变化                                                                                                                                                        |
| ---------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/profile/fetcher.go`      | 精简 \~145 行，删除重复的 `userByScreenName` API 定义和 `makeProfileUrl()`，改用 `twitter.GetUserByScreenName()` + `userToProfileInfo()` 转换；新增 `errors.As` 类型解包修复客户端错误处理 |
| `internal/profile/fetcher_test.go` | 新增测试用例（userToProfileInfo 转换 + GetHighResAvatarURL 各质量参数）                                                                                                  |

### Changed

#### 架构重构 - 单体文件拆分与去重

**删除的文件：**

- `internal/database/crud.go` → 拆分为 8 个职责单一的文件
- `internal/downloading/features.go` → 拆分为 9 个职责单一的文件

**代码统计：**

- 删除 \~1722 行（两个巨型单体文件）
- 新增 \~18 个模块化文件
- `main.go` 精简，配置/登录/重试逻辑分别迁移到 config/twitter/downloading 包

#### 命名规范化

全局修正 5 处拼写/命名不一致（编译器级安全替换）：

| 旧名称                    | 新名称                     | 影响范围                                                                       |
| ---------------------- | ----------------------- | -------------------------------------------------------------------------- |
| `GetMeidas`            | `GetMedias`             | twitter/user.go, downloading/\*, twitter\_test.go                          |
| `PackgedTweet`         | `PackagedTweet`         | downloading/\*, main.go, json\_download.go                                 |
| `JsonPackgedTweet`     | `JsonPackagedTweet`     | json\_download.go                                                          |
| `shouldIngoreUser`     | `shouldIgnoreUser`      | downloading/user\_sync.go, batch\_download.go                              |
| `userInLstEntity`      | `userInListEntity`      | downloading/types.go, batch\_download.go, list\_download.go, batch\_any.go |
| `syncLstAndGetMembers` | `syncListAndGetMembers` | list\_download.go, batch\_any.go                                           |

#### 逻辑统一

| 变化                                       | 说明                                        |
| ---------------------------------------- | ----------------------------------------- |
| `database.SyncUser()`                    | 提取共享用户同步逻辑，downloading 和 profile 包复用同一函数  |
| `profile/downloader.syncUserDirectory()` | 简化为调用 `database.SyncUser()`，消除 \~35 行重复代码 |
| `downloading/user_sync.syncUser()`       | 简化为调用 `database.SyncUser()` 单行委托          |

#### 错误处理改进

| 文件                              | 变化                                                                                             |
| ------------------------------- | ---------------------------------------------------------------------------------------------- |
| `profile/fetcher.go`            | `handleClientError` 从类型断言改为 `errors.As`，修复因 `GetUserByScreenName` 包装 error 导致客户端限制错误无法被正确识别的问题 |
| `downloading/batch_download.go` | TwitterApiError 类型断言同样改为 `errors.As`，防御性编程                                                     |
| `database/user_sync.go`         | `SyncUser` 中 `RecordUserPreviousName` 错误现在向上传播而非仅记录日志，保持与原始行为一致                                |
| `downloading/types.go`          | `TweetInEntity.GetPath()` 移除不必要的裸 `recover()`，改为直接返回空字符串                                       |
| `downloading/user_download.go`  | 修正 "skiped" → "skipped" 拼写                                                                     |

#### 其他修改

| 文件                                 | 变化                                                                 |
| ---------------------------------- | ------------------------------------------------------------------ |
| `main.go`                          | 使用 config 包；使用 twitter.BatchLogin；使用 downloading.RetryFailedTweets |
| `internal/twitter/user.go`         | `GetMeidas` → `GetMedias`                                          |
| `internal/twitter/twitter_test.go` | 测试用例适配新方法名                                                         |
| `go.mod` / `go.sum`                | 依赖更新（testify）                                                      |

### Fixed

- 修复 `profile/fetcher.go` 中 `handleClientError` 因 error 包装导致类型断言永远为 false 的 bug
- 修复 `downloading/batch_download.go` 中同类型的 TwitterApiError 类型断言隐患
- 修复 `database/SyncUser` 吞掉 `RecordUserPreviousName` 错误导致调用方丢失重命名历史失败信息的问题
- 修复 `TweetInEntity.GetPath()` 中不必要的裸 `recover()` 调用
- 修正 `shouldIngoreUser` / `PackgedTweet` / `GetMeidas` / `userInLstEntity` / `syncLstAndGetMembers` 共 6 处拼写/命名错误

***

## \[2.8.0] - 2026-04-12

### Added

#### 新增用户可访问状态记录功能（扩展）

新增批量标记用户为可访问的方法：

| 文件                          | 变更                                                             |
| --------------------------- | -------------------------------------------------------------- |
| `internal/database/crud.go` | 新增 `SetUsersAccessible()` 和 `MarkListMembersAccessibleByIDs()` |
| `internal/utils/user.go`    | 新增通用 ID 提取函数 `ExtractIDs()`                                    |

**核心功能：**

- `SetUsersAccessible()` - 批量标记用户为可访问状态
- `MarkListMembersAccessibleByIDs()` - 异步标记列表成员为可访问
- `ExtractIDs()` - 通用 ID 提取函数，使用泛型

**调用位置：**

- `internal/downloading/features.go`: `downloadList()`, `syncLstAndGetMembers()`
- `main.go`: `handleProfileDownload()`

#### 新增用户可访问状态记录功能

在 `users` 表中新增 `is_accessible` 字段，用于记录 Twitter 用户是否可通过 API 正常访问（非封禁/注销状态）：

| 文件                             | 变更                             |
| ------------------------------ | ------------------------------ |
| `internal/database/model.go`   | 新增 `IsAccessible` 字段           |
| `internal/database/crud.go`    | 新增 `UpdateUserAccessible()` 方法 |
| `internal/database/db_test.go` | 新增测试用例                         |

**核心功能：**

- 区分可访问/不可访问用户：识别 Twitter API 返回的 `UserUnavailable` 类型
- 自动更新：每次获取列表成员时同步更新数据库中的访问状态
- 向后兼容：对已有 `foo.db` 数据库无破坏性影响

**调用链路：**

```
main.go (handleProfileDownload)
    └── lst.GetMembers()
        └── downloading/features.go
            ├── downloadList()
            ├── syncLstAndGetMembers()
            └── MarkUsersAsDownloaded()
```

#### 新增文档

- `doc/user-accessible-status-changelog.md` - 用户可访问状态记录功能说明

### Changed

#### 数据库层 (`internal/database/`)：

- `crud.go` - 新增用户可访问状态更新方法，扩展错误处理

#### 下载层 (`internal/downloading/`)：

- `features.go` - 集成用户可访问状态检测逻辑

#### Twitter API 层 (`internal/twitter/`)：

- `list.go` - 列表成员获取逻辑优化
- `tweet.go` - 推文处理优化
- `user.go` - 用户数据处理优化
- `twitter_test.go` - 测试用例更新

#### 主程序 (`main.go`)：

- 优化配置和错误处理
- 集成用户可访问状态功能

***

## \[2.7.0] - 2026-04-12

### Added

#### 新增 `internal/entity/` 包 - 实体类型定义

将分散在各处的实体类型集中管理：

| 文件             | 功能     |
| -------------- | ------ |
| `interface.go` | 实体接口定义 |
| `list.go`      | 列表相关实体 |
| `sync.go`      | 同步相关实体 |
| `user.go`      | 用户相关实体 |

#### 新增文档

- `doc/用户名变更处理机制.md` - 用户名变更处理机制说明

### Changed

#### 代码重构与优化

**数据库层 (`internal/database/`)：**

- `crud.go` - 重构 CRUD 操作，优化错误处理
- `db_test.go` - 补充测试用例
- `model.go` - 模型定义优化

**下载层 (`internal/downloading/`)：**

- `dumper.go` - 优化文件转储逻辑
- `entity.go` - 移除冗余代码（-256行）
- `features.go` - 重构下载特性
- `json_download.go` - JSON下载优化

**命名服务 (`internal/naming/`)：**

- `naming.go` - 优化命名逻辑
- `naming_test.go` - 测试用例更新

**Twitter API层 (`internal/twitter/`)：**

- `list.go` - 列表功能优化
- `timeline.go` - 时间线处理优化
- `tweet.go` - 推文处理优化
- `user.go` - 用户数据处理优化

**其他：**

- `main.go` - 主程序优化
- `internal/profile/downloader.go` - 下载器优化
- `internal/profile/storage.go` - 存储层优化

### Removed

- `internal/downloading/entity.go` - 实体类型迁移到 `internal/entity/` 包

***

## \[2.6.0] - 2026-04-12

### Added

#### 新增 `internal/downloader/` 包 - 通用下载基础设施

将下载逻辑从业务代码中抽离，提供可复用的下载能力：

| 文件                   | 行数  | 功能                                           |
| -------------------- | --- | -------------------------------------------- |
| `types.go`           | 75  | 接口定义（Downloader, FileWriter, VersionManager） |
| `downloader.go`      | 118 | HTTP下载实现，支持批量下载和上下文取消                        |
| `file_writer.go`     | 145 | 原子写入、MD5去重、版本管理                              |
| `version_manager.go` | 95  | 文件版本备份管理                                     |

**特性：**

- **原子写入**：先写临时文件，再重命名，确保数据完整性
- **MD5 去重**：相同内容自动跳过写入
- **并发安全**：使用 `sync.Mutex` 保护并发写入
- **版本管理**：文件变更时自动备份历史版本

#### 新增 `internal/naming/` 包 - 统一命名服务

集中管理推文和用户的文件命名逻辑：

| 类型                    | 功能                              |
| --------------------- | ------------------------------- |
| `TweetNaming`         | 推文文件名生成，支持日志格式、文件名、文件路径         |
| `UserNaming`          | 用户目录命名，生成 `Name(ScreenName)` 格式 |
| `SetMaxFileNameLen()` | 统一配置文件名长度限制                     |

**特性：**

- 缓存清理后的文本，避免重复计算
- 日志格式与文件名前缀一致
- 单一配置入口，无需手动同步

#### 新增 `internal/utils/recovery.go` - Panic 恢复工具

统一的 panic 恢复机制：

```go
defer utils.RecoverWithLog("functionName")
```

#### 新增 `internal/downloading/json_download.go` - JSON 下载功能

支持从 JSON 文件批量下载推文媒体：

| 函数                        | 功能              |
| ------------------------- | --------------- |
| `BatchDownloadFromJson()` | 从 JSON 批量下载     |
| `DownloadJsonDir()`       | 下载目录下所有 JSON 文件 |

### Changed

#### 架构重构

**依赖注入模式：**

- `downloader.Downloader` 接口注入到业务层
- `main.go` 统一创建和注入依赖
- 支持测试时 Mock

**分层架构：**

```
main.go (应用层)
    └── downloading/profile (业务层)
            └── downloader (基础设施层)
                    └── file_writer, version_manager
```

#### `internal/downloading/features.go` 重构

| 变化                     | 说明                            |
| ---------------------- | ----------------------------- |
| `downloadTweetMedia()` | 使用 `downloader.Downloader` 接口 |
| `BatchDownloadTweet()` | 新增 `dwn` 参数                   |
| `saveLoongTweet()`     | 统一数据来源（RawJSON 优先）            |
| `saveTweetJson()`      | 使用 `naming.TweetNaming`       |

#### `internal/profile/downloader.go` 重构

| 变化                    | 说明                         |
| --------------------- | -------------------------- |
| 构造函数                  | 新增 `dwn` 和 `fw` 参数         |
| `downloadAvatar()`    | 使用 `downloader.Downloader` |
| `downloadBanner()`    | 使用 `downloader.Downloader` |
| `saveDescription()`   | 使用 `downloader.FileWriter` |
| `ensureProfileDirs()` | 提取目录创建逻辑                   |

#### `internal/utils/fs.go` 修改

| 变化                           | 说明                              |
| ---------------------------- | ------------------------------- |
| 移除 `TweetFileName()`         | 使用 `naming.TweetNaming` 替代      |
| 移除 `MaxFileNameLen` 变量       | 使用 `naming.SetMaxFileNameLen()` |
| 新增 `WinFileNameWithMaxLen()` | 支持自定义长度限制                       |

#### `internal/profile/storage.go` 简化

| 变化                  | 说明                 |
| ------------------- | ------------------ |
| `EnsureDirectory()` | 移除 `screenName` 参数 |
| `GetFilePath()`     | 移除 `screenName` 参数 |

### Fixed

- 修复 `saveLoongTweet` 中 `tweet.Creator` 为 nil 时的 panic
- 修复 `MaxFileNameLen` 双变量同步问题
- 修复循环依赖风险（naming 包不再直接依赖 utils 变量）

### Stats

- **新增文件**: 6 个
- **修改文件**: 8 个
- **+1,200 lines / -300 lines**

***

## \[2.5.0] - 2026-04-04

### Added

#### Profile 下载功能

完整的用户资料下载系统，支持批量下载和版本管理：

**下载内容：**

- `avatar.jpg/png/gif/webp` - 高清头像 (默认 400x400)
- `banner.jpg/png/gif/webp` - 个人主页横幅
- `description.txt` - 用户简介纯文本
- `profile.json` - 完整资料 JSON

**新特性：**

- **去重下载**：基于 MD5 校验，profile文件未变更时自动跳过
- **版本管理**：资料变更时自动备份到 `.versions/` 目录
- **批量下载**：支持并发下载多个用户资料
- **智能复用**：从推文下载中复用已获取的用户数据，避免重复 API 调用

**存储结构：**

```
users/{UserName(screenName)}/.loongtweet/.profile/
├── avatar.jpg           # 当前头像
├── banner.jpg           # 当前横幅
├── description.txt      # 当前简介
├── profile.json         # 当前资料
└── .versions/          # 历史版本备份
    ├── avatar_20240115_103045.jpg
    └── profile_20240115_103045.json
```

**新增模块** **`internal/profile/`：**

- `downloader.go` (558 行) - Profile 下载器，支持单用户/批量下载
- `fetcher.go` (257 行) - Twitter API 获取器
- `storage.go` (183 行) - 文件存储管理器，支持版本管理
- `types.go` (158 行) - 类型定义和接口

#### 推文 JSON 保存

- 推文完整信息保存为格式化 JSON 到 `.loongtweet/` 目录
- 即使下载失败也能记录推文信息，便于调试
- 使用 `TweetFileName()` 生成一致的文件名

#### 命令行参数扩展

| 参数                 | 类型     | 说明                              |
| ------------------ | ------ | ------------------------------- |
| `--profile`        | bool   | 推文下载时同时下载用户资料（默认开启）             |
| `-noprofile`       | bool   | 跳过 Profile 下载                   |
| `-profile-user`    | string | 单独指定下载 Profile 的用户（可重复）         |
| `-profile-list`    | uint64 | 下载指定列表所有成员的 Profile（可重复）        |
| `-mark-downloaded` | bool   | 仅标记用户为已下载，不下载内容                 |
| `-mark-time`       | string | 指定标记时间戳（格式：2006-01-02T15:04:05） |

#### Twitter 客户端增强

**代理支持改进：**

- 支持 `HTTPS_PROXY` 环境变量（优先）
- 支持 `HTTP_PROXY` 环境变量（备用）
- 自动适配 Windows/Linux/macOS

**重试机制增强：**

- 网络错误（connection reset, broken pipe, timeout）自动重试
- Twitter API 内部错误（130, 0, -1）自动重试
- HTTP 5xx 服务器错误自动重试
- HTTP 429 速率限制自动等待

**客户端选择策略：**

- `SelectProfileClient()` - Profile 下载专用客户端选择
- `SelectClientMFQ()` - MFQ（多级反馈队列）客户端选择算法
  - 优先使用备用账号（非受保护用户）
  - 受保护用户专用主账号
  - 自动跳过有限制的客户端

#### 文件工具函数

- `TweetFileName(text, tweetId, ext)` - 生成统一的推文文件名
- `CopyFile(src, dst)` - 文件复制工具
- `MaxFileNameLen` - 可配置的文件名长度限制（默认 155，范围 50-250）
- `WinFileName()` - Windows 文件名清理（移除非法字符）

#### 依赖更新

**新增依赖：**

- `github.com/tidwall/gjson v1.17.3` - JSON 快速解析（Profile 获取）
- `github.com/natefinch/lumberjack v2.0.0` - 日志文件轮转

**现有依赖更新：**

- `github.com/mattn/go-sqlite3 v1.14.22`
- `github.com/go-resty/resty/v2 v2.14.0`
- `gopkg.in/yaml.v3 v3.0.1`

### Changed

#### main.go 重构 (+340 行)

- 重新设计命令行参数结构，支持可重复参数
- 添加 Profile 下载完整流程
- 改进配置引导程序，支持保留现有配置
- 优化信号处理，支持优雅退出
- 添加 Profile 下载结果输出格式

#### `internal/twitter/client.go` 重构 (+163 行)

- 重构 `Login()` 函数，增强错误处理
- 改进速率限制器日志输出
- 添加多个客户端选择算法

#### `internal/downloading/features.go` 重构 (+485 行)

- 添加推文 JSON 保存功能
- 重构下载流程错误处理
- 优化并发下载控制

#### `internal/utils/fs.go` 扩展

- 添加 `TweetFileName()` 函数
- 添加 `CopyFile()` 函数
- `MaxFileNameLen` 改为可配置变量

#### README.md 完整重写 (+460 行)

- 重新组织文档结构，添加完整目录
- 新增功能特性详解
- 新增安装与配置指南
- 新增命令行参数详解（表格形式）
- 新增 Profile 下载功能说明
- 新增文件存储结构图示
- 新增 9 个使用场景与示例
- 新增高级设置说明
- 新增参数兼容性速查表
- 新增常见问题解答 (FAQ)
- 新增输出结果格式说明

### Fixed

- 修复文件名过长导致 Windows 保存失败的问题
- 修复代理环境变量在 Windows 上的兼容性问题
- 修复并发下载时的竞态条件
- 修复数据库连接池问题

### Stats

- **23 files changed**
- **+4,554 lines / -240 lines**

***

## \[0.x.x] - Previous Versions

历史版本记录请参考 Git 提交历史:

```bash
git log --oneline
```

