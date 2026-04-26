package service

import (
	"context"

	"github.com/unkmonster/tmd/internal/twitter"
)

// DownloadOptions 下载选项
type DownloadOptions struct {
	AutoFollow  bool
	SkipProfile bool
	NoRetry     bool
	MarkTime    *string // 格式: "2006-01-02T15:04:05"
}

// DownloadService 下载服务接口
type DownloadService interface {
	// UserDownload 下载用户推文
	// 对应 CLI: -user <screen_name>
	UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error

	// ListDownload 下载列表推文
	// 对应 CLI: -list <list_id>
	ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error

	// FollowingDownload 下载关注列表
	// 对应 CLI: -foll <screen_name>
	FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error

	// ProfileDownload 下载用户资料
	// 对应 CLI: -profile-user <screen_name>
	ProfileDownload(ctx context.Context, taskID string, screenNames []string, reporter ProgressReporter) error

	// ListProfileDownload 下载列表用户资料
	// 对应 CLI: -profile-list <list_id>
	ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter ProgressReporter) error

	// MarkDownloaded 标记已下载
	// 对应 CLI: -user <screen_name> -mark-downloaded
	MarkDownloaded(ctx context.Context, taskID string, users []*twitter.User, lists []twitter.ListBase, markTime *string, reporter ProgressReporter) error

	// JsonFileDownload 从第三方工具导出的JSON文件下载用户资料（头像/横幅/metadata）
	// 对应 CLI: -jsonfile <paths...>
	JsonFileDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error

	// JsonFolderDownload 从TMD生成的.loongtweet文件夹下载推文媒体
	// 对应 CLI: -jsonfolder <paths...>
	JsonFolderDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error

	// BatchDownload 批量下载
	// 对应 CLI: -user <u1> -user <u2> -list <l1>
	BatchDownload(ctx context.Context, taskID string, users []*twitter.User, lists []twitter.ListBase, opts DownloadOptions, reporter ProgressReporter) error
}
