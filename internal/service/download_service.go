package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/downloading/profile"
	"github.com/unkmonster/tmd/internal/path"
	"github.com/unkmonster/tmd/internal/twitter"
)

type downloadServiceImpl struct {
	deps     *Dependencies
	dumperMu sync.Mutex
}

func (s *downloadServiceImpl) getReporterOrDefault(reporter ProgressReporter) ProgressReporter {
	if reporter == nil {
		return &NopReporter{}
	}
	return reporter
}

func (s *downloadServiceImpl) completeTask(taskID string, reporter ProgressReporter, message string, stats *Result, warning string) {
	result := Result{Message: message}
	if warning != "" {
		result.Message = fmt.Sprintf("%s (%s)", message, warning)
	}
	if stats != nil {
		if stats.Main != nil {
			main := *stats.Main
			result.Main = &main
		}
		if stats.Profile != nil {
			profile := *stats.Profile
			result.Profile = &profile
		}
	}
	reporter.OnComplete(taskID, result)
}

func (s *downloadServiceImpl) completeProfileTask(taskID string, reporter ProgressReporter, profileResult *ProfileResult) {
	if profileResult == nil {
		reporter.OnComplete(taskID, Result{Message: "No profile downloads performed"})
		return
	}
	profile := *profileResult
	reporter.OnComplete(taskID, Result{
		Profile: &profile,
		Message: formatProfileCompletionMessage(profile),
	})
}

func (s *downloadServiceImpl) newBatchProgressCallback(taskID string, reporter ProgressReporter) downloading.BatchProgressFunc {
	return func(progress downloading.BatchProgress) {
		reporter.OnProgress(taskID, Progress{
			Stage:     "downloading",
			Total:     progress.Total,
			Completed: progress.Completed,
			Failed:    progress.Failed,
			Current:   progress.Current,
		})
	}
}

func (s *downloadServiceImpl) newRetryProgressCallback(taskID string, reporter ProgressReporter) downloading.RetryProgressFunc {
	return func(progress downloading.RetryProgress) {
		reporter.OnProgress(taskID, Progress{
			Stage:     "retrying",
			Total:     progress.Total,
			Completed: progress.Completed,
			Failed:    progress.Failed,
		})
	}
}

func (s *downloadServiceImpl) buildMainDownloadResult(summary downloading.BatchDownloadSummary, failed int) *MainResult {
	if summary.TotalEntities == 0 {
		return nil
	}
	return &MainResult{
		Downloaded: max(0, summary.TotalEntities-failed),
		Failed:     failed,
	}
}

type failedTweetSet map[int]map[uint64]struct{}

func collectFailedTweetSet(failedTweets []*downloading.TweetInEntity) failedTweetSet {
	failures := make(failedTweetSet)
	for _, failedTweet := range failedTweets {
		if failedTweet == nil || failedTweet.Tweet == nil || failedTweet.Entity == nil {
			continue
		}
		entityID, err := failedTweet.Entity.Id()
		if err != nil {
			continue
		}
		if failures[entityID] == nil {
			failures[entityID] = make(map[uint64]struct{})
		}
		failures[entityID][failedTweet.Tweet.Id] = struct{}{}
	}
	return failures
}

func countRemainingFailedEntities(dumper *downloading.TweetDumper, failures failedTweetSet) int {
	if dumper == nil || len(failures) == 0 {
		return 0
	}
	count := 0
	for entityID, tweetIDs := range failures {
		for tweetID := range tweetIDs {
			if dumper.HasTweet(entityID, tweetID) {
				count++
				break
			}
		}
	}
	return count
}

func (s *downloadServiceImpl) resolveUsers(ctx context.Context, screenNames []string) []*twitter.User {
	var users []*twitter.User
	for _, name := range screenNames {
		user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, name)
		if err != nil {
			database.MarkUserInaccessible(s.deps.DB, uid, name)
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
			database.MarkUserInaccessible(s.deps.DB, uid, name)
			log.Warnf("Failed to get user %s for following list: %v", name, err)
			continue
		}
		lists = append(lists, user.Following())
	}
	return lists
}

func shouldFollowMember(user *twitter.User) bool {
	if user == nil || user.Id == 0 {
		return false
	}
	if user.Blocking || user.Muting {
		return false
	}
	return user.Followstate == twitter.FS_UNFOLLOW
}

