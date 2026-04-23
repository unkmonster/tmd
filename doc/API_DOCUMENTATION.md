# TMD API 使用文档

## 概述

TMD (Twitter Media Downloader) 提供 HTTP REST API，允许通过编程方式控制下载任务。API Server 模式支持 Web/AI 调用。

## 启动 API Server

### 基本启动

```bash
# 使用默认端口 25556 启动
tmd -server

# 指定端口启动
tmd -server -port 8080
```

### 启动参数

| 参数        | 说明               | 默认值     |
| --------- | ---------------- | ------- |
| `-server` | 启用 API Server 模式 | -       |
| `-port`   | API Server 监听端口  | `25556` |

## API 端点

### 1. 健康检查

检查 API Server 是否正常运行。

**请求：**

```http
GET /api/v1/health
```

**响应：**

```json
{
  "status": "ok",
  "version": "2.0.0",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**示例：**

```bash
curl http://localhost:25556/api/v1/health
```

***

### 2. 下载用户推文

下载指定用户的推文媒体。

**请求：**

```http
POST /api/v1/users/{screen_name}/download
Content-Type: application/json

{
  "auto_follow": false,
  "skip_profile": false,
  "no_retry": false
}
```

**URL 参数：**

| 字段            | 类型     | 必填 | 说明                          |
| ------------- | ------ | -- | --------------------------- |
| `screen_name` | string | 是  | 用户 Twitter 用户名（例如：elonmusk） |

**请求体参数：**

| 字段             | 类型   | 必填 | 默认值     | 说明                  |
| -------------- | ---- | -- | ------- | ------------------- |
| `auto_follow`  | bool | 否  | `false` | 自动关注受保护用户           |
| `skip_profile` | bool | 否  | `false` | 跳过 Profile 下载（默认下载） |
| `no_retry`     | bool | 否  | `false` | 失败后不重试              |

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_abc123",
    "status": "queued",
    "user": {
      "id": 44196397,
      "screen_name": "elonmusk",
      "name": "Elon Musk"
    },
    "auto_follow": false,
    "skip_profile": false,
    "no_retry": false,
    "message": "Download task queued successfully"
  }
}
```

**示例：**

```bash
# 基本下载
curl -X POST http://localhost:25556/api/v1/users/elonmusk/download

# 跳过 Profile 下载
curl -X POST http://localhost:25556/api/v1/users/elonmusk/download \
  -H "Content-Type: application/json" \
  -d '{"skip_profile": true}'
```

***

### 3. 下载用户 Profile

仅下载用户的 Profile 信息（头像、横幅、简介等）。

**请求：**

```http
POST /api/v1/users/{screen_name}/profile
```

**URL 参数：**

| 字段            | 类型     | 必填 | 说明                          |
| ------------- | ------ | -- | --------------------------- |
| `screen_name` | string | 是  | 用户 Twitter 用户名（例如：elonmusk） |

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_def456",
    "status": "queued",
    "screen_name": "elonmusk",
    "message": "Profile download task queued"
  }
}
```

**示例：**

```bash
curl -X POST http://localhost:25556/api/v1/users/elonmusk/profile
```

***

### 4. 标记用户为已下载

将用户标记为已下载状态，跳过历史推文。

**请求：**

```http
POST /api/v1/users/{screen_name}/mark
Content-Type: application/json

{
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**URL 参数：**

| 字段            | 类型     | 必填 | 说明                          |
| ------------- | ------ | -- | --------------------------- |
| `screen_name` | string | 是  | 用户 Twitter 用户名（例如：elonmusk） |

**请求体参数：**

| 字段          | 类型     | 必填 | 默认值  | 说明                         |
| ----------- | ------ | -- | ---- | -------------------------- |
| `timestamp` | string | 否  | 当前时间 | 标记时间（ISO 8601格式），不传则使用当前时间 |

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_ghi789",
    "status": "queued",
    "screen_name": "elonmusk",
    "timestamp": "2024-01-15T10:30:00Z",
    "message": "Mark downloaded task queued"
  }
}
```

**注意：** 如果请求中未提供 `timestamp`，响应中的 `timestamp` 字段将为 `null`，任务将使用当前时间执行。

**示例：**

```bash
# 标记为当前时间
curl -X POST http://localhost:25556/api/v1/users/elonmusk/mark

