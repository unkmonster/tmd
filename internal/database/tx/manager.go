package tx

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Manager 事务管理器
type Manager struct {
	db *sqlx.DB
}

// NewManager 创建事务管理器
func NewManager(db *sqlx.DB) *Manager {
	return &Manager{db: db}
}

// RunInTransaction 在事务中执行函数
// 自动处理事务开始、提交和回滚
func (m *Manager) RunInTransaction(ctx context.Context, fn func(*sqlx.Tx) error) (err error) {
	tx, err := m.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
