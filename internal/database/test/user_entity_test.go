package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func setupUserEntityTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	require.NoError(t, err)
	database.CreateTables(db)

	usr := &database.User{
		Id:           1,
		ScreenName:   "testuser",
		Name:         "Test User",
		IsProtected:  false,
		FriendsCount: 100,
		IsAccessible: true,
	}
	err = database.CreateUser(db, usr)
	require.NoError(t, err)

	return db
}

func TestCreateUserEntity(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	t.Run("create_valid_entity", func(t *testing.T) {
		tmpDir := t.TempDir()
		entity := &database.UserEntity{
			UserId:    1,
			Name:      "testuser",
			ParentDir: tmpDir,
		}
		err := database.CreateUserEntity(db, entity)
		assert.NoError(t, err)
		assert.True(t, entity.Id.Valid)
		assert.Greater(t, entity.Id.Int32, int32(0))
	})

	t.Run("create_same_user_different_dir", func(t *testing.T) {
		// 使用相同Uid但不同ParentDir创建实体应该成功
		tmpDir := t.TempDir()
		entity := &database.UserEntity{
			UserId:    1,
			Name:      "testuser2",
			ParentDir: tmpDir,
		}
		err := database.CreateUserEntity(db, entity)
		assert.NoError(t, err)
	})

	t.Run("create_with_relative_path", func(t *testing.T) {
		entity := &database.UserEntity{
			UserId:    1,
			Name:      "reluser",
			ParentDir: ".",
		}
		err := database.CreateUserEntity(db, entity)
		assert.NoError(t, err)

		wd, _ := os.Getwd()
		assert.Equal(t, wd, entity.ParentDir)
	})

	t.Run("create_with_invalid_parent_dir", func(t *testing.T) {
		entity := &database.UserEntity{
			UserId:    1,
			Name:      "invalid",
			ParentDir: "\x00invalid",
		}
		err := database.CreateUserEntity(db, entity)
		assert.Error(t, err)
	})
}

