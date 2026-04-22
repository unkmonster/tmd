package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/profile"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

// Dependencies 执行依赖
type Dependencies struct {
	Client            *resty.Client
	AdditionalClients []*resty.Client
	DB                *sqlx.DB
	Conf              *config.Config
	AppRootPath       string
}

// Execute 执行 CLI 命令
func Execute(ctx context.Context, args []string, deps *Dependencies) error {
	// 解析参数
	_, cfg, err := ParseArgs(args)
	if err != nil {
		return fmt.Errorf("parse args failed: %w", err)
	}

	// 获取存储路径
	pathHelper, err := NewStorePath(deps.Conf.RootPath)
	if err != nil {
		return fmt.Errorf("failed to make store dir: %w", err)
	}

	// 初始化下载器
	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 初始化 Dumper
	dumper := downloading.NewDumper()
	_ = dumper.Load(pathHelper.ErrorJ)

	// 创建任务
	task, err := MakeTask(ctx, deps.Client, deps.DB, cfg.UsrArgs, cfg.ListArgs, cfg.FollArgs)
	if err != nil {
		return fmt.Errorf("failed to make task: %w", err)
	}

	// 保存 Dumper 的 defer（需要在函数返回前执行）
	defer func() {
		if dumper.Count() > 0 {
			dumper.Dump(pathHelper.ErrorJ)
			log.Infof("%d tweets have been dumped", dumper.Count())
		}
	}()

	// 检查是否有下载任务
	if len(task.Users) == 0 && len(task.Lists) == 0 && len(cfg.JsonArgs.GetPaths()) == 0 {
		// 仅处理 profile
		return handleProfileOnly(ctx, cfg, deps, pathHelper, versionManager, fileWriter, dwn)
	}

	log.Infoln("start working for...")
	PrintTask(task)

	// 执行下载
	if cfg.MarkDownloaded {
		return executeMarkDownloaded(ctx, cfg, deps, task, pathHelper)
	}

	if len(cfg.JsonArgs.GetPaths()) > 0 {
		return executeJsonDownload(ctx, cfg, deps, pathHelper, dwn, fileWriter)
	}

	return executeBatchDownload(ctx, cfg, deps, task, pathHelper, dwn, fileWriter, versionManager, dumper)
}

func executeBatchDownload(ctx context.Context, cfg *CLIConfig, deps *Dependencies, task *Task, pathHelper *StorePath, dwn downloader.Downloader, fileWriter downloader.FileWriter, versionManager downloader.VersionManager, dumper *downloading.TweetDumper) error {
	// 执行批量下载
	failed, err := downloading.BatchDownloadAny(
		ctx, deps.Client, deps.DB,
		task.Lists, task.Users,
		pathHelper.Root, pathHelper.Users,
		cfg.AutoFollow, deps.AdditionalClients,
		dwn, fileWriter,
	)

	if err != nil {
		return err
	}

	// 保存失败推文
	for _, f := range failed {
		eid, err := f.Entity.Id()
		if err != nil {
			log.Warnln("failed to get entity id:", err)
			continue
		}
		dumper.Push(eid, f.Tweet)
	}

	// 下载 Profile
	if !cfg.NoProfile {
		handleProfileDownload(ctx, cfg, deps, task, pathHelper, dwn, fileWriter, versionManager)
	}

	// 重试失败的
	if !cfg.NoRetry {
		downloading.RetryFailedTweets(ctx, dumper, deps.DB, deps.Client, dwn, fileWriter)
	}

	return nil
}

func executeMarkDownloaded(ctx context.Context, cfg *CLIConfig, deps *Dependencies, task *Task, pathHelper *StorePath) error {
	results, err := downloading.MarkUsersAsDownloaded(ctx, deps.Client, deps.DB, task.Lists, task.Users, pathHelper.Users, cfg.MarkTime)
	if err != nil {
		return fmt.Errorf("failed to mark users as downloaded: %w", err)
	}
	if len(results) > 0 {
		fmt.Println("\n=== MARK_DOWNLOADED_RESULTS ===")
		for _, r := range results {
			status := "OK"
			if !r.Success {
				status = "FAIL"
			}
			fmt.Printf("ENTITY_ID:%d|USER_ID:%d|SCREEN_NAME:%s|STATUS:%s\n", r.EntityID, r.UserID, r.ScreenName, status)
		}
		fmt.Println("=== END_RESULTS ===")
	}
	return nil
}

func executeJsonDownload(ctx context.Context, cfg *CLIConfig, deps *Dependencies, pathHelper *StorePath, dwn downloader.Downloader, fileWriter downloader.FileWriter) error {
	log.Infof("downloading from %d JSON file(s)...", len(cfg.JsonArgs.GetPaths()))
	results := downloading.DownloadJsonDir(ctx, deps.Client, pathHelper.Root, dwn, fileWriter, cfg.JsonArgs.GetPaths()...)
	var successCount, failCount int
	for _, r := range results {
		if r.Success {
			successCount++
			log.Infof("✓ %s: %d tweets processed in %v", filepath.Base(r.Path), r.TweetCount, r.Duration)
		} else {
			failCount++
			log.Errorf("✗ %s: %v", filepath.Base(r.Path), r.Error)
		}
	}
	log.Infof("JSON download completed: %d success, %d failed", successCount, failCount)
	return nil
}

