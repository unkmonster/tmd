package database_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/unkmonster/tmd/internal/database"
)

var db *sqlx.DB

func opentmpdb() *sqlx.DB {
	var err error
	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	path := tmpFile.Name()

	db, err = sqlx.Connect("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&cache=shared", path))

	if err != nil {
		panic(err)
	}

	database.CreateTables(db)
	return db
}

func generateUser(n int) *database.User {
	usr := &database.User{}
	usr.Id = uint64(n)
	name := fmt.Sprintf("user%d", n)
	usr.ScreenName = name
	usr.Name = name
	usr.IsAccessible = true
	return usr
}
func TestUserOperation(t *testing.T) {
	db = opentmpdb()
	defer db.Close()

	n := 100
	users := make([]*database.User, n)
	for i := 0; i < n; i++ {
		users[i] = generateUser(i)
	}

	for _, usr := range users {
		testUser(t, usr)
	}
}

func testUser(t *testing.T, usr *database.User) {
	// create
	if err := database.CreateUser(db, usr); err != nil {
		t.Error(err)
		return
	}

	same, err := hasSameUserRecord(usr)
	if err != nil {
		t.Error(err)
		return
	}
	if !same {
		t.Error("record mismatch after create user")
		return
	}

	// update
	usr.Name = "renamed"
	if err := database.UpdateUser(db, usr); err != nil {
		t.Error(err)
		return
	}

	same, err = hasSameUserRecord(usr)
	if err != nil {
		t.Error(err)
		return
	}
	if !same {
		t.Error("record mismatch after update user")
		return
	}

	// record previous name
	if err = database.RecordUserPreviousName(db, usr.Id, usr.Name, usr.ScreenName); err != nil {
		t.Error(err)
		return
	}
}

func hasSameUserRecord(usr *database.User) (bool, error) {
	retrieved, err := database.GetUserById(db, usr.Id)
	return retrieved != nil && *retrieved == *usr, err
}

func generateList(id int) *database.Lst {
	lst := &database.Lst{}
	lst.Id = uint64(id)
	lst.Name = fmt.Sprintf("lst%d", id)
	return lst
}

func TestList(t *testing.T) {
	db = opentmpdb()
	defer db.Close()
	n := 100
	lsts := make([]*database.Lst, n)
	for i := 0; i < n; i++ {
		lsts[i] = generateList(i)
	}

	for _, lst := range lsts {
		// create
		if err := database.CreateLst(db, lst); err != nil {
			t.Error(err)
			return
		}

		// read
		same, err := isSameLstRecord(lst)
		if err != nil {
			t.Error(err)
			return
		}
		if !same {
			t.Error("record mismatch after create lst")
			return
		}

		// update
		lst.Name = "renamed"
		if err = database.UpdateLst(db, lst); err != nil {
			t.Error(err)
			return
		}
		same, err = isSameLstRecord(lst)
		if err != nil {
			t.Error(err)
			return
		}
		if !same {
			t.Error("record mismatch after update lst")
			return
		}
	}
}

func isSameLstRecord(lst *database.Lst) (bool, error) {
	record, err := database.GetLst(db, lst.Id)
	return record != nil && *record == *lst, err
}

func TestUserEntity(t *testing.T) {
	db = opentmpdb()
	defer db.Close()
	n := 100
	entities := make([]*database.UserEntity, n)
	tempDir := os.TempDir()
	for i := 0; i < n; i++ {
		entities[i] = generateUserEntity(uint64(i), tempDir)
	}

	for _, entity := range entities {
		// path
		expectedPath := filepath.Join(tempDir, entity.Name)
		actualPath, err := entity.Path()
		if err != nil {
			t.Error(err)
			return
		}
		if expectedPath != actualPath {
			t.Errorf("entity.Path() = %v want %v", actualPath, expectedPath)
			return
		}

		// create
		if err := database.CreateUserEntity(db, entity); err != nil {
			t.Error(err)
			return
		}

		// read
		yes, err := hasSameUserEntityRecord(entity)
		if err != nil {
			t.Error(err)
			return
		}
		if !yes {
			t.Error("record mismatch after create user entity")
			return
		}

		// update
		entity.Name = entity.Name + "renamed"
		if err := database.UpdateUserEntity(db, entity); err != nil {
			t.Error(err)
			return
		}
		yes, err = hasSameUserEntityRecord(entity)
		if err != nil {
			t.Error(err)
			return
		}
		if !yes {
			t.Error("record mismatch after update user entity")
			return
		}

		// latest release time
		now := time.Now()
		if err = database.UpdateUserEntityTweetStat(db, int(database.NullInt32(entity.Id)), now, 25); err != nil {
			t.Error(err)
			return
		}
		entity.MediaCount.Scan(25)

		// locate
		record, err := database.LocateUserEntity(db, entity.Uid, tempDir)
		if err != nil {
			t.Error(err)
			return
		}
		if record == nil {
			t.Error("record mismatch on locate user entity")
			return
		}
		// 单独比较时间字段
		if !record.LatestReleaseTime.Time.Equal(now) {
			t.Errorf("recorded latest release time: %v want %v", record.LatestReleaseTime.Time, now)
		}
		record.LatestReleaseTime = sql.NullTime{}
		entity.LatestReleaseTime = sql.NullTime{}
		if *record != *entity {
			t.Error("record mismatch on locate user entity")
			return
		}

		// delete
		if err = database.DelUserEntity(db, uint32(database.NullInt32(entity.Id))); err != nil {
			t.Error(err)
			return
		}

		yes, err = hasSameUserEntityRecord(entity)
		if err != nil {
			t.Error(err)
			return
		}
		if yes {
			t.Error("record mismatch after delete user entity")
		}
	}
}

