package database_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func setupQueryTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	require.NoError(t, err)
	database.CreateTables(db)

	for i := 0; i < 10; i++ {
		usr := &database.User{
			Id:           uint64(i + 1),
			ScreenName:   "user" + string(rune('0'+i)),
			Name:         "User " + string(rune('0'+i)),
			IsProtected:  i%2 == 0,
			FriendsCount: (i + 1) * 10,
			IsAccessible: i%3 != 0,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)
	}

	for i := 0; i < 5; i++ {
		lst := &database.Lst{
			Id:          uint64(i + 1),
			Name:        "List " + string(rune('0'+i)),
			OwnerUserId: uint64(i + 100),
		}
		err := database.CreateLst(db, lst)
		require.NoError(t, err)
	}

	return db
}

func TestCount(t *testing.T) {
	db := setupQueryTestDB(t)
	defer db.Close()

	t.Run("count_all", func(t *testing.T) {
		count, err := database.Count(db, "users", nil)
		assert.NoError(t, err)
		assert.Equal(t, 10, count)
	})

	t.Run("count_with_where", func(t *testing.T) {
		opts := &database.QueryOptions{
			Where: "is_accessible = ?",
			Args:  []interface{}{true},
		}
		count, err := database.Count(db, "users", opts)
		assert.NoError(t, err)
		// Users 0..9: IsAccessible = i%3 != 0 → accessible count = 10 - 4 = 6
		accessibleCount := 0
		for i := 0; i < 10; i++ {
			if i%3 != 0 {
				accessibleCount++
			}
		}
		assert.Equal(t, accessibleCount, count)
	})

	t.Run("count_empty_table", func(t *testing.T) {
		count, err := database.Count(db, "user_entities", nil)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestBuildSearchCondition(t *testing.T) {
	t.Run("single_field", func(t *testing.T) {
		condition, args := database.BuildSearchCondition([]string{"name"}, "test")
		assert.Equal(t, "(name LIKE ?)", condition)
		assert.Equal(t, []interface{}{"%test%"}, args)
	})

	t.Run("multiple_fields", func(t *testing.T) {
		condition, args := database.BuildSearchCondition([]string{"name", "screen_name", "email"}, "john")
		assert.Equal(t, "(name LIKE ? OR screen_name LIKE ? OR email LIKE ?)", condition)
		assert.Equal(t, []interface{}{"%john%", "%john%", "%john%"}, args)
	})

	t.Run("empty_keyword", func(t *testing.T) {
		condition, args := database.BuildSearchCondition([]string{"name"}, "")
		assert.Empty(t, condition)
		assert.Nil(t, args)
	})

	t.Run("empty_fields", func(t *testing.T) {
		condition, args := database.BuildSearchCondition([]string{}, "test")
		assert.Empty(t, condition)
		assert.Nil(t, args)
	})

	t.Run("nil_fields", func(t *testing.T) {
		condition, args := database.BuildSearchCondition(nil, "test")
		assert.Empty(t, condition)
		assert.Nil(t, args)
	})
}

func TestQueryUsers(t *testing.T) {
	db := setupQueryTestDB(t)
	defer db.Close()

	t.Run("query_all", func(t *testing.T) {
		users, err := database.QueryUsers(db, "", nil, "ORDER BY id", 5, 0)
		assert.NoError(t, err)
		assert.Len(t, users, 5)
	})

	t.Run("query_with_where", func(t *testing.T) {
		users, err := database.QueryUsers(db, "is_accessible = ?", []interface{}{true}, "ORDER BY id", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, users, 6)
		for _, u := range users {
			assert.True(t, u.IsAccessible)
		}
	})

	t.Run("query_with_pagination", func(t *testing.T) {
		users, err := database.QueryUsers(db, "", nil, "ORDER BY id", 3, 3)
		assert.NoError(t, err)
		assert.Len(t, users, 3)
		assert.Equal(t, uint64(4), users[0].Id)
	})

	t.Run("query_no_results", func(t *testing.T) {
		users, err := database.QueryUsers(db, "id > ?", []interface{}{1000}, "", 10, 0)
		assert.NoError(t, err)
		assert.Empty(t, users)
	})
}

func TestQueryLists(t *testing.T) {
	db := setupQueryTestDB(t)
	defer db.Close()

	t.Run("query_all", func(t *testing.T) {
		lists, err := database.QueryLists(db, "", nil, "ORDER BY id", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, lists, 5)
	})

	t.Run("query_with_where", func(t *testing.T) {
		lists, err := database.QueryLists(db, "owner_user_id = ?", []interface{}{uint64(100)}, "", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, lists, 1)
		assert.Equal(t, "List 0", lists[0].Name)
	})

	t.Run("query_with_pagination", func(t *testing.T) {
		lists, err := database.QueryLists(db, "", nil, "ORDER BY id", 2, 2)
		assert.NoError(t, err)
		assert.Len(t, lists, 2)
		assert.Equal(t, uint64(3), lists[0].Id)
	})
}

func TestQueryUserEntities(t *testing.T) {
	db := setupQueryTestDB(t)
	defer db.Close()

	for i := 0; i < 5; i++ {
		entity := &database.UserEntity{
			UserId:    uint64(i + 1),
			Name:      "entity" + string(rune('0'+i)),
			ParentDir: "/tmp/test",
		}
		err := database.CreateUserEntity(db, entity)
		require.NoError(t, err)
	}

	t.Run("query_all", func(t *testing.T) {
		entities, err := database.QueryUserEntities(db, "", nil, "ORDER BY id", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, entities, 5)
	})

	t.Run("query_with_where", func(t *testing.T) {
		entities, err := database.QueryUserEntities(db, "user_id = ?", []interface{}{uint64(1)}, "", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, entities, 1)
	})

	t.Run("query_empty", func(t *testing.T) {
		entities, err := database.QueryUserEntities(db, "user_id > ?", []interface{}{1000}, "", 10, 0)
		assert.NoError(t, err)
		assert.Empty(t, entities)
	})
}

func TestQueryLstEntities(t *testing.T) {
	db := setupQueryTestDB(t)
	defer db.Close()

	for i := 0; i < 3; i++ {
		entity := &database.LstEntity{
			LstId:     int64(i + 1),
			Name:      "lst_entity" + string(rune('0'+i)),
			ParentDir: "/tmp/lists",
		}
		err := database.CreateLstEntity(db, entity)
		require.NoError(t, err)
	}

	t.Run("query_all", func(t *testing.T) {
		entities, err := database.QueryLstEntities(db, "", nil, "ORDER BY id", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, entities, 3)
	})

	t.Run("query_with_where", func(t *testing.T) {
		entities, err := database.QueryLstEntities(db, "lst_id = ?", []interface{}{int64(1)}, "", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, entities, 1)
	})
}

func TestQueryUserLinks(t *testing.T) {
	db := setupQueryTestDB(t)
	defer db.Close()

	// 使用新的list ID避免冲突
	lst := &database.Lst{Id: 999, Name: "TestList999", OwnerUserId: 100}
	err := database.CreateLst(db, lst)
	require.NoError(t, err)

	le := &database.LstEntity{
		LstId:     999,
		Name:      "list_entity",
		ParentDir: t.TempDir(),
	}
	err = database.CreateLstEntity(db, le)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		link := &database.UserLink{
			UserId:            uint64(i + 1),
			Name:              "link" + string(rune('0'+i)),
			ParentLstEntityId: database.NullInt32(le.Id),
		}
		err := database.CreateUserLink(db, link)
		require.NoError(t, err)
	}

	t.Run("query_all", func(t *testing.T) {
		links, err := database.QueryUserLinks(db, "", nil, "", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, links, 5)
	})

	t.Run("query_with_where", func(t *testing.T) {
		links, err := database.QueryUserLinks(db, "user_id = ?", []interface{}{uint64(1)}, "", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, links, 1)
		assert.Equal(t, "link0", links[0].Name)
	})
}

func TestQueryUserPreviousNames(t *testing.T) {
	db := setupQueryTestDB(t)
	defer db.Close()

	usr := &database.User{
		Id:           999,
		ScreenName:   "current",
		Name:         "Current Name",
		IsProtected:  false,
		FriendsCount: 0,
		IsAccessible: true,
	}
	err := database.CreateUser(db, usr)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		err := database.RecordUserPreviousName(db, 999, "Old Name "+string(rune('0'+i)), "oldname"+string(rune('0'+i)))
		require.NoError(t, err)
	}

	t.Run("query_all", func(t *testing.T) {
		names, err := database.QueryUserPreviousNames(db, 999, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, names, 5)
	})

	t.Run("query_with_limit", func(t *testing.T) {
		names, err := database.QueryUserPreviousNames(db, 999, 3, 0)
		assert.NoError(t, err)
		assert.Len(t, names, 3)
	})

	t.Run("query_with_offset", func(t *testing.T) {
		names, err := database.QueryUserPreviousNames(db, 999, 10, 3)
		assert.NoError(t, err)
		assert.Len(t, names, 2)
	})

	t.Run("query_nonexistent_user", func(t *testing.T) {
		names, err := database.QueryUserPreviousNames(db, 88888, 10, 0)
		assert.NoError(t, err)
		assert.Empty(t, names)
	})
}
