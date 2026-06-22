# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

***

## [v3.4.25] - 2026-06-22

### Added

#### 版本信息可点击跳转
- **web1/web2**: 底部版本信息 `TMD Pro` / `v2 · Go + SQLite` 改为 `<a>` 标签，点击跳转到 GitHub 仓库（`https://github.com/leeexx2001/tmd`），web1 最后一个内联 `onclick` 至此消除

#### 文档补全
- `readme.md`: 补充 API 端点速查表中缺失的 theme 端点
- `API_DOCUMENTATION.md` / `call_chain_analysis_report.md`: 补充 Theme API 文档
- 修复三份文档中的过时代码引用

#### 其他
- `.gitignore`: 将 `CONTEXT.md`、`docs/`、`course/` 加入忽略列表

### Changed

#### web1 前端全面重构（内联样式→CSS 类、事件委托统一）
- **移除 CodeMirror**: 删除全部 CodeMirror 5 依赖（CDN 加载、init/destroy/状态跟踪），YAML 编辑器改用原生 `<textarea>` + monospace 样式，离线完全可用，净减 102 行
- **日志页面重构**: 重写为 web2 风格——移除 `card-title`/`card-subtitle`，改用 `toolbar` 结构（`toolbar-left`/`toolbar-right`），`onclick`/`onchange`/`onkeydown` 全部替换为 `data-action` 统一事件委托
- **数据页面**: 加 `page-container` 包裹层
- **任务页面**: 创建任务卡片补 `card-subtitle`

#### 内联样式→CSS 类提取
| CSS 类 | 替代 | 替换次数 |
|--------|------|---------|
| `.raw-editor-body` / `.raw-editor-container` / `.raw-editor-hint` | 3 处原始编辑器内联布局 | 9 处 |
| `.mode-tabs-wrapper` | 6 处 `display:flex;flex-direction:column;height:100%` | 6 处 |
| `.skeleton-icon` | 6 处骨架屏 loading | 6 处 |
| `.mobile-card-title` / `.mobile-card-meta` | 12 处移动端卡片内联样式 | 12 处 |
| `.code-display` | 5 处 ID 显示块 | 5 处 |
| `.tag-sm` | 状态标签内联 `font-size`/`padding` | 1 处 |
| `.schedule-meta-sep` | 分隔点内联 `color` | 1 处 |
| `.checkbox-inline` | 6 组调度表单 checkbox | 12 处 |
| `.form-hint-inline` | 3 处"每行一个"提示 | 3 处 |
| `.form-hint-validate` | 调度验证提示 div | 1 处 |
| `.log-level-filters` → `.toolbar` 结构 | 日志页筛选栏 | 整套 |

#### web2 改进
- 新增 SSE 日志流相关支持
- 日志按钮改用 SVG 箭头图标

### Fixed

- **web1 toolbar padding**: 日志页 toolbar 纵向 padding 从硬编码 `6px` 改为 `var(--space-3)`（12px），与 CSS 变量体系一致
- **颜色 fallback 不一致**: `var(--danger, #ef4444)` → `var(--danger, #f85149)`（CSS 变量实际值为 `#f85149`）
- **冗余内联样式**: 移除 `.empty-state` 上多余的 `display:flex`（CSS 类自带）
- **日志排序反转**: 修正日志顺序，匹配终端输出顺序
- **新日志到达按钮图标**: 📌 → ↓（web2 保留 ↓ 箭头）
- **syncSchedulesPage**: 骨架屏缺失时触发全页渲染
- **scheduler_test**: 修正 `nextDailyTrigger` 测试 `times` 顺序

### Removed

- **CodeMirror 5**: 移除 CDN 加载（`codemirror.min.js`/`codemirror.min.css`/`yaml.min.js`/`material-darker` 主题）、`loadCodeMirrorAssets`/`waitForCodeMirror`/`initCodeMirror`(CM分支)/`destroyCodeMirror` 等全部相关代码
- **CM 状态跟踪变量**: `_cmDestroyVersion`、`_cmWaitCancelled`、`_configCmInitializing`、`_cookiesCmInitializing`、`_scheduleCmInitializing`

### Stats

- **19 个文件变更**
- **+754 行 / -577 行**

***

## [v3.4.24] - 2026-06-21

### Added

#### 前端主题动态切换系统
- **主题切换 API**: `GET /api/v1/config/themes` 列出可用主题、`GET /api/v1/config/theme` 获取当前主题、`POST /api/v1/config/theme` 切换主题，支持运行时热切换
- **通用主题切换器 UI**: 通过 `handleWeb` 自动注入到所有主题 HTML 中，支持任意数量主题的动态发现和切换
- **主题目录自动发现**: `listThemes()` 遍历 embed FS 中所有包含 `index.html` 的子目录，新增主题目录无需修改 Go 代码
- **浏览器缓存消除**: 切换主题时强制 reload(true)，`handleWeb`/`handleStatic` 均使用 `Cache-Control: no-cache` + ETag 校验，304 状态码保证未修改文件零字节传输

#### 前端 Web 目录重构
- `internal/api/web/` → `web/web1/` + `web/web2/` 双主题结构，通过 `frontendTheme` 运行时变量控制当前启用的主题目录
- `setFrontendTheme` 加锁方式修复：单一 `Lock+defer` 消除验证和写入之间的 TOCTOU 竞态窗口
- 删除未使用的 `getFrontendDir()` 函数，清理死代码
- 前端单元测试补充：`TestHandleGetThemes_ReturnsList`、`TestHandleSetTheme_InvalidTheme`、`TestHandleSetTheme_Success`

#### 调度器验证
- `internal/scheduler/validate.go`: 新增独立的调度配置验证模块，支持 raw YAML、单个 entry、entries 列表三种模式的校验
- `POST /api/v1/schedules/validate` 新增 API 端点，web2 保存原始 schedules.yaml 时先调验证

### Changed

#### web2 前端改进
- **Shut Down Server 按钮**: 从 System 区域移至 Configuration 卡片右上角，与配置操作标签同行
- **favicon 支持**: 新增 `favicon.svg`，`handleFavicon` 返回 SVG 图标
- **SSE 事件优化**: PublishServerShutdown 确保客户端收到关闭通知

#### Scheduler 重构
- `internal/scheduler/scheduler.go`: 大幅简化解耦，消除与 validate 的耦合，提取独立验证逻辑到 `validate.go`
- 周期性检查逻辑简化，`RunOnStart` 执行时机优化

#### Service 层精简
- `internal/service/download_service.go`: 移除重复的错误处理检查和冗余日志记录，减少 36 行
- 测试用例同步更新适配

### Fixed

- **TOCTOU 竞态**: `setFrontendTheme` 验证后写入之间存在的竞态窗口，已通过全锁操作修复
- **主题切换缓存问题**: 切换后浏览器缓存旧 CSS/JS，需 Ctrl+F5 才能生效。修复方案：强制 reload + no-cache Cache-Control + ETag 校验
- **前端死代码**: 清理 `internal/api/web1/styles.css` 中 CSS 冗余声明、web2 中未使用的事件绑定

### Removed

- `internal/api/web/web3/`: 经过两轮设计迭代后未达到要求，已移除
- `internal/database/connect.go`: 未使用的数据库连接辅助代码
- `internal/database/schema.go`: 重复的 schema 定义
- `internal/database/sqlite_migration.go`: 已合并到主 migration 流程

### Stats

- **32 个文件变更**
- **+3,576 行 / -355 行**

***



### Added

#### 完整架构文档体系
- `AGENTS.md`: 基于 CodeGraph 全面重写，从 11 节扩展为 22 节的完整架构文档，涵盖分层关系、调用链、任务类型、并发模型、API 路由表等
- `doc/mark-downloaded详解.md`: 完全重写，移除已不存在的代码引用和行号，替换为当前 Service 层实现
- `doc/SERVICE_LAYER.md`: 修复 Dependencies 缺少 ListSyncManager、API Server config copy、CLI 局部变量模式 3 处过时代码
- `doc/foo.db 技术文档.md`: 修复 uid→user_id、owner_uid→owner_user_id、索引名等 9 处过时 Schema
- `doc/call_chain_analysis_report.md`: 修正 DB 端点 PUT→PATCH，新增 scheduler API 13 个路由
- `doc/用户名变更处理机制.md`: 修复 user_previous_names 表字段名 uid→user_id

#### CLUADE.md 行为准则完善
- 修正「九荣八耻」→「十荣八耻」，与实际条款数一致
- 新增「以使用 RTK 为荣」条款及 RTK 速查表（git / diff / log / test 等命令的 rtk 前缀用法）

#### 独立工具说明
- `readme.md` 开发指南中新增 tmd-db-migrate 和 convert_db_to_legacy.py 说明表格

### Changed

#### readme.md 全面重构（精简约 130 行）
- **精简**：功能特性 65 行→9 行概览，删除与「命令行参数详解」完全重复的「参数类型总结」(32 行)
- **去重**：API 端点速查中 13 行 scheduler 路由改为引用「调度器 API」节
- **前移**：安全说明移至安装与配置之后，使用示例前移至第 4 节（快速开始）
- **重组**：参数兼容性速查表改为命令行参数详解的子节，项目架构移至开发指南前
- **更新**：测试文件数 49→58、最大文件名范围 50-250→50-245、优雅关闭顺序补充队列 15s 等待、删除已不存在的 PROFILE_DOWNLOAD_RESULTS 和 MARK_DOWNLOADED_RESULTS 输出格式

#### 前端架构重构（Web UI）
- **事件系统**：替换所有内联 `onclick`/`oninput`/`onchange` 为 `data-action`/`data-binding` 事件委托，消除全局命名空间污染
- **DOM 更新策略**：全量 `render()` → 页面级「手术刀」增量 DOM 更新，保持滚动位置和表单状态
- **状态管理**：`store.setState` 通知从同步改为微任务批处理（`Promise.resolve().then`），消除连续 setState 导致的重复渲染
- **变化检测**：引入 `makeChangeDetector` 自动快照对比，消除 syncDataPage/syncSchedulesPage/syncLogsPage/syncOverviewPage 中 13 处手动 `_state.last*` 维护
- **CodeMirror**：按需延迟加载（首次进入 YAML 编辑器时才加载 CDN 资源）
- **事件委托**：`blur` capture 改为 `focusout` 冒泡，消除对点击事件分发的干扰

#### 应用配置面板优化
- 三个面板（配置/额外账户/任务配置）统一预加载原始数据，切换高级模式时无需等待异步请求
- 面板重建时保存/恢复滚动位置
- `rerenderSystemPanel` save/restore 增加空内容跳过逻辑，防止空值覆盖已加载数据

### Fixed

- **原始 YAML 编辑器内容空白**：`rerenderSystemPanel` 的 save/restore 机制在数据异步加载完成后，saveFn 保存了旧 CodeMirror 的空内容（`''`），restoreFn 用空内容覆盖了新加载的数据。修复：restore 条件增加 `saved !== ''` 检查
- **三个面板高级模式无法回退**：`_configPanelSkipNextRebuild` 等 skip 标志在模式切换后未清除，阻挡了订阅路径的面板重建。修复：`setSystemTab` 切换 tab 时清除所有 skip 标志，syncSystemPage 增加方向判断
- **setState 微任务批处理时序问题**：setConfigMode/setCookiesMode/setScheduleTab 中 store.setState 异步通知 + loadRaw 异步请求导致编辑器初始化时序错乱。修复：访问 tab 时即预加载原始数据
- **SSE 初始连接闪变**：首次连接未建立时的 onerror 被误判为断线。修复：sseManager 增加 `_everConnected` 标志
- **表单切换丢失输入**：任务中心 tab 切换时表单值被清空。修复：增加 `saveTaskFormState` / `restoreTaskFormState`

