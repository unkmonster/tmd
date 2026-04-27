package service

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/downloading/profile"
	"github.com/unkmonster/tmd/internal/path"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

type downloadServiceImpl struct {
	deps *Dependencies
}

// UserDownload 下载用户推文
func (s *downloadServiceImpl) UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Current: screenName})

	// 获取存储路径
	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return fmt.Errorf("failed to make store dir: %w", err)
	}

	// 初始化 Dumper
	dumper := downloading.NewDumper()
	if err := dumper.Load(pathHelper.ErrorJ); err != nil {
		log.Warnf("Failed to load dumper: %v", err)
	}
	defer s.saveDumper(dumper, pathHelper.ErrorJ)

	// 获取用户信息
	user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
	if err != nil {
		database.MarkUserInaccessible(s.deps.DB, uid, screenName)
		return err
	}

	// 初始化下载器
	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 执行批量下载（单个用户）
	failedTweets, _, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, nil, []*twitter.User{user}, pathHelper.Root, pathHelper.Users, opts.AutoFollow, s.deps.AdditionalClients, dwn, fileWriter)
	if err != nil {
		return err
	}

	// 收集失败的推文到 Dumper
	s.collectFailedTweets(dumper, failedTweets)

	// 重试失败的推文
	if !opts.NoRetry {
		if err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载
	if !opts.SkipProfile {
		if err := s.downloadProfile(ctx, taskID, []*twitter.User{user}, pathHelper, versionManager, fileWriter, dwn, reporter, opts.SkipProfile); err != nil {
			// Profile 下载失败，但主任务继续
			log.Warnf("Profile download failed for %s: %v", screenName, err)
		}
	}

	reporter.OnComplete(taskID, Result{Message: "User download completed"})
	return nil
}

// ListDownload 下载列表推文
func (s *downloadServiceImpl) ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "syncing", Current: fmt.Sprintf("list:%d", listID)})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	// 初始化 Dumper
	dumper := downloading.NewDumper()
	if err := dumper.Load(pathHelper.ErrorJ); err != nil {
		log.Warnf("Failed to load dumper: %v", err)
	}
	defer s.saveDumper(dumper, pathHelper.ErrorJ)

	// 获取列表信息
	list, err := twitter.GetLst(ctx, s.deps.Client, listID)
	if err != nil {
		return err
	}

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Current: fmt.Sprintf("list:%d", listID)})

	// 初始化下载器
	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 执行下载 - BatchDownloadAny 会处理列表同步并返回列表成员
	failedTweets, listMembers, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, []twitter.ListBase{list}, nil, pathHelper.Root, pathHelper.Users, opts.AutoFollow, s.deps.AdditionalClients, dwn, fileWriter)
	if err != nil {
		return err
	}

	// 收集失败的推文到 Dumper
	s.collectFailedTweets(dumper, failedTweets)

	// 重试失败的推文
	if !opts.NoRetry {
		if err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载（复用 BatchDownloadAny 返回的 listMembers）
	if !opts.SkipProfile && len(listMembers) > 0 {
		reporter.OnProgress(taskID, Progress{Stage: "profile", Current: fmt.Sprintf("list:%d", listID)})
		memberIDs := utils.ExtractIDs(listMembers, func(u *twitter.User) uint64 { return u.Id })
		database.MarkListMembersAccessibleByIDs(s.deps.DB, memberIDs)

		if err := s.downloadProfile(ctx, taskID, listMembers, pathHelper, versionManager, fileWriter, dwn, reporter, opts.SkipProfile); err != nil {
			log.Warnf("Profile download failed for list %d: %v", listID, err)
		}
	}

	reporter.OnComplete(taskID, Result{Message: "List download completed"})
	return nil
}