func (s *downloadServiceImpl) followMembersIfNeeded(ctx context.Context, users []*twitter.User) error {
	if len(users) == 0 {
		return nil
	}

	seen := make(map[uint64]struct{}, len(users))
	for _, user := range users {
		if !shouldFollowMember(user) {
			continue
		}
		if _, ok := seen[user.Id]; ok {
			continue
		}
		seen[user.Id] = struct{}{}

		if err := ctx.Err(); err != nil {
			return err
		}
		if err := twitter.FollowUser(ctx, s.deps.Client, user); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			log.Warnf("Follow member failed for @%s (%d): %v", user.ScreenName, user.Id, err)
			continue
		}
	}
	return nil
}

func effectiveAutoFollow(opts DownloadOptions) bool {
	return opts.AutoFollow && !opts.FollowMembers
}

func dedupeProfileUsers(users []*twitter.User) []*twitter.User {
	if len(users) <= 1 {
		return users
	}

	seen := make(map[string]struct{}, len(users))
	deduped := make([]*twitter.User, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		key := ""
		if user.Id != 0 {
			key = fmt.Sprintf("id:%d", user.Id)
		} else if screenName := strings.ToLower(strings.TrimSpace(user.ScreenName)); screenName != "" {
			key = "screen:" + screenName
		} else {
			key = fmt.Sprintf("ptr:%p", user)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, user)
	}
	return deduped
}

// initDownloader 初始化下载器组件，返回 versionManager, fileWriter, downloader
func (s *downloadServiceImpl) initDownloader() (*downloader.DefaultVersionManager, *downloader.DefaultFileWriter, *downloader.DefaultDownloader) {
	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	dwn := downloader.NewDownloader(fileWriter)
	return versionManager, fileWriter, dwn
}

// UserDownload 下载用户推文
func (s *downloadServiceImpl) UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

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
		database.MarkUserInaccessible(s.deps.DB, uid, screenName)
		return err
	}

	// 初始化下载器
	versionManager, fileWriter, dwn := s.initDownloader()

	progress := s.newBatchProgressCallback(taskID, reporter)
	retryProgress := s.newRetryProgressCallback(taskID, reporter)

	// 执行批量下载（单个用户）
	failedTweets, _, summary, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, nil, []*twitter.User{user}, pathHelper.Root, pathHelper.Users, effectiveAutoFollow(opts), s.deps.AdditionalClients, dwn, fileWriter, progress)
	if err != nil {
		return err
	}
	if opts.FollowMembers {
		if err := s.followMembersIfNeeded(ctx, []*twitter.User{user}); err != nil {
			return err
		}
	}
	mainFailures := collectFailedTweetSet(failedTweets)

	// 收集失败的推文到 Dumper
	s.collectFailedTweets(dumper, failedTweets)

	// 重试失败的推文
	if !opts.NoRetry {
		if _, err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter, retryProgress); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载
	var profileResult *ProfileResult
	profileWarning := ""
	if !opts.SkipProfile {
		profileResult, err = s.downloadProfile(ctx, taskID, []*twitter.User{user}, pathHelper, versionManager, fileWriter, dwn, reporter)
		if err != nil {
			log.Warnf("Profile download failed for %s: %v", screenName, err)
			reporter.OnProgress(taskID, Progress{Stage: "profile_warning", Current: fmt.Sprintf("profile failed for %s: %v", screenName, err)})
			profileWarning = "with profile warnings"
		}
	}

	s.completeTask(taskID, reporter, "User download completed", &Result{
		Main:    s.buildMainDownloadResult(summary, countRemainingFailedEntities(dumper, mainFailures)),
		Profile: cloneProfileResult(profileResult),
	}, profileWarning)
	return nil
}

