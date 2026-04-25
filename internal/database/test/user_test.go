package database_test

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func setupUserTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)
	database.CreateTables(db)
	return db
}

func TestCreateUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	t.Run("create_valid_user", func(t *testing.T) {
		usr := &database.User{
			Id:           1,
			ScreenName:   "testuser",
			Name:         "Test User",
			IsProtected:  false,
			FriendsCount: 100,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		assert.NoError(t, err)

		retrieved, err := database.GetUserById(db, 1)
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "testuser", retrieved.ScreenName)
	})

	t.Run("create_duplicate_user", func(t *testing.T) {
		usr := &database.User{
			Id:           1,
			ScreenName:   "testuser",
			Name:         "Test User",
			IsProtected:  false,
			FriendsCount: 100,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create user")
	})

	t.Run("create_user_with_same_screen_name", func(t *testing.T) {
		usr := &database.User{
			Id:           2,
			ScreenName:   "testuser",
			Name:         "Another User",
			IsProtected:  false,
			FriendsCount: 50,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		assert.Error(t, err)
	})
}

func TestGetUserById(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	t.Run("get_existing_user", func(t *testing.T) {
		usr := &database.User{
			Id:           1,
			ScreenName:   "testuser",
			Name:         "Test User",
			IsProtected:  true,
			FriendsCount: 200,
			IsAccessible: false,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		retrieved, err := database.GetUserById(db, 1)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, uint64(1), retrieved.Id)
		assert.Equal(t, "testuser", retrieved.ScreenName)
		assert.Equal(t, "Test User", retrieved.Name)
		assert.True(t, retrieved.IsProtected)
		assert.Equal(t, 200, retrieved.FriendsCount)
		assert.False(t, retrieved.IsAccessible)
	})

	t.Run("get_nonexistent_user", func(t *testing.T) {
		retrieved, err := database.GetUserById(db, 99999)
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestSetUserAccessible_Basic(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	t.Run("set_accessible_true", func(t *testing.T) {
		usr := &database.User{
			Id:           1,
			ScreenName:   "user1",
			Name:         "User 1",
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: false,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		err = database.SetUserAccessible(db, 1, true)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserById(db, 1)
		assert.True(t, retrieved.IsAccessible)
	})

	t.Run("set_accessible_false", func(t *testing.T) {
		usr := &database.User{
			Id:           2,
			ScreenName:   "user2",
			Name:         "User 2",
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		err = database.SetUserAccessible(db, 2, false)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserById(db, 2)
		assert.False(t, retrieved.IsAccessible)
	})

	t.Run("set_accessible_nonexistent_user", func(t *testing.T) {
		err := database.SetUserAccessible(db, 99999, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSetUserAccessibleByScreenName(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	t.Run("set_accessible_by_screen_name", func(t *testing.T) {
		usr := &database.User{
			Id:           1,
			ScreenName:   "testuser",
			Name:         "Test User",
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		err = database.SetUserAccessibleByScreenName(db, "testuser", false)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserById(db, 1)
		assert.False(t, retrieved.IsAccessible)
	})

	t.Run("set_accessible_by_nonexistent_screen_name", func(t *testing.T) {
		err := database.SetUserAccessibleByScreenName(db, "nonexistent", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUpdateUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	t.Run("update_all_fields", func(t *testing.T) {
		usr := &database.User{
			Id:           1,
			ScreenName:   "original",
			Name:         "Original Name",
			IsProtected:  false,
			FriendsCount: 100,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		usr.ScreenName = "updated"
		usr.Name = "Updated Name"
		usr.IsProtected = true
		usr.FriendsCount = 200
		usr.IsAccessible = false

		err = database.UpdateUser(db, usr)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserById(db, 1)
		assert.Equal(t, "updated", retrieved.ScreenName)
		assert.Equal(t, "Updated Name", retrieved.Name)
		assert.True(t, retrieved.IsProtected)
		assert.Equal(t, 200, retrieved.FriendsCount)
		assert.False(t, retrieved.IsAccessible)
	})

	t.Run("update_nonexistent_user", func(t *testing.T) {
		usr := &database.User{
			Id:           99999,
			ScreenName:   "ghost",
			Name:         "Ghost User",
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: true,
		}
		err := database.UpdateUser(db, usr)
		assert.NoError(t, err)

		retrieved, _ := database.GetUserById(db, 99999)
		assert.Nil(t, retrieved)
	})
}

func TestSetUsersAccessible(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	for i := 1; i <= 5; i++ {
		usr := &database.User{
			Id:           uint64(i),
			ScreenName:   "user" + string(rune('0'+i)),
			Name:         "User " + string(rune('0'+i)),
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: false,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)
	}

	t.Run("batch_set_accessible", func(t *testing.T) {
		uids := []uint64{1, 3, 5}
		err := database.SetUsersAccessible(db, uids)
		assert.NoError(t, err)

		for i := 1; i <= 5; i++ {
			retrieved, _ := database.GetUserById(db, uint64(i))
			if i%2 == 1 {
				assert.True(t, retrieved.IsAccessible, "user %d should be accessible", i)
			} else {
				assert.False(t, retrieved.IsAccessible, "user %d should not be accessible", i)
			}
		}
	})

	t.Run("empty_uid_list", func(t *testing.T) {
		err := database.SetUsersAccessible(db, []uint64{})
		assert.NoError(t, err)
	})

	t.Run("nil_uid_list", func(t *testing.T) {
		err := database.SetUsersAccessible(db, nil)
		assert.NoError(t, err)
	})
}

func TestMarkListMembersAccessibleByIDs(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	for i := 1; i <= 3; i++ {
		usr := &database.User{
			Id:           uint64(i),
			ScreenName:   "user" + string(rune('0'+i)),
			Name:         "User " + string(rune('0'+i)),
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: false,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)
	}

	t.Run("mark_members_accessible", func(t *testing.T) {
		uids := []uint64{1, 2, 3}
		database.MarkListMembersAccessibleByIDs(db, uids)

		time.Sleep(100 * time.Millisecond)

		for i := 1; i <= 3; i++ {
			retrieved, _ := database.GetUserById(db, uint64(i))
			assert.True(t, retrieved.IsAccessible, "user %d should be accessible", i)
		}
	})

	t.Run("empty_uid_list", func(t *testing.T) {
		database.MarkListMembersAccessibleByIDs(db, []uint64{})
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("nil_db", func(t *testing.T) {
		uids := []uint64{1, 2, 3}
		database.MarkListMembersAccessibleByIDs(nil, uids)
	})
}

func TestRecordUserPreviousName(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	usr := &database.User{
		Id:           1,
		ScreenName:   "current",
		Name:         "Current Name",
		IsProtected:  false,
		FriendsCount: 0,
		IsAccessible: true,
	}
	err := database.CreateUser(db, usr)
	require.NoError(t, err)

	t.Run("record_previous_name", func(t *testing.T) {
		err := database.RecordUserPreviousName(db, 1, "Old Name", "oldname")
		assert.NoError(t, err)

		names, err := database.QueryUserPreviousNames(db, 1, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, names, 1)
		assert.Equal(t, "Old Name", names[0].Name)
		assert.Equal(t, "oldname", names[0].ScreenName)
		assert.Equal(t, uint64(1), names[0].Uid)
	})

	t.Run("record_multiple_names", func(t *testing.T) {
		err := database.RecordUserPreviousName(db, 1, "Name 1", "name1")
		assert.NoError(t, err)
		err = database.RecordUserPreviousName(db, 1, "Name 2", "name2")
		assert.NoError(t, err)

		names, err := database.QueryUserPreviousNames(db, 1, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, names, 3)
	})
}

func TestMarkUserInaccessible(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	t.Run("mark_by_uid", func(t *testing.T) {
		usr := &database.User{
			Id:           1,
			ScreenName:   "testuser",
			Name:         "Test User",
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		database.MarkUserInaccessible(db, 1, "")

		retrieved, _ := database.GetUserById(db, 1)
		assert.False(t, retrieved.IsAccessible)
	})

	t.Run("mark_by_screen_name", func(t *testing.T) {
		usr := &database.User{
			Id:           2,
			ScreenName:   "testuser2",
			Name:         "Test User 2",
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		database.MarkUserInaccessible(db, 0, "testuser2")

		retrieved, _ := database.GetUserById(db, 2)
		assert.False(t, retrieved.IsAccessible)
	})

	t.Run("mark_nonexistent_by_uid", func(t *testing.T) {
		database.MarkUserInaccessible(db, 99999, "")
	})

	t.Run("mark_nonexistent_by_screen_name", func(t *testing.T) {
		database.MarkUserInaccessible(db, 0, "nonexistent")
	})

	t.Run("no_uid_or_screen_name", func(t *testing.T) {
		database.MarkUserInaccessible(db, 0, "")
	})
}