// FollowingDownload 下载关注列表
func (s *downloadServiceImpl) FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Current: screenName})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	// 初始化 Dumper
	dumper := downloading.NewDumper()
	if err := dumper.Load(pathHelper.ErrorJ); err != nil {
		log.Warnf("Failed to load dumper: %v", err)
	}
	defer s.saveDumper(dumper, pathHelper.ErrorJ)

	// 获取用户信息
	user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
	if err != nil {
		database.MarkUserInaccessible(s.deps.DB, uid, screenName)
		return err
	}

	// 使用 Following() 方法获取关注列表（作为 ListBase）
	following := user.Following()

	// 初始化下载器
	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 执行批量下载 - 将 Following 作为 List 传递给 BatchDownloadAny
	// 这样可以通过 syncListAndGetMembers 同步列表成员，并返回完整 User 对象
	failedTweets, listMembers, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, []twitter.ListBase{following}, nil, pathHelper.Root, pathHelper.Users, opts.AutoFollow, s.deps.AdditionalClients, dwn, fileWriter)
	if err != nil {
		return err
	}

	// 收集失败的推文到 Dumper
	s.collectFailedTweets(dumper, failedTweets)

	// 重试失败的推文
	if !opts.NoRetry {
		if err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载（复用 BatchDownloadAny 返回的 listMembers）
	if !opts.SkipProfile && len(listMembers) > 0 {
		reporter.OnProgress(taskID, Progress{Stage: "profile", Current: fmt.Sprintf("following:%s", screenName)})
		memberIDs := utils.ExtractIDs(listMembers, func(u *twitter.User) uint64 { return u.Id })
		database.MarkListMembersAccessibleByIDs(s.deps.DB, memberIDs)

		if err := s.downloadProfile(ctx, taskID, listMembers, pathHelper, versionManager, fileWriter, dwn, reporter, opts.SkipProfile); err != nil {
			log.Warnf("Profile download failed for following %s: %v", screenName, err)
		}
	}

	reporter.OnComplete(taskID, Result{Message: "Following download completed"})
	return nil
}

// ProfileDownload 下载用户资料
func (s *downloadServiceImpl) ProfileDownload(ctx context.Context, taskID string, screenNames []string, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "profile", Total: len(screenNames)})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 获取所有用户信息（使用 screenName 去重）
	users := make([]*twitter.User, 0)
	seen := make(map[string]bool)
	for _, screenName := range screenNames {
		if seen[screenName] {
			continue
		}
		user, _, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
		if err != nil {
			log.Warnf("Failed to get user %s: %v", screenName, err)
			continue
		}
		seen[screenName] = true
		users = append(users, user)
	}

	if err := s.downloadProfile(ctx, taskID, users, pathHelper, versionManager, fileWriter, dwn, reporter, false); err != nil {
		log.Warnf("Profile download failed: %v", err)
	}

	reporter.OnComplete(taskID, Result{Message: "Profile download completed"})
	return nil
}

// ListProfileDownload 下载列表用户资料
func (s *downloadServiceImpl) ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "syncing", Current: fmt.Sprintf("list:%d", listID)})

	// 获取列表成员
	list, err := twitter.GetLst(ctx, s.deps.Client, listID)
	if err != nil {
		return err
	}

	membersResult, err := list.GetMembers(ctx, s.deps.Client)
	if err != nil {
		return err
	}

	var screenNames []string
	seen := make(map[string]bool)
	for _, u := range membersResult.Users {
		if !seen[u.ScreenName] {
			seen[u.ScreenName] = true
			screenNames = append(screenNames, u.ScreenName)
		}
	}

	return s.ProfileDownload(ctx, taskID, screenNames, reporter)
}

// MarkDownloaded 标记已下载
func (s *downloadServiceImpl) MarkDownloaded(ctx context.Context, taskID string, users []*twitter.User, lists []twitter.ListBase, markTime *string, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "marking", Total: len(users) + len(lists)})

	log.Infof("Marking %d users and %d lists as downloaded", len(users), len(lists))

	if len(users) == 0 && len(lists) == 0 {
		return fmt.Errorf("no users or lists to mark")
	}

	// 构建参数
	var markTimeStr string
	if markTime != nil {
		markTimeStr = *markTime
	}

	// 执行标记
	// 注意：MarkUsersAsDownloaded 内部会自动获取列表成员并标记
	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}
	results, err := downloading.MarkUsersAsDownloaded(ctx, s.deps.Client, s.deps.DB, lists, users, pathHelper.Users, markTimeStr)

	if err != nil {
		return err
	}

	reporter.OnComplete(taskID, Result{Message: fmt.Sprintf("Marked %d users as downloaded", len(results))})
	return nil
}

