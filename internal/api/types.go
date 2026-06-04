package api

import (
	"time"

	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/scheduler"
)

// UserDownloadTaskData 用户下载任务数据
type UserDownloadTaskData struct {
	ScreenName    string `json:"screen_name"`
	AutoFollow    bool   `json:"auto_follow"`
	FollowMembers bool   `json:"follow_members"`
	SkipProfile   bool   `json:"skip_profile"`
	NoRetry       bool   `json:"no_retry"`
}

// ListDownloadTaskData 列表下载任务数据
type ListDownloadTaskData struct {
	ListID        StringUint64 `json:"list_id"`
	AutoFollow    bool         `json:"auto_follow"`
	FollowMembers bool         `json:"follow_members"`
	SkipProfile   bool         `json:"skip_profile"`
	NoRetry       bool         `json:"no_retry"`
}

// FollowingDownloadTaskData 关注下载任务数据
type FollowingDownloadTaskData struct {
	ScreenName    string `json:"screen_name"`
	AutoFollow    bool   `json:"auto_follow"`
	FollowMembers bool   `json:"follow_members"`
	SkipProfile   bool   `json:"skip_profile"`
	NoRetry       bool   `json:"no_retry"`
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

// FollowingMarkDownloadedTaskData 标记 following 已下载任务数据
type FollowingMarkDownloadedTaskData struct {
	ScreenName string     `json:"screen_name"`
	Timestamp  *time.Time `json:"timestamp,omitempty"`
}

// ListMarkDownloadedTaskData 标记列表已下载任务数据
type ListMarkDownloadedTaskData struct {
	ListID    StringUint64 `json:"list_id"`
	Timestamp *time.Time   `json:"timestamp,omitempty"`
}

// BatchMarkDownloadedTaskData 批量标记已下载任务数据
type BatchMarkDownloadedTaskData struct {
	Users          []string       `json:"users"`
	Lists          []StringUint64 `json:"lists"`
	FollowingNames []string       `json:"following_names"`
	Timestamp      *time.Time     `json:"timestamp,omitempty"`
}

// JsonFileDownloadTaskData 第三方工具JSON下载任务数据（用户资料）
type JsonFileDownloadTaskData struct {
	Paths      []string `json:"paths"`
	NoRetry    bool     `json:"no_retry"`
	FromUpload bool     `json:"from_upload,omitempty"` // 标记是否来自上传（上传文件在任务结束后会被清理，不可重试）
}

// JsonFolderDownloadTaskData loongtweet文件夹下载任务数据（推文媒体）
type JsonFolderDownloadTaskData struct {
	Paths      []string `json:"paths"`
	NoRetry    bool     `json:"no_retry"`
	FromUpload bool     `json:"from_upload,omitempty"` // 标记是否来自上传（上传文件在任务结束后会被清理，不可重试）
}

// BatchDownloadTaskData 批量下载任务数据
type BatchDownloadTaskData struct {
	Users          []string       `json:"users"`
	Lists          []StringUint64 `json:"lists"`
	FollowingNames []string       `json:"following_names"`
	AutoFollow     bool           `json:"auto_follow"`
	FollowMembers  bool           `json:"follow_members"`
	SkipProfile    bool           `json:"skip_profile"`
	NoRetry        bool           `json:"no_retry"`
}

// ListProfileTaskData 列表 Profile 下载任务数据
type ListProfileTaskData struct {
	ListID StringUint64 `json:"list_id"`
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
}

// TaskStatsResponse 任务统计响应
type TaskStatsResponse struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
	Total     int `json:"total"`
}

// DBUserItem 数据库用户项（前端友好格式）
type DBUserItem struct {
	ID           string `json:"id"`
	ScreenName   string `json:"screen_name"`
	Name         string `json:"name"`
	IsProtected  bool   `json:"protected"`
	FriendsCount int    `json:"friends_count"`
	IsAccessible bool   `json:"is_accessible"`
}

// DBListItem 数据库列表项（前端友好格式）
type DBListItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnerID string `json:"owner_user_id"`
}

