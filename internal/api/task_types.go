package api

import (
	"context"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type CancelTaskResult string

const (
	CancelTaskResultCancelled      CancelTaskResult = "cancelled"
	CancelTaskResultNotFound       CancelTaskResult = "not_found"
	CancelTaskResultNotCancellable CancelTaskResult = "not_cancellable"
)

type DeleteTaskResult string

const (
	DeleteTaskResultDeleted      DeleteTaskResult = "deleted"
	DeleteTaskResultNotFound     DeleteTaskResult = "not_found"
	DeleteTaskResultNotDeletable DeleteTaskResult = "not_deletable"
)

type RetryTaskResult string

const (
	RetryTaskResultSuccess       RetryTaskResult = "success"
	RetryTaskResultNotFound      RetryTaskResult = "not_found"
	RetryTaskResultNotRetryable  RetryTaskResult = "not_retryable"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeUserDownload       TaskType = "user_download"
	TaskTypeListDownload       TaskType = "list_download"
	TaskTypeFollowingDownload  TaskType = "following_download"
	TaskTypeProfileDownload    TaskType = "profile_download"
	TaskTypeMarkDownloaded     TaskType = "mark_downloaded"
	TaskTypeJsonFileDownload   TaskType = "json_file_download"
	TaskTypeJsonFolderDownload TaskType = "json_folder_download"
	TaskTypeBatchDownload      TaskType = "batch_download"
	TaskTypeListProfile        TaskType = "list_profile"
	TaskTypeRetryAllFailed     TaskType = "retry_all_failed"
)

// Task 任务
type Task struct {
	ID        string             `json:"task_id"`
	EntryID   string             `json:"entry_id,omitempty"`
	Type      TaskType           `json:"type"`
	Status    TaskStatus         `json:"status"`
	Data      interface{}        `json:"data"`
	Progress  *TaskProgress      `json:"progress,omitempty"`
	Result    *TaskResult        `json:"result,omitempty"`
	Error     string             `json:"error,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	StartedAt *time.Time         `json:"started_at,omitempty"`
	EndedAt   *time.Time         `json:"ended_at,omitempty"`
	Ctx       context.Context    `json:"-"`
	Cancel    context.CancelFunc `json:"-"`
}

// TaskProgress 任务进度
type TaskProgress struct {
	Stage     string `json:"stage"` // "syncing", "downloading", "retrying", "profile", "profile_warning", "marking", "completed"
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	Current   string `json:"current"` // 当前处理的用户/列表
}

// TaskResult 任务结果
type TaskMainResult struct {
	Downloaded int `json:"downloaded,omitempty"`
	Failed     int `json:"failed,omitempty"`
}

type TaskProfileResult struct {
	Downloaded int `json:"downloaded,omitempty"`
	Failed     int `json:"failed,omitempty"`
	Versioned  int `json:"versioned,omitempty"` // 版本化（旧文件已备份到 .versions）
}

type TaskResult struct {
	Main    *TaskMainResult    `json:"main,omitempty"`
	Profile *TaskProfileResult `json:"profile,omitempty"`
	Message string             `json:"message,omitempty"`
}

func taskTypeName(t TaskType) string {
	switch t {
	case TaskTypeUserDownload:
		return "User Download"
	case TaskTypeListDownload:
		return "List Download"
	case TaskTypeFollowingDownload:
		return "Following Download"
	case TaskTypeProfileDownload:
		return "Profile Download"
	case TaskTypeMarkDownloaded:
		return "Mark Downloaded"
	case TaskTypeJsonFileDownload:
		return "JSON File Download"
	case TaskTypeJsonFolderDownload:
		return "Folder Download"
	case TaskTypeBatchDownload:
		return "Batch Download"
	case TaskTypeListProfile:
		return "List Profile"
	case TaskTypeRetryAllFailed:
		return "Retry All Failed"
	default:
		return string(t)
	}
}