### Removed

- `readme.md`：「参数类型总结」整节（与命令行参数详解完全重复）
- `readme.md`：「功能特性」7 个子节（被后面各节详细重复说明）
- `internal/api/web/app.js`：11 个内联 `on*` 处理器（全部改为 `data-action`/`data-binding` 委托）
- `internal/api/web/app.js`：`blur` capture 监听器（改为 `focusout` 冒泡）
- `internal/naming/base.go`：可变的全局 `MaxFileNameLen`（改为通过参数注入）
- `internal/downloading/list_sync.go`：`ListSyncManager` 全局单例模式（改为依赖注入）

### Stats

- **48 个文件变更**
- **+3,778 行 / -2,401 行**

***

## [v3.4.22] - 2026-06-17

### Added

#### SSE 性能优化：预序列化 JSON 事件缓存
- `internal/api/event_bus.go`:
  - `SSEEvent` 新增 `Raw []byte` 字段，在 `Publish` 时预序列化一次，所有订阅者共享同一份 JSON 字节切片
  - 修复「N 个订阅者 → N 次 `json.Marshal`」的重复序列化问题
- `internal/api/sse_tasks.go`:
  - `writeSSEEvent()` 优先使用 `evt.Raw`，无缓存时回退到 `json.Marshal`

#### DB 字段校验：Name 和 ScreenName 防空/防超长
- `internal/api/db_handlers.go`:
  - 新增 `validateFieldName()` — 拒绝空串和超过 250 字符的 Name
  - 新增 `validateScreenName()` — 复用 `utils.IsValidScreenName`，校验 1-15 字符格式
  - 应用于 5 个 PATCH 端点：`/db/users/{id}`、`/db/lists/{id}`、`/db/entities/{id}`、`/db/list-entities/{id}`、`/db/user-links/{id}`
- `internal/api/db_handlers_test.go` — 新增 `TestHandleDBUserUpdate_RejectsEmptyStrings`

### Changed

#### 配置指针解耦
- `internal/api/server.go`:
  - `NewServerWithConsoleLogHub()` 中 `service.Dependencies.Config` 改为持有 `*config.Config` 的值拷贝，不再与 `Server.config` 共享指针
  - 消除配置热更新在运行时组件间"部分生效"的不一致问题

#### MaxDownloadRoutine 上限保护
- `internal/config/config.go`:
  - 新增常量 `MaxDownloadRoutine = 100`
  - `GetFieldDefs()` setter 拒绝 n&lt;1 或 n&gt;100 的输入
  - `NormalizeLoadedConf()` 将负数归零、超大值截断到上限

#### 表单 API 防御性校验
- `internal/api/config_handlers.go`:
  - 新增 `isMaskedValue()` 函数，检测 `maskSensitive` 输出的占位文本（`"abc•••xyz"` 或 `"***"`）
  - `handleSaveConfigFields()` 中拒绝掩码值写入配置
- `internal/api/cookie_handlers.go`:
  - `resolveCookieSaveValue()` 新增掩码值检测

#### 调度器 Triggering 标志释放安全网
- `internal/scheduler/scheduler.go`:
  - `execute()` 和 `triggerEntry()` 的 defer 重构，panic 路径与正常路径互斥
  - 新增 `releaseTriggeringLocked()` 方法，仅在同 `generation` 时强制释放 Triggering
  - 防止 `releaseAndUpdateStatus` 因 generation/ID 校验失败跳过清理

#### 代码审计修复
- `internal/api/download_handlers.go` — `cleanupUploadDirAfterTask` 的 defer 改为命名返回值，panic 时 `log.Errorf` + 返回错误
- `internal/twitter/client.go` — `apiCounts.LoadOrStore` 和 `Range` 中类型断言加 `ok` 检查
- `internal/twitter/client.go` — Twitter API client 新增 `SetTimeout(15s)`
- `internal/downloader/downloader.go` — 媒体下载 client 新增 `SetTimeout(5m)`
- `internal/twitter/tweet.go` — `parseUserResults` 错误改为 `log.Debugf` 记录而非静默丢弃

### Fixed

- **配置指针共享**（Bug 8）：`Server.config` 与 `service.Dependencies.Config` 共享同一指针，配置热更新后部分组件生效、部分不生效。修复：`Dependencies.Config` 持有值拷贝，隔离热更新影响
- **表单 API Cookie 更新**（Bug 10）：`maskSensitive` 输出的掩码值可被误写入配置，导致认证失败。修复：后端增加 `isMaskedValue` 检测
- **MaxDownloadRoutine 无上限**（Bug #6）：可设为 `math.MaxInt` 导致 OOM。修复：Setter 拒绝 &gt;100 的值，`NormalizeLoadedConf` 截断到上限
- **MediaCount NULL 混淆**（Bug #7）：`MediaCount()` 直接返回 `.Int32` 不检查 `.Valid`，NULL 被当作 0。修复：`.Valid` 检查 + `MediaCountValid()`；调用方据此区分未知
- **Dumper count-- 下溢**（Bug #11）：`count--` 无下溢保护。修复：前加 `> 0` 检查
- **parseTwitterDate 回退为 time.Now()**（Bug #12）：空/无效日期返回 `time.Now()`，文件 mtime 被设为当前时间。修复：返回固定零值 `2000-01-01`
- **调度器 Triggering 永久锁定**（Bug 2）：generation 变更后 `releaseAndUpdateStatus` 返回 false，Triggering 永不释放。修复：新增 `releaseTriggeringLocked` 安全网
- **BatchDownloadTweet context 泄漏**：`context.WithCancelCause` 的 cancel 只在 `ENOSPC` 路径被调用。修复：`defer cancel(nil)`
- **DownloadFromLoongTweetFolder 无并发限制**：每个 `folderPath` 直接 `go func()`。修复：复用信号量模式
- **detachJob 关闭时资源泄漏**：关闭路径跳过 `delete(q.detached)` 和 `releaseTargets`。修复：关闭路径也执行清理
- **信号量获取阻塞不可取消**：`sem <- struct{}{}` 在主循环中阻塞。修复：用 `select` 同时监听 `ctx.Done()`
- **SSE 事件队列内存泄漏**：`s.queue = s.queue[1:]` 底层数组永不收缩。修复：完全消费后 `s.queue = nil`
- **logSubscriber 队列内存泄漏**：同上模式。修复：同上

### Stats

- **40 个文件变更**
- **+459 行 / -186 行**

***

## [v3.4.21] - 2026-06-12

### Changed

#### 调度器：panic 恢复 + 停止不挂起 + 每日时间上限 + DST 安全
- `internal/scheduler/scheduler.go`:
  - `triggerEntry()`: 新增 panic recovery，捕获 downloadFunc panic 时更新状态（LastError/ConsecutiveFailures）后重新 panic，通过 `released` 标记防止重复释放
  - `execute()`: 新增 panic recovery，捕获 downloadFunc panic 时记录错误日志并更新状态
  - `stopLocked()`: 移除 `sc.cancel = nil / sc.ctx = nil` 重置，将 goroutine+5s 超时等待改为直接 `sc.wg.Wait()`，解决 Reload 失败恢复后 Stop 因 `context.Background()` 永不退出的 hang 问题
  - `nextDailyTrigger()`: 将 `today.Add(24 * time.Hour)` 替换为 `today.AddDate(0, 0, 1)`，夏令时（DST）切换日安全
  - `ParseSchedule()`: 新增 `maxDailyTimes = 96` 常量，超过 96 个时间点时报错"too many daily times"

#### API 重构：抽取通用资源处理器
- `internal/api/resource_handler.go` **新文件** - 创建通用工具层：
  - `resolvePathID()` - 路径参数统一解析为 uint64，失败自动写 400
  - `decodeBody()` - JSON 请求体统一解析，失败自动写 400
  - `requireResource[T any]()` - 泛型资源存在性检查（错误→500、nil→404）
  - `writeResourceJSON()` / `writeResourceDeleted()` - 标准化成功响应写入
  - `countWithError()` - 安全 COUNT 查询（失败自动写 500）
  - 实体转换函数：`dbUserToItem`、`dbListToItem`、`dbEntityToItem`、`dbLstEntityToItem`、`dbUserLinkToItem`、`dbPrevNameToItem`
  - `countUserCascade()` - 级联删除记录统计
- `internal/api/db_handlers.go` **大量简化**（-591 行）：
  - 所有 handler 的内联 ID 解析、JSON 解码、nil/err 检查、Item 构建、COUNT 查询、响应写入统一替换为上述工具函数
  - 所有删除 handler 的级联统计替换为 `countUserCascade`

#### 关闭流程：移除无根据的 2 秒延迟
- `internal/api/server.go`:
  - `GracefulShutdown()`: 移除 `time.Sleep(2 * time.Second)` 注释行（该延迟原是给正在运行的 handler 完成 DB 访问的缓冲，但实际并无 detached goroutine 依赖此延迟）

#### 下载服务：Dumper 写入模式改为直接覆盖 + 并发安全修复
- `internal/service/download_service.go`:
  - `saveDumper()` / `saveJsonDumper()`: 从 load-merge-save 改为直接覆盖写入，确保 Remove 操作被持久化（修复 BUG：重试成功后已移除的推文被 load-merge 加回来）
  - 新增 `replaceDumper()` / `replaceJsonDumper()`: 直接写入不做 load-merge，用于 RetryAllFailed 需要持久化 Remove 的场景
  - `RetryAllFailed()`: 改用 replaceDumper/replaceJsonDumper，且失败推文的 Load 操作置于 dumperMu 锁保护下，避免与 saveDumper 的并发写入产生竞态（修复 BUG 62）
  - `JsonFileDownload()` / `JsonFolderDownload()`: jsonDumper.Load() 置于 dumperMu 锁保护下（修复 Issue A）
  - `ClearErrors()`: os.Remove 操作置于 dumperMu 锁保护下，避免与 saveDumper 并发写入冲突（修复 Issue B）

### Added

#### 测试覆盖：调度器停止挂起、每日时间上限
- `internal/scheduler/scheduler_test.go`:
  - `TestStopAfterReloadRecoverDoesNotHang` - 验证 Reload 失败→恢复后 Stop 能在 5 秒内正常完成
  - `TestParseScheduleRejectsTooManyDailyTimes` - 验证 97 个时间点被拒绝
  - `TestParseScheduleAcceptsMaxDailyTimes` - 验证 96 个时间点被接受

