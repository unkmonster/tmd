package tx

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/database"
)

func newTestManager(t *testing.T) (*Manager, *sqlx.DB) {
	t.Helper()

	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	if err != nil {
		t.Fatalf("failed to connect test database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	return NewManager(db), db
}

func TestRunInTransaction_CancelledContextDoesNotRunCallback(t *testing.T) {
	manager, _ := newTestManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := manager.RunInTransaction(ctx, func(tx *sqlx.Tx) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("RunInTransaction should return an error for a cancelled context")
	}
	if called {
		t.Fatal("RunInTransaction should not run callback for a cancelled context")
	}
}

func TestRunInTransaction_RollsBackWhenContextCancelledBeforeCommit(t *testing.T) {
	manager, db := newTestManager(t)

	db.MustExec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)

	ctx, cancel := context.WithCancel(context.Background())
	err := manager.RunInTransaction(ctx, func(tx *sqlx.Tx) error {
		if _, execErr := tx.Exec(`INSERT INTO items(id, name) VALUES(1, 'item')`); execErr != nil {
			return execErr
		}
		cancel()
		return nil
	})
	if err == nil {
		t.Fatal("RunInTransaction should return an error when context is cancelled before commit")
	}

	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM items`); err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("transaction should be rolled back, got %d rows", count)
	}
}