// ListDownload 下载列表推文
func (s *downloadServiceImpl) ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

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

	progress := s.newBatchProgressCallback(taskID, reporter)
	retryProgress := s.newRetryProgressCallback(taskID, reporter)

	// 执行下载 - BatchDownloadAny 会处理列表同步并返回列表成员
	failedTweets, listMembers, summary, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, []twitter.ListBase{list}, nil, pathHelper.Root, pathHelper.Users, effectiveAutoFollow(opts), s.deps.AdditionalClients, dwn, fileWriter, progress)
	if err != nil {
		return err
	}
	if opts.FollowMembers {
		if err := s.followMembersIfNeeded(ctx, listMembers); err != nil {
			return err
		}
	}
	mainFailures := collectFailedTweetSet(failedTweets)

	// 收集失败的推文到 Dumper
	s.collectFailedTweets(dumper, failedTweets)

	// 重试失败的推文
	if !opts.NoRetry {
		if _, err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter, retryProgress); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载（复用 BatchDownloadAny 返回的 listMembers）
	var profileResult *ProfileResult
	profileWarning := ""
	if !opts.SkipProfile && len(listMembers) > 0 {
		profileResult, err = s.downloadProfile(ctx, taskID, listMembers, pathHelper, versionManager, fileWriter, dwn, reporter)
		if err != nil {
			log.Warnf("Profile download failed for list %d: %v", listID, err)
			reporter.OnProgress(taskID, Progress{Stage: "profile_warning", Current: fmt.Sprintf("profile failed for list %d: %v", listID, err)})
			profileWarning = "with profile warnings"
		}
	}

	s.completeTask(taskID, reporter, "List download completed", &Result{
		Main:    s.buildMainDownloadResult(summary, countRemainingFailedEntities(dumper, mainFailures)),
		Profile: cloneProfileResult(profileResult),
	}, profileWarning)
	return nil
}

// FollowingDownload 下载关注列表
func (s *downloadServiceImpl) FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

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
		database.MarkUserInaccessible(s.deps.DB, uid, screenName)
		return err
	}
	following := user.Following()

	// 初始化下载器
	versionManager, fileWriter, dwn := s.initDownloader()

	progress := s.newBatchProgressCallback(taskID, reporter)
	retryProgress := s.newRetryProgressCallback(taskID, reporter)

	// 执行批量下载 - 将 Following 作为 List 传递给 BatchDownloadAny
	// 这样可以通过 syncListAndGetMembers 同步列表成员，并返回完整 User 对象
	failedTweets, listMembers, summary, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, []twitter.ListBase{following}, nil, pathHelper.Root, pathHelper.Users, effectiveAutoFollow(opts), s.deps.AdditionalClients, dwn, fileWriter, progress)
	if err != nil {
		return err
	}
	if opts.FollowMembers {
		if err := s.followMembersIfNeeded(ctx, listMembers); err != nil {
			return err
		}
	}
	mainFailures := collectFailedTweetSet(failedTweets)

	// 收集失败的推文到 Dumper
	s.collectFailedTweets(dumper, failedTweets)

	// 重试失败的推文
	if !opts.NoRetry {
		if _, err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter, retryProgress); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载（复用 BatchDownloadAny 返回的 listMembers）
	var profileResult *ProfileResult
	profileWarning := ""
	if !opts.SkipProfile && len(listMembers) > 0 {
		profileResult, err = s.downloadProfile(ctx, taskID, listMembers, pathHelper, versionManager, fileWriter, dwn, reporter)
		if err != nil {
			log.Warnf("Profile download failed for following %s: %v", screenName, err)
			reporter.OnProgress(taskID, Progress{Stage: "profile_warning", Current: fmt.Sprintf("profile failed for following %s: %v", screenName, err)})
			profileWarning = "with profile warnings"
		}
	}

	s.completeTask(taskID, reporter, "Following download completed", &Result{
		Main:    s.buildMainDownloadResult(summary, countRemainingFailedEntities(dumper, mainFailures)),
		Profile: cloneProfileResult(profileResult),
	}, profileWarning)
	return nil
}

// ProfileDownload 下载用户资料
func (s *downloadServiceImpl) ProfileDownload(ctx context.Context, taskID string, screenNames []string, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	versionManager, fileWriter, dwn := s.initDownloader()

	unique := make([]string, 0)
	seen := make(map[string]struct{})
	for _, name := range screenNames {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			unique = append(unique, name)
		}
	}
	users := s.resolveUsers(ctx, unique)
	if len(unique) > 0 && len(users) == 0 {
		return fmt.Errorf("all profile users failed to resolve")
	}

	profileResult, err := s.downloadProfile(ctx, taskID, users, pathHelper, versionManager, fileWriter, dwn, reporter)
	if err != nil {
		return err
	}

	s.completeProfileTask(taskID, reporter, profileResult)
	return nil
}

