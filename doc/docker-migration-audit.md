# TMD Docker 迁移审查与落地参考

> 审查日期: 2026-05-01  
> 审查基线: 当前工作区，HEAD `832d38c`，包含尚未提交的本地修复  
> 目的: 为后续 Docker 化提供可执行清单。每个判断都附带当前代码佐证，避免把泛泛的跨平台建议误当迁移阻塞项。

## 结论

当前项目可以作为普通主机进程运行，但还没有达到“开箱即用 Docker 服务”的状态。真正需要优先处理的是构建链、配置注入、数据卷约定和 Windows 宿主机卷上的 symlink 降级。Server 健康检查和优雅关闭已经具备，不应再列为阻塞项。

建议先做一个保守 Docker 版本: 保留 `mattn/go-sqlite3`，使用 Debian/Ubuntu 系构建阶段安装 C 工具链，运行阶段用 slim 镜像。等容器运行契约稳定后，再评估是否替换纯 Go SQLite。

## 目标运行契约

建议 Docker 版本明确以下契约:

```text
配置目录: /config        -> 宿主机持久化卷，保存 conf.yaml、additional_cookies.yaml、日志
下载目录: /data          -> 宿主机持久化卷，对应 conf.yaml 中的 root_path
服务端口: 25556          -> 可通过环境变量或命令行覆盖
运行模式: tmd -server    -> 容器内默认只运行 API Server/Web UI
```

当前代码已经把“应用配置目录”和“下载数据目录”分开，但还没有提供 Docker 友好的入口方式。

代码佐证:

```go
// main.go
appRootPath := filepath.Join(homepath, ".tmd2")
confPath := filepath.Join(appRootPath, "conf.yaml")
additionalCookiesPath := filepath.Join(appRootPath, "additional_cookies.yaml")
```

```go
// internal/path/store.go
sp.Users = filepath.Join(root, "users")
sp.Data = filepath.Join(root, ".data")
sp.DB = filepath.Join(sp.Data, "foo.db")
sp.ErrorJ = filepath.Join(sp.Data, "errors.json")
```

## 必须处理

### 1. 缺少 Docker 构建与编排文件

当前仓库根目录未提供 `Dockerfile`、`docker-compose.yml`、`.dockerignore`。这不是代码 bug，但会阻止后续形成稳定部署方式。

代码/仓库佐证:

```powershell
Get-ChildItem -Path . -Force -Name Dockerfile,docker-compose.yml,docker-compose.yaml,.dockerignore
# 当前没有匹配文件
```

建议:

- 新增 `Dockerfile`，先使用多阶段构建。
- 新增 `.dockerignore`，排除 `.git`、下载数据、日志、临时文件。
- 新增 `docker-compose.yml` 示例，固定 `/config` 和 `/data` 两个卷。

首版 Dockerfile 不建议立刻追求 `scratch`。当前 SQLite 依赖 CGO，先用 Debian slim 更稳。

### 2. SQLite 使用 CGO，Docker 构建必须安装 C 工具链

`mattn/go-sqlite3` 是 CGO driver。当前程序在 `main.go` 和测试里都注册了该 driver，数据库连接使用 driver name `sqlite3`。因此 Docker 构建阶段必须提供 C 编译器，不能直接 `CGO_ENABLED=0` 静态构建。

代码佐证:

```go
// go.mod
github.com/mattn/go-sqlite3 v1.14.22
```

```go
// main.go
_ "github.com/mattn/go-sqlite3"
```

```go
// internal/database/connect.go
dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&busy_timeout=2147483647", path)
db, err := sqlx.Connect("sqlite3", dsn)
```

CI 也显式使用 CGO:

```yaml
# .github/workflows/go.yml
CGO_ENABLED=1 go build ...
```

建议:

- Phase 1: 保留 `mattn/go-sqlite3`，Docker build stage 用 `golang:1.25-bookworm` 并安装 `gcc`/`libc6-dev`。
- Phase 2: 如果要做 distroless/scratch 或跨平台单机交叉编译，再评估 `modernc.org/sqlite`。这一步不能只改 import，必须完整验证 DSN、WAL、时间类型、并发、迁移和全部数据库测试。

验证要求:

