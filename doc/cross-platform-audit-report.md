# TMD 跨平台兼容性审查报告

> **审查日期**: 2026-04-27  
> **项目**: Twitter Media Downloader (tmd) v3.0.3  
> **目标平台**: Linux / macOS / Windows / Docker  
> **审查范围**: 全部源码，重点关注跨平台兼容性和 Docker 部署就绪度

---

## 目录

- [总览](#总览)
- [严重问题 (Critical)](#严重问题-critical)
- [中等问题 (Medium)](#中等问题-medium)
- [低等问题 (Low)](#低等问题-low)
- [已正确处理的部分](#已正确处理的部分)
- [Docker 部署专项检查](#docker-部署专项检查)
- [修复优先级路线图](#修复优先级路线图)

---

## 总览

| 严重级别 | 数量 | 说明 |
|---------|------|------|
| 🔴 严重 | 2 | 阻塞跨平台/Docker 部署的核心问题 |
| 🟡 中等 | 5 | 影响特定平台行为或运维体验 |
| 🟢 低等 | 4 | 代码规范或微小改进 |
| ✅ 已处理 | 3 | 已正确实现跨平台兼容 |

---

## 严重问题 (Critical)

### C1. CGO 依赖 (mattn/go-sqlite3) — 跨平台编译和 Docker 构建的主要障碍

**位置**: [go.mod:10](internal/../../go.mod#L10)

**现状**:
```go
require (
    github.com/mattn/go-sqlite3 v1.14.22
)
```

**问题**:
- `mattn/go-sqlite3` 是 CGO 库，编译必须 `CGO_ENABLED=1`
- 跨平台交叉编译需要对应平台的 C 交叉编译器（如 `x86_64-linux-musl-gcc`）
- Docker 构建镜像必须安装 `gcc` / `musl-dev` / `libc-dev`，导致镜像体积膨胀
- Alpine Linux 需额外安装 `build-base` 和 `musl-dev`
- Windows 交叉编译到 Linux 需要 MinGW 或类似工具链
- CI/CD 中每个平台都需要原生 Runner（当前已是如此，但本地开发体验差）

**影响范围**: 所有平台编译、Docker 镜像构建

**建议方案**:
- 替换为 `modernc.org/sqlite`（纯 Go 实现，无 CGO 依赖）
- 或使用 `github.com/glebarez/go-sqlite3`（mattn/go-sqlite3 的纯 Go 分支）
- 替换后可实现 `CGO_ENABLED=0` 静态编译，Docker 镜像可使用 `FROM scratch` 或 `distroless`

---

### C2. 符号链接 (Symlink) 无降级方案 — Windows 和受限 Linux 环境下功能缺失

**位置**:
- [internal/downloading/entity.go:29](internal/downloading/entity.go#L29)
- [internal/downloading/entity.go:41](internal/downloading/entity.go#L41)
- [internal/downloading/batch_download.go:187](internal/downloading/batch_download.go#L187)
- [internal/downloading/list_sync.go:84](internal/downloading/list_sync.go#L84)

**现状**:
```go
// entity.go:29 — 创建符号链接
err = os.Symlink(path, linkpath)

// batch_download.go:187 — 列表目录中创建用户目录的符号链接
if err = os.Symlink(upath, linkpath); err == nil || os.IsExist(err) {

// batch_download.go:212 — 仅打印警告
log.Warnf("symlink permission denied: %d errors suppressed (run as admin to enable symlinks)", symlinkWarnCount)
```

**问题**:
- **Windows**: 创建符号链接需要管理员权限或开启开发者模式，普通用户运行时符号链接全部失败
- **Docker**: 默认容器内可以创建符号链接，但如果以非 root 用户运行且安全策略限制 `sysctl fs.protected_regular`，也可能失败
- 当前代码在符号链接失败时仅打印警告，**没有降级方案**（如创建目录快捷方式、复制目录等）
- 符号链接失败后，列表目录结构完全缺失，用户无法通过列表目录访问对应用户的媒体文件

**影响范围**: Windows 普通用户、受限 Docker 环境

**建议方案**:
- 方案 A: 在 Windows 上使用目录联接 (Junction) 替代符号链接（不需要管理员权限）
- 方案 B: 符号链接失败时降级为创建空 `.shortcut` 文件记录目标路径
- 方案 C: 符号链接失败时降级为复制目录（占用更多磁盘空间但功能完整）
- 建议优先实现方案 A，对 Windows 最友好

---

## 中等问题 (Medium)

### M1. 主目录路径检测不完善 — Docker 环境下配置路径不灵活

**位置**: [main.go:81-88](main.go#L81-L88)

**现状**:
```go
if runtime.GOOS == "windows" {
    homepath = os.Getenv("appdata")
} else {
    homepath = os.Getenv("HOME")
}
if homepath == "" {
    panic("failed to get home path from env")
}
```

**问题**:
- Docker 最小化镜像（如 `scratch`、`distroless`）可能没有 `HOME` 环境变量
- 不支持 `XDG_CONFIG_HOME` 标准（Linux 桌面规范）
- 不支持自定义环境变量覆盖（如 `TMD_HOME`）
- 直接 `panic` 而非优雅退出
- Docker 中通常期望配置在 `/config` 或 `/app/config` 等固定路径

**建议方案**:
```go
// 优先级: TMD_HOME > XDG_CONFIG_HOME > 平台默认
homepath := os.Getenv("TMD_HOME")
if homepath == "" {
    if runtime.GOOS == "windows" {
        homepath = os.Getenv("appdata")
    } else {
        homepath = os.Getenv("XDG_CONFIG_HOME")
        if homepath == "" {
            homepath = os.Getenv("HOME")
        }
    }
}
if homepath == "" {
    homepath = "/tmp"  // 最终降级
}
```

---

### M2. 信号处理不跨平台 — SIGHUP/SIGQUIT 在 Windows 不存在

**位置**: [main.go:199](main.go#L199)

**现状**:
```go
signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
```

**问题**:
- `syscall.SIGHUP` 和 `syscall.SIGQUIT` 在 Windows 上不存在
- Go 在 Windows 上编译时，这两个常量可能不存在或行为未定义
- Docker 中 `SIGTERM` 是容器停止信号，`SIGHUP` 常用于配置热重载

**建议方案**:
- 使用构建标签分离平台特定信号
- 或使用 `os.Signal` 接口 + 运行时判断

---

### M3. 数据库路径比较使用 COLLATE NOCASE — 跨平台路径语义不一致

**位置**: [internal/database/schema.go:43,53](internal/database/schema.go#L43)

**现状**:
```sql
parent_dir VARCHAR NOT NULL COLLATE NOCASE,
parent_dir VARCHAR COLLATE NOCASE NOT NULL,
```

**问题**:
- `COLLATE NOCASE` 使路径比较不区分大小写，这在 Windows/macOS 上正确（文件系统不区分大小写）
- 但在 Linux/Docker 上，文件系统区分大小写，数据库却做不区分大小写比较
- 如果用户在 Linux 上有 `UserA` 和 `usera` 两个目录，数据库查询会混淆
- 从 Windows 迁移数据到 Linux 时，可能出现路径匹配异常

**建议方案**:
- 根据运行平台动态决定 COLLATE 策略
- 或统一使用 `COLLATE BINARY`（区分大小写），在查询时做规范化处理

---

### M4. Server 模式缺少优雅关闭 — Docker 停止容器时可能丢失数据

**位置**: [internal/api/server.go:122-129](internal/api/server.go#L122-L129)

**现状**:
```go
server := &http.Server{
    Addr:         addr,
    Handler:      handler,
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
}
return server.ListenAndServe()
```

**问题**:
- 没有实现优雅关闭（Graceful Shutdown）
- Docker 发送 `SIGTERM` 后，正在进行的下载任务会被强制中断
- 没有信号监听机制来触发 `server.Shutdown(ctx)`
- 可能导致数据库写入不完整或文件下载中断

**建议方案**:
- 在 `runServer` 中监听系统信号
- 收到 SIGTERM 后调用 `server.Shutdown(ctx)` 等待活跃连接完成
- 配合任务管理器取消正在执行的任务

---

### M5. 配置不支持环境变量 — 不符合 12-Factor App 和 Docker 最佳实践

**位置**: [internal/config/config.go](internal/config/config.go)

**现状**:
- 配置仅通过 YAML 文件 + CLI 交互式输入
- 不支持通过环境变量覆盖配置项
- Docker 部署时必须挂载配置文件或进入容器交互式配置

**建议方案**:
- 支持环境变量覆盖: `TMD_ROOT_PATH`, `TMD_AUTH_TOKEN`, `TMD_CT0`, `TMD_MAX_ROUTINE`, `TMD_MAX_FILENAME_LEN`
- 优先级: 环境变量 > 配置文件 > 默认值
- Docker Compose 示例:
  ```yaml
  environment:
    - TMD_AUTH_TOKEN=xxx
    - TMD_CT0=xxx
    - TMD_ROOT_PATH=/data
  ```

---

## 低等问题 (Low)

### L1. 文件名清理函数命名和逻辑 Windows 专属化

**位置**: [internal/utils/fs.go:18,44](internal/utils/fs.go#L18)

**现状**:
```go
var reWinNonSupport = regexp.MustCompile(`[/\\:*?"<>\|]`)

func WinFileNameWithMaxLen(name string, maxLen int) string {
    name = reWinNonSupport.ReplaceAllString(name, "")
    ...
}
```

**问题**:
- 函数名 `WinFileNameWithMaxLen` 暗示 Windows 专用，但实际在所有平台使用
- `reWinNonSupport` 包含 `:*?"<>|` 这些仅在 Windows 上非法的字符
- 在 Linux/macOS/Docker 上，这些字符是合法的文件名字符，被不必要地移除
- 注释 `NTFS 单个文件名硬限制为 255 字符` 仅适用于 Windows

**建议方案**:
- 重命名为 `SanitizeFileName` 或 `CleanFileName`
- 根据平台使用不同的非法字符正则
- Linux/macOS 仅过滤 `/` 和空字节

---

### L2. 文件权限值在 Windows 上无意义

**位置**: 多处使用 `0755` 和 `0666`

**涉及文件**:
- [internal/path/store.go:25-33](internal/path/store.go#L25) — `os.MkdirAll(sp.Root, 0755)`
- [internal/downloader/file_writer.go:168,229](internal/downloader/file_writer.go#L168) — `os.MkdirAll(dir, 0755)`
- [internal/downloading/dumper.go:80](internal/downloading/dumper.go#L80) — `os.WriteFile(path, data, 0666)`
- [main.go:95](main.go#L95) — `os.MkdirAll(appRootPath, 0755)`

**问题**:
- Windows 上文件权限位被忽略，不影响功能
- Docker 中以非 root 用户运行时，`0755` 可能导致其他用户无法写入
- `0666` 在 umask 作用下实际为 `0644`

**建议方案**:
- Docker 部署时注意 umask 设置
- 考虑数据目录使用 `0777` 或可配置权限

---

### L3. 原子写入在 Windows 上的边界情况

**位置**: [internal/downloader/file_writer.go:192,248](internal/downloader/file_writer.go#L192)

**现状**:
```go
// atomicWrite / atomicWriteStream
return os.Rename(tempPath, path)
```

**问题**:
- `os.Rename` 在 Windows 上如果目标文件已存在且被占用，会返回错误
- Go 1.15+ 已改善此行为，但在极端情况下（文件被杀毒软件锁定）仍可能失败
- Linux/macOS/Docker 上 `os.Rename` 是原子操作，无此问题

**建议方案**:
- 在 Windows 上先删除目标文件再 Rename（添加重试逻辑）
- 或使用 `os.WriteFile` 直接写入（牺牲原子性换取兼容性）

---

### L4. 控制台标题设置在非交互式环境下无意义

**位置**: [internal/twitter/client.go:210-213,515-518](internal/twitter/client.go#L210)

**现状**:
```go
origin, err := utils.GetConsoleTitle()
utils.SetConsoleTitle(fmt.Sprintf("idle - sleeping until %v", ...))
defer utils.SetConsoleTitle(origin)
```

**问题**:
- 在 Docker 容器中通常没有交互式终端，设置控制台标题无意义
- 非 Windows 平台的 stub 实现直接返回 nil，不产生副作用
- 但调用链仍然执行，有微小的性能开销

**建议方案**:
- 低优先级，当前 stub 实现已足够
- 可考虑在 Server 模式下跳过控制台标题设置

---

## 已正确处理的部分

### ✅ P1. 平台特定代码使用构建标签

**位置**:
- [internal/utils/win32.go](internal/utils/win32.go) — `//go:build windows`
- [internal/utils/stub.go](internal/utils/stub.go) — `//go:build !windows`

Windows 平台使用 `kernel32.dll` API 设置控制台标题，其他平台使用空实现 stub。这是正确的跨平台模式。

---

### ✅ P2. URL 路径使用 `path` 包而非 `filepath` 包

**位置**: [internal/utils/fs.go:110-112](internal/utils/fs.go#L110)

```go
// 使用 path.Ext 而不是 filepath.Ext，因为 URL path 总是使用正斜杠
return path.Ext(pu.Path), nil
```

正确识别了 URL 路径和文件系统路径的区别。

---

### ✅ P3. CI/CD 已配置三平台构建

**位置**: [.github/workflows/go.yml](.github/workflows/go.yml)

```yaml
strategy:
  matrix:
    os: [ubuntu-latest, windows-latest, macos-latest]
```

已在三大平台上进行构建和测试。

---

## Docker 部署专项检查

### 当前 Docker 就绪度评估: ❌ 未就绪

| 检查项 | 状态 | 说明 |
|-------|------|------|
| Dockerfile | ❌ 缺失 | 无 Docker 构建文件 |
| docker-compose.yml | ❌ 缺失 | 无编排配置 |
| 环境变量配置 | ❌ 不支持 | 仅支持 YAML 文件配置 |
| 优雅关闭 | ❌ 缺失 | Server 模式无 SIGTERM 处理 |
| 健康检查端点 | ✅ 已有 | `/api/v1/health` 可用 |
| 数据卷挂载 | ⚠️ 需设计 | 配置和数据路径需明确 |
| 非 root 运行 | ⚠️ 未考虑 | 符号链接和文件权限需验证 |
| 时区处理 | ⚠️ 未考虑 | 依赖系统时区，Docker 默认 UTC |
| 多阶段构建 | ❌ 缺失 | CGO 依赖导致构建复杂 |
| 日志输出 | ✅ 已有 | 使用 logrus，可输出到 stdout |

### Docker 部署需解决的关键问题

1. **CGO 依赖导致构建复杂**
   - 必须使用 `golang:1.25-bookworm` 或类似带编译工具的镜像
   - 无法使用 `scratch` 或 `distroless` 作为最终镜像
   - 替换为纯 Go SQLite 库后可大幅简化

2. **配置注入方式**
   - 当前必须挂载 `conf.yaml` 文件
   - 应支持环境变量注入，便于 Docker Compose / Kubernetes 配置

3. **数据持久化**
   - 需要明确两个挂载点:
     - 配置目录: `~/.tmd2/` → `/config`
     - 下载数据: `conf.yaml` 中 `root_path` → `/data`
   - 数据库文件 `foo.db` 在 `.data/` 子目录中

4. **Server 模式是 Docker 的主要运行模式**
   - CLI 交互模式不适合容器化
   - `-server` 和 `-port` 参数应可通过环境变量配置

5. **时区问题**
   - 推文时间戳依赖本地时区
   - Docker 默认 UTC 时区
   - 应通过 `TZ` 环境变量或 `/etc/localtime` 挂载处理

---

## 修复优先级路线图

### Phase 1: Docker 基础就绪 (高优先级)

| 序号 | 任务 | 涉及文件 | 说明 |
|------|------|---------|------|
| 1 | 替换 CGO SQLite 为纯 Go 实现 | `go.mod`, `database/connect.go` | 解除交叉编译限制 |
| 2 | 支持环境变量配置 | `config/config.go`, `main.go` | Docker 12-Factor 规范 |
| 3 | Server 模式优雅关闭 | `api/server.go`, `main.go` | 处理 SIGTERM |
| 4 | 主目录路径支持环境变量覆盖 | `main.go` | `TMD_HOME` / `XDG_CONFIG_HOME` |
| 5 | 创建 Dockerfile (多阶段构建) | 新文件 | 构建和运行镜像 |

### Phase 2: 跨平台兼容性增强 (中优先级)

| 序号 | 任务 | 涉及文件 | 说明 |
|------|------|---------|------|
| 6 | 符号链接降级方案 | `downloading/entity.go`, `batch_download.go` | Windows Junction / 目录复制 |
| 7 | 信号处理跨平台 | `main.go` | 构建标签分离 SIGHUP/SIGQUIT |
| 8 | 文件名清理函数平台适配 | `utils/fs.go`, `naming/*.go` | 按平台使用不同过滤规则 |
| 9 | 数据库 COLLATE 策略 | `database/schema.go` | 路径比较大小写敏感性 |

### Phase 3: Docker 运维完善 (低优先级)

| 序号 | 任务 | 涉及文件 | 说明 |
|------|------|---------|------|
| 10 | 创建 docker-compose.yml | 新文件 | 示例编排配置 |
| 11 | 非 root 用户运行支持 | Dockerfile, 权限代码 | 安全最佳实践 |
| 12 | 原子写入 Windows 边界处理 | `downloader/file_writer.go` | 重试逻辑 |
| 13 | 时区处理文档 | 文档 | TZ 环境变量说明 |

---

## 附录: 关键文件跨平台问题速查表

| 文件 | 行号 | 问题 | 严重级别 |
|------|------|------|---------|
| `go.mod` | 10 | CGO 依赖 (mattn/go-sqlite3) | 🔴 严重 |
| `main.go` | 81-88 | 主目录路径检测不完善 | 🟡 中等 |
| `main.go` | 199 | SIGHUP/SIGQUIT Windows 不兼容 | 🟡 中等 |
| `internal/downloading/entity.go` | 29,41 | 符号链接无降级 | 🔴 严重 |
| `internal/downloading/batch_download.go` | 187 | 符号链接无降级 | 🔴 严重 |
| `internal/downloading/list_sync.go` | 84 | 符号链接删除无降级 | 🟡 中等 |
| `internal/api/server.go` | 122-129 | 无优雅关闭 | 🟡 中等 |
| `internal/config/config.go` | 全文件 | 不支持环境变量 | 🟡 中等 |
| `internal/database/schema.go` | 43,53 | COLLATE NOCASE 跨平台不一致 | 🟡 中等 |
| `internal/utils/fs.go` | 18,44 | 文件名清理 Windows 专属化 | 🟢 低等 |
| `internal/downloader/file_writer.go` | 192,248 | 原子写入 Windows 边界 | 🟢 低等 |
| `internal/downloading/dumper.go` | 80 | 文件权限 Windows 无意义 | 🟢 低等 |
| `internal/twitter/client.go` | 210-213 | 控制台标题 Docker 无意义 | 🟢 低等 |









## 跨平台兼容性审查报告

### 🔴 严重问题（必须修复）

#### 1. 符号链接（symlink）— Windows 上需要管理员权限

**涉及文件：**
- [entity.go:29](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/downloading/entity.go#L29) — `os.Symlink(path, linkpath)`
- [entity.go:41](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/downloading/entity.go#L41) — `os.Symlink(path, newlinkpath)`
- [batch_download.go:187](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/downloading/batch_download.go#L187) — `os.Symlink(upath, linkpath)`

**问题：** Windows 上创建符号链接需要 `SeCreateSymbolicLinkPrivilege` 权限（管理员或开发者模式），否则直接报错。当前代码虽然有 `symlinkWarnCount` 压制日志，但**功能上是降级的**——列表目录结构无法正确建立。

**Docker 影响：** Docker 容器中默认以 root 运行，symlink 没有问题。但如果以非 root 用户运行，且内核版本较老，可能也会遇到权限问题。

**建议：** 为 Windows 提供降级方案，symlink 失败时自动回退到创建**目录快捷方式（junction）**或**直接复制目录**：

```go
// 需要新增一个跨平台 symlink 函数
func CreateSymlink(oldname, newname string) error {
    err := os.Symlink(oldname, newname)
    if err != nil && runtime.GOOS == "windows" {
        // 回退方案1：尝试 junction（仅限目录）
        return createJunction(oldname, newname)
    }
    return err
}
```

---

#### 2. 信号处理 — `SIGHUP` 在 Windows 上不存在

**涉及文件：** [main.go:199](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/main.go#L199)

```go
signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
```

**问题：** `SIGHUP` 和 `SIGQUIT` 在 Windows 上不存在，Go 在 Windows 上编译时会报未定义错误。

**Docker 影响：** Docker 停止容器时发送 `SIGTERM`，这是正确的。但 `SIGHUP` 常用于重载配置，在容器场景下也需要支持。

**建议：** 使用构建标签分离信号处理：

```go
// signal_unix.go
//go:build !windows

package main

import "syscall"

var shutdownSignals = []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}
```

```go
// signal_windows.go
//go:build windows

package main

import "syscall"

var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
```

---

#### 3. CGO 依赖 — `mattn/go-sqlite3` 需要 C 编译器

**涉及文件：** [go.mod:10](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/go.mod#L10) — `github.com/mattn/go-sqlite3 v1.14.22`

**问题：** `go-sqlite3` 是 CGO 库，交叉编译时需要对应平台的 C 工具链。Docker 镜像需要安装 `gcc`/`musl-dev`，macOS 交叉编译需要 `xcrun`。

**Docker 影响：** 构建镜像需要 `glibc` 或 `musl` 兼容的 C 库。Alpine 镜像需要额外安装 `musl-dev` 和 `gcc`。

**建议（二选一）：**

- **方案 A（推荐）：** 替换为纯 Go 实现的 SQLite 库 `modernc.org/sqlite`，消除 CGO 依赖，实现真正的静态编译和零 C 工具链要求：
  ```go
  import _ "modernc.org/sqlite"  // 替代 mattn/go-sqlite3
  ```
  DSN 格式兼容，改动极小。

- **方案 B：** 保留 `go-sqlite3`，但为 Docker 构建提供多阶段 Dockerfile，在构建阶段安装 gcc。

---

#### 4. 主目录路径获取 — 硬编码环境变量名

**涉及文件：** [main.go:81-85](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/main.go#L81-L85)

```go
if runtime.GOOS == "windows" {
    homepath = os.Getenv("appdata")
} else {
    homepath = os.Getenv("HOME")
}
```

**问题：**
1. Linux/macOS 上应使用 `os.UserHomeDir()` 而非直接读 `$HOME`（更健壮，处理无 HOME 变量的情况）
2. Windows 上 `appdata` 环境变量名大小写不敏感但代码写的是小写 `appdata`，虽然 Windows 的 `os.Getenv` 是大小写不敏感的，但这不够规范
3. **Docker 场景下**，容器通常以 root 运行，`$HOME` 是 `/root`，配置目录放在这里没问题，但如果挂载卷覆盖了 `$HOME`，路径会变

**建议：**
```go
homepath, err := os.UserHomeDir()
if err != nil {
    panic("failed to get home path: " + err.Error())
}
// Windows 上 os.UserHomeDir() 返回 C:\Users\xxx，不是 AppData
// 如需放 AppData，可使用 os.UserConfigDir()
```

如果确实需要区分配置目录和数据目录（XDG 规范），应使用 `os.UserConfigDir()`（Windows 返回 `%AppData%`，Linux 返回 `~/.config`）。

---

### 🟡 中等问题（建议修复）

#### 5. 文件名清理函数命名和逻辑偏 Windows

**涉及文件：** [fs.go:44](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/utils/fs.go#L44)

```go
func WinFileNameWithMaxLen(name string, maxLen int) string {
```

**问题：**
1. 函数名 `WinFileName` 暗示 Windows 专用，但实际在所有平台都调用（[tweet_naming.go:21](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/naming/tweet_naming.go#L21)、[user_naming.go:15](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/naming/user_naming.go#L15)、[list_naming.go:14](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/naming/list_naming.go#L14)）
2. `reWinNonSupport` 正则 `[/\\:*?"<>\|]` 包含了 Windows 特有字符 `:*?"<>|`，在 Linux/macOS 上这些字符是合法的文件名字符
3. `DefaultMaxFileNameLen = 158` 是基于 NTFS 255 字符限制计算的，ext4/xfs/APFS 的限制不同（通常 255 字节）

**建议：** 重命名并按平台调整：

```go
func SanitizeFileName(name string, maxLen int) string {
    // 清理逻辑保持不变（Windows 的严格规则是安全超集，在所有平台都安全）
    // 但 maxLen 应根据平台调整
}
```

> 注：虽然 Windows 规则更严格，但作为"最严格公共子集"在所有平台上使用是安全的——只是 Linux/macOS 上文件名会更受限。如果需要 Linux/macOS 上保留更多字符，可以按平台分支。

---

#### 6. 数据库路径存储 — `COLLATE NOCASE` 在跨平台时可能不匹配

**涉及文件：** [schema.go:43,53](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/database/schema.go#L43)

```sql
parent_dir VARCHAR NOT NULL COLLATE NOCASE,
parent_dir VARCHAR COLLATE NOCASE NOT NULL,
```

**问题：** `COLLATE NOCASE` 使路径比较大小写不敏感，这在 Windows 上是正确的（Windows 路径大小写不敏感），但在 Linux/macOS 上路径是大小写敏感的。如果数据库在 Windows 上创建后迁移到 Linux，可能导致路径匹配失败。

**Docker 影响：** 如果用户在 Windows 上创建了数据库文件，然后挂载到 Docker（Linux）容器中使用，`COLLATE NOCASE` 会导致路径查询行为不一致。

**建议：** 这个问题比较微妙。如果数据库只在同一平台上使用，问题不大。如果要支持跨平台数据库迁移，需要考虑路径规范化。

---

#### 7. Server 模式缺少优雅关闭

**涉及文件：** [server.go:122-129](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/api/server.go#L122-L129)

```go
server := &http.Server{
    Addr:         addr,
    Handler:      handler,
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
}
return server.ListenAndServe()
```

**问题：** Server 模式没有监听系统信号，Docker 发送 `SIGTERM` 时不会优雅关闭，可能导致：
- 正在进行的下载任务被强制中断
- SQLite WAL 文件未正确 checkpoint
- SSE 连接未正确关闭

**建议：** 在 `runServer` 中添加信号监听和 `server.Shutdown()`：

```go
func (s *Server) Start(port int) error {
    // ... 现有代码 ...
    
    srv := &http.Server{Addr: addr, Handler: handler, ...}
    
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan
        log.Infoln("shutting down server...")
        srv.Shutdown(context.Background())
    }()
    
    return srv.ListenAndServe()
}
```

---

#### 8. 时区处理 — Docker 容器默认 UTC

**涉及文件：** [server.go:149](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/api/server.go#L149)

```go
Timestamp: time.Now().UTC(),
```

**问题：** 代码中 `time.Now().UTC()` 是好的实践，但其他地方如 `time.Now()` 没有统一使用 UTC。Docker 容器默认时区是 UTC，如果用户期望本地时区显示，需要在 Docker 层面设置 `TZ` 环境变量。

**建议：** 在 Dockerfile 或文档中说明需要设置 `TZ` 环境变量。

---

### 🟢 良好实践（无需修改）

#### ✅ 已正确处理的部分

1. **`win32.go` + `stub.go` 的构建标签**：`SetConsoleTitle`/`GetConsoleTitle` 已通过 `//go:build windows` 和 `//go:build !windows` 正确分离，非 Windows 平台返回空值。

2. **`filepath.Join` 的使用**：全项目统一使用 `filepath.Join` 而非字符串拼接，路径分隔符由 Go 自动处理。

3. **`GetExtFromUrl` 使用 `path.Ext`**：[fs.go:111](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/utils/fs.go#L111) 正确使用 `path.Ext`（而非 `filepath.Ext`）解析 URL 路径，避免了 Windows 反斜杠问题。

4. **`ExtractImageExtFromURL` 使用 `pathpkg.Ext`**：[helpers.go:17](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/downloader/helpers.go#L17) 同样正确。

5. **文件权限 `0755`/`0644`**：在 Windows 上被忽略但不报错，在 Linux/macOS/Docker 上正确生效。

6. **CI/CD 多平台构建**：[go.yml](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/.github/workflows/go.yml) 已配置 `ubuntu-latest`, `windows-latest`, `macos-latest` 三平台矩阵。

7. **代理环境变量**：[client.go:31-51](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/twitter/client.go#L31-L51) 正确处理了 `HTTP_PROXY`/`HTTPS_PROXY` 环境变量，Docker 场景下可通过环境变量配置代理。

8. **`syscall.ENOSPC` 磁盘满检测**：[tweet_download.go:357](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/downloading/tweet_download.go#L357) 和 [profile/downloader.go:341](file:///c:/Users/leeexxx/Documents/trae_projects/tmd/internal/downloading/profile/downloader.go#L341) — `ENOSPC` 在所有 Unix 系统上通用，Windows 上虽然错误码不同但 Go 的 `syscall` 包做了映射。

---

### 📋 Docker 部署专项清单

| 检查项 | 状态 | 说明 |
|--------|------|------|
| CGO 依赖 | 🔴 | `go-sqlite3` 需要 gcc，构建镜像需安装 |
| 信号处理 | 🔴 | `SIGHUP`/`SIGQUIT` Windows 不存在 |
| 优雅关闭 | 🟡 | Server 模式缺少 SIGTERM 优雅关闭 |
| 时区 | 🟡 | 容器默认 UTC，需文档说明 |
| 端口暴露 | ✅ | `-port` 参数可配置 |
| 数据持久化 | ✅ | 配置目录和数据目录可分离 |
| 非 root 运行 | 🟡 | symlink 在非 root 下可能受限 |
| 健康检查 | ✅ | `/api/v1/health` 端点已实现 |
| 代理支持 | ✅ | 环境变量代理已实现 |

---

### 📝 修复优先级总结

| 优先级 | 问题 | 影响 |
|--------|------|------|
| P0 | 信号处理 `SIGHUP`/`SIGQUIT` | Windows 编译失败 |
| P0 | CGO 依赖 `go-sqlite3` | 交叉编译和 Docker 构建复杂 |
| P1 | 主目录路径获取 | Docker 环境变量不一致 |
| P1 | Server 优雅关闭 | Docker 数据丢失风险 |
| P1 | Symlink 降级方案 | Windows 功能缺失 |
| P2 | 文件名清理函数重命名 | 代码可读性和语义正确性 |
| P2 | 数据库 COLLATE NOCASE | 跨平台数据库迁移 |
| P2 | 时区文档 | Docker 用户体验 |