#### 测试覆盖：Dumper 并发安全 + BUG 回归
- `internal/service/download_service_test.go`（+312 行）:
  - `TestDownloadServiceImpl_SaveDumper_RemoveAfterRetry` - BUG 61 回归：replaceDumper 能持久化 Remove（推文 10 不在文件中）
  - `TestDownloadServiceImpl_SaveDumper_RemoveViaSaveDumper` - BUG 61 回归：saveDumper 直接覆盖不再 load-merge
  - `TestDownloadServiceImpl_SaveDumper_FileRace` - BUG 62 回归：Load 在锁内不再被 saveDumper 并发写入破坏
  - `TestDownloadServiceImpl_JsonLoadWithoutLock` - Issue A 回归：jsonDumper.Load() 在锁内不再被 replaceJsonDumper 破坏
  - `TestDownloadServiceImpl_ClearErrorsRace` - Issue B 回归：ClearErrors 在锁内不再与 saveDumper 并发冲突
  - 并发测试适配：`TestSaveDumperConcurrentWritesPreserveData` 更新断言（直接覆盖模式下只保证至少 1 条，不保证两条都保留）

#### 前端：任务详情面板 UI
- `internal/api/web/styles.css`（+169 行）:
  - 新增 `.task-detail-header` / `.task-detail-header-info` / `.task-detail-header-title` / `.task-detail-header-sub` - 任务详情头部
  - 新增 `.task-detail-section` / `.task-detail-section-title` - 详情分区标题
  - 新增 `.task-detail-grid` / `.task-detail-label` / `.task-detail-value` - 详情键值网格
  - 新增 `.task-detail-card` - 详情卡片容器
  - 新增 `.task-detail-time-row` / `.task-detail-time-dot` / `.task-detail-time-label` / `.task-detail-time-value` / `.task-detail-time-line` - 时间线组件
  - 新增 `.task-detail-error` - 错误信息区块
  - 新增 `.task-detail-stats` / `.task-detail-stat` / `.task-detail-stat-val` / `.task-detail-stat-lbl` - 统计徽标
  - 新增 `.task-detail-msg` - 消息区域
  - 修复 `--accent` → `--accent-primary` CSS 变量名
  - 移除未使用的 `.config-label` 样式
- `internal/api/web/index.html`:
  - 移除 `id="sidebarFooter"`，布局微调
- `internal/api/web/app.js`（全局重写为 SPA，+1562 行/-1213 行）:
  - 任务详情面板完整实现

### Fixed

- **BUG 61**: 下载重试(download_service)中 `saveDumper`/`saveJsonDumper` 使用 load-merge 模式，导致已成功重试并 Remove 的推文重新出现在错误文件中
  - 修复：改为直接覆盖写入，不再加载旧数据合并
- **BUG 62**: `RetryAllFailed` 的 dumper Load 与 `saveDumper` 并发写入产生竞态，导致读取到部分写入的损坏 JSON
  - 修复：Load 操作置于 dumperMu 锁保护下
- **Issue A**: `JsonFileDownload`/`JsonFolderDownload` 中 jsonDumper.Load() 无锁，与 `replaceJsonDumper` 的锁内写入竞态
  - 修复：Load 操作置于 dumperMu 锁保护下
- **Issue B**: `ClearErrors` 中 os.Remove 无锁，与 `saveDumper` 锁内写入竞态（saveDumper 可能在 Remove 后重新创建文件）
  - 修复：os.Remove 操作置于 dumperMu 锁保护下
- **调度器停止挂起**: Reload 失败后 `stopLocked` 对 `sc.cancel/sc.ctx` 置 nil，导致 `sc.wg.Wait()` 永不返回
  - 修复：移除 cancel/ctx 的 nil 重置；超时等待改为直接 `sc.wg.Wait()`
- **夏令时计算**: `nextDailyTrigger` 使用 `Add(24*time.Hour)` 在 DST 切换日偏差 1 小时
  - 修复：替换为 `AddDate(0, 0, 1)`，按日历天计算
- **CSS 变量**: 多处使用错误的 `--accent` 变量名，应为 `--accent-primary`
  - 修复：统一替换

## [v3.4.20] - 2026-06-04

### Changed

#### 调度器：随机延迟改为确定性 FNV-1a 哈希
- `internal/scheduler/scheduler.go`:
  - 将 `math/rand` 替换为 `hash/fnv`，启动延迟由纯随机改为基于 entry ID 的确定性 FNV-1a 哈希值取模，避免多任务同时期扎堆
  - `randomIntervalDelay` 新增 `entryID` 参数，空 ID 时回退纯随机（兼容测试场景）
  - `Stop()` 新增 5 秒超时保护，避免 goroutine 停止卡死
  - `entrySnapshot()` 检测 `sc.ctx` 为 nil 时返回 `context.Background()`，防止空上下文 panic
  - `runDailyLoop` 遇到空 times 时提前返回，跳过日志
  - 导出 `ScheduleIDPattern` 供外部复用
  - 新增 User/Following 类型校验 `screen_name` 合法性
  - 中文描述 "每天" → "每 24 小时"，消除歧义

#### API 路由：PUT → PATCH
- `internal/api/server.go`:
  - 6 个 DB 更新端点由 `PUT` 改为 `PATCH`：`/db/users/{id}`、`/db/lists/{id}`、`/db/user-entities/{id}`、`/db/list-entities/{id}`、`/db/user-links/{id}`
  - 关闭超时从 5 秒延长至 30 秒
  - `Shutdown()` 中增加 2 秒延迟再关闭 DB，确保处理中的 handler 和 detached goroutine 完成

#### 新增 API 路由
- `internal/api/server.go`:
  - `GET /api/v1/db/users/{id}/entities` - 查询指定用户的所有 entities
  - `GET /api/v1/db/users/{id}/links` - 查询指定用户的所有 links
  - `GET /api/v1/db/lists/{id}/entities` - 查询指定列表的所有 entities
  - `GET /api/v1/db/stats` - 数据库统计概览（各表记录数）
  - `GET /api/v1/queue/status` - 下载队列状态（pending/held/detached 数）
  - `POST /api/v1/schedules/trigger-all` - 手动触发所有调度
  - `GET /api/v1/schedules/stats` - 调度统计

#### 下载队列增强
- `internal/api/download_queue.go`:
  - 新增 `close` 状态检查，关闭队列上阻止后续操作
  - 新增 `Status()` 方法返回 pending/held/detached 三级队列计数

#### 下载器：版本管理 + 质量升级
- `internal/downloader/version_manager.go` - 新增 `CleanupOldVersions()` 方法，按修改时间清理过期版本备份（当前无调用方，预留给定时清理）
- `internal/downloading/tweet_json_converter.go` - `cleanMediaHighQuality` 重命名为 `upgradeMediaToHighQuality`，语义更明确
- `internal/downloading/json_folder_download.go` - 将 `fmt.Sscanf` 替换为 `strconv.ParseUint`，提供更可靠的 uint64 解析和错误日志

#### 数据库迁移增强
- `internal/database/parent_dir_migration.go` - 空字符串视为无需迁移，跳过处理
- `internal/database/sqlite_schema.go` - 新增 `requiredSchema` 注释说明，明确声明预期表结构

#### 错误处理与日志改进
- `internal/downloading/entity.go` - 符号链接失败时输出 restore rename 的具体错误详情
- `internal/api/event_bus.go` - 慢订阅者警告日志打印具体溢出数量

### Documentation

- 全面同步 10+ 个文档文件与代码库一致（API_DOCUMENTATION.md、SERVICE_LAYER.md、call_chain_analysis_report.md、readme.md 等）
- 新增 CLAUDE.md（AI 编码规范）和用户状态变更日志

### Fixed

- 修复 SSE 慢订阅者日志格式（未显示具体订阅者数量）
- 修复 JSON 文件夹下载中 `fmt.Sscanf` 的 uint64 解析问题
- 修复符号链接失败时错误信息遗漏 restore 失败详情

## [v3.4.19] - 2026-06-04

### Added

#### 任务管理增强：统计、批量取消、删除、重试
- `internal/api/task_manager.go` - 新增 4 个方法：
  - `GetTaskStats()` - 按状态（queued/running/completed/failed/cancelled）统计任务数量，用于前端概览展示
  - `CancelQueuedTasks()` - 批量取消所有排队中（queued）的任务，返回取消数量，通过 eventBus 发布通知
  - `DeleteTask(id)` - 删除指定终端状态（completed/failed/cancelled）任务，返回 DeleteTaskResult
  - `RetryTask(id)` - 基于失败或取消的原始任务创建新任务（克隆 taskData），返回新 Task
  - 清理周期从 8 小时延长至 24 小时，减少已完成任务过早被清理
- `internal/api/task_types.go` - 新增 `DeleteTaskResult`（deleted/not_found/not_deletable）和 `RetryTaskResult`（success/not_found/not_retryable）类型
- `internal/api/server.go` - 注册新路由：`GET /api/v1/tasks/stats`、`POST /api/v1/tasks/cancel-queued`、`POST /api/v1/tasks/{task_id}/retry`、`DELETE /api/v1/tasks/{task_id}`
- `internal/api/server_test.go` - 新增 `TestHandleTaskStats`、`TestHandleCancelQueuedTasks`、`TestHandleDeleteTask`、`TestHandleRetryTask` 等端到端测试（+308 行）

#### 批量标记下载（Batch Mark）
- `internal/api/download_handlers.go` - 新增 `handleBatchMark` 处理器：
  - 接收 `BatchMarkDownloadedTaskData`，支持同时标记用户、列表、关注列表
  - 内部使用 `buildTaskRunFunc` 统一构建任务运行函数
  - 路由：`POST /api/v1/batch/mark`
- `internal/api/types.go` - 新增 `BatchMarkDownloadedTaskData` 结构体，包含 `Users`、`Lists`、`FollowingNames`、`Timestamp` 字段
- `internal/api/task_manager.go` - `cloneTaskData` 新增 `BatchMarkDownloadedTaskData` 分支的深拷贝支持

#### 失败推文管理：重试全部 & 清除记录
- `internal/service/download_service.go` - 新增 2 个方法：
  - `RetryAllFailed()` - 重试所有历史失败推文（先重试常规下载错误，再重试 JSON 导入错误），兼容两种 dumper
  - `ClearErrors()` - 清除所有失败推文记录文件（errors.json + json_errors.json）
- `internal/service/interfaces.go` - `DownloadService` 接口新增 `RetryAllFailed()` 和 `ClearErrors()`
- `internal/downloading/dumper.go` - 新增方法：
  - `TweetDumper.Summary()` - 返回每个 entity 的失败推文计数（map[int]int）
  - `JsonTweetDumper.Summary()` - 返回每个来源文件的失败推文摘要（[]JsonDumpSummary）
  - `JsonDumpSummary` 结构体：包含 `SourcePath`、`Type`、`Count`
- `internal/api/types.go` - 新增 `ErrorSummaryResponse`，含 `Regular`（entity ID → count）和 `JSON`（来源文件摘要）
- `internal/api/server.go` - 注册新路由：`GET /api/v1/errors`、`POST /api/v1/retry/failed`、`DELETE /api/v1/errors`

#### JSON 下载并发限流与上传标记
- `internal/downloading/json_file_download.go` - `DownloadThirdPartyTweets` 新增信号量（semaphore）并发控制：
  - 限制同时处理的 JSON 文件数不超过 `maxDownloadRoutine`
  - 避免大量文件同时启动 goroutine 导致内部 BatchDownloadTweet 并发叠加
