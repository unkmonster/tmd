package database_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func setupTestDBForSync(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	require.NoError(t, err)
	database.CreateTables(db)
	return db
}

func TestSyncUser_NewUser(t *testing.T) {
	db := setupTestDBForSync(t)
	defer db.Close()

	err := database.SyncUser(db, 12345, "Test User", "testuser", false, 100, true)
	require.NoError(t, err)

	user, err := database.GetUserById(db, 12345)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, uint64(12345), user.Id)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "testuser", user.ScreenName)
	assert.Equal(t, 100, user.FriendsCount)
	assert.False(t, user.IsProtected)
	assert.True(t, user.IsAccessible)
}

func TestSyncUser_UpdateExistingUser(t *testing.T) {
	db := setupTestDBForSync(t)
	defer db.Close()

	err := database.SyncUser(db, 12345, "Test User", "testuser", false, 100, true)
	require.NoError(t, err)

	err = database.SyncUser(db, 12345, "Updated Name", "testuser", true, 200, false)
	require.NoError(t, err)

	user, err := database.GetUserById(db, 12345)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "Updated Name", user.Name)
	assert.Equal(t, "testuser", user.ScreenName)
	assert.Equal(t, 200, user.FriendsCount)
	assert.True(t, user.IsProtected)
	assert.False(t, user.IsAccessible)
}

func TestSyncUser_RenamedUser(t *testing.T) {
	db := setupTestDBForSync(t)
	defer db.Close()

	err := database.SyncUser(db, 12345, "Old Name", "oldname", false, 100, true)
	require.NoError(t, err)

	err = database.SyncUser(db, 12345, "New Name", "newname", false, 100, true)
	require.NoError(t, err)

	user, err := database.GetUserById(db, 12345)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "New Name", user.Name)
	assert.Equal(t, "newname", user.ScreenName)

	var historyCount int
	err = db.Get(&historyCount, "SELECT COUNT(*) FROM user_previous_names WHERE user_id = ? AND name = ?", uint64(12345), "Old Name")
	assert.NoError(t, err)
	assert.Greater(t, historyCount, 0)
}

func TestSyncUser_ProfileScenario(t *testing.T) {
	db := setupTestDBForSync(t)
	defer db.Close()

	err := database.SyncUser(db, 99999, "Profile User", "profileuser", true, 0, true)
	require.NoError(t, err)

	user, err := database.GetUserById(db, 99999)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, 0, user.FriendsCount)
	assert.True(t, user.IsProtected)
	assert.True(t, user.IsAccessible)
}