# 指定时间标记
curl -X POST http://localhost:25556/api/v1/users/elonmusk/mark \
  -H "Content-Type: application/json" \
  -d '{"timestamp": "2024-01-01T00:00:00Z"}'
```

***

### 5. 下载关注列表

下载某用户关注的所有用户的推文。

**请求：**

```http
POST /api/v1/users/{screen_name}/following/download
Content-Type: application/json

{
  "auto_follow": false,
  "skip_profile": false,
  "no_retry": false
}
```

**URL 参数：**

| 字段            | 类型     | 必填 | 说明                          |
| ------------- | ------ | -- | --------------------------- |
| `screen_name` | string | 是  | 用户 Twitter 用户名（例如：elonmusk） |

**请求体参数：**

| 字段             | 类型   | 必填 | 默认值     | 说明                       |
| -------------- | ---- | -- | ------- | ------------------------ |
| `auto_follow`  | bool | 否  | `false` | 自动关注受保护用户                |
| `skip_profile` | bool | 否  | `false` | 跳过关注用户的 Profile 下载（默认下载） |
| `no_retry`     | bool | 否  | `false` | 失败后不重试                   |

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_jkl012",
    "status": "queued",
    "user": {
      "id": 44196397,
      "screen_name": "elonmusk",
      "name": "Elon Musk"
    },
    "auto_follow": false,
    "skip_profile": false,
    "no_retry": false,
    "message": "Following download task queued successfully"
  }
}
```

**示例：**

```bash
curl -X POST http://localhost:25556/api/v1/users/elonmusk/following/download

# 跳过 Profile 下载
curl -X POST http://localhost:25556/api/v1/users/elonmusk/following/download \
  -H "Content-Type: application/json" \
  -d '{"skip_profile": true}'
```

***

### 6. 下载列表成员推文

批量下载 Twitter 列表中所有成员的推文。

**请求：**

```http
POST /api/v1/lists/{list_id}/download
Content-Type: application/json

{
  "auto_follow": false,
  "skip_profile": false,
  "no_retry": false
}
```

**URL 参数：**

| 字段        | 类型     | 必填 | 说明                          |
| --------- | ------ | -- | --------------------------- |
| `list_id` | uint64 | 是  | Twitter 列表 ID（例如：123456789） |

**请求体参数：**

| 字段             | 类型   | 必填 | 默认值     | 说明                       |
| -------------- | ---- | -- | ------- | ------------------------ |
| `auto_follow`  | bool | 否  | `false` | 自动关注受保护用户                |
| `skip_profile` | bool | 否  | `false` | 跳过列表成员的 Profile 下载（默认下载） |
| `no_retry`     | bool | 否  | `false` | 失败后不重试                   |

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_mno345",
    "status": "queued",
    "list_id": 123456789,
    "skip_profile": false,
    "auto_follow": false,
    "no_retry": false,
    "message": "List download task queued"
  }
}
```

**示例：**

```bash
curl -X POST http://localhost:25556/api/v1/lists/123456789/download \
  -H "Content-Type: application/json" \
  -d '{"auto_follow": true}'
```

***

### 7. 下载列表成员 Profile

仅下载 Twitter 列表中所有成员的 Profile。

**请求：**

```http
POST /api/v1/lists/{list_id}/profile
```

**URL 参数：**

| 字段        | 类型     | 必填 | 说明                          |
| --------- | ------ | -- | --------------------------- |
| `list_id` | uint64 | 是  | Twitter 列表 ID（例如：123456789） |

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_pqr678",
    "status": "queued",
    "list_id": 123456789,
    "user_count": 50,
    "message": "List profile download task queued"
  }
}
```

**示例：**

```bash
curl -X POST http://localhost:25556/api/v1/lists/123456789/profile
```

***

### 8. 从 JSON 文件下载

从 JSON 文件（如其他工具导出的 Twitter 数据）下载媒体。

**请求：**

```http
POST /api/v1/json/download
Content-Type: application/json

{
  "paths": ["/path/to/tweets1.json", "/path/to/tweets2.json"],
  "no_retry": false
}
```

**请求体参数：**

