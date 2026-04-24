package api

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/cli"
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
		d, ok := data.(*UserDownloadTaskData)
		if !ok {
			return nil, fmt.Errorf("invalid data type for UserDownload, expected *UserDownloadTaskData, got %T", data)
		}
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
		d, ok := data.(*ListDownloadTaskData)
		if !ok {
			return nil, fmt.Errorf("invalid data type for ListDownload, expected *ListDownloadTaskData, got %T", data)
		}
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
		d, ok := data.(*FollowingDownloadTaskData)
		if !ok {
			return nil, fmt.Errorf("invalid data type for FollowingDownload, expected *FollowingDownloadTaskData, got %T", data)
		}
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
		d, ok := data.(*ProfileDownloadTaskData)
		if !ok {
			return nil, fmt.Errorf("invalid data type for ProfileDownload, expected *ProfileDownloadTaskData, got %T", data)
		}
		return []string{"-profile-user", d.ScreenName, "-noprofile"}, nil

	case TaskTypeMarkDownloaded:
		d, ok := data.(*MarkDownloadedTaskData)
		if !ok {
			return nil, fmt.Errorf("invalid data type for MarkDownloaded, expected *MarkDownloadedTaskData, got %T", data)
		}
		args := []string{"-user", d.ScreenName, "-mark-downloaded"}
		if d.Timestamp != nil {
			args = append(args, "-mark-time", d.Timestamp.Format("2006-01-02T15:04:05"))
		}
		return args, nil

	case TaskTypeJsonDownload:
		d, ok := data.(*JsonDownloadTaskData)
		if !ok {
			return nil, fmt.Errorf("invalid data type for JsonDownload, expected *JsonDownloadTaskData, got %T", data)
		}
		args := []string{"-json"}
		args = append(args, d.Paths...)
		if d.NoRetry {
			args = append(args, "-no-retry")
		}
		return args, nil

	case TaskTypeBatchDownload:
		d, ok := data.(*BatchDownloadTaskData)
		if !ok {
			return nil, fmt.Errorf("invalid data type for BatchDownload, expected *BatchDownloadTaskData, got %T", data)
		}
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

	case TaskTypeListProfile:
		d, ok := data.(*ListProfileTaskData)
		if !ok {
			return nil, fmt.Errorf("invalid data type for ListProfile, expected *ListProfileTaskData, got %T", data)
		}
		return []string{"-profile-list", fmt.Sprintf("%d", d.ListID), "-noprofile"}, nil

	default:
		return nil, fmt.Errorf("unknown task type: %s", taskType)
	}
}
