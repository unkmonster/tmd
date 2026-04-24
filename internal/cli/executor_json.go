package cli

import (
	"context"
	"path/filepath"

	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/downloading"
	log "github.com/sirupsen/logrus"
)

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
