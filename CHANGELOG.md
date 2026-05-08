# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

***

## [3.3.8] - 2026-05-04

### Added

#### StringUint64 类型解决精度丢失问题

| 文件 | 变更 |
|------|------|
| `internal/api/types.go` | 新增 `StringUint64` 类型，JSON 中以字符串形式传输 64 位整数 |
| `internal/api/types_test.go` | 新增 `TestStringUint64JSON` 测试用例 |
| `internal/api/download_handlers.go` | List ID 参数解析使用 `StringUint64` |
| `internal/api/task_manager.go` | 任务数据克隆使用 `StringUint64` |
| `internal/api/task_manager_test.go` | 测试用例更新为 `StringUint64` |

#### 快速下载支持剪切板读取

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | `handleQuickDownload` 支持自动读取剪切板内容 |
| `internal/api/web/app.js` | 按钮文案改为"粘贴并创建任务" |

#### List ID 输入验证

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 新增 `readListIDsFromTextarea` 函数验证 List ID 格式 |

### Changed

#### Web 界面优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/index.html` | 导航栏调整："定时任务"移到"数据管理"前面 |
| `internal/api/web/app.js` | 定时任务状态标签支持点击切换启用/禁用 |
| `internal/api/web/app.js` | 移除任务列表复选框 |
| `internal/api/web/app.js` | 调度器提示文案更新 |
| `internal/api/web/styles.css` | 移除 `.task-checkbox` 样式 |

#### 文档更新

| 文件 | 变更 |
|------|------|
| `doc/API_DOCUMENTATION.md` | 说明 List ID 使用十进制字符串传输 |
| `readme.md` | 新增 API JSON 传输说明 |

### Stats

- **11 个文件变更**
- **+118 行 / -80 行**

***

## [3.3.7] - 2026-05-04

### Added

#### 新增 `-follow-members` 参数

| 文件 | 变更 |
|------|------|
| `internal/cli/args.go` | 新增 `-follow-members` CLI 参数 |
| `internal/service/download_service.go` | 实现 `followMembersIfNeeded` 方法，下载时关注目标/成员 |
| `internal/service/interfaces.go` | `DownloadOptions` 新增 `FollowMembers` 字段 |
| `internal/twitter/user.go` | `FollowUser` 增加状态码和 API 响应检查 |
| `internal/api/types.go` | API 类型新增 `FollowMembers` 字段 |
| `internal/api/download_handlers.go` | 下载处理器支持 `follow_members` 参数 |
| `internal/scheduler/types.go` | 定时任务配置新增 `follow_members` 字段 |
| `internal/scheduler/scheduler.go` | 调度器支持 `follow_members` 配置 |
| `internal/api/web/app.js` | Web 界面定时任务表单新增 `follow_members` 选项 |

#### 功能说明

- `-follow-members`：下载时关注目标/成员（用户/列表成员/关注列表成员）
- 与 `-auto-follow` 区分：
  - `-auto-follow`：仅关注受保护且未关注的用户
  - `-follow-members`：关注所有未关注的用户（不限是否受保护）
- 关注失败仅输出 warning，不阻塞下载流程

### Changed

#### 文档更新

| 文件 | 变更 |
|------|------|
| `readme.md` | 新增 `-follow-members` 参数说明和语义区别 |
| `doc/API_DOCUMENTATION.md` | API 文档新增 `follow_members` 字段说明 |

### Stats

- **17 个文件变更**
- **+364 行 / -156 行**

***

## [3.3.6] - 2026-05-04

### Added

#### 自动配置引导

| 文件 | 变更 |
|------|------|
| `main.go` | 配置文件不存在时自动进入配置引导模式，不再报错退出 |

### Changed

#### 配置提示优化

| 文件 | 变更 |
|------|------|
| `internal/config/config.go` | 配置引导时显示默认值，提示信息输出到 stderr |

#### 日志系统优化

| 文件 | 变更 |
|------|------|
| `internal/consolelog/hub.go` | 改用缓冲区读取代替逐行扫描，提高日志捕获性能 |

### Stats

- **3 个文件变更**
- **+33 行 / -10 行**

***

## [3.3.5] - 2026-05-04

### Changed

#### Web 界面数据管理优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/styles.css` | 数据表格样式优化（表头固定、斑马纹、排序图标样式） |
| `internal/api/web/styles.css` | 新增表格滚动容器（table-scroll-container） |
| `internal/api/web/styles.css` | 移动端卡片视图优化 |
| `internal/api/web/app.js` | 分页默认每页 200 条（从 20 条调整） |
| `internal/api/web/app.js` | 数据表格排序功能优化 |

#### 定时任务优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | "启动后立即运行"改为"首次启动时立即运行" |
| `internal/api/web/app.js` | 定时任务保存逻辑优化（批量创建/更新/删除） |

#### 调度器修复

| 文件 | 修复 |
|------|------|
| `internal/scheduler/scheduler.go` | 修复 `run_on_start` 只在系统首次启动时执行 |
| `internal/scheduler/scheduler.go` | 新增 `firstStart` 和 `hasEverStarted` 标志位 |

#### 测试增强

| 文件 | 新增测试 |
|------|----------|
| `internal/scheduler/scheduler_test.go` | `TestReloadDoesNotTriggerRunOnStart` - 验证重载不触发 run_on_start |
| `internal/scheduler/scheduler_test.go` | `TestStopStartDoesNotTriggerRunOnStart` - 验证停止后启动不触发 run_on_start |

#### 文档更新

| 文件 | 变更 |
|------|------|
| `doc/API_DOCUMENTATION.md` | 更新 `run_on_start` 字段描述 |
| `readme.md` | 更新 `run_on_start` 字段描述 |