func generateUserEntity(uid uint64, pdir string) *database.UserEntity {
	ue := database.UserEntity{}
	user := generateUser(int(uid))
	if err := database.CreateUser(db, user); err != nil {
		panic(err)
	}

	ue.Name = user.Name
	ue.Uid = uid
	ue.ParentDir = pdir
	return &ue
}

func hasSameUserEntityRecord(entity *database.UserEntity) (bool, error) {
	record, err := database.GetUserEntity(db, int(database.NullInt32(entity.Id)))
	return record != nil && *record == *entity, err
}

func TestLstEntity(t *testing.T) {
	db = opentmpdb()
	defer db.Close()
	tempdir := os.TempDir()
	n := 100
	entities := make([]*database.LstEntity, n)
	for i := 0; i < n; i++ {
		entities[i] = generateLstEntity(int64(i), tempdir)
	}

	for _, entity := range entities {
		// path
		expectedPath := filepath.Join(tempdir, entity.Name)
		actualPath, err := entity.Path()
		if err != nil {
			t.Error(err)
			return
		}
		if expectedPath != actualPath {
			t.Errorf("entity.Path() = %v want %v", actualPath, expectedPath)
			return
		}
		// create
		if err := database.CreateLstEntity(db, entity); err != nil {
			t.Error(err)
			return
		}

		// read
		yes, err := hasSameLstEntityRecord(entity)
		if err != nil {
			t.Error(err)
			return
		}
		if !yes {
			t.Error("record mismatch after create lst entity")
		}

		// update
		entity.Name = entity.Name + "renamed"
		if err = database.UpdateLstEntity(db, entity); err != nil {
			t.Error(err)
			return
		}
		yes, err = hasSameLstEntityRecord(entity)
		if err != nil {
			t.Error(err)
			return
		}
		if !yes {
			t.Error("record mismatch after update lst entity")
			return
		}

		// locate
		record, err := database.LocateLstEntity(db, entity.LstId, entity.ParentDir)
		if err != nil {
			t.Error(err)
			return
		}
		if record == nil || *record != *entity {
			t.Error("record mismatch after locate lst entity")
			return
		}

		// delete
		if err = database.DelLstEntity(db, int(database.NullInt32(entity.Id))); err != nil {
			t.Error(err)
			return
		}
		yes, err = hasSameLstEntityRecord(entity)
		if err != nil {
			t.Error(err)
			return
		}
		if yes {
			t.Error("record mismatch after delete lst entity")
			return
		}
	}
}

func generateLstEntity(lid int64, pdir string) *database.LstEntity {
	lst := generateList(int(lid))
	if err := database.CreateLst(db, lst); err != nil {
		panic(err)
	}
	entity := database.LstEntity{}
	entity.LstId = lid
	entity.ParentDir = pdir
	entity.Name = lst.Name
	return &entity
}

func hasSameLstEntityRecord(entity *database.LstEntity) (bool, error) {
	record, err := database.GetLstEntity(db, int(database.NullInt32(entity.Id)))
	return record != nil && *record == *entity, err
}

