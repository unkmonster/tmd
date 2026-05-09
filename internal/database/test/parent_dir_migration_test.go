package database_test

import (
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func TestMigrateParentDirsInSQLiteFile_DryRun(t *testing.T) {
	dbPath := createParentDirMigrationDB(t)
	seedParentDirMigrationData(t, dbPath, []parentDirSeed{
		{table: "user_entities", id: 1, ownerID: 1, name: "alice", parentDir: `f:\Twitter_DL\users`},
		{table: "user_entities", id: 2, ownerID: 1, name: "alice-rel", parentDir: `users`},
		{table: "lst_entities", id: 1, ownerID: 1, name: "list-a", parentDir: `F:\twitter_dl`},
	})

	result, err := database.MigrateParentDirsInSQLiteFile(dbPath, database.ParentDirMigrationOptions{
		FromRoot: `F:\twitter_dl`,
		ToRoot:   `/data`,
		DryRun:   true,
	})
	require.NoError(t, err)

	assert.Empty(t, result.BackupPath)
	assert.Equal(t, 2, result.UserEntitiesTotal)
	assert.Equal(t, 1, result.UserEntitiesUpdated)
	assert.Equal(t, 1, result.LstEntitiesTotal)
	assert.Equal(t, 1, result.LstEntitiesUpdated)
	require.Len(t, result.Samples, 2)
	assert.Equal(t, `/data/users`, result.Samples[0].To)
	assert.Equal(t, `/data`, result.Samples[1].To)

	userParentDirs := loadParentDirs(t, dbPath, "user_entities")
	assert.Equal(t, `f:\Twitter_DL\users`, userParentDirs[1])
	assert.Equal(t, `users`, userParentDirs[2])
}

func TestMigrateParentDirsInSQLiteFile_RewritesAndBacksUp(t *testing.T) {
	dbPath := createParentDirMigrationDB(t)
	seedParentDirMigrationData(t, dbPath, []parentDirSeed{
		{table: "user_entities", id: 1, ownerID: 1, name: "alice", parentDir: `F:\twitter_dl\users`},
		{table: "lst_entities", id: 1, ownerID: 1, name: "list-a", parentDir: `F:\twitter_dl`},
	})

	result, err := database.MigrateParentDirsInSQLiteFile(dbPath, database.ParentDirMigrationOptions{
		FromRoot: `F:\twitter_dl`,
		ToRoot:   `/data`,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, result.BackupPath)
	assert.FileExists(t, result.BackupPath)
	assert.Equal(t, 1, result.UserEntitiesUpdated)
	assert.Equal(t, 1, result.LstEntitiesUpdated)

	userParentDirs := loadParentDirs(t, dbPath, "user_entities")
	lstParentDirs := loadParentDirs(t, dbPath, "lst_entities")
	assert.Equal(t, `/data/users`, userParentDirs[1])
	assert.Equal(t, `/data`, lstParentDirs[1])

	backupUserParentDirs := loadParentDirs(t, result.BackupPath, "user_entities")
	assert.Equal(t, `F:\twitter_dl\users`, backupUserParentDirs[1])
}

func TestMigrateParentDirsInSQLiteFile_DetectsConflicts(t *testing.T) {
	dbPath := createParentDirMigrationDB(t)
	seedParentDirMigrationData(t, dbPath, []parentDirSeed{
		{table: "user_entities", id: 1, ownerID: 1, name: "alice", parentDir: `F:\twitter_dl\users`},
		{table: "user_entities", id: 2, ownerID: 1, name: "alice-copy", parentDir: `/data/users`},
	})

	result, err := database.MigrateParentDirsInSQLiteFile(dbPath, database.ParentDirMigrationOptions{
		FromRoot: `F:\twitter_dl`,
		ToRoot:   `/data`,
	})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "duplicate owner/path pair")

	userParentDirs := loadParentDirs(t, dbPath, "user_entities")
	assert.Equal(t, `F:\twitter_dl\users`, userParentDirs[1])
	assert.Equal(t, `/data/users`, userParentDirs[2])
}

type parentDirSeed struct {
	table     string
	id        int64
	ownerID   int64
	name      string
	parentDir string
}

func createParentDirMigrationDB(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "foo.db")

	db, err := database.Connect(dbPath)
	require.NoError(t, err)
	seedParentDirMigrationOwners(t, db)
	require.NoError(t, db.Close())

	return dbPath
}

func seedParentDirMigrationOwners(t *testing.T, db *sqlx.DB) {
	t.Helper()

	_, err := db.Exec(
		`INSERT INTO users(id, screen_name, name, protected, friends_count, is_accessible) VALUES(?, ?, ?, ?, ?, ?)`,
		1, "alice", "Alice", false, 10, true,
	)
	require.NoError(t, err)

	_, err = db.Exec(
		`INSERT INTO lsts(id, name, owner_user_id) VALUES(?, ?, ?)`,
		1, "list-a", 1,
	)
	require.NoError(t, err)
}

func seedParentDirMigrationData(t *testing.T, dbPath string, seeds []parentDirSeed) {
	t.Helper()

	db, err := database.Connect(dbPath)
	require.NoError(t, err)
	defer db.Close()

	for _, seed := range seeds {
		switch seed.table {
		case "user_entities":
			_, err = db.Exec(
				`INSERT INTO user_entities(id, user_id, name, parent_dir) VALUES(?, ?, ?, ?)`,
				seed.id, seed.ownerID, seed.name, seed.parentDir,
			)
		case "lst_entities":
			_, err = db.Exec(
				`INSERT INTO lst_entities(id, lst_id, name, parent_dir) VALUES(?, ?, ?, ?)`,
				seed.id, seed.ownerID, seed.name, seed.parentDir,
			)
		default:
			t.Fatalf("unsupported seed table %q", seed.table)
		}
		require.NoError(t, err)
	}
}

func loadParentDirs(t *testing.T, dbPath string, table string) map[int64]string {
	t.Helper()

	db, err := sqlx.Connect(database.DriverName, database.MustFileDSN(dbPath, true))
	require.NoError(t, err)
	defer db.Close()

	rows := []struct {
		ID        int64  `db:"id"`
		ParentDir string `db:"parent_dir"`
	}{}
	err = db.Select(&rows, `SELECT id, parent_dir FROM `+table+` ORDER BY id`)
	require.NoError(t, err)

	result := make(map[int64]string, len(rows))
	for _, row := range rows {
		result[row.ID] = row.ParentDir
	}
	return result
}