### Stats

- **7 个文件变更**
- **+416 行 / -135 行**

***

## [3.3.4] - 2026-05-04

### Fixed

#### 定时任务 run_on_start 逻辑修复

| 文件 | 修复 |
|------|------|
| `internal/scheduler/scheduler.go` | 修复 `run_on_start` 只在系统首次启动时执行，避免配置重载或调度器重启时重复触发 |
| `internal/scheduler/scheduler.go` | 新增 `firstStart` 和 `hasEverStarted` 标志位控制执行逻辑 |

#### Web 界面优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | "启动后立即运行"改为"首次启动时立即运行" |
| `internal/api/web/app.js` | 定时任务保存逻辑优化，支持批量创建/更新/删除 |

#### 文档更新

| 文件 | 变更 |
|------|------|
| `doc/API_DOCUMENTATION.md` | 更新 `run_on_start` 字段描述 |
| `readme.md` | 更新 `run_on_start` 字段描述 |

#### 测试增强

| 文件 | 新增测试 |
|------|----------|
| `internal/scheduler/scheduler_test.go` | `TestReloadDoesNotTriggerRunOnStart` - 验证重载不触发 run_on_start |
| `internal/scheduler/scheduler_test.go` | `TestStopStartDoesNotTriggerRunOnStart` - 验证停止后启动不触发 run_on_start |

### Stats

- **6 个文件变更**
- **+168 行 / -27 行**

***

## [3.3.3] - 2026-05-04

### Added

#### Web 界面交互优化

| 功能 | 说明 |
|------|------|
| 新增项目高亮动画 | 新增定时任务/Cookie 账户时，项目顶部显示 glowPulse 高亮效果 |
| SSE 连接状态指示器 | 顶部标题栏显示实时连接状态（绿色/红色圆点） |

### Changed

#### Web 界面优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 定时任务表单移除"启动后立即运行"默认勾选 |
| `internal/api/web/app.js` | 新增项目默认添加到列表顶部 |
| `internal/api/web/app.js` | 定时任务保存逻辑优化，只保存变更项 |
| `internal/api/web/app.js` | CodeMirror 编辑器销毁优化，防止内存泄漏 |
| `internal/api/web/app.js` | 服务器关闭流程优化 |

#### 后端功能增强

| 文件 | 变更 |
|------|------|
| `internal/scheduler/scheduler.go` | 新增 OnStatusChange 回调机制 |
| `internal/api/task_manager.go` | 任务管理器优化 |
| `internal/api/sse.go` | SSE 连接管理优化 |

### Stats

- **13 个文件变更**
- **+609 行 / -228 行**

***

## [3.3.2] - 2026-05-04

### Added

#### Web 界面实时功能增强

| 功能 | 说明 |
|------|------|
| SSE 连接状态指示器 | 顶部标题栏显示实时连接状态（绿色/红色圆点） |
| 定时任务实时推送 | 通过 SSE 自动推送定时任务状态变更 |
| 新增任务高亮动画 | 定时任务列表项新增 glowPulse 高亮效果 |

#### 样式优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/styles.css` | 新增 tag-info、tag-success、tag-warning、tag-danger 标签样式 |
| `internal/api/web/styles.css` | 新增 schedule-item 相关样式 |
| `internal/api/web/styles.css` | 移动端适配优化 |

#### 后端功能增强

| 文件 | 变更 |
|------|------|
| `internal/scheduler/scheduler.go` | 新增 OnStatusChange 回调机制 |
| `internal/api/task_manager.go` | 任务管理器优化 |
| `internal/api/sse.go` | SSE 连接管理优化 |

### Stats

- **12 个文件变更**
- **+720 行 / -266 行**

***

## [3.3.1] - 2026-05-04

### Added

#### Web 界面独立定时任务页面

| 功能 | 说明 |
|------|------|
| `/schedules` 路由 | 新增独立的定时任务页面 |
| 调度器状态检测 | 未启动时显示警告提示 |

### Changed

#### Web 界面优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 定时任务从 System 标签页移出，改为独立页面 |
| `internal/api/web/app.js` | 简化调度器界面，移除"表格模式"标签 |
| `internal/api/server.go` | 新增 `GET /schedules` 路由 |

### Fixed

| 文件 | 修复 |
|------|------|
| `internal/downloading/batch_download.go` | 添加 `u.user != nil` 空指针检查 |
| `internal/downloading/retry.go` | 添加 `leg.Tweet != nil` 空指针检查 |
| `internal/downloading/retry.go` | 添加 `te.Tweet != nil` 空指针检查 |

### Stats

- **7 个文件变更**
- **+89 行 / -47 行**

***

## [3.3.0] - 2026-04-29

### Added

#### 定时任务调度器

| 功能 | 说明 |
|------|------|
| `internal/scheduler/` | 新增调度器模块，支持 interval 和 daily 两种模式 |
| `ScheduleEntry` | 定时任务配置结构体 |
| `ScheduleStatus` | 定时任务状态结构体 |
| `Scheduler.Start()` / `Stop()` | 调度器启停控制 |

#### 调度器 API

| 端点 | 功能 |
|------|------|
| `GET /api/v1/schedules` | 获取调度器状态和任务列表 |
| `POST /api/v1/schedules` | 创建定时任务 |
| `GET/PUT /api/v1/schedules/raw` | 原始配置读写 |
| `POST /api/v1/schedules/reload` | 重载配置 |
| `POST /api/v1/schedules/validate` | 验证配置 |
| `PUT/DELETE /api/v1/schedules/{id}` | 更新/删除任务 |
| `PATCH /api/v1/schedules/{id}/enabled` | 启用/禁用任务 |
| `POST /api/v1/schedules/{id}/trigger` | 手动触发任务 |