func TestLink(t *testing.T) {
	db = opentmpdb()
	defer db.Close()
	n := 100
	links := make([]*database.UserLink, n)
	for i := 0; i < n; i++ {
		links[i] = generateLink(i, i)
	}

	for _, link := range links {
		// path
		le, err := database.GetLstEntity(db, int(link.ParentLstEntityId))
		if err != nil {
			t.Error(err)
			return
		}
		lePath, err := le.Path()
		if err != nil {
			t.Error(err)
			return
		}
		expectedPath := filepath.Join(lePath, link.Name)
		path, err := link.Path(db)
		if err != nil {
			t.Error(err)
			return
		}
		if expectedPath != path {
			t.Errorf("link.Path() = %v want %v", path, expectedPath)
			return
		}

		// c
		if err := database.CreateUserLink(db, link); err != nil {
			t.Error(err)
			return
		}

		// r
		yes, err := hasSameUserLinkRecord(link)
		if err != nil {
			t.Error(err)
			return
		}
		if !yes {
			t.Error("mismatch record after create user link")
			return
		}

		records, err := database.GetUserLinks(db, link.UserId)
		if err != nil {
			t.Error(err)
			return
		}
		if len(records) != 1 || *records[0] != *link {
			t.Error("mismatch record after get all user links")
			return
		}

		// u
		link.Name = link.Name + "renamed"
		if err = database.UpdateUserLink(db, link.Id, link.Name); err != nil {
			t.Error(err)
			return
		}
		yes, err = hasSameUserLinkRecord(link)
		if err != nil {
			t.Error(err)
			return
		}
		if !yes {
			t.Error("mismatch record after update user link")
			return
		}
	}
}

func generateLink(uid int, lid int) *database.UserLink {
	usr := generateUser(uid)
	le := generateLstEntity(int64(lid), os.TempDir())
	if err := database.CreateLstEntity(db, le); err != nil {
		panic(err)
	}

	ul := database.UserLink{}
	ul.Name = fmt.Sprintf("%d-%d", lid, uid)
	ul.ParentLstEntityId = database.NullInt32(le.Id)
	ul.UserId = usr.Id
	return &ul
}

func hasSameUserLinkRecord(link *database.UserLink) (bool, error) {
	record, err := database.GetUserLink(db, link.UserId, link.ParentLstEntityId)
	return record != nil && *record == *link, err
}

