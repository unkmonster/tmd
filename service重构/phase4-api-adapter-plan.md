# Phase 4: API 适配计划

## 1. 目标

将 API 层从调用 CLI 改为直接调用 Service 层，实现真正的进度报告（SSE 推送），移除冗余的 AsyncExecutor 和 BuildArgs。

## 2. 设计原则

遵循 CLAUDE.md 准则：
- **简单优先**：API Handler 只做 HTTP 相关处理，业务逻辑交给 Service
- **外科手术式修改**：保留 API 端点和响应格式，只修改内部实现
- **目标驱动**：确保 API 行为与重构前一致，但增加进度追踪能力

## 3. 当前 API 架构分析

```
当前 API 流程:
HTTP Request → Server.handler → BuildArgs() → AsyncExecutor.Execute() → cli.Execute()
                                                      ↓
                                               TaskManager (状态管理)
                                                      ↓
                                               SSE 推送 (仅状态变更)
```

```
目标 API 流程:
HTTP Request → Server.handler → Service.UserDownload/ListDownload/...
                                        ↓
                                 TaskManager (状态管理)
                                        ↓
                                 SSEProgressReporter (实时进度推送)
```

## 4. 模块划分

```
internal/
├── api/
│   ├── server.go           # 修改：直接调用 Service
│   ├── handlers.go         # 修改：简化 handler 逻辑
│   ├── task_manager.go     # 修改：集成 Service 进度
│   ├── async_executor.go   # 删除：逻辑移至 Service
│   ├── types.go            # 修改：简化任务数据类型
│   ├── progress.go         # 新增：SSE 进度报告实现
│   └── web/                # 保持不变
├── service/                # Phase 2 创建
│   └── ...
```

## 5. 详细步骤

### 步骤 1: 创建 SSE 进度报告实现

**文件**: `internal/api/progress.go`

**新增代码**:
```go
package api

import (
	"encoding/json"
	"fmt"
	"github.com/unkmonster/tmd/internal/service"
	log "github.com/sirupsen/logrus"
)

// SSEProgressReporter SSE 进度报告器
type SSEProgressReporter struct {
	server    *Server
	taskID    string
	userID    string // SSE 客户端标识
}

// NewSSEProgressReporter 创建 SSE 进度报告器
func NewSSEProgressReporter(server *Server, taskID, userID string) service.ProgressReporter {
	return &SSEProgressReporter{
		server: server,
		taskID: taskID,
		userID: userID,
	}
}

func (s *SSEProgressReporter) OnProgress(taskID string, p service.Progress) {
	// 构建 SSE 事件
	event := map[string]interface{}{
		"type":      "progress",
		"task_id":   taskID,
		"stage":     p.Stage,
		"total":     p.Total,
		"completed": p.Completed,
		"failed":    p.Failed,
		"current":   p.Current,
	}
	
	data, _ := json.Marshal(event)
	s.server.broadcastSSE(s.userID, string(data))
	
	// 同时更新 TaskManager 中的进度
	s.server.taskManager.UpdateTaskProgress(taskID, &TaskProgress{
		Stage:     p.Stage,
		Total:     p.Total,
		Completed: p.Completed,
		Failed:    p.Failed,
		Current:   p.Current,
	})
}

func (s *SSEProgressReporter) OnComplete(taskID string, r service.Result) {
	event := map[string]interface{}{
		"type":      "complete",
		"task_id":   taskID,
		"downloaded": r.Downloaded,
		"failed":    r.Failed,
		"skipped":   r.Skipped,
		"message":   r.Message,
	}
	
	data, _ := json.Marshal(event)
	s.server.broadcastSSE(s.userID, string(data))
	
	// 更新任务状态
	s.server.taskManager.UpdateTaskStatus(taskID, TaskStatusCompleted)
}

func (s *SSEProgressReporter) OnError(taskID string, err error) {
	event := map[string]interface{}{
		"type":    "error",
		"task_id": taskID,
		"error":   err.Error(),
	}
	
	data, _ := json.Marshal(event)
	s.server.broadcastSSE(s.userID, string(data))
	
	// 更新任务状态
	s.server.taskManager.SetTaskError(taskID, err)
}
```

