package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var db *sqlx.DB
var dbname = "test.db"

func opendb() *sqlx.DB {
	var err error
	tempdir := os.TempDir()
	path := filepath.Join(tempdir, dbname)
	err = os.RemoveAll(path)
	if err != nil {
		panic(err)
	}

	db, err = sqlx.Connect("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&cache=shared", path))

	if err != nil {
		panic(err)
	}

	CreateTables(db)
	return db
}

func generateUser(n int) *User {
	usr := &User{}
	usr.Id = uint64(n)
	name := fmt.Sprintf("user%d", n)
	usr.ScreenName = name
	usr.Name = name
	return usr
}
func TestUserOperation(t *testing.T) {
	db = opendb()
	defer db.Close()

	n := 100
	users := make([]*User, n)
	for i := 0; i < n; i++ {
		users[i] = generateUser(i)
	}

	for _, usr := range users {
		testUser(t, usr)
	}
}

func testUser(t *testing.T, usr *User) {
	// create
	if err := CreateUser(db, usr); err != nil {
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
	if err := UpdateUser(db, usr); err != nil {
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
	if err = RecordUserPreviousName(db, usr.Id, usr.Name, usr.ScreenName); err != nil {
		t.Error(err)
		return
	}

	// delete
	if err = DelUser(db, usr.Id); err != nil {
		t.Error(err)
		return
	}

	usr, err = GetUserById(db, usr.Id)
	if err != nil {
		t.Error(err)
		return
	}
	if usr != nil {
		t.Error("record mismatch after delete user")
	}
}

func hasSameUserRecord(usr *User) (bool, error) {
	retrieved, err := GetUserById(db, usr.Id)
	return retrieved != nil && *retrieved == *usr, err
}

func generateList(id int) *Lst {
	lst := &Lst{}
	lst.Id = uint64(id)
	lst.Name = fmt.Sprintf("lst%d", id)
	return lst
}

func TestList(t *testing.T) {
	db = opendb()
	defer db.Close()
	n := 100
	lsts := make([]*Lst, n)
	for i := 0; i < n; i++ {
		lsts[i] = generateList(i)
	}

	for _, lst := range lsts {
		// create
		if err := CreateLst(db, lst); err != nil {
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
		if err = UpdateLst(db, lst); err != nil {
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

		// delete
		if err = DelLst(db, lst.Id); err != nil {
			t.Error(err)
			return
		}
		record, err := GetLst(db, lst.Id)
		if err != nil {
			t.Error(err)
			return
		}
		if record != nil {
			t.Error("record mismatch after delete lst")
			return
		}
	}
}

func isSameLstRecord(lst *Lst) (bool, error) {
	record, err := GetLst(db, lst.Id)
	return record != nil && *record == *lst, err
}

func TestUserEntity(t *testing.T) {
	db = opendb()
	defer db.Close()
	n := 100
	entities := make([]*UserEntity, n)
	tempDir := os.TempDir()
	for i := 0; i < n; i++ {
		entities[i] = generateUserEntity(uint64(i), tempDir)
	}

	for _, entity := range entities {
		// path
		expectedPath := filepath.Join(tempDir, entity.Name)
		if expectedPath != entity.Path() {
			t.Errorf("entity.Path() = %v want %v", entity.Path(), expectedPath)
			return
		}

		// create
		if err := CreateUserEntity(db, entity); err != nil {
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
		if err := UpdateUserEntity(db, entity); err != nil {
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
		if err = SetUserEntityLatestReleaseTime(db, int(entity.Id.Int32), now); err != nil {
			t.Error(err)
			return
		}

		// locate
		record, err := LocateUserEntity(db, entity.Uid, tempDir)
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
		if err = DelUserEntity(db, uint32(entity.Id.Int32)); err != nil {
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

func generateUserEntity(uid uint64, pdir string) *UserEntity {
	ue := UserEntity{}
	user := generateUser(int(uid))
	if err := CreateUser(db, user); err != nil {
		panic(err)
	}

	ue.Name = user.Name
	ue.Uid = uid
	ue.ParentDir = pdir
	return &ue
}

func hasSameUserEntityRecord(entity *UserEntity) (bool, error) {
	record, err := GetUserEntity(db, int(entity.Id.Int32))
	return record != nil && *record == *entity, err
}

func TestLstEntity(t *testing.T) {
	db = opendb()
	defer db.Close()
	tempdir := os.TempDir()
	n := 100
	entities := make([]*LstEntity, n)
	for i := 0; i < n; i++ {
		entities[i] = generateLstEntity(int64(i), tempdir)
	}

	for _, entity := range entities {
		// path
		expectedPath := filepath.Join(tempdir, entity.Name)
		if expectedPath != entity.Path() {
			t.Errorf("entity.Path() = %v want %v", entity.Path(), expectedPath)
			return
		}
		// create
		if err := CreateLstEntity(db, entity); err != nil {
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
		if err = UpdateLstEntity(db, entity); err != nil {
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
		record, err := LocateLstEntity(db, entity.LstId, entity.ParentDir)
		if err != nil {
			t.Error(err)
			return
		}
		if record == nil || *record != *entity {
			t.Error("record mismatch after locate lst entity")
			return
		}

		// delete
		if err = DelLstEntity(db, int(entity.Id.Int32)); err != nil {
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

func generateLstEntity(lid int64, pdir string) *LstEntity {
	lst := generateList(int(lid))
	if err := CreateLst(db, lst); err != nil {
		panic(err)
	}
	entity := LstEntity{}
	entity.LstId = lid
	entity.ParentDir = pdir
	entity.Name = lst.Name
	return &entity
}

func hasSameLstEntityRecord(entity *LstEntity) (bool, error) {
	record, err := GetLstEntity(db, int(entity.Id.Int32))
	return record != nil && *record == *entity, err
}

func TestLink(t *testing.T) {
	db = opendb()
	defer db.Close()
	n := 100
	links := make([]*UserLink, n)
	for i := 0; i < n; i++ {
		links[i] = generateLink(i, i)
	}

	for _, link := range links {
		// path
		le, err := GetLstEntity(db, int(link.ParentLstEntityId))
		if err != nil {
			t.Error(err)
			return
		}
		expectedPath := filepath.Join(le.Path(), link.Name)
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
		if err := CreateUserLink(db, link); err != nil {
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

		records, err := GetUserLinks(db, link.Uid)
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
		if err = UpdateUserLink(db, link.Id.Int32, link.Name); err != nil {
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

		// d
		if err := DelUserLink(db, link.Id.Int32); err != nil {
			t.Error(err)
			return
		}
		yes, err = hasSameUserLinkRecord(link)
		if err != nil {
			t.Error(err)
			return
		}
		if yes {
			t.Error("mismatch record after delete user link")
			return
		}
	}
}

func generateLink(uid int, lid int) *UserLink {
	usr := generateUser(uid)
	le := generateLstEntity(int64(lid), os.TempDir())
	if err := CreateLstEntity(db, le); err != nil {
		panic(err)
	}

	ul := UserLink{}
	ul.Name = fmt.Sprintf("%d-%d", lid, uid)
	ul.ParentLstEntityId = le.Id.Int32
	ul.Uid = usr.Id
	return &ul
}

func hasSameUserLinkRecord(link *UserLink) (bool, error) {
	record, err := GetUserLink(db, link.Uid, link.ParentLstEntityId)
	return record != nil && *record == *link, err
}

func benchmarkUpdateUser(b *testing.B, routines int) {
	db = opendb()
	defer db.Close()

	n := 500
	users := make(chan *User, n)
	for i := 0; i < n; i++ {
		user := generateUser(i)
		if err := CreateUser(db, user); err != nil {
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
			if err := UpdateUser(db, user); err != nil {
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