func TestDelUserEntity(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	t.Run("delete_existing_entity", func(t *testing.T) {
		tmpDir := t.TempDir()
		entity := &database.UserEntity{
			UserId:    1,
			Name:      "todelete",
			ParentDir: tmpDir,
		}
		err := database.CreateUserEntity(db, entity)
		require.NoError(t, err)

		err = database.DelUserEntity(db, uint32(entity.Id.Int32))
		assert.NoError(t, err)

		retrieved, err := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("delete_nonexistent_entity", func(t *testing.T) {
		err := database.DelUserEntity(db, 99999)
		assert.NoError(t, err)
	})
}

func TestLocateUserEntity(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	entity := &database.UserEntity{
		UserId:    1,
		Name:      "located",
		ParentDir: tmpDir,
	}
	err := database.CreateUserEntity(db, entity)
	require.NoError(t, err)

	t.Run("locate_existing_entity", func(t *testing.T) {
		retrieved, err := database.LocateUserEntity(db, 1, tmpDir)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "located", retrieved.Name)
	})

	t.Run("locate_nonexistent_entity", func(t *testing.T) {
		retrieved, err := database.LocateUserEntity(db, 99999, tmpDir)
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("locate_with_different_parent_dir", func(t *testing.T) {
		retrieved, err := database.LocateUserEntity(db, 1, "/different/path")
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("locate_with_relative_path", func(t *testing.T) {
		retrieved, err := database.LocateUserEntity(db, 1, ".")
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestGetUserEntity(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	entity := &database.UserEntity{
		UserId:    1,
		Name:      "testentity",
		ParentDir: tmpDir,
	}
	err := database.CreateUserEntity(db, entity)
	require.NoError(t, err)
	entityId := int(entity.Id.Int32)

	t.Run("get_existing_entity", func(t *testing.T) {
		retrieved, err := database.GetUserEntity(db, entityId)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, int32(entityId), retrieved.Id.Int32)
		assert.Equal(t, uint64(1), retrieved.UserId)
		assert.Equal(t, "testentity", retrieved.Name)
	})

	t.Run("get_nonexistent_entity", func(t *testing.T) {
		retrieved, err := database.GetUserEntity(db, 99999)
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestUpdateUserEntity(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	entity := &database.UserEntity{
		UserId:    1,
		Name:      "original",
		ParentDir: tmpDir,
	}
	err := database.CreateUserEntity(db, entity)
	require.NoError(t, err)

	t.Run("update_name", func(t *testing.T) {
		entity.Name = "updated"
		err := database.UpdateUserEntity(db, entity)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.Equal(t, "updated", retrieved.Name)
	})

	t.Run("update_latest_release_time", func(t *testing.T) {
		now := time.Now()
		entity.LatestReleaseTime = sql.NullTime{Time: now, Valid: true}
		err := database.UpdateUserEntity(db, entity)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.True(t, retrieved.LatestReleaseTime.Valid)
		assert.WithinDuration(t, now, retrieved.LatestReleaseTime.Time, time.Second)
	})

	t.Run("update_media_count", func(t *testing.T) {
		entity.MediaCount = sql.NullInt32{Int32: 100, Valid: true}
		err := database.UpdateUserEntity(db, entity)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.Equal(t, int32(100), retrieved.MediaCount.Int32)
	})

	t.Run("update_nonexistent_entity", func(t *testing.T) {
		nonexistent := &database.UserEntity{
			Id:   sql.NullInt32{Int32: 99999, Valid: true},
			Name: "ghost",
		}
		err := database.UpdateUserEntity(db, nonexistent)
		assert.NoError(t, err)
	})
}

func TestUpdateUserEntityMediCount(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	entity := &database.UserEntity{
		UserId:    1,
		Name:      "mediacount",
		ParentDir: tmpDir,
	}
	err := database.CreateUserEntity(db, entity)
	require.NoError(t, err)

	t.Run("update_media_count", func(t *testing.T) {
		err := database.UpdateUserEntityMediCount(db, int(entity.Id.Int32), 50)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.Equal(t, int32(50), retrieved.MediaCount.Int32)
	})

	t.Run("update_nonexistent_entity", func(t *testing.T) {
		err := database.UpdateUserEntityMediCount(db, 99999, 100)
		assert.NoError(t, err)
	})
}

func TestUpdateUserEntityTweetStat(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	entity := &database.UserEntity{
		UserId:    1,
		Name:      "tweetstat",
		ParentDir: tmpDir,
	}
	err := database.CreateUserEntity(db, entity)
	require.NoError(t, err)

	t.Run("update_tweet_stat", func(t *testing.T) {
		now := time.Now()
		err := database.UpdateUserEntityTweetStat(db, int(entity.Id.Int32), now, 25)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.True(t, retrieved.LatestReleaseTime.Valid)
		assert.WithinDuration(t, now, retrieved.LatestReleaseTime.Time, time.Second)
		assert.Equal(t, int32(25), retrieved.MediaCount.Int32)
	})

	t.Run("update_tweet_stat_does_not_regress", func(t *testing.T) {
		newer := time.Now()
		older := newer.Add(-2 * time.Hour)
		require.NoError(t, database.UpdateUserEntityTweetStat(db, int(entity.Id.Int32), newer, 40))
		require.NoError(t, database.UpdateUserEntityTweetStat(db, int(entity.Id.Int32), older, 10))

		retrieved, _ := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.True(t, retrieved.LatestReleaseTime.Valid)
		assert.WithinDuration(t, newer, retrieved.LatestReleaseTime.Time, time.Second)
		assert.Equal(t, int32(40), retrieved.MediaCount.Int32)
	})

	t.Run("update_nonexistent_entity", func(t *testing.T) {
		err := database.UpdateUserEntityTweetStat(db, 99999, time.Now(), 10)
		assert.NoError(t, err)
	})
}

func TestSetUserEntityLatestReleaseTime(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	entity := &database.UserEntity{
		UserId:    1,
		Name:      "releasetime",
		ParentDir: tmpDir,
	}
	err := database.CreateUserEntity(db, entity)
	require.NoError(t, err)

	t.Run("set_release_time", func(t *testing.T) {
		now := time.Now()
		err := database.SetUserEntityLatestReleaseTime(db, int(entity.Id.Int32), now)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.True(t, retrieved.LatestReleaseTime.Valid)
		assert.WithinDuration(t, now, retrieved.LatestReleaseTime.Time, time.Second)
	})

	t.Run("set_nonexistent_entity", func(t *testing.T) {
		err := database.SetUserEntityLatestReleaseTime(db, 99999, time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no user entity found")
	})
}

func TestClearUserEntityLatestReleaseTime(t *testing.T) {
	db := setupUserEntityTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	entity := &database.UserEntity{
		UserId:            1,
		Name:              "cleartime",
		ParentDir:         tmpDir,
		LatestReleaseTime: sql.NullTime{Time: time.Now(), Valid: true},
	}
	err := database.CreateUserEntity(db, entity)
	require.NoError(t, err)

	err = database.SetUserEntityLatestReleaseTime(db, int(entity.Id.Int32), time.Now())
	require.NoError(t, err)

	t.Run("clear_release_time", func(t *testing.T) {
		err := database.ClearUserEntityLatestReleaseTime(db, int(entity.Id.Int32))
		assert.NoError(t, err)

		retrieved, _ := database.GetUserEntity(db, int(entity.Id.Int32))
		assert.False(t, retrieved.LatestReleaseTime.Valid)
	})

	t.Run("clear_nonexistent_entity", func(t *testing.T) {
		err := database.ClearUserEntityLatestReleaseTime(db, 99999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no user entity found")
	})
}

func TestUserEntity_PathMethod(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid_path", func(t *testing.T) {
		entity := &database.UserEntity{
			Name:      "testuser",
			ParentDir: tmpDir,
		}
		path, err := entity.Path()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpDir, "testuser"), path)
	})

	t.Run("empty_parent_dir", func(t *testing.T) {
		entity := &database.UserEntity{
			Name:      "testuser",
			ParentDir: "",
		}
		path, err := entity.Path()
		assert.Error(t, err)
		assert.Empty(t, path)
	})

	t.Run("empty_name", func(t *testing.T) {
		entity := &database.UserEntity{
			Name:      "",
			ParentDir: tmpDir,
		}
		path, err := entity.Path()
		assert.Error(t, err)
		assert.Empty(t, path)
	})
}
