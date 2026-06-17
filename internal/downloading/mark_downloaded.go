package downloading

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/twitter"
)

type MarkedUserInfo struct {
	UserID     uint64 `json:"user_id"`
	ScreenName string `json:"screen_name"`
	EntityID   int    `json:"entity_id"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

func MarkUsersAsDownloaded(ctx context.Context, client *resty.Client, db *sqlx.DB, lists []twitter.ListBase, users []*twitter.User, dir string, markTimeStr string, maxLen int) ([]MarkedUserInfo, error) {
	var timestamp *time.Time
	if markTimeStr == "" {
		now := time.Now()
		timestamp = &now
		log.Infoln("marking users as downloaded, timestamp:", timestamp.Format(time.RFC3339))
	} else if strings.ToLower(markTimeStr) == "null" || strings.ToLower(markTimeStr) == "nil" {
		timestamp = nil
		log.Infoln("marking users as downloaded, timestamp: NULL (full download)")
	} else {
		loc, locErr := time.LoadLocation("Local")
		if locErr != nil {
			loc = time.UTC
		}
		parsedTime, err := time.ParseInLocation("2006-01-02T15:04:05", markTimeStr, loc)
		if err != nil {
			return nil, fmt.Errorf("invalid mark-time format '%s', expected: 2006-01-02T15:04:05 (example: 2024-01-15T10:30:00) or 'null' for full download: %v", markTimeStr, err)
		}
		timestamp = &parsedTime
		log.Infoln("marking users as downloaded, timestamp:", timestamp.Format(time.RFC3339))
	}

	var results []MarkedUserInfo
	var successCount, failCount int

	for _, lst := range lists {
		if err := context.Cause(ctx); err != nil {
			return results, err
		}

		if lst == nil {
			continue
		}

		membersResult, err := lst.GetMembers(ctx, client)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "does not exist or is not accessible") ||
				strings.Contains(errStr, "unable to get timeline data") {
				return nil, fmt.Errorf("list %s does not exist or is not accessible", lst.Title())
			}
			log.Warnln("✗", lst.Title(), "-", "failed to get list members:", err)
			continue
		}

		for _, user := range membersResult.Users {
			if err := context.Cause(ctx); err != nil {
				return results, err
			}

			if user == nil {
				continue
			}

			info := markSingleUserWithInfo(db, user, dir, timestamp, maxLen)
			results = append(results, info)
			if info.Success {
				successCount++
			} else {
				failCount++
			}
		}
	}

	for _, user := range users {
		if err := context.Cause(ctx); err != nil {
			return results, err
		}

		if user == nil {
			continue
		}

		info := markSingleUserWithInfo(db, user, dir, timestamp, maxLen)
		results = append(results, info)
		if info.Success {
			successCount++
		} else {
			failCount++
		}
	}

	log.Infoln("finished marking users as downloaded, success:", successCount, "failed:", failCount)
	return results, nil
}

func markSingleUserWithInfo(db *sqlx.DB, user *twitter.User, dir string, timestamp *time.Time, maxLen int) (info MarkedUserInfo) {
	if user == nil {
		info.Success = false
		info.Error = "user is nil"
		return info
	}

	info = MarkedUserInfo{
		UserID:     user.Id,
		ScreenName: user.ScreenName,
		Success:    false,
	}

	defer func() {
		if r := recover(); r != nil {
			info.Success = false
			info.Error = fmt.Sprintf("panic: %v", r)
			log.Errorf("[markSingleUserWithInfo] panic recovered: %v", r)
		}
	}()

	entity, err := syncUserAndEntity(db, user, dir, maxLen)
	if err != nil {
		info.Error = fmt.Sprintf("failed to sync user and entity: %v", err)
		log.Warnln("✗", user.Title(), "-", "failed to mark user:", err)
		return info
	}

	if timestamp == nil {
		if err := entity.ClearLatestReleaseTime(); err != nil {
			info.Error = fmt.Sprintf("failed to clear latest release time: %v", err)
			log.Warnln("✗", user.Title(), "-", "failed to clear latest release time:", err)
			return info
		}
		log.Infoln("✓", user.Title(), "-", "cleared latest release time for full download")
	} else {
		if err := entity.SetLatestReleaseTime(*timestamp); err != nil {
			info.Error = fmt.Sprintf("failed to set latest release time: %v", err)
			log.Warnln("✗", user.Title(), "-", "failed to set latest release time:", err)
			return info
		}
	}

	info.Success = true
	eid, err := entity.Id()
	if err != nil {
		info.Error = fmt.Sprintf("failed to get entity id: %v", err)
		log.Warnln("✗", user.Title(), "-", "failed to get entity id:", err)
		return info
	}
	info.EntityID = eid
	log.Infoln("✓", user.Title(), "-", "marked as downloaded")
	return info
}
