package cli

import (
	"context"
	"fmt"

	"github.com/unkmonster/tmd/internal/downloading"
)

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