#### Web 界面 Schedules 页面

| 功能 | 说明 |
|------|------|
| 任务列表 | 显示所有定时任务及其状态 |
| 创建任务 | 支持 interval 和 daily 两种调度模式 |
| 任务类型 | 支持 list/user/following 三种下载类型 |
| 任务控制 | 启用/禁用、手动触发、删除 |
| 原始编辑 | 支持 YAML 格式批量编辑 |

### Changed

| 文件 | 变更 |
|------|------|
| `internal/api/server.go` | 集成调度器，自动启停 |
| `internal/api/download_handlers.go` | 新增 `scheduledDownload()` 方法 |
| `internal/config/config.go` | 新增 `SchedulesFile` 配置项 |
| `internal/api/web/app.js` | 新增 Schedules 页面 |
| `internal/api/web/index.html` | 新增导航菜单和页面结构 |
| `internal/api/web/styles.css` | 新增调度器相关样式 |

### Stats

- **15 个文件变更**
- **+1298 行 / -260 行**

***

## [3.2.19] - 2026-04-29

### Added

#### Web 界面搜索状态管理

| 功能 | 说明 |
|------|------|
| `updateSearchState()` | 统一更新搜索状态辅助函数 |
| `restoreSearchValue()` | 恢复搜索输入框值辅助函数 |
| `taskFilter` / `taskSearch` | 新增任务筛选状态 |

### Changed

#### Web 界面优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 数据库页面搜索值实时保存到 state |
| `internal/api/web/app.js` | 日志搜索值实时保存到 state |
| `internal/api/web/app.js` | System 标签页独立更新，避免整页重渲染 |
| `internal/api/web/app.js` | 任务列表筛选从 state 获取条件 |

#### CodeMirror 编辑器修复

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 切换模式时清理 CodeMirror 实例，避免重复初始化 |
| `internal/api/web/app.js` | `initCodeMirror()` 清空容器后再初始化 |

### Fixed

- 修复搜索框值在渲染后丢失的问题
- 修复 CodeMirror 编辑器重复初始化的问题
- 修复 System 标签页切换时整页重渲染导致的性能问题

### Stats

- **1 个文件变更**
- **+94 行 / -41 行**

***

## [3.2.18] - 2026-04-29

### Added

#### 控制台日志模块

| 功能 | 说明 |
|------|------|
| `internal/consolelog/hub.go` | 新增控制台日志捕获和分发中心 |
| `Hub.Add()` | 添加日志行到内存并推送给订阅者 |
| `Hub.Snapshot()` | 获取当前日志快照 |
| `Hub.Subscribe()` | 订阅实时日志流 |
| `StartCapture()` | 启动 stdout/stderr 捕获 |

### Changed

#### 日志系统重构

| 文件 | 变更 |
|------|------|
| `internal/api/log_handlers.go` | 从文件读取改为内存快照 |
| `internal/api/log_handlers.go` | 日志流使用订阅模式替代轮询 |
| `internal/api/handlers.go` | 新增 `consoleLogHub()` 方法 |
| `internal/api/server.go` | 集成 consolelog 模块 |
| `main.go` | 启动时调用 `consolelog.StartCapture()` |

#### Web 界面日志实时化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 默认开启自动刷新 |
| `internal/api/web/app.js` | 新增 `startLogStream()` / `stopLogStream()` |
| `internal/api/web/app.js` | 使用 EventSource 实现真正实时日志流 |
| `internal/api/web/app.js` | "自动刷新"按钮改为"实时" |
| `internal/api/web/app.js` | 优化日志级别颜色匹配 |

### Stats

- **13 个文件变更**
- **+493 行 / -339 行**

***

## [3.2.17] - 2026-04-29

### Added

#### 代理配置增强

| 功能 | 说明 |
|------|------|
| `normalizeProxyURL()` | 新增代理 URL 格式验证函数 |
| `parseConfiguredProxyURL()` | 新增配置代理解析函数 |
| `proxyFromEnvWithoutBypass()` | 新增环境变量代理获取函数 |
| `proxyEnvPriority()` | 新增代理环境变量优先级定义 |
| `logProxySelection()` | 新增代理选择日志记录 |

#### 列表同步管理器单例模式

| 功能 | 说明 |
|------|------|
| `InitListSyncManager()` | 新增初始化函数 |
| `GetListSyncManager()` | 新增获取单例函数 |

### Changed

#### 代理功能重构

| 文件 | 变更 |
|------|------|
| `internal/config/config.go` | 新增代理 URL 验证，支持 http/https/socks5 |
| `internal/twitter/client.go` | 重构代理逻辑，分离配置和环境变量代理 |
| `internal/twitter/client.go` | HTTPS 请求优先 HTTPS_PROXY，回退 HTTP_PROXY |

#### 列表同步事务优化

| 文件 | 变更 |
|------|------|
| `internal/downloading/list_sync.go` | 改为单例模式 |
| `internal/downloading/list_sync.go` | 文件删除移到事务外，避免事务内 IO 操作 |

### Removed

- 删除 `DefaultMaxFileNameLen()` 函数，直接使用 `utils.DefaultMaxFileNameLen`

### Stats

- **38 个文件变更**
- **+353 行 / -222 行**

***

## [3.2.16] - 2026-04-29

### Changed

#### 默认并发数优化