func handleProfileOnly(ctx context.Context, cfg *CLIConfig, deps *Dependencies, pathHelper *StorePath, versionManager downloader.VersionManager, fileWriter downloader.FileWriter, dwn downloader.Downloader) error {
	shouldDownloadProfile := len(cfg.ProfileUsers.ScreenName) > 0 || len(cfg.ProfileList.ID) > 0
	if !shouldDownloadProfile {
		return nil
	}

	profileCtx, profileCancel := context.WithCancel(ctx)
	defer profileCancel()

	task := &Task{
		Users: make([]*twitter.User, 0),
		Lists: make([]twitter.ListBase, 0),
	}

	handleProfileDownload(profileCtx, cfg, deps, task, pathHelper, dwn, fileWriter, versionManager)
	return nil
}

func handleProfileDownload(ctx context.Context, cfg *CLIConfig, deps *Dependencies, task *Task, pathHelper *StorePath, dwn downloader.Downloader, fileWriter downloader.FileWriter, versionManager downloader.VersionManager) {
	clients := make([]*resty.Client, 0)
	clients = append(clients, deps.Client)
	clients = append(clients, deps.AdditionalClients...)

	storage, err := profile.NewFileStorageManager(pathHelper.Users)
	if err != nil {
		log.Fatalln("failed to create profile storage:", err)
	}
	storage.SetVersionManager(versionManager)

	profileDownloader := profile.NewProfileDownloaderWithDB(nil, storage, clients, deps.DB, dwn, fileWriter)

	requests := make([]profile.DownloadRequest, 0)

	if len(task.Users) > 0 {
		for _, user := range task.Users {
			req := profile.DownloadRequest{
				ScreenName: user.ScreenName,
				UserTitle:  user.Title(),
				Name:       user.Name,
				UserID:     user.Id,
			}
			if !cfg.NoProfile {
				req.AvatarURL = user.AvatarURL
				req.BannerURL = user.BannerURL
				req.Description = user.Description
				req.Location = user.Location
				req.URL = user.URL
				req.Verified = user.Verified
				req.Protected = user.IsProtected
				req.CreatedAt = user.CreatedAt
			}
			requests = append(requests, req)
		}
	}

	for _, screenName := range cfg.ProfileUsers.ScreenName {
		requests = append(requests, profile.DownloadRequest{
			ScreenName: screenName,
			UserTitle:  "",
			Name:       "",
			UserID:     0,
		})
	}

	if len(cfg.ProfileList.ID) > 0 {
		lists, err := cfg.ProfileList.GetList(ctx, deps.Client)
		if err != nil {
			log.WithError(err).Errorln("failed to get profile lists")
		} else {
			for _, lst := range lists {
				appendListMemberRequests(ctx, deps.Client, deps.DB, lst, &requests)
			}
		}
	}

	if len(task.Lists) > 0 {
		for _, lst := range task.Lists {
			appendListMemberRequests(ctx, deps.Client, deps.DB, lst, &requests)
		}
	}

	seen := make(map[string]bool)
	uniqueRequests := make([]profile.DownloadRequest, 0)
	for _, req := range requests {
		if !seen[req.ScreenName] {
			seen[req.ScreenName] = true
			uniqueRequests = append(uniqueRequests, req)
		}
	}

	if len(uniqueRequests) == 0 {
		log.Infoln("no users to download profile")
		return
	}

	log.Infoln("starting profile download for", len(uniqueRequests), "users")

	results := profileDownloader.DownloadMultiple(ctx, uniqueRequests)

	success := 0
	failed := 0
	skipped := 0
	for _, r := range results {
		if r.Success {
			success++
		} else if r.Error != nil {
			failed++
		} else {
			skipped++
		}
	}

	log.Infoln("profile download completed - total:", len(results), "success:", success, "failed:", failed, "skipped:", skipped)

	fmt.Println("\n=== PROFILE_DOWNLOAD_RESULTS ===")
	for _, r := range results {
		if !r.Success {
			status := "SKIP"
			if r.Error != nil {
				status = "FAIL"
			}
			fmt.Printf("SCREEN_NAME:%s|STATUS:%s\n", r.ScreenName, status)
		}
	}
	fmt.Println("=== END_RESULTS ===")
}

func appendListMemberRequests(ctx context.Context, client *resty.Client, db *sqlx.DB, lst twitter.ListBase, requests *[]profile.DownloadRequest) {
	membersResult, err := lst.GetMembers(ctx, client)
	if err != nil {
		log.WithError(err).WithField("list", lst.Title()).Errorln("failed to get list members")
		return
	}

	uids := utils.ExtractIDs(membersResult.Users, func(u *twitter.User) uint64 { return u.Id })
	database.MarkListMembersAccessibleByIDs(db, uids)

	for _, member := range membersResult.Users {
		*requests = append(*requests, profile.DownloadRequest{
			ScreenName:  member.ScreenName,
			UserTitle:   member.Title(),
			Name:        member.Name,
			UserID:      member.Id,
			AvatarURL:   member.AvatarURL,
			BannerURL:   member.BannerURL,
			Description: member.Description,
			Location:    member.Location,
			URL:         member.URL,
			Verified:    member.Verified,
			Protected:   member.IsProtected,
			CreatedAt:   member.CreatedAt,
		})
	}
}

// SetClientLogger 设置客户端日志
func SetClientLogger(client *resty.Client, out io.Writer) {
	logger := log.New()
	logger.SetLevel(log.InfoLevel)
	logger.SetOutput(out)
	logger.SetFormatter(&log.TextFormatter{
		FullTimestamp:  true,
		DisableQuote:   true,
		DisableSorting: true,
		PadLevelText:   false,
	})
	client.SetLogger(logger)
}