```bash
go test ./internal/database/... ./internal/service ./internal/api ./...
docker build .
docker run --rm -v tmd-config:/config -v tmd-data:/data ...
```

### 3. 应用配置目录不支持 Docker 显式覆盖

当前应用配置目录由系统环境推导: Windows 用 `appdata`，其他平台用 `HOME`，再拼 `.tmd2`。容器内这通常会落到 `/root/.tmd2`，不利于清晰挂载，也不支持 `TMD_HOME=/config` 这类部署约定。

代码佐证:

```go
// main.go
if runtime.GOOS == "windows" {
    homepath = os.Getenv("appdata")
} else {
    homepath = os.Getenv("HOME")
}
if homepath == "" {
    panic("failed to get home path from env")
}

appRootPath := filepath.Join(homepath, ".tmd2")
```

建议:

- 增加 `TMD_HOME`，优先级高于平台默认目录。
- 非 Windows 平台可考虑 `XDG_CONFIG_HOME` 或 `os.UserConfigDir()`，但 Docker 首选仍应是 `TMD_HOME=/config`。
- 不建议 fallback 到 `/tmp`，配置和 cookies 是持久数据，不应落入临时目录。

建议规则:

```text
TMD_HOME > os.UserConfigDir()/平台默认 > 明确报错
```

Docker 目标:

```yaml
environment:
  TMD_HOME: /config
volumes:
  - ./config:/config
```

### 4. 配置项不支持环境变量注入

当前配置读取是 `conf.yaml` 加交互式初始化，API/Web 可以编辑配置文件，但 Docker Compose/Kubernetes 常用的环境变量覆盖还不存在。

代码佐证:

```go
// internal/config/config.go
type Config struct {
    RootPath           string `yaml:"root_path"`
    Cookie             Cookie `yaml:"cookie"`
    MaxDownloadRoutine int    `yaml:"max_download_routine"`
    MaxFileNameLen     int    `yaml:"max_file_name_len"`
    ProxyURL           string `yaml:"proxy_url"`
}
```

```go
// main.go
conf, err := config.ReadConf(confPath)
if os.IsNotExist(err) || confArg {
    conf, err = config.PromptConfig(confPath)
}
```

当前 `internal/config` 没有读取 `TMD_*` 环境变量；`main.go` 中 `os.Getenv` 只用于 home path。

建议新增环境变量覆盖，优先级:

```text
环境变量 > conf.yaml > 交互式初始化/默认值
```

建议变量:

```text
TMD_ROOT_PATH=/data
TMD_AUTH_TOKEN=...
TMD_CT0=...
TMD_PROXY_URL=http://host.docker.internal:7897
TMD_MAX_DOWNLOAD_ROUTINE=8
TMD_MAX_FILE_NAME_LEN=158
TMD_PORT=25556
```

实现建议:

- 在 `internal/config` 增加 `ApplyEnv(conf *Config) error`。
- `ReadConf` 后调用，不要绕过现有 YAML。
- 对敏感项继续沿用 API 返回脱敏逻辑。

### 5. Server 端口只支持 CLI 参数

当前端口通过 `-port` 解析，默认 `25556`。Docker 可以用 command 覆盖，但 Compose/Kubernetes 下用 `TMD_PORT` 会更自然。

代码佐证:

```go
// main.go
case "-port":
    if i+1 < len(args) {
        serverPort, _ = strconv.Atoi(args[i+1])
        i++
    }
...
if serverPort == 0 {
    serverPort = 25556
}
```

建议:

- 保留 `-port`。
- 增加 `TMD_PORT`，优先级低于 CLI 参数。
- Dockerfile 默认 `EXPOSE 25556`。

### 6. Windows 宿主机或跨平台数据卷上的 symlink 需要降级方案

下载列表目录依赖 `os.Symlink` 建用户链接。Linux 容器内通常能创建 symlink，但如果宿主机是 Windows、卷类型特殊，或用户直接在 Windows 运行，symlink 可能失败。目前失败后主要记录 warning，列表目录访问能力会下降。

代码佐证:

```go
// internal/downloading/entity.go
err = os.Symlink(path, linkpath)
...
if err = os.Symlink(path, newlinkpath); err != nil && !os.IsExist(err) {
    return err
}
```

