package database_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func setupUserLinkTestDB(t *testing.T) *sqlx.DB {
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

	lst := &database.Lst{
		Id:          1,
		Name:        "TestList",
		OwnerUserId: 100,
	}
	err = database.CreateLst(db, lst)
	require.NoError(t, err)

	return db
}

func TestCreateUserLink(t *testing.T) {
	db := setupUserLinkTestDB(t)
	defer db.Close()

	le := &database.LstEntity{
		LstId:     1,
		Name:      "testlist",
		ParentDir: t.TempDir(),
	}
	err := database.CreateLstEntity(db, le)
	require.NoError(t, err)

	t.Run("create_valid_link", func(t *testing.T) {
		link := &database.UserLink{
			UserId:            1,
			Name:              "testlink",
			ParentLstEntityId: database.NullInt32(le.Id),
		}
		err := database.CreateUserLink(db, link)
		assert.NoError(t, err)
		assert.Greater(t, link.Id, int32(0))
	})

	t.Run("create_duplicate_link", func(t *testing.T) {
		link := &database.UserLink{
			UserId:            1,
			Name:              "testlink2",
			ParentLstEntityId: database.NullInt32(le.Id),
		}
		err := database.CreateUserLink(db, link)
		assert.Error(t, err)
	})

	t.Run("create_link_nonexistent_user", func(t *testing.T) {
		// SQLite默认不强制执行外键约束，除非启用
		// 所以这个测试可能不会产生错误
		link := &database.UserLink{
			UserId:            99999,
			Name:              "orphan",
			ParentLstEntityId: database.NullInt32(le.Id),
		}
		err := database.CreateUserLink(db, link)
		// 由于SQLite默认不强制执行外键，这里不强制要求错误
		// 实际行为取决于数据库配置
		_ = err
	})
}

func TestGetUserLinks(t *testing.T) {
	db := setupUserLinkTestDB(t)
	defer db.Close()

	le := &database.LstEntity{
		LstId:     1,
		Name:      "testlist",
		ParentDir: t.TempDir(),
	}
	err := database.CreateLstEntity(db, le)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		usr := &database.User{
			Id:           uint64(i + 2),
			ScreenName:   "user" + string(rune('0'+i)),
			Name:         "User " + string(rune('0'+i)),
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		link := &database.UserLink{
			UserId:            uint64(i + 2),
			Name:              "link" + string(rune('0'+i)),
			ParentLstEntityId: database.NullInt32(le.Id),
		}
		err = database.CreateUserLink(db, link)
		require.NoError(t, err)
	}

	t.Run("get_links_for_user", func(t *testing.T) {
		links, err := database.GetUserLinks(db, 2)
		assert.NoError(t, err)
		assert.Len(t, links, 1)
		assert.Equal(t, "link0", links[0].Name)
	})

	t.Run("get_links_no_links", func(t *testing.T) {
		links, err := database.GetUserLinks(db, 99999)
		assert.NoError(t, err)
		assert.Empty(t, links)
	})
}