| 字段         | 类型        | 必填 | 默认值     | 说明                |
| ---------- | --------- | -- | ------- | ----------------- |
| `paths`    | \[]string | 是  | -       | JSON 文件路径列表（绝对路径） |
| `no_retry` | bool      | 否  | `false` | 失败后不重试            |

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_stu901",
    "status": "queued",
    "paths": ["/path/to/tweets1.json", "/path/to/tweets2.json"],
    "no_retry": false,
    "message": "JSON download task queued"
  }
}
```

**示例：**

```bash
curl -X POST http://localhost:25556/api/v1/json/download \
  -H "Content-Type: application/json" \
  -d '{"paths": ["/data/tweets.json"]}'
```

***

### 9. 批量下载

同时下载多个用户和列表。

**请求：**

```http
POST /api/v1/batch/download
Content-Type: application/json

{
  "users": ["elonmusk", "twitter", "github"],
  "lists": [123456789, 987654321],
  "auto_follow": false,
  "skip_profile": false,
  "no_retry": false
}
```

**请求体参数：**

| 字段             | 类型        | 必填 | 默认值     | 说明                  |
| -------------- | --------- | -- | ------- | ------------------- |
| `users`        | \[]string | 否  | -       | 要下载的用户名列表           |
| `lists`        | \[]uint64 | 否  | -       | 要下载的列表 ID 列表        |
| `auto_follow`  | bool      | 否  | `false` | 自动关注受保护用户           |
| `skip_profile` | bool      | 否  | `false` | 跳过 Profile 下载（默认下载） |
| `no_retry`     | bool      | 否  | `false` | 失败后不重试              |

**注意：** `users` 和 `lists` 至少需要一个。

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_vwx234",
    "status": "queued",
    "users": ["elonmusk", "twitter", "github"],
    "lists": [123456789, 987654321],
    "user_count": 3,
    "list_count": 2,
    "auto_follow": false,
    "skip_profile": false,
    "no_retry": false,
    "message": "Batch download task queued"
  }
}
```

**示例：**

```bash
curl -X POST http://localhost:25556/api/v1/batch/download \
  -H "Content-Type: application/json" \
  -d '{
    "users": ["elonmusk", "twitter"],
    "lists": [123456789]
  }'

# 跳过 Profile 下载
curl -X POST http://localhost:25556/api/v1/batch/download \
  -H "Content-Type: application/json" \
  -d '{
    "users": ["elonmusk", "twitter"],
    "lists": [123456789],
    "skip_profile": true
  }'
```

***

### 10. 获取任务列表

获取所有任务的当前状态。

**请求：**

```http
GET /api/v1/tasks
```

**响应：**

```json
{
  "success": true,
  "data": {
    "tasks": [
      {
        "task_id": "task_abc123",
        "type": "user_download",
        "status": "running",
        "progress": {
          "total": 100,
          "completed": 45,
          "failed": 2
        },
        "created_at": "2024-01-15T10:30:00Z",
        "started_at": "2024-01-15T10:30:05Z"
      }
    ],
    "total": 1
  }
}
```

**任务状态：**

- `queued` - 排队中
- `running` - 运行中
- `completed` - 已完成
- `failed` - 失败
- `cancelled` - 已取消

**示例：**

```bash
curl http://localhost:25556/api/v1/tasks
```

***

### 11. 获取任务详情

获取单个任务的详细信息。

**请求：**

```http
GET /api/v1/tasks/{task_id}
```

**URL 参数：**

| 字段        | 类型     | 必填 | 说明                     |
| --------- | ------ | -- | ---------------------- |
| `task_id` | string | 是  | 任务 ID（例如：task\_abc123） |

**响应：**

```json
{
  "success": true,
  "data": {
    "task_id": "task_abc123",
    "type": "user_download",
    "status": "completed",
    "progress": {
      "total": 100,
      "completed": 100,
      "failed": 0
    },
    "result": {
      "downloaded": 100,
      "failed": 0,
      "skipped": 0
    },
    "created_at": "2024-01-15T10:30:00Z",
    "started_at": "2024-01-15T10:30:05Z",
    "ended_at": "2024-01-15T10:35:00Z"
  }
}
```

**示例：**

```bash
curl http://localhost:25556/api/v1/tasks/task_abc123
```

***

### 12. 取消任务

取消正在运行或排队中的任务。

**请求：**

```http
POST /api/v1/tasks/{task_id}/cancel
```

**URL 参数：**