```go
// internal/downloading/batch_download.go
if err = os.Symlink(upath, linkpath); err == nil || os.IsExist(err) {
    err = database.CreateUserLink(db, curlink)
}
...
log.Warnf("symlink permission denied: %d errors suppressed (run as admin to enable symlinks)", symlinkWarnCount)
```

```go
// internal/downloading/list_sync.go
if err := os.Remove(linkpath); err != nil && !os.IsNotExist(err) {
    log.Warnln("failed to remove symlink:", linkpath, err)
}
```

建议:

- 抽一个 `internal/utils/link` 或 `internal/downloading/link.go`，集中处理 `CreateUserLinkPath(old, new)`。
- Linux/macOS: 继续 symlink。
- Windows: 优先目录 junction；失败时写 `.shortcut` 文本文件记录目标路径。
- Docker 文档要说明: Linux 容器 + Linux 原生 volume 最稳；Windows bind mount 需要验证 symlink 行为。

不建议直接复制目录作为默认降级，媒体目录可能很大，会造成重复占用。

## 需要设计决策

### 7. 下载数据和 SQLite 文件的卷边界

当前 `root_path` 下会自动创建 `users` 和 `.data`，数据库固定在 `.data/foo.db`，失败记录固定在 `.data/errors.json`。

代码佐证:

```go
// internal/path/store.go
sp.Users = filepath.Join(root, "users")
sp.Data = filepath.Join(root, ".data")
sp.DB = filepath.Join(sp.Data, "foo.db")
sp.ErrorJ = filepath.Join(sp.Data, "errors.json")
```

建议:

- Docker 文档明确 `/data` 是完整下载数据卷，不要只挂载 `/data/users`。
- SQLite WAL 会在 `.data` 目录生成伴随文件，必须保证 `.data` 可写且持久化。
- 不建议把 `.data` 和 `users` 分散到不同卷，除非额外设计迁移工具。

### 8. SQLite 路径大小写语义和跨平台迁移

Schema 对 `parent_dir` 使用 `COLLATE NOCASE`，这让路径比较大小写不敏感。Windows 默认较吻合，但 Linux 容器文件系统通常大小写敏感。如果用户从 Windows 迁移已有数据库到 Linux 容器，路径匹配可能出现语义差异。

代码佐证:

```sql
-- internal/database/schema.go
parent_dir VARCHAR NOT NULL COLLATE NOCASE,
...
parent_dir VARCHAR COLLATE NOCASE NOT NULL,
```

相关查询依赖 `parent_dir`:

```go
// internal/database/user_entity.go
stmt := `SELECT * FROM user_entities WHERE user_id=? AND parent_dir=?`
```

建议:

- 不要在 Docker 迁移第一阶段改 schema。这个问题涉及历史数据兼容。
- 文档先声明: 不建议直接把 Windows 旧数据库挂到 Linux 容器后混用不同大小写路径。
- 如果要解决，单独设计“路径规范化/迁移”任务，而不是简单移除 `COLLATE NOCASE`。

### 9. 资源限制需要暴露给容器部署

主媒体下载并发可由配置控制，但 Profile 下载还有独立的包级默认并发。Docker 部署时需要能限制总资源，否则小容器可能被下载任务压满网络、文件描述符或内存。

代码佐证:

```go
// main.go
if conf.MaxDownloadRoutine > 0 {
    downloading.MaxDownloadRoutine = conf.MaxDownloadRoutine
}
```

```go
// internal/downloading/types.go
MaxDownloadRoutine = min(10, runtime.GOMAXPROCS(0)*2)
```

```go
// internal/downloading/batch_download.go
tweetChan := make(chan PackagedTweet, MaxDownloadRoutine)
...
for i := 0; i < MaxDownloadRoutine; i++ {
    go tweetDownloader(...)
}
```

```go
// internal/downloading/profile/downloader.go
var MaxDownloadRoutine = 20
...
numRoutine := min(len(requests), MaxDownloadRoutine)
```

当前 API Server 已有任务级总并发闸门:

```go
// internal/api/server.go
const maxConcurrentDownloadTasks = 1
downloadTaskSlots chan struct{}
```