- `internal/api/types.go` - `JsonFileDownloadTaskData` 和 `JsonFolderDownloadTaskData` 新增 `FromUpload` 字段：
  - 标记任务来源是否为上传，上传文件在任务结束后会被清理，不可重试
- `internal/api/download_handlers.go` - `handleJsonFileUpload` 和 `handleJsonFolderUpload`：
  - 设置 `FromUpload: true`
  - 初始化失败时清理已创建的 uploadDir

#### 用户历史名称查询增强
- `internal/database/query.go` - 新增 `QueryUserPreviousNames()` 函数：
  - 支持 `currentName` 参数按当前名称（screen_name 或 name）筛选
  - 返回 `UserPreviousNameWithCurrent` 结构，含 `current_screen_name` 和 `current_name`
- `internal/database/model.go` - 新增 `UserPreviousNameWithCurrent` 结构体，含 `CurrentScreenName` 和 `CurrentName` 字段
- `internal/api/types.go` - `DBUserPreviousNameItem` 新增 `CurrentScreenName` 和 `CurrentName` 字段
- `internal/api/server.go` - 注册路由 `GET /api/v1/db/user-previous-names`
- `internal/api/server_test.go` - 新增相关测试用例

#### 下载处理器重构：统一任务构建
- `internal/api/download_handlers.go` - 大规模重构（+481 行）：
  - 新增 `buildTaskRunFunc(task)` 方法：根据 Task 类型自动构建运行函数，集中分发到对应的 DownloadService 方法
  - 所有下载处理器入口（handleUserDownload、handleListDownload、handleFollowingDownload、handleBatchDownload、handleUserProfile、handleListProfile、handleUserMark、handleListMark、handleFollowingMark、handleJsonFileDownload、handleJsonFileUpload、handleJsonFolderDownload、handleJsonFolderUpload）统一使用 `buildTaskRunFunc` + `buildTaskOptions` 模式
  - 消除每个处理器中的重复 `service.DownloadOptions{}` 构造和闭包定义
  - `enqueueTask` 在 `downloadQueue == nil` 时正确设置任务错误状态
- `internal/api/download_queue.go` - 新增 `Wakeup()` 方法：唤醒可能正在 cond.Wait 上休眠的 worker，用于任务取消后让 worker 检查清理

### Changed

#### API-Version 响应头
- `internal/api/server.go` - 新增 API-Version 响应头中间件，在所有 API 响应中添加 `API-Version: v1` 头部，为未来版本迁移预留

### Fixed

#### Dumper 并发安全修复
- `internal/service/download_service.go` - `executeDownloadTemplate` 和 `BatchDownload` 中 `dumper.Load()` 调用前后添加 `s.dumperMu.Lock()/Unlock()`，修复并发写入 dumper 时的竞态条件

### Web UI

#### 日志统计与导出功能
- `internal/api/log_handlers.go` - 新增 2 个处理器：
  - `handleLogStats` - 按日志级别统计计数（debug/info/warn/error）
  - `handleLogExport` - 提供完整日志文件下载
- `internal/api/server.go` - 注册路由：`GET /api/v1/logs/stats`、`GET /api/v1/logs/export`
- `internal/api/web/app.js` - 日志页面新增：
  - 级别过滤按钮显示各级别计数（如 `DEBUG (3)`、`INFO (42)`）
  - 新增"📥 导出"按钮，调用 `api.downloadLogExport()` 下载完整日志
  - 日志搜索输入新增 300ms 防抖（debounce），减少输入过程中的请求频率
  - 修复日志分页 total 上限（min(total+1, 1000)），防止溢出
  - 新增 `_logsPageLoaded` 标志位，防止日志页面重复加载
- `internal/api/web/styles.css` - 日志相关样式优化

#### 编辑器初始化竞态条件修复（CodeMirror）
- `internal/api/web/app.js` - 修复配置和额外账户页面 CodeMirror 编辑器初始化竞态：
  - 新增 `_configCmInitializing` 和 `_cookiesCmInitializing` 标志位，防止重复初始化
  - `initConfigCodeMirror()` / `initCookiesCodeMirror()` 增加初始化中检查和容器存在性检查
  - 页面切换清理逻辑中重置标志位
  - 系统标签页选中改用 `data-tab` 属性而非 `textContent` 比较，更可靠
- `internal/api/web/styles.css` - 系统面板 `.card` 和 `.card-body` 新增 flex 布局，CodeMirror 自适应高度

#### 配置/额外账户/定时任务编辑器全高度布局
- `internal/api/web/app.js` - `renderConfigEditor()`、`renderCookiesEditor()`、`renderScheduleViewer()`：
  - 外部包裹 `flex-col + height:100%` 容器，使编辑器面板占满可用空间
  - CodeMirror 高度从固定 `400px` 改为 `100%`，自适应容器
  - `card-body` 使用 `flex-col + overflow:hidden`，`config-hint` 设为 `flex-shrink:0`
  - 定时任务 schedule raw editor 同理适配
- `internal/api/web/styles.css` - 新增样式：
  - `.system-panel .card` / `.card-body`：flex 弹性布局 + overflow 控制
  - `.system-panel .CodeMirror`：`height: 100% !important`
  - `.system-panel .CodeMirror-scroll`：移除 `max-height` 限制
  - `.mode-tab`：新增 `flex-shrink:0` 防止被压缩

#### 定时任务表单 UI 改进
- `internal/api/web/app.js` - `renderScheduleForm()`：
  - 新增"类型"选择器（`sf_type`）下拉框
  - "启用"复选框移到表单字段内（`config-group-title` 移至 `config-field` 区域），所有 checkbox label 使用 `inline-flex` 布局
  - 各配置项标签（auto_follow、follow_members 等）改为 `display:inline-flex;align-items:center;gap:4px` 对齐

### Stats

- **25 个文件变更**
- **+2,100 行 / -254 行**

***

## [v3.4.18] - 2026-05-30

### Changed

#### CI/CD：Docker 构建修复 - 版本号注入
- `.github/workflows/docker.yml` - 新增 2 行：
  - `build-push-action` 新增 `build-args: VERSION=${{ github.ref_name }}`，将 git tag（如 `v3.4.18`）作为 `VERSION` ARG 传入 Dockerfile
  - 修复此前所有 Docker 镜像版本号始终显示 `dev` 的问题，现在 `tmd -version` 正确输出实际版本号

#### Web UI：全高度弹性布局重构
- `internal/api/web/app.js` - 修改 86 行：
  - **概览页**：新增 `.overview-container` 外层容器，使用 `flex-col + full-height`；"快速下载"卡设为 `flex-shrink:0`，"最近任务"卡使用 `flex:1` 占满剩余空间，空状态列表 `overflow-y:auto` 独立滚动
  - **任务页**：`.tasks-layout` 两侧卡片均使用 `flex:1; display:flex; flex-direction:column; overflow:hidden` 撑满父容器，"创建新任务"卡 `card-body` 使用 `flex:1; overflow-y:auto` 独立滚动；`.toolbar` 设为 `flex-shrink:0` 防止被压缩；任务列表/空状态 `flex:1; overflow-y:auto`
  - **数据页**：卡片整体 `height:calc(100vh - ...)`，`card-header` 和 `pagination` 设为 `flex-shrink:0`，`card-body` 使用 `flex:1; overflow:hidden; flex-column`
  - **日志页**：卡片同数据页布局，`log-container` 通过 CSS `flex:1; overflow-y:auto` 独立滚动而不依赖 JS 固定高度
  - **定时任务页**：卡片使用全高度 flex，`card-header` `flex-shrink:0`，`card-body` 内新增 `.schedule-list` 包裹任务项列表做独立滚动
  - **定时任务滚动位置保持**：`render()` 在重渲染前记录 `.schedule-list` 的 `scrollTop`，渲染后通过 `requestAnimationFrame` 恢复，避免 SSE 推送/状态变更导致的页面跳动
  - 移除 `.tasks-layout` 内部旧的 card flex 样式（已改为行内 style 统一管理）

- `internal/api/web/styles.css` - 修改 48 行：
  - `content-wrapper` 新增 `overflow: hidden`，阻止外层出现双滚动条
  - 新增 `.overview-container`：flex-column 全高度布局（使概览页所有卡片撑满视口）
  - 新增 `.overview-tasks-list`：`flex:1; overflow-y:auto` 概览页任务列表独立滚动
  - `.table-scroll-container` 新增 `flex:1; overflow-y:auto` 内置滚动
  - `.task-list` / `.empty-state` 新增 `flex:1; overflow-y:auto` 撑满父容器并独立滚动
  - `.tasks-layout` 改用 `height: calc(100vh - ...)` 固定高度而非靠内部撑开
  - `.log-container` 新增 `flex:1; overflow-y:auto` 替代旧的 JS `max-height` 控制
  - 新增 `.schedule-list`：`flex:1; overflow-y:auto` 定时任务列表独立滚动区

#### 文件版本管理器修复
- `internal/downloader/version_manager.go` - 修改 11 行：
  - `CreateVersion()` 中将 `os.ReadFile(sourcePath)` 提前到 `os.MkdirAll()` 之前执行
  - 逻辑意义：源文件不存在时直接报错返回，避免先创建空目录再失败，目录不会被残留

#### README 文档清理
- `readme.md` - 删除 64 行：
  - 移除"场景 4：数据库锁定错误"（不再常见的问题）
  - 移除"场景 6：Profile 下载跳过所有用户"（此为正常行为无需单独说明）
  - 移除"获取帮助"章节（含指向旧 repo `unkmonster/tmd` 的 Issues/Discussions 链接）
  - 场景编号重新编排：5 → 4

### Stats

- **5 个文件变更**
- **+92 行 / -119 行**

***

## [v3.4.17] - 2026-05-30

### Changed

#### 配置加载与写入流程重构
- `internal/config/config.go` - 修改 188 行，重构配置模块核心逻辑：
  - 新增 `StartupLoadResult` 结构体，封装启动配置加载的完整结果（含环境变量回退、交互式提示等状态标记）
  - 提取 `normalizeRootPath()` 函数，统一 RootPath 的 trim、mkdir、abs 路径处理（原内联在 FieldDef.Setter 中）
  - 新增 `ParseConfYAML()` 函数：使用 `yaml.NewDecoder` + `KnownFields(true)` 严格模式解析 YAML，拒绝未知字段；解析后自动调用 `NormalizeLoadedConf` 做运行时字段归一化
  - 新增 `MarshalConf()` 函数：序列化前先做归一化，确保写出的 YAML 始终是干净格式
  - 新增 `NormalizeLoadedConf()` 函数：统一对 RootPath 做 abs 路径转换和 trim、对 ProxyURL 做格式校验与规范化
  - 新增 `Validate()` 函数（从 main.go 迁入）：配置非空性校验，错误信息引导用户检查 conf.yaml 或 TMD_ROOT_PATH 环境变量
  - 新增 `LoadStartupConfig()` / `loadStartupConfig()` 函数（从 main.go 迁入）：封装启动时配置文件读取→缺失处理→环境变量覆盖的完整流程，支持依赖注入 promptFn 便于测试
  - 改造 `ReadConf()`：先 ReadFile 再 ParseConfYAML，读与解耦分离
  - 改造 `WriteConf()`：先 MarshalConf 再 writeFileAtomic，写出内容始终经过归一化
  - 重构 `writeYAMLFile()` → `writeFileAtomic()`：改为原子写入（先写 .tmp 文件 → chmod → fsync → rename），防止写入中断导致配置损坏