| 文件 | 变更 |
|------|------|
| `internal/config/config.go` | 新增 `DefaultMaxDownloadRoutine()` 函数，统一默认并发数计算 |
| `internal/config/config.go` | 默认并发数从 `min(10, GOMAXPROCS*2)` 改为 `min(100, GOMAXPROCS*10)` |
| `internal/downloading/types.go` | 使用 `config.DefaultMaxDownloadRoutine()` 替代硬编码逻辑 |

#### 代码注释增强

| 文件 | 变更 |
|------|------|
| `internal/downloading/tweet_download.go` | 添加注释说明辅助文件是尽力而为，失败不影响媒体下载 |
| `internal/downloading/tweet_download.go` | 添加注释说明 `.json` 和 `.txt` 元数据不应阻塞媒体下载 |

### Stats

- **4 个文件变更**
- **+17 行 / -8 行**

***

## [3.2.15] - 2026-04-29

### Changed

#### 任务结果结构重构

| 文件 | 变更 |
|------|------|
| `internal/api/task_manager.go` | `TaskResult` 拆分为 `Main` 和 `Profile` 两个独立结果 |
| `internal/api/task_manager.go` | 新增 `TaskMainResult` 和 `TaskProfileResult` 结构体 |
| `internal/service/download_service.go` | `Result` 拆分为 `MainResult` 和 `ProfileResult` |
| `internal/service/download_service.go` | 新增失败推文追踪和统计功能 |

#### 媒体下载错误处理优化

| 文件 | 变更 |
|------|------|
| `internal/downloading/tweet_download.go` | 新增 `mediaDownloadError` 包装错误类型 |
| `internal/downloading/tweet_download.go` | 新增 `isNonRetriableMediaError()` 判断 403/404 |
| `internal/downloading/tweet_download.go` | 403/404 错误直接跳过，不进入重试队列 |
| `internal/downloading/tweet_download.go` | 区分可重试失败和跳过的 URL |

#### Profile 下载容错增强

| 文件 | 变更 |
|------|------|
| `internal/service/download_service.go` | Profile 下载失败不再导致整个任务失败 |
| `internal/service/download_service.go` | Profile 失败作为警告，主下载继续完成 |

### Removed

- 删除 `internal/service/interfaces_test.go`

### Fixed

- 修复任务结果深拷贝不完整的问题
- 修复媒体下载 403/404 错误进入无限重试的问题

### Stats

- **20 个文件变更**
- **+995 行 / -227 行**

***

## [3.2.14] - 2026-04-29

### Added

#### 实时进度报告

| 功能 | 说明 |
|------|------|
| `BatchProgress` / `RetryProgress` | 新增进度结构体，包含 total/completed/failed/current |
| `BatchProgressFunc` / `RetryProgressFunc` | 新增进度回调类型 |
| `BatchDownloadSummary` / `RetrySummary` | 新增下载摘要返回类型 |

### Changed

#### 下载进度实时报告

| 文件 | 变更 |
|------|------|
| `internal/downloading/batch_any.go` | `BatchDownloadAny()` 支持进度回调和摘要返回 |
| `internal/downloading/batch_download.go` | `BatchUserDownload()` 支持实时进度报告 |
| `internal/downloading/retry.go` | `RetryFailedTweets()` 支持进度回调和摘要返回 |
| `internal/service/download_service.go` | 新增 `newBatchProgressCallback()` 和 `newRetryProgressCallback()` |
| `internal/service/download_service.go` | 新增 `buildMainDownloadResult()` 构建下载结果 |
| `internal/service/download_service.go` | 所有下载方法统一使用进度回调 |

#### 其他优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | Web 界面优化 |
| `internal/cli/executor.go` | CLI 执行器增强 |
| `internal/api/handlers.go` | 处理器优化 |

### Stats

- **47 个文件变更**
- **+1062 行 / -648 行**

***

## [3.2.13] - 2026-04-29

### Changed

#### 进度报告增强

| 文件 | 变更 |
|------|------|
| `internal/service/progress.go` | `OnComplete()` 新增下载统计信息输出（downloaded/failed/versioned） |

#### 启动日志优化

| 文件 | 变更 |
|------|------|
| `main.go` | 新增下载路径输出 |
| `main.go` | 优化配置日志输出顺序 |

### Stats

- **3 个文件变更**
- **+21 行 / -3 行**

***

## [3.2.12] - 2026-04-29

### Added

#### 下载处理器优化

| 功能 | 说明 |
|------|------|
| `enqueueTask()` | 新增任务队列方法，简化任务创建 |
| `formatTaskMarkTime()` | 新增时间格式化函数 |
| 状态检查 | `executeDownloadTask()` 添加终端状态判断 |

### Changed

#### 下载服务优化

| 文件 | 变更 |
|------|------|
| `internal/service/download_service.go` | 删除 `returnWithReportedError()`，简化错误处理 |
| `internal/service/download_service.go` | `completeTask()` 新增 `warning` 参数 |
| `internal/service/download_service.go` | Profile 下载失败时添加警告而非直接失败 |
| `internal/service/download_service.go` | 移除冗余的 `DB != nil` 检查 |
| `internal/api/download_handlers.go` | 所有处理器统一使用 `enqueueTask()` |

#### 代码清理

| 文件 | 变更 |
|------|------|
| `internal/api/server.go` | 移除未使用的导入 |
| `main.go` | 主程序优化 |

### Fixed

- 修复任务状态转换时的竞态条件
- 修复 Profile 下载失败导致整个任务失败的问题

### Stats

- **12 个文件变更**
- **+223 行 / -175 行**

***

## [3.2.11] - 2026-04-29

### Added

#### 日志系统重构

