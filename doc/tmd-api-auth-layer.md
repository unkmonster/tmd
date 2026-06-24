# TMD API 认证/授权层

> 本文档详细说明 TMD (Twitter Media Downloader) 中 API 认证/授权层的设计原理、配置方法、认证流程和前端集成。认证基于 **JWT Session Token**（会话令牌），通过 API Key 从 `POST /api/v1/auth/login` 获取。

---

## 目录

1. [功能概述](#功能概述)
2. [架构概览](#架构概览)
3. [配置说明](#配置说明)
4. [架构与中间件链](#架构与中间件链)
5. [认证方式：JWT Session Token](#认证方式jwt-session-token)
6. [认证流程详解](#认证流程详解)
8. [公开路径白名单](#公开路径白名单)
9. [SSE 端点的特殊处理](#sse-端点的特殊处理)
10. [API 使用示例](#api-使用示例)
11. [前端认证流程](#前端认证流程)
12. [前端 JWT 集成详解](#前端-jwt-集成详解)
13. [配置持久化与生效机制](#配置持久化与生效机制)
14. [环境变量覆盖](#环境变量覆盖)
15. [安全注意事项](#安全注意事项)
16. [实现参考](#实现参考)
17. [变更历史](#变更历史)

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
| **JWT 纯令牌** | 所有 API 请求强制使用 JWT 会话令牌（通过 `POST /api/v1/auth/login` 获取） |
| **密钥派生** | JWT 签名密钥从 API Key 派生（HMAC-SHA256），泄露 JWT secret 不影响原始 Key |

---

## 架构概览

```
┌─────────────────────────────────────────────────────┐
│                    客户端                              │
│  ┌────────────────────────────────────┐              │
│  │  Web UI (web1 / web2 双主题)        │             │
│  │  ┌──────────────────────────────┐  │              │
│  │  │ localStorage:                │  │              │
│  │  │  tmd_jwt_token   ← JWT 优先  │  │              │
│  │  │  tmd_jwt_expiry  ← 过期时间  │  │              │
│  │  │  tmd_api_key     ← 回退方案  │  │              │
│  │  └──────────────────────────────┘  │              │
│  │  401 自动 refresh → 成功重试       │              │
│  │  Proactive refresh 每 45min       │              │
│  └────────────────────────────────────┘              │
│  ┌────────────────────────────────────┐              │
│  │  curl / 第三方客户端                │              │
│  │  Authorization: Bearer <jwt|key>   │              │
│  └────────────────────────────────────┘              │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│                    TMD Server                         │
│  ┌─ securityHeadersMiddleware ─────────────────────┐ │
│  │  X-Content-Type-Options, X-Frame-Options 等     │ │
│  ├─ loggingMiddleware ─────────────────────────────┤ │
│  │  记录 [METHOD] /path ip STATUS (duration)       │ │
│  ├─ API-Version header ───────────────────────────┤ │
│  │  API-Version: v1                                │ │
│  ├─ CORS middleware ──────────────────────────────┤ │
│  │  OPTIONS → 204, 不进入内层                      │ │
│  ├─ authMiddleware ───────────────────────────────┤ │
│  │  ① isPublicPath? → 放行                        │ │
│  │  ② apiKey == ""? → 放行 （认证未配置）           │ │
│  │  ③ 提取 token (Authorization > ?token=)        │ │
│  │  ④ JWT 验证 validateSessionToken()              │ │
│  │  ⑤ 通过 → 放行；失败 → 401 + X-Token-Type      │ │
│  ├─ ServeMux ────────────────────────────────────┤ │
│  │  正常路由分发到 handler                          │ │
│  └────────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────────┐ │
│  │  /api/v1/auth/login    → 公开, 签发 JWT         │ │
│  │  /api/v1/auth/refresh  → 公开, 刷新 JWT         │ │
│  │  /api/v1/auth/check    → 公开, 查询 JWT 状态    │ │
│  │  /api/v1/sse/tasks     → 受保护, ?token= 回退   │ │
│  │  /api/v1/logs/stream   → 受保护, ?token= 回退   │ │
│  └────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

---

## 配置说明

### conf.yaml

在 `conf.yaml` 中添加 `api_key` 字段：

```yaml
root_path: "D:/TwitterMedia"
cookie:
  auth_token: "..."
  ct0: "..."
api_key: "your-secret-api-key"    # ← API Bearer Token / JWT 签发密钥
max_download_routine: 20
proxy_url: "http://127.0.0.1:7897"
```

`api_key` 为空字符串（或不设置）时，认证层不生效，所有请求照常放行。

### 配置字段元信息

| 字段 | 类型 | 环境变量 | 默认值 | 分组 | Web UI 类型 | 脱敏 |
|------|------|---------|-------|------|------------|------|
| `api_key` | `string` | `TMD_API_KEY` | `""`（空 = 禁用） | `security` | password | 是 (`abc•••xyz`) |

### 配置加载优先级

1. `conf.yaml` 文件中的值
2. 环境变量 `TMD_API_KEY`（优先级更高，启动时覆盖）
3. Web UI 配置编辑保存（运行时覆盖，**立即生效**）

### Web UI 配置编辑

`api_key` 在系统设置的配置编辑器中以 **password** 类型显示（值被脱敏，如 `abc•••xyz`），归属于 `security` 分组。通过 Web UI 保存后**立即生效**，无需重启。

**特殊 sentinel 值：**

| 值 | 行为 | 适用场景 |
|------|------|---------|
| `__KEEP_OLD__` | 保留当前值不变 | 配置表单默认行为 |
| `__CLEAR__` | 显式清空该字段 | 清除 `api_key` 关闭认证 |

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
│ authMiddleware      │  ← 纯 JWT 认证（通过 /api/v1/auth/login 获取令牌）
├─────────────────────┤
│ ServeMux (路由器)    │  ← 最内层：分发到具体 handler
└─────────────────────┘
```

**关键设计**：
- OPTIONS 预检请求由 CORS 中间件直接处理（返回 204），**不会经过 authMiddleware**，因此无需在预检中携带 token
- 认证失败的请求仍然被 `loggingMiddleware` 记录，便于排查问题
- `authMiddleware` 在 `ServeMux` 之前，对所有路由统一检查，无需每个 handler 单独处理
- CORS `AllowedHeaders` 包含 `Authorization`，确保跨域请求可以携带 Bearer token

### 认证中间件代码位置

- **实现**：`internal/api/middleware.go` — `Server.authMiddleware()` 方法
- **注入**：`internal/api/server.go` — `buildHandler()` 函数

---


## 认证方式：JWT Session Token

### 核心原理

JWT（JSON Web Token）是一种开放标准（RFC 7519），用于在客户端和服务端之间安全传输声明。TMD 使用 HS256（HMAC-SHA256）签名的 JWT 作为短期会话令牌。

```
┌─────────────┐         ┌──────────────────────────┐         ┌──────────────┐
│  客户端      │         │      TMD Server           │         │  存储        │
├─────────────┤         ├──────────────────────────┤         ├──────────────┤
│ API Key     │──登录──►│ ① 验证 API Key           │         │ conf.yaml    │
│             │         │ ② deriveJWTSecret(key)    │         │ 中的 api_key │
│             │         │ ③ 签发 HS256 JWT (1h)    │         │              │
│             │◄──JWT──│ ④ 返回 {token, expires}   │         │              │
├─────────────┤         ├──────────────────────────┤         └──────────────┘
│ JWT         │──请求──►│ authMiddleware:           │
│ (后续请求)   │         │ ① 提取 token              │
│             │         │ ② validateSessionToken()  │
│             │         │ ③ 签名验证通过 → 放行      │
│             │◄──200──│ ④ 失败 → 回退原始 Key 比较 │
└─────────────┘         └──────────────────────────┘
```

### 密钥派生（Key Derivation）

JWT 的 HMAC-SHA256 签名密钥**不直接使用 `api_key`**，而是通过 HMAC 派生：

```go
func deriveJWTSecret(apiKey string) []byte {
    mac := hmac.New(sha256.New, []byte("tmd-jwt-v1"))
    mac.Write([]byte(apiKey))
    return mac.Sum(nil)
}
```

**设计理由**：
- 即使 `jwt_secret` 泄露（如通过进程内存 dump），原始 `api_key` 不受影响
- 只需清除所有 JWT，无需修改 `conf.yaml`
- **HKDF 风格**：`HMAC(context, key_material)` 是标准密钥派生模式

### JWT 结构

```json
// Header
{
  "alg": "HS256",
  "typ": "JWT"
}

// Payload (claims)
{
  "iss": "tmd",                    // Issuer
  "sub": "tmd-session",            // Subject
  "iat": 1719212345,               // Issued At (Unix timestamp)
  "exp": 1719215945,               // Expiration (iat + 3600s)
  "jti": "1719212345012345678"     // JWT ID (基于 UnixNano，唯一性)
}
```

**claims 验证规则（`validateSessionToken`）：**

| Claim | 验证 | 容忍度 |
|-------|------|--------|
| `alg` | 必须是 `HS256` | 通过 `jwt.WithValidMethods` 强制执行 |
| `iss` | 必须是 `"tmd"` | 通过 `jwt.WithIssuer` |
| `sub` | 必须是 `"tmd-session"` | 通过 `jwt.WithSubject` |
| `exp` | 不能超过当前时间 | 30 秒时钟偏差容忍（`jwt.WithLeeway`） |
| 签名 | 用 `deriveJWTSecret(apiKey)` 验证 | 密钥错误 → 签名无效 |

### 登录端点（public）

`POST /api/v1/auth/login` — 用 API Key 换取 JWT。

**请求：**
```http
POST /api/v1/auth/login
Authorization: Bearer <api_key>
```

**成功响应（200）：**
```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2026-06-25T12:00:00Z",
    "expires_in": 3600
  }
}
```

**失败响应（401）：**
```json
{
  "success": false,
  "error": "unauthorized"
}
```

**处理逻辑（伪代码）：**
```
1. 从 Authorization 头或 ?token= 提取 token
2. 速率检查（同一 IP 最多 5 次/分钟）
3. 读取 s.config.APIKey
4. if apiKey == "" → 401（认证未配置）
5. if token != apiKey → 记录失败 + 401
6. jwtToken = generateSessionToken(apiKey)
7. 返回 { token, expires_at, expires_in }
```

### 刷新端点

`POST /api/v1/auth/refresh` — 用有效（或刚过期）的 JWT 换取新 JWT。

**请求：**
```http
POST /api/v1/auth/refresh
Authorization: Bearer <jwt_token>
```

**特殊处理：**
此端点是公开路径（`isPublicPath`），authMiddleware 不会拦截。handler 自行验证 JWT：

```
1. 提取 token
2. validateSessionToken(token, apiKey)
3. if 错误不是"过期"→ 401（签名无效/格式错误）
4. if 错误是"过期"→ 仍然接受（客户端正在刷新）
5. if token == nil → 401
6. jwtToken = generateSessionToken(apiKey)
7. 返回 { token, expires_at, expires_in }
```

**关键设计**：JWT 过期后 authMiddleware 会返回 401，导致客户端无法访问受保护端点。为解决这个「鸡生蛋」问题，`/api/v1/auth/refresh` 被列为**公开路径**，handler 自己处理签名验证和过期判断。

### 检查端点

`GET /api/v1/auth/check` — 查询当前 JWT 状态。

```http
GET /api/v1/auth/check
Authorization: Bearer <jwt_token>
```

**成功响应：**
```json
{
  "success": true,
  "data": {
    "authenticated": true,
    "valid": true,
    "expires_at": "2026-06-25T12:00:00Z",
    "expires_in": 3550,
    "needs_refresh": false
  }
}
```

**`needs_refresh` 字段**：当剩余时间小于 `jwtRefreshMargin`（5 分钟）时为 `true`，提示前端主动刷新。

**无 token 响应：**
```json
{
  "success": true,
  "data": {
    "authenticated": false
  }
}
```

**过期 token 响应：**
```json
{
  "success": true,
  "data": {
    "authenticated": false,
    "valid": false,
    "expired": true,
    "error": "token has invalid claims: token is expired"
  }
}
```

---

## 双模式认证详解

`authMiddleware` 支持两种认证方式同时生效，按顺序尝试：

```
authMiddleware(request):
  │
  ├─ isPublicPath(path) → 跳过所有认证
  │
  ├─ apiKey == "" → 跳过所有认证（认证未配置）
  │
  ├─ 提取 token:
  │   ├─ Authorization: Bearer <xxx> → 优先
  │   └─ ?token=xxx → 回退（SSE 兼容）
  │
  ├─ token == "" → 401 (X-Token-Type: missing)
  │
  └─ JWT 验证:
      ├─ validateSessionToken(token, apiKey)
      ├─ 通过 → 放行 ✅
      └─ 失败 → 401 (X-Token-Type: expired|invalid)
```

### X-Token-Type 响应头

| 值 | 触发条件 | 客户端行为 |
|---------|---------|-----------|
| `missing` | 未携带任何 token | 弹出 API Key 输入对话框 |
| `expired` | JWT 已过期但签名有效 | 尝试调用 `/api/v1/auth/refresh` |
| `invalid` | JWT 签名无效、格式错误、或原始 Key 不匹配 | 清除本地凭据，弹出认证对话框 |

### 前端 401 处理策略

```
客户端收到 401
  │
  ├─ 调用 /api/v1/auth/refresh（尝试用当前 JWT 刷新）
  │   ├─ 成功 → 用新 JWT 重试原请求 ✅
  │   └─ 失败 → 显示认证对话框，让用户输入 API Key
  │
  └─ 用户在对话框中输入 API Key
      └─ 调用 /api/v1/auth/login
          ├─ 成功 → 存储 JWT，刷新页面 ✅
          └─ 失败 → 显示错误
```

### 场景对照表

| 场景 | 请求携带 | 中间件行为 | 结果 |
|------|---------|-----------|------|
| 全新客户端 | 无 token | 401 `missing` | 客户端弹认证框 |
| 有有效 JWT | `Bearer <jwt>` | JWT 验证通过 → 放行 ✅ |
| JWT 过期 | `Bearer <expired_jwt>` | JWT 验证失败（过期）→ 401 `expired` | 客户端调用 refresh |
| JWT 签名错误 | `Bearer <tampered_jwt>` | JWT 验证失败（签名）→ 401 `invalid` | 客户端弹认证框 |
| API Key 变更后旧 JWT | `Bearer <old_jwt>` | JWT 验证失败（签名，密钥已变）→ 401 `invalid` | 客户端弹认证框 |
| 认证未配置 | 任何 | `apiKey == ""` → 放行 | ✅ 无认证 |

---

## 公开路径白名单

以下路径**不需要认证**，直接放行。这是为了保证 Web UI 可以正常加载，以及 JWT 登录/刷新流程可用。

| 路径 | 原因 |
|------|------|
| `GET /` | Web UI 首页（SPA 入口） |
| `GET /favicon.ico` | 浏览器图标请求 |
| `GET /tasks` | SPA 页面路由 |
| `GET /data` | SPA 页面路由 |
| `GET /schedules` | SPA 页面路由 |
| `GET /system` | SPA 页面路由 |
| `GET /logs` | SPA 页面路由 |
| `GET /api/v1/health` | 健康检查（Docker healthcheck、负载均衡探测） |
| `POST /api/v1/auth/login` | JWT 登录（需要在没有 JWT 时调用） |
| `POST /api/v1/auth/refresh` | JWT 刷新（需要在 JWT 过期时调用） |
| `GET /api/v1/auth/check` | JWT 状态查询（检查端点不需要认证） |
| `GET /api/v1/config/theme` | 获取当前主题（主题切换器内联 JS 调用） |
| `POST /api/v1/config/theme` | 切换主题（主题切换器内联 JS 调用） |
| `GET /api/v1/config/themes` | 列出可用主题（主题切换器内联 JS 调用） |
| `GET /static/*` | 静态文件（JS、CSS、图片等） |

> ⚠️ **安全设计**：`/api/v1/auth/login` 虽然是公开的，但有**内存速率限制**（同一 IP 最多 5 次失败尝试/分钟），防止暴力破解。`/api/v1/auth/refresh` 公开但要求 JWT 签名有效（handler 自行验证）。

### 为什么 Web UI 页面需要免认证？

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
// ✅ 使用查询参数（JWT 优先，回退到 API Key）
const token = localStorage.getItem('tmd_jwt_token') || localStorage.getItem('tmd_api_key');
const es = new EventSource('/api/v1/sse/tasks?token=' + encodeURIComponent(token));
```

### 受影响的 SSE 端点

| 端点 | 用途 | 认证方式 |
|------|------|---------|
| `GET /api/v1/sse/tasks` | 任务状态和调度状态实时推送 | `Authorization` 头或 `?token=` |
| `GET /api/v1/logs/stream` | 日志实时流 | `Authorization` 头或 `?token=` |

这两个端点在 `authMiddleware` 中检查 `?token=` 参数，**不在公开路径白名单中**。

### SSE 认证流程

```
EventSource 连接
  │
  ├─ 普通 API 请求: Authorization: Bearer <jwt>
  │   └─ 中间件从 Header 提取 token → 验证 → 放行
  │
  └─ EventSource (SSE): URL?token=<jwt>
      └─ 中间件从 Query 提取 token → 验证 → 放行
```

### SSE 断线重连的 JWT 处理

当 JWT 过期导致 SSE 连接断开时：

1. EventSource 触发 `onerror`
2. 客户端 Exponential backoff 重连
3. 重连时使用当前 `localStorage` 中的 token
4. 如果 token 已过期 → 服务端返回 401 → SSE 连接失败
5. 前端 `unhandledrejection` 捕获 401 → 尝试 `/api/v1/auth/refresh`
6. 成功 → 新 JWT 存入 localStorage
7. SSE 下一次重连使用新 JWT → 连接成功

---

## API 使用示例

### 前提

假设 `conf.yaml` 中配置了 `api_key: "my-secret-key-123"`，Server 运行在 `http://localhost:25556`。

### 1. JWT 完整登录流程

```bash
# Step 1: 用 API Key 换取 JWT
JWT=$(curl -s -X POST http://localhost:25556/api/v1/auth/login \
  -H "Authorization: Bearer my-secret-key-123" | jq -r '.data.token')

echo "JWT: $JWT"

# Step 2: 用 JWT 访问 API
curl -H "Authorization: Bearer $JWT" \
  http://localhost:25556/api/v1/tasks

# Step 3: JWT 过期前刷新
NEW_JWT=$(curl -s -X POST http://localhost:25556/api/v1/auth/refresh \
  -H "Authorization: Bearer $JWT" | jq -r '.data.token')

# Step 4: 检查 JWT 状态
curl http://localhost:25556/api/v1/auth/check \
  -H "Authorization: Bearer $NEW_JWT"
```


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
X-Token-Type: missing
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
# 用 API Key
curl -N "http://localhost:25556/api/v1/sse/tasks?token=my-secret-key-123"

# 用 JWT（先获取 JWT）
JWT=$(curl -s -X POST -H "Authorization: Bearer my-secret-key-123" \
  http://localhost:25556/api/v1/auth/login | jq -r '.data.token')
curl -N "http://localhost:25556/api/v1/sse/tasks?token=$JWT"
```

### 7. 日志流（带 token）

```bash
curl -N "http://localhost:25556/api/v1/logs/stream?token=my-secret-key-123&level=info"
```

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

### 认证方式

请求仅接受 **JWT 会话令牌**（`localStorage.tmd_jwt_token`），通过 `POST /api/v1/auth/login` 用 API Key 换取。Web UI 中不再存储或传输原始 API Key。

### 首次加载（无认证凭据）

```
浏览器打开 http://localhost:25556/
  │
  ├─ 页面 HTML/JS/CSS 加载 ✓（公开路径）
  │
  ├─ SSE 连接: EventSource('/api/v1/sse/tasks') → 401（无 token）
  │
  ├─ api.getTasks() → fetch('/api/v1/tasks') → 401
  │   └─ Error: { status: 401, message: 'unauthorized' }
  │
  ├─ unhandledrejection 捕获 401
  │   └─ localStorage 中既无 JWT 也无 API Key
  │       └─ showAuthDialog() → 弹出认证对话框
  │           ├─ 用户输入 API Key
  │           └─ 调用 POST /api/v1/auth/login
  │               ├─ 成功 → 存储 JWT + API Key
  │               └─ 失败 → 显示错误
  │
  └─ 页面刷新
      ├─ 有 JWT:
      │   ├─ SSE: /api/v1/sse/tasks?token=<jwt> → 成功 ✓
      │   └─ API: Authorization: Bearer <jwt> → 200 ✓
      └─ 页面正常渲染
```

### JWT 过期自动恢复

```
进行中的请求 → 服务端返回 401
  │
  ├─ 当前有 JWT？→ 是
  │   ├─ 调用 POST /api/v1/auth/refresh
  │   │   ├─ 成功（换新 JWT）
  │   │   │   ├─ 存储新 JWT + 新过期时间
  │   │   │   └─ 用新 JWT 重试原请求 ✅
  │   │   └─ 失败（签名无效等）
  │   │       └─ 弹出认证对话框
  │   └─ 否
  │       └─ 弹出认证对话框
  └─ 用户操作
```

### 定期主动刷新

```
页面加载
  ├─ 检查 tmd_jwt_expiry
  │   └─ 剩余 < 10 分钟？
  │       └─ 是 → 静默调用 /api/v1/auth/refresh
  │
  └─ 设置 setInterval(45 分钟)
      └─ 每次触发：
          ├─ 有 JWT → 调用 /api/v1/auth/refresh ✅
          └─ 无 JWT → 跳过
```

### API Key 变更时的 JWT 失效

```
用户在配置编辑页修改 api_key
  │
  ├─ 调用 PUT /api/v1/config/fields
  │   └─ 成功
  │       ├─ 同步新 api_key 到 localStorage
  │       └─ 清除 tmd_jwt_token + tmd_jwt_expiry
  │           （旧 JWT 用旧密钥签发，已全部失效）
  │
  └─ 用户下次请求时：
      ├─ 无 JWT → 使用原始 API Key 认证 ✅（向后兼容）
      └─ 需要新 JWT → 通过 auth dialog 或 Security tab 重新 login
```

### Security 标签页（web2 特有）

系统设置 → **Security** 标签页提供完整的 JWT 管理：

| 功能 | 说明 |
|------|------|
| **JWT 状态显示** | 显示当前 JWT 有效状态和剩余时间（绿色/红色指示） |
| **Login & Save** | 输入 API Key → 调用 login 端点 → 存储 JWT + API Key |
| **Test Connection** | 直接用 API Key 测试服务端连接（绕过 `API._fetch()` 的 401 处理） |
| **Clear** | 清除所有本地凭据（`tmd_jwt_token` + `tmd_jwt_expiry` + `tmd_api_key`） |
| **Refresh Session** | 手动刷新 JWT（调用 refresh 端点），仅当有有效 JWT 时显示 |

---

## 前端 JWT 集成详解

### web1 主题实现（经典主题）

**文件**：`internal/api/web/web1/app.js`

**API 客户端**（`api.request`）：

```javascript
// 注入 Authorization 头（优先用 JWT，回退到 API Key）
if (!extra.skipAuthInject) {
  const jwt = localStorage.getItem('tmd_jwt_token');
  const apiKey = localStorage.getItem('tmd_api_key');
  if (jwt) {
    options.headers['Authorization'] = 'Bearer ' + jwt;
  } else if (apiKey) {
    options.headers['Authorization'] = 'Bearer ' + apiKey;
  }
}
```

**401 自动处理**：收到 401 后有 JWT 时自动 refresh → 成功重试 → 失败弹窗：

```javascript
if (res.status === 401) {
  const haveJWT = !!localStorage.getItem('tmd_jwt_token');
  if (haveJWT) {
    const refreshed = await this._tryRefreshJWT();
    if (refreshed) {
      // 重新调用自身，从 localStorage 读取新 JWT
      return this.request(method, path, body, extra);
    }
  }
  // 抛出 401 供外层处理
  const authErr = new Error('unauthorized');
  authErr.status = 401;
  authErr._isUnauthorized = true;
  throw authErr;
}
```

**JWT 刷新方法**：

```javascript
async _tryRefreshJWT() {
  const oldJWT = localStorage.getItem('tmd_jwt_token');
  if (!oldJWT) return false;
  const res = await fetch('/api/v1/auth/refresh', {
    method: 'POST',
    headers: { 'Authorization': 'Bearer ' + oldJWT }
  });
  if (!res.ok) return false;
  const json = await res.json();
  if (!json.success || !json.data || !json.data.token) return false;
  localStorage.setItem('tmd_jwt_token', json.data.token);
  if (json.data.expires_at) localStorage.setItem('tmd_jwt_expiry', json.data.expires_at);
  return true;
}
```

**SSE 连接**：

```javascript
_tokenParam() {
  const jwt = localStorage.getItem('tmd_jwt_token');
  if (jwt) return '?token=' + encodeURIComponent(jwt);
  const key = localStorage.getItem('tmd_api_key');
  return key ? '?token=' + encodeURIComponent(key) : '';
}

connect() {
  this.conn = new EventSource('/api/v1/sse/tasks' + this._tokenParam());
  // ...
}
```

**初始化主动刷新**：

```javascript
async function init() {
  // 剩余 < 10min 时静默刷新
  const jwtExpiry = localStorage.getItem('tmd_jwt_expiry');
  if (jwtExpiry) {
    const remaining = new Date(jwtExpiry) - new Date();
    if (remaining > 0 && remaining < 10 * 60 * 1000) {
      api._tryRefreshJWT();
    }
  }
  // 每 45 分钟定时刷新
  setInterval(() => {
    const jwt = localStorage.getItem('tmd_jwt_token');
    if (jwt) api._tryRefreshJWT();
  }, 45 * 60 * 1000);
}
```

**认证对话框提交**：

```javascript
async function submitAuthKey() {
  const key = input.value.trim();
  // 调用 login 端点获取 JWT（不再调用 saveConfigFields）
  const res = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Authorization': 'Bearer ' + key }
  });
  const json = await res.json();
  // 存储 JWT + API Key（API Key 作为 SSE 等场景的 fallback）
  localStorage.setItem('tmd_jwt_token', json.data.token);
  localStorage.setItem('tmd_jwt_expiry', json.data.expires_at);
  localStorage.setItem('tmd_api_key', key);
  window.location.reload();
}
```

**配置保存时的 JWT 清理**：

```javascript
if (fields['api_key'] && fields['api_key'] !== '__KEEP_OLD__') {
  localStorage.setItem('tmd_api_key', fields['api_key']);
  // API Key 变更 → 旧 JWT 已失效 → 清除
  localStorage.removeItem('tmd_jwt_token');
  localStorage.removeItem('tmd_jwt_expiry');
}
```

### web2 主题实现（精简主题）

**文件**：`internal/api/web/web2/app.js`

**核心区别**：

| 特性 | web1 | web2 |
|------|------|------|
| API 客户端 | `api.request()` 对象，统一 JSON 解析 | `API._fetch()` 纯函数，各自处理 JSON |
| 401 重试 | 重新调用 `this.request()` 递归 | 直接在新请求中注入新 token |
| SSE token | `sseManager._tokenParam()` 方法 | `sseApiKey()` 纯函数 |
| 认证弹窗 | 硬编码 HTML（`index.html`） | 动态 `openModal()` 生成 |
| 弹窗提交 | `async/await` + `try/catch` | `.then().catch()` Promise 链 |
| Security 标签 | 已合并至配置编辑页 | 独立 `renderSecurityEditor()` |
| login 调用 | `fetch('/api/v1/auth/login')` | `fetch(apiBase() + '/api/v1/auth/login')` |
| 主动 auth 检查 | 无（依赖 401 触发） | `checkAuth()` 函数主动探测 |

**Security 标签页**（web2 特有）：

```javascript
function renderSecurityEditor(content) {
  // 显示 JWT 状态（有效/过期/未使用）
  const jwt = localStorage.getItem('tmd_jwt_token');
  const jwtExpiry = localStorage.getItem('tmd_jwt_expiry');
  let jwtStatus = '';
  if (jwt && jwtExpiry) {
    const remaining = new Date(jwtExpiry) - new Date();
    if (remaining > 0) {
      jwtStatus = `✅ JWT active (expires in ~${Math.round(remaining / 60000)} min)`;
    } else {
      jwtStatus = `❌ JWT expired — re-login required`;
    }
  }
  // 渲染输入框 + 按钮
  content.innerHTML = `...Login & Save...Test Connection...Refresh Session...Clear...`;
}
```

**JWT 手动刷新按钮**（web2 特有）：

```javascript
function refreshSecJWT() {
  API._tryRefreshJWT().then(ok => {
    if (ok) {
      const expiry = localStorage.getItem('tmd_jwt_expiry');
      const remaining = expiry ? Math.round((new Date(expiry) - new Date()) / 60000) : '?';
      updateSecStatus(`✅ Session refreshed (expires in ~${remaining} min)`);
    } else {
      updateSecStatus('❌ Session refresh failed, please re-login');
    }
  });
}
```

### 两主题共同行为

| 行为 | 实现方式 | 触发时机 |
|------|---------|---------|
| JWT 优先 | `localStorage.getItem('tmd_jwt_token')` 优先于 `tmd_api_key` | 每次 API 请求 |
| 401 自动 refresh | 调用 `_tryRefreshJWT()` 后重试 | 收到 HTTP 401 |
| Proactive refresh | `setInterval(45 * 60 * 1000)` | 页面加载后持续 |
| 初始提前刷新 | 检查 `tmd_jwt_expiry` 剩余 < 10min | 页面加载时 |
| API Key 变更清除 JWT | `localStorage.removeItem('tmd_jwt_token')` | 配置保存成功后 |
| 全局 401 兜底 | `window.addEventListener('unhandledrejection')` | 未捕获的 Promise reject |

---

## 配置持久化与生效机制

### API Key 的存储与生效

| 修改方式 | 生效时机 | 说明 |
|---------|---------|------|
| Web UI 配置编辑 → 简易模式 | **立即生效** | 调用 `PUT /api/v1/config/fields`，同时更新内存 `*s.config` 和写入 `conf.yaml` |
| Web UI 配置编辑 → YAML 模式 | **立即生效** | 调用 `PUT /api/v1/config/raw`，同上路径 |
| 直接编辑 `conf.yaml` | 需重启 | 程序不监听文件变化 |
| 环境变量 `TMD_API_KEY` | 需重启 | 只在启动时读取 |

### JWT 的生命周期

JWT 不持久化到任何服务端存储——它是**无状态**的：

```
API Key 配置 (conf.yaml)
  │
  ├─ POST /api/v1/auth/login
  │   └─ deriveJWTSecret(apiKey) → 签发 HS256 JWT
  │       └─ JWT 包含: { iss, sub, iat, exp, jti }
  │           └─ 签名: HMAC-SHA256(jwt_secret, header.payload)
  │
  ├─ authMiddleware 验证
  │   └─ deriveJWTSecret(apiKey) → 验证 JWT 签名
  │       └─ 签名匹配 → 验证通过（无数据库查询）
  │
  └─ API Key 变更
      └─ deriveJWTSecret(newKey) 产生不同的 jwt_secret
          └─ 所有旧 JWT 签名验证失败 → 自动失效
```

**关键结论**：
- JWT 验证**不依赖数据库**，纯计算验证
- JWT 无法单独撤销，但**变更 API Key 可以使所有 JWT 立即失效**
- JWT 过期时间 1 小时，clock skew 容忍 30 秒

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

- **强烈建议使用 HTTPS**：HTTP 明文传输时，API Key 和 JWT 都可被网络中间人截获
- **反向代理方案**：使用 Nginx/Caddy 等反向代理终止 TLS，将请求转发到 TMD
- **同机使用**：仅在本机 `localhost` 使用时，HTTP 风险可控

### 3. 会话令牌安全

JWT Session Token 相比直接传输 API Key 的优势：

| 方面 | 直接传输 API Key | JWT Session Token |
|------|-----------------|-------------------|
| 传输频率 | 每次请求完整 Key | 仅 login 时传输一次 Key |
| 泄露影响 | Key 永久有效，需修改 `conf.yaml` | 1 小时有效，过期自动失效 |
| 撤销方式 | 修改 `conf.yaml` | 等待过期或修改 API Key（密钥派生变，所有 JWT 自动失效） |

### 4. Key 轮换

- 定期更换 API Key
- 更换后更新所有客户端中的 Key
- JWT 在 Key 变更后全部自动失效（派生密钥已变）
- 更换后用户需重新 login

### 5. 日志安全

API Key 和 JWT **不会**出现在 TMD 的请求日志中。`loggingMiddleware` 只记录：

```
[GET] /api/v1/tasks 127.0.0.1 200 (2.3ms)
```

仅包含：HTTP 方法、请求路径、客户端 IP、状态码、处理时间。**不记录请求头内容**。

### 6. 登录速率限制

`POST /api/v1/auth/login` 端点内置**内存速率限制器**：

```go
type authRateLimiter struct {
    mu       sync.Mutex
    attempts map[string]*rateLimitEntry // key: RemoteAddr（含端口）
}

const (
    maxLoginAttempts = 5     // 最多 5 次失败
    loginWindow     = 1 * time.Minute // 每分钟重置
)
```

- 同一 IP 在 1 分钟内最多 5 次失败尝试
- 成功尝试不计入限制
- 限制器仅存在于内存中，服务重启后重置

### 7. 不适用于 CLI 模式

认证层仅影响 HTTP API Server 模式（`-server` 参数）。CLI 模式不走 HTTP，不受影响。

### 8. 已知限制

- **HTTP Basic Auth 不支持**：仅支持 `Bearer` 方案
- **多用户/角色/权限**：当前版本不实现，所有认证用户拥有相同的完全访问权限
- **JWT 撤销**：不实现主动撤销列表（CRL），通过 API Key 变更间接实现
- **刷新端点无速率限制**：`/api/v1/auth/refresh` 要求有效 JWT，降低了暴力风险

---

## 实现参考

### 核心文件

| 文件 | 说明 |
|------|------|
| `internal/api/middleware.go` | `authMiddleware()` 纯 JWT 认证实现、`isPublicPath()` 白名单、`extractBearerToken()` token 提取、`writeAuth401()` 401 响应 |
| `internal/api/auth_jwt.go` | JWT 核心：`deriveJWTSecret()` 密钥派生、`generateSessionToken()` 签发、`validateSessionToken()` 验证；三个 handler：`handleAuthLogin`/`handleAuthRefresh`/`handleAuthCheck`；`authRateLimiter` 登录速率限制 |
| `internal/api/server.go` | `Server.authRateLimit` 字段、`buildHandler()` 中路由注册和中间件注入 |
| `internal/config/config.go` | `Config.APIKey` 字段、`GetFieldDefs()` 注册、`NormalizeLoadedConf()` 配置加载 |
| `internal/api/config_handlers.go` | `buildConfigFieldMeta()` 中 api_key 的 UI 映射（security 分组、password 类型、脱敏）、`handleSaveConfigFields`/`handleUpdateConfigRaw` 中 api_key 变更检测和消息提示 |
| `internal/api/handlers.go` | `TMD_DEV=1` 开发模式本地 FS 主题加载 |
| `internal/api/web/web1/app.js` | web1 前端 JWT 集成：JWT 优先的 Authorization 注入、401 自动 refresh、`sseManager` JWT token、`submitAuthKey()` login 端点调用、proactive refresh 定时器、配置保存 JWT 清理 |
| `internal/api/web/web1/index.html` | web1 认证对话框 HTML 结构（`.auth-overlay` + `.auth-modal`） |
| `internal/api/web/web1/styles.css` | web1 认证对话框样式（122 行） |
| `internal/api/web/web2/app.js` | web2 前端 JWT 集成：`API._fetch()` JWT 优先、401 自动 refresh、`sseApiKey()` JWT 优先、`renderSecurityEditor()` JWT 状态显示、`saveSecKey()` login 调用、`refreshSecJWT()` 手动刷新 |
| `go.mod` / `go.sum` | `github.com/golang-jwt/jwt/v5 v5.3.1` 直接依赖 |

### 单元测试

测试文件：`internal/api/middleware_test.go`

**现有 API Key 认证测试（10 个）：**

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

**新增 JWT 测试（11 个）：**

| 测试函数 | 覆盖内容 |
|---------|---------|
| `TestGenerateAndValidateJWT` | JWT 签发/验证/错误 key/无效字符串 |
| `TestIsJWTFormat` | 5 种格式判断 |
| `TestAuthMiddleware_JWT_Succeeds` | JWT 通过 authMiddleware |
| `TestAuthMiddleware_JWT_SSEQueryParam_Succeeds` | JWT 通过 SSE `?token=` |
| `TestAuthMiddleware_JWT_WrongKey_Returns401` | 错误密钥签发的 JWT → 401 |
| `TestAuthMiddleware_DualMode_OldKeyStillWorks` | 原始 API Key 仍然可用 |
| `TestIsJWTExpiredError` | 过期错误类型检测 |
| `TestAuthLogin_Success` | login 端点成功流程 |
| `TestAuthLogin_WrongKey_Returns401` | login 错误 key → 401 |
| `TestAuthLogin_NoKey_Returns401` | login 无 key → 401 |
| `TestAuthRefresh_ValidJWT_Succeeds` | refresh 端点成功流程 |
| `TestAuthCheck_ValidJWT_ReturnsAuthenticated` | check 端点返回认证状态 |
| `TestAuthCheck_NoToken_ReturnsUnauthenticated` | check 端点无 token → 未认证 |
| `TestAuthMiddleware_BuildHandler_JWT_Integration` | 集成测试：JWT 认证/过期/双模式 |

---

## 变更历史

| 版本 | 日期 | 变更内容 |
|------|------|---------|
| 2.0 | 2026-06 | 文档全面重写：新增 JWT Session Token 章节、双模式认证详解、前端 JWT 集成对比、登录速率限制说明、SSE JWT 处理流程 |
| 1.0 | 2026-06 | 初始版本：API Key Bearer Token 认证 |
