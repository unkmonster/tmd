# TMD API 认证/授权层

> 本文档详细说明 TMD (Twitter Media Downloader) 中 API 认证/授权层的设计原理、配置方法、认证流程和前端集成。

---

## 目录

1. [功能概述](#功能概述)
2. [配置说明](#配置说明)
3. [架构与中间件链](#架构与中间件链)
4. [认证流程](#认证流程)
5. [公开路径白名单](#公开路径白名单)
6. [SSE 端点的特殊处理](#sse-端点的特殊处理)
7. [API 使用示例](#api-使用示例)
8. [前端认证流程](#前端认证流程)
9. [配置持久化](#配置持久化)
10. [环境变量覆盖](#环境变量覆盖)
11. [安全注意事项](#安全注意事项)
12. [实现参考](#实现参考)

---

## 功能概述

TMD 默认的 HTTP API Server 端口是全开放的，任何能访问该端口的设备都可以调用所有 API，包括执行下载、管理数据、修改配置等敏感操作。这在以下场景中构成安全风险：

- **公网部署**：将 TMD 暴露到公网时，任何人都可以控制下载任务
- **内网多设备访问**：同一内网的其他设备可无限制调用 API

**认证层的目标**：在保持向后兼容的前提下，为 API 添加 Bearer Token 认证，同时确保 Web UI 的正常使用不受影响。

### 核心设计原则

| 原则 | 说明 |
|------|------|
| **向后兼容** | `api_key` 配置为空时，认证层完全跳过，行为零变化 |
| **最小侵入** | 只需在 `conf.yaml` 中加一行配置即可开启 |
| **SPA 兼容** | Web UI 页面（HTML/CSS/JS）不受认证限制，API 调用才需要认证 |
| **SSE 兼容** | EventSource 无法设置自定义 HTTP 头，通过 `?token=` 查询参数回退 |

---

## 配置说明

### conf.yaml

在 `conf.yaml` 中添加 `api_key` 字段：

```yaml
root_path: "D:/TwitterMedia"
cookie:
  auth_token: "..."
  ct0: "..."
api_key: "your-secret-api-key"    # ← 新增：API Bearer Token
max_download_routine: 20
proxy_url: "http://127.0.0.1:7897"
```

`api_key` 为空字符串（或不设置）时，认证层不生效，所有请求照常放行。

### 配置字段元信息

| 字段 | 类型 | 环境变量 | 默认值 | 分组 |
|------|------|---------|-------|------|
| `api_key` | `string` | `TMD_API_KEY` | `""`（空 = 禁用） | `security` |

### Web UI 配置编辑

`api_key` 在系统设置的配置编辑器中以 **password** 类型显示（值被脱敏，如 `abc•••xyz`），归属于 `security` 分组。通过 Web UI 保存后**立即生效**，无需重启。

---

## 架构与中间件链

### 中间件包裹顺序

认证中间件被放置在 CORS 中间件**内层**（即 CORS 先处理请求），这是为了保证 OPTIONS 预检请求不被认证拦截。

```
请求进入
  │
  ▼
┌─────────────────────┐
│ securityHeaders     │  ← 最外层：设置 X-Content-Type-Options 等安全头
├─────────────────────┤
│ loggingMiddleware   │  ← 记录所有请求日志（包括认证失败的 401）
├─────────────────────┤
│ apiVersion          │  ← 设置 API-Version: v1 响应头
├─────────────────────┤
│ CORS middleware     │  ← 处理 OPTIONS 预检直接返回，不进入内层
├─────────────────────┤
│ authMiddleware      │  ← 检查 Authorization 头或 ?token= 参数
├─────────────────────┤
│ ServeMux (路由器)    │  ← 最内层：分发到具体 handler
└─────────────────────┘
```

**关键设计**：
- OPTIONS 预检请求由 CORS 中间件直接处理（返回 204），**不会经过 authMiddleware**，因此无需在预检中携带 token
- 认证失败的请求仍然被 `loggingMiddleware` 记录，便于排查问题
- `authMiddleware` 在 `ServeMux` 之前，对所有路由统一检查，无需每个 handler 单独处理

### 认证中间件代码位置

- **实现**：`internal/api/middleware.go` — `Server.authMiddleware()` 方法
- **注入**：`internal/api/server.go` — `buildHandler()` 函数

---

## 认证流程

### 正常认证流程

```
客户端                                    TMD Server
  │                                           │
  │  GET /api/v1/tasks                        │
  │  Authorization: Bearer <key>              │
  │──────────────────────────────────────────►│
  │                                           │  ┌─ authMiddleware
  │                                           │  ├─ extractBearerToken()
  │                                           │  ├─ token == config.APIKey? → 通过
  │                                           │  └─ 放行到 ServeMux
  │                                           │
  │  HTTP 200 + JSON 数据                      │
  │◄──────────────────────────────────────────│
  │                                           │
```

### 认证失败流程

```
客户端                                    TMD Server
  │                                           │
  │  GET /api/v1/tasks                        │
  │  (无 Authorization 头)                     │
  │──────────────────────────────────────────►│
  │                                           │  ┌─ authMiddleware
  │                                           │  ├─ extractBearerToken() → ""
  │                                           │  ├─ r.URL.Query().Get("token") → ""
  │                                           │  ├─ token != APIKey → 拒绝
  │                                           │  └─ 返回 401
  │                                           │
  │  HTTP 401                                 │
  │  WWW-Authenticate: Bearer realm="TMD API" │
  │  {"success":false,"error":"unauthorized"} │
  │◄──────────────────────────────────────────│
  │                                           │
```

### 认证方式优先级

1. **`Authorization` HTTP 头**（首选）：`Authorization: Bearer <token>`
   - 用于所有常规 API 调用（`fetch`、`curl`、axios 等）
   - SSE `EventSource` 无法使用此方式（JS API 限制）
2. **`?token=` 查询参数**（回退）：`GET /api/v1/sse/tasks?token=<key>`
   - 专为 SSE `EventSource` 设计的回退方案
   - 也适用于无法设置自定义 HTTP 头的场景

### token 提取逻辑

```go
// 1. 尝试从 Authorization 头提取
token = extractBearerToken(r)  // 解析 "Bearer <token>"

// 2. 头为空时回退到查询参数
if token == "" {
    token = r.URL.Query().Get("token")
}

// 3. 与配置中的 APIKey 比较
if token != apiKey {
    // 返回 401
}
```

---

## 公开路径白名单

以下路径**不需要认证**，直接放行。这是为了保证 Web UI 可以正常加载。

| 路径 | 原因 |
|------|------|
| `GET /` | Web UI 首页（SPA 入口） |
| `GET /favicon.ico` | 浏览器图标请求 |
| `GET /tasks` | SPA 页面路由 |
| `GET /data` | SPA 页面路由 |
| `GET /schedules` | SPA 页面路由 |
| `GET /system` | SPA 页面路由 |
| `GET /logs` | SPA 页面路由 |
| `GET /api/v1/health` | 健康检查（用于 Docker healthcheck、负载均衡探测） |
| `GET /api/v1/config/theme` | 获取当前主题（主题切换器内联 JS 调用） |
| `POST /api/v1/config/theme` | 切换主题（主题切换器内联 JS 调用） |
| `GET /api/v1/config/themes` | 列出可用主题（主题切换器内联 JS 调用） |
| `GET /static/*` | 静态文件（JS、CSS、图片等） |

**为什么 Web UI 页面需要免认证？**

SPA（单页应用）的认证流程是：页面加载 → JS 执行 → 发现 API 返回 401 → 弹认证对话框 → 用户输入 Key → localStorage 存储 → 后续请求携带 token。

如果页面本身需要认证才能加载，用户将**永远看不到登录界面**（经典的"鸡生蛋"问题）。

---

## SSE 端点的特殊处理

### 问题

浏览器 `EventSource` API 无法设置自定义 HTTP 头：

```javascript
// ❌ 无法设置 Authorization 头
const es = new EventSource('/api/v1/sse/tasks', {
    headers: { 'Authorization': 'Bearer xxx' } // EventSource 忽略此参数
});
```

### 解决方案

SSE 端点支持 `?token=` 查询参数作为认证回退：

```javascript
// ✅ 使用查询参数
const key = localStorage.getItem('tmd_api_key');
const es = new EventSource('/api/v1/sse/tasks?token=' + encodeURIComponent(key));
```

### 受影响的 SSE 端点

| 端点 | 用途 |
|------|------|
| `GET /api/v1/sse/tasks` | 任务状态和调度状态实时推送 |
| `GET /api/v1/logs/stream?level=...&q=...` | 日志实时流 |

这两个端点在 authMiddleware 中检查 `?token=` 参数，**不在公开路径白名单中**。

---

## API 使用示例

### 前提

假设 `conf.yaml` 中配置了 `api_key: "my-secret-key-123"`，Server 运行在 `http://localhost:25556`。

### 1. 正确认证

```bash
curl -H "Authorization: Bearer my-secret-key-123" \
  http://localhost:25556/api/v1/tasks
```

**预期**：HTTP 200，返回任务列表 JSON。

### 2. 未认证请求

```bash
curl http://localhost:25556/api/v1/tasks
```

**预期**：HTTP 401，响应体：
```json
{
  "success": false,
  "error": "unauthorized"
}
```

响应头包含：
```
WWW-Authenticate: Bearer realm="TMD API"
```

### 3. 错误 Token

```bash
curl -H "Authorization: Bearer wrong-key" \
  http://localhost:25556/api/v1/tasks
```

**预期**：HTTP 401。

### 4. 缺少 Bearer 前缀

```bash
curl -H "Authorization: my-secret-key-123" \
  http://localhost:25556/api/v1/tasks
```

**预期**：HTTP 401（必须使用 `Bearer ` 前缀）。

### 5. 公开路径（免认证）

```bash
# 健康检查
curl http://localhost:25556/api/v1/health

# Web UI 首页
curl http://localhost:25556/

# 静态文件
curl http://localhost:25556/static/app.js
```

**预期**：HTTP 200，无需携带 token。

### 6. SSE 实时推送（带 token）

```bash
curl -N "http://localhost:25556/api/v1/sse/tasks?token=my-secret-key-123"
```

**预期**：持续的 SSE 流，包含任务和调度状态事件。

### 7. 日志流（带 token）

```bash
curl -N "http://localhost:25556/api/v1/logs/stream?token=my-secret-key-123"
```

**预期**：持续的 SSE 流，包含实时日志。

### 8. CORS 预检请求

```bash
curl -X OPTIONS \
  -H "Origin: http://example.com" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: authorization" \
  http://localhost:25556/api/v1/tasks
```

**预期**：HTTP 204，返回 `Access-Control-Allow-Origin: *`，**无需 token**。CORS 中间件在内层 auth 之前处理 OPTIONS。

---

## 前端认证流程

### 首次加载（未认证）

```
浏览器打开 http://localhost:25556/
  │
  ├─ 页面 HTML/JS/CSS 加载 ✓（公开路径）
  │
  ├─ sseManager.connect()
  │   └─ EventSource('/api/v1/sse/tasks') → 401（因无 token）
  │
  ├─ api.getTasks()
  │   └─ fetch('/api/v1/tasks') → 401
  │       └─ 错误对象: { status: 401, _isUnauthorized: true }
  │
  └─ init() 捕获到 401
      └─ showAuthDialog()
          └─ 弹出认证对话框
              ├─ 输入 API Key
              └─ submitAuthKey()
                  ├─ localStorage.setItem('tmd_api_key', key)
                  └─ window.location.reload()
```

### 重新加载（已认证）

```
浏览器打开 http://localhost:25556/
  │
  ├─ 页面加载 ✓
  │
  ├─ sseManager.connect()
  │   └─ EventSource('/api/v1/sse/tasks?token=xxx') → SSE 连接成功 ✓
  │
  ├─ api.getTasks()
  │   └─ options.headers['Authorization'] = 'Bearer xxx'
  │   └─ fetch('/api/v1/tasks') → 200 ✓
  │
  └─ 页面正常渲染，所有功能可用
```

### 认证对话框 UI

认证对话框在两个主题中采用不同实现：

- **web1 主题**：使用独立的 `.auth-overlay` + `.auth-modal` CSS（非 Drawer），输入框为 `password` 类型，带 👁️ 切换显示按钮
- **web2 主题**：复用已有 `openModal()` 基础设施，呈现为居中模态框（`.modal-overlay` + `.modal`），输入框为 `text` 类型（无切换按钮）

共同行为：

- **触发条件**：任何 API 调用返回 HTTP 401，或页面加载时通过 `checkAuth()` 主动探测
- **输入**：输入 API Key 后保存到 `localStorage`（键名 `tmd_api_key`），同时调用 API 持久化到服务端配置
- **确认**：保存后刷新页面使新 key 在所有后续请求中生效

### 系统设置 → 安全标签页

在系统设置页面新增了 **🔐 安全** 标签页（两个主题中 tab 顺序为最后），提供：

| 功能 | 说明 |
|------|------|
| **API Key 输入框** | web1 为 password 类型（👁️ 切换可见），web2 为 text 类型 |
| **保存到服务端** | 调用 API 将 key 写入 `conf.yaml`，同时写入 `localStorage`。失败时自动回滚 localStorage，保持一致性 |
| **测试连接** | 向 `/api/v1/tasks` 发送认证请求验证 key 有效性 |
| **清除** | 先调 API 清空服务端配置，成功后才删除 `localStorage` 中的 key。失败时保留 localStorage 中的 key 不影响当前会话 |

### 前端存储机制

- **存储位置**：`localStorage`（键名：`tmd_api_key`），同时持久化到服务端 `conf.yaml`
- **生命周期**：localStorage 为浏览器持久化，关闭浏览器不清除；服务端配置为永久存储
- **自动携带**：所有 API 请求通过 `api.request()` 自动从 `localStorage` 读取并注入 `Authorization` 头；SSE 连接自动追加 `?token=` 参数
- **清除方式**：系统设置 → 安全 → 清除按钮，或手动从 localStorage 和/或服务端配置中移除

---

## 配置持久化

### 方式一：直接编辑 conf.yaml

```yaml
api_key: "my-secret-key-123"
```

修改后需重启 Server 生效（无文件监听）。

### 方式二：Web UI 配置编辑

1. 进入系统设置 → 配置编辑
2. **简易模式**：在 `api_key` 字段中输入值
3. **原始 YAML 模式**：直接写入 `api_key: "xxx"`
4. 保存后，`api_key` 字段通过 `saveConfigFields` 同时更新内存和文件，**立即生效**（无需重启）。其他配置字段（如 `root_path`）仍需要重启

### 方式三：环境变量

参见[环境变量覆盖](#环境变量覆盖)章节。

### 生效时机

API Key 的生效时机取决于修改方式：

| 修改方式 | 生效时机 | 说明 |
|---------|---------|------|
| Web UI 配置编辑 → 简易/YAML 模式 | **立即生效** | `saveConfigFields` 同时更新内存 `*s.config = *newConf` 和写入 `conf.yaml`；authMiddleware 通过 `s.configMu.RLock()` 运行时读取 |
| Web UI 安全标签页 → 保存到服务端 | **立即生效** | 同上路径，先调 API 写入服务端配置，成功后更新 localStorage |
| 直接编辑 `conf.yaml` | 需重启 | 程序不监听文件变化 |
| 环境变量 `TMD_API_KEY` | 需重启 | 只在启动时读取 |

---

## 环境变量覆盖

### TMD_API_KEY

支持通过环境变量覆盖 `conf.yaml` 中的 `api_key` 值：

```bash
# Linux/Mac
export TMD_API_KEY="env-key-456"
./tmd -server

# Windows (CMD)
set TMD_API_KEY=env-key-456
tmd.exe -server

# Windows (PowerShell)
$env:TMD_API_KEY="env-key-456"
.\tmd.exe -server
```

### 优先级

环境变量 > `conf.yaml` 文件。同时设置时，环境变量生效。

### 使用场景

- Docker 容器部署时通过 `-e TMD_API_KEY=xxx` 注入
- CI/CD 流水线中通过环境变量管理密钥
- 不想将 Key 写入配置文件的情况

---

## 安全注意事项

### 1. API Key 强度

- **最小长度建议**：16 个字符以上
- **推荐格式**：使用随机字符串（字母+数字+特殊字符）
- **生成示例**：
  ```bash
  # Linux/Mac
  openssl rand -base64 32
  
  # Windows (PowerShell)
  [Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
  ```

### 2. 传输安全

- **强烈建议使用 HTTPS**：HTTP 明文传输时，API Key 可被网络中间人截获
- **反向代理方案**：使用 Nginx/Caddy 等反向代理终止 TLS，将请求转发到 TMD
- **同机使用**：仅在本机 `localhost` 使用时，HTTP 风险可控

### 3. Key 轮换

- 定期更换 API Key
- 更换后更新所有客户端中的 Key
- Server 重启后新 Key 生效

### 4. 日志安全

API Key **不会**出现在 TMD 的请求日志中。`loggingMiddleware` 只记录请求方法、路径、状态码和客户端地址，不记录请求头内容。

### 5. 不适用于 CLI 模式

认证层仅影响 HTTP API Server 模式（`-server` 参数）。CLI 模式不走 HTTP，不受影响。

### 6. 已知限制

- **HTTP Basic Auth 不支持**：仅支持 `Bearer` 方案
- **多用户/角色/权限**：当前版本不实现，所有认证用户拥有相同的完全访问权限
- **Token 过期/刷新**：不实现，Key 为静态字符串
- **速率限制**：认证层不包含速率限制功能，如需限制可在外层 Nginx 配置

---

## 实现参考

### 核心文件

| 文件 | 说明 |
|------|------|
| `internal/api/middleware.go` | `authMiddleware()` 实现、`isPublicPath()`、`extractBearerToken()` |
| `internal/api/server.go` | `buildHandler()` 中注入 authMiddleware |
| `internal/config/config.go` | `Config.APIKey` 字段、`GetFieldDefs()`、`NormalizeLoadedConf()` |
| `internal/api/config_handlers.go` | `buildConfigFieldMeta()` 中 api_key 的 UI 映射 |
| `internal/api/web/web1/app.js` | web1 前端认证集成：api.request() 自动注入 Authorization、SSE token 参数、认证对话框、安全标签页 |
| `internal/api/web/web1/index.html` | web1 认证对话框 HTML 结构 |
| `internal/api/web/web2/app.js` | web2 前端认证集成：api._fetch() 自动注入 Authorization、SSE token 参数、认证对话框（`openModal()`）、安全标签页 |
| `internal/api/web/web2/index.html` | web2 SPA 入口（认证对话框由 `openModal()` 动态生成，无硬编码 HTML） |

### 单元测试

| 测试函数 | 覆盖内容 |
|---------|---------|
| `TestIsPublicPath` | 22 个路径的公开/非公开判断 |
| `TestExtractBearerToken` | 7 种 Authorization 头格式提取 |
| `TestAuthMiddleware_NoKey_PassesThrough` | api_key 为空时放行 |
| `TestAuthMiddleware_CorrectBearerToken_Succeeds` | 正确 Bearer → 200 |
| `TestAuthMiddleware_WrongBearerToken_Returns401` | 错误 token → 401 + JSON + header |
| `TestAuthMiddleware_SSEQueryParam_Succeeds` | `?token=` → 200 |
| `TestAuthMiddleware_PublicPaths_BypassAuth` | 全部公开路径绕过认证 |
| `TestAuthMiddleware_ProtectedPaths_RequireAuth` | 全部 API 路径要求认证 |
| `TestAuthMiddleware_BuildHandlerIntegration` | 完整中间件链集成 |
| `TestAuthMiddleware_OPTIONS_PreflightPasses` | OPTIONS 预检绕过 auth |

测试位置：`internal/api/middleware_test.go`

---

## 变更历史

| 版本 | 日期 | 变更内容 |
|------|------|---------|
| 1.0 | 2026-06 | 初始版本：API Key Bearer Token 认证 |
