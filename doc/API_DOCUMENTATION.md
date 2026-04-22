# TMD2 API 使用文档

## 概述

TMD2 (Twitter Media Downloader 2) 提供 HTTP REST API，允许通过编程方式控制下载任务。API Server 模式支持 Web/AI 调用。

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
    "message": "Mark downloaded task queued"
  }
}
```

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
    "user_count": 3,
    "list_count": 2,
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

***

## 跨域支持 (CORS)

API 默认启用 CORS 支持，允许 Web 前端直接调用：

- **允许来源：** `*`（所有来源）
- **允许方法：** GET, POST, PUT, DELETE, OPTIONS
- **允许头：** Content-Type, Authorization

***

## 任务管理

### 任务生命周期

1. **创建** → `queued`（排队中）
2. **开始执行** → `running`（运行中）
3. **完成** → `completed`（已完成）或 `failed`（失败）
4. **取消** → `cancelled`（已取消）

### 自动清理

- 任务保留时间：24 小时
- 最大任务数：1000 个
- 清理频率：每小时

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

