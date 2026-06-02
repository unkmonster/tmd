# Twitter Media Downloader

[![Go Version](https://img.shields.io/badge/Go-1.25.0-blue.svg)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](./LICENSE)
[![CI/CD](https://github.com/unkmonster/tmd/actions/workflows/go.yml/badge.svg)](.github/workflows/go.yml)
[Release](https://github.com/unkmonster/tmd/releases/latest)

> **版本**: 3.4.4 | **状态**: 活跃维护 | **许可证**: GPL-3.0

本项目的代码基于 [unkmonster/tmd](https://github.com/unkmonster/tmd) 项目，修改了部分代码，添加了新的功能特性。新增的功能见 [CHANGELOG.md文件](CHANGELOG.md)

## 目录

- [项目架构](#项目架构)
- [功能特性](#功能特性)
- [安装与配置](#安装与配置)
- [命令行参数详解](#命令行参数详解)
- [API Server 模式](#api-server-模式)
- [定时任务调度器](#定时任务调度器)
- [Profile 下载功能](#profile-下载功能)
- [推文 JSON 保存](#推文-json-保存)
- [文件存储结构](#文件存储结构)
- [使用场景与示例](#使用场景与示例)
- [高级设置](#高级设置)
- [常见问题](#常见问题)

***

## 功能特性

### 下载入口

- 按用户下载媒体推文（图片、视频、GIF）
- 按列表下载列表成员的媒体推文
- 按关注下载某个用户关注列表中的媒体推文
- 支持混合批量任务：用户、列表、关注源可组合提交
- 支持单独下载 Profile：头像、横幅、简介、`profile.json`
- 支持下载列表成员的 Profile

### 下载过程与行为

- 媒体文件按推文时间设置修改时间
- 保存推文侧车文件：`.txt` / `.json`
- 下载时可选跳过 Profile（`-noprofile` / `skip_profile`）
- 下载失败后可选关闭重试（`-no-retry` / `no_retry`）
- 支持下载时主动关注目标或成员（`-follow-members`）
- 支持对受保护账号自动发起关注请求（`-auto-follow`）
- 支持附加 Cookie 多账号分摊请求压力
- 内置速率限制处理与重试逻辑，降低触发 X/Twitter 限流后的失败率

### 增量、去重与失败处理

- 基于数据库中的 `latest_release_time` 做增量拉取，避免重复抓取历史推文
- 同一用户在多个列表中只维护一份用户目录，列表目录通过链接复用
- 失败推文记录到 `.data/errors.json`，后续下载时只重试失败项
- 403/404 类媒体错误不进入重试队列
- 用户/列表元数据会同步到 SQLite，包括用户名、历史用户名、受保护状态、可访问状态等
- 可将用户、列表或关注源直接标记为“已下载”，跳过历史推文（`-mark-downloaded`）
- `mark-time` 支持指定时间戳，或设为 `null` / `nil` 以清空增量游标

### 导入与补录

- **第三方 JSON 导入**（`-jsonfile`）：读取外部导出的推文 JSON，转换为内部推文结构后下载媒体，并保存 `.txt` / `.json`
- **LoongTweet 文件夹导入**（`-jsonfolder`）：递归读取 `.loongtweet` / JSON 目录中的推文数据并补下载媒体
- JSON 导入与文件夹导入都复用统一的推文下载与命名逻辑

### 文件写入与资料版本

- 小文件使用内存缓冲下载，大文件（≥10MB）自动切换流式下载
- 文件写入采用原子替换，减少中断时的半写入风险
- 对未变化文件可跳过重写
- Profile 与需要版本化的文件支持写入前备份到 `.versions/`
- Profile 数据保存在用户目录下 `.loongtweet/.profile/`

### API Server 与 Web UI

- 提供 HTTP API 与内置 Web 管理界面
- Server 模式中的下载任务异步执行，先返回 `task_id`
- 任务支持排队、运行、完成、失败、取消等状态流转
- 通过 SSE 实时推送任务进度与任务列表更新
- 提供实时日志流，支持按级别和关键词过滤
- 提供配置、附加 Cookie、调度任务的表单/原始 YAML 编辑
- 提供数据库浏览与基础维护接口（用户、列表、实体、关联）
- 支持通过 API 触发优雅关闭

### 调度与自动化

- 内置调度器，支持 `interval` 和 `daily` 两种调度模式
- 支持 `user`、`list`、`following`、`mixed` 四种调度目标类型
- 调度项支持 `auto_follow`、`follow_members`、`skip_profile`、`no_retry`
- 调度配置可热重载、校验、启停和手动触发

***

## 安装与配置

### 环境要求

- **Go**: >= 1.25.0（从源码编译时需要）
- **操作系统**: Windows 10+, macOS 10.15+, Ubuntu 18.04+
- **编译器**: 支持 `CGO_ENABLED=0` 纯 Go 构建
- **内存**: 建议 >= 512MB
- **磁盘空间**: 根据下载数量而定
- **权限**: Windows 需要管理员权限（创建符号链接）

### 下载/编译

**直接下载（推荐）**

前往 [Release](https://github.com/unkmonster/tmd/releases/latest) 自行选择合适的版本：

| 平台 | 文件名 |
|------|--------|
| Windows | `tmd-windows-amd64.exe` |
| Linux | `tmd-linux-amd64` |
| macOS | `tmd-darwin-amd64` |

**自行编译**

```bash
# 克隆项目
git clone https://github.com/unkmonster/tmd.git
cd tmd

# 编译 Windows 版本
go build -o tmd.exe .

# 交叉编译 Linux 版本
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tmd-linux .

# 交叉编译 macOS 版本
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o tmd-macos .
```

> **说明**: SQLite 使用 `modernc.org/sqlite` 纯 Go driver，源码构建不再需要 GCC/MingW 等 C 编译器。

### Docker

项目通过 GitHub Actions 自动发布 Docker 镜像到 **Docker Hub** 和 **GHCR** 双仓库，两者镜像内容完全一致，任选其一即可：

| 镜像源 | 地址 |
|--------|------|
| **Docker Hub**（推荐） | `docker.io/leeexx00/tmd:<tag>` |
| **GHCR** | `ghcr.io/leeexx2001/tmd:<tag>` |

```bash
# Docker Hub
docker pull leeexx00/tmd:latest
docker pull leeexx00/tmd:v3.4.4

# GHCR
docker pull ghcr.io/leeexx2001/tmd:latest
docker pull ghcr.io/leeexx2001/tmd:v3.4.4
```

**推荐方式：使用 docker compose**

1. 创建目录：

```bash
mkdir -p config data
```

2. 创建 `.env` 文件或者直接修改yml文件中对应项：

```env
TMD_AUTH_TOKEN=your_auth_token
TMD_CT0=your_ct0
TMD_PROXY_URL=
TMD_MAX_DOWNLOAD_ROUTINE=8
TMD_MAX_FILE_NAME_LEN=158
TZ=Asia/Shanghai
```

3. 使用 `docker-compose.yml` 启动：

```bash
docker compose up -d
```

4. 查看状态：

```bash
docker compose ps
docker compose logs -f
```

README 中的 compose 示例与仓库根目录的 [docker-compose.yml](./docker-compose.yml) 保持一致（默认使用 Docker Hub 镜像源）：

```yaml
services:
  tmd:
    image: leeexx00/tmd:latest
    container_name: tmd
    restart: unless-stopped
    ports:
      - "25556:25556"
    environment:
      TMD_HOME: /config
      TMD_ROOT_PATH: /data
      TMD_AUTH_TOKEN: ${TMD_AUTH_TOKEN}
      TMD_CT0: ${TMD_CT0}
      TMD_PORT: 25556
      TMD_PROXY_URL: ${TMD_PROXY_URL:-}
      TMD_MAX_DOWNLOAD_ROUTINE: ${TMD_MAX_DOWNLOAD_ROUTINE:-8}
      TMD_MAX_FILE_NAME_LEN: ${TMD_MAX_FILE_NAME_LEN:-158}
      TZ: ${TZ:-Asia/Shanghai}
    volumes:
      - ./config:/config
      - ./data:/data
    stop_grace_period: 30s
```

如需切换为 GHCR 镜像源，将上述 `image` 改为：
```yaml
image: ghcr.io/leeexx2001/tmd:latest
```

**单容器最小运行示例**

```bash
# Docker Hub
docker run -d \
  --name tmd \
  -p 25556:25556 \
  -v /path/to/config:/config \
  -v /path/to/data:/data \
  -e TMD_HOME=/config \
  -e TMD_ROOT_PATH=/data \
  -e TMD_AUTH_TOKEN=your_auth_token \
  -e TMD_CT0=your_ct0 \
  -e TMD_PORT=25556 \
  -e TMD_PROXY_URL= \
  -e TMD_MAX_DOWNLOAD_ROUTINE=8 \
  -e TMD_MAX_FILE_NAME_LEN=158 \
  -e TZ=Asia/Shanghai \
  leeexx00/tmd:latest -server

# 或使用 GHCR（将最后一行镜像地址替换即可）
# ghcr.io/leeexx2001/tmd:latest -server
```

启动后可访问：

```text
http://localhost:25556/
http://localhost:25556/api/v1/health
```

**部署说明**

- `/config`：配置、额外 cookies、调度文件、日志目录
- `/data`：下载数据目录，包含 `users/` 和 `.data/foo.db`
- `TMD_AUTH_TOKEN`、`TMD_CT0`：必填
- `TMD_PROXY_URL`：可选，使用代理时设置，例如 `http://host.docker.internal:7897`
- `TMD_MAX_DOWNLOAD_ROUTINE`：可选，默认 `8`
- `TMD_MAX_FILE_NAME_LEN`：可选，默认 `158`
- `TZ`：可选，默认 `Asia/Shanghai`
- 同一个 `/data` 卷只建议同时运行一个 TMD 实例
- 如果宿主机端口 `25556` 被占用，可改 compose 里的左侧端口，例如 `"8080:25556"`
- 如果不想把数据放在当前目录，可把 `./config`、`./data` 改成宿主机绝对路径

### 首次运行

```bash
tmd -conf
```

程序会提示输入以下配置：

| 配置项                  | 说明                            | 默认值                   | 示例                     |
| -------------------- | ----------------------------- | ---------------------- | ---------------------- |
| storage dir          | 文件存储目录                        | 无（必填）                | `D:\twitter_downloads` |
| auth\_token          | Twitter Cookie 中的 auth\_token | 无（必填）                | `a1b2c3d4e5f6...`      |
| ct0                  | Twitter Cookie 中的 ct0         | 无（必填）                | `x1y2z3...`            |
| max download routine | 最大并发下载数（范围 1-100）         | `min(100, CPU×10)`¹                | `35`                   |
| max file name len    | 最大文件名长度（50-250）           | `158`                   | `158`                  |
| proxy_url            | 代理服务器 URL（支持 http/https/socks5） | 空（使用系统代理） | `http://127.0.0.1:7890` |

> ¹ `max download routine` 默认值为 `min(100, runtime.GOMAXPROCS(0)*10)`，即 CPU 核数的 10 倍且不超过 100。首次通过 `-conf` 配置时建议输入 35。

### 配置文件位置

| 系统          | 路径                          |
| ----------- | --------------------------- |
| Windows     | `%APPDATA%\.tmd2\conf.yaml` |
| macOS/Linux | `~/.tmd2/conf.yaml`         |

### 其他配置文件

| 文件 | 位置 | 说明 |
|------|------|------|
| 备用 Cookie | `$HOME/.tmd2/additional_cookies.yaml` | 多账号 Cookie |
| 定时任务 | `$HOME/.tmd2/schedules.yaml` | 调度器配置 |
| 日志文件 | `$HOME/.tmd2/tmd2.log` | 主日志 |
| CLI 日志 | `$HOME/.tmd2/client.log` | REST 客户端日志 |

### 获取 Cookie

1. 登录 [Twitter/X](https://x.com)
2. 打开浏览器开发者工具 (F12)
3. 进入 Application → Cookies → x.com
4. 复制 `auth_token` 和 `ct0` 的值

> 详细获取方式请参考 [获取 Cookie](https://github.com/unkmonster/tmd/blob/master/doc/help.md#获取-cookie)

***

## 命令行参数详解

### 基础参数

| 参数        | 类型   | 默认值   | 说明                                 |
| --------- | ---- | ----- | ---------------------------------- |
| `-conf`     | bool | false | 重新配置程序（部分更新，显示当前值可逐项修改）       |
| `-dbg`      | bool | false | 显示调试信息，包括请求计数等                     |
| `-server`   | bool | false | 启动 API Server 模式                   |
| `-port`     | int  | 25556 | API Server 监听端口（仅与 `-server` 一起使用） |

### 推文下载参数

| 参数      | 类型     | 可重复 | 说明                       |
| ------- | ------ | --- | ------------------------ |
| `-user` | string | ✅   | 指定下载用户名（可带@前缀，如 `elonmusk` 或 `@elonmusk`） |
| `-list` | uint64 | ✅   | 指定下载列表ID                 |
| `-foll` | string | ✅   | 指定用户，下载其关注的所有用户          |

### JSON 下载参数

| 参数           | 类型     | 可重复 | 说明                                                         |
| -------------- | ------ | --- | ---------------------------------------------------------- |
| `-jsonfile`    | string | ✅   | 从第三方工具导出的 JSON 文件下载推文媒体（图片/视频/txt/json） |
| `-jsonfolder`  | string | ✅   | 从 TMD 生成的 `.loongtweet` 文件夹下载推文媒体 |

**`-jsonfile` 参数**：
- 用于第三方工具导出的 Twitter 推文搜索结果 JSON（包含推文列表和 media 数组）
- **下载内容**：推文媒体文件（图片/视频）、推文文本（`.txt`）、完整 metadata（`.json`）
- **保存位置**：`users/{screen_name}/` 目录下
  - 媒体文件：`{推文文本}_{tweetID}.jpg`、`{推文文本}_{tweetID}(1).jpg`
  - `.loongtweet/` 子目录：
    - `{推文文本}_{tweetID}.txt` — 推文文本内容
    - `{推文文本}_{tweetID}.json` — 完整 metadata（已转换+清理）
- 文件命名与 `-user` 模式完全一致（使用 `TweetNaming`）
- **格式转换**：自动将第三方新格式 JSON 转换为 TMD 兼容旧格式
  - 嵌套对象扁平化：`RelationshipPerspectives.blocked_by` → `legacy.blocked_by`
  - 头像 URL 清理：移除 `_normal` 后缀
  - **高清参数**：图片 URL 自动追加 `?name=4096x4096`
- 转换失败时降级使用原始 metadata（不阻塞下载）

**`-jsonfolder` 参数**：
- 用于 TMD 之前下载保存的 `.loongtweet` 文件夹中的 JSON 文件
- **仅下载推文媒体文件**（图片/视频），**不保存** `.json`、`.txt`、`.profile` 等元数据
- 适合重新下载或迁移媒体文件
- 文件命名与 `-user` 模式完全一致
- 图片 URL 自动追加 `?name=4096x4096` 高清参数

> 💡 **推荐搭配**：使用 [twitter-web-exporter](https://github.com/prinsss/twitter-web-exporter) 浏览器脚本导出推文或用户列表为 JSON 格式，然后用 `-jsonfile` 或 `-jsonfolder` 参数下载。

### 下载行为参数

| 参数             | 类型   | 默认值   | 说明                        |
| -------------- | ---- | ----- | ------------------------- |
| `-auto-follow` | bool | false | 自动向受保护用户发送关注请求（列表下载时默认启用） |
| `-follow-members` | bool | false | 下载时关注目标/成员（用户/列表成员/关注列表成员），失败仅 warning 不阻塞下载 |
| `-no-retry`    | bool | false | 快速退出，不重试失败的推文             |

> 语义区别：
> - `-auto-follow`：仅在下载过程中遇到 **受保护且未关注** 用户时发送关注请求。
> - `-follow-members`：对下载目标/成员中 **未关注** 的用户尝试关注（不限是否受保护），并避免与 `-auto-follow` 重复请求。

### 标记参数

| 参数                 | 类型     | 默认值   | 说明                               |
| ------------------ | ------ | ----- | -------------------------------- |
| `-mark-downloaded` | bool   | false | 仅标记用户为已下载，不下载内容                  |
| `-mark-time`       | string | 当前时间  | 指定标记时间戳，格式：`2006-01-02T15:04:05` |

### Profile 下载参数

| 参数              | 类型     | 可重复 | 说明                                                         |
| --------------- | ------ | --- | ---------------------------------------------------------- |
| `-noprofile`    | bool   | -   | 跳过 Profile 下载（默认在使用 `-user`/`-list`/`-foll` 时自动下载 Profile） |
| `-profile-user` | string | ✅   | 单独指定下载 profile 的用户（无需同时下载推文）                               |
| `-profile-list` | uint64 | ✅   | 单独指定下载 profile 的列表ID（无需同时下载推文）                             |

> **注意**：使用 `-user`、`-list`、`-foll` 下载推文时，Profile 下载默认启用。使用 `-noprofile` 可跳过。使用 `-profile-user`/`-profile-list` 可仅下载 Profile 而不下载推文。

***

## API Server 模式

TMD 支持以 API Server 模式运行，提供 HTTP REST API 和 Web 管理界面，便于远程控制、自动化集成和实时监控。

### 启动 API Server

```bash
# 使用默认端口 25556 启动
tmd -server

# 指定端口启动
tmd -server -port 8080
```

### 功能特性

| 功能           | 说明                              |
| ------------ | ------------------------------- |
| **REST API** | 完整的 HTTP API，支持下载任务管理、状态查询、任务取消 |
| **Web 管理界面** | 内置可视化界面，支持浏览器访问和操作 |
| **实时任务监控** | SSE 推送任务状态更新，无需刷新页面 |
| **数据库浏览** | 查看已下载的用户、列表、用户实体信息 |
| **跨域支持** | 默认启用 CORS，支持 Web 前端直接调用 |
| **配置管理** | 双模式配置编辑器：结构化表单 + 原始 YAML 编辑 |
| **Cookie 管理** | 独立管理主 Cookie 和备用 Cookie，支持表单和原始 YAML 编辑 |
| **日志查看** | 实时日志流（SSE）+ 历史日志查看，支持按级别筛选、搜索、分页 |
| **定时任务** | 可视化调度器管理，支持创建/编辑/启禁/手动触发 |
| **服务器控制** | 支持通过 API/Web 优雅关闭服务器 |

### API 端点速查

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| **GET** | `/api/v1/health` | 健康检查 | ❌ |
| **POST** | `/api/v1/users/{screen_name}/download` | 下载用户推文 | ❌ |
| **POST** | `/api/v1/users/{screen_name}/profile` | 下载用户 Profile | ❌ |
| **POST** | `/api/v1/users/{screen_name}/following/download` | 下载关注列表 | ❌ |
| **POST** | `/api/v1/users/{screen_name}/following/mark` | 标记关注列表已下载 | ❌ |
| **POST** | `/api/v1/users/{screen_name}/mark` | 标记用户已下载 | ❌ |
| **POST** | `/api/v1/lists/{list_id}/download` | 下载列表推文 | ❌ |
| **POST** | `/api/v1/lists/{list_id}/profile` | 下载列表 Profile | ❌ |
| **POST** | `/api/v1/lists/{list_id}/mark` | 标记列表已下载 | ❌ |
| **POST** | `/api/v1/json/file/download` | JSON 文件导入下载（支持路径列表/文件上传） | ❌ |
| **POST** | `/api/v1/json/folder/download` | LoongTweet 文件夹下载（支持路径列表/文件上传） | ❌ |
| **POST** | `/api/v1/batch/download` | 批量下载（多用户/列表） | ❌ |
| **GET** | `/api/v1/tasks` | 任务列表 | ❌ |
| **GET** | `/api/v1/tasks/{task_id}` | 任务详情 | ❌ |
| **POST** | `/api/v1/tasks/{task_id}/cancel` | 取消任务 | ❌ |
| **GET** | `/api/v1/sse/tasks` | SSE 实时任务推送 | ❌ |
| **GET** | `/api/v1/db/users` | 用户列表（分页） | ❌ |
| **GET** | `/api/v1/db/users/{id}` | 用户详情 | ❌ |
| **PUT** | `/api/v1/db/users/{id}` | 更新用户 | ❌ |
| **DELETE** | `/api/v1/db/users/{id}` | 删除用户 | ❌ |
| **GET** | `/api/v1/db/users/{id}/previous-names` | 用户历史名称 | ❌ |
| **GET** | `/api/v1/db/lists` | 列表列表（分页） | ❌ |
| **GET** | `/api/v1/db/lists/{id}` | 列表详情 | ❌ |
| **PUT** | `/api/v1/db/lists/{id}` | 更新列表 | ❌ |
| **DELETE** | `/api/v1/db/lists/{id}` | 删除列表 | ❌ |
| **GET** | `/api/v1/db/user-entities` | 用户实体列表（分页） | ❌ |
| **GET** | `/api/v1/db/user-entities/{id}` | 用户实体详情 | ❌ |
| **PUT** | `/api/v1/db/user-entities/{id}` | 更新用户实体 | ❌ |
| **DELETE** | `/api/v1/db/user-entities/{id}` | 删除用户实体 | ❌ |
| **GET** | `/api/v1/db/list-entities` | 列表实体列表（分页） | ❌ |
| **GET** | `/api/v1/db/list-entities/{id}` | 列表实体详情 | ❌ |
| **PUT** | `/api/v1/db/list-entities/{id}` | 更新列表实体 | ❌ |
| **DELETE** | `/api/v1/db/list-entities/{id}` | 删除列表实体 | ❌ |
| **GET** | `/api/v1/db/user-links` | 用户链接查询 | ❌ |
| **GET** | `/api/v1/db/user-links/{id}` | 用户链接详情 | ❌ |
| **PUT** | `/api/v1/db/user-links/{id}` | 更新用户链接 | ❌ |
| **DELETE** | `/api/v1/db/user-links/{id}` | 删除用户链接 | ❌ |
| **GET** | `/api/v1/config` | 系统配置（脱敏） | ❌ |
| **GET** | `/api/v1/config/raw` | 获取原始配置文件内容 | ❌ |
| **PUT** | `/api/v1/config/raw` | 更新原始配置文件 (YAML) | ❌ |
| **GET** | `/api/v1/config/fields` | 获取结构化配置字段列表 | ❌ |
| **PUT** | `/api/v1/config/fields` | 保存结构化配置字段 | ❌ |
| **GET** | `/api/v1/cookies` | 获取备用 Cookie 列表（脱敏） | ❌ |
| **PUT** | `/api/v1/cookies` | 保存备用 Cookie（表单） | ❌ |
| **GET** | `/api/v1/cookies/raw` | 获取原始 Cookie 文件内容 | ❌ |
| **PUT** | `/api/v1/cookies/raw` | 更新原始 Cookie 文件 (YAML) | ❌ |
| **POST** | `/api/v1/server/shutdown` | 优雅关闭服务器 | ❌ |
| **GET** | `/api/v1/logs` | 获取系统日志（支持筛选/分页） | ❌ |
| **GET** | `/api/v1/logs/stream` | SSE 实时日志流 | ❌ |
| **GET** | `/api/v1/schedules` | 获取定时任务列表和状态 | ❌ |
| **PUT** | `/api/v1/schedules` | 替换全部调度配置 | ❌ |
| **POST** | `/api/v1/schedules` | 创建定时任务 | ❌ |
| **GET** | `/api/v1/schedules/raw` | 获取原始调度配置 | ❌ |
| **PUT** | `/api/v1/schedules/raw` | 更新原始调度配置 (YAML) | ❌ |
| **POST** | `/api/v1/schedules/reload` | 重载调度配置 | ❌ |
| **POST** | `/api/v1/schedules/validate` | 验证调度配置 | ❌ |
| **PUT** | `/api/v1/schedules/{id}` | 更新定时任务 | ❌ |
| **DELETE** | `/api/v1/schedules/{id}` | 删除定时任务 | ❌ |
| **PATCH** | `/api/v1/schedules/{id}/enabled` | 启用/禁用定时任务 | ❌ |
| **POST** | `/api/v1/schedules/{id}/trigger` | 手动触发定时任务 | ❌ |
| **GET** | `/` | Web 管理界面 - 仪表盘 | ❌ |
| **GET** | `/tasks` | Web 管理界面 - 任务 | ❌ |
| **GET** | `/data` | Web 管理界面 - 数据 | ❌ |
| **GET** | `/schedules` | Web 管理界面 - 调度 | ❌ |
| **GET** | `/system` | Web 管理界面 - 系统 | ❌ |
| **GET** | `/static/{$}` | 静态资源文件（精确匹配） | ❌ |
| **GET** | `/static/{path...}` | 静态资源文件（路径匹配） | ❌ |

> API JSON 中的 Twitter list ID 使用十进制字符串传输（例如 `"2033436439346905439"`），避免 JavaScript Number 对 64 位 ID 产生精度丢失；URL 路径参数仍直接使用同一个十进制 ID。

> ⚠️ **安全提示**: 当前版本 API 无需认证，仅建议在本地或可信网络使用。生产环境请配合反向代理（Nginx/Caddy）添加 Basic Auth 或 IP 白名单。

### JSON 导入 API 详细说明

JSON 导入端点（`/api/v1/json/file/download` 和 `/api/v1/json/folder/download`）支持**两种请求格式**，根据 `Content-Type` 自动分发：

- **multipart/form-data**（推荐）：适用于 Web UI 和远程调用，直接上传 JSON 文件，无需服务端路径
- **application/json**：用于 CLI 和高级用法，提供服务端文件/文件夹路径列表

> 📖 **完整文档**：详细的请求/响应格式、参数说明、curl 示例和上传限制请参考 [API_DOCUMENTATION.md - 第8节](doc/API_DOCUMENTATION.md#8-从-json-文件下载)

### API 通用参数

**分页参数**（适用于所有 `GET /api/v1/db/*` 端点）：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `page` | 1 | 页码 |
| `pageSize` | 20 | 每页数量（最大 100） |
| `sortBy` | id | 排序字段（白名单限制） |
| `sortOrder` | desc | 排序方向（asc/desc） |
| `q` | - | 搜索关键词 |

**筛选参数**（按端点不同）：

| 参数 | 适用端点 | 说明 |
|------|---------|------|
| `accessible` | `/db/users` | 用户可访问状态筛选 |
| `protected` | `/db/users` | 用户保护状态筛选 |
| `userId` | `/db/user-entities`, `/db/user-links` | 按用户ID筛选 |
| `listId` | `/db/list-entities` | 按列表ID筛选 |
| `ownerId` | `/db/lists` | 按所有者ID筛选 |

### SSE 实时推送

**任务状态推送** - `GET /api/v1/sse/tasks`：

- 每 **2 秒**推送一次所有任务列表（全量推送，非增量）
- 客户端断开时服务端通过 `context.Done()` 自动感知
- 无心跳机制，依赖 HTTP keep-alive 保持连接

**实时日志流** - `GET /api/v1/logs/stream`：

- 基于控制台日志捕获（`consolelog.Hub`），实时推送新日志行
- 支持 `level` 和 `q` 查询参数进行服务端筛选
- 客户端断开时自动取消订阅

### 任务自动清理

- 已完成/失败/取消的任务在 **8 小时**后自动清理
- 清理每 **1 小时**执行一次
- 运行中的任务不会被清理

### 服务器优雅关闭

Server 支持优雅关闭，确保所有资源正确释放：

- **信号触发**：收到 SIGINT/SIGTERM 信号时自动执行
- **API 触发**：`POST /api/v1/server/shutdown`
- **关闭顺序**：取消所有任务 → 停止调度器 → 关闭 HTTP Server → 关闭数据库 → 关闭日志写入器
- **超时保护**：HTTP Server 关闭超时 5 秒
- **幂等性**：使用 `sync.Once` 确保关闭只执行一次

### Web 管理界面

启动 Server 后，打开浏览器访问：

```
http://localhost:25556/
```

界面功能：

- **仪表盘**：系统状态、任务统计、快速操作
- **新建任务**：创建用户/列表/批量/JSON 下载任务
- **任务列表**：实时显示任务状态、进度条、取消操作
- **数据管理**：完整的数据库 CRUD 操作
  - **Users**：查看、搜索、排序、编辑、删除用户
  - **Lists**：查看、搜索、排序、编辑、删除列表
  - **User Entities**：查看、搜索、排序、编辑、删除用户实体
  - **List Entities**：查看、搜索、排序、编辑、删除列表实体
  - **User Links**：查看、搜索、排序、编辑、删除用户链接
  - **User Previous Names**：查看用户历史名称变更记录
- **定时任务**：调度器管理
  - 创建任务：支持 interval 和 daily 两种调度模式
  - 任务类型：支持 list/user/following 三种下载类型
  - 任务控制：启用/禁用、手动触发、删除
  - 原始编辑：支持 YAML 格式批量编辑
- **系统管理**
  - **配置编辑**（双模式）：
    - 📝 **简易模式**：结构化表单，按分组显示字段（基础设置/Cookie认证/高级选项）
    - 🔧 **高级模式**：原始 YAML 编辑器，适合高级用户
    - 自动备份、实时验证、敏感信息脱敏显示
  - **Cookie 管理**：
    - 📝 **表单模式**：结构化编辑备用 Cookie
    - 🔧 **原始模式**：YAML 格式编辑
    - 敏感信息脱敏显示
  - **日志查看器**：
    - 实时日志流（SSE 推送，无需轮询）
    - 按级别筛选（DEBUG/INFO/WARN/ERROR）
    - 关键词搜索
    - 分页浏览
  - **服务器控制**：优雅关闭服务器

### API 文档

详细的 API 文档请参考 [API\_DOCUMENTATION.md](doc/API_DOCUMENTATION.md)，包含：

- 所有 API 端点说明
- 请求/响应格式
- 错误处理
- 使用示例
- **数据库管理 API**：完整的 CRUD 操作文档
  - 用户管理（Users）
  - 列表管理（Lists）
  - 用户实体管理（User Entities）
  - 列表实体管理（List Entities）
  - 用户链接查询（User Links）
  - 用户历史名称查询（User Previous Names）

### 快速示例

```bash
# 1. 启动 Server
tmd -server

# 2. 创建下载任务
curl -X POST http://localhost:25556/api/v1/users/elonmusk/download

# 3. 查询任务列表
curl http://localhost:25556/api/v1/tasks

# 4. 取消任务
curl -X POST http://localhost:25556/api/v1/tasks/task_xxx/cancel
```

***

## 定时任务调度器

TMD Server 内置定时任务调度器，支持按时间间隔或每天固定时间自动执行下载任务。

### 调度模式

| 模式 | 格式 | 示例 | 说明 |
|------|------|------|------|
| **interval** | `interval:<duration>` | `interval:2h` | 每隔指定时间执行一次 |
| **daily** | `daily:<times>` | `daily:07:00,21:00` | 每天在指定时间执行 |

> interval 最小值为 `1m`（1 分钟）。

### 任务类型

| 类型 | target 格式 | 说明 |
|------|------------|------|
| `list` | 列表 ID（正整数） | 下载列表成员推文 |
| `user` | 用户 screen_name | 下载用户推文 |
| `following` | 用户 screen_name | 下载关注列表推文 |

### 配置文件

调度器配置文件位于 `$HOME/.tmd2/schedules.yaml`（Windows: `%APPDATA%\.tmd2\schedules.yaml`）：

```yaml
schedules:
  - id: daily_tech_list
    type: list
    target: "1234567890123"
    name: "科技圈每日同步"
    schedule: "daily:07:00,21:00"
    enabled: true
    run_on_start: false
    auto_follow: false
    follow_members: false
    skip_profile: false
    no_retry: false
  - id: hourly_elon
    type: user
    target: elonmusk
    name: "Elon 每小时同步"
    schedule: "interval:1h"
    enabled: true
    run_on_start: true
    auto_follow: false
    follow_members: false
    skip_profile: false
    no_retry: false
```

### ScheduleEntry 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 否 | 唯一标识（自动生成，格式 `sch_xxxxxxxxxxxx`） |
| `type` | string | 是 | 任务类型：`list` / `user` / `following` |
| `target` | string | 是 | 目标（列表 ID 或用户名） |
| `name` | string | 否 | 任务显示名称 |
| `schedule` | string | 是 | 调度规则（`interval:` 或 `daily:`） |
| `enabled` | bool | 否 | 是否启用（默认 false） |
| `run_on_start` | bool | 否 | 系统首次启动时是否立即执行一次（仅 interval 模式） |
| `auto_follow` | bool | 否 | 自动关注受保护用户 |
| `follow_members` | bool | 否 | 下载时关注目标/成员（失败仅 warning，不阻塞下载） |
| `skip_profile` | bool | 否 | 跳过 Profile 下载 |
| `no_retry` | bool | 否 | 不重试失败推文 |

### 调度器 API

| 方法 | 端点 | 说明 |
|------|------|------|
| **GET** | `/api/v1/schedules` | 获取调度器状态和任务列表 |
| **PUT** | `/api/v1/schedules` | 替换全部调度配置 |
| **POST** | `/api/v1/schedules` | 创建定时任务 |
| **GET** | `/api/v1/schedules/raw` | 获取原始调度配置 |
| **PUT** | `/api/v1/schedules/raw` | 更新原始调度配置 (YAML) |
| **POST** | `/api/v1/schedules/reload` | 重载调度配置 |
| **POST** | `/api/v1/schedules/validate` | 验证调度配置 |
| **PUT** | `/api/v1/schedules/{id}` | 更新定时任务 |
| **DELETE** | `/api/v1/schedules/{id}` | 删除定时任务 |
| **PATCH** | `/api/v1/schedules/{id}/enabled` | 启用/禁用定时任务 |
| **POST** | `/api/v1/schedules/{id}/trigger` | 手动触发定时任务 |

### 调度器状态

`GET /api/v1/schedules` 返回每个任务的状态信息：

| 字段 | 说明 |
|------|------|
| `scheduler_running` | 调度器是否运行中 |
| `entries[].last_run_at` | 上次执行时间 |
| `entries[].next_run_at` | 下次执行时间 |
| `entries[].run_count` | 累计执行次数 |
| `entries[].last_task_id` | 上次执行的任务 ID |
| `entries[].last_error` | 上次执行的错误信息 |
| `entries[].consecutive_failures` | 连续失败次数 |
| `entries[].triggering` | 是否正在触发该调度规则；仅表示正在创建任务，不代表后台下载任务仍在运行 |

创建、更新、启用或重载定时任务后，如果存在启用中的规则且调度器未运行，服务端会自动启动调度器。

***

## Profile 下载功能

### 功能说明

Profile 下载功能可以保存用户的完整个人资料：

| 文件                        | 说明             | 格式   |
| ------------------------- | -------------- | ---- |
| `avatar.jpg/png/gif/webp` | 高清头像 (400x400) | 图片   |
| `banner.jpg/png/gif/webp` | 个人主页横幅         | 图片   |
| `description.txt`         | 用户简介           | 纯文本  |
| `profile.json`            | 完整资料信息         | JSON |

### Profile JSON 结构

```json
{
  "ID": 123456789,
  "Name": "用户名称",
  "ScreenName": "username",
  "URL": "https://example.com",
  "Location": "地点",
  "Verified": true,
  "Protected": false,
  "CreatedAt": "Wed Oct 01 00:00:00 +0000 2014"
}
```

> **注意**: `AvatarURL`、`BannerURL`、`Description` 不会写入 `profile.json`，它们分别保存为独立的图片文件和 `description.txt`。

### 版本管理

当资料变更时，旧文件自动备份：

```
.loongtweet/.profile/.versions/
├── avatar_20240115_103045.jpg
├── banner_20240115_103045.jpg
├── description_20240115_103045.txt
└── profile_20240115_103045.json
```

版本命名格式：`{类型}_{日期}_{时间}.{扩展名}`

***

## 推文 JSON 保存

每次下载推文媒体时，会同时保存推文的完整信息到 `.loongtweet/` 子目录。

### 保存内容

| 文件                | 格式   | 说明               |
| ----------------- | ---- | ---------------- |
| `{tweet_id}.json` | JSON | 推文完整信息（格式化 JSON） |
| `{tweet_id}.txt`  | TXT  | 人类可读的文本格式        |

### JSON 内容

- 推文文本、时间戳、URL
- 用户信息（头像已清理为高清 URL）
- 媒体信息（已清理冗余字段，图片追加 `?name=4096x4096` 高清参数）
- 完整的原始数据
- **`-jsonfile` 模式额外处理**：第三方新格式自动转换为 TMD 兼容旧格式（嵌套对象扁平化）

### 用途

- 即使下载失败也能记录推文信息，便于调试
- 可用于数据备份和迁移
- 便于第三方工具读取推文数据

### TXT 格式示例

```
time:2024-01-15T10:30:00
url:https://x.com/username/status/1234567890
media:2

这是推文的文本内容...
```

***

## 文件存储结构

```
{存储目录}/
├── users/                          # 用户目录
│   ├── Elon Musk(elonmusk)/        # 用户文件夹
│   │   ├── 推文媒体文件...         # -user/-jsonfile/-jsonfolder 媒体文件均在此
│   │   └── .loongtweet/           # 仅 -user 和 -jsonfile 创建
│   │       ├── {推文文本}_{tweetID}.json    # 推文 JSON（均已清理：-user cleanTweetJson / -jsonfile 格式转换+清理）
│   │       ├── {推文文本}_{tweetID}.txt     # 推文文本
│   │       └── .profile/            # 仅 -user 创建
│   │           ├── avatar.jpg
│   │           ├── banner.jpg
│   │           ├── description.txt
│   │           ├── profile.json
│   │           └── .versions/      # 历史版本
│   └── NASA(NASA)/
│       └── ...
├── .data/                          # 数据目录
    ├── foo.db                      # SQLite 数据库
    │                                 # 包含以下数据表：
    │                                 # - users: 用户信息（含 is_accessible 状态）
    │                                 # - lsts: 列表信息
    │                                 # - user_entities: 用户下载实体（含 media_count）
    │                                 # - lst_entities: 列表下载实体
    │                                 # - user_links: 用户链接关联
    │                                 # - user_previous_names: 用户历史名称（含 record_date）
    └── errors.json                 # 失败推文记录
```

***

## 使用场景与示例

### 场景1：首次使用

```bash
# 1. 配置
tmd -conf

# 2. 测试下载
tmd -user elonmusk -dbg
```

### 场景2：下载单个用户

```bash
# 下载推文 + Profile（默认行为）
tmd -user elonmusk

# 仅下载推文，不下载 Profile
tmd -user elonmusk -noprofile

# 使用数字用户名（如纯数字的 screen_name）
tmd -user 44196397

# 使用 @ 前缀
tmd -user @elonmusk
```

### 场景3：批量下载多个用户

```bash
# 下载多个用户的推文 + Profile
tmd -user elonmusk -user NASA -user SpaceX

# 下载多个用户的推文，不下载 Profile
tmd -user elonmusk -user NASA -user SpaceX -noprofile

# 仅下载多个用户的 Profile
tmd -profile-user elonmusk -profile-user NASA -profile-user SpaceX
```

### 场景4：下载列表

```bash
# 下载列表成员推文 + Profile
tmd -list 1234567890123

# 下载列表成员推文，不下载 Profile
tmd -list 1234567890123 -noprofile

# 仅下载列表成员 Profile
tmd -profile-list 1234567890123

# 多个列表
tmd -list 111111 -list 222222
```

### 场景5：下载关注列表

```bash
# 下载某用户关注的所有人
tmd -foll myusername
```

### 场景6：混合下载

```bash
# 用户 + 列表 + 关注列表
tmd -user elonmusk -list 123456 -foll myusername

# Profile 专用下载，只下载 profile
tmd -profile-user elonmusk -profile-list 123456
```

### 场景7：处理受保护用户

```bash
# 自动发送关注请求
tmd -user protected_user -auto-follow
```

### 场景8：标记已下载

```bash
# 标记为当前时间
tmd -user elonmusk -mark-downloaded

# 标记为指定时间
tmd -user elonmusk -mark-downloaded -mark-time "2024-01-01T00:00:00"

# 批量标记
tmd -user a -user b -user c -mark-downloaded
```

### 场景9：从 JSON 文件/文件夹下载

```bash
# 从第三方工具导出的推文搜索结果 JSON 下载推文媒体（图片/视频/txt/json）
tmd -jsonfile ./twitter-search-results-123.json

# 从多个 JSON 文件下载
tmd -jsonfile ./search1.json -jsonfile ./search2.json -jsonfile ./followers.json

# 从 TMD 生成的 .loongtweet 文件夹下载推文媒体（仅媒体，无元数据）
tmd -jsonfolder ./path/to/.loongtweet

# 从多个 .loongtweet 文件夹下载
tmd -jsonfolder ./folder1/.loongtweet -jsonfolder ./folder2/.loongtweet

# 注意：-jsonfile 和 -jsonfolder 是独占参数，优先级最高
# 以下命令只会执行 -jsonfile，-user 被忽略
tmd -jsonfile ./search.json -user elonmusk
```

**`-jsonfile` 输出示例**：
```
[screen_name] 推文文本内容_1234567890 [3/3 succeeded]
[screen_name] 另一条推文_1234567891 [2/2 succeeded]
JSON file download completed: 2 success, 0 failed, 5 media
```

**`-jsonfolder` 输出示例**：
```
[jsonfolder] .loongtweet: 8/10 tweets succeeded (2 failed)
LoongTweet folder download completed: 1 folder(s) processed, 8 succeeded, 2 failed, 15 media
```

> 💡 **推荐搭配**：使用 [twitter-web-exporter](https://github.com/prinsss/twitter-web-exporter) 浏览器脚本导出推文或用户列表为 JSON 格式，然后用 `-jsonfile` 或 `-jsonfolder` 参数下载。

### 场景10：调试与排错

```bash
# 调试模式
tmd -user elonmusk -dbg

# 快速退出（不重试）
tmd -user elonmusk -no-retry
```

***

## 高级设置

### 设置代理

支持三种代理方式：

**方式一：配置文件设置（推荐）**

在 `tmd -conf` 配置时输入 `proxy_url`，或直接编辑 `conf.yaml`：

```yaml
proxy_url: http://127.0.0.1:7890
```

支持的协议：`http://`、`https://`、`socks5://`

**方式二：环境变量设置**

运行前通过环境变量指定代理服务器（TUN 模式跳过这一步）

**Windows CMD:**

```bash
set HTTP_PROXY=http://127.0.0.1:7890
set HTTPS_PROXY=http://127.0.0.1:7890
tmd -user elonmusk
```

**Windows PowerShell:**

```powershell
$Env:HTTP_PROXY="http://127.0.0.1:7890"
$Env:HTTPS_PROXY="http://127.0.0.1:7890"
tmd -user elonmusk
```

**Linux/macOS:**

```bash
export HTTP_PROXY=http://127.0.0.1:7890
export HTTPS_PROXY=http://127.0.0.1:7890
tmd -user elonmusk
```

### 忽略用户

程序默认会忽略被静音或被屏蔽的用户，所以当你想要下载的列表中包含你不想包含的用户，可以在推特将他们屏蔽或静音。

### 添加额外 Cookie

程序动态从所有可用 cookie 中选择一个不会被速率限制的 cookie 请求用户推文，以避免因单一 cookie 的速率限制导致程序被阻塞。

按如下格式创建 `$HOME/.tmd2/additional_cookies.yaml` 或 `%appdata%/.tmd2/additional_cookies.yaml`：

```yaml
- auth_token: xxxxxxxxx1
  ct0: xxxxxxxxxxxxxxxxxxxxxxx
- auth_token: xxxxxxxxx2
  ct0: xxxxxxxxxxxxxxxx2
- auth_token: xxxxxxxxxxxxxxxx3
  ct0: xxxxxxxxxxxxxxxxxxxxx3
```

> 这些添加的备用 cookie，仅用来提升获取推文的速率和总量。判断是否忽略用户和自动关注受保护的用户依然使用主账号。

### 关于速率限制

Twitter API 限制一段时间内过快的请求（例如某端点每15分钟仅允许请求500次，超出这个次数会以429响应）。当某一端点将要达到速率限制程序会打印一条通知并阻塞尝试请求这个端点的协程直到余量刷新（这最多是15分钟），但并不会阻塞所有协程，所以其余协程打印的消息可能将这条休眠通知覆盖让人认为程序无响应了，等待余量刷新程序会继续工作。

### 启动脚本

项目提供 Server 模式的启动脚本：

**Windows (`start-server.bat`)**：

```bash
# 直接运行
start-server.bat

# 指定额外参数
start-server.bat -port 8080
```

> 脚本行为：自动查找同目录下的 `tmd.exe` 并以 `-server` 模式启动，额外参数会透传给 tmd。

***

## 参数兼容性速查表

| 组合                                    |  兼容 | 说明                      |
| ------------------------------------- | :-: | ----------------------- |
| `-user` + `-list` + `-foll`           |  ✅  | 多种来源可叠加                 |
| `-user` + `-list` + `-foll` + `-jsonfile` |  ⚠️  | **仅执行 `-jsonfile`**（高优先级独占） |
| `-user` + `-list` + `-foll` + `-jsonfolder` |  ⚠️  | **仅执行 `-jsonfolder`**（高优先级独占） |
| `-jsonfile` + `-noprofile`            |  ⚠️  | **仅执行 `-jsonfile`**（高优先级独占） |
| `-jsonfolder` + `-noprofile`           |  ⚠️  | **仅执行 `-jsonfolder`**（高优先级独占） |
| `-user` + Profile 自动下载                |  ✅  | 下载推文时自动下载 Profile       |
| `-list` + Profile 自动下载                |  ✅  | 下载列表成员推文时自动下载 Profile   |
| `-foll` + Profile 自动下载                |  ✅  | 下载关注用户推文时自动下载 Profile   |
| `-profile-user` + `-profile-list`     |  ✅  | 仅下载资料，不下载推文             |
| `-user` + `-profile-user`             |  ✅  | 推文下载 + 额外用户资料           |
| `-dbg` + 任意参数                         |  ✅  | 启用调试输出                  |
| `-auto-follow` + 推文下载                 |  ✅  | 自动关注受保护用户               |
| `-no-retry` + 推文下载                    |  ✅  | 失败不重试                   |
| `-mark-downloaded` + `-mark-time`     |  ✅  | 指定标记时间                  |
| `-mark-downloaded` + 推文下载             |  ⚠️  | **仅执行标记，不下载推文**（与稳定版不同） |
| `-jsonfile` + `-mark-downloaded`        |  ⚠️  | **仅执行 `-jsonfile`**（高优先级独占） |
| `-jsonfolder` + `-mark-downloaded`      |  ⚠️  | **仅执行 `-jsonfolder`**（高优先级独占） |
| `-conf` + 其他参数                        |  ⚠️ | CLI 模式：配置后退出，忽略其他；Server 模式：配置后启动 Server |
| `-noprofile` + 推文下载参数                 |  ✅  | 下载推文但跳过 Profile         |
| `-server` + `-port`                   |  ✅  | 指定 API Server 端口        |
| `-server` + 下载参数                      |  ⚠️ | Server 模式下忽略下载参数        |
| `-server` + `-conf`                   |  ⚠️ | 配置后启动 Server           |

***
## 项目架构

本项目当前的主干调用关系是：

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

整体上采用“入口层 + 应用服务层 + 业务层 + 基础设施层”的结构，核心复用点是 **`internal/service`**：CLI 和 API Server 共用同一套下载编排逻辑，避免两边各自维护一份下载流程。

```
┌─────────────────────────────────────────────────────────────┐
│  main.go (应用入口层)                                        │
│  - 预解析全局参数                                            │
│  - 初始化配置、日志、数据库、Twitter 客户端                    │
│  - 模式选择：Server / CLI                                    │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────────┐
│  internal/config (配置层)                                   │
│  - config.go: 配置结构、读写、环境变量覆盖、交互式配置          │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼───────────────────┐  ┌────────────────────────────▼┐
│  internal/twitter            │  │  internal/database          │
│  (API 客户端层)               │  │  (数据持久化层)              │
│                              │  │                             │
│  - client.go: 登录与客户端管理 │  │  - connect/sqlite/schema     │
│  - api.go: 请求封装与通用能力  │  │  - sqlite_schema/sqlite_migration │
│  - user/tweet/list/timeline  │  │  - model/query/helpers       │
│  - batch_login.go: 多账号     │  │  - user/lst/user_entity/lst_entity │
│                              │  │  - user_link/user_sync       │
│                              │  │  - parent_dir_migration/path_validation │
│                              │  │  - tx/manager                │
└──────────┬───────────────────┘  └─────────────┬───────────────┘
           │                                    │
           └──────────────┬─────────────────────┘
                          │
          ┌───────────────▼───────────────────────┐
          │  🎯 internal/service (Service 层)     │
          │        ★ 核心业务编排层 ★             │
          │                                       │
          │  - interfaces.go: DownloadService 接口 │
          │  - download_service.go: 用户/列表/关注/ │
          │    JSON/Profile/重试等统一入口          │
          │  - deps.go: 依赖注入与构造              │
          │  - progress.go: 进度上报               │
          └───────────────┬───────────────────────┘
                          │
          ┌───────────────┼───────────────────────────────┐
          │               │                               │
┌─────────▼────────┐ ┌────▼───────────┐         ┌─────────▼─────────┐
│  internal/api    │ │  internal/cli  │         │  internal/path    │
│  (Server 层)      │ │  (CLI 层)       │         │  (路径工具)        │
│                   │ │                 │         │                   │
│  - server.go      │ │  - args.go      │         │  - store.go       │
│  - download_*     │ │  - executor.go  │         │                   │
│  - task_manager   │ │                 │         │                   │
│  - download_queue │ │                 │         │                   │
│  - progress / sse │ │                 │         │                   │
│  - db/config/     │ │                 │         │                   │
│    cookie/log_*   │ │                 │         │                   │
│  - scheduler_*    │ │                 │         │                   │
│  - event_bus.go   │ │                 │         │                   │
└────────┬─────────┘ └─────────────────┘         └───────────────────┘
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
│  - 负责抓取、同步实体、组织批量下载、失败重试                   │
├─────────────────────────────────────────────────────────────┤
│  internal/downloading/profile (Profile 业务子包)             │
│  - downloader.go / storage.go / types.go                    │
│  - 负责头像、横幅、简介、profile.json 与版本备份               │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────────┐
│  internal/entity (数据实体层)                                │
│  - interface.go / user.go / list.go / sync.go               │
├─────────────────────────────────────────────────────────────┤
│  internal/downloader (基础设施层 - 通用下载)                  │
│  - downloader.go: 单文件下载、流式下载、大小校验               │
│  - file_writer.go: 原子写入、跳过未变化文件、并发锁管理         │
│  - version_manager.go: 版本备份管理                          │
├─────────────────────────────────────────────────────────────┤
│  internal/naming (命名服务)                                  │
│  - base.go / tweet_naming.go / user_naming.go / list_naming.go │
├─────────────────────────────────────────────────────────────┤
│  internal/scheduler (定时任务调度器)                         │
│  - scheduler.go: 调度执行与状态维护                           │
│  - types.go: ScheduleEntry / ScheduleStatus / ParsedSchedule│
├─────────────────────────────────────────────────────────────┤
│  internal/consolelog (控制台日志)                            │
│  - hub.go: 日志捕获和分发中心，支持 SSE 实时推送               │
├─────────────────────────────────────────────────────────────┤
│  internal/utils (工具层)                                     │
│  - fs.go / http.go / algo.go / time_range.go / recovery.go  │
│  - user.go / win32.go (Windows) / stub.go (!Windows)        │
└─────────────────────────────────────────────────────────────┘
```

### 核心设计原则

| 原则 | 实现 |
| --- | --- |
| **Service 复用** | CLI 和 API 统一走 `DownloadService`，避免重复维护下载逻辑 |
| **入口与业务分离** | `internal/cli` / `internal/api` 只负责参数、HTTP、任务编排和响应 |
| **业务与下载器分离** | `internal/downloading` 负责“下载什么、按什么流程下载”，`internal/downloader` 负责“文件怎么下、怎么写” |
| **数据库集中管理** | SQLite schema、迁移、查询和实体同步集中在 `internal/database` |
| **任务异步化** | Server 模式通过 TaskManager、DownloadQueue、SSE 推进长任务 |
| **增量与重试** | 基于数据库和 `.data/errors.json` 做增量抓取与失败重试 |
| **跨入口一致性** | 用户下载、列表下载、关注下载、JSON 导入、Profile 下载都通过 service 层统一暴露 |

***

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

***

## 常见问题

### Q: 如何查看失败的下载？

失败的任务保存在 `{存储目录}/.data/errors.json`，下次运行会自动重试。

### Q: Profile 文件存在时还会重新下载吗？

如果文件内容未变更（MD5校验），会自动跳过。

### Q: 如何更新已下载用户的 Profile？

重新运行相同的命令即可，只会下载变更的文件。

### Q: 下载中断后怎么办？

直接重新运行相同命令，程序会自动恢复。

### Q: `-mark-downloaded` 的用途？

用于标记用户为"已下载到最后"，下次运行时不会下载历史推文，只下载新推文。

### Q: 如何获取列表ID？

在 Twitter 网页版打开列表，URL 格式为：

```
https://x.com/i/lists/1234567890123
```

其中数字就是列表ID。

### Q: 不知道啥是 user\_id/list\_id/screen\_name?

请参考 [获取 list\_id, user\_id, screen\_name](https://github.com/unkmonster/tmd/blob/master/doc/help.md#获取-list_id-user_id-screen_name)

### Q: Windows 上需要管理员权限吗？

为了创建符号链接，在 Windows 上应该以管理员身份运行程序。

### Q: 推文 JSON 文件有什么用？

即使媒体下载失败，推文信息也会保存到 `.loongtweet/` 目录。JSON 文件包含完整的推文数据，可用于数据分析或备份。

### Q: `-jsonfile` 保存的 JSON 与 `-user` 的有什么区别？

两者最终格式一致，但处理流程不同：
- **`-user`**：直接调用 `cleanTweetJson` 清理冗余字段
- **`-jsonfile`**：先经过 `ConvertThirdPartyTweetJSON` 格式转换（第三方新格式 → TMD 兼容旧格式），再由 `cleanTweetJson` 清理

转换规则包括：嵌套对象扁平化（如 `RelationshipPerspectives` → `legacy` 扁平字段）、头像 `_normal` 后缀移除、图片 URL 追加高清参数等。

### Q: `-jsonfile` 和 `-jsonfolder` 的区别？

| 特性 | `-jsonfile` | `-jsonfolder` |
|------|------------|--------------|
| 输入来源 | 第三方工具导出的 JSON 文件 | TMD 生成的 `.loongtweet/` 文件夹 |
| 保存元数据 | ✅ 保存 `.json` + `.txt` | ❌ 不保存 |
| 格式转换 | ✅ 新格式→旧格式 | 不需要（已是 TMD 格式） |
| 适用场景 | 首次从第三方导入 | 重新下载/迁移媒体文件 |

***

## 输出结果格式

### 推文下载结果

```
users: 3
    - Elon Musk(elonmusk)
    - NASA(NASA)
    - SpaceX(SpaceX)
```

### Profile 下载结果

```
=== PROFILE_DOWNLOAD_RESULTS ===
SCREEN_NAME:elonmusk|STATUS:OK
SCREEN_NAME:NASA|STATUS:OK
SCREEN_NAME:SpaceX|STATUS:SKIP
SCREEN_NAME:test|STATUS:FAIL
=== END_RESULTS ===
```

状态说明：

- `OK` - 下载成功
- `SKIP` - 跳过（文件未变更）
- `FAIL` - 下载失败

### 标记结果

```
=== MARK_DOWNLOADED_RESULTS ===
ENTITY_ID:1|USER_ID:44196397|SCREEN_NAME:elonmusk|STATUS:OK
ENTITY_ID:2|USER_ID:23248887|SCREEN_NAME:NASA|STATUS:OK
=== END_RESULTS ===
```

***

## 参数类型总结

### 布尔型参数（开关型，无需值）

| 参数                 | 说明               |
| ------------------ | ---------------- |
| `-conf`            | 重新配置             |
| `-dbg`             | 调试模式             |
| `-server`          | 启动 API Server 模式 |
| `-auto-follow`     | 自动关注受保护用户        |
| `-follow-members`  | 下载时关注目标/成员        |
| `-no-retry`        | 不重试失败推文          |
| `-mark-downloaded` | 仅标记已下载           |
| `-noprofile`       | 跳过 Profile 下载    |

### 可重复参数（可多次使用）

| 参数              | 说明                 |
| --------------- | ------------------ |
| `-user`         | 用户名/ID             |
| `-list`         | 列表ID               |
| `-foll`         | 用户名/ID             |
| `-jsonfile`     | 第三方工具导出的 JSON 文件路径   |
| `-jsonfolder`   | TMD 生成的 `.loongtweet` 文件夹路径 |
| `-profile-user` | 用户名/ID             |
| `-profile-list` | 列表ID               |

### 字符串参数

| 参数           | 说明                       |
| ------------ | ------------------------ |
| `-mark-time` | 时间戳（2006-01-02T15:04:05） |

***

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
├── .github/workflows/           # CI/CD 配置
└── test/                        # 集成测试
```

### 运行测试

项目包含 **49 个测试文件**，覆盖核心业务逻辑：

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
- **编码准则**: 参考 [CLAUDE.md](./CLAUDE.md) 中的 AI 编码准则
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
  1. 多平台构建 (Windows / Linux / macOS)
  2. 运行测试套件 (go test -race)
  3. 上报覆盖率到 Coveralls
  4. 发布版本时自动创建 Release
```

***

## 安全说明

### Cookie 安全 ⚠️

`auth_token` 和 `ct0` 相当于你的 **Twitter 登录凭证**，请务必妥善保管！

**安全建议：**

- ❌ **不要**将配置文件提交到公开 Git 仓库（已在 `.gitignore` 排除）
- ❌ **不要**分享包含真实 Cookie 的配置文件或截图
- ❌ **不要**在日志或调试信息中暴露完整 Cookie
- ✅ 定期更新 Cookie（Twitter 可能会使其失效或定期轮换）
- ✅ 使用 `tmd -conf` 安全更新配置，避免手动编辑出错
- ✅ 仅在可信设备上运行程序

**Cookie 存储位置：**

| 平台 | 路径 | 权限 |
|------|------|------|
| Windows | `%APPDATA%\.tmd2\conf.yaml` | 当前用户 |
| macOS/Linux | `~/.tmd2/conf.yaml` | 当前用户 (600) |

### 权限要求

| 操作系统 | 特殊权限 | 原因 |
|---------|---------|------|
| **Windows** | 管理员权限 | 创建符号链接需要 SeCreateSymbolicLinkPrivilege |
| **Linux/macOS** | 文件系统写入权限 | 写入存储目录和数据库文件 |

> 💡 **提示**: Windows 用户可以右键点击 `tmd.exe` → "以管理员身份运行"，或在管理员 PowerShell 中执行。

### 数据隐私

所有下载的数据**仅存储在本地**，不会上传到任何第三方服务器：

```
{存储目录}/
├── users/              # 推文媒体文件（图片/视频/GIF）
│   └── {用户名}/
│       ├── .loongtweet/   # 推文元数据（JSON/TXT）
│       │   └── .profile/ # 用户资料（头像/横幅/简介）
│       └── {日期}/        # 按日期组织的媒体文件
├── .data/
│   ├── foo.db          # SQLite 数据库（用户/列表/实体关系）
│   └── errors.json     # 失败推文记录
└── ...
```

**数据保护建议：**
- 定期备份 `{存储目录}` 和 `.data/foo.db`
- 敏感数据（如受保护用户的推文）注意访问控制
- 删除用户数据时同时清理数据库记录

### API Server 安全

当前版本 API Server **无需认证**，适用于本地使用：

**生产环境安全加固方案：**

```nginx
# Nginx 反向代理示例 - 添加 Basic Auth
server {
    listen 8080;
    
    location /api/v1/ {
        auth_basic "TMD API";
        auth_basic_user_file /etc/nginx/.htpasswd;
        
        proxy_pass http://127.0.0.1:25556;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        
        # 限制请求速率
        limit_req zone=api burst=20 nodelay;
    }
}
```

**推荐安全措施：**
1. **网络隔离**: 仅绑定到 localhost (`127.0.0.1:25556`)
2. **反向代理**: 使用 Nginx/Caddy 添加认证层
3. **IP 白名单**: 防火墙限制访问来源 IP
4. **HTTPS**: 公网部署时强制 TLS 加密
5. **速率限制**: 防止 API 滥用

***

## 性能参考

### 下载速度调优

| 并发数 | 适用场景 | 带宽占用 | 推荐度 |
|-------|---------|---------|--------|
| **10-20** | 家庭网络 / 共享网络 | 低 (5-20 Mbps) | ⭐⭐⭐ 稳定首选 |
| **20-35** | 企业网络 / 独享带宽 | 中 (20-50 Mbps) | ⭐⭐⭐⭐ **推荐值** |
| **35-50** | 服务器 / VPS / 高速宽带 | 高 (50-100 Mbps) | ⭐⭐ 需要优质网络 |

**配置方法：**

```bash
# 方式1: 首次配置时设置
tmd -conf
# 输入 max download routine: 35

# 方式2: 修改配置文件后更新
tmd -conf
# 仅修改需要调整的字段，其他留空保持原值
```

### 资源占用参考

| 资源类型 | 空闲状态 | 下载中（并发35） | 备注 |
|---------|---------|-----------------|------|
| **内存** | ~40-60 MB | ~100-200 MB | 取决于并发数和文件大小 |
| **CPU** | < 1% | 5-15% | 单核即可满足 |
| **磁盘 I/O** | 极低 | 中等 | SSD 推荐用于大文件下载 |
| **网络连接** | 1 个（登录） | 35+ 个 | 每个媒体文件一个连接 |
| **数据库** | ~5 MB | ~50-200 MB | SQLite，无需额外服务 |

### 性能优化特性

TMD 内置多项性能优化机制：

#### 1. 流式下载（v2.12.3+）

自动根据文件大小选择最优策略：

```
文件大小 < 10MB → Buffer 模式（内存缓冲，支持 MD5 去重）
文件大小 ≥ 10MB → 流式模式（分块写入，节省内存）
```

**优势：**
- 大视频文件不再占用大量内存
- 实时进度跟踪
- 失败时仅重试未完成部分

#### 2. 增量下载

基于 `latest_release_time` 时间戳的智能增量拉取：

```sql
-- 仅获取比上次更新的推文
WHERE created_at > '2024-01-15 10:30:00'
```

**效果：**
- 首次运行：全量下载用户所有推文
- 后续运行：仅下载新增推文（通常几分钟完成）
- 节省 API 配额和网络带宽

#### 3. MD5 去重

相同内容的文件自动跳过：

```go
// 文件写入前计算 MD5
if fileWriter.Exists(md5Hash) {
    log.Info("File already exists, skipping...")
    return nil  // 跳过重复下载
}
```

**适用场景：**
- 重试失败任务时跳过已成功的文件
- 多列表包含同一用户时避免重复保存
- Profile 未变更时自动跳过

#### 4. 符号链接去重

多列表包含同一用户时使用符号链接：

```
lists/科技圈/users/ -> ../../users/Elon Musk(elonmusk)/
lists/新闻/users/   -> ../../users/Elon Musk(elonmusk)/
```

**节省空间：**
- 无论多少列表包含同一用户，本地仅保留一份存档
- 显著减少磁盘空间占用（尤其对于热门用户）

### 性能瓶颈与解决方案

| 瓶颈 | 表现 | 解决方案 |
|------|------|---------|
| **Twitter API 速率限制** | 日志显示 `rate limit` 提示 | 添加备用 Cookie（`additional_cookies.yaml`） |
| **磁盘 I/O 瓶颈** | 下载速度远低于带宽 | 使用 SSD 存储，或降低并发数 |
| **网络延迟高** | 单个文件下载时间长 | 检查代理设置，或启用调试模式 (`-dbg`) 查看请求耗时 |
| **内存不足** | 系统卡顿或 OOM | 降低 `max_download_routine` 到 10-20 |

### 监控与诊断

```bash
# 启用调试模式查看详细性能指标
tmd -user elonmusk -dbg

# 输出示例：
# [INFO] Download routine count: 35
# [INFO] Total requests: 150
# [INFO] Success rate: 98.5%
# [INFO] Average download speed: 2.3 MB/s
# [INFO] Total time: 5m 23s
```

***

## 故障排除进阶

### 常见错误码速查

| HTTP 状态码 | 错误类型 | 原因 | 解决方案 |
|------------|---------|------|---------|
| **429** | Too Many Requests | 触发 Twitter API 速率限制 | 等待 15 分钟自动恢复；或添加备用 Cookie |
| **401** | Unauthorized | Cookie 失效或过期 | 运行 `tmd -conf` 更新 Cookie |
| **403** | Forbidden | 用户受保护且未关注 | 使用 `-auto-follow` / `-follow-members` 或手动关注后重试 |
| **404** | Not Found | 用户不存在/已注销/被封禁 | 检查用户名是否正确；用户可能已被封禁 |
| **500** | Internal Server Error | Twitter 服务器内部错误 | 稍后自动重试；检查网络连接 |
| **503** | Service Unavailable | Twitter 服务暂时不可用 | 等待服务恢复后重试 |
| **connection reset** | 网络连接中断 | 代理不稳定或网络波动 | 检查代理设置；启用 `-no-retry` 快速测试 |

### 调试技巧集锦

#### 基础调试

```bash
# 1. 启用调试模式（查看请求计数和详细日志）
tmd -user elonmusk -dbg

# 2. 快速退出模式（不重试失败项，快速验证配置）
tmd -user elonmusk -no-retry

# 3. 仅标记不下载（测试同步逻辑，不实际下载文件）
tmd -user elonmusk -mark-downloaded

# 4. 指定标记时间（回溯到特定时间点）
tmd -user elonmusk -mark-downloaded -mark-time "2024-01-01T00:00:00"
```

#### 高级诊断

```bash
# 5. 测试单用户下载（最小化变量）
tmd -user elonmusk -noprofile -dbg

# 6. 检查 API Server 是否正常
tmd -server
# 然后在浏览器访问 http://localhost:25556/api/v1/health

# 7. 查看数据库内容（确认同步状态）
sqlite3 .data/foo.db "SELECT screen_name, latest_release_time FROM users;"

# 8. 检查失败记录
cat .data/errors.json | head -20
```

#### 网络问题排查

```bash
# 9. 测试代理连通性（Windows PowerShell）
$Env:HTTP_PROXY="http://127.0.0.1:7890"
$Env:HTTPS_PROXY="http://127.0.0.1:7890"
tmd -user elonmusk -dbg

# 10. 绕过代理直连（TUN 模式下不需要设置代理）
# 直接运行 tmd，不设置 HTTP_PROXY/HTTPS_PROXY
```

### 日志系统详解

#### 日志位置

| 平台 | 主日志路径 | CLI 输出日志 |
|------|----------|-------------|
| **Windows** | `%APPDATA%\.tmd2\tmd2.log` | `%APPDATA%\.tmd2\client.log` |
| **macOS/Linux** | `~/.tmd2/tmd2.log` | `~/.tmd2/client.log` |

#### 日志轮转配置

程序使用 [lumberjack](https://github.com/natefinch/lumberjack) 进行日志轮转：

| 配置项 | 当前值 | 说明 |
|--------|-------|------|
| 单文件最大 | **2 MB** | 防止单个日志文件过大 |
| 保留份数 | **2** | 最多保留 2 个历史日志文件 |
| 保留天数 | **14 天** | 自动清理 14 天前的日志 |
| 压缩 | ❌ 关闭 | 不压缩历史日志（便于查看） |

#### 日志级别

```bash
# 默认级别：Info（显示重要信息）
tmd -user elonmusk

# 调试级别：Debug（显示所有请求详情）
tmd -user elonmusk -dbg
```

**Debug 模式额外输出：**
- 每个 Twitter API 请求的 URL 和响应时间
- 总请求数统计（`twitter.ReportRequestCount()`）
- 数据库查询详情
- 文件写入操作日志

### 典型问题场景与解决方案

#### 场景 1：首次使用完全无法下载

**症状：**
```
[ERROR] failed to login: invalid cookie or token
```

**排查步骤：**
1. ✅ 确认 Cookie 正确性（重新从浏览器复制）
2. ✅ 检查 Cookie 是否过期（Twitter 会定期刷新）
3. ✅ 尝试重新配置：`tmd -conf`
4. ✅ 确认网络可以访问 Twitter（非墙内环境）

---

#### 场景 2：下载一段时间后停止

**症状：**
```
[WARN] rate limit approaching, sleeping for 5m0s...
```

**原因：** 触发 Twitter API 速率限制（每 15 分钟 500 次请求）

**解决方案：**
- **短期**：等待 15 分钟自动恢复
- **长期**：添加备用 Cookie 到 `additional_cookies.yaml`
- **优化**：降低并发数到 10-20

---

#### 场景 3：符号链接创建失败（Windows）

**症状：**
```
[ERROR] failed to create symlink: A required privilege is not held by the client
```

**原因：** Windows 需要管理员权限才能创建符号链接

**解决方案：**
1. 右键点击 `tmd.exe` → **"以管理员身份运行"**
2. 或在管理员 PowerShell 中执行：
   ```powershell
   Start-Process tmd.exe -Verb RunAs -ArgumentList "-user elonmusk"
   ```

---

#### 场景 4：大文件下载失败

**症状：**
```
[ERROR] download failed: context deadline exceeded
[WARN] retrying tweet 1234567890 with 3 media(s)
```

**原因：** 大视频文件下载超时或网络不稳定

**解决方案：**
1. 启用调试模式查看具体耗时：`-dbg`
2. 降低并发数减少带宽竞争
3. 检查磁盘空间是否充足
4. 使用 `-no-retry` 快速定位问题文件