- `internal/config/config_test.go` - 新增 140 行测试：
  - `TestReadConfNormalizesRuntimeFields`：验证 ReadConf 自动创建目录并返回 abs 路径、trim ProxyURL 空格
  - `TestReadConfRejectsUnknownFields`：验证 KnownFields(true) 模式下非法字段名被拒绝
  - `TestParseConfYAMLRejectsInvalidProxyURL`：验证无效代理 URL scheme 在解析阶段即报错
  - `TestWriteConfPersistsNormalizedConfig`：验证 WriteConf 写出的文件经 ReadConf 读回后字段已被归一化
  - `TestLoadStartupConfigUsesEnvFallbackWhenConfigMissing`：配置文件不存在且有 ENV 时走 fallback 不触发 prompt
  - `TestLoadStartupConfigAppliesEnvOverridesToExistingConfig`：已有配置文件时 ENV 覆盖仍生效
  - `TestLoadStartupConfigPromptModeUsesPromptFunction`：prompt=true 时调用自定义 promptFn
  - `TestValidateRejectsMissingRootPath` / `TestValidateRejectsNilConfig`：Validate 边界用例

#### API 层适配新配置流程
- `internal/api/config_handlers.go` - 修改 22 行：
  - 移除直接 `yaml.Unmarshal` / `yaml.Marshal` 调用，改用 `config.ParseConfYAML()` 和 `config.MarshalConf()`
  - `handleUpdateConfigRaw` 中保存配置改用 `config.WriteConf(testConf)` 替代直接 `os.WriteFile`，确保持久化的配置经过归一化
  - YAML 预览响应也使用 `config.MarshalConf()` 生成，保证前端看到的是标准化后的内容

- `internal/api/server_test.go` - 新增 59 行测试：
  - `TestServer_UpdateConfigRawRejectsInvalidSemanticConfig`：验证语义级配置校验（如非法 proxy_url scheme）在 API 层正确拒绝并返回 400
  - `TestServer_UpdateConfigRawPersistsNormalizedConfig`：验证通过 raw editor 保存的配置经过归一化后持久化，ProxyURL 空格被去除、RootPath 转为 abs 路径

#### 主程序简化
- `main.go` - 删除 52 行：
  - 将 ~40 行的内联配置加载/缺失处理/ENV 覆盖逻辑替换为单行 `config.LoadStartupConfig()` 调用
  - 将 `validateConfig()` 内联函数删除，改用 `config.Validate()`
  - 根据 `loadResult.UsedEnvFallback` / `loadResult.EnvApplied` 分别输出对应日志

- `main_test.go` - 修改 6 行：测试中 `validateConfig` 调用点改为 `config.Validate`

#### 调度器生命周期与重载辅助重构
- `internal/scheduler/scheduler.go` - 修改 175 行：
  - 提取 `startLocked(skipIfRunning bool) (int, bool)` 方法：将 Start() 中的 hasEverStarted/firstStart 标记设置 + 启动去重判断 + goroutine 启动封装为独立方法，供 Reload 恢复路径复用（skipIfRunning=false 允许重启）
  - 提取 `stopLocked() bool` 方法：将 Stop() 中的停止逻辑（取消 context → 等待 goroutine → 清理状态）封装为独立方法，返回是否实际执行了停止操作
  - 提取 `applyConfig(entries, parsed, statuses)` 方法：将 Reload() 中的 entries/parsed/statuses 交换 + generation++ 封装
  - 提取 `statusChangeSnapshot()` / `statusChangeSnapshotLocked()` 方法：将分散在各处的「加锁 → 复制 statuses → 取 callback → 解锁」模式统一抽取，callback 为 nil 时直接跳过复制
  - 提取 `notifyStatusChange(callback, statuses)` 自由函数：统一回调通知逻辑
  - 提取 `logReloadSummary(entries)` 自由函数：Reload 结尾的活跃调度计数日志
  - **Reload 错误恢复改进**：当 readConfig 失败且调度器之前处于运行状态时，现在能正确恢复启动（之前因代码重复导致 firstStart/hasEverStarted 标记遗漏），并通过 `startLocked(false)` 强制重新启动

- `internal/scheduler/scheduler_test.go` - 新增 72 行测试：
  - `TestStopIsIdempotent`：连续两次 Stop() 后调度器仍保持停止状态，不会 panic 或状态异常
  - `TestReloadRecoversAfterConfigError`：写入非法 schedules.yaml 后 Reload 返回 error，但调度器仍保持运行状态且原有 statuses 不丢失

#### Web UI：系统日志独立页面 & 导航重构
- `internal/api/web/app.js` - 修改 92 行：
  - **日志页面独立**：从 System 页面的 logs 子标签页提升为独立的 `/logs` 一级路由页面（`pages.logs()` 直接渲染 `renderLogViewer()`）
  - **SSE 刷新适配**：`sseManager.onReconnect` 新增 `'logs'` 分支，日志页面断线重连时自动 reload
  - **导航系统更新**：路由表 `parseRoute()` / `updateURL()` 新增 `'logs': '/logs'` 映射；页面标题映射新增 `logs: '系统日志'`
  - **清理逻辑调整**：`navigateTo()` 和 `onpopstate` 中离开 logs 页面时同样触发 cleanupSystemTimers + CodeMirror 销毁（与 system 页面同等对待）
  - **System 标签精简**：移除 system 页面 tabs 中的"系统日志"标签及对应的 `systemLogsPanel` DOM；`setSystemTab()` 不再需要在切换到其他 tab 时 stopLogStream
  - **渲染与订阅分离**：日志相关的 state 变更（logs/logLevel/logPagination）从 system 页面的 subscribe 分支中移出，改为独立的 `currentPage === 'logs'` 分支触发 render()
  - **函数重命名**：`syncLogsTabView()` → `syncLogsPageView()`，反映其不再作为子标签调用的定位
  - **移动端遮罩层**：导航到 schedules/logs 页面时同步关闭 sidebarOverlay
  - **SSE 指示器**：点击刷新按钮在 logs 页面时触发 `loadLogs()`
  - **定时任务表格**：移除固定高度滚动约束 `max-height:calc(100vh - 280px);overflow-y:auto`

- `internal/api/web/index.html` - 修改 19 行：
  - 侧边栏导航新增"📋 系统日志" nav-item（data-page="logs"），位于"数据管理"和"应用配置"之间
  - 移动端底部导航同步新增日志入口
  - "系统"菜单项图标从 📌 改为 🔧，文字从"系统"改为"应用配置"
  - 新增 `<div class="sidebar-overlay" id="sidebarOverlay">` 移动端侧边栏遮罩层

- `internal/api/web/styles.css` - 修改 27 行：
  - 新增 `.sidebar-overlay` 样式：fixed 全屏半透明黑色背景 (z-index:99)，配合 `.open` 类控制显隐过渡动画
  - `.table-scroll-container`：移除 `max-height` 和 `overflow-y: auto`（定时任务表格不再限制固定高度）
  - `.log-container`：移除 `max-height:500px` 和 `overflow-y:auto`（日志容器不再截断高度）
  - `.mobile-nav-items`：布局从 `flex` + `justify-content:space-around` 改为 `grid; grid-template-columns:repeat(6,1fr)` 以容纳新增的第 6 个导航项
  - `.mobile-nav-item`：`min-width` 从 64px 改为 0，新增 `text-align:center` 适应 grid 布局

### Stats

- **11 个文件变更**
- **+637 行 / -215 行**

***

### Changed

#### 事件总线重构
- `internal/api/event_bus.go` - 修改 197 行，重构事件总线，添加更多事件类型和处理逻辑
- `internal/api/event_bus_test.go` - 新增 74 行，添加事件总线测试

#### 日志处理优化
- `internal/api/log_handlers.go` - 删除 61 行，移除旧的日志处理器

#### SSE 功能拆分
- `internal/api/sse.go` - 删除 95 行，移除旧的 SSE 实现
- `internal/api/sse_logs.go`（新增文件）- 新增 71 行，创建 SSE 日志流实现
- `internal/api/sse_tasks.go`（新增文件）- 新增 139 行，创建 SSE 任务流实现
- `internal/api/sse_test.go` - 新增 48 行，添加 SSE 测试

#### 任务管理器重构
- `internal/api/task_manager.go` - 删除 113 行，移除任务类型定义
- `internal/api/task_types.go`（新增文件）- 新增 108 行，创建任务类型定义文件

#### 控制台日志中心增强
- `internal/consolelog/hub.go` - 修改 163 行，增强控制台日志中心功能
- `internal/consolelog/hub_test.go` - 修改 46 行，更新控制台日志中心测试

### Stats

- **11 个文件变更**
- **+818 行 / -297 行**

***

## [v3.4.15] - 2026-05-15

### Changed

#### GitHub Actions 工作流优化
- `.github/workflows/docker.yml` - 删除 1 行，移除 master 分支触发，只在标签推送时构建 Docker 镜像
- `.github/workflows/go.yml` - 删除 76 行，移除 master 分支和 PR 触发，只在标签推送时运行 CI

#### 事件总线增强
- `internal/api/event_bus.go` - 修改 77 行，增强事件总线功能，添加更多事件类型和处理逻辑
- `internal/api/event_bus_test.go` - 修改 21 行，更新事件总线测试

#### 日志处理优化
- `internal/api/log_handlers.go` - 修改 21 行，优化日志处理器

#### SSE 功能增强
- `internal/api/sse.go` - 修改 26 行，增强 SSE（Server-Sent Events）功能
- `internal/api/sse_test.go` - 新增 19 行，添加 SSE 测试

#### Web 界面优化
- `internal/api/web/app.js` - 修改 21 行，优化前端 JavaScript 逻辑

#### CLI 参数解析重构
- `internal/cli/args.go` - 修改 83 行，重构命令行参数解析逻辑，添加更多参数支持
- `internal/cli/args_test.go` - 修改 43 行，更新参数解析测试
- `internal/cli/executor_test.go` - 新增 90 行，添加执行器测试

#### 推文下载优化
- `internal/downloading/tweet_download.go` - 修改 7 行，优化推文下载逻辑

#### 命名规则清理
- `internal/naming/tweet_naming.go` - 删除 2 行，清理命名规则代码

#### 路径存储重构
- `internal/path/store.go` - 修改 36 行，重构路径存储逻辑，优化 Root 路径处理
- `internal/path/store_test.go` - 修改 101 行，更新路径存储测试，添加更多测试用例

#### 下载服务更新
- `internal/service/download_service.go` - 修改 16 行，更新错误文件路径引用（ErrorJ → ErrorsPath, JsonErrorJ → JSONErrorsPath）

#### 工具函数重构
- `internal/utils/fs.go` - 删除 28 行，移除 Twitter 图片质量参数函数
- `internal/utils/twitter_media.go`（新增文件）- 新增 14 行，创建 Twitter 媒体处理工具函数
- `internal/utils/user.go` - 修改 9 行，优化用户相关工具函数，改进图片质量处理
- `internal/utils/utils_test.go` - 修改 17 行，更新工具函数测试

