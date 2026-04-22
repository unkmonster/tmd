package api

import (
	"fmt"

	"github.com/unkmonster/tmd/internal/cli"
	log "github.com/sirupsen/logrus"
)

// AsyncExecutor 异步执行器
type AsyncExecutor struct {
	taskManager *TaskManager
	server      *Server
}

// NewAsyncExecutor 创建异步执行器
func NewAsyncExecutor(tm *TaskManager, server *Server) *AsyncExecutor {
	return &AsyncExecutor{
		taskManager: tm,
		server:      server,
	}
}

// Execute 异步执行 CLI 命令
func (ae *AsyncExecutor) Execute(taskID string, args []string) {
	task, ok := ae.taskManager.GetTask(taskID)
	if !ok {
		log.Errorf("[AsyncExecutor] Task not found: %s", taskID)
		return
	}

	// 更新状态为运行中
	ae.taskManager.UpdateTaskStatus(taskID, TaskStatusRunning)

	// 构造依赖
	deps := &cli.Dependencies{
		Client:            ae.server.client,
		AdditionalClients: ae.server.additionalClients,
		DB:                ae.server.db,
		Conf:              ae.server.config,
		AppRootPath:       ae.server.appRootPath,
	}

	// 在 goroutine 中执行
	go func() {
		log.Infof("[Task:%s] Starting async execution with args: %v", taskID, args)

		// 使用任务的 context
		err := cli.Execute(task.Ctx, args, deps)

		if err != nil {
			log.Errorf("[Task:%s] Execution failed: %v", taskID, err)
			ae.taskManager.SetTaskError(taskID, err)
		} else {
			log.Infof("[Task:%s] Execution completed", taskID)
			ae.taskManager.UpdateTaskStatus(taskID, TaskStatusCompleted)
		}
	}()
}

// BuildArgs 构建 CLI 参数
func BuildArgs(taskType TaskType, data interface{}) ([]string, error) {
	switch taskType {
	case TaskTypeUserDownload:
		d := data.(*UserDownloadTaskData)
		args := []string{"-user", d.ScreenName}
		if d.AutoFollow {
			args = append(args, "-auto-follow")
		}
		if d.NoRetry {
			args = append(args, "-no-retry")
		}
		if d.SkipProfile {
			args = append(args, "-noprofile")
		}
		return args, nil

	case TaskTypeListDownload:
		d := data.(*ListDownloadTaskData)
		args := []string{"-list", fmt.Sprintf("%d", d.ListID)}
		if d.AutoFollow {
			args = append(args, "-auto-follow")
		}
		if d.NoRetry {
			args = append(args, "-no-retry")
		}
		if d.SkipProfile {
			args = append(args, "-noprofile")
		}
		return args, nil

	case TaskTypeFollowingDownload:
		d := data.(*FollowingDownloadTaskData)
		args := []string{"-foll", d.ScreenName}
		if d.AutoFollow {
			args = append(args, "-auto-follow")
		}
		if d.NoRetry {
			args = append(args, "-no-retry")
		}
		if d.SkipProfile {
			args = append(args, "-noprofile")
		}
		return args, nil

	case TaskTypeProfileDownload:
		d := data.(*ProfileDownloadTaskData)
		return []string{"-profile-user", d.ScreenName, "-noprofile"}, nil

	case TaskTypeMarkDownloaded:
		d := data.(*MarkDownloadedTaskData)
		args := []string{"-user", d.ScreenName, "-mark-downloaded"}
		if d.Timestamp != nil {
			args = append(args, "-mark-time", d.Timestamp.Format("2006-01-02T15:04:05"))
		}
		return args, nil

	case TaskTypeJsonDownload:
		d := data.(*JsonDownloadTaskData)
		args := []string{"-json"}
		args = append(args, d.Paths...)
		if d.NoRetry {
			args = append(args, "-no-retry")
		}
		return args, nil

	case TaskTypeBatchDownload:
		d := data.(*BatchDownloadTaskData)
		args := []string{}
		for _, u := range d.Users {
			args = append(args, "-user", u)
		}
		for _, l := range d.Lists {
			args = append(args, "-list", fmt.Sprintf("%d", l))
		}
		if d.AutoFollow {
			args = append(args, "-auto-follow")
		}
		if d.NoRetry {
			args = append(args, "-no-retry")
		}
		if d.SkipProfile {
			args = append(args, "-noprofile")
		}
		return args, nil

	default:
		return nil, fmt.Errorf("unknown task type: %s", taskType)
	}
}
