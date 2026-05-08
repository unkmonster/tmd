package database_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	require.NoError(t, err)
	database.CreateTables(db)
	return db
}

func TestUserEntity_Path(t *testing.T) {
	t.Run("valid_path", func(t *testing.T) {
		ue := &database.UserEntity{
			Name:      "testuser",
			ParentDir: "/tmp/test",
		}
		path, err := ue.Path()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join("/tmp/test", "testuser"), path)
	})

	t.Run("empty_parent_dir", func(t *testing.T) {
		ue := &database.UserEntity{
			Name:      "testuser",
			ParentDir: "",
		}
		path, err := ue.Path()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no enough info")
		assert.Empty(t, path)
	})

	t.Run("empty_name", func(t *testing.T) {
		ue := &database.UserEntity{
			Name:      "",
			ParentDir: "/tmp/test",
		}
		path, err := ue.Path()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no enough info")
		assert.Empty(t, path)
	})
}

func TestLstEntity_Path(t *testing.T) {
	t.Run("valid_path", func(t *testing.T) {
		le := &database.LstEntity{
			Name:      "testlist",
			ParentDir: "/tmp/lists",
		}
		path, err := le.Path()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join("/tmp/lists", "testlist"), path)
	})

	t.Run("empty_parent_dir", func(t *testing.T) {
		le := &database.LstEntity{
			Name:      "testlist",
			ParentDir: "",
		}
		path, err := le.Path()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no enough info")
		assert.Empty(t, path)
	})

	t.Run("empty_name", func(t *testing.T) {
		le := &database.LstEntity{
			Name:      "",
			ParentDir: "/tmp/lists",
		}
		path, err := le.Path()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no enough info")
		assert.Empty(t, path)
	})
}

func TestUserLink_Path(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Run("valid_path", func(t *testing.T) {
		lst := &database.Lst{Id: 1, Name: "TestList", OwnerUserId: 100}
		err := database.CreateLst(db, lst)
		require.NoError(t, err)

		le := &database.LstEntity{
			LstId:     1,
			Name:      "list_entity",
			ParentDir: "/tmp/lists",
		}
		err = database.CreateLstEntity(db, le)
		require.NoError(t, err)

		ul := &database.UserLink{
			UserId:            200,
			Name:              "user_link",
			ParentLstEntityId: database.NullInt32(le.Id),
		}

		path, err := ul.Path(db)
		assert.NoError(t, err)
		assert.Contains(t, path, "list_entity")
		assert.Contains(t, path, "user_link")
	})

	t.Run("parent_not_exists", func(t *testing.T) {
		ul := &database.UserLink{
			UserId:            200,
			Name:              "user_link",
			ParentLstEntityId: 99999,
		}

		path, err := ul.Path(db)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parent lst was not exists")
		assert.Empty(t, path)
	})

	t.Run("db_error", func(t *testing.T) {
		db.Close()

		ul := &database.UserLink{
			UserId:            200,
			Name:              "user_link",
			ParentLstEntityId: 1,
		}

		path, err := ul.Path(db)
		assert.Error(t, err)
		assert.Empty(t, path)
	})
}

func TestNullInt32(t *testing.T) {
	t.Run("valid_value", func(t *testing.T) {
		n := sql.NullInt32{
			Int32: 42,
			Valid: true,
		}
		assert.Equal(t, int32(42), database.NullInt32(n))
	})

	t.Run("null_value", func(t *testing.T) {
		n := sql.NullInt32{
			Int32: 42,
			Valid: false,
		}
		assert.Equal(t, int32(0), database.NullInt32(n))
	})
}

func TestUser_Struct(t *testing.T) {
	usr := database.User{
		Id:           1,
		ScreenName:   "testuser",
		Name:         "Test User",
		IsProtected:  true,
		FriendsCount: 100,
		IsAccessible: true,
	}

	assert.Equal(t, uint64(1), usr.Id)
	assert.Equal(t, "testuser", usr.ScreenName)
	assert.Equal(t, "Test User", usr.Name)
	assert.True(t, usr.IsProtected)
	assert.Equal(t, 100, usr.FriendsCount)
	assert.True(t, usr.IsAccessible)
}

func TestUserEntity_Struct(t *testing.T) {
	ue := database.UserEntity{
		Id:        sql.NullInt32{Int32: 1, Valid: true},
		UserId:    100,
		Name:      "testuser",
		ParentDir: "/tmp/test",
	}

	assert.Equal(t, int32(1), ue.Id.Int32)
	assert.True(t, ue.Id.Valid)
	assert.Equal(t, uint64(100), ue.UserId)
	assert.Equal(t, "testuser", ue.Name)
	assert.Equal(t, "/tmp/test", ue.ParentDir)
}

func TestUserLink_Struct(t *testing.T) {
	ul := database.UserLink{
		Id:                1,
		UserId:            100,
		Name:              "testlink",
		ParentLstEntityId: 50,
	}

	assert.Equal(t, int32(1), ul.Id)
	assert.Equal(t, uint64(100), ul.UserId)
	assert.Equal(t, "testlink", ul.Name)
	assert.Equal(t, int32(50), ul.ParentLstEntityId)
}

func TestUserPreviousName_Struct(t *testing.T) {
	upn := database.UserPreviousName{
		Id:         1,
		UserId:     100,
		ScreenName: "oldname",
		Name:       "Old Name",
	}

	assert.Equal(t, int32(1), upn.Id)
	assert.Equal(t, uint64(100), upn.UserId)
	assert.Equal(t, "oldname", upn.ScreenName)
	assert.Equal(t, "Old Name", upn.Name)
}

func TestLstModel_Struct(t *testing.T) {
	lst := database.Lst{
		Id:          1,
		Name:        "Test List",
		OwnerUserId: 100,
	}

	assert.Equal(t, uint64(1), lst.Id)
	assert.Equal(t, "Test List", lst.Name)
	assert.Equal(t, uint64(100), lst.OwnerUserId)
}

func TestLstEntity_Struct(t *testing.T) {
	le := database.LstEntity{
		Id:        sql.NullInt32{Int32: 1, Valid: true},
		LstId:     100,
		Name:      "testlist",
		ParentDir: "/tmp/lists",
	}

	assert.Equal(t, int32(1), le.Id.Int32)
	assert.True(t, le.Id.Valid)
	assert.Equal(t, int64(100), le.LstId)
	assert.Equal(t, "testlist", le.Name)
	assert.Equal(t, "/tmp/lists", le.ParentDir)
}

func TestUserEntity_Path_AbsoluteConversion(t *testing.T) {
	ue := &database.UserEntity{
		Name:      "testuser",
		ParentDir: ".",
	}

	path, err := ue.Path()
	assert.NoError(t, err)

	// 在Windows上路径格式可能不同，所以只检查是否包含文件名
	assert.Contains(t, path, "testuser")
	assert.NotEqual(t, ".", path)
}