#### 主程序重构
- `main.go` - 修改 106 行，重构主程序：
  - 提取 bootstrap 参数解析为独立函数 `parseBootstrapArgs`
  - 添加配置验证函数 `validateConfig`
  - 优化端口解析和验证逻辑
  - 改进错误处理
- `main_test.go`（新增文件）- 新增 76 行，添加主程序测试：
  - 测试参数解析
  - 测试端口验证
  - 测试配置验证

### Stats

- **22 个文件变更**
- **+644 行 / -246 行**

***

## [v3.4.14] - 2026-05-15

### Changed

#### Docker 工作流优化
- `.github/workflows/docker.yml` - 删除 2 行配置，简化 Docker 工作流

#### 批量下载重构
- `internal/downloading/batch_any.go` - 修改 4 行，优化批量下载任意类型推文逻辑
- `internal/downloading/batch_any_test.go` - 修改 26 行，更新批量下载测试用例
- `internal/downloading/batch_download.go` - 修改 9 行，重构批量下载核心逻辑
- `internal/downloading/batch_download_test.go` - 修改 18 行，更新批量下载测试

#### JSON 下载优化
- `internal/downloading/json_file_download.go` - 修改 3 行，优化第三方推文 JSON 下载
- `internal/downloading/json_folder_download.go` - 修改 4 行，优化 Loong Tweet 文件夹下载

#### Profile 下载增强
- `internal/downloading/profile/downloader.go` - 修改 7 行，增强 Profile 图片下载器，优化头像 URL 质量处理
- `internal/downloading/profile/types.go` - 新增 5 行，扩展 Profile 类型定义
- `internal/downloading/profile/types_test.go` - 修改 17 行，更新 Profile 类型测试

#### 重试机制优化
- `internal/downloading/retry.go` - 修改 8 行，优化重试逻辑
- `internal/downloading/retry_test.go` - 修改 4 行，更新重试测试

#### 推文下载改进
- `internal/downloading/tweet_download.go` - 修改 5 行，优化推文下载逻辑
- `internal/downloading/tweet_download_test.go` - 修改 14 行，简化推文下载测试

#### 类型系统优化
- `internal/downloading/types.go` - 修改 12 行，优化下载类型定义
- `internal/downloading/types_test.go` - 修改 17 行，更新类型测试

#### 调度器修复
- `internal/scheduler/scheduler.go` - 修改 21 行，修复调度器任务状态处理

#### 下载服务优化
- `internal/service/download_service.go` - 修改 42 行，重构下载服务，优化最大下载协程数处理逻辑

#### 主程序优化
- `main.go` - 修改 7 行，优化配置加载，改进 MaxDownloadRoutine 默认值处理

### Stats

- **19 个文件变更**
- **+141 行 / -84 行**

***

## [v3.4.13] - 2026-05-15

### Fixed

#### 调度器修复与优化
- `internal/scheduler/scheduler.go` - 修复调度器核心问题
  - 修复 `Stop()` 方法的互斥锁泄漏问题
  - 去重 goroutine 启动，防止重复执行
  - 修复 OOB (Out of Bounds) 风险
  - 重构调度器核心逻辑，提升稳定性

- `internal/scheduler/scheduler_test.go` - 新增 179 行测试代码，增强调度器测试覆盖

#### 同步逻辑修复
- `internal/downloading/list_sync.go` / `user_sync.go`
  - 修复 `syncUser` 未定义引用问题
  - 清理过时的目标代码
  - 简化列表同步逻辑

#### 下载死锁修复
- `internal/downloading/tweet_download.go`
  - 修复 `BatchDownloadTweet` 和 `BatchUserDownload` 的死锁问题
  - 修复 `tweetDownloader` panic 恢复机制中的死锁

### Changed

#### 代码重构与清理
- 修复乱码文本 (mojibake)
- 提取通用工具函数
- 简化抽象层

#### API 性能优化
- `internal/api/task_manager.go` - 优化 TaskManager
  - 引入快照缓存机制提升性能
  - 清理冗余代码

#### 其他改进
- `internal/path/store.go` - 路径存储优化
- `internal/utils/fs.go` / `user.go` - 工具函数增强
- `readme.md` / `doc/API_DOCUMENTATION.md` - 文档更新
- `docker-compose.yml` - 配置调整
- 新增 `tools/check_recent_json_name_params.py` - JSON 命名参数检查工具

### Stats

- **32 个文件变更**
- **+824 行 / -386 行**

***

## [v3.4.12] - 2026-05-15

### Added

#### Docker 工作流增强
- `.github/workflows/docker.yml` - 新增 Docker Hub 镜像推送支持
  - 镜像名称配置扩展：同时支持 GHCR (`ghcr.io/leeexx2001/tmd`) 和 Docker Hub (`docker.io/leeexx00/tmd`)
  - 添加 Docker Hub 登录步骤 (`docker/login-action@v3`)
  - 更新 metadata-action 配置以支持多镜像推送

### Changed

#### Web 界面优化
- `internal/api/web/app.js` - 定时任务表格区域添加滚动支持
  - 添加最大高度限制 (`max-height: calc(100vh - 280px)`)
  - 添加垂直滚动 (`overflow-y: auto`)
  - 改善长列表的显示体验

#### 代码格式优化
- `internal/service/download_service.go` - 统一代码格式
  - 统一 `downloadTemplateConfig` 结构体字段对齐
  - 统一 `UserDownload`、`ListDownload`、`FollowingDownload` 函数调用参数对齐
  - 纯格式化改动，无功能变更

### Stats

- **3 个文件变更**
- **+29 行 / -20 行**

***

## [v3.4.11] - 2026-05-15

### Changed

#### 下载服务重构 - 模板方法模式
- `internal/service/download_service.go` - 重构下载服务核心逻辑
  - 新增 `downloadTemplateConfig` 结构体，封装下载流程模板方法的差异点配置
    - `Prepare` - 准备阶段回调，返回用户、列表、显式 Profile 用户
    - `ReportBeforeDownload` - 下载前报告回调
    - `ShouldDownloadProfile` - 是否下载 Profile 的判断函数
    - `ProfileIdentifier` - Profile 标识符（用于日志）
    - `CompletionMessage` - 完成消息
  - 新增 `appendUsers` 辅助函数，合并用户切片
  - 新增 `executeDownloadTemplate` 模板方法核心
    - 统一处理路径初始化、Dumper 管理、下载执行、重试逻辑、Profile 下载、任务完成
    - 通过配置回调实现不同下载类型的差异化行为
  - 重构 `UserDownload`、`ListDownload`、`FollowingDownload`
    - 使用模板方法，各函数代码从 ~80-90 行简化到 ~20 行
    - 消除重复代码（路径初始化、Dumper 管理、重试逻辑、Profile 下载）
    - 统一错误处理和日志输出
    - 提高代码可维护性和可测试性

### Stats

- **1 个文件变更**
- **+136 行 / -160 行**

***

## [v3.4.10] - 2026-05-15

### Added

#### JSON 下载错误持久化与重试机制
- `internal/downloading/dumper.go` - 新增 `JsonTweetDumper` 结构体
  - 支持 JSON 下载错误持久化到 `json_errors.json`
  - 实现 `PushWithDir`、`Load`、`Dump`、`Merge`、`Count`、`Clear` 方法
  - 按源路径、条目类型、目录组织失败推文
  - 支持 Load-then-Merge 模式避免数据丢失

- `internal/downloading/dumper_test.go` - 新增 `JsonTweetDumper` 完整单元测试（237 行）
  - 覆盖 Push、Load、Dump、Merge、Clear 等所有操作

- `internal/downloading/retry.go` - 新增 `RetryFailedJsonTweets` 函数
  - 支持 JSON 下载失败推文的重试机制
  - 实现进度回调和详细日志输出
  - 区分成功下载和非可重试跳过的情况

- `internal/downloading/json_file_download.go`
  - `DownloadThirdPartyTweets` 返回失败推文映射 `failedBySource`
  - 支持错误收集用于后续重试

- `internal/downloading/json_folder_download.go`
  - `DownloadFromLoongTweetFolder` 返回失败推文映射
  - 新增 `JsonPackagedTweet` 结构体包含目录信息

- `internal/service/download_service.go`
  - `JsonFileDownload` 和 `JsonFolderDownload` 集成重试机制
  - 新增 `collectJsonFailedTweets` 收集失败推文到 dumper
  - 新增 `saveJsonDumper` 保存错误状态（Load-then-Merge 模式）

- `internal/path/store.go` - 新增 `JsonErrorJ` 路径（`json_errors.json`）

### Changed

#### 路径存储优化
- `internal/path/store.go` - 调整数据库文件名为 `tmd.db`

#### 错误处理改进
- `main.go` - 初始化失败时输出错误信息到 stderr

#### 文档更新
- `CLAUDE.md` - 重构为开发助手规范文档
  - 添加任务管理、代码审查、发布流程等规范
  - 包含 Git 工作流、代码风格、工具使用指南

### Stats

- **9 个文件变更**
- **+629 行 / -77 行**

***

## [v3.4.9] - 2026-05-15

### Added

#### 数据库转换工具
- `convert_db_to_legacy.py` (新文件，177 行) - 将新版本 tmd 的 SQLite 数据库转换为旧版本 tmd-2.4.4 兼容格式
  - 处理 schema 差异：
    - users 表：丢弃新版 is_accessible 列
    - user_previous_names 表：列名 user_id → uid
    - lsts 表：列名 owner_user_id → owner_uid
  - lst_entities、user_entities、user_links 表结构一致，直接复制
  - 命令行用法：`python convert_db_to_legacy.py <新版db路径> <输出旧版db路径>`

#### TweetDumper 增强
- `internal/downloading/dumper.go` - 新增 `Remove` 方法
  - 支持从 dumper 中删除指定推文
  - 从 set 和 data 中同时删除
  - 自动清理空实体，更新计数器
- `internal/downloading/dumper_test.go` - 新增 `Remove` 方法完整单元测试

### Changed

#### Web 界面版本号显示优化
- `internal/api/web/app.js` - 概览页面状态卡片版本号显示移除 "v" 前缀
- `internal/api/web/index.html` - Sidebar 版本号格式改为 "TMD Pro <版本号>"（移除 "v" 前缀）

#### 重试机制优化
- `internal/downloading/retry.go` - 重构 `RetryFailedTweets` 函数
  - 使用 `Remove` 方法替代 `Clear`，精确移除已成功重试的推文
  - 改进进度报告，使用原子计数器和 `dumper.Count()`
  - 优化日志输出，区分成功下载和非可重试跳过的情况
  - 修复无推文需要重试时的空操作问题

### Stats

- **6 个文件变更**
- **+295 行 / -13 行**

***

## [v3.4.8] - 2026-05-15

### Changed

#### Web 界面重构与优化
- `internal/api/web/app.js` - 重构事件处理机制
  - 使用事件委托替代内联 onclick，减少内存泄漏风险
  - 任务列表项点击事件改为事件委托模式
  - 移除多个页面的刷新按钮（任务、数据、日志、定时任务），统一使用 SSE 指示器点击刷新
  - 简化搜索框交互，移除 onkeypress 事件，改为全局 Enter 键监听
  - 简化任务项渲染，移除内联事件处理
  - 额外账户和定时任务表单添加分隔线
  - 定时任务表单简化类型标签显示

