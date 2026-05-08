# AGENTS.md

面向本仓库的编码 agent 开发指南。目标是让后续开发少走弯路：先理解边界，再做小而准的改动，最后用合适的测试闭环。

本项目是 Go 编写的 Twitter/X Media Downloader，支持 CLI、API Server 和内置 Web 管理界面。CLI 和 API 共享同一套 `internal/service` 应用服务层。

## 1. 工作原则

- 先读代码，再下判断。不要仅凭文件名或 README 推断行为。
- 优先复用现有分层：CLI/API 只做输入输出和任务编排，下载业务放在 `internal/service` 或更下层。
- 改动要能直接追溯到用户请求。不要顺手格式化、重构、删除无关代码。
- 保持现有风格。这个仓库大量代码是直接、显式、偏过程化的 Go，不要为了“优雅”引入复杂抽象。
- 发现需求歧义时先说清楚歧义和可选方案；简单、低风险的问题可以按最保守假设继续推进。
- 不要写预设性扩展。只解决当前问题，不提前设计未来可能用不到的配置、接口或插件点。

## 2. 项目地图

- `main.go`：进程入口。负责全局参数预解析、配置加载、日志、Twitter 登录、数据库连接，并分流到 CLI 或 Server。
- `internal/config`：`conf.yaml`、`additional_cookies.yaml`、交互式配置和 Web 配置字段定义。
- `internal/cli`：CLI 参数解析和任务优先级。新增命令行参数时通常同时改 `args.go` 和 `executor.go`。
- `internal/api`：HTTP API、任务管理、SSE、配置/日志/数据库管理接口、Web 静态文件托管。
- `internal/api/web`：无构建步骤的原生 HTML/CSS/JS Web UI。
- `internal/service`：CLI 和 API 共享的应用服务层。新增下载能力优先从这里建入口。
- `internal/downloading`：用户/列表/关注/JSON 下载、失败重试、标记已下载等业务流程。
- `internal/downloading/profile`：Profile 下载，保存头像、横幅、简介和 `profile.json`。
- `internal/downloader`：单文件下载与写文件，包括 buffer/stream 策略、大小校验、版本备份、跳过未变化文件。
- `internal/twitter`：Twitter/X 登录、GraphQL 参数、响应解析、限流、多账号选择。这里依赖私有接口，改动风险最高。
- `internal/database`：SQLite schema、迁移、CRUD、查询 helper。
- `internal/entity`、`internal/naming`、`internal/path`、`internal/utils`：实体抽象、命名、路径和通用工具。
- `doc`：重要设计说明和历史行为记录。涉及用户名变更、数据库、mark-downloaded、API 时先查对应文档。

## 3. 运行与验证

本仓库使用 Go 1.25.0，并依赖 `modernc.org/sqlite` 作为纯 Go SQLite driver。常规构建和测试应支持 `CGO_ENABLED=0`。

常用命令：

```bash
go test ./...
go test -race -covermode atomic -coverprofile=covprofile ./...
go build -o tmd.exe .
go build -o tmd ./main.go
```

CI 以 `.github/workflows/go.yml` 为准：

- Windows 构建：`go build -o bin/tmd-${{ runner.os }}-amd64.exe -v -ldflags "-w -s" .`
- Unix 构建：`GOARCH=amd64 CGO_ENABLED=0 go build -o bin/tmd-${{ runner.os }}-amd64 -v -ldflags "-w -s" .`
- 非 tag 构建测试：`go test -race -covermode atomic -coverprofile=covprofile ./...`

验证选择：

- 只改单个纯函数或小工具：跑对应包测试。
- 改 CLI/API/service/downloading 任一层：至少跑相关包测试，优先再跑 `go test ./...`。
- 改并发、任务状态、SSE、数据库写入、下载器：优先跑 `go test -race ...`，除非环境不支持。
- 改 Web 静态 UI：启动 server 后用浏览器验证主要页面和控制台错误。

## 4. 常见任务入口

### CLI 参数

入口：

- `internal/cli/args.go`
- `internal/cli/executor.go`

注意：

- `-jsonfile`、`-jsonfolder`、`-mark-downloaded` 有独占/优先级语义，不要随意改变。
- CLI 最终应调用 `service.DownloadService`，不要在 CLI 层直接复制下载逻辑。

### API 任务

入口：

- `internal/api/types.go`
- `internal/api/download_handlers.go`
- `internal/api/server.go`
- `internal/api/task_manager.go`
- `internal/api/progress.go`

注意：

- 长任务必须创建 task 后异步执行。
- 响应统一使用 `NewSuccessResponse` / `NewErrorResponse`。
- 任务状态只允许按 `queued -> running -> completed/failed/cancelled` 推进。
- 新任务要同步考虑 SSE reporter、Web UI 渲染和取消语义。

### 下载行为

入口：

- `internal/service/download_service.go`
- `internal/downloading/batch_any.go`
- `internal/downloading/batch_download.go`
- `internal/downloading/tweet_download.go`
- `internal/downloader/downloader.go`
- `internal/downloader/file_writer.go`