| 字段        | 类型     | 必填 | 说明                     |
| --------- | ------ | -- | ---------------------- |
| `task_id` | string | 是  | 任务 ID（例如：task\_abc123） |

**响应：**

```json
{
  "success": true,
  "data": {
    "message": "Task cancelled"
  }
}
```

**错误响应：**

```json
{
  "success": false,
  "error": "Task cannot be cancelled"
}
```

**示例：**

```bash
curl -X POST http://localhost:25556/api/v1/tasks/task_abc123/cancel
```

***

## 错误处理

### 错误响应格式

```json
{
  "success": false,
  "error": "错误描述信息"
}
```

### HTTP 状态码

| 状态码 | 说明          |
| --- | ----------- |
| 200 | 请求成功        |
| 202 | 任务已创建（异步操作） |
| 400 | 请求参数错误      |
| 404 | 资源不存在       |
| 405 | 方法不允许       |
| 500 | 服务器内部错误     |
| 503 | 服务不可用（数据库连接失败）|

### 日志记录

API Server 记录所有请求的详细信息：

```
2024/01/15 10:30:00 127.0.0.1 GET /api/v1/tasks 200 2.3ms
```

日志包含：客户端 IP、HTTP 方法、请求路径、状态码、处理时间

### 响应写入错误处理

所有 JSON 响应的编码错误都会被记录到日志，便于排查客户端连接问题。

***

## 跨域支持 (CORS)

API 默认启用 CORS 支持，允许 Web 前端直接调用：

- **允许来源：** `*`（所有来源）
- **允许方法：** GET, POST, PUT, DELETE, OPTIONS
- **允许头：** Content-Type, Authorization

SSE 端点 (`/api/v1/sse/tasks`) 同样支持 CORS，确保 Web 界面可以跨域接收实时推送。

***

## 安全特性

### 路径穿越防护

静态文件服务已实施路径穿越防护：

- 自动过滤 `..` 路径组件
- 禁止访问根目录之外的文件
- 仅允许访问嵌入的静态资源

### 配置脱敏

`/api/v1/config` 端点返回的配置信息已脱敏：

- `root_path` 仅返回目录名，不返回完整绝对路径
- 敏感信息（如 Cookie）不会返回

### 缓存控制

Web 界面响应包含适当的缓存头：

| 资源类型 | Cache-Control | 说明 |
|---------|---------------|------|
| HTML 页面 | `public, max-age=3600` | 1小时缓存 |
| 静态资源 | `public, max-age=86400` | 24小时缓存 |
| API 响应 | 无缓存 | 实时数据 |

***

## 任务管理

### 任务生命周期

1. **创建** → `queued`（排队中）
2. **开始执行** → `running`（运行中）
3. **完成** → `completed`（已完成）或 `failed`（失败）
4. **取消** → `cancelled`（已取消）

### 自动清理

- 任务保留时间：8 小时
- 最大任务数：1000 个
- 清理频率：每小时

### SSE 实时更新

Web 界面使用 Server-Sent Events (SSE) 技术实现任务状态实时推送：

- **推送频率**：每 2 秒
- **重连策略**：指数退避（2s → 4s → 8s ... 最大 30s）
- **连接断开**：客户端断开时立即清理 goroutine，无资源泄漏

***

## Web 管理界面

Server 模式提供内置的 Web 管理界面，可通过浏览器访问。

### 访问方式

启动 Server 后，打开浏览器访问：

```
http://localhost:25556/
```

### 功能模块

| 模块 | 功能描述 |
|------|----------|
| **仪表盘** | 显示系统健康状态、任务统计、快速操作入口 |
| **新建任务** | 创建用户下载、列表下载、批量下载、JSON 下载任务 |
| **任务列表** | 实时显示所有任务状态（支持 SSE 实时更新）、进度条、取消操作 |
| **数据浏览** | 查看数据库中的 Users、Lists、User Entities |
| **系统配置** | 显示当前配置信息（脱敏） |

### 实时任务更新

Web 界面使用 Server-Sent Events (SSE) 技术实现任务状态实时推送，无需手动刷新页面即可看到任务进度更新。

***

## 新增 API 端点（Web 集成）

### SSE 实时任务推送

**请求：**

```http
GET /api/v1/sse/tasks
```

**说明：**

