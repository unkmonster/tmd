package downloading

import (
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/database"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlx.Connect(database.DriverName, database.MustFileDSN(path, true))
	if err != nil {
		t.Fatalf("Failed to connect to test DB: %v", err)
	}
	database.CreateTables(db)
	return db
}