建议:

- Docker 示例默认 `TMD_MAX_DOWNLOAD_ROUTINE=4` 或 `8`，不要沿用高并发默认值。
- Profile 下载并发应纳入配置或至少文档说明当前固定为 20。
- 如果未来允许 API 多任务并行，需要同步评估 `maxConcurrentDownloadTasks * MaxDownloadRoutine` 的总并发。

### 10. 文件写入和重命名在容器卷上需要验证

文件写入采用临时文件 + `os.Rename` 的原子写入方式。Linux 原生文件系统通常没问题，但 bind mount、网络盘、杀毒软件扫描的 Windows 目录可能出现 rename 失败。

代码佐证:

```go
// internal/downloader/file_writer.go
tempFile, err := os.CreateTemp(dir, ".tmp_*")
...
if err := os.Rename(tempPath, path); err != nil {
    return 0, fmt.Errorf("failed to rename temp file: %w", err)
}
```

建议:

- Docker 首版优先支持本地 volume 或普通 bind mount。
- Windows 宿主机 bind mount 做专门验收: 大文件、重复覆盖、杀毒软件开启场景。
- 如实际出现 rename 错误，再加 Windows/挂载卷重试策略。

### 11. 文件名策略是“跨平台安全子集”，不是平台最宽松策略

当前文件名清理函数命名为 Windows 专用，并移除 Windows 非法字符。作为 Docker/Linux 运行并不构成错误，但会让 Linux 上合法字符也被剔除。

代码佐证:

```go
// internal/utils/fs.go
reWinNonSupport = regexp.MustCompile(`[/\\:*?"<>\|]`)
...
func WinFileNameWithMaxLen(name string, maxLen int) string {
    name = reUrl.ReplaceAllString(name, "")
    name = reWinNonSupport.ReplaceAllString(name, "")
}
```

调用点:

```go
// internal/naming/user_naming.go
sanitized: utils.WinFileNameWithMaxLen(title, MaxFileNameLen)
```

建议:

- Docker 迁移不必先改行为。
- 后续可重命名为 `PortableFileNameWithMaxLen` 或 `SafeFileNameWithMaxLen`。
- 不建议为了 Linux 放宽规则，除非确认第三方工具和 Windows 用户不再共享同一下载目录。

## 已具备能力

### 12. Server 已有健康检查端点

Docker/Kubernetes 可以直接使用 HTTP healthcheck。

代码佐证:

```go
// internal/api/server.go
mux.HandleFunc("GET /api/v1/health", s.handleHealth)
...
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    if err := s.db.Ping(); err != nil {
        s.writeJSON(w, http.StatusServiceUnavailable, NewErrorResponse("Database unavailable"))
        return
    }
}
```

建议 Dockerfile:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s \
  CMD wget -qO- http://127.0.0.1:25556/api/v1/health || exit 1
```

如果最终镜像不带 `wget`/`curl`，改用 Compose healthcheck 或内置小工具。

### 13. Server 已有优雅关闭

旧审查报告中“Server 缺少优雅关闭”已经过时。当前 server 模式收到信号后会走 `GracefulShutdown`，关闭 HTTP server、任务管理器、DB 和 log writer。

代码佐证:

```go
// main.go
signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
startServerSignalHandler(sigChan, server.GracefulShutdown)
```

```go
// internal/api/server.go
func (s *Server) GracefulShutdown(reason string) {
    if s.taskManager != nil {
        s.taskManager.CancelAllTasks()
        s.taskManager.Close()
    }
    if s.httpServer != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = s.httpServer.Shutdown(ctx)
    }
    if s.db != nil {
        _ = s.db.Close()
    }
}
```

Docker 迁移建议:

- Compose 中设置合理 `stop_grace_period`，例如 `30s`。
- 不需要再把“优雅关闭”列为 P0，但要在容器内验收下载中断后的错误记录和任务状态。

### 14. SQLite 连接池已经按单连接收敛

文件型 SQLite 在容器内更怕并发写争用。当前连接池已限制为单连接，并启用 WAL 和 busy timeout。

代码佐证:

