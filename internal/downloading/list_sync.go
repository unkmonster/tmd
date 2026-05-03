package downloading

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/database/tx"
)

type ListSyncManager struct {
	txManager *tx.Manager
	mu        sync.Mutex
}

var globalListSyncManager *ListSyncManager

func InitListSyncManager(db *sqlx.DB) {
	globalListSyncManager = &ListSyncManager{
		txManager: tx.NewManager(db),
	}
}

func GetListSyncManager() *ListSyncManager {
	return globalListSyncManager
}

func (lsm *ListSyncManager) SyncListMembers(ctx context.Context, lstEntityId int, lstName string, currentMemberIDs []uint64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	var pathsToRemove []string

	err := lsm.txManager.RunInTransaction(ctx, func(tx *sqlx.Tx) error {
		var txErr error
		pathsToRemove, txErr = lsm.syncListMembersInTx(ctx, tx, lstEntityId, lstName, currentMemberIDs)
		return txErr
	})

	if err != nil {
		return err
	}

	for _, p := range pathsToRemove {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Warnln("failed to remove symlink:", p, err)
		}
	}

	return nil
}

func (lsm *ListSyncManager) syncListMembersInTx(_ context.Context, tx *sqlx.Tx, lstEntityId int, lstName string, currentMemberIDs []uint64) ([]string, error) {
	links, err := database.GetUserLinksByLstEntityId(tx, lstEntityId)
	if err != nil {
		return nil, err
	}

	memberSet := make(map[uint64]bool)
	for _, id := range currentMemberIDs {
		memberSet[id] = true
	}

	removedCount := 0
	pathsToRemove := make([]string, 0)

	for _, link := range links {
		if !memberSet[link.UserId] {
			linkPaths, linkErr := lsm.removeUserLinkInTx(tx, link, lstEntityId)
			if linkErr != nil {
				log.Warnln("failed to remove user link:", link.UserId, linkErr)
				return nil, linkErr
			}
			pathsToRemove = append(pathsToRemove, linkPaths...)
			removedCount++
		}
	}

	if removedCount > 0 {
		log.Infoln("Removed", removedCount, "users from list", lstName, "(no longer members)")
	}

	return pathsToRemove, nil
}

func (lsm *ListSyncManager) removeUserLinkInTx(tx *sqlx.Tx, link *database.UserLink, lstEntityId int) ([]string, error) {
	if link.Id == 0 {
		return nil, fmt.Errorf("link id is not valid for user %d in list %d", link.UserId, lstEntityId)
	}

	var pathsToRemove []string

	linkpath, err := link.PathWithTx(tx)
	if err == nil && linkpath != "" {
		pathsToRemove = append(pathsToRemove, linkpath)
	}

	_, err = tx.Exec(`DELETE FROM user_links WHERE id = ?`, link.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to delete user link %d from database: %w", link.Id, err)
	}

	return pathsToRemove, nil
}
