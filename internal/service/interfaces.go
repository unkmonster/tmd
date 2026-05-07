package service

import (
	"context"
)

// DownloadOptions 下载选项
type DownloadOptions struct {
	AutoFollow    bool
	FollowMembers bool
	SkipProfile   bool
	NoRetry       bool
}

// DownloadService 下载服务接口
type DownloadService interface {
	UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error

	ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error

	FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error

	ProfileDownload(ctx context.Context, taskID string, screenNames []string, reporter ProgressReporter) error

	ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter ProgressReporter) error

	MarkDownloaded(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, markTime *string, reporter ProgressReporter) error

	// JsonFileDownload 从第三方工具导出的JSON文件下载用户资料（头像/横幅/metadata）
	JsonFileDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error

	// JsonFolderDownload 从TMD生成的.loongtweet文件夹下载推文媒体
	JsonFolderDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error

	BatchDownload(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, opts DownloadOptions, reporter ProgressReporter) error
}
