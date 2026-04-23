package downloading

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/database"
)

type ListSyncManager struct {
	db *sqlx.DB
	mu sync.Mutex
}

func NewListSyncManager(db *sqlx.DB) *ListSyncManager {
	return &ListSyncManager{
		db: db,
	}
}

func (lsm *ListSyncManager) SyncListMembers(ctx context.Context, lstEntityId int, lstName string, currentMemberIDs []uint64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	tx, err := lsm.db.Beginx()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	links, err := database.GetUserLinksByLstEntityId(tx, lstEntityId)
	if err != nil {
		return err
	}

	memberSet := make(map[uint64]bool)
	for _, id := range currentMemberIDs {
		memberSet[id] = true
	}

	removedCount := 0
	for _, link := range links {
		if !memberSet[link.UserId] {
			if err := lsm.removeUserLinkWithTx(tx, link, lstEntityId); err != nil {
				log.Warnln("failed to remove user link:", link.UserId, err)
			} else {
				removedCount++
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	if removedCount > 0 {
		log.Infoln("Removed", removedCount, "users from list", lstName, "(no longer members)")
	}

	return nil
}

func (lsm *ListSyncManager) removeUserLinkWithTx(tx *sqlx.Tx, link *database.UserLink, lstEntityId int) error {
	if link.Id == 0 {
		return fmt.Errorf("link id is not valid for user %d in list %d", link.UserId, lstEntityId)
	}

	linkpath, err := link.Path(lsm.db)
	if err == nil {
		if err := os.Remove(linkpath); err != nil && !os.IsNotExist(err) {
			log.Warnln("failed to remove symlink:", linkpath, err)
		}
	}

	_, err = tx.Exec(`DELETE FROM user_links WHERE id = ?`, link.Id)
	if err != nil {
		return fmt.Errorf("failed to delete user link %d from database: %w", link.Id, err)
	}

	log.Debugln("Removed user link:", link.UserId, "from list", lstEntityId)
	return nil
}
