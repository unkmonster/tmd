package cli

import (
	"context"
	"errors"
	"flag"
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

type cliTaskMode int

const (
	cliTaskModeNone cliTaskMode = iota
	cliTaskModeJSONFile
	cliTaskModeJSONFolder
	cliTaskModeMarkDownloaded
	cliTaskModeBatch
	cliTaskModeProfile
)

type cliTaskSelection struct {
	cfg *CLIConfig
}

func newCLITaskSelection(cfg *CLIConfig) cliTaskSelection {
	return cliTaskSelection{cfg: cfg}
}

func (s cliTaskSelection) hasJSONFile() bool {
	return len(s.cfg.JsonFileArgs.GetPaths()) > 0
}

func (s cliTaskSelection) hasJSONFolder() bool {
	return len(s.cfg.JsonFolderArgs.GetPaths()) > 0
}

func (s cliTaskSelection) hasMarkDownloaded() bool {
	return s.cfg.MarkDownloaded
}

func (s cliTaskSelection) hasBatchDownload() bool {
	return len(s.cfg.UsrArgs.ScreenName) > 0 ||
		len(s.cfg.ListArgs.ID) > 0 ||
		len(s.cfg.FollArgs.ScreenName) > 0
}

func (s cliTaskSelection) hasProfileDownload() bool {
	return len(s.cfg.ProfileUsers.ScreenName) > 0 ||
		len(s.cfg.ProfileList.ID) > 0
}

func (s cliTaskSelection) hasAnyTasks() bool {
	return s.hasJSONFile() ||
		s.hasJSONFolder() ||
		s.hasMarkDownloaded() ||
		s.hasBatchDownload() ||
		s.hasProfileDownload()
}

func (s cliTaskSelection) primaryMode() cliTaskMode {
	switch {
	case s.hasJSONFile():
		return cliTaskModeJSONFile
	case s.hasJSONFolder():
		return cliTaskModeJSONFolder
	case s.hasMarkDownloaded():
		return cliTaskModeMarkDownloaded
	case s.hasBatchDownload():
		return cliTaskModeBatch
	case s.hasProfileDownload():
		return cliTaskModeProfile
	default:
		return cliTaskModeNone
	}
}

func (s cliTaskSelection) shouldWarnExclusiveMode(mode cliTaskMode) bool {
	switch mode {
	case cliTaskModeJSONFile:
		return s.hasJSONFolder() || s.hasMarkDownloaded() || s.hasBatchDownload() || s.hasProfileDownload()
	case cliTaskModeJSONFolder:
		return s.hasJSONFile() || s.hasMarkDownloaded() || s.hasBatchDownload() || s.hasProfileDownload()
	case cliTaskModeMarkDownloaded:
		return s.hasProfileDownload()
	default:
		return false
	}
}

// Execute 执行 CLI 命令
func Execute(ctx context.Context, args []string, deps *Dependencies) error {
	if deps == nil {
		return fmt.Errorf("dependencies is nil")
	}

	// 解析参数
	cfg, err := ParseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse args failed: %w", err)
	}

	selection := newCLITaskSelection(cfg)
	if !selection.hasAnyTasks() {
		log.Infoln("no download tasks specified")
		return nil
	}

	log.Infoln("start working for...")

	// 如果没有提供 Service，为本次执行创建默认 Service；不写回 deps，避免复用同一 deps 时产生隐式共享。
	downloadService := deps.DownloadService
	if downloadService == nil {
		var err error
		downloadService, err = service.NewDownloadService(&deps.Dependencies)
		if err != nil {
			return fmt.Errorf("failed to create download service: %w", err)
		}
	}

	// 创建进度报告器
	reporter := service.NewLogReporter(log.Infof)

	// 构建下载选项
	opts := service.DownloadOptions{
		AutoFollow:    cfg.AutoFollow,
		FollowMembers: cfg.FollowMembers,
		SkipProfile:   cfg.NoProfile,
		NoRetry:       cfg.NoRetry,
	}

	var markTime *string
	if cfg.MarkTime != "" {
		markTime = &cfg.MarkTime
	}

	// 处理不同类型的下载任务
	// 参数优先级（从高到低，前面的参数会独占执行，后面的参数被忽略）：
	// 1. -jsonfile    : 第三方工具导出的 JSON 文件（推文媒体下载）- 完全独占
	// 2. -jsonfolder  : TMD生成的.loongtweet文件夹（推文媒体下载）- 完全独占
	// 3. -mark-downloaded : 标记已下载 - 完全独占
	// 4. -user/-list/-foll : 批量下载推文 - 可与 -profile-user/-profile-list 组合
	// 5. -profile-user/-profile-list : Profile下载 - 可与批量下载组合执行

	switch selection.primaryMode() {
	case cliTaskModeJSONFile:
		log.Infof("jsonfile: %d files", len(cfg.JsonFileArgs.GetPaths()))
		if selection.shouldWarnExclusiveMode(cliTaskModeJSONFile) {
			log.Warn("-jsonfile is exclusive; jsonfolder/mark-downloaded/batch/profile will be ignored")
		}
		return downloadService.JsonFileDownload(ctx, "cli", cfg.JsonFileArgs.GetPaths(), cfg.NoRetry, reporter)
	case cliTaskModeJSONFolder:
		log.Infof("jsonfolder: %d folders", len(cfg.JsonFolderArgs.GetPaths()))
		if selection.shouldWarnExclusiveMode(cliTaskModeJSONFolder) {
			log.Warn("-jsonfolder is exclusive; jsonfile/mark-downloaded/batch/profile will be ignored")
		}
		return downloadService.JsonFolderDownload(ctx, "cli", cfg.JsonFolderArgs.GetPaths(), cfg.NoRetry, reporter)
	case cliTaskModeMarkDownloaded:
		log.Infoln("mark downloaded mode")
		if selection.shouldWarnExclusiveMode(cliTaskModeMarkDownloaded) {
			log.Warn("-mark-downloaded is exclusive; profile download parameters will be ignored")
		}
		log.Infof("mark downloaded: users: %d, lists: %d, following: %d",
			len(cfg.UsrArgs.ScreenName), len(cfg.ListArgs.ID), len(cfg.FollArgs.ScreenName))
		return downloadService.MarkDownloaded(ctx, "cli",
			cfg.UsrArgs.ScreenName, cfg.ListArgs.ID, cfg.FollArgs.ScreenName,
			markTime, reporter)
	}

	// 4. 批量下载（包含用户、列表、关注）- 第四优先级
	screenNames := cfg.UsrArgs.ScreenName
	listIDs := cfg.ListArgs.ID
	followingNames := cfg.FollArgs.ScreenName

	if selection.hasBatchDownload() {
		log.Infof("users: %d, lists: %d, following: %d", len(screenNames), len(listIDs), len(followingNames))

		var batchErr error
		if len(screenNames) == 1 && len(listIDs) == 0 && len(followingNames) == 0 {
			if err := downloadService.UserDownload(ctx, "cli", screenNames[0], opts, reporter); err != nil {
				log.Warnf("User download failed: %v", err)
				batchErr = err
			}
		} else if len(screenNames) == 0 && len(listIDs) == 0 && len(followingNames) == 1 {
			if err := downloadService.FollowingDownload(ctx, "cli", followingNames[0], opts, reporter); err != nil {
				log.Warnf("Following download failed: %v", err)
				batchErr = err
			}
		} else {
			// 直接使用 BatchDownload 处理单个列表或多个列表的下载
			// BatchDownload 在底层处理单个列表时不会产生性能浪费，并且能正确处理各种类型的 ListBase
			if err := downloadService.BatchDownload(ctx, "cli", screenNames, listIDs, followingNames, opts, reporter); err != nil {
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
	// 注意：如果批量下载失败，此处的 Profile 下载不会执行
	// （第4步失败时直接返回，不会走到这里）
	hasProfileUsers := len(cfg.ProfileUsers.ScreenName) > 0
	hasProfileLists := len(cfg.ProfileList.ID) > 0

	if selection.hasProfileDownload() {
		var profileErr error
		// 先处理用户 Profile
		if hasProfileUsers {
			log.Infof("profile users: %d", len(cfg.ProfileUsers.ScreenName))
			if err := downloadService.ProfileDownload(ctx, "cli", cfg.ProfileUsers.ScreenName, reporter); err != nil {
				log.Warnf("Profile download failed for users: %v", err)
				profileErr = err
			}
		}

		// 再处理列表 Profile
		if hasProfileLists {
			log.Infof("profile lists: %d", len(cfg.ProfileList.ID))
			for _, listID := range cfg.ProfileList.ID {
				if err := downloadService.ListProfileDownload(ctx, "cli", listID, reporter); err != nil {
					log.Warnf("Failed to download profile for list %d: %v", listID, err)
					profileErr = errors.Join(profileErr, err)
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
