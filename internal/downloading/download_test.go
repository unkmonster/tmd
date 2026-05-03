package downloading

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/entity"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

/*
创建
改名
*/
func TestUserEntity(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(tempdir)

	name := "test"
	uid := 0

	os.RemoveAll(filepath.Join(tempdir, name))
	testSyncUser(t, db, name, uid, tempdir, false)

	// 改名
	name = name + "renamed"
	os.RemoveAll(filepath.Join(tempdir, name))
	testSyncUser(t, db, name, uid, tempdir, true)

	// 什么都不干
	os.RemoveAll(filepath.Join(tempdir, name))
	ue := testSyncUser(t, db, name, uid, tempdir, true)

	minTime, err := ue.LatestReleaseTime()
	if err != nil {
		t.Error(err)
		return
	}
	if !minTime.IsZero() {
		t.Errorf("default time is not null")
		return
	}

	now := time.Now()
	if err := ue.SetLatestReleaseTime(now); err != nil {
		t.Error(err)
		return
	}
	minTime, err = ue.LatestReleaseTime()
	if err != nil {
		t.Error(err)
		return
	}
	if !minTime.Equal(now) {
		t.Errorf("latest release: %v, want %v", minTime, now)
	}

	eid, err := ue.Id()
	if err != nil {
		t.Error(err)
		return
	}
	record, err := database.GetUserEntity(db, eid)
	if err != nil {
		t.Error(err)
		return
	}
	if !record.LatestReleaseTime.Time.Equal(now) {
		t.Errorf("recorded time: %v, want %v", record.LatestReleaseTime.Time, now)
	}

	// remove
	eid, err = ue.Id()
	if err != nil {
		t.Error(err)
		return
	}
	if err := ue.Remove(); err != nil {
		t.Error(err)
		return
	}

	ex, err := utils.PathExists(filepath.Join(tempdir, name))
	if err != nil {
		t.Error(err)
		return
	}
	if ex {
		t.Errorf("dir is exist after remove")
	}

	record, err = database.GetUserEntity(db, eid)
	if err != nil {
		t.Error(err)
		return
	}
	if record != nil {
		t.Errorf("record is exist after remove")
	}
}

func TestListEntity(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(tempdir)

	name := "test"
	uid := 0
	os.RemoveAll(filepath.Join(tempdir, name))
	testSyncList(t, db, name, uid, tempdir, false)

	// 改名
	name = name + "renamed"
	os.RemoveAll(filepath.Join(tempdir, name))
	testSyncList(t, db, name, uid, tempdir, true)

	os.RemoveAll(filepath.Join(tempdir, name))
	le := testSyncList(t, db, name, uid, tempdir, true)

	// remove
	eid, err := le.Id()
	if err != nil {
		t.Error(err)
		return
	}
	if err := le.Remove(); err != nil {
		t.Error(err)
		return
	}

	ex, err := utils.PathExists(filepath.Join(tempdir, name))
	if err != nil {
		t.Error(err)
		return
	}
	if ex {
		t.Errorf("dir is exist after remove")
	}

	record, err := database.GetLstEntity(db, eid)
	if err != nil {
		t.Error(err)
		return
	}
	if record != nil {
		t.Errorf("record is exist after remove")
	}
}

func verifyDir(t *testing.T, e entity.Entity, wantPath string) {
	path, _ := e.Path()
	if wantPath != path {
		t.Errorf("path: %s, want %s", path, wantPath)
		return
	}

	// 目录存在
	ex, err := utils.PathExists(path)
	if err != nil {
		t.Error(err)
		return
	}
	if !ex {
		t.Errorf("path is not exist")
		return
	}
}

func verifyUserRecord(t *testing.T, db *sqlx.DB, e entity.Entity, uid uint64, name string, parentDir string) *entity.UserEntity {
	wantPath := filepath.Join(parentDir, name)
	record, err := database.LocateUserEntity(db, uid, parentDir)
	if err != nil {
		t.Error(err)
		return nil
	}
	eid, err := e.Id()
	if err != nil {
		t.Error(err)
		return nil
	}
	if int(database.NullInt32(record.Id)) != eid {
		t.Errorf("eid: %d, want %d", eid, database.NullInt32(record.Id))
	}
	recordPath, err := record.Path()
	if err != nil {
		t.Error(err)
		return nil
	}
	if recordPath != wantPath {
		t.Errorf("recorded path: %s, want %s", recordPath, wantPath)
	}
	if record.Name != name {
		t.Errorf("recorded name: %s, want %s", record.Name, name)
	}
	if record.UserId != uid {
		t.Errorf("uid: %d, want %d", record.UserId, uid)
	}
	return e.(*entity.UserEntity)
}