注意：

- `service` 层负责业务编排和进度上报。
- `downloading` 层负责抓推文、同步实体、组织批量下载和重试。
- `downloader` 层只负责单文件下载/写入，不要让它知道用户、列表、推文业务。
- 403/404 类媒体错误当前视为不可重试，不要重新写入 `.data/errors.json`。

### Profile 下载

入口：

- `internal/downloading/profile/downloader.go`
- `internal/downloading/profile/storage.go`
- `internal/service/download_service.go`

注意：

- Profile 文件位于用户目录下 `.loongtweet/.profile/`。
- 版本备份位于 `.loongtweet/.profile/.versions/`。
- Profile 下载会同步用户目录和数据库实体，改目录逻辑时要同时检查数据库记录。

### 数据库与迁移

入口：

- `internal/database/schema.go`
- `internal/database/model.go`
- `internal/database/query.go`
- `internal/database/helpers.go`
- `internal/database/test`

注意：

- SQLite 使用 WAL，连接池最大打开连接数为 1。不要随意提高。
- 新增字段必须写可重复执行的 migration，兼容旧数据库。
- 改 schema 后补数据库测试。
- 用户目录名、历史用户名、`latest_release_time` 和实体表高度相关，改前先读 `doc/用户名变更处理机制.md`、`doc/foo.db 技术文档.md`、`doc/mark-downloaded详解.md`。

### Twitter/X API

入口：

- `internal/twitter/client.go`
- `internal/twitter/api.go`
- `internal/twitter/user.go`
- `internal/twitter/tweet.go`
- `internal/twitter/timeline.go`
- `internal/twitter/list.go`

注意：

- GraphQL endpoint、features、fieldToggles 可能随 X 改版失效。改这里要有明确失败样例或新接口依据。
- 受保护用户应使用主账号；非保护用户优先使用 additional clients 分摊限流。
- 限流逻辑涉及阻塞、重试和多账号选择，改动后重点跑并发/任务相关测试。

### Web UI

入口：

- `internal/api/web/index.html`
- `internal/api/web/app.js`
- `internal/api/web/styles.css`

注意：

- 当前没有 npm、打包器或框架。不要引入前端构建链，除非用户明确要求。
- API 字段变化要同步检查 `app.js` 的任务列表、任务详情、数据管理、系统设置页面。
- 配置和 cookies 页面涉及敏感信息，默认脱敏；raw 编辑模式才显示原始内容。

## 5. 数据和文件兼容性

- 下载根目录由配置 `root_path` 决定。不要在代码里写死用户机器路径。
- 运行状态文件默认在用户目录 `.tmd2` 下，包括配置、日志和额外 cookies。
- 下载数据通常在 `root_path/users`；数据库在 `root_path/.data/foo.db`；失败记录在 `root_path/.data/errors.json`。
- `.loongtweet` JSON/TXT 是用户可见产物，也可能被第三方工具消费。改格式要谨慎。
- 文件命名受 `naming.MaxFileNameLen` 和 Windows 文件名限制影响，改命名规则必须考虑跨平台。
- 软链接在 Windows 可能因权限失败。现有代码会压缩 warning，不要把这类失败升级成全局 fatal。

## 6. 测试策略

优先补贴近改动层级的测试：

- 参数解析：`internal/cli/*_test.go`
- 任务状态/SSE/API handler：`internal/api/*_test.go`
- 服务编排：`internal/service/*_test.go`
- 下载器和写文件：`internal/downloader/*_test.go`
- 下载业务：`internal/downloading/*_test.go`
- 数据库：`internal/database/test/*_test.go`
- 命名/路径/工具：对应包测试

测试应覆盖行为，不要只覆盖实现细节。涉及 bug 修复时，优先先写能复现问题的测试，再修。

## 7. 不要做的事

- 不要提交真实 `auth_token`、`ct0`、cookies、日志或用户下载数据。
- 不要修改或删除用户本地 `.tmd2`、下载目录、数据库文件、日志文件，除非用户明确要求。
- 不要绕过 `service.DownloadService` 在 API 和 CLI 分别实现同一份下载逻辑。
- 不要随意改 Twitter bearer、GraphQL endpoint 或 features，除非正在修相关问题。
- 不要把 server 长任务改成同步 HTTP 请求。
- 不要把不可重试媒体错误放回重试队列。
- 不要无依据改启动脚本退出码语义；`start.bat` 和 `start.sh` 依赖正常退出码为 0。
- 不要做仓库级大格式化。

## 8. 提交前检查

完成改动前至少确认：

- 改动入口是否在正确层级。
- CLI 和 Server 两条路径是否需要同时更新。
- API 类型、handler、task、SSE、Web UI 是否保持一致。
- 数据库改动是否有 migration 和测试。
- 文件/目录命名改动是否考虑旧数据兼容。
- 已运行与改动风险匹配的测试，并在最终回复中说明结果。

如果无法运行测试，要明确说明原因和剩余风险。