**风险评估**:
- 风险: 中
- 注意: 需要实现 `broadcastSSE` 方法，确保线程安全

**测试要点**:
- 编译通过
- SSE 事件格式正确
- 线程安全（并发任务同时报告进度）

---

### 步骤 2: 修改 Server 结构体添加 SSE 广播功能

**文件**: `internal/api/server.go`

**修改前**:
```go
// Server API Server
type Server struct {
	client            *resty.Client
	additionalClients []*resty.Client
	db                *sqlx.DB
	config            *config.Config
	appRootPath       string
	taskManager       *TaskManager
	asyncExecutor     *AsyncExecutor
}
```

**修改后**:
```go
// Server API Server
type Server struct {
	client            *resty.Client
	additionalClients []*resty.Client
	db                *sqlx.DB
	config            *config.Config
	appRootPath       string
	taskManager       *TaskManager
	
	// Service 层依赖（新增）
	downloadService   service.DownloadService
	
	// SSE 客户端管理（新增）
	sseClients        map[string]chan string
	sseMutex          sync.RWMutex
}
```

**新增方法**:
```go
// broadcastSSE 广播 SSE 事件到指定客户端
func (s *Server) broadcastSSE(userID string, data string) {
	s.sseMutex.RLock()
	ch, ok := s.sseClients[userID]
	s.sseMutex.RUnlock()
	
	if ok {
		select {
		case ch <- data:
		default:
			// 通道已满，丢弃旧消息
			log.Warnf("SSE channel full for user %s, dropping message", userID)
		}
	}
}

// registerSSEClient 注册 SSE 客户端
func (s *Server) registerSSEClient(userID string) chan string {
	ch := make(chan string, 100)
	s.sseMutex.Lock()
	s.sseClients[userID] = ch
	s.sseMutex.Unlock()
	return ch
}

// unregisterSSEClient 注销 SSE 客户端
func (s *Server) unregisterSSEClient(userID string) {
	s.sseMutex.Lock()
	if ch, ok := s.sseClients[userID]; ok {
		close(ch)
		delete(s.sseClients, userID)
	}
	s.sseMutex.Unlock()
}
```

**风险评估**:
- 风险: 中
- 注意: 线程安全，防止并发访问 map 导致的 panic

---

### 步骤 3: 修改 NewServer 初始化 Service

**文件**: `internal/api/server.go`

**修改前**:
```go
// NewServer 创建 API Server
func NewServer(client *resty.Client, additionalClients []*resty.Client, db *sqlx.DB, cfg *config.Config, appRootPath string) *Server {
	s := &Server{
		client:            client,
		additionalClients: additionalClients,
		db:                db,
		config:            cfg,
		appRootPath:       appRootPath,
		taskManager:       NewTaskManager(),
	}
	
	s.asyncExecutor = NewAsyncExecutor(s)
	
	return s
}
```

**修改后**:
```go
// NewServer 创建 API Server
func NewServer(client *resty.Client, additionalClients []*resty.Client, db *sqlx.DB, cfg *config.Config, appRootPath string) *Server {
	s := &Server{
		client:            client,
		additionalClients: additionalClients,
		db:                db,
		config:            cfg,
		appRootPath:       appRootPath,
		taskManager:       NewTaskManager(),
		sseClients:        make(map[string]chan string),
	}
	
	// 创建 Service 层
	s.downloadService = service.NewDownloadService(&service.Dependencies{
		Client:            client,
		AdditionalClients: additionalClients,
		DB:                db,
		Config:            cfg,
		AppRootPath:       appRootPath,
	})
	
	return s
}
```

**风险评估**:
- 风险: 低

