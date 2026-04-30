package service

import (
	"context"
	"fmt"

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

func (s *downloadServiceImpl) resolveUsers(ctx context.Context, screenNames []string) []*twitter.User {
	var users []*twitter.User
	for _, name := range screenNames {
		user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, name)
		if err != nil {
			if s.deps.DB != nil {
				database.MarkUserInaccessible(s.deps.DB, uid, name)
			}
			log.Warnf("Failed to get user %s: %v", name, err)
			continue
		}
		users = append(users, user)
	}
	return users
}

func (s *downloadServiceImpl) resolveLists(ctx context.Context, listIDs []uint64) []twitter.ListBase {
	var lists []twitter.ListBase
	for _, id := range listIDs {
		list, err := twitter.GetLst(ctx, s.deps.Client, id)
		if err != nil {
			log.Warnf("Failed to get list %d: %v", id, err)
			continue
		}
		lists = append(lists, list)
	}
	return lists
}

func (s *downloadServiceImpl) resolveFollowings(ctx context.Context, screenNames []string) []twitter.ListBase {
	var lists []twitter.ListBase
	for _, name := range screenNames {
		user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, name)
		if err != nil {
			if s.deps.DB != nil {
				database.MarkUserInaccessible(s.deps.DB, uid, name)
			}
			log.Warnf("Failed to get user %s for following list: %v", name, err)
			continue
		}
		lists = append(lists, user.Following())
	}
	return lists
}

// initDownloader 初始化下载器组件，返回 versionManager, fileWriter, downloader
func (s *downloadServiceImpl) initDownloader() (*downloader.DefaultVersionManager, *downloader.DefaultFileWriter, *downloader.DefaultDownloader) {
	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)
	return versionManager, fileWriter, dwn
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

	user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
	if err != nil {
		if s.deps.DB != nil {
			database.MarkUserInaccessible(s.deps.DB, uid, screenName)
		}
		return err
	}

	// 初始化下载器
	versionManager, fileWriter, dwn := s.initDownloader()

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
		if err := s.downloadProfile(ctx, taskID, []*twitter.User{user}, pathHelper, versionManager, fileWriter, dwn, reporter); err != nil {
			log.Warnf("Profile download failed for %s: %v", screenName, err)
			reporter.OnProgress(taskID, Progress{Stage: "profile_warning", Current: fmt.Sprintf("profile failed for %s: %v", screenName, err)})
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
	versionManager, fileWriter, dwn := s.initDownloader()

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
		memberIDs := utils.ExtractIDs(listMembers, func(u *twitter.User) uint64 { return u.Id })
		database.MarkListMembersAccessibleByIDs(s.deps.DB, memberIDs)

		if err := s.downloadProfile(ctx, taskID, listMembers, pathHelper, versionManager, fileWriter, dwn, reporter); err != nil {
			log.Warnf("Profile download failed for list %d: %v", listID, err)
			reporter.OnProgress(taskID, Progress{Stage: "profile_warning", Current: fmt.Sprintf("profile failed for list %d: %v", listID, err)})
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

	user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
	if err != nil {
		if s.deps.DB != nil {
			database.MarkUserInaccessible(s.deps.DB, uid, screenName)
		}
		return err
	}
	following := user.Following()

	// 初始化下载器
	versionManager, fileWriter, dwn := s.initDownloader()

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
		memberIDs := utils.ExtractIDs(listMembers, func(u *twitter.User) uint64 { return u.Id })
		database.MarkListMembersAccessibleByIDs(s.deps.DB, memberIDs)

		if err := s.downloadProfile(ctx, taskID, listMembers, pathHelper, versionManager, fileWriter, dwn, reporter); err != nil {
			log.Warnf("Profile download failed for following %s: %v", screenName, err)
			reporter.OnProgress(taskID, Progress{Stage: "profile_warning", Current: fmt.Sprintf("profile failed for following %s: %v", screenName, err)})
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

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	versionManager, fileWriter, dwn := s.initDownloader()

	unique := make([]string, 0)
	seen := make(map[string]bool)
	for _, name := range screenNames {
		if !seen[name] {
			seen[name] = true
			unique = append(unique, name)
		}
	}
	users := s.resolveUsers(ctx, unique)

	if err := s.downloadProfile(ctx, taskID, users, pathHelper, versionManager, fileWriter, dwn, reporter); err != nil {
		return err
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
func (s *downloadServiceImpl) MarkDownloaded(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, markTime *string, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "resolving"})

	users := s.resolveUsers(ctx, screenNames)
	lists := s.resolveLists(ctx, listIDs)
	lists = append(lists, s.resolveFollowings(ctx, followingNames)...)

	if len(users) == 0 && len(lists) == 0 {
		return fmt.Errorf("no users or lists to mark (all failed to resolve)")
	}

	reporter.OnProgress(taskID, Progress{Stage: "marking", Total: len(users) + len(lists), Current: fmt.Sprintf("%d users, %d lists", len(users), len(lists))})

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
// 注意：noRetry 参数设计如此，第三方 JSON 文件下载不涉及 TweetDumper 机制，
// 失败项不会进入 error.json，因此无需重试逻辑
func (s *downloadServiceImpl) JsonFileDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths), Current: fmt.Sprintf("%d JSON files", len(paths))})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	_, fileWriter, dwn := s.initDownloader()

	// 使用 pathHelper.Users 确保与 profile 下载目录一致
	// 日志已在 downloading 层打印
	results := downloading.DownloadThirdPartyTweets(ctx, s.deps.Client, pathHelper.Users, dwn, fileWriter, paths...)

	var successCount, failCount, totalMedia int
	for _, r := range results {
		if r.Success {
			successCount++
			totalMedia += r.MediaCount
		} else {
			failCount++
		}
	}

	reporter.OnComplete(taskID, Result{
		Downloaded: successCount,
		Failed:     failCount,
		Message:    fmt.Sprintf("JSON file download: %d success, %d failed, %d media", successCount, failCount, totalMedia),
	})
	return nil
}

