package downloading

import (
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/unkmonster/tmd/internal/database"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	dsn := "file:" + path + "?_journal_mode=WAL&cache=shared"
	db, err := sqlx.Connect("sqlite3", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to test DB: %v", err)
	}
	database.CreateTables(db)
	return db
}
