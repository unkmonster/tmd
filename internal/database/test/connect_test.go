package database_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jmoiron/sqlx"
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
	if runtime.GOOS == "windows" {
		t.Skip("path semantics differ on Windows")
	}

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

func TestConnect_RebuildsLegacySchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_legacy.db")

	legacyDB, err := sqlx.Connect(database.DriverName, database.MustFileDSN(dbPath, true))
	require.NoError(t, err)

	legacyDB.MustExec(`
CREATE TABLE users (
	id INTEGER NOT NULL,
	screen_name VARCHAR NOT NULL,
	name VARCHAR NOT NULL,
	protected BOOLEAN NOT NULL,
	friends_count INTEGER NOT NULL,
	PRIMARY KEY (id),
	UNIQUE (screen_name)
);
`)
	legacyDB.MustExec(`INSERT INTO users(id, screen_name, name, protected, friends_count) VALUES(1, 'legacyuser', 'Legacy User', 0, 10)`)
	require.NoError(t, legacyDB.Close())

	db, err := database.Connect(dbPath)
	require.NoError(t, err)
	defer db.Close()

	retrieved, err := database.GetUserById(db, 1)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "legacyuser", retrieved.ScreenName)
	assert.True(t, retrieved.IsAccessible)

	backups, err := filepath.Glob(dbPath + ".backup.*")
	require.NoError(t, err)
	assert.NotEmpty(t, backups, "legacy database should be backed up before rebuild")
}