func benchmarkUpdateUser(b *testing.B, routines int) {
	db = opentmpdb()
	defer db.Close()

	n := 500
	users := make(chan *database.User, n)
	for i := 0; i < n; i++ {
		user := generateUser(i)
		if err := database.CreateUser(db, user); err != nil {
			b.Error(err)
			return
		}
		user.Name = user.Name + "renamed"
		users <- user
	}
	close(users)

	wg := sync.WaitGroup{}
	routine := func() {
		defer wg.Done()
		for user := range users {
			if err := database.UpdateUser(db, user); err != nil {
				b.Error(err)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < routines; j++ {
			wg.Add(1)
			go routine()
		}
		wg.Wait()
	}
}

func BenchmarkUpdateUser1(b *testing.B) {
	benchmarkUpdateUser(b, 1)
}

func BenchmarkUpdateUser6(b *testing.B) {
	benchmarkUpdateUser(b, 6)
}

func BenchmarkUpdateUser12(b *testing.B) {
	benchmarkUpdateUser(b, 12)
}

func BenchmarkUpdateUser24(b *testing.B) {
	benchmarkUpdateUser(b, 24)
}

func TestSetUserAccessible(t *testing.T) {
	db = opentmpdb()
	defer db.Close()

	t.Run("update_existing_user_to_inaccessible", func(t *testing.T) {
		usr := generateUser(1)
		if err := database.CreateUser(db, usr); err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		if err := database.SetUserAccessible(db, usr.Id, false); err != nil {
			t.Fatalf("SetUserAccessible failed: %v", err)
		}

		retrieved, err := database.GetUserById(db, usr.Id)
		if err != nil {
			t.Fatalf("GetUserById failed: %v", err)
		}
		if retrieved.IsAccessible != false {
			t.Errorf("IsAccessible = %v, want false", retrieved.IsAccessible)
		}
	})

	t.Run("update_existing_user_back_to_accessible", func(t *testing.T) {
		usr := generateUser(2)
		usr.IsAccessible = false
		if err := database.CreateUser(db, usr); err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		if err := database.SetUserAccessible(db, usr.Id, true); err != nil {
			t.Fatalf("SetUserAccessible failed: %v", err)
		}

		retrieved, err := database.GetUserById(db, usr.Id)
		if err != nil {
			t.Fatalf("GetUserById failed: %v", err)
		}
		if retrieved.IsAccessible != true {
			t.Errorf("IsAccessible = %v, want true", retrieved.IsAccessible)
		}
	})

	t.Run("error_when_user_not_exists", func(t *testing.T) {
		newUID := uint64(99999)

		err := database.SetUserAccessible(db, newUID, false)
		if err == nil {
			t.Fatal("expected error for non-existent user, got nil")
		}

		expectedMsg := "user 99999 not found"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("error message = %q, want to contain %q", err.Error(), expectedMsg)
		}

		// 确认用户确实没有被创建
		retrieved, err := database.GetUserById(db, newUID)
		if err != nil {
			t.Fatalf("GetUserById failed: %v", err)
		}
		if retrieved != nil {
			t.Error("user should not be created when SetUserAccessible returns error")
		}
	})

	t.Run("idempotent_on_same_value", func(t *testing.T) {
		usr := generateUser(3)
		if err := database.CreateUser(db, usr); err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		if err := database.SetUserAccessible(db, usr.Id, true); err != nil {
			t.Fatalf("first SetUserAccessible failed: %v", err)
		}
		if err := database.SetUserAccessible(db, usr.Id, true); err != nil {
			t.Fatalf("second SetUserAccessible (same value) failed: %v", err)
		}

		retrieved, err := database.GetUserById(db, usr.Id)
		if err != nil {
			t.Fatalf("GetUserById failed: %v", err)
		}
		if retrieved.IsAccessible != true {
			t.Errorf("IsAccessible = %v, want true after idempotent update", retrieved.IsAccessible)
		}
	})
}

func TestSetUserAccessibleConcurrent(t *testing.T) {
	db = opentmpdb()
	defer db.Close()

	const n = 100
	for i := 0; i < n; i++ {
		usr := &database.User{
			Id:           uint64(i),
			ScreenName:   fmt.Sprintf("concurrent_user_%d", i),
			Name:         fmt.Sprintf("Concurrent User %d", i),
			IsProtected:  false,
			FriendsCount: 0,
			IsAccessible: true,
		}
		if err := database.CreateUser(db, usr); err != nil {
			t.Fatalf("failed to pre-create user %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(uid uint64) {
			defer wg.Done()
			if err := database.SetUserAccessible(db, uid, uid%2 == 0); err != nil {
				t.Error(err)
			}
		}(uint64(i))
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		usr, err := database.GetUserById(db, uint64(i))
		if err != nil {
			t.Errorf("GetUserById(%d) failed: %v", i, err)
			continue
		}
		if usr == nil {
			t.Errorf("user %d should exist after concurrent SetUserAccessible", i)
			continue
		}
		expected := i%2 == 0
		if usr.IsAccessible != expected {
			t.Errorf("user %d: IsAccessible = %v, want %v", i, usr.IsAccessible, expected)
		}
	}
}

func TestMigrateDatabase(t *testing.T) {
	t.Run("migrate_old_table_without_is_accessible", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "")
		if err != nil {
			t.Fatal(err)
		}
		path := tmpFile.Name()
		tmpFile.Close()

		oldDB, err := sqlx.Connect("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&cache=shared", path))
		if err != nil {
			t.Fatal(err)
		}
		defer oldDB.Close()

		oldSchema := `
CREATE TABLE users (
	id INTEGER NOT NULL, 
	screen_name VARCHAR NOT NULL, 
	name VARCHAR NOT NULL, 
	protected BOOLEAN NOT NULL, 
	friends_count INTEGER NOT NULL, 
	PRIMARY KEY (id), 
	UNIQUE (screen_name)
);
`
		oldDB.MustExec(oldSchema)

		insertStmt := `INSERT INTO users(id, screen_name, name, protected, friends_count) VALUES(1, 'olduser', 'Old User', 0, 100)`
		oldDB.MustExec(insertStmt)

		var accessibleExists int
		err = oldDB.Get(&accessibleExists, "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='is_accessible'")
		if err != nil {
			t.Fatal(err)
		}
		if accessibleExists != 0 {
			t.Fatal("old table should not have is_accessible column before migration")
		}

		if err := database.MigrateDatabase(oldDB); err != nil {
			t.Fatalf("MigrateDatabase failed: %v", err)
		}

		err = oldDB.Get(&accessibleExists, "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='is_accessible'")
		if err != nil {
			t.Fatal(err)
		}
		if accessibleExists != 1 {
			t.Fatal("is_accessible column should exist after migration")
		}

		var isAccessible bool
		err = oldDB.Get(&isAccessible, "SELECT is_accessible FROM users WHERE id=1")
		if err != nil {
			t.Fatalf("failed to query is_accessible after migration: %v", err)
		}
		if !isAccessible {
			t.Errorf("existing row should have is_accessible=true (DEFAULT), got %v", isAccessible)
		}
	})

	t.Run("idempotent_migration", func(t *testing.T) {
		testDB := opentmpdb()
		defer testDB.Close()

		if err := database.MigrateDatabase(testDB); err != nil {
			t.Fatalf("first MigrateDatabase failed: %v", err)
		}
		if err := database.MigrateDatabase(testDB); err != nil {
			t.Fatalf("second MigrateDatabase (idempotent) failed: %v", err)
		}

		usr := generateUser(42)
		if err := database.CreateUser(testDB, usr); err != nil {
			t.Fatalf("CreateUser after double migration failed: %v", err)
		}
		retrieved, err := database.GetUserById(testDB, usr.Id)
		if err != nil {
			t.Fatalf("GetUserById failed: %v", err)
		}
		if !retrieved.IsAccessible {
			t.Errorf("IsAccessible should be true for newly created user after double migration")
		}
	})
}