---

### 步骤 4: 重写 handleUserDownload

**文件**: `internal/api/server.go`

**修改前**:
```go
// handleUserDownload 处理用户下载
func (s *Server) handleUserDownload(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req UserDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = UserDownloadTaskData{}
	}
	req.ScreenName = screenName

	// 创建任务
	task := s.taskManager.CreateTask(TaskTypeUserDownload, &req)

	// 构建参数并执行
	args, err := BuildArgs(TaskTypeUserDownload, &req)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to build args: %v", err))
		return
	}
	s.asyncExecutor.Execute(task.ID, args)

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"screen_name": req.ScreenName,
		"auto_follow": req.AutoFollow,
		"skip_profile": req.SkipProfile,
		"no_retry":    req.NoRetry,
		"message":     "Download task queued successfully",
	}))
}
```

**修改后**:
```go
// handleUserDownload 处理用户下载
func (s *Server) handleUserDownload(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req UserDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = UserDownloadTaskData{}
	}
	req.ScreenName = screenName

	// 创建任务
	task := s.taskManager.CreateTask(TaskTypeUserDownload, &req)

	// 获取用户 ID（用于 SSE）
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	// 创建进度报告器
	reporter := NewSSEProgressReporter(s, task.ID, userID)

	// 异步执行下载
	go func() {
		opts := service.DownloadOptions{
			AutoFollow:  req.AutoFollow,
			SkipProfile: req.SkipProfile,
			NoRetry:     req.NoRetry,
		}
		s.downloadService.UserDownload(task.Ctx, task.ID, screenName, opts, reporter)
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"screen_name": req.ScreenName,
		"auto_follow": req.AutoFollow,
		"skip_profile": req.SkipProfile,
		"no_retry":    req.NoRetry,
		"message":     "Download task queued successfully",
	}))
}
```

**风险评估**:
- 风险: **高**（核心 handler）
- 注意:
  - 确保与旧 API 响应格式一致
  - 错误处理需要完善

---

### 步骤 5: 重写其他 handlers

类似地修改以下 handlers：

1. `handleFollowingDownload`
2. `handleListDownload`
3. `handleListProfile`
4. `handleJsonDownload`
5. `handleBatchDownload`

**模式**:
```go
// 创建任务
task := s.taskManager.CreateTask(TaskTypeXXX, &req)

// 创建进度报告器
reporter := NewSSEProgressReporter(s, task.ID, userID)

// 异步执行
go func() {
    s.downloadService.XXX(task.Ctx, task.ID, ..., reporter)
}()

// 返回响应
s.writeJSON(w, http.StatusAccepted, ...)
```

---

### 步骤 6: 修改 SSE handler

**文件**: `internal/api/server.go`

**修改前**:
```go
// handleSSETasks 处理 SSE 任务流
func (s *Server) handleSSETasks(w http.ResponseWriter, r *http.Request) {
	// 设置 SSE 头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}

	// 发送初始任务列表
	tasks := s.taskManager.GetAllTasks()
	for _, task := range tasks {
		data, _ := json.Marshal(task)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	// 订阅任务变更
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// 发送心跳或更新
			fmt.Fprintf(w, "data: %s\n\n", `{"type":"heartbeat"}`)
			flusher.Flush()
		}
	}
}
```

**修改后**:
```go
// handleSSETasks 处理 SSE 任务流
func (s *Server) handleSSETasks(w http.ResponseWriter, r *http.Request) {
	// 设置 SSE 头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}

	// 获取用户 ID
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	// 注册 SSE 客户端
	ch := s.registerSSEClient(userID)
	defer s.unregisterSSEClient(userID)

	// 发送初始任务列表
	tasks := s.taskManager.GetAllTasks()
	for _, task := range tasks {
		data, _ := json.Marshal(task)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	// 监听 SSE 事件
	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
```

**风险评估**:
- 风险: 中
- 注意: 确保客户端断开连接时能正确清理资源