- 建立 SSE 连接，服务器每 2 秒推送一次任务列表更新
- 支持跨域访问（CORS）
- 连接断开后会自动重连（指数退避策略）

**响应格式：**

```
data: [{"task_id":"task_xxx","status":"running",...}]
```

**示例：**

```javascript
const sse = new EventSource('http://localhost:25556/api/v1/sse/tasks');
sse.onmessage = (event) => {
    const tasks = JSON.parse(event.data);
    console.log('Tasks updated:', tasks);
};
```

***

### 查询数据库用户

**请求：**

```http
GET /api/v1/db/users
```

**响应：**

```json
{
  "success": true,
  "data": {
    "users": [
      {
        "id": 44196397,
        "screen_name": "elonmusk",
        "name": "Elon Musk",
        "protected": false,
        "friends_count": 100,
        "is_accessible": true
      }
    ],
    "total": 1
  }
}
```

**说明：**

- 返回数据库中最近 100 条用户记录
- 用于 Web 界面数据浏览

***

### 查询数据库列表

**请求：**

```http
GET /api/v1/db/lists
```

**响应：**

```json
{
  "success": true,
  "data": {
    "lists": [
      {
        "id": 123456789,
        "name": "Tech News",
        "owner_uid": 44196397
      }
    ],
    "total": 1
  }
}
```

***

### 查询用户实体

**请求：**

```http
GET /api/v1/db/user-entities
```

**响应：**

```json
{
  "success": true,
  "data": {
    "entities": [
      {
        "id": 1,
        "user_id": 44196397,
        "name": "Elon Musk(elonmusk)",
        "latest_release_time": "2024-01-15 10:30:00",
        "parent_dir": "users",
        "media_count": 150
      }
    ],
    "total": 1
  }
}
```

***

### 获取系统配置

**请求：**

```http
GET /api/v1/config
```

**响应：**

```json
{
  "success": true,
  "data": {
    "root_path": "twitter_downloads",
    "max_download_routine": 20,
    "max_file_name_len": 155
  }
}
```

**说明：**

- 返回脱敏后的配置信息（不包含敏感 Cookie）
- `root_path` 仅返回目录名，不返回完整绝对路径

***

## 数据库管理 API 详解

### 通用查询参数

所有数据库列表查询端点（GET）支持以下通用参数：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码 |
| `pageSize` | int | 20 | 每页数量（最大 100） |
| `sortBy` | string | `id` | 排序字段 |
| `sortOrder` | string | `desc` | 排序方向：`asc` 或 `desc` |
| `q` | string | - | 搜索关键词 |

### 1. 用户管理

#### 查询用户列表

**请求：**

```http
GET /api/v1/db/users?page=1&pageSize=20&sortBy=id&sortOrder=desc&q=elonmusk
```

**查询参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `accessible` | bool | 按可访问状态筛选 |
| `protected` | bool | 按保护状态筛选 |

**响应：**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "44196397",
        "screen_name": "elonmusk",
        "name": "Elon Musk",
        "protected": false,
        "friends_count": 100,
        "is_accessible": true
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```

#### 获取用户详情

**请求：**

```http
GET /api/v1/db/users/44196397
```

**响应：**

```json
{
  "success": true,
  "data": {
    "id": "44196397",
    "screen_name": "elonmusk",
    "name": "Elon Musk",
    "protected": false,
    "friends_count": 100,
    "is_accessible": true
  }
}
```

#### 更新用户

**请求：**

```http
PUT /api/v1/db/users/44196397
Content-Type: application/json

{
  "screen_name": "elonmusk",
  "name": "Elon Musk Updated",
  "friends_count": 150,
  "protected": true,
  "is_accessible": false
}
```

**请求体参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `screen_name` | string | 否 | 用户 Screen Name |
| `name` | string | 否 | 显示名称 |
| `friends_count` | int | 否 | 关注数 |
| `protected` | bool | 否 | 是否受保护 |
| `is_accessible` | bool | 否 | 是否可访问 |

**响应：**

```json
{
  "success": true,
  "data": {
    "id": "44196397",
    "screen_name": "elonmusk",
    "name": "Elon Musk Updated",
    "protected": true,
    "friends_count": 150,
    "is_accessible": false
  }
}
```

#### 删除用户

**请求：**

```http
DELETE /api/v1/db/users/44196397
```

**响应：**

```json
{
  "success": true,
  "data": {
    "message": "User deleted successfully"
  }
}
```

#### 获取用户历史名称

**请求：**

```http
GET /api/v1/db/users/44196397/previous-names
```

**响应：**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "1",
        "uid": "44196397",
        "screen_name": "elonmusk_old",
        "name": "Elon Musk Old Name",
        "record_date": "2023-01-15"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```