// DBEntityItem 数据库用户实体项（前端友好格式）
type DBEntityItem struct {
	ID                string `json:"id"`
	UserID            string `json:"user_id"`
	Name              string `json:"name"`
	LatestReleaseTime string `json:"latest_release_time"`
	ParentDir         string `json:"parent_dir"`
	MediaCount        int32  `json:"media_count"`
}

// DBListEntityItem 数据库列表实体项
type DBListEntityItem struct {
	ID        string `json:"id"`
	LstID     string `json:"lst_id"`
	Name      string `json:"name"`
	ParentDir string `json:"parent_dir"`
}

// DBUserLinkItem 用户链接项
type DBUserLinkItem struct {
	ID                string `json:"id"`
	UserID            string `json:"user_id"`
	Name              string `json:"name"`
	ParentLstEntityID string `json:"parent_lst_entity_id"`
}

// DBUserPreviousNameItem 用户历史名称项
type DBUserPreviousNameItem struct {
	ID                string `json:"id"`
	UserID            string `json:"user_id"`
	ScreenName        string `json:"screen_name"`
	Name              string `json:"name"`
	RecordDate        string `json:"record_date"`
	CurrentScreenName string `json:"current_screen_name"`
	CurrentName       string `json:"current_name"`
}

// ConfigResponse 配置响应（脱敏）
type ConfigResponse struct {
	RootPath           string `json:"root_path"`
	MaxDownloadRoutine int    `json:"max_download_routine"`
	MaxFileNameLen     int    `json:"max_file_name_len"`
}

// ConfigRawResponse 原始配置响应
type ConfigRawResponse struct {
	Content string `json:"content"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
}

// ConfigUpdateRequest 配置更新请求
type ConfigUpdateRequest struct {
	Content string `json:"content"`
}

// LogsResponse 日志响应
type LogsResponse struct {
	Logs       []string `json:"logs"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
	TotalPages int      `json:"totalPages"`
}

// ConfigFieldItem 单个配置字段的 Web 表示
type ConfigFieldItem struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Prompt      string `json:"prompt"`
	Value       string `json:"value"`
	Default     string `json:"default"`
	Type        string `json:"type"`
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
	Group       string `json:"group"`
}

// ConfigFieldsResponse 结构化配置响应
type ConfigFieldsResponse struct {
	Exists bool              `json:"exists"`
	Fields []ConfigFieldItem `json:"fields"`
}

// ConfigFieldsRequest 结构化配置保存请求
type ConfigFieldsRequest struct {
	Fields map[string]string `json:"fields"`
}

// CookieItem 单个额外账户的 Web 表示（脱敏）
type CookieItem struct {
	Index     int    `json:"index"`
	AuthToken string `json:"auth_token"`
	Ct0       string `json:"ct0"`
}

// CookiesRawResponse 原始 cookies 响应
type CookiesRawResponse struct {
	Content string `json:"content"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
}

// CookiesSaveRequest cookies 保存请求（form 模式）
type CookiesSaveRequest struct {
	Cookies []CookieSaveItem `json:"cookies"`
}

// CookieSaveItem 单个额外账户的结构化保存请求。
// Index 是该账户在原始 cookies 文件中的位置，用于稳定解析 __KEEP_OLD__。
type CookieSaveItem struct {
	Index     *int   `json:"index,omitempty"`
	AuthToken string `json:"auth_token"`
	Ct0       string `json:"ct0"`
}

type SchedulesRawResponse struct {
	Content string `json:"content"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
}

type SchedulesReplaceRequest struct {
	Entries []scheduler.ScheduleEntry `json:"entries"`
}

type ScheduleEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type ScheduleValidateRequest struct {
	Entry   *scheduler.ScheduleEntry  `json:"entry,omitempty"`
	Entries []scheduler.ScheduleEntry `json:"entries,omitempty"`
	Raw     string                    `json:"raw,omitempty"`
}

type ScheduleValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// ErrorSummaryResponse 失败推文摘要响应
type ErrorSummaryResponse struct {
	Regular map[int]int                    `json:"regular"`      // entity ID → 失败推文数
	JSON    []downloading.JsonDumpSummary  `json:"json"`         // JSON 文件来源摘要
}