| 功能 | 说明 |
|------|------|
| `logFollower` 结构体 | 优化日志流式读取，使用 `ReadAt` 替代 `Scanner` |
| 日志常量定义 | 日志文件名、分页大小、轮询间隔等 |
| 反向读取算法 | `readLogLinesTail()` 优化大文件读取性能 |

#### 服务依赖验证

| 功能 | 说明 |
|------|------|
| `Dependencies.Validate()` | 验证依赖项完整性 |
| `NewDownloadService()` | 返回错误而非 panic |

### Changed

#### 服务器关闭优化

| 文件 | 变更 |
|------|------|
| `internal/api/server.go` | 移除重启功能，简化关闭流程 |
| `internal/api/server.go` | 新增 `shutdownOnce` 确保关闭只执行一次 |
| `internal/api/server.go` | 新增 `WaitForShutdown()` 方法 |
| `internal/api/server.go` | 统一处理器命名规范 |

#### API 处理器优化

| 文件 | 变更 |
|------|------|
| `internal/api/config_handlers.go` | 处理器优化 |
| `internal/api/cookie_handlers.go` | 处理器优化 |
| `internal/api/download_handlers.go` | 处理器优化 |

### Removed

- 删除 `POST /api/v1/server/restart` 端点

### Fixed

- 修复日志大文件读取时的内存问题
- 修复服务依赖缺失时的 panic 问题
- 修复服务器关闭时可能的重复执行问题

### Stats

- **23 个文件变更**
- **+938 行 / -432 行**

***

## [3.2.10] - 2026-04-29

### Added

#### Web 界面功能增强

| 功能 | 说明 |
|------|------|
| `createFollowingTask()` | 新增关注下载任务创建功能 |
| `createMarkTask()` | 新增批量标记任务创建功能 |
| `escapeHtml()` / `escapeAttr()` | 新增 HTML/属性转义函数，防止 XSS |

### Changed

#### 文件写入优化

| 文件 | 变更 |
|------|------|
| `internal/downloader/file_writer.go` | 统一原子写入方法，合并 `atomicWrite` 和 `atomicWriteStream` 为 `atomicWriteFromReader` |
| `internal/downloader/file_writer.go` | 删除重复的 `atomicWrite` 函数 |
| `internal/downloader/downloader.go` | 新增 `waitRetryDelay()` 支持 context 取消 |
| `internal/downloader/downloader.go` | 下载结果新增 `Versioned` 字段 |

#### Web 界面安全增强

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 全面添加 HTML/属性转义，防止 XSS 攻击 |
| `internal/api/web/app.js` | 批量下载支持 `following_names` 参数 |

### Removed

- 删除 `internal/downloader/helpers.go`（功能已合并到 file_writer.go）

### Fixed

- 修复下载重试时无法响应 context 取消的问题
- 修复 Web 界面潜在的 XSS 安全风险

### Stats

- **16 个文件变更**
- **+482 行 / -245 行**

***

## [3.2.9] - 2026-04-29

### Changed

#### 任务管理器重构

| 文件 | 变更 |
|------|------|
| `internal/api/task_manager.go` | 新增完整的深拷贝机制，避免并发数据竞争 |
| `internal/api/task_manager.go` | 新增状态转换检查，防止非法状态变更 |
| `internal/api/task_manager.go` | 新增 `Close()` 方法，支持优雅停止清理 goroutine |
| `internal/api/task_manager.go` | 改进 `cleanupLoop()`，支持通过 channel 停止 |
| `internal/api/server.go` | 服务器关闭时调用 `taskManager.Close()` |
| `internal/api/server.go` | 日志前缀从 `[WebUI]` 统一改为 `[server]` |

### Fixed

- 修复任务数据在并发访问时的数据竞争问题
- 修复任务状态可能被非法转换的问题
- 修复服务器关闭时任务管理器 goroutine 泄漏问题

### Stats

- **15 个文件变更**
- **+518 行 / -172 行**

***

## [3.2.8] - 2026-04-29

### Added

#### 输入校验增强

| 校验项 | 规则 | 位置 |
|--------|------|------|
| ScreenName 格式 | 1-15字符，只允许字母、数字、下划线 | API + CLI |
| List ID 有效性 | 必须大于 0 | API + CLI |

#### 启动脚本

| 文件 | 说明 |
|------|------|
| `start.bat` | Windows 启动脚本 |
| `start.sh` | Linux/macOS 启动脚本 |

### Changed

| 文件 | 变更 |
|------|------|
| `internal/api/download_handlers.go` | 统一使用 `executeDownloadTask()` 执行下载任务，代码复用 |
| `internal/api/task_manager.go` | 新增 `executeDownloadTask()` 方法，统一任务执行逻辑 |
| `internal/api/handlers.go` | 处理器基类增强 |
| `internal/cli/args.go` | 新增 `isValidScreenName()` 校验函数 |
| `internal/service/download_service.go` | 下载服务优化 |
| `main.go` | 主程序优化 |

### Fixed

- API 层移除重复的 HTTP 方法检查（已在路由层统一处理）
- 批量下载增加参数校验，提前发现无效输入

### Stats

- **15 个文件变更**
- **+356 行 / -279 行**

***

## [3.2.7] - 2026-04-29

### Changed

#### API 代码重构

| 变更 | 说明 |
|------|------|
| 拆分 `internal/api/server.go` | 将 1246 行的 monolithic 文件拆分为多个处理器文件 |
| 新增 `config_handlers.go` | 配置管理相关处理器（配置读取、更新、表单） |
| 新增 `cookie_handlers.go` | Cookie 管理相关处理器（读取、保存、原始编辑） |
| 新增 `download_handlers.go` | 下载相关处理器（用户/列表/关注/批量下载） |
| 新增 `log_handlers.go` | 日志管理相关处理器（日志查看、实时流） |