***

### 2. 列表管理

#### 查询列表

**请求：**

```http
GET /api/v1/db/lists?page=1&pageSize=20&q=tech
```

**查询参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `ownerId` | string | 按所有者 ID 筛选 |

**响应：**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "123456789",
        "name": "Tech News",
        "owner_uid": "44196397"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```

#### 获取列表详情

**请求：**

```http
GET /api/v1/db/lists/123456789
```

**响应：**

```json
{
  "success": true,
  "data": {
    "id": "123456789",
    "name": "Tech News",
    "owner_uid": "44196397"
  }
}
```

#### 更新列表

**请求：**

```http
PUT /api/v1/db/lists/123456789
Content-Type: application/json

{
  "name": "Updated List Name",
  "owner_uid": "44196397"
}
```

**请求体参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 否 | 列表名称 |
| `owner_uid` | string | 否 | 所有者用户 ID |

**响应：**

```json
{
  "success": true,
  "data": {
    "id": "123456789",
    "name": "Updated List Name",
    "owner_uid": "44196397"
  }
}
```

#### 删除列表

**请求：**

```http
DELETE /api/v1/db/lists/123456789
```

**响应：**

```json
{
  "success": true,
  "data": {
    "message": "List deleted successfully"
  }
}
```

***

### 3. 用户实体管理

#### 查询用户实体

**请求：**

```http
GET /api/v1/db/user-entities?page=1&pageSize=20&q=elonmusk
```

**查询参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `userId` | string | 按用户 ID 筛选 |

**响应：**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "1",
        "user_id": "44196397",
        "name": "Elon Musk(elonmusk)",
        "latest_release_time": "2024-01-15 10:30:00",
        "parent_dir": "users",
        "media_count": 150
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```

#### 获取用户实体详情

**请求：**

```http
GET /api/v1/db/user-entities/1
```

**响应：**

```json
{
  "success": true,
  "data": {
    "id": "1",
    "user_id": "44196397",
    "name": "Elon Musk(elonmusk)",
    "latest_release_time": "2024-01-15 10:30:00",
    "parent_dir": "users",
    "media_count": 150
  }
}
```

#### 更新用户实体

**请求：**

```http
PUT /api/v1/db/user-entities/1
Content-Type: application/json

{
  "name": "Updated Entity Name",
  "parent_dir": "users/updated",
  "media_count": 200
}
```

**请求体参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 否 | 实体名称 |
| `parent_dir` | string | 否 | 父目录路径 |
| `media_count` | int | 否 | 媒体文件数量 |

**响应：**

```json
{
  "success": true,
  "data": {
    "id": "1",
    "user_id": "44196397",
    "name": "Updated Entity Name",
    "latest_release_time": "2024-01-15 10:30:00",
    "parent_dir": "users/updated",
    "media_count": 200
  }
}
```

#### 删除用户实体

**请求：**

```http
DELETE /api/v1/db/user-entities/1
```

**响应：**

```json
{
  "success": true,
  "data": {
    "message": "Entity deleted successfully"
  }
}
```

***

### 4. 列表实体管理

#### 查询列表实体

**请求：**

```http
GET /api/v1/db/list-entities?page=1&pageSize=20&q=listname
```

**查询参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `listId` | string | 按列表 ID 筛选 |

**响应：**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "1",
        "lst_id": "123456789",
        "name": "List Entity Name",
        "parent_dir": "lists"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```

#### 获取列表实体详情

**请求：**

```http
GET /api/v1/db/list-entities/1
```

**响应：**

```json
{
  "success": true,
  "data": {
    "id": "1",
    "lst_id": "123456789",
    "name": "List Entity Name",
    "parent_dir": "lists"
  }
}
```

#### 更新列表实体

**请求：**

```http
PUT /api/v1/db/list-entities/1
Content-Type: application/json