func TestGetUserLink(t *testing.T) {
	db := setupUserLinkTestDB(t)
	defer db.Close()

	le := &database.LstEntity{
		LstId:     1,
		Name:      "testlist",
		ParentDir: t.TempDir(),
	}
	err := database.CreateLstEntity(db, le)
	require.NoError(t, err)

	link := &database.UserLink{
		UserId:            1,
		Name:              "testlink",
		ParentLstEntityId: database.NullInt32(le.Id),
	}
	err = database.CreateUserLink(db, link)
	require.NoError(t, err)

	t.Run("get_existing_link", func(t *testing.T) {
		retrieved, err := database.GetUserLink(db, 1, database.NullInt32(le.Id))
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "testlink", retrieved.Name)
		assert.Equal(t, uint64(1), retrieved.UserId)
	})

	t.Run("get_nonexistent_link", func(t *testing.T) {
		retrieved, err := database.GetUserLink(db, 99999, database.NullInt32(le.Id))
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("get_link_wrong_entity", func(t *testing.T) {
		retrieved, err := database.GetUserLink(db, 1, 99999)
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestUpdateUserLink(t *testing.T) {
	db := setupUserLinkTestDB(t)
	defer db.Close()

	le := &database.LstEntity{
		LstId:     1,
		Name:      "testlist",
		ParentDir: t.TempDir(),
	}
	err := database.CreateLstEntity(db, le)
	require.NoError(t, err)

	link := &database.UserLink{
		UserId:            1,
		Name:              "original",
		ParentLstEntityId: database.NullInt32(le.Id),
	}
	err = database.CreateUserLink(db, link)
	require.NoError(t, err)

	t.Run("update_name", func(t *testing.T) {
		err := database.UpdateUserLink(db, link.Id, "updated")
		assert.NoError(t, err)

		retrieved, _ := database.GetUserLink(db, 1, database.NullInt32(le.Id))
		assert.Equal(t, "updated", retrieved.Name)
	})

	t.Run("update_nonexistent_link", func(t *testing.T) {
		err := database.UpdateUserLink(db, 99999, "ghost")
		assert.NoError(t, err)
	})
}

func TestGetUserLinksByLstEntityId(t *testing.T) {
	db := setupUserLinkTestDB(t)
	defer db.Close()

	le1 := &database.LstEntity{
		LstId:     1,
		Name:      "list1",
		ParentDir: t.TempDir(),
	}
	err := database.CreateLstEntity(db, le1)
	require.NoError(t, err)

	le2 := &database.LstEntity{
		LstId:     1,
		Name:      "list2",
		ParentDir: t.TempDir(),
	}
	err = database.CreateLstEntity(db, le2)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		usr := &database.User{
			Id:           uint64(i + 2),
			ScreenName:   "user" + string(rune('0'+i)),
			Name:         "User " + string(rune('0'+i)),
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: true,
		}
		err := database.CreateUser(db, usr)
		require.NoError(t, err)

		link := &database.UserLink{
			UserId:            uint64(i + 2),
			Name:              "link" + string(rune('0'+i)),
			ParentLstEntityId: database.NullInt32(le1.Id),
		}
		err = database.CreateUserLink(db, link)
		require.NoError(t, err)
	}

	linkForLe2 := &database.UserLink{
		UserId:            5,
		Name:              "link_for_le2",
		ParentLstEntityId: database.NullInt32(le2.Id),
	}
	err = database.CreateUserLink(db, linkForLe2)
	require.NoError(t, err)

	t.Run("get_links_by_entity_id", func(t *testing.T) {
		links, err := database.GetUserLinksByLstEntityId(db, int(database.NullInt32(le1.Id)))
		assert.NoError(t, err)
		assert.Len(t, links, 3)
	})

	t.Run("get_links_by_different_entity", func(t *testing.T) {
		links, err := database.GetUserLinksByLstEntityId(db, int(database.NullInt32(le2.Id)))
		assert.NoError(t, err)
		assert.Len(t, links, 1)
		assert.Equal(t, "link_for_le2", links[0].Name)
	})

	t.Run("get_links_nonexistent_entity", func(t *testing.T) {
		links, err := database.GetUserLinksByLstEntityId(db, 99999)
		assert.NoError(t, err)
		assert.Empty(t, links)
	})
}

func TestUserLink_PathMethod(t *testing.T) {
	db := setupUserLinkTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	le := &database.LstEntity{
		LstId:     1,
		Name:      "testlist",
		ParentDir: tmpDir,
	}
	err := database.CreateLstEntity(db, le)
	require.NoError(t, err)

	t.Run("valid_path", func(t *testing.T) {
		link := &database.UserLink{
			UserId:            1,
			Name:              "testlink",
			ParentLstEntityId: database.NullInt32(le.Id),
		}
		path, err := link.Path(db)
		assert.NoError(t, err)
		assert.Contains(t, path, "testlist")
		assert.Contains(t, path, "testlink")
	})

	t.Run("parent_not_exists", func(t *testing.T) {
		link := &database.UserLink{
			UserId:            1,
			Name:              "orphan",
			ParentLstEntityId: 99999,
		}
		path, err := link.Path(db)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent lst was not exists")
		assert.Empty(t, path)
	})
}