// ListProfileDownload 下载列表用户资料
func (s *downloadServiceImpl) ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

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

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	versionManager, fileWriter, dwn := s.initDownloader()

	users := make([]*twitter.User, 0, len(membersResult.Users))
	seen := make(map[string]struct{})
	for _, user := range membersResult.Users {
		if _, ok := seen[user.ScreenName]; !ok {
			seen[user.ScreenName] = struct{}{}
			users = append(users, user)
		}
	}

	profileResult, err := s.downloadProfile(ctx, taskID, users, pathHelper, versionManager, fileWriter, dwn, reporter)
	if err != nil {
		return err
	}

	s.completeProfileTask(taskID, reporter, profileResult)

	return nil
}

// MarkDownloaded 标记已下载
func (s *downloadServiceImpl) MarkDownloaded(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, markTime *string, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

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
	_ = noRetry // 显式标记为有意忽略：第三方 JSON 文件下载不涉及 TweetDumper 机制，无需重试逻辑
	reporter = s.getReporterOrDefault(reporter)

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths), Current: fmt.Sprintf("%d JSON files", len(paths))})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	_, fileWriter, dwn := s.initDownloader()

	// 使用 pathHelper.Users 确保导入 JSON 与常规推文下载落在同一 users 目录结构下
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
		Main: &MainResult{
			Downloaded: successCount,
			Failed:     failCount,
		},
		Message: fmt.Sprintf("JSON file download: %d success, %d failed, %d media", successCount, failCount, totalMedia),
	})
	return nil
}

// JsonFolderDownload 从TMD生成的.loongtweet文件夹下载推文媒体
// 注意：noRetry 参数设计如此，loongtweet 文件夹下载不涉及 TweetDumper 机制，
// 失败项不会进入 error.json，因此无需重试逻辑
func (s *downloadServiceImpl) JsonFolderDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error {
	_ = noRetry // 显式标记为有意忽略：loongtweet 文件夹下载不涉及 TweetDumper 机制，无需重试逻辑
	reporter = s.getReporterOrDefault(reporter)

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths), Current: fmt.Sprintf("%d loongtweet folders", len(paths))})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	_, fileWriter, dwn := s.initDownloader()

	// 使用 pathHelper.Users 确保 .loongtweet 导入与常规推文下载落在同一 users 目录结构下
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
		Main: &MainResult{
			Downloaded: successCount,
			Failed:     failCount,
		},
		Message: fmt.Sprintf("JSON folder download: %d success, %d failed", successCount, failCount),
	})
	return nil
}

// BatchDownload 批量下载
func (s *downloadServiceImpl) BatchDownload(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, opts DownloadOptions, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

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

	reporter.OnProgress(taskID, Progress{Stage: "downloading"})

	// 初始化下载器
	versionManager, fileWriter, dwn := s.initDownloader()
	progress := s.newBatchProgressCallback(taskID, reporter)
	retryProgress := s.newRetryProgressCallback(taskID, reporter)

	// 执行批量下载（返回列表成员用于 Profile 下载）
	failedTweets, listMembers, summary, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, lists, users, pathHelper.Root, pathHelper.Users, effectiveAutoFollow(opts), s.deps.AdditionalClients, dwn, fileWriter, progress)
	if err != nil {
		return err
	}
	if opts.FollowMembers {
		followTargets := make([]*twitter.User, 0, len(users)+len(listMembers))
		followTargets = append(followTargets, users...)
		followTargets = append(followTargets, listMembers...)
		if err := s.followMembersIfNeeded(ctx, followTargets); err != nil {
			return err
		}
	}
	mainFailures := collectFailedTweetSet(failedTweets)

	// 收集失败的推文到 Dumper
	s.collectFailedTweets(dumper, failedTweets)

	// 重试失败的推文
	if !opts.NoRetry {
		if _, err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter, retryProgress); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载（复用 BatchDownloadAny 返回的 listMembers，避免重复 API 调用）
	var profileResult *ProfileResult
	profileWarning := ""
	if !opts.SkipProfile && (len(users) > 0 || len(listMembers) > 0) {
		profileUsers := make([]*twitter.User, 0)
		seen := make(map[string]struct{}) // 使用 screenName 去重（与稳定版一致）

		// 添加直接传入的用户
		for _, user := range users {
			if _, ok := seen[user.ScreenName]; !ok {
				seen[user.ScreenName] = struct{}{}
				profileUsers = append(profileUsers, user)
			}
		}

		// 复用 BatchDownloadAny 返回的列表成员（无需再次调用 GetMembers）
		for _, member := range listMembers {
			if _, ok := seen[member.ScreenName]; !ok {
				seen[member.ScreenName] = struct{}{}
				profileUsers = append(profileUsers, member)
			}
		}

		if len(profileUsers) > 0 {
			profileResult, err = s.downloadProfile(ctx, taskID, profileUsers, pathHelper, versionManager, fileWriter, dwn, reporter)
			if err != nil {
				log.Warnf("Profile download failed for batch: %v", err)
				reporter.OnProgress(taskID, Progress{Stage: "profile_warning", Current: fmt.Sprintf("profile failed for batch: %v", err)})
				profileWarning = "with profile warnings"
			}
		}
	}

	s.completeTask(taskID, reporter, "Batch download completed", &Result{
		Main:    s.buildMainDownloadResult(summary, countRemainingFailedEntities(dumper, mainFailures)),
		Profile: cloneProfileResult(profileResult),
	}, profileWarning)
	return nil
}

