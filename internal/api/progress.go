package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/unkmonster/tmd/internal/service"
)

// SSEProgressReporter SSE 进度报告器
type SSEProgressReporter struct {
	server *Server
	taskID string
}

// NewSSEProgressReporter 创建 SSE 进度报告器
func NewSSEProgressReporter(server *Server, taskID string) service.ProgressReporter {
	return &SSEProgressReporter{
		server: server,
		taskID: taskID,
	}
}

func (r *SSEProgressReporter) OnProgress(taskID string, p service.Progress) {
	// 更新 TaskManager 中的进度（包含完整信息）
	r.server.taskManager.UpdateTaskProgress(taskID, &TaskProgress{
		Stage:     p.Stage,
		Total:     p.Total,
		Completed: p.Completed,
		Failed:    p.Failed,
		Current:   p.Current,
	})
}

func (r *SSEProgressReporter) OnComplete(taskID string, result service.Result) {
	// 更新任务结果
	r.server.taskManager.SetTaskResult(taskID, &TaskResult{
		Downloaded: result.Downloaded,
		Failed:     result.Failed,
		Versioned:  result.Versioned,
		Message:    result.Message,
	})
	// 更新任务状态为完成
	r.server.taskManager.UpdateTaskStatus(taskID, TaskStatusCompleted)
}

func (r *SSEProgressReporter) OnError(taskID string, err error) {
	// 设置任务错误
	r.server.taskManager.SetTaskError(taskID, err)
}

// sseClient SSE 客户端
type sseClient struct {
	id     string
	writer http.ResponseWriter
	done   chan struct{}
}

// sseManager SSE 管理器
type sseManager struct {
	clients map[string]*sseClient
	mu      sync.RWMutex
}

// newSSEManager 创建 SSE 管理器
func newSSEManager() *sseManager {
	return &sseManager{
		clients: make(map[string]*sseClient),
	}
}

// register 注册客户端
func (m *sseManager) register(client *sseClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[client.id] = client
}

// unregister 注销客户端
func (m *sseManager) unregister(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, id)
}

// broadcast 广播消息
func (m *sseManager) broadcast(data []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, client := range m.clients {
		select {
		case <-client.done:
			// 客户端已断开
		default:
			// 发送数据
			fmt.Fprintf(client.writer, "data: %s\n\n", data)
			if flusher, ok := client.writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// SSEEvent SSE 事件（使用 TaskProgress 和 TaskResult，避免类型重复）
type SSEEvent struct {
	Type      string        `json:"type"`
	TaskID    string        `json:"task_id,omitempty"`
	Progress  *TaskProgress `json:"progress,omitempty"`
	Result    *TaskResult   `json:"result,omitempty"`
	Error     string        `json:"error,omitempty"`
	Timestamp int64         `json:"timestamp"`
}

// broadcastProgress 广播进度更新
func (s *Server) broadcastProgress(taskID string, progress service.Progress) {
	event := SSEEvent{
		Type:   "progress",
		TaskID: taskID,
		Progress: &TaskProgress{
			Stage:     progress.Stage,
			Total:     progress.Total,
			Completed: progress.Completed,
			Failed:    progress.Failed,
			Current:   progress.Current,
		},
	}
	data, _ := json.Marshal(event)
	s.sseMgr.broadcast(data)
}

// broadcastComplete 广播完成事件
func (s *Server) broadcastComplete(taskID string, result service.Result) {
	event := SSEEvent{
		Type:   "complete",
		TaskID: taskID,
		Result: &TaskResult{
			Downloaded: result.Downloaded,
			Failed:     result.Failed,
			Versioned:  result.Versioned,
			Message:    result.Message,
		},
	}
	data, _ := json.Marshal(event)
	s.sseMgr.broadcast(data)
}

// broadcastError 广播错误事件
func (s *Server) broadcastError(taskID string, err error) {
	event := SSEEvent{
		Type:   "error",
		TaskID: taskID,
		Error:  err.Error(),
	}
	data, _ := json.Marshal(event)
	s.sseMgr.broadcast(data)
}