### Fixed

- `internal/api/task_manager.go` 新增互斥锁，修复并发安全问题
- `internal/service/download_service.go` 优化错误处理逻辑
- `internal/cli/executor.go` CLI 执行器优化

### Stats

- **9 个文件变更**
- **+15 行 / -1252 行**（server.go 大幅精简）

***

## [3.2.6] - 2026-04-29

### Added

#### API 增强

| 端点 | 功能 |
|------|------|
| `POST /api/v1/users/{screen_name}/following/mark` | 标记用户的关注列表已下载 |
| `POST /api/v1/batch/download` | 批量下载支持 `following_names` 参数 |

### Changed

#### 服务层重构

| 文件 | 变更 |
|------|------|
| `internal/service/download_service.go` | 新增 `resolveUsers()`, `resolveLists()`, `resolveFollowings()` 方法，统一解析逻辑 |
| `internal/service/download_service.go` | `MarkDownloaded()` 和 `BatchDownload()` 改为接收原始参数，内部解析 |
| `internal/api/server.go` | 新增 `handleFollowingMark()` 处理关注列表标记 |
| `internal/api/server.go` | 简化 `handleUserMark()`, `handleListMark()`, `handleBatchDownload()` |
| `internal/api/types.go` | `BatchDownloadTaskData` 新增 `FollowingNames` 字段 |

### Fixed

- 修复 `database.MarkUserInaccessible()` 在 DB 为 nil 时的空指针问题
- 优化批量下载的错误处理和进度报告

### Removed

- 删除 `internal/cli/helpers.go`

### Stats

- **10 个文件变更**
- **+246 行 / -201 行**

***

## [3.2.5] - 2026-04-29

### Added

#### API 增强

| 端点 | 功能 |
|------|------|
| `GET/PUT/DELETE /api/v1/db/users/{id}` | 用户详情、更新、删除 |
| `GET/PUT/DELETE /api/v1/db/lists/{id}` | 列表详情、更新、删除 |
| `GET/PUT/DELETE /api/v1/db/entities/{id}` | 用户实体详情、更新、删除 |
| `GET/PUT/DELETE /api/v1/db/list-entities/{id}` | 列表实体详情、更新、删除 |
| `GET /api/v1/db/users/{id}/entities` | 获取用户的所有实体 |
| `GET /api/v1/db/lists/{id}/entities` | 获取列表的所有实体 |
| `GET /api/v1/db/entities/{id}/links` | 获取实体的所有链接 |
| `GET /api/v1/db/user-links/{id}` | 获取用户链接详情 |

#### 数据库操作增强

| 文件 | 新增功能 |
|------|----------|
| `internal/database/user.go` | 新增 `GetUserById`, `UpdateUser`, `DelUser` |
| `internal/database/lst.go` | 新增 `GetListById`, `UpdateList`, `DelList` |
| `internal/database/user_entity.go` | 新增 `GetUserEntity`, `UpdateUserEntity`, `GetUserEntitiesByUserId` |
| `internal/database/lst_entity.go` | 新增 `GetLstEntity`, `UpdateLstEntity`, `GetLstEntitiesByLstId` |
| `internal/database/user_link.go` | 新增 `GetUserLinkById`, `UpdateUserLink`, `DelUserLink` |

### Removed

- 删除 `internal/api/progress.go` 和 `internal/api/progress_test.go`（进度追踪功能移除）
- 删除 `internal/downloader/coverage` 测试覆盖率文件
- 删除 `internal/utils/algo.go` 和 `internal/utils/utils_test.go` 中的部分算法

### Changed

| 文件 | 变更 |
|------|------|
| `internal/api/db_handlers.go` | 新增完整的 CRUD 操作接口 |
| `internal/api/types.go` | 新增数据库项类型定义 |
| `internal/api/server.go` | 注册新的数据库管理路由 |
| `internal/api/task_manager.go` | 优化任务管理 |
| `internal/downloading/batch_download.go` | 优化下载逻辑 |
| `internal/downloading/list_sync.go` | 优化列表同步 |
| `internal/downloader/file_writer.go` | 优化文件写入 |

### Stats

- **21 个文件变更**
- **+280 行 / -483 行**

***

## [3.2.4] - 2026-04-29

### Fixed

| 文件 | 修复内容 |
|------|----------|
| `internal/twitter/user.go` | 解析用户数据时解码 HTML 实体（Name、Description、Location） |
| `internal/twitter/list.go` | 解析列表数据时解码 HTML 实体（Name） |
| `internal/downloading/tweet_download.go` | 保存推文时解码 HTML 实体（text） |
| `internal/downloading/json_folder_download.go` | 解析 JSON 时解码 HTML 实体（Creator.Name） |

### Changed

- 使用 `html.UnescapeString()` 统一处理 Twitter API 返回的 HTML 实体编码文本
- 修复用户名、简介、推文内容等显示为 `&amp;` `&lt;` 等 HTML 实体的问题

### Stats

- **4 个文件变更**
- **+11 行 / -7 行**

***

## [3.2.3] - 2026-04-29

### Changed

| 文件 | 变更 |
|------|------|
| `internal/downloading/types.go` | `userInListEntity.leid` 从 `*int` 改为 `int`，使用值类型替代指针类型 |
| `internal/downloading/list_download.go` | 简化赋值逻辑，直接使用值类型 |
| `internal/downloading/batch_any.go` | 使用 0 替代 nil 作为无效 leid |
| `internal/downloading/batch_download.go` | 使用 0 替代 nil 作为无效 leid |
| `*_test.go` | 同步更新测试用例 |

