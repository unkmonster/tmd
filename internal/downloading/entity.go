package downloading

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/database"
)

// 路径Plus
type SmartPath interface {
	Path() (string, error)
	Create(name string) error
	Rename(string) error
	Remove() error
	Name() string
	Id() int
	Recorded() bool
}

func syncPath(path SmartPath, expectedName string) error {
	if !path.Recorded() {
		return path.Create(expectedName)
	}

	if path.Name() != expectedName {
		return path.Rename(expectedName)
	}

	p, err := path.Path()
	if err != nil {
		return err
	}

	return os.MkdirAll(p, 0755)
}

type UserEntity struct {
	record  *database.UserEntity
	db      *sqlx.DB
	created bool
}

func NewUserEntity(db *sqlx.DB, uid uint64, parentDir string) (*UserEntity, error) {
	created := true
	record, err := database.LocateUserEntity(db, uid, parentDir)
	if err != nil {
		return nil, err
	}
	if record == nil {
		record = &database.UserEntity{}
		record.Uid = uid
		record.ParentDir = parentDir
		created = false
	}
	return &UserEntity{record: record, db: db, created: created}, nil
}

func (ue *UserEntity) Create(name string) error {
	ue.record.Name = name
	path, _ := ue.Path()
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	if err := database.CreateUserEntity(ue.db, ue.record); err != nil {
		return err
	}
	ue.created = true
	return nil
}

func (ue *UserEntity) Remove() error {
	path, _ := ue.Path()

	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if err := database.DelUserEntity(ue.db, uint32(ue.record.Id.Int32)); err != nil {
		return err
	}
	ue.created = false
	return nil
}

func (ue *UserEntity) Rename(title string) error {
	if !ue.created {
		return fmt.Errorf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.Uid)
	}

	old, _ := ue.Path()
	newPath := filepath.Join(filepath.Dir(old), title)

	err := os.Rename(old, newPath)
	if os.IsNotExist(err) {
		err = os.Mkdir(newPath, 0755)
	}
	if err != nil && !os.IsExist(err) {
		return err
	}

	ue.record.Name = title
	return database.UpdateUserEntity(ue.db, ue.record)
}

func (ue *UserEntity) Path() (string, error) {
	return ue.record.Path(), nil
}

func (ue *UserEntity) Name() string {
	if ue.record.Name == "" {
		panic(fmt.Errorf("the name of user entity [%s:%d] was unset", ue.record.ParentDir, ue.record.Uid))
	}
	return ue.record.Name
}

func (ue *UserEntity) Id() int {
	if !ue.created {
		panic(fmt.Sprintf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.Uid))
	}
	return int(ue.record.Id.Int32)
}

func (ue *UserEntity) LatestReleaseTime() time.Time {
	if !ue.created {
		panic(fmt.Sprintf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.Uid))
	}
	return ue.record.LatestReleaseTime.Time
}

func (ue *UserEntity) SetLatestReleaseTime(t time.Time) error {
	if !ue.created {
		return fmt.Errorf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.Uid)
	}
	err := database.SetUserEntityLatestReleaseTime(ue.db, int(ue.record.Id.Int32), t)
	if err == nil {
		ue.record.LatestReleaseTime.Scan(t)
	}
	return err
}

func (ue *UserEntity) Uid() uint64 {
	return ue.record.Uid
}

func (ue *UserEntity) Recorded() bool {
	return ue.created
}

type ListEntity struct {
	record  *database.LstEntity
	db      *sqlx.DB
	created bool
}

func NewListEntity(db *sqlx.DB, lid int64, parentDir string) (*ListEntity, error) {
	created := true
	record, err := database.LocateLstEntity(db, lid, parentDir)
	if err != nil {
		return nil, err
	}
	if record == nil {
		record = &database.LstEntity{}
		record.LstId = lid
		record.ParentDir = parentDir
		created = false
	}
	return &ListEntity{record: record, db: db, created: created}, nil
}

func (le *ListEntity) Create(name string) error {
	le.record.Name = name
	path, _ := le.Path()
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil
	}

	if err := database.CreateLstEntity(le.db, le.record); err != nil {
		return err
	}
	le.created = true
	return nil
}

func (le *ListEntity) Remove() error {
	if !le.created {
		return fmt.Errorf("list entity [%s:%d] was not created", le.record.ParentDir, le.record.LstId)
	}

	path, _ := le.Path()
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if err := database.DelLstEntity(le.db, int(le.record.Id.Int32)); err != nil {
		return err
	}
	le.created = false
	return nil
}

func (le *ListEntity) Rename(title string) error {
	if !le.created {
		return fmt.Errorf("list entity [%s:%d] was not created", le.record.ParentDir, le.record.LstId)
	}

	path, _ := le.Path()
	newPath := filepath.Join(filepath.Dir(path), title)
	err := os.Rename(path, newPath)
	if os.IsNotExist(err) {
		err = os.Mkdir(newPath, 0755)
	}
	if err != nil && !os.IsExist(err) {
		return err
	}

	le.record.Name = title
	return database.UpdateLstEntity(le.db, le.record)
}

func (le *ListEntity) Path() (string, error) {
	return le.record.Path(), nil
}

func (le ListEntity) Name() string {
	if le.record.Name == "" {
		panic(fmt.Sprintf("the name of list entity [%s:%d] was unset", le.record.ParentDir, le.record.LstId))
	}

	return le.record.Name
}

func (le *ListEntity) Id() int {
	if !le.created {
		panic(fmt.Sprintf("list entity [%s:%d] was not created", le.record.ParentDir, le.record.LstId))
	}

	return int(le.record.Id.Int32)
}

func (le *ListEntity) Recorded() bool {
	return le.created
}

func updateUserLink(lnk *database.UserLink, db *sqlx.DB, path string) error {
	name := filepath.Base(path)

	linkpath, err := lnk.Path(db)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	if lnk.Name == name {
		// 用户未改名，但仍应确保链接存在
		err = os.Symlink(path, linkpath)
		if os.IsExist(err) {
			err = nil
		}
		return err
	}

	newlinkpath := filepath.Join(filepath.Dir(linkpath), name)

	if err = os.RemoveAll(linkpath); err != nil {
		return err
	}
	if err = os.Symlink(path, newlinkpath); err != nil && !os.IsExist(err) {
		return err
	}

	if err = database.UpdateUserLink(db, lnk.Id.Int32, name); err != nil {
		return err
	}

	lnk.Name = name
	return nil
}