// saveDumper 保存 Dumper 到文件
func (s *downloadServiceImpl) saveDumper(dumper *downloading.TweetDumper, path string) {
	s.dumperMu.Lock()
	defer s.dumperMu.Unlock()

	merged := downloading.NewDumper()
	if err := merged.Load(path); err != nil {
		log.Warnf("Failed to load dumper for merge: %v", err)
	}
	merged.Merge(dumper)

	if merged.Count() > 0 {
		if err := merged.Dump(path); err != nil {
			log.Warnf("Failed to save dumper: %v", err)
		} else {
			log.Infof("%d tweets have been dumped", merged.Count())
		}
		return
	}
	_ = os.Remove(path)
}

// collectFailedTweets 收集失败的推文到 Dumper
func (s *downloadServiceImpl) collectFailedTweets(dumper *downloading.TweetDumper, failedTweets []*downloading.TweetInEntity) {
	for _, tweet := range failedTweets {
		if tweet == nil || tweet.Tweet == nil || tweet.Entity == nil {
			continue
		}
		if id, err := tweet.Entity.Id(); err == nil {
			dumper.Push(id, tweet.Tweet)
		}
	}
}

// 内部辅助方法：下载 Profile
func (s *downloadServiceImpl) downloadProfile(ctx context.Context, taskID string, users []*twitter.User, pathHelper *path.StorePath, versionManager downloader.VersionManager, fileWriter downloader.FileWriter, dwn downloader.Downloader, reporter ProgressReporter) (*ProfileResult, error) {
	users = dedupeProfileUsers(users)
	if len(users) == 0 {
		return nil, nil
	}

	reporter.OnProgress(taskID, Progress{Stage: "profile", Total: len(users), Current: users[0].ScreenName})

	// 创建 storage manager
	storage, err := profile.NewFileStorageManager(pathHelper.Users)
	if err != nil {
		log.Errorf("Failed to create file storage manager: %v", err)
		return nil, fmt.Errorf("failed to create profile storage: %w", err)
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
	pd.SetProgressCallback(func(progress profile.DownloadProgress) {
		reporter.OnProgress(taskID, Progress{
			Stage:     "profile",
			Total:     progress.Total,
			Completed: progress.Completed,
			Failed:    progress.Failed,
			Current:   progress.Current,
		})
	})

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
	var firstErr error
	for _, result := range results {
		if result == nil {
			failCount++
			if firstErr == nil {
				firstErr = fmt.Errorf("profile download returned nil result")
			}
			continue
		}
		if result.Error != nil {
			failCount++
			if firstErr == nil {
				firstErr = result.Error
			}
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

	profileResult := &ProfileResult{
		Downloaded: successCount,
		Failed:     failCount,
		Versioned:  versionedFileCount,
	}

	if successCount == 0 && failCount > 0 {
		if firstErr == nil {
			firstErr = fmt.Errorf("unknown profile download error")
		}
		return profileResult, fmt.Errorf("profile download failed for all %d users: %w", failCount, firstErr)
	}

	return profileResult, nil
}

func formatProfileCompletionMessage(result ProfileResult) string {
	return fmt.Sprintf(
		"Profile download completed: %d success, %d failed, %d versioned files",
		result.Downloaded,
		result.Failed,
		result.Versioned,
	)
}

func cloneProfileResult(result *ProfileResult) *ProfileResult {
	if result == nil {
		return nil
	}
	clone := *result
	return &clone
}