### Fixed

- 彻底解决循环变量地址共享问题，避免潜在的内存共享风险
- 使用 0 作为无效值替代 nil 检查，代码更简洁安全

### Stats

- **7 个文件变更**
- **+30 行 / -49 行**

***

## [3.2.2] - 2026-04-29

### Fixed

| 文件 | 问题 | 修复 |
|------|------|------|
| `internal/downloading/list_download.go` | 循环中共享局部变量地址导致所有用户指向同一内存 | 为每个用户创建独立的 eid 副本 |
| `internal/api/web/app.js` | 搜索框值通过 HTML 内嵌导致潜在 XSS 风险 | 改为渲染后动态恢复搜索值 |

### Stats

- **2 个文件变更**
- **+21 行 / -2 行**

***

## [3.2.1] - 2026-04-26

### Added

#### Web 管理界面增强

| 功能 | 说明 |
|------|------|
| **Cookie 管理** | 支持表单和原始格式编辑 Twitter Cookie |
| **服务器控制** | 支持通过 Web 界面重启和关闭服务器 |
| **优雅关闭** | 服务器支持优雅关闭，确保资源释放 |

#### API 增强

| 端点 | 功能 |
|------|------|
| `GET/PUT /api/v1/cookies` | Cookie 表单管理 |
| `GET/PUT /api/v1/cookies/raw` | 原始 Cookie 读写 |
| `POST /api/v1/server/restart` | 服务器重启 |
| `POST /api/v1/server/shutdown` | 服务器关闭 |

### Changed

| 文件 | 变更 |
|------|------|
| `internal/api/server.go` | 新增 Cookie 和服务器控制端点，优雅关闭支持 |
| `internal/api/web/app.js` | 新增 Cookie 管理和服务器控制页面 |
| `internal/api/web/styles.css` | 新增样式 |
| `internal/database/user.go` | 新增用户数据库操作方法 |
| `internal/database/lst.go` | 新增列表数据库操作方法 |

### Removed

- 删除 `internal/path/coverage` 测试覆盖率文件

### Stats

- **14 个文件变更**
- **+1,053 行 / -422 行**

***

## [3.2.0] - 2026-04-26

### Added

#### Web 管理界面重大更新

| 功能 | 说明 |
|------|------|
| **配置管理** | 支持表单和 YAML 两种编辑模式 |
| **实时日志** | 查看和流式传输应用日志 |
| **配置字段管理** | 动态管理配置字段 |

#### API 增强

| 端点 | 功能 |
|------|------|
| `GET/PUT /api/v1/config/raw` | 原始配置读写 |
| `GET/PUT /api/v1/config/fields` | 配置字段管理 |
| `GET /api/v1/logs` | 分页获取日志 |
| `GET /api/v1/logs/stream` | 流式日志传输 |

### Changed

| 文件 | 变更 |
|------|------|
| `internal/api/server.go` | 新增配置和日志端点，添加并发安全锁 |
| `internal/api/web/app.js` | 新增系统配置和日志页面 |
| `internal/api/web/styles.css` | 新增配置和日志样式 |
| `doc/API_DOCUMENTATION.md` | 更新 API 文档 |

### Stats

- **11 个文件变更**
- **+1,321 行 / -137 行**

***

## [3.1.7] - 2026-04-26

### Changed

| 文件 | 变更 |
|------|------|
| `internal/service/download_service.go` | 删除冗余日志，统一使用 Progress.Current 传递上下文 |
| `internal/service/progress.go` | 简化 OnComplete 方法，统一使用 r.Message |

**主要变更：**
- 删除 `log.Infof` 冗余日志输出
- 将日志上下文信息整合到 `Progress.Current` 字段
- 统一完成日志格式，简化 `LogReporter.OnComplete` 逻辑

***

## [3.1.6] - 2026-04-26

### Fixed

| 文件 | 变更 |
|------|------|
| `internal/downloading/profile/downloader.go` | 修复失败时未设置 Error 的问题，添加 Versioned 字段 |
| `internal/downloading/profile/types.go` | FileResult 新增 Versioned 字段 |
| `internal/service/download_service.go` | 优化版本化文件统计逻辑 |

**修复内容：**
- Profile 下载失败时正确设置错误信息
- 下载结果现在正确报告版本化文件数量（旧文件备份到 .versions 目录）
- 失败时返回 FilePath 便于调试

***

## [3.1.5] - 2026-04-26

### Changed

| 文件 | 变更 |
|------|------|
| `main.go` | 重构 CLI 启动流程，优化错误处理 |
| `internal/config/config.go` | 简化配置初始化逻辑 |
| `internal/cli/executor.go` | 优化执行器错误处理 |
| `internal/cli/executor_test.go` | 更新测试用例 |

**主要变更：**
- 重构 CLI 启动流程，简化配置初始化
- 优化错误处理机制
- 改进日志输出格式

***

## [3.1.4] - 2026-04-26

### Changed

- 优化 profile 下载完成报告，添加 versioned 计数

***

## [3.1.3] - 2026-04-26

### Changed

- 优化下载器和进度报告，清理冗余代码

***

## [3.1.2] - 2026-04-26

### Changed

- 重构配置模块，删除部分更新功能，优化客户端初始化

***

## [3.1.1] - 2026-04-26

### Fixed

- 修正文档：删除 readme.md 中错误的月份子目录描述

***

## [3.1.0] - 2026-04-26

### Changed

#### JSON 下载模块重构