// JsonFolderDownload 从TMD生成的.loongtweet文件夹下载推文媒体
// 注意：noRetry 参数设计如此，loongtweet 文件夹下载不涉及 TweetDumper 机制，
// 失败项不会进入 error.json，因此无需重试逻辑
func (s *downloadServiceImpl) JsonFolderDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths), Current: fmt.Sprintf("%d loongtweet folders", len(paths))})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	_, fileWriter, dwn := s.initDownloader()

	// 使用 pathHelper.Users 确保与 profile 下载目录结构一致
	// 日志已在 downloading 层打印
	results := downloading.DownloadFromLoongTweetFolder(ctx, s.deps.Client, pathHelper.Users, dwn, fileWriter, paths...)

	var successCount, failCount int
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	reporter.OnComplete(taskID, Result{
		Downloaded: successCount,
		Failed:     failCount,
		Message:    fmt.Sprintf("JSON folder download: %d success, %d failed", successCount, failCount),
	})
	return nil
}

// BatchDownload 批量下载
func (s *downloadServiceImpl) BatchDownload(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, opts DownloadOptions, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = &NopReporter{}
	}

	reporter.OnProgress(taskID, Progress{Stage: "resolving"})

	users := s.resolveUsers(ctx, screenNames)
	lists := s.resolveLists(ctx, listIDs)
	lists = append(lists, s.resolveFollowings(ctx, followingNames)...)

	if len(users) == 0 && len(lists) == 0 {
		return fmt.Errorf("all users and lists failed to resolve")
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
	versionManager, fileWriter, dwn := s.initDownloader()

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
			if err := s.downloadProfile(ctx, taskID, profileUsers, pathHelper, versionManager, fileWriter, dwn, reporter); err != nil {
				log.Warnf("Profile download failed for batch: %v", err)
				reporter.OnProgress(taskID, Progress{Stage: "profile_warning", Current: fmt.Sprintf("profile failed for batch: %v", err)})
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
		} else {
			log.Infof("%d tweets have been dumped", dumper.Count())
		}
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
func (s *downloadServiceImpl) downloadProfile(ctx context.Context, taskID string, users []*twitter.User, pathHelper *path.StorePath, versionManager *downloader.DefaultVersionManager, fileWriter *downloader.DefaultFileWriter, dwn *downloader.DefaultDownloader, reporter ProgressReporter) error {
	if len(users) == 0 {
		return nil
	}

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
			ScreenName:  user.ScreenName,
			UserTitle:   user.Title(),
			Name:        user.Name,
			UserID:      user.Id,
			AvatarURL:   user.AvatarURL,
			BannerURL:   user.BannerURL,
			Description: user.Description,
			Location:    user.Location,
			URL:         user.URL,
			Verified:    user.Verified,
			Protected:   user.IsProtected,
			CreatedAt:   user.CreatedAt,
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