{
  "name": "Updated List Entity",
  "parent_dir": "lists/updated"
}
```

**请求体参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 否 | 实体名称 |
| `parent_dir` | string | 否 | 父目录路径 |

**响应：**

```json
{
  "success": true,
  "data": {
    "id": "1",
    "lst_id": "123456789",
    "name": "Updated List Entity",
    "parent_dir": "lists/updated"
  }
}
```

#### 删除列表实体

**请求：**

```http
DELETE /api/v1/db/list-entities/1
```

**响应：**

```json
{
  "success": true,
  "data": {
    "message": "List entity deleted successfully"
  }
}
```

***

### 5. 用户链接管理

#### 查询用户链接

**请求：**

```http
GET /api/v1/db/user-links?page=1&pageSize=20
```

**查询参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `userId` | string | 按用户 ID 筛选 |
| `listEntityId` | string | 按列表实体 ID 筛选 |

**响应：**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "1",
        "user_id": "44196397",
        "name": "elonmusk_link",
        "parent_lst_entity_id": "1"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```

**说明：**

- 用户链接是关联表，不支持直接编辑/删除
- 通过列表下载和用户下载自动创建/更新

***

## 使用场景示例

### 场景 1：监控下载进度

```bash
# 1. 提交下载任务
TASK_ID=$(curl -s -X POST http://localhost:25556/api/v1/users/elonmusk/download | jq -r '.data.task_id')

# 2. 轮询检查进度
while true; do
  STATUS=$(curl -s http://localhost:25556/api/v1/tasks/$TASK_ID | jq -r '.data.status')
  echo "Task status: $STATUS"
  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
    break
  fi
  sleep 5
done
```

### 场景 2：批量下载多个用户

```bash
curl -X POST http://localhost:25556/api/v1/batch/download \
  -H "Content-Type: application/json" \
  -d '{
    "users": ["user1", "user2", "user3", "user4", "user5"]
  }'

# 跳过 Profile 下载
curl -X POST http://localhost:25556/api/v1/batch/download \
  -H "Content-Type: application/json" \
  -d '{
    "users": ["user1", "user2", "user3", "user4", "user5"],
    "skip_profile": true
  }'
```

### 场景 3：下载列表并监控

```bash
# 下载列表
TASK_ID=$(curl -s -X POST http://localhost:25556/api/v1/lists/123456789/download | jq -r '.data.task_id')

# 如需取消
# curl -X POST http://localhost:25556/api/v1/tasks/$TASK_ID/cancel
```

***

## 注意事项

1. **异步执行** - 所有下载任务都是异步执行的，创建后立即返回 task\_id
2. **Twitter 限流** - API 会自动处理 Twitter 的速率限制
3. **任务取消** - 只能取消 `queued` 或 `running` 状态的任务
4. **存储位置** - 下载的文件保存在配置的数据目录中
5. **日志查看** - 服务器日志显示在控制台，可用于调试

***

## 完整 API 端点速查表

### 核心 API

| 端点                                        | 方法   | 功能             |
| ----------------------------------------- | ---- | -------------- |
| `/api/v1/health`                          | GET  | 健康检查           |
| `/api/v1/users/{name}/download`           | POST | 下载用户推文         |
| `/api/v1/users/{name}/profile`            | POST | 下载用户 Profile   |
| `/api/v1/users/{name}/mark`               | POST | 标记用户为已下载       |
| `/api/v1/users/{name}/following/download` | POST | 下载关注列表         |
| `/api/v1/lists/{id}/download`             | POST | 下载列表成员推文       |
| `/api/v1/lists/{id}/profile`              | POST | 下载列表成员 Profile |
| `/api/v1/json/download`                   | POST | 从 JSON 文件下载    |
| `/api/v1/batch/download`                  | POST | 批量下载           |
| `/api/v1/tasks`                           | GET  | 获取任务列表         |
| `/api/v1/tasks/{id}`                      | GET  | 获取任务详情         |
| `/api/v1/tasks/{id}/cancel`               | POST | 取消任务           |

### Web 界面与数据 API

| 端点                                        | 方法   | 功能               |
| ----------------------------------------- | ---- | ---------------- |
| `/`                                       | GET  | Web 管理界面首页     |
| `/static/*`                               | GET  | 静态资源（CSS/JS）   |
| `/api/v1/sse/tasks`                       | GET  | SSE 实时任务推送     |
| `/api/v1/config`                          | GET  | 获取系统配置（脱敏）  |