---

### 步骤 7: 删除 AsyncExecutor

**删除文件**: `internal/api/async_executor.go`

**原因**: 逻辑已移至 Service 层，API 直接调用 Service。

---

### 步骤 8: 简化 types.go

**文件**: `internal/api/types.go`

**删除**:
```go
// AsyncExecutor 相关类型（删除）
// BuildArgs 相关类型（删除）
```

**保留**:
```go
// Task 相关类型（保留）
// TaskData 相关类型（保留，但简化）
// API 响应类型（保留）
```

---

## 6. 与现有代码的关系

### 6.1 删除的代码
- `internal/api/async_executor.go` - 逻辑移至 Service

### 6.2 修改的代码
- `internal/api/server.go` - 重写 handlers，添加 SSE 广播
- `internal/api/handlers.go` - 简化 handler 逻辑
- `internal/api/task_manager.go` - 更新进度存储
- `internal/api/types.go` - 简化类型定义

### 6.3 新增的代码
- `internal/api/progress.go` - SSE 进度报告

### 6.4 保持不变的代码
- `internal/api/web/*` - Web UI 静态文件

---

## 7. 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| SSE 连接泄漏 | 中 | 高 | 1. 确保 unregisterSSEClient 被调用<br>2. 设置连接超时<br>3. 定期清理僵尸连接 |
| 并发安全问题 | 中 | 高 | 1. 使用 sync.RWMutex 保护 sseClients map<br>2. 使用 channel 缓冲避免阻塞 |
| API 响应格式变化 | 低 | 高 | 1. 保持与旧 API 响应格式一致<br>2. 对比测试响应内容 |
| Service 层未完整实现 | 中 | 高 | 1. 确保 Phase 2 完成后再进行 Phase 4<br>2. 检查所有功能点 |

---

## 8. 验证步骤

### 8.1 编译验证
```bash
cd C:\Users\leeexxx\Documents\trae_projects\tmd
go build ./...
```

### 8.2 API 功能测试
```bash
# 启动服务器
tmd -server

# 测试用户下载
curl -X POST http://localhost:25556/api/v1/users/testuser/download \
  -H "Content-Type: application/json" \
  -d '{"auto_follow": true}'

# 测试列表下载
curl -X POST http://localhost:25556/api/v1/lists/12345/download

# 测试任务查询
curl http://localhost:25556/api/v1/tasks/{task_id}

# 测试 SSE（使用浏览器或 curl）
curl http://localhost:25556/api/v1/sse/tasks \
  -H "Accept: text/event-stream"
```

### 8.3 进度报告测试
- 启动下载任务
- 连接 SSE 端点
- 验证进度事件实时推送
- 验证完成/错误事件推送

---

## 9. 成功标准

- [ ] API 可以正常编译
- [ ] 所有 API 端点正常工作
- [ ] 响应格式与重构前一致
- [ ] SSE 进度报告正常工作
- [ ] 任务状态管理正常工作
- [ ] 代码符合 CLAUDE.md 准则

---

## 10. 预计时间

- 步骤 1 (SSE 进度报告): 2 小时
- 步骤 2-3 (Server 修改): 2 小时
- 步骤 4-5 (重写 handlers): 3-4 小时
- 步骤 6 (SSE handler): 1 小时
- 步骤 7-8 (清理): 1 小时
- 测试与验证: 3-4 小时
- **总计**: 12-14 小时

---

## 11. 回滚方案

如果出现问题，可以：
1. 恢复 `internal/api/server.go` 到原版本
2. 恢复 `internal/api/async_executor.go`
3. 移除 Service 相关代码

建议在进行 Phase 4 前，先备份当前可以工作的代码。

---

## 12. 后续优化（可选）

- 添加 SSE 心跳机制
- 实现任务队列限制并发数
- 添加 WebSocket 支持（替代 SSE）
- 实现任务优先级调度