// JsonFileDownload 从第三方工具导出的JSON文件下载推文媒体
// 支持推文搜索结果格式（包含 media 数组）
func (s *downloadServiceImpl) JsonFileDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	log.Infof("downloading media from %d third-party JSON file(s)...", len(paths))
	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths)})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 使用 pathHelper.Users 确保与 profile 下载目录结构一致
	results := downloading.DownloadThirdPartyTweets(ctx, s.deps.Client, pathHelper.Users, dwn, fileWriter, paths...)

	var successCount, failCount, totalMedia int
	for _, r := range results {
		if r.Success {
			successCount++
			totalMedia += r.MediaCount
			log.Infof("✓ %s: %d media", filepath.Base(r.Path), r.MediaCount)
		} else {
			failCount++
			log.Errorf("✗ %s: %v", filepath.Base(r.Path), r.Error)
		}
	}

	log.Infof("JSON file download completed: %d success, %d failed, %d media", successCount, failCount, totalMedia)

	reporter.OnComplete(taskID, Result{
		Downloaded: successCount,
		Failed:     failCount,
		Message:    fmt.Sprintf("JSON file download: %d success, %d failed, %d media", successCount, failCount, totalMedia),
	})
	return nil
}

// JsonFolderDownload 从TMD生成的.loongtweet文件夹下载推文媒体
func (s *downloadServiceImpl) JsonFolderDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	log.Infof("downloading media from %d loongtweet folder(s)...", len(paths))
	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths)})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 使用 pathHelper.Users 确保与 profile 下载目录结构一致
	results := downloading.DownloadFromLoongTweetFolder(ctx, s.deps.Client, pathHelper.Users, dwn, fileWriter, paths...)

	var successCount, failCount int
	for _, r := range results {
		if r.Success {
			successCount++
			log.Infof("✓ %s: %d tweets processed", filepath.Base(r.Path), r.TweetCount)
		} else {
			failCount++
			log.Errorf("✗ %s: %v", filepath.Base(r.Path), r.Error)
		}
	}

	log.Infof("JSON folder download completed: %d success, %d failed", successCount, failCount)

	reporter.OnComplete(taskID, Result{
		Downloaded: successCount,
		Failed:     failCount,
		Message:    fmt.Sprintf("JSON folder download: %d success, %d failed", successCount, failCount),
	})
	return nil
}

// BatchDownload 批量下载
func (s *downloadServiceImpl) BatchDownload(ctx context.Context, taskID string, users []*twitter.User, lists []twitter.ListBase, opts DownloadOptions, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "preparing"})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	// 初始化 Dumper
	dumper := downloading.NewDumper()
	if err := dumper.Load(pathHelper.ErrorJ); err != nil {
		log.Warnf("Failed to load dumper: %v", err)
	}
	defer s.saveDumper(dumper, pathHelper.ErrorJ)

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(users) + len(lists)})

	// 初始化下载器
	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 执行批量下载（返回列表成员用于 Profile 下载）
	failedTweets, listMembers, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, lists, users, pathHelper.Root, pathHelper.Users, opts.AutoFollow, s.deps.AdditionalClients, dwn, fileWriter)
	if err != nil {
		return err
	}

	// 收集失败的推文到 Dumper
	s.collectFailedTweets(dumper, failedTweets)

	// 重试失败的推文
	if !opts.NoRetry {
		if err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载（复用 BatchDownloadAny 返回的 listMembers，避免重复 API 调用）
	if !opts.SkipProfile && (len(users) > 0 || len(listMembers) > 0) {
		profileUsers := make([]*twitter.User, 0)
		seen := make(map[string]bool) // 使用 screenName 去重（与稳定版一致）

		// 添加直接传入的用户
		for _, user := range users {
			if !seen[user.ScreenName] {
				seen[user.ScreenName] = true
				profileUsers = append(profileUsers, user)
			}
		}

		// 复用 BatchDownloadAny 返回的列表成员（无需再次调用 GetMembers）
		for _, member := range listMembers {
			if !seen[member.ScreenName] {
				seen[member.ScreenName] = true
				profileUsers = append(profileUsers, member)
			}
		}

		if len(profileUsers) > 0 {
			if err := s.downloadProfile(ctx, taskID, profileUsers, pathHelper, versionManager, fileWriter, dwn, reporter, opts.SkipProfile); err != nil {
				log.Warnf("Profile download failed for batch: %v", err)
			}
		}
	}

	reporter.OnComplete(taskID, Result{Message: "Batch download completed"})
	return nil
}