func TestIsAccessibleInCRUD(t *testing.T) {
	db = opentmpdb()
	defer db.Close()

	t.Run("create_with_is_accessible_true", func(t *testing.T) {
		usr := &database.User{
			Id:           100,
			ScreenName:   "acc_true",
			Name:         "Acc True",
			IsProtected:  false,
			FriendsCount: 50,
			IsAccessible: true,
		}
		if err := database.CreateUser(db, usr); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		retrieved, _ := database.GetUserById(db, usr.Id)
		if retrieved.IsAccessible != true {
			t.Errorf("after create: IsAccessible = %v, want true", retrieved.IsAccessible)
		}
	})

	t.Run("create_with_is_accessible_false", func(t *testing.T) {
		usr := &database.User{
			Id:           101,
			ScreenName:   "acc_false",
			Name:         "Acc False",
			IsProtected:  true,
			FriendsCount: 0,
			IsAccessible: false,
		}
		if err := database.CreateUser(db, usr); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		retrieved, _ := database.GetUserById(db, usr.Id)
		if retrieved.IsAccessible != false {
			t.Errorf("after create: IsAccessible = %v, want false", retrieved.IsAccessible)
		}
	})

	t.Run("update_is_accessible_flip", func(t *testing.T) {
		usr := generateUser(102)
		if err := database.CreateUser(db, usr); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		usr.IsAccessible = false
		if err := database.UpdateUser(db, usr); err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		retrieved, _ := database.GetUserById(db, usr.Id)
		if retrieved.IsAccessible != false {
			t.Errorf("after update to false: IsAccessible = %v, want false", retrieved.IsAccessible)
		}

		usr.IsAccessible = true
		if err := database.UpdateUser(db, usr); err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		retrieved, _ = database.GetUserById(db, usr.Id)
		if retrieved.IsAccessible != true {
			t.Errorf("after update back to true: IsAccessible = %v, want true", retrieved.IsAccessible)
		}
	})

	t.Run("update_preserves_other_fields", func(t *testing.T) {
		usr := &database.User{
			Id:           103,
			ScreenName:   "preserve_test",
			Name:         "Preserve Test",
			IsProtected:  true,
			FriendsCount: 999,
			IsAccessible: true,
		}
		if err := database.CreateUser(db, usr); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		usr.IsAccessible = false
		usr.FriendsCount = 123
		if err := database.UpdateUser(db, usr); err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		retrieved, _ := database.GetUserById(db, usr.Id)
		if retrieved.ScreenName != usr.ScreenName {
			t.Errorf("ScreenName changed: got %q, want %q", retrieved.ScreenName, usr.ScreenName)
		}
		if retrieved.Name != usr.Name {
			t.Errorf("Name changed: got %q, want %q", retrieved.Name, usr.Name)
		}
		if retrieved.IsProtected != usr.IsProtected {
			t.Errorf("IsProtected changed: got %v, want %v", retrieved.IsProtected, usr.IsProtected)
		}
		if retrieved.FriendsCount != usr.FriendsCount {
			t.Errorf("FriendsCount changed: got %d, want %d", retrieved.FriendsCount, usr.FriendsCount)
		}
		if retrieved.IsAccessible != usr.IsAccessible {
			t.Errorf("IsAccessible mismatch: got %v, want %v", retrieved.IsAccessible, usr.IsAccessible)
		}
	})
}
