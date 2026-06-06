package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

func SyncUser(db *sqlx.DB, userId uint64, name string, screenName string, isProtected bool, friendsCount int, accessible bool) error {
	renamed := false
	isNew := false

	usrdb, err := GetUserById(db, userId)
	if err != nil {
		return err
	}
	if usrdb == nil {
		isNew = true
	} else {
		renamed = usrdb.Name != name || usrdb.ScreenName != screenName
	}

	// renamed：在 UPSERT 之前记录旧名称，确保 RecordUserPreviousName 失败时
	// 用户表中仍保留旧名称，不丢失改名历史。
	if renamed {
		if err := RecordUserPreviousName(db, userId, usrdb.Name, usrdb.ScreenName); err != nil {
			return err
		}
	}

	// 使用 UPSERT（INSERT ... ON CONFLICT）消除并发下的 TOCTOU 竞态：
	// 两个 goroutine 同时判断用户不存在时，后一个 INSERT 不再因 PK 冲突而失败。
	stmt := `INSERT INTO users(id, screen_name, name, protected, friends_count, is_accessible)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			screen_name=excluded.screen_name,
			name=excluded.name,
			protected=excluded.protected,
			friends_count=excluded.friends_count,
			is_accessible=excluded.is_accessible`
	if _, err := db.Exec(stmt, userId, screenName, name, isProtected, friendsCount, accessible); err != nil {
		return fmt.Errorf("failed to upsert user %d (%s): %w", userId, screenName, err)
	}

	// isNew：首次同步，记录当前名称作为基线。这样后续改名时
	// user_previous_names 表中才包含完整改名链（含原始名称）。
	if isNew {
		if err := RecordUserPreviousName(db, userId, name, screenName); err != nil {
			return fmt.Errorf("failed to record initial name for new user %d (%s): %w", userId, screenName, err)
		}
	}
	return nil
}
