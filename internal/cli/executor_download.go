package cli

import (
	"context"

	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/downloading"
	log "github.com/sirupsen/logrus"
)

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