- `internal/api/web/index.html`
  - 版本号默认值改为 `--`
  - SSE 指示器添加点击刷新提示
  - 移除刷新按钮

- `internal/api/web/styles.css`
  - 移除 breadcrumb 样式
  - SSE 指示器改为可点击（cursor: pointer）
  - 优化 config-group-title 样式，字体变大加粗
  - 新增 config-divider 和 config-label 样式
  - 简化 tag-running 样式（移除动画，改为左边框）

### Added

#### 启动脚本增强
- `start-server.bat` - 支持 `tmd-Windows-amd64.exe` 可执行文件

### Fixed

#### 错误处理改进
- `main.go` - 初始化路径失败时从 `Fatalln` 改为 `Warnln`，允许优雅降级
- `main.go` - 添加 nil 检查防止空指针异常

### Stats

- **5 个文件变更**
- **+86 行 / -81 行**

***

## [v3.4.7] - 2026-05-15

### Added

#### Profile 下载用户去重
- `internal/service/download_service.go` - 新增 `dedupeProfileUsers` 函数，对 Profile 下载用户列表进行去重
  - 优先按用户 ID 去重
  - 其次按 ScreenName（不区分大小写）去重
  - 处理 nil 用户和无效用户的情况
- `internal/service/download_service_test.go` - 新增去重功能单元测试

### Changed

#### Web 界面优化
- `internal/api/web/index.html` - Sidebar 版本号改为动态获取，添加 `appVersion` span 元素
- `internal/api/web/app.js` - 在 `setState` 中自动更新 sidebar 版本号显示
- `internal/api/web/app.js` - 配置页面 (`renderConfigForm`) 密码输入框移除 `config-mask-hint` div，提示整合到 placeholder
- `internal/api/web/app.js` - 额外账户页面 (`renderCookiesForm`) 密码输入框同样优化 placeholder 显示

### Stats

- **4 个文件变更**
- **+99 行 / -9 行**

***

## [v3.4.6] - 2026-05-15

### Changed

#### 配置验证增强
- `internal/config/config.go` - 新增配置字段验证：MaxRetries 限制在 0-10，MaxConcurrency/BatchSize/ProfileTimeout 必须大于 0

#### 日志系统改进
- `internal/consolelog/hub.go` - 新增线程安全的 `Clear()` 方法，支持日志历史管理和清理
- `internal/consolelog/hub_test.go` - 新增完整测试覆盖

#### 数据库层优化
- `internal/database/helpers.go` - 新增通用数据库辅助函数
- `internal/database/lst.go`, `lst_entity.go`, `user_entity.go`, `user_link.go` - 优化实体操作
- `internal/database/parent_dir_migration.go` - 改进迁移逻辑

#### 下载功能修复
- `internal/downloading/batch_download.go` - 修复空指针检查，优化并发下载文件名冲突处理
- `internal/downloading/tweet_download.go` - 优化推文下载逻辑
- `internal/downloading/types.go` - 新增错误类型标记

#### 命名规则改进
- `internal/naming/base.go`, `list_naming.go`, `tweet_naming.go`, `user_naming.go` - 优化命名规则，增强边界情况处理

#### 工具函数增强
- `internal/utils/algo.go` - `Heap.Pop()` 现在返回被弹出的值
- `internal/utils/fs.go` - 新增 `UniquePathResolver` 支持并发安全的唯一路径分配，优化 `WinFileNameWithMaxLen`
- `internal/utils/http.go` - `StripAvatarSuffix` 改用正则表达式实现
- `internal/utils/recovery.go` - panic 恢复时增加堆栈跟踪输出
- `internal/utils/win32.go` - 修复 `SetConsoleTitle` Windows API 调用返回值判断

#### 测试改进
- 多个测试文件使用 `t.TempDir()` 替代硬编码路径
- 新增大量单元测试，提升测试稳定性和可靠性

### Fixed
- 修复测试中使用硬编码路径导致的不稳定性
- 修复 Windows 控制台标题设置的错误处理
- 修复批量下载中的并发文件名冲突问题

### Stats

- **31 个文件变更**
- **+623 行 / -202 行**

***

## [v3.4.5] - 2026-05-09

### Added

#### 新增基础设施层

| 文件 | 说明 |
|------|------|
| `internal/downloader/downloader.go` | 新增通用下载器基础设施，支持单文件下载、流式下载、大小校验 |
| `internal/downloader/file_writer.go` | 原子写入、跳过未变化文件、并发锁管理 |
| `internal/downloader/version_manager.go` | 版本备份管理 |
| `internal/naming/base.go` | 命名服务基础接口 |
| `internal/utils/user.go` | 用户相关工具函数 |

### Changed

#### 架构重构

| 文件 | 变更 |
|------|------|
| `internal/downloading/entity.go` | 实体处理逻辑优化，适配新的基础设施层 |
| `internal/downloading/list_sync.go` | 列表同步逻辑优化 |
| `internal/downloading/tweet_download.go` | 推文下载逻辑优化 |
| `internal/downloading/json_file_download.go` | JSON 文件下载优化 |
| `internal/downloading/json_folder_download.go` | JSON 文件夹下载优化 |

#### API 层优化

| 文件 | 变更 |
|------|------|
| `internal/api/config_handlers.go` | 配置处理器优化 |
| `internal/api/cookie_handlers.go` | Cookie 处理器优化 |
| `internal/api/db_handlers.go` | 数据库处理器优化 |
| `internal/api/download_handlers.go` | 下载处理器优化 |
| `internal/api/handlers.go` | 通用处理器优化 |
| `internal/api/log_handlers.go` | 日志处理器优化 |
| `internal/api/scheduler_handlers.go` | 调度器处理器优化 |
| `internal/api/server.go` | 服务器配置优化 |
| `internal/api/sse.go` | SSE 增强，支持更多事件类型 |
| `internal/api/task_manager.go` | 任务管理器优化 |
| `internal/api/types.go` | 类型定义优化 |

#### CLI 层增强

| 文件 | 变更 |
|------|------|
| `internal/cli/args.go` | 新增参数解析功能 |
| `internal/cli/executor.go` | 执行器优化 |

#### Service 层调整

| 文件 | 变更 |
|------|------|
| `internal/service/deps.go` | 依赖注入优化 |
| `internal/service/download_service.go` | 下载服务优化 |
| `internal/service/interfaces.go` | 接口定义调整 |

#### CI/CD 和部署

| 文件 | 变更 |
|------|------|
| `.github/workflows/go.yml` | GitHub Actions 工作流优化 |
| `Dockerfile` | Docker 构建优化 |
| `start.bat` / `start.sh` | 移除旧启动脚本 |
| `start-server.bat` | 新增 Windows Server 模式启动脚本 |

#### 文档更新

| 文件 | 变更 |
|------|------|
| `readme.md` | 架构图更新，目录结构调整 |

### Added

#### 测试增强

| 文件 | 说明 |
|------|------|
| `internal/api/db_handlers_test.go` | 新增数据库 handler 测试 |
| `internal/api/handlers_test.go` | 新增 handler 测试 |
| `internal/api/server_test.go` | 新增服务器测试 |
| `internal/api/task_manager_test.go` | 新增任务管理器测试 |
| `internal/api/types_test.go` | 新增类型测试 |
| `internal/cli/args_test.go` | 新增参数解析测试 |
| `internal/cli/executor_test.go` | 新增执行器测试 |
| `internal/downloading/entity_test.go` | 新增实体测试 |
| `internal/downloading/list_sync_test.go` | 新增列表同步测试 |
| `internal/downloading/tweet_download_test.go` | 新增推文下载测试 |
| `internal/service/deps_test.go` | 新增依赖注入测试 |
| `internal/service/download_service_test.go` | 新增下载服务测试 |

### Stats

- **38 个文件变更**
- **+1091 行 / -340 行**

***

## [v3.4.4] - 2026-05-09

### Changed

#### 数据库层优化

| 文件 | 变更 |
|------|------|
| `internal/database/user_entity.go` | `UpdateUserEntityTweetStat` 改为条件更新，只更新更大的 `latest_release_time` 和 `media_count` |
| `internal/database/user_link.go` | `CreateUserLink` 使用 `INSERT OR IGNORE` 避免重复插入错误，返回已存在的记录 |

#### 下载服务优化

| 文件 | 变更 |
|------|------|
| `internal/service/download_service.go` | `saveDumper` 支持合并已有文件，避免数据丢失 |
| `internal/service/download_service.go` | 添加 `dumperMu` 互斥锁保证并发写入安全 |
| `internal/downloading/dumper.go` | 新增 `Merge` 方法合并两个 Dumper 的数据 |

#### Profile 下载优化

| 文件 | 变更 |
|------|------|
| `internal/downloading/profile/types.go` | 文件下载超时从 2 分钟缩短至 40 秒 |

#### API 层优化

| 文件 | 变更 |
|------|------|
| `internal/api/download_handlers.go` | 下载处理器优化 |
| `internal/api/task_manager.go` | 任务管理器优化 |
| `internal/api/types.go` | 类型定义优化 |
| `internal/api/server.go` | 服务器配置优化 |

### Fixed

#### 测试修复

| 文件 | 变更 |
|------|------|
| `internal/database/test/user_link_test.go` | 修复 `CreateUserLink` 重复插入测试期望 |
| `internal/database/test/user_entity_test.go` | 更新测试用例 |

### Added

#### 测试增强

| 文件 | 说明 |
|------|------|
| `internal/downloading/dumper_test.go` | 新增 `TweetDumper_Merge` 测试 |
| `internal/service/download_service_test.go` | 新增 `saveDumper` 合并测试 |
| `internal/service/download_service_test.go` | 新增 `saveDumper` 并发写入测试 |
| `internal/downloading/profile/types_test.go` | 新增超时配置测试 |
| `internal/api/server_test.go` | 新增 API 测试用例 |
| `internal/api/task_manager_test.go` | 新增任务管理器测试 |

### Stats

- **16 个文件变更**
- **+601 行 / -111 行**

***

## [v3.4.3] - 2026-05-09

### Fixed

#### 批量下载进度统计修复

| 文件 | 变更 |
|------|------|
| `internal/downloading/batch_download.go` | 修复按用户维度统计完成数的逻辑，新增 `userProgressState` 跟踪每个用户的进度 |
| `internal/downloading/batch_download.go` | 新增 `completedUsers` 原子计数器，正确计算已完成用户数 |
| `internal/downloading/batch_download.go` | 修复 `markUserDone` 在错误路径的调用，确保失败用户也被统计 |

#### 下载回调接口调整

| 文件 | 变更 |
|------|------|
| `internal/downloading/types.go` | `onTweetDone` 回调签名从 `func(tweet *twitter.Tweet, failed bool)` 改为 `func(pt PackagedTweet, failed bool)` |
| `internal/downloading/tweet_download.go` | 适配新的回调签名，传递 `PackagedTweet` 以获取实体信息 |
| `internal/downloading/retry.go` | 适配新的回调签名 |
| `internal/downloading/tweet_download_test.go` | 更新测试用例适配新签名 |

#### 日志与界面文案统一

