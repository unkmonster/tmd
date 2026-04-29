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

func NewListSyncManager(db *sqlx.DB) *ListSyncManager {
	return &ListSyncManager{
		txManager: tx.NewManager(db),
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

	// 使用事务管理器统一处理事务生命周期
	return lsm.txManager.RunInTransaction(ctx, func(tx *sqlx.Tx) error {
		return lsm.syncListMembersInTx(ctx, tx, lstEntityId, lstName, currentMemberIDs)
	})
}

// syncListMembersInTx 在事务内同步列表成员
// 所有数据库操作必须使用传入的 tx 参数
func (lsm *ListSyncManager) syncListMembersInTx(_ context.Context, tx *sqlx.Tx, lstEntityId int, lstName string, currentMemberIDs []uint64) error {
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
			if err := lsm.removeUserLinkInTx(tx, link, lstEntityId); err != nil {
				log.Warnln("failed to remove user link:", link.UserId, err)
				return err
			} else {
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		log.Infoln("Removed", removedCount, "users from list", lstName, "(no longer members)")
	}

	return nil
}

// removeUserLinkInTx 在事务内删除用户链接
// 所有数据库操作必须使用传入的 tx 参数
func (lsm *ListSyncManager) removeUserLinkInTx(tx *sqlx.Tx, link *database.UserLink, lstEntityId int) error {
	if link.Id == 0 {
		return fmt.Errorf("link id is not valid for user %d in list %d", link.UserId, lstEntityId)
	}

	// 使用 PathWithTx 确保在事务内查询
	linkpath, err := link.PathWithTx(tx)
	if err == nil {
		if err := os.Remove(linkpath); err != nil && !os.IsNotExist(err) {
			log.Warnln("failed to remove symlink:", linkpath, err)
		}
	}

	_, err = tx.Exec(`DELETE FROM user_links WHERE id = ?`, link.Id)
	if err != nil {
		return fmt.Errorf("failed to delete user link %d from database: %w", link.Id, err)
	}

	return nil
}
