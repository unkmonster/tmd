package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/service"
	"github.com/unkmonster/tmd/internal/twitter"
)

// Dependencies 执行依赖，嵌入 service.Dependencies 避免重复
type Dependencies struct {
	service.Dependencies
	DownloadService service.DownloadService // 可选：Service 层实例
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
		deps.DownloadService = service.NewDownloadService(&deps.Dependencies)
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
	// 参数优先级（从高到低，前面的参数会独占执行，后面的参数被忽略）：
	// 1. -jsonfile    : 第三方工具导出的JSON文件（用户资料下载）- 完全独占
	// 2. -jsonfolder  : TMD生成的.loongtweet文件夹（推文媒体下载）- 完全独占
	// 3. -mark-downloaded : 标记已下载 - 完全独占
	// 4. -user/-list/-foll : 批量下载推文 - 可与 -profile-user/-profile-list 组合
	// 5. -profile-user/-profile-list : Profile下载 - 可与批量下载组合执行

	log.Infoln("start working for...")

	// 1. 第三方工具JSON文件下载（-jsonfile）- 最高优先级，独占执行
	if len(cfg.JsonFileArgs.GetPaths()) > 0 {
		log.Infof("jsonfile: %d files", len(cfg.JsonFileArgs.GetPaths()))
		if hasOtherParams(cfg, "jsonfile") {
			log.Warn("-jsonfile is exclusive, other download parameters will be ignored")
		}
		return deps.DownloadService.JsonFileDownload(ctx, "cli", cfg.JsonFileArgs.GetPaths(), cfg.NoRetry, reporter)
	}

	// 2. TMD loongtweet文件夹下载（-jsonfolder）- 第二优先级，独占执行
	if len(cfg.JsonFolderArgs.GetPaths()) > 0 {
		log.Infof("jsonfolder: %d folders", len(cfg.JsonFolderArgs.GetPaths()))
		if hasOtherParams(cfg, "jsonfolder") {
			log.Warn("-jsonfolder is exclusive, other download parameters will be ignored")
		}
		return deps.DownloadService.JsonFolderDownload(ctx, "cli", cfg.JsonFolderArgs.GetPaths(), cfg.NoRetry, reporter)
	}

	// 3. 标记已下载（-mark-downloaded）- 第三优先级，独占执行
	if cfg.MarkDownloaded {
		log.Infoln("mark downloaded mode")
		if hasOtherParams(cfg, "mark") {
			log.Warn("-mark-downloaded is exclusive, other download parameters will be ignored")
		}
		entities := ResolveUsersAndLists(ctx, deps.Client, deps.DB, cfg.UsrArgs, cfg.ListArgs, cfg.FollArgs)
		log.Infof("mark downloaded: users: %d, lists: %d", len(entities.Users), len(entities.Lists))
		return deps.DownloadService.MarkDownloaded(ctx, "cli", entities.Users, entities.Lists, opts.MarkTime, reporter)
	}

	// 4. 批量下载（包含用户、列表、关注）- 第四优先级
	// 可与 Profile 下载（第5步）组合执行（-user + -profile-user）
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

	// 5. Profile 下载 - 第五优先级
	// 可以与批量下载（第4步）同时执行（-user + -profile-user）
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

// hasOtherParams 检查是否有其他下载参数被忽略（用于独占参数警告）
func hasOtherParams(cfg *CLIConfig, current string) bool {
	switch current {
	case "jsonfile":
		// -jsonfile 独占时，检查其他参数
		return len(cfg.JsonFolderArgs.GetPaths()) > 0 ||
			cfg.MarkDownloaded ||
			len(cfg.UsrArgs.ScreenName) > 0 ||
			len(cfg.ListArgs.ID) > 0 ||
			len(cfg.FollArgs.ScreenName) > 0 ||
			len(cfg.ProfileUsers.ScreenName) > 0 ||
			len(cfg.ProfileList.ID) > 0
	case "jsonfolder":
		// -jsonfolder 独占时，检查其他参数
		return len(cfg.JsonFileArgs.GetPaths()) > 0 ||
			cfg.MarkDownloaded ||
			len(cfg.UsrArgs.ScreenName) > 0 ||
			len(cfg.ListArgs.ID) > 0 ||
			len(cfg.FollArgs.ScreenName) > 0 ||
			len(cfg.ProfileUsers.ScreenName) > 0 ||
			len(cfg.ProfileList.ID) > 0
	case "mark":
		// -mark-downloaded 独占时，检查其他参数（除了自身需要的 -user/-list/-foll）
		return len(cfg.ProfileUsers.ScreenName) > 0 ||
			len(cfg.ProfileList.ID) > 0
	}
	return false
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