| 文件 | 变更 |
|------|------|
| `internal/downloading/json_download.go` | **删除** - 拆分为独立模块 |
| `internal/downloading/json_file_download.go` | 新增 - 单文件 JSON 下载 |
| `internal/downloading/json_folder_download.go` | 新增 - 文件夹 JSON 下载 |
| `internal/downloading/tweet_json_converter.go` | 新增 - Tweet 转换器 |
| `internal/database/tx/manager.go` | 新增 - 事务管理器 |

**主要变更：**
- 将 JSON 下载功能拆分为文件和文件夹两种模式
- 新增事务管理器，优化数据库事务处理
- 更新 CLI 参数支持新的 JSON 下载选项
- 完善 Web 界面交互

### Stats

- **24 个文件变更**
- **+2,398 行 / -1,415 行**
- **新增文件：** 5 个
- **删除文件：** 2 个

***

## [3.0.3] - 2026-04-25

### Changed

#### 文档更新

| 文件 | 变更 |
|------|------|
| `readme.md` | 添加 Service 层架构详细说明 |
| `doc/API_DOCUMENTATION.md` | 更新 API 文档，完善错误码说明 |

**文档内容：**
- 新增 Service 层架构设计目标和 `DownloadService` 接口说明
- 完善项目架构图，展示 Service 层位置
- 更新 API 文档中的错误码和响应格式说明

### Removed

#### 清理未使用的类型

| 文件 | 变更 |
|------|------|
| `internal/api/types.go` | 删除未使用的响应类型（`DBUserResponse`, `DBListResponse`, `DBEntityResponse`） |
| `internal/api/types_test.go` | 删除相关测试代码 |

### Stats

- **4 个文件变更**
- **+155 行 / -110 行**

***

## [3.0.2] - 2026-04-25

### Fixed

#### 修复数据库死锁问题

| 文件 | 变更 |
|------|------|
| `internal/database/lst_entity.go` | 新增 `GetLstEntityWithTx` 函数，支持事务查询 |
| `internal/database/model.go` | 新增 `PathWithTx` 方法，支持事务内路径计算 |
| `internal/downloading/list_sync.go` | 修复 `removeUserLinkWithTx`，统一使用事务对象 |

**修复内容：**
- 修复 `-list` 下载时可能出现的数据库死锁问题
- 根本原因：事务内混合使用 `tx` 和 `lsm.db` 导致单连接配置下循环等待
- 解决方案：统一使用事务对象 `tx` 进行所有数据库操作

### Stats

- **3 个文件变更**
- **+20 行 / -5 行**

***

## [3.0.1] - 2026-04-24

### Fixed

#### 数据库事务支持修复

| 文件 | 变更 |
|------|------|
| `internal/database/model.go` | 修复 `UserLink.Path` 方法，添加 `Querier` 接口支持 |
| `internal/downloading/list_sync.go` | 使用事务对象 `tx` 替代 `lsm.db` 确保数据一致性 |

**修复内容：**
- 新增 `Querier` 接口，支持 `*sqlx.DB` 和 `*sqlx.Tx` 两种类型
- 修复 `UserLink.Path` 方法，使其能在事务中正确执行查询
- 优化 `list_sync.go` 中的 `removeUserLinkWithTx`，确保在事务内一致地读取数据
- 改进错误处理，提供更详细的错误信息

### Stats

- **4 个文件变更**
- **+17 行 / -11 行**

***

## [3.0.0] - 2026-04-24

### Changed

#### 架构重构

| 文件 | 变更 |
|------|------|
| `internal/api/async_executor.go` | **删除** - 移除异步执行器 |
| `internal/api/server.go` | 重构 - 简化任务创建流程 |
| `internal/api/progress.go` | 新增 - 进度追踪模块 |
| `internal/cli/executor.go` | 重构 - 整合执行器 |
| `internal/cli/executor_*.go` | **删除** - 合并到主 executor |
| `internal/cli/paths.go` | **移动** 到 `internal/path/store.go` |
| `internal/service/` | 新增 - Service 层架构 |
| `internal/path/` | 新增 - 路径管理模块 |

**架构变更：**
- 移除 `async_executor`，改为直接在 server 中处理任务
- 新增 Service 层 (`internal/service/`)，实现业务逻辑分离
- 新增 Path 模块 (`internal/path/`)，统一管理路径存储
- 合并 CLI executor 文件，简化代码结构
- 移除 `internal/profile/fetcher.go`

#### API 优化

| 文件 | 变更 |
|------|------|
| `internal/api/server.go` | 简化任务创建流程，移除同步 Twitter API 调用 |
| `internal/api/task_manager.go` | 优化任务管理 |

**优化内容：**
- 移除任务创建时的同步 Twitter API 调用
- 任务创建改为纯异步模式，提高响应速度
- 简化 API 响应数据结构

#### 测试增强

| 文件 | 变更 |
|------|------|
| `internal/api/*_test.go` | 新增 11 个测试文件 |
| `internal/cli/*_test.go` | 新增 3 个测试文件 |
| `internal/database/test/` | 新增数据库测试套件 |
| `internal/downloading/*_test.go` | 新增 12 个测试文件 |
| `internal/service/*_test.go` | 新增 5 个测试文件 |

**测试覆盖：**
- 大幅扩展单元测试覆盖范围
- 新增数据库集成测试
- 新增 API 端点测试
- 新增 Service 层测试

### Stats

- **87 个文件变更**
- **+25,557 行 / -1,251 行**
- **新增文件：** 64 个
- **删除文件：** 6 个

***

## [2.14.2] - 2026-04-23

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

## [2.14.1] - 2026-04-23

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

## [2.14.0] - 2026-04-23

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

## [2.12.3] - 2026-04-22

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

## [2.12.2] - 2026-04-22

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

## [2.12.1] - 2026-04-21

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

