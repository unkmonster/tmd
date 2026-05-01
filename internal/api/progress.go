package api

import (
	"github.com/unkmonster/tmd/internal/service"
)

// SSEProgressReporter SSE 进度报告器
type SSEProgressReporter struct {
	server *Server
}

// NewSSEProgressReporter 创建 SSE 进度报告器
func NewSSEProgressReporter(server *Server) service.ProgressReporter {
	return &SSEProgressReporter{
		server: server,
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
	// 自动更新任务结果和状态，避免竞态条件
	taskResult := &TaskResult{Message: result.Message}
	if result.Main != nil {
		taskResult.Main = &TaskMainResult{
			Downloaded: result.Main.Downloaded,
			Failed:     result.Main.Failed,
		}
	}
	if result.Profile != nil {
		taskResult.Profile = &TaskProfileResult{
			Downloaded: result.Profile.Downloaded,
			Failed:     result.Profile.Failed,
			Versioned:  result.Profile.Versioned,
		}
	}
	r.server.taskManager.CompleteTask(taskID, taskResult)
}

func (r *SSEProgressReporter) OnError(taskID string, err error) {
	// 设置任务错误
	r.server.taskManager.SetTaskError(taskID, err)
}
