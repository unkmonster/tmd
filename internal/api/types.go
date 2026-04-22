package api

import (
	"time"
)

// UserDownloadTaskData 用户下载任务数据
type UserDownloadTaskData struct {
	ScreenName  string `json:"screen_name"`
	AutoFollow  bool   `json:"auto_follow"`
	SkipProfile bool   `json:"skip_profile"`
	NoRetry     bool   `json:"no_retry"`
}

// ListDownloadTaskData 列表下载任务数据
type ListDownloadTaskData struct {
	ListID      uint64 `json:"list_id"`
	AutoFollow  bool   `json:"auto_follow"`
	SkipProfile bool   `json:"skip_profile"`
	NoRetry     bool   `json:"no_retry"`
}

// FollowingDownloadTaskData 关注下载任务数据
type FollowingDownloadTaskData struct {
	ScreenName  string `json:"screen_name"`
	AutoFollow  bool   `json:"auto_follow"`
	SkipProfile bool   `json:"skip_profile"`
	NoRetry     bool   `json:"no_retry"`
}

// ProfileDownloadTaskData Profile 下载任务数据
type ProfileDownloadTaskData struct {
	ScreenName string `json:"screen_name"`
}

// MarkDownloadedTaskData 标记已下载任务数据
type MarkDownloadedTaskData struct {
	ScreenName string     `json:"screen_name"`
	Timestamp  *time.Time `json:"timestamp,omitempty"`
}

// JsonDownloadTaskData JSON 下载任务数据
type JsonDownloadTaskData struct {
	Paths   []string `json:"paths"`
	NoRetry bool     `json:"no_retry"`
}

// BatchDownloadTaskData 批量下载任务数据
type BatchDownloadTaskData struct {
	Users       []string `json:"users"`
	Lists       []uint64 `json:"lists"`
	AutoFollow  bool     `json:"auto_follow"`
	SkipProfile bool     `json:"skip_profile"`
	NoRetry     bool     `json:"no_retry"`
}

// APIResponse API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(data interface{}) APIResponse {
	return APIResponse{
		Success: true,
		Data:    data,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(err string) APIResponse {
	return APIResponse{
		Success: false,
		Error:   err,
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	Tasks []*Task `json:"tasks"`
	Total int     `json:"total"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID         uint64 `json:"id"`
	ScreenName string `json:"screen_name"`
	Name       string `json:"name"`
}
