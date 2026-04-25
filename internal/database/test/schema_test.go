package database_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func setupSchemaTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)
	return db
}

func TestCreateTables(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	tables := []string{
		"users",
		"user_previous_names",
		"lsts",
		"lst_entities",
		"user_entities",
		"user_links",
	}

	for _, table := range tables {
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "table %s should exist", table)
	}
}

func TestCreateTables_UsersSchema(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	columns := []string{"id", "screen_name", "name", "protected", "friends_count", "is_accessible"}
	for _, col := range columns {
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name=?", col)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "column %s should exist in users table", col)
	}

	var pkCount int
	err := db.Get(&pkCount, "SELECT COUNT(*) FROM pragma_table_info('users') WHERE pk=1")
	require.NoError(t, err)
	assert.Equal(t, 1, pkCount, "users table should have primary key")
}

func TestCreateTables_UserPreviousNamesSchema(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	columns := []string{"id", "uid", "screen_name", "name", "record_date"}
	for _, col := range columns {
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM pragma_table_info('user_previous_names') WHERE name=?", col)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "column %s should exist in user_previous_names table", col)
	}
}

func TestCreateTables_LstsSchema(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	columns := []string{"id", "name", "owner_uid"}
	for _, col := range columns {
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM pragma_table_info('lsts') WHERE name=?", col)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "column %s should exist in lsts table", col)
	}
}

func TestCreateTables_LstEntitiesSchema(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	columns := []string{"id", "lst_id", "name", "parent_dir"}
	for _, col := range columns {
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM pragma_table_info('lst_entities') WHERE name=?", col)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "column %s should exist in lst_entities table", col)
	}
}

func TestCreateTables_UserEntitiesSchema(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	columns := []string{"id", "user_id", "name", "latest_release_time", "parent_dir", "media_count"}
	for _, col := range columns {
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM pragma_table_info('user_entities') WHERE name=?", col)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "column %s should exist in user_entities table", col)
	}
}

func TestCreateTables_UserLinksSchema(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	columns := []string{"id", "user_id", "name", "parent_lst_entity_id"}
	for _, col := range columns {
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM pragma_table_info('user_links') WHERE name=?", col)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "column %s should exist in user_links table", col)
	}
}

func TestCreateTables_Indexes(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	indexes := []string{
		"idx_users_screen_name",
		"idx_users_name",
		"idx_users_accessible",
		"idx_users_protected",
		"idx_lsts_name",
		"idx_lsts_owner",
		"idx_user_entities_user_id",
		"idx_user_entities_name",
		"idx_lst_entities_lst_id",
		"idx_user_links_user_id",
		"idx_user_links_lst_entity",
		"idx_user_previous_names_uid",
	}

	for _, idx := range indexes {
		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idx)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "index %s should exist", idx)
	}
}

func TestMigrateDatabase_AddIsAccessible(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	oldSchema := `
CREATE TABLE users (
	id INTEGER NOT NULL,
	screen_name VARCHAR NOT NULL,
	name VARCHAR NOT NULL,
	protected BOOLEAN NOT NULL,
	friends_count INTEGER NOT NULL,
	PRIMARY KEY (id)
);
`
	db.MustExec(oldSchema)

	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='is_accessible'")
	require.NoError(t, err)
	assert.Equal(t, 0, count, "is_accessible should not exist initially")

	err = database.MigrateDatabase(db)
	require.NoError(t, err)

	err = db.Get(&count, "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='is_accessible'")
	require.NoError(t, err)
	assert.Equal(t, 1, count, "is_accessible should exist after migration")
}

func TestMigrateDatabase_Idempotent(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	err := database.MigrateDatabase(db)
	require.NoError(t, err)

	err = database.MigrateDatabase(db)
	require.NoError(t, err, "migration should be idempotent")
}

func TestMigrateDatabase_NoErrorOnNewDatabase(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)

	err := database.MigrateDatabase(db)
	assert.NoError(t, err)
}

func TestCreateTables_Idempotent(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	database.CreateTables(db)
	database.CreateTables(db)
	database.CreateTables(db)

	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
