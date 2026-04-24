package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/downloading"
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
