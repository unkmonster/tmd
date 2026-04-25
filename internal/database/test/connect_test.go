package database_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func TestConnect_NewDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_new.db")

	db, err := database.Connect(dbPath)
	require.NoError(t, err)
	defer db.Close()

	assert.NotNil(t, db)

	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database file should be created")

	var count int
	err = db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table'")
	require.NoError(t, err)
	assert.Greater(t, count, 0, "tables should be created")
}

func TestConnect_ExistingDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_existing.db")

	db1, err := database.Connect(dbPath)
	require.NoError(t, err)

	usr := &database.User{
		Id:           1,
		ScreenName:   "testuser",
		Name:         "Test User",
		IsProtected:  false,
		FriendsCount: 100,
		IsAccessible: true,
	}
	err = database.CreateUser(db1, usr)
	require.NoError(t, err)
	db1.Close()

	db2, err := database.Connect(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	retrieved, err := database.GetUserById(db2, 1)
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "testuser", retrieved.ScreenName)
}

func TestConnect_WALMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_wal.db")

	db, err := database.Connect(dbPath)
	require.NoError(t, err)
	defer db.Close()

	var journalMode string
	err = db.Get(&journalMode, "PRAGMA journal_mode")
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode, "database should be in WAL mode")
}

func TestConnect_InvalidPath(t *testing.T) {
	invalidPath := "/nonexistent/directory/test.db"

	db, err := database.Connect(invalidPath)
	assert.Error(t, err)
	assert.Nil(t, db)
}

func TestConnect_ConnectionPoolSettings(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_pool.db")

	db, err := database.Connect(dbPath)
	require.NoError(t, err)
	defer db.Close()

	assert.Equal(t, 1, db.Stats().MaxOpenConnections, "MaxOpenConns should be 1 for SQLite")
}