func verifyLstRecord(t *testing.T, db *sqlx.DB, e entity.Entity, lid int64, name string, parentDir string) {
	wantPath := filepath.Join(parentDir, name)
	record, err := database.LocateLstEntity(db, lid, parentDir)
	if err != nil {
		t.Error(err)
		return
	}
	eid, err := e.Id()
	if err != nil {
		t.Error(err)
		return
	}
	if int(database.NullInt32(record.Id)) != eid {
		t.Errorf("eid: %d, want %d", eid, database.NullInt32(record.Id))
	}
	recordPath, err := record.Path()
	if err != nil {
		t.Error(err)
		return
	}
	if recordPath != wantPath {
		t.Errorf("recorded path: %s, want %s", recordPath, wantPath)
	}
	if record.Name != name {
		t.Errorf("recorded name: %s, want %s", record.Name, name)
	}
	if record.LstId != lid {
		t.Errorf("uid: %d, want %d", record.LstId, lid)
	}
}

func testSyncUser(t *testing.T, db *sqlx.DB, name string, uid int, parentdir string, exist bool) *entity.UserEntity {
	ue, err := entity.NewUserEntity(db, uint64(uid), parentdir)
	if err != nil {
		t.Error(err)
		return nil
	}

	if ue.Recorded() && !exist {
		t.Errorf("ue.created = true, want false")
	} else if !ue.Recorded() && exist {
		t.Errorf("ue.created = false, want true")
	}

	if err := entity.Sync(ue, name); err != nil {
		t.Error(err)
		return nil
	}

	wantPath := filepath.Join(parentdir, name)
	verifyDir(t, ue, wantPath)

	verifyUserRecord(t, db, ue, uint64(uid), name, parentdir)

	if ue.UserId() != uint64(uid) {
		t.Errorf("uid: %d, want %d", ue.UserId(), uid)
	}
	return ue
}

func testSyncList(t *testing.T, db *sqlx.DB, name string, lid int, parentDir string, exist bool) *entity.ListEntity {
	le, err := entity.NewListEntity(db, int64(lid), parentDir)
	if err != nil {
		t.Error(err)
		return nil
	}

	if le.Recorded() && !exist {
		t.Errorf("ue.created = true, want false")
	} else if !le.Recorded() && exist {
		t.Errorf("ue.created = false, want true")
	}

	if err := entity.Sync(le, name); err != nil {
		t.Error(err)
		return nil
	}

	wantPath := filepath.Join(parentDir, name)
	verifyDir(t, le, wantPath)

	verifyLstRecord(t, db, le, int64(lid), name, parentDir)
	return le
}

func TestDumper(t *testing.T) {
	dumper := NewDumper()

	n := 3
	tweets := generateSomeTweets(n * 10)

	k := 0
	for i := 0; i < n; i++ {
		for j := 0; j < 10; j++ {
			dumper.Push(i, tweets[k])
			k++
		}

	}

	// 重复推送
	k = 0
	for i := 0; i < n; i++ {
		for j := 0; j < 10; j++ {
			if dumper.Push(i, tweets[k]) != 0 {
				t.Errorf("repeat push")
			}
			k++
		}

	}

	if dumper.count != 30 {
		t.Errorf("dumper.count: %d, want %d", dumper.count, 30)
	}

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.Remove(f.Name())

	if err := dumper.Dump(f.Name()); err != nil {
		t.Error(err)
		return
	}

	dumper.Clear()
	if dumper.count != 0 {
		t.Errorf("dumper.count: %d, want %d", dumper.count, 0)
		return
	}

	if err := dumper.Load(f.Name()); err != nil {
		t.Error(err)
		return
	}

	if dumper.count != 30 {
		t.Errorf("dumper.count: %d want %d", dumper.count, 30)
	}

	k = 0
	for i := 0; i < n; i++ {
		for j := 0; j < 10; j++ {
			if dumper.Push(i, tweets[k]) != 0 {
				t.Errorf("repeat push after load")
				break
			}
			k++
		}

	}
}

func generateSomeTweets(n int) []*twitter.Tweet {
	res := []*twitter.Tweet{}
	for i := 0; i < n; i++ {
		tw := &twitter.Tweet{}
		tw.CreatedAt = time.Now()
		tw.Creator = nil
		tw.Id = uint64(i)
		tw.Text = fmt.Sprintf("tweet %d", i)
		res = append(res, tw)
	}
	return res
}