| 文件 | 变更 |
|------|------|
| `internal/service/progress.go` | 日志字段名统一：`failed` → `Failedtweet`，`versioned` → `versionedfile` |
| `internal/service/progress_test.go` | 更新测试期望输出 |
| `internal/api/web/app.js` | Web 界面统计文案同步更新 |

### Stats

- **8 个文件变更**
- **+79 行 / -28 行**

***

## [v3.4.2] - 2026-05-09

### Added

#### Profile 下载超时与进度上报

| 文件 | 变更 |
|------|------|
| `internal/downloading/profile/downloader.go` | 新增 downloadCtx 超时机制，推进日志上下文 |
| `internal/downloading/profile/types.go` | 添加 `downloadTimeout` 和 `progressInterval` |
| `internal/downloading/profile/downloader_test.go` | 新增超时取消和进度上报测试 |

#### 调度器批量更新

| 文件 | 变更 |
|------|------|
| `internal/scheduler/scheduler.go` | 支持批量更新（替换式同步），新增 `BulkUpdate` 方法 |
| `internal/scheduler/scheduler_test.go` | 新增批量更新测试用例 |
| `internal/scheduler/types.go` | Request 新增 `BulkUpdate` 字段 |
| `internal/api/scheduler_handlers.go` | 新增 `handleBulkUpdate` 处理器 |
| `internal/api/server.go` | 注册 `POST /schedules/bulk` 路由 |
| `internal/api/server_test.go` | 新增批量更新端到端测试 |

#### 下载支持 auto_follow 参数

| 文件 | 变更 |
|------|------|
| `internal/api/download_handlers.go` | 用户下载和列表下载支持 `auto_follow` 参数 |
| `internal/service/download_service.go` | 新增 `auto_follow` 字段传递 |
| `internal/service/download_service_test.go` | 新增相关测试 |

### Changed

#### Tampermonkey 脚本重写

| 文件 | 说明 |
|------|------|
| `tools/tmd-download-button.user.js` | 重构为异步/等待模式，支持 Toast 通知和主题自适配 |

**功能改进：**
- 替换按钮状态文字为 Toast 通知（更友好的用户反馈）
- 自动检测 Twitter/X 暗黑/亮色模式并适配按钮样式
- 按钮位置从关注按钮旁改为"更多"按钮旁，更稳定
- 使用 `data-testid` 属性检测 DOM，更可靠
- 添加 `auto_follow: true` 参数支持自动关注
- 改进错误处理和用户反馈信息

#### API 文档更新

| 文件 | 变更 |
|------|------|
| `doc/API_DOCUMENTATION.md` | 新增调度器批量更新接口说明 |
| `doc/API_DOCUMENTATION.md` | 更新下载参数文档（auto_follow 等） |

#### Web 界面调度器优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 调度器 UI 适配批量更新和混合 API |
| `internal/api/web/index.html` | 页面结构调整 |
| `internal/api/web/styles.css` | 样式微调 |

### Stats

- **18 个文件变更（15 修改 + 3 新增）**
- **+1520 行 / -577 行**

***

## [3.4.1] - 2026-05-09

### Added

#### Tampermonkey 用户脚本

| 文件 | 说明 |
|------|------|
| `tools/tmd-download-button.user.js` | 新增 Tampermonkey 用户脚本，在 Twitter/X 个人资料页面添加 TMD 下载按钮 |

**功能特性：**
- 在 Twitter/X 个人资料页面的关注按钮旁添加下载按钮
- 点击按钮可直接将用户推送到 TMD 下载队列
- 支持状态反馈（加载中、成功、错误）
- 自动适配页面布局和主题

### Fixed

#### API 文档与 CORS 配置修复

| 文件 | 变更 |
|------|------|
| `doc/API_DOCUMENTATION.md` | 修复 `/health` 接口响应格式，添加 `success` 和 `data` 包装 |
| `internal/api/server.go` | CORS 允许方法列表添加 `PATCH` |

#### Web 界面优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | 任务统计卡片添加 `data-overview-stat` 属性，支持独立更新 |
| `internal/api/web/app.js` | 任务计数副标题添加 `data-task-count-subtitle` 属性 |
| `internal/api/web/app.js` | 新增 `getTaskStats` 和 `updateOverviewStatsUI` 函数 |
| `internal/api/web/app.js` | 优化空状态和任务列表的 DOM 结构，修复切换时的样式问题 |

#### 文档更新

| 文件 | 变更 |
|------|------|
| `doc/call_chain_analysis_report.md` | 路由描述更新为 `http.ServeMux` |
| `doc/call_chain_analysis_report.md` | CORS 中间件方法列表添加 `PATCH` |
| `doc/call_chain_analysis_report.md` | 请求大小限制状态更新 |

### Stats

- **5 个文件变更（4 修改 + 1 新增）**
- **+508 行 / -34 行**

***

## [3.4.0] - 2026-05-09

### Added

#### Docker 部署与发布支持

| 文件 | 变更 |
|------|------|
| `Dockerfile` | 新增多阶段 Docker 构建，运行镜像内置 `ca-certificates` 和 `tzdata` |
| `docker-compose.yml` | 新增本地编排示例，固定 `/config` 和 `/data` 卷约定 |
| `.dockerignore` | 新增 Docker 构建上下文排除规则 |
| `.github/workflows/docker.yml` | 新增 GHCR 自动构建并发布多架构镜像的 workflow |

#### 跨平台信号处理

| 文件 | 变更 |
|------|------|
| `signal_unix.go` | 新增 Unix 信号定义，处理 `SIGHUP`、`SIGINT`、`SIGTERM`、`SIGQUIT` |
| `signal_windows.go` | 新增 Windows 信号定义，处理 `os.Interrupt` 和 `SIGTERM` |

### Changed

#### 配置与环境变量覆盖

| 文件 | 变更 |
|------|------|
| `main.go` | 新增 `TMD_HOME` 和 `TMD_PORT` 支持，启动时可直接使用环境变量配置 |
| `internal/config/config.go` | 新增 `ApplyEnv` / `HasEnvOverrides`，支持 `TMD_ROOT_PATH`、cookies、代理、并发和文件名长度覆盖 |
| `internal/config/config_test.go` | 新增环境变量覆盖、错误回滚和空值行为测试 |

#### CI/CD 与发布流程

| 文件 | 变更 |
|------|------|
| `.github/workflows/go.yml` | 三平台 CI 收口为 `CGO_ENABLED=0` 构建/测试，并拆分 tag 发布流程 |
| `Dockerfile` | 构建阶段改为跟随 `buildx` 的 `TARGETOS` / `TARGETARCH`，修复多架构镜像实际产物不一致问题 |
| `readme.md` | 新增 GHCR 拉取、`docker run`、`docker-compose` 使用示例，并更新版本到 `3.4.0` |
| `.gitignore` | 新增 `covprofile`、`config/`、`data/` 忽略规则 |

### Stats

- **13 个文件变更**
- **Docker / GHCR / CI 发布链路已接通**

***

## [3.3.11] - 2026-05-04

### Changed

#### SQLite 驱动迁移：从 mattn/go-sqlite3 到 modernc.org/sqlite

| 文件 | 变更 |
|------|------|
| `go.mod` | 移除 `github.com/mattn/go-sqlite3`，新增 `modernc.org/sqlite` |
| `go.sum` | 更新依赖校验和 |
| `main.go` | 移除 CGO sqlite3 导入 |
| `internal/database/connect.go` | 使用 `database.DriverName` 和 `database.MemoryDSN` |
| `internal/database/tx/manager.go` | 事务管理器新增 context 检查 |
| `internal/twitter/client.go` | MFQ 日志优化，记录 rate_limited 和 errors 计数 |
| 所有测试文件 | 统一使用 `database.DriverName` 和 `database.MemoryDSN/MustFileDSN` |

#### 优势

- **纯 Go 实现**：不再需要 CGO，支持 `CGO_ENABLED=0` 编译
- **跨平台简化**：交叉编译无需 C 交叉编译器
- **构建简化**：Windows 不再需要安装 MinGW-w64

#### 文档更新

| 文件 | 变更 |
|------|------|
| `AGENTS.md` | 更新 Go 1.25.0 和 modernc.org/sqlite 说明 |
| `readme.md` | 更新编译说明，移除 CGO 要求 |
| `.github/workflows/go.yml` | 更新 CI 配置 |

#### 清理

| 文件 | 说明 |
|------|------|
| `doc/foo.db 技术文档.md` | 删除过时的数据库文档 |

### Stats

- **28 个文件变更**
- **+227 行 / -124 行**

***

## [3.3.10] - 2026-05-04

### Changed

#### 上传限制调整

| 文件 | 变更 |
|------|------|
| `internal/api/download_handlers.go` | 单文件最大上传大小从 50MB 提升至 400MB |
| `internal/api/download_handlers.go` | 请求总大小限制从 200MB 提升至 1GB |

#### 文档更新

| 文件 | 变更 |
|------|------|
| `doc/API_DOCUMENTATION.md` | 新增 multipart/form-data 上传方式详细说明 |
| `doc/API_DOCUMENTATION.md` | 明确两种请求模式（multipart vs JSON Body） |
| `doc/API_DOCUMENTATION.md` | 添加 curl 上传示例 |

#### 启动脚本移除

| 文件 | 说明 |
|------|------|
| `start.bat` | 移除 Windows 启动脚本（用户可直接运行二进制） |
| `start.sh` | 移除 Linux 启动脚本（用户可直接运行二进制） |

### Stats

- **4 个文件变更**
- **+159 行 / -84 行**

***

## [3.3.9] - 2026-05-04

### Added

#### JSON 文件上传功能

| 文件 | 变更 |
|------|------|
| `internal/api/download_handlers.go` | 新增 `handleJsonFileDownloadMultipart` 处理文件上传 |
| `internal/api/download_handlers.go` | 新增 `handleJsonFolderDownloadMultipart` 处理文件夹上传 |
| `internal/api/download_handlers.go` | 新增 `validateUploadFile` 验证上传文件 |
| `internal/api/download_handlers.go` | 新增 `uniqueUploadFileName` 避免文件名冲突 |
| `internal/api/download_handlers.go` | 新增 `copyUploadedFile` 复制上传文件 |
| `internal/api/server_test.go` | 新增 multipart 上传测试用例 |
| `internal/api/web/app.js` | JSON 下载任务支持文件上传 |
| `internal/api/web/app.js` | JSON 文件夹任务支持文件上传 |
| `internal/api/web/app.js` | 新增 `upload` API 方法支持 FormData |

#### 功能说明

- **JSON 文件上传**：支持多选上传第三方工具导出的 JSON 文件
- **JSON 文件夹上传**：支持多选上传 LoongTweet 生成的 JSON 文件
- **文件名冲突处理**：自动添加序号避免覆盖（如 `tweets-2.json`）
- **文件验证**：仅允许 `.json` 扩展名，拒绝非法文件名
- **错误处理**：上传失败不删除已创建目录

### Changed

#### Web 界面优化

| 文件 | 变更 |
|------|------|
| `internal/api/web/app.js` | JSON 任务表单新增文件上传控件 |
| `internal/api/web/app.js` | 支持文件上传和服务端路径两种模式 |
| `internal/api/web/app.js` | 新增 `readTextareaLines` 辅助函数 |

### Stats

- **3 个文件变更**
- **+537 行 / -18 行**

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

