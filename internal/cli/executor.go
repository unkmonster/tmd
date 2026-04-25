package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/service"
	"github.com/unkmonster/tmd/internal/twitter"
)

// Dependencies 执行依赖
type Dependencies struct {
	Client            *resty.Client
	AdditionalClients []*resty.Client
	DB                *sqlx.DB
	Conf              *config.Config
	AppRootPath       string
	DownloadService   service.DownloadService // 新增：Service 层
}

// Execute 执行 CLI 命令
func Execute(ctx context.Context, args []string, deps *Dependencies) error {
	// 解析参数
	_, cfg, err := ParseArgs(args)
	if err != nil {
		return fmt.Errorf("parse args failed: %w", err)
	}

	// 如果没有提供 Service，创建一个默认的
	if deps.DownloadService == nil {
		deps.DownloadService = service.NewDownloadService(&service.Dependencies{
			Client:            deps.Client,
			AdditionalClients: deps.AdditionalClients,
			DB:                deps.DB,
			Config:            deps.Conf,
			AppRootPath:       deps.AppRootPath,
		})
	}

	// 创建进度报告器
	reporter := service.NewLogReporter(log.Infof)

	// 构建下载选项
	opts := service.DownloadOptions{
		AutoFollow:  cfg.AutoFollow,
		SkipProfile: cfg.NoProfile,
		NoRetry:     cfg.NoRetry,
	}
	if cfg.MarkTime != "" {
		opts.MarkTime = &cfg.MarkTime
	}

	// 处理不同类型的下载任务

	log.Infoln("start working for...")

	// 1. JSON 下载
	if len(cfg.JsonArgs.GetPaths()) > 0 {
		log.Infof("json files: %d", len(cfg.JsonArgs.GetPaths()))
		return deps.DownloadService.JsonDownload(ctx, "cli", cfg.JsonArgs.GetPaths(), cfg.NoRetry, reporter)
	}

	// 2. 标记已下载
	if cfg.MarkDownloaded {
		entities := ResolveUsersAndLists(ctx, deps.Client, deps.DB, cfg.UsrArgs, cfg.ListArgs, cfg.FollArgs)
		log.Infof("mark downloaded: users: %d, lists: %d", len(entities.Users), len(entities.Lists))
		return deps.DownloadService.MarkDownloaded(ctx, "cli", entities.Users, entities.Lists, opts.MarkTime, reporter)
	}

	// 3. 批量下载（包含用户、列表、关注）
	// 注意：批量下载和 Profile 下载可以同时执行（-user + -profile-user）
	entities := ResolveUsersAndLists(ctx, deps.Client, deps.DB, cfg.UsrArgs, cfg.ListArgs, cfg.FollArgs)
	hasBatchDownload := len(entities.Users) > 0 || len(entities.Lists) > 0

	if hasBatchDownload {
		log.Infof("users: %d, lists: %d", len(entities.Users), len(entities.Lists))
		for _, u := range entities.Users {
			log.Infof("    - %s", u.Title())
		}
		for _, l := range entities.Lists {
			log.Infof("    - %s", l.Title())
		}

		// 单个用户下载
		if len(entities.Users) == 1 && len(entities.Lists) == 0 {
			if err := deps.DownloadService.UserDownload(ctx, "cli", entities.Users[0].ScreenName, opts, reporter); err != nil {
				log.Warnf("User download failed: %v", err)
			}
		} else if len(entities.Users) == 0 && len(entities.Lists) == 1 {
			// 单个列表下载
			// 检查是否是真实列表（正数ID）还是虚拟关注列表（负数ID）
			if list, ok := entities.Lists[0].(*twitter.List); ok {
				if err := deps.DownloadService.ListDownload(ctx, "cli", list.Id, opts, reporter); err != nil {
					log.Warnf("List download failed: %v", err)
				}
			} else {
				// 虚拟关注列表，使用 BatchDownload
				if err := deps.DownloadService.BatchDownload(ctx, "cli", nil, entities.Lists, opts, reporter); err != nil {
					log.Warnf("Batch download failed: %v", err)
				}
			}
		} else {
			// 批量下载
			if err := deps.DownloadService.BatchDownload(ctx, "cli", entities.Users, entities.Lists, opts, reporter); err != nil {
				log.Warnf("Batch download failed: %v", err)
			}
		}
	}

	// 4. Profile 下载
	// 可以与批量下载同时执行（-user + -profile-user）
	// 也可以单独执行（仅 -profile-user/-profile-list）
	hasProfileUsers := len(cfg.ProfileUsers.ScreenName) > 0
	hasProfileLists := len(cfg.ProfileList.ID) > 0

	if hasProfileUsers || hasProfileLists {
		// 先处理用户 Profile
		if hasProfileUsers {
			log.Infof("profile users: %d", len(cfg.ProfileUsers.ScreenName))
			if err := deps.DownloadService.ProfileDownload(ctx, "cli", cfg.ProfileUsers.ScreenName, reporter); err != nil {
				log.Warnf("Profile download failed for users: %v", err)
			}
		}

		// 再处理列表 Profile
		if hasProfileLists {
			log.Infof("profile lists: %d", len(cfg.ProfileList.ID))
			for _, listID := range cfg.ProfileList.ID {
				if err := deps.DownloadService.ListProfileDownload(ctx, "cli", listID, reporter); err != nil {
					log.Warnf("Failed to download profile for list %d: %v", listID, err)
				}
			}
		}
		return nil
	}

	// 如果没有执行任何任务，返回错误
	if !hasBatchDownload && !hasProfileUsers && !hasProfileLists {
		if len(cfg.UsrArgs.ScreenName) > 0 || len(cfg.ListArgs.ID) > 0 || len(cfg.FollArgs.ScreenName) > 0 {
			return fmt.Errorf("no valid users or lists to download (all API calls failed)")
		}
	}

	return nil
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
