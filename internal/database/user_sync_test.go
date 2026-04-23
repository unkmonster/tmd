package database

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDBForSync(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)
	CreateTables(db)
	return db
}

func TestSyncUser_NewUser(t *testing.T) {
	db := setupTestDBForSync(t)
	defer db.Close()

	err := SyncUser(db, 12345, "Test User", "testuser", false, 100, true)
	require.NoError(t, err)

	user, err := GetUserById(db, 12345)
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

	err := SyncUser(db, 12345, "Test User", "testuser", false, 100, true)
	require.NoError(t, err)

	err = SyncUser(db, 12345, "Updated Name", "testuser", true, 200, false)
	require.NoError(t, err)

	user, err := GetUserById(db, 12345)
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

	err := SyncUser(db, 12345, "Old Name", "oldname", false, 100, true)
	require.NoError(t, err)

	err = SyncUser(db, 12345, "New Name", "newname", false, 100, true)
	require.NoError(t, err)

	user, err := GetUserById(db, 12345)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "New Name", user.Name)
	assert.Equal(t, "newname", user.ScreenName)
}

func TestSyncUser_ProfileScenario(t *testing.T) {
	db := setupTestDBForSync(t)
	defer db.Close()

	err := SyncUser(db, 99999, "Profile User", "profileuser", true, 0, true)
	require.NoError(t, err)

	user, err := GetUserById(db, 99999)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, 0, user.FriendsCount)
	assert.True(t, user.IsProtected)
	assert.True(t, user.IsAccessible)
}

func TestSyncUser_ClearLatestReleaseTime(t *testing.T) {
	db := setupTestDBForSync(t)
	defer db.Close()

	err := SyncUser(db, 12345, "Test User", "testuser", false, 100, true)
	require.NoError(t, err)

	entity := &UserEntity{Uid: 12345, Name: "testuser", ParentDir: "/tmp/test"}
	err = CreateUserEntity(db, entity)
	require.NoError(t, err)

	now := time.Now()
	err = SetUserEntityLatestReleaseTime(db, int(NullInt32(entity.Id)), now)
	require.NoError(t, err)

	err = ClearUserEntityLatestReleaseTime(db, int(NullInt32(entity.Id)))
	require.NoError(t, err)

	updated, err := GetUserEntity(db, int(NullInt32(entity.Id)))
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, updated.LatestReleaseTime.Time.IsZero())
}
