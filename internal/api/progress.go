package api

import (
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
