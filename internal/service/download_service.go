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

	"github.com/unkmonster/tmd/internal/config"
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

func (s *downloadServiceImpl) maxDownloadRoutine() int {
	if s.deps != nil && s.deps.Config != nil && s.deps.Config.MaxDownloadRoutine > 0 {
		return s.deps.Config.MaxDownloadRoutine
	}
	return config.DefaultMaxDownloadRoutine()
}

func (s *downloadServiceImpl) runtimeOptions() downloading.RuntimeOptions {
	return downloading.RuntimeOptions{
		MaxDownloadRoutine: s.maxDownloadRoutine(),
	}
}

func (s *downloadServiceImpl) profileDownloaderConfig() *profile.Config {
	cfg := profile.DefaultConfig()
	cfg.MaxDownloadRoutine = s.maxDownloadRoutine()
	return cfg
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

	seenByScreenName := make(map[string]struct{}, len(users))
	seenByID := make(map[uint64]struct{}, len(users))
	seenByPointer := make(map[*twitter.User]struct{}, len(users))
	deduped := make([]*twitter.User, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		screenName := strings.ToLower(strings.TrimSpace(user.ScreenName))
		if screenName != "" {
			if _, ok := seenByScreenName[screenName]; ok {
				continue
			}
		}
		if user.Id != 0 {
			if _, ok := seenByID[user.Id]; ok {
				continue
			}
		}
		if screenName == "" && user.Id == 0 {
			if _, ok := seenByPointer[user]; ok {
				continue
			}
			seenByPointer[user] = struct{}{}
		}
		if screenName != "" {
			seenByScreenName[screenName] = struct{}{}
		}
		if user.Id != 0 {
			seenByID[user.Id] = struct{}{}
		}
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

// downloadTemplateConfig 封装下载流程模板方法的差异点配置
type downloadTemplateConfig struct {
	TaskID   string
	Opts     DownloadOptions
	Reporter ProgressReporter

	Prepare func(ctx context.Context, pathHelper *path.StorePath) (
		users []*twitter.User,
		lists []twitter.ListBase,
		explicitProfileUsers []*twitter.User,
		err error,
	)

	ReportBeforeDownload  func(taskID string, reporter ProgressReporter)
	ShouldDownloadProfile func(users []*twitter.User) bool
	ProfileIdentifier     string
	CompletionMessage     string
}

func (s *downloadServiceImpl) executeDownloadTemplate(ctx context.Context, config downloadTemplateConfig) error {
	reporter := s.getReporterOrDefault(config.Reporter)

	config.ReportBeforeDownload(config.TaskID, reporter)

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return fmt.Errorf("failed to make store dir: %w", err)
	}

	dumper := downloading.NewDumper()
	if err := dumper.Load(pathHelper.ErrorJ); err != nil {
		log.Warnf("Failed to load dumper: %v", err)
	}
	defer s.saveDumper(dumper, pathHelper.ErrorJ)

	users, lists, explicitProfileUsers, err := config.Prepare(ctx, pathHelper)
	if err != nil {
		return err
	}

	versionManager, fileWriter, dwn := s.initDownloader()
	progress := s.newBatchProgressCallback(config.TaskID, reporter)
	retryProgress := s.newRetryProgressCallback(config.TaskID, reporter)
	runtimeOptions := s.runtimeOptions()

	failedTweets, listMembers, summary, err := downloading.BatchDownloadAny(
		ctx, s.deps.Client, s.deps.DB, lists, users,
		pathHelper.Root, pathHelper.Users, effectiveAutoFollow(config.Opts),
		s.deps.AdditionalClients, dwn, fileWriter, runtimeOptions, progress,
	)
	if err != nil {
		return err
	}

	if config.Opts.FollowMembers {
		followTargets := make([]*twitter.User, 0, len(users)+len(listMembers))
		followTargets = append(followTargets, users...)
		followTargets = append(followTargets, listMembers...)
		if err := s.followMembersIfNeeded(ctx, followTargets); err != nil {
			return err
		}
	}
	mainFailures := collectFailedTweetSet(failedTweets)

	s.collectFailedTweets(dumper, failedTweets)
	if !config.Opts.NoRetry {
		if _, err := downloading.RetryFailedTweets(
			ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter, runtimeOptions, retryProgress,
		); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	var profileResult *ProfileResult
	profileWarning := ""

	var profileTargetUsers []*twitter.User
	if explicitProfileUsers != nil {
		profileTargetUsers = explicitProfileUsers
	} else {
		profileTargetUsers = listMembers
	}

	if config.ShouldDownloadProfile(profileTargetUsers) && len(profileTargetUsers) > 0 {
		profileResult, err = s.downloadProfile(
			ctx, config.TaskID, profileTargetUsers,
			pathHelper, versionManager, fileWriter, dwn, reporter,
		)
		if err != nil {
			log.Warnf("Profile download failed for %s: %v", config.ProfileIdentifier, err)
			reporter.OnProgress(config.TaskID, Progress{
				Stage:   "profile_warning",
				Current: fmt.Sprintf("profile failed for %s: %v", config.ProfileIdentifier, err),
			})
			profileWarning = "with profile warnings"
		}
	}

	s.completeTask(config.TaskID, reporter, config.CompletionMessage, &Result{
		Main:    s.buildMainDownloadResult(summary, countRemainingFailedEntities(dumper, mainFailures)),
		Profile: cloneProfileResult(profileResult),
	}, profileWarning)

	return nil
}

// UserDownload 下载用户推文
func (s *downloadServiceImpl) UserDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error {
	return s.executeDownloadTemplate(ctx, downloadTemplateConfig{
		TaskID:            taskID,
		Opts:              opts,
		Reporter:          reporter,
		ProfileIdentifier: screenName,
		CompletionMessage: "User download completed",

		ReportBeforeDownload: func(tid string, r ProgressReporter) {
			r.OnProgress(tid, Progress{Stage: "downloading", Current: screenName})
		},

		Prepare: func(ctx context.Context, ph *path.StorePath) ([]*twitter.User, []twitter.ListBase, []*twitter.User, error) {
			user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
			if err != nil {
				database.MarkUserInaccessible(s.deps.DB, uid, screenName)
				return nil, nil, nil, err
			}
			return []*twitter.User{user}, nil, []*twitter.User{user}, nil
		},

		ShouldDownloadProfile: func(_ []*twitter.User) bool {
			return !opts.SkipProfile
		},
	})
}

// ListDownload 下载列表推文
func (s *downloadServiceImpl) ListDownload(ctx context.Context, taskID string, listID uint64, opts DownloadOptions, reporter ProgressReporter) error {
	return s.executeDownloadTemplate(ctx, downloadTemplateConfig{
		TaskID:            taskID,
		Opts:              opts,
		Reporter:          reporter,
		ProfileIdentifier: fmt.Sprintf("list %d", listID),
		CompletionMessage: "List download completed",

		ReportBeforeDownload: func(tid string, r ProgressReporter) {
			r.OnProgress(tid, Progress{Stage: "syncing", Current: fmt.Sprintf("list:%d", listID)})
			r.OnProgress(tid, Progress{Stage: "downloading", Current: fmt.Sprintf("list:%d", listID)})
		},

		Prepare: func(ctx context.Context, ph *path.StorePath) ([]*twitter.User, []twitter.ListBase, []*twitter.User, error) {
			list, err := twitter.GetLst(ctx, s.deps.Client, listID)
			if err != nil {
				return nil, nil, nil, err
			}
			return nil, []twitter.ListBase{list}, nil, nil
		},

		ShouldDownloadProfile: func(_ []*twitter.User) bool {
			return !opts.SkipProfile
		},
	})
}

// FollowingDownload 下载关注列表
func (s *downloadServiceImpl) FollowingDownload(ctx context.Context, taskID string, screenName string, opts DownloadOptions, reporter ProgressReporter) error {
	return s.executeDownloadTemplate(ctx, downloadTemplateConfig{
		TaskID:            taskID,
		Opts:              opts,
		Reporter:          reporter,
		ProfileIdentifier: fmt.Sprintf("following %s", screenName),
		CompletionMessage: "Following download completed",

		ReportBeforeDownload: func(tid string, r ProgressReporter) {
			r.OnProgress(tid, Progress{Stage: "downloading", Current: screenName})
		},

		Prepare: func(ctx context.Context, ph *path.StorePath) ([]*twitter.User, []twitter.ListBase, []*twitter.User, error) {
			user, uid, err := twitter.GetUserByScreenName(ctx, s.deps.Client, screenName)
			if err != nil {
				database.MarkUserInaccessible(s.deps.DB, uid, screenName)
				return nil, nil, nil, err
			}
			return nil, []twitter.ListBase{user.Following()}, nil, nil
		},

		ShouldDownloadProfile: func(_ []*twitter.User) bool {
			return !opts.SkipProfile
		},
	})
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

	users := dedupeProfileUsers(membersResult.Users)

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
// noRetry=true 时跳过重试，失败项仍会持久化到 json_errors.json 供下次运行使用
func (s *downloadServiceImpl) JsonFileDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths), Current: fmt.Sprintf("%d JSON files", len(paths))})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	_, fileWriter, dwn := s.initDownloader()

	jsonDumper := downloading.NewJsonDumper()
	if err := jsonDumper.Load(pathHelper.JsonErrorJ); err != nil {
		log.Warnf("Failed to load JSON dumper: %v", err)
	}
	defer s.saveJsonDumper(jsonDumper, pathHelper.JsonErrorJ)

	retryProgress := s.newRetryProgressCallback(taskID, reporter)

	runtimeOptions := s.runtimeOptions()
	results, failedBySource := downloading.DownloadThirdPartyTweets(ctx, s.deps.Client, pathHelper.Users, dwn, fileWriter, runtimeOptions, paths...)

	s.collectJsonFailedTweets(jsonDumper, failedBySource, "file")

	if !noRetry {
		if _, err := downloading.RetryFailedJsonTweets(ctx, jsonDumper, s.deps.Client, dwn, fileWriter, runtimeOptions, retryProgress); err != nil {
			log.Warnf("Retry failed JSON tweets error: %v", err)
		}
	}

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
// noRetry=true 时跳过重试，失败项仍会持久化到 json_errors.json 供下次运行使用
func (s *downloadServiceImpl) JsonFolderDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter ProgressReporter) error {
	reporter = s.getReporterOrDefault(reporter)

	reporter.OnProgress(taskID, Progress{Stage: "downloading", Total: len(paths), Current: fmt.Sprintf("%d loongtweet folders", len(paths))})

	pathHelper, err := path.NewStorePath(s.deps.Config.RootPath)
	if err != nil {
		return err
	}

	_, fileWriter, dwn := s.initDownloader()

	jsonDumper := downloading.NewJsonDumper()
	if err := jsonDumper.Load(pathHelper.JsonErrorJ); err != nil {
		log.Warnf("Failed to load JSON dumper: %v", err)
	}
	defer s.saveJsonDumper(jsonDumper, pathHelper.JsonErrorJ)

	retryProgress := s.newRetryProgressCallback(taskID, reporter)

	runtimeOptions := s.runtimeOptions()
	results, failedBySource := downloading.DownloadFromLoongTweetFolder(ctx, s.deps.Client, pathHelper.Users, dwn, fileWriter, runtimeOptions, paths...)

	s.collectJsonFailedTweets(jsonDumper, failedBySource, "folder")

	if !noRetry {
		if _, err := downloading.RetryFailedJsonTweets(ctx, jsonDumper, s.deps.Client, dwn, fileWriter, runtimeOptions, retryProgress); err != nil {
			log.Warnf("Retry failed JSON tweets error: %v", err)
		}
	}

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
	runtimeOptions := s.runtimeOptions()

	// 执行批量下载（返回列表成员用于 Profile 下载）
	failedTweets, listMembers, summary, err := downloading.BatchDownloadAny(ctx, s.deps.Client, s.deps.DB, lists, users, pathHelper.Root, pathHelper.Users, effectiveAutoFollow(opts), s.deps.AdditionalClients, dwn, fileWriter, runtimeOptions, progress)
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
		if _, err := downloading.RetryFailedTweets(ctx, dumper, s.deps.DB, s.deps.Client, dwn, fileWriter, runtimeOptions, retryProgress); err != nil {
			log.Warnf("Retry failed tweets error: %v", err)
		}
	}

	// Profile 下载（复用 BatchDownloadAny 返回的 listMembers，避免重复 API 调用）
	var profileResult *ProfileResult
	profileWarning := ""
	if !opts.SkipProfile && (len(users) > 0 || len(listMembers) > 0) {
		profileUsers := make([]*twitter.User, 0, len(users)+len(listMembers))
		profileUsers = append(profileUsers, users...)
		profileUsers = append(profileUsers, listMembers...)
		profileUsers = dedupeProfileUsers(profileUsers)

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

// collectJsonFailedTweets 收集 JSON 下载失败的推文到 JsonTweetDumper（保留 dir 用于重试时定位用户目录）
func (s *downloadServiceImpl) collectJsonFailedTweets(dumper *downloading.JsonTweetDumper, failedBySource map[string][]downloading.JsonPackagedTweet, entryType string) {
	for sourcePath, items := range failedBySource {
		for _, item := range items {
			dumper.PushWithDir(sourcePath, entryType, item.Dir, item.Tweet)
		}
	}
}

// saveJsonDumper 保存 JsonTweetDumper 到文件（Load-then-Merge 模式，与 saveDumper 一致）
func (s *downloadServiceImpl) saveJsonDumper(dumper *downloading.JsonTweetDumper, path string) {
	s.dumperMu.Lock()
	defer s.dumperMu.Unlock()

	merged := downloading.NewJsonDumper()
	if err := merged.Load(path); err != nil {
		log.Warnf("Failed to load JSON dumper for merge: %v", err)
	}
	merged.Merge(dumper)

	if merged.Count() > 0 {
		if err := merged.Dump(path); err != nil {
			log.Warnf("Failed to save JSON dumper: %v", err)
		} else {
			log.Infof("%d JSON tweets have been dumped", merged.Count())
		}
		return
	}
	_ = os.Remove(path)
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
		s.profileDownloaderConfig(),
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