// saveDumper 保存 Dumper 到文件
func (s *downloadServiceImpl) saveDumper(dumper *downloading.TweetDumper, path string) {
	if dumper.Count() > 0 {
		if err := dumper.Dump(path); err != nil {
			log.Warnf("Failed to save dumper: %v", err)
		}
		log.Infof("%d tweets have been dumped", dumper.Count())
	}
}

// collectFailedTweets 收集失败的推文到 Dumper
func (s *downloadServiceImpl) collectFailedTweets(dumper *downloading.TweetDumper, failedTweets []*downloading.TweetInEntity) {
	for _, tweet := range failedTweets {
		if tweet.Entity != nil {
			if id, err := tweet.Entity.Id(); err == nil {
				dumper.Push(id, tweet.Tweet)
			}
		}
	}
}

// 内部辅助方法：下载 Profile
func (s *downloadServiceImpl) downloadProfile(ctx context.Context, taskID string, users []*twitter.User, pathHelper *path.StorePath, versionManager *downloader.DefaultVersionManager, fileWriter *downloader.DefaultFileWriter, dwn *downloader.DefaultDownloader, reporter ProgressReporter, skipProfile bool) error {
	if len(users) == 0 {
		return nil
	}

	log.Infof("Downloading profiles for %d users", len(users))
	reporter.OnProgress(taskID, Progress{Stage: "profile", Total: len(users), Current: users[0].ScreenName})

	// 创建 storage manager
	storage, err := profile.NewFileStorageManager(pathHelper.Users)
	if err != nil {
		log.Errorf("Failed to create file storage manager: %v", err)
		return fmt.Errorf("failed to create profile storage: %w", err)
	}
	storage.SetVersionManager(versionManager)

	// 创建 profile 下载器（使用 DB 版本以同步用户实体信息）
	pd := profile.NewProfileDownloaderWithDB(
		profile.DefaultConfig(),
		storage,
		append([]*resty.Client{s.deps.Client}, s.deps.AdditionalClients...),
		s.deps.DB,
		dwn,
		fileWriter,
	)

	// 构建下载请求
	requests := make([]profile.DownloadRequest, len(users))
	for i, user := range users {
		requests[i] = profile.DownloadRequest{
			ScreenName: user.ScreenName,
			UserTitle:  user.Title(),
			Name:       user.Name,
			UserID:     user.Id,
		}
		// 如果未设置 skipProfile，则填充详细资料字段
		if !skipProfile {
			requests[i].AvatarURL = user.AvatarURL
			requests[i].BannerURL = user.BannerURL
			requests[i].Description = user.Description
			requests[i].Location = user.Location
			requests[i].URL = user.URL
			requests[i].Verified = user.Verified
			requests[i].Protected = user.IsProtected
			requests[i].CreatedAt = user.CreatedAt
		}
	}

	// 执行批量下载
	results := pd.DownloadMultiple(ctx, requests)

	// 统计结果
	var successCount, failCount, versionedFileCount int
	for _, result := range results {
		if result.Error != nil {
			failCount++
		} else if result.Success {
			successCount++
			// 统计被版本化的文件数
			for _, file := range result.Files {
				if file.Versioned {
					versionedFileCount++
				}
			}
		}
	}

	reporter.OnComplete(taskID, Result{
		Downloaded: successCount,
		Failed:     failCount,
		Versioned:  versionedFileCount,
		Message:    fmt.Sprintf("Profile download completed: %d success, %d failed, %d versioned files", successCount, failCount, versionedFileCount),
	})
	return nil
}
