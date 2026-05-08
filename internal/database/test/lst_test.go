package database_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/database"
)

func setupLstTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	require.NoError(t, err)
	database.CreateTables(db)
	return db
}

func TestCreateLst(t *testing.T) {
	db := setupLstTestDB(t)
	defer db.Close()

	t.Run("create_valid_list", func(t *testing.T) {
		lst := &database.Lst{
			Id:          1,
			Name:        "Test List",
			OwnerUserId: 100,
		}
		err := database.CreateLst(db, lst)
		assert.NoError(t, err)
	})

	t.Run("create_duplicate_list", func(t *testing.T) {
		lst := &database.Lst{
			Id:          1,
			Name:        "Another List",
			OwnerUserId: 200,
		}
		err := database.CreateLst(db, lst)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create list")
	})

	t.Run("create_multiple_lists", func(t *testing.T) {
		for i := 2; i <= 5; i++ {
			lst := &database.Lst{
				Id:          uint64(i),
				Name:        "List " + string(rune('0'+i)),
				OwnerUserId: uint64(i * 100),
			}
			err := database.CreateLst(db, lst)
			assert.NoError(t, err)
		}
	})
}

func TestGetLst(t *testing.T) {
	db := setupLstTestDB(t)
	defer db.Close()

	lst := &database.Lst{
		Id:          1,
		Name:        "Test List",
		OwnerUserId: 100,
	}
	err := database.CreateLst(db, lst)
	require.NoError(t, err)

	t.Run("get_existing_list", func(t *testing.T) {
		retrieved, err := database.GetLst(db, 1)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, uint64(1), retrieved.Id)
		assert.Equal(t, "Test List", retrieved.Name)
		assert.Equal(t, uint64(100), retrieved.OwnerUserId)
	})

	t.Run("get_nonexistent_list", func(t *testing.T) {
		retrieved, err := database.GetLst(db, 99999)
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

func TestUpdateLst(t *testing.T) {
	db := setupLstTestDB(t)
	defer db.Close()

	lst := &database.Lst{
		Id:          1,
		Name:        "Original Name",
		OwnerUserId: 100,
	}
	err := database.CreateLst(db, lst)
	require.NoError(t, err)

	t.Run("update_name", func(t *testing.T) {
		lst.Name = "Updated Name"
		err := database.UpdateLst(db, lst)
		assert.NoError(t, err)

		retrieved, _ := database.GetLst(db, 1)
		assert.Equal(t, "Updated Name", retrieved.Name)
		assert.Equal(t, uint64(100), retrieved.OwnerUserId)
	})

	t.Run("update_nonexistent_list", func(t *testing.T) {
		nonexistent := &database.Lst{
			Id:          99999,
			Name:        "Ghost List",
			OwnerUserId: 999,
		}
		err := database.UpdateLst(db, nonexistent)
		assert.NoError(t, err)

		retrieved, _ := database.GetLst(db, 99999)
		assert.Nil(t, retrieved)
	})
}

func TestLst_CRUD(t *testing.T) {
	db := setupLstTestDB(t)
	defer db.Close()

	t.Run("full_crud_lifecycle", func(t *testing.T) {
		lst := &database.Lst{
			Id:          42,
			Name:        "Lifecycle List",
			OwnerUserId: 999,
		}

		err := database.CreateLst(db, lst)
		require.NoError(t, err)

		retrieved, err := database.GetLst(db, 42)
		require.NoError(t, err)
		assert.Equal(t, "Lifecycle List", retrieved.Name)
		assert.Equal(t, uint64(999), retrieved.OwnerUserId)

		lst.Name = "Updated Lifecycle List"
		err = database.UpdateLst(db, lst)
		require.NoError(t, err)

		retrieved, _ = database.GetLst(db, 42)
		assert.Equal(t, "Updated Lifecycle List", retrieved.Name)
	})
}

func TestLst_Struct(t *testing.T) {
	lst := database.Lst{
		Id:          1,
		Name:        "Test List",
		OwnerUserId: 100,
	}

	assert.Equal(t, uint64(1), lst.Id)
	assert.Equal(t, "Test List", lst.Name)
	assert.Equal(t, uint64(100), lst.OwnerUserId)
}