### 数据库管理 API

| 端点                                        | 方法   | 功能               |
| ----------------------------------------- | ---- | ---------------- |
| `/api/v1/db/users`                        | GET  | 查询用户列表（分页/排序/搜索） |
| `/api/v1/db/users/{id}`                   | GET  | 获取用户详情       |
| `/api/v1/db/users/{id}`                   | PUT  | 更新用户信息       |
| `/api/v1/db/users/{id}`                   | DELETE | 删除用户         |
| `/api/v1/db/users/{id}/previous-names`    | GET  | 获取用户历史名称    |
| `/api/v1/db/lists`                        | GET  | 查询列表（分页/排序/搜索） |
| `/api/v1/db/lists/{id}`                   | GET  | 获取列表详情       |
| `/api/v1/db/lists/{id}`                   | PUT  | 更新列表信息       |
| `/api/v1/db/lists/{id}`                   | DELETE | 删除列表         |
| `/api/v1/db/user-entities`                | GET  | 查询用户实体（分页/排序/搜索） |
| `/api/v1/db/user-entities/{id}`           | GET  | 获取用户实体详情    |
| `/api/v1/db/user-entities/{id}`           | PUT  | 更新用户实体       |
| `/api/v1/db/user-entities/{id}`           | DELETE | 删除用户实体     |
| `/api/v1/db/list-entities`                | GET  | 查询列表实体（分页/排序/搜索） |
| `/api/v1/db/list-entities/{id}`           | GET  | 获取列表实体详情    |
| `/api/v1/db/list-entities/{id}`           | PUT  | 更新列表实体       |
| `/api/v1/db/list-entities/{id}`           | DELETE | 删除列表实体     |
| `/api/v1/db/user-links`                   | GET  | 查询用户链接（分页/搜索） |

***

## 请求参数汇总表

### URL 参数

| 端点                                               | URL 参数        | 类型     | 说明          |
| ------------------------------------------------ | ------------- | ------ | ----------- |
| `/api/v1/users/{screen_name}/download`           | `screen_name` | string | Twitter 用户名 |
| `/api/v1/users/{screen_name}/profile`            | `screen_name` | string | Twitter 用户名 |
| `/api/v1/users/{screen_name}/mark`               | `screen_name` | string | Twitter 用户名 |
| `/api/v1/users/{screen_name}/following/download` | `screen_name` | string | Twitter 用户名 |
| `/api/v1/lists/{list_id}/download`               | `list_id`     | uint64 | 列表 ID       |
| `/api/v1/lists/{list_id}/profile`                | `list_id`     | uint64 | 列表 ID       |
| `/api/v1/tasks/{task_id}`                        | `task_id`     | string | 任务 ID       |
| `/api/v1/tasks/{task_id}/cancel`                 | `task_id`     | string | 任务 ID       |

### 请求体参数

#### 通用下载选项（适用于用户/列表/批量下载）

| 参数             | 类型   | 默认值     | 说明                  |
| -------------- | ---- | ------- | ------------------- |
| `auto_follow`  | bool | `false` | 自动关注受保护用户           |
| `skip_profile` | bool | `false` | 跳过 Profile 下载（默认下载） |
| `no_retry`     | bool | `false` | 失败后不重试              |

#### 各端点特有参数

| 端点                          | 参数             | 类型        | 必填 | 说明                  |
| --------------------------- | -------------- | --------- | -- | ------------------- |
| `/api/v1/users/{name}/mark` | `timestamp`    | string    | 否  | 标记时间（ISO 8601）      |
| `/api/v1/json/download`     | `paths`        | \[]string | 是  | JSON 文件路径列表         |
| `/api/v1/json/download`     | `no_retry`     | bool      | 否  | 失败后不重试              |
| `/api/v1/batch/download`    | `users`        | \[]string | 否  | 用户名列表               |
| `/api/v1/batch/download`    | `lists`        | \[]uint64 | 否  | 列表 ID 列表            |
| `/api/v1/batch/download`    | `auto_follow`  | bool      | 否  | 自动关注受保护用户           |
| `/api/v1/batch/download`    | `skip_profile` | bool      | 否  | 跳过 Profile 下载（默认下载） |
| `/api/v1/batch/download`    | `no_retry`     | bool      | 否  | 失败后不重试              |

