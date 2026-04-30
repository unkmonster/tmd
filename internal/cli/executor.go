package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/service"
)

// Dependencies 执行依赖，嵌入 service.Dependencies 避免重复
type Dependencies struct {
	service.Dependencies
	DownloadService service.DownloadService // 可选：Service 层实例
}

// Execute 执行 CLI 命令
func Execute(ctx context.Context, args []string, deps *Dependencies) error {
	if deps == nil {
		return fmt.Errorf("dependencies is nil")
	}

	// 解析参数
	_, cfg, err := ParseArgs(args)
	if err != nil {
		return fmt.Errorf("parse args failed: %w", err)
	}

	if !hasAnyTasks(cfg) {
		log.Infoln("no download tasks specified")
		return nil
	}

	log.Infoln("start working for...")

	// 如果没有提供 Service，创建一个默认的
	if deps.DownloadService == nil {
		downloadService, err := service.NewDownloadService(&deps.Dependencies)
		if err != nil {
			return fmt.Errorf("failed to create download service: %w", err)
		}
		deps.DownloadService = downloadService
	}

	// 创建进度报告器
	reporter := service.NewLogReporter(log.Infof)

	// 构建下载选项
	opts := service.DownloadOptions{
		AutoFollow:  cfg.AutoFollow,
		SkipProfile: cfg.NoProfile,
		NoRetry:     cfg.NoRetry,
	}

	var markTime *string
	if cfg.MarkTime != "" {
		markTime = &cfg.MarkTime
	}

	// 处理不同类型的下载任务
	// 参数优先级（从高到低，前面的参数会独占执行，后面的参数被忽略）：
	// 1. -jsonfile    : 第三方工具导出的JSON文件（用户资料下载）- 完全独占
	// 2. -jsonfolder  : TMD生成的.loongtweet文件夹（推文媒体下载）- 完全独占
	// 3. -mark-downloaded : 标记已下载 - 完全独占
	// 4. -user/-list/-foll : 批量下载推文 - 可与 -profile-user/-profile-list 组合
	// 5. -profile-user/-profile-list : Profile下载 - 可与批量下载组合执行

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
		log.Infof("mark downloaded: users: %d, lists: %d, following: %d",
			len(cfg.UsrArgs.ScreenName), len(cfg.ListArgs.ID), len(cfg.FollArgs.ScreenName))
		return deps.DownloadService.MarkDownloaded(ctx, "cli",
			cfg.UsrArgs.ScreenName, cfg.ListArgs.ID, cfg.FollArgs.ScreenName,
			markTime, reporter)
	}

	// 4. 批量下载（包含用户、列表、关注）- 第四优先级
	screenNames := cfg.UsrArgs.ScreenName
	listIDs := cfg.ListArgs.ID
	followingNames := cfg.FollArgs.ScreenName
	hasBatchDownload := len(screenNames) > 0 || len(listIDs) > 0 || len(followingNames) > 0

	if hasBatchDownload {
		log.Infof("users: %d, lists: %d, following: %d", len(screenNames), len(listIDs), len(followingNames))

		var batchErr error
		if len(screenNames) == 1 && len(listIDs) == 0 && len(followingNames) == 0 {
			if err := deps.DownloadService.UserDownload(ctx, "cli", screenNames[0], opts, reporter); err != nil {
				log.Warnf("User download failed: %v", err)
				batchErr = err
			}
		} else if len(screenNames) == 0 && len(listIDs) == 0 && len(followingNames) == 1 {
			if err := deps.DownloadService.FollowingDownload(ctx, "cli", followingNames[0], opts, reporter); err != nil {
				log.Warnf("Following download failed: %v", err)
				batchErr = err
			}
		} else {
			// 直接使用 BatchDownload 处理单个列表或多个列表的下载
			// BatchDownload 在底层处理单个列表时不会产生性能浪费，并且能正确处理各种类型的 ListBase
			if err := deps.DownloadService.BatchDownload(ctx, "cli", screenNames, listIDs, followingNames, opts, reporter); err != nil {
				log.Warnf("Batch download failed: %v", err)
				batchErr = err
			}
		}

		if batchErr != nil {
			return batchErr
		}
	}

	// 5. Profile 下载 - 第五优先级
	// 可以与批量下载（第4步）同时执行（-user + -profile-user）
	// 也可以单独执行（仅 -profile-user/-profile-list）
	hasProfileUsers := len(cfg.ProfileUsers.ScreenName) > 0
	hasProfileLists := len(cfg.ProfileList.ID) > 0

	if hasProfileUsers || hasProfileLists {
		var profileErr error
		// 先处理用户 Profile
		if hasProfileUsers {
			log.Infof("profile users: %d", len(cfg.ProfileUsers.ScreenName))
			if err := deps.DownloadService.ProfileDownload(ctx, "cli", cfg.ProfileUsers.ScreenName, reporter); err != nil {
				log.Warnf("Profile download failed for users: %v", err)
				profileErr = err
			}
		}

		// 再处理列表 Profile
		if hasProfileLists {
			log.Infof("profile lists: %d", len(cfg.ProfileList.ID))
			for _, listID := range cfg.ProfileList.ID {
				if err := deps.DownloadService.ListProfileDownload(ctx, "cli", listID, reporter); err != nil {
					log.Warnf("Failed to download profile for list %d: %v", listID, err)
					profileErr = err
				}
			}
		}
		if profileErr != nil {
			return profileErr
		}
		return nil
	}

	return nil
}

func hasAnyTasks(cfg *CLIConfig) bool {
	return len(cfg.JsonFileArgs.GetPaths()) > 0 ||
		len(cfg.JsonFolderArgs.GetPaths()) > 0 ||
		cfg.MarkDownloaded ||
		len(cfg.UsrArgs.ScreenName) > 0 ||
		len(cfg.ListArgs.ID) > 0 ||
		len(cfg.FollArgs.ScreenName) > 0 ||
		len(cfg.ProfileUsers.ScreenName) > 0 ||
		len(cfg.ProfileList.ID) > 0
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