```go
// internal/database/connect.go
dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&busy_timeout=2147483647", path)
db.SetMaxOpenConns(1)
db.SetMaxIdleConns(1)
```

建议:

- Docker 文档声明同一个 `/data` 卷只允许运行一个 TMD 实例。
- 不要让多个容器共享同一个 `foo.db` 写入。

### 15. 代理环境变量已有基础支持

Twitter client 支持配置代理，也会读取标准代理环境变量。

代码佐证:

```go
// internal/twitter/client.go
proxyURL, err := http.ProxyFromEnvironment(req)
...
for _, env := range []string{"HTTP_PROXY", "http_proxy"} {
    if value := os.Getenv(env); value != "" {
        return url.Parse(value)
    }
}
```

Docker 建议:

```yaml
environment:
  HTTPS_PROXY: http://host.docker.internal:7897
  HTTP_PROXY: http://host.docker.internal:7897
```

也可以使用 `proxy_url` 配置项，二者优先级需要在最终文档中说明。

## 建议实施路线

### Phase 1: 最小可用 Docker

1. 新增 `TMD_HOME` 和 `TMD_PORT`。
2. 新增 `ApplyEnv`，支持 `TMD_ROOT_PATH=/data`、cookies、并发、代理。
3. 新增 Dockerfile、.dockerignore、docker-compose.yml。
4. 保留 `mattn/go-sqlite3`，使用 CGO 构建。
5. 文档明确 `/config` 和 `/data` 两个 volume。

验收:

```bash
docker compose up -d
curl http://localhost:25556/api/v1/health
docker compose stop
docker compose start
```

### Phase 2: 容器行为完善

1. symlink 降级实现和 Windows bind mount 验收。
2. Profile 下载并发纳入配置。
3. 增加 Docker 运行文档: 初始化配置、cookies、代理、TZ、备份。
4. 验证大文件下载、重复写入、异常中断、SQLite WAL 文件恢复。

### Phase 3: 构建链优化

1. 评估 `modernc.org/sqlite` 或其他纯 Go SQLite driver。
2. 若替换 driver，完整跑数据库迁移测试和真实数据回归。
3. 再考虑 distroless/scratch 镜像。

## Compose 草案

这是目标形态草案，不代表当前代码已经全部支持。

```yaml
services:
  tmd:
    build: .
    command: ["tmd", "-server", "-port", "${TMD_PORT:-25556}"]
    ports:
      - "${TMD_PORT:-25556}:25556"
    environment:
      TMD_HOME: /config
      TMD_ROOT_PATH: /data
      TMD_MAX_DOWNLOAD_ROUTINE: "4"
      TZ: Asia/Shanghai
    volumes:
      - ./config:/config
      - ./data:/data
    stop_grace_period: 30s
```

当前缺口:

- `TMD_HOME` 尚未实现。
- `TMD_ROOT_PATH` 等配置覆盖尚未实现。
- Dockerfile 尚未实现。

## 不建议的做法

- 不建议把配置 fallback 到 `/tmp`，会丢失 cookies 和配置。
- 不建议为了解决非 root 写入而在代码里统一改成 `0777`，应通过 volume owner/UID/GID 解决。
- 不建议第一阶段就替换 SQLite driver，除非愿意投入完整数据库兼容测试。
- 不建议直接复制用户目录作为 symlink 默认降级，媒体目录可能非常大。

## 最终迁移检查表

| 项目 | 当前状态 | Docker 前是否必须 |
|---|---:|---:|
| Dockerfile / compose | 缺失 | 是 |
| `/config` 配置目录 | 缺失 `TMD_HOME` | 是 |
| `/data` 下载数据目录 | 已有 `root_path`，缺 env 覆盖 | 是 |
| 环境变量配置 | 缺失 | 是 |
| CGO SQLite 构建 | 已确认依赖 CGO | 是，先用 CGO 镜像 |
| 健康检查 | 已有 `/api/v1/health` | 已满足 |
| 优雅关闭 | 已有 `GracefulShutdown` | 已满足，需容器验收 |
| symlink 降级 | 缺失 | 建议 Docker 正式发布前做 |
| 单实例数据库约束 | 代码单连接，文档未写 | 是 |
| Profile 并发控制 | 固定 20 | 建议 |
