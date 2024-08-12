package downloading

import (
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd2/database"
)

// 路径Plus
type SmartPath interface {
	Create() error
	Rename(string) error
	Remove() error
	LoadRecordedName() (bool, error)
	Name() string
	Path() (string, error)
	SetName(name string)
	Id() int
}

func syncPath(path SmartPath, expectedName string) error {
	ok, err := path.LoadRecordedName()
	if err != nil {
		return err
	}
	if !ok {
		path.SetName(expectedName)
		return path.Create()
	}
	if path.Name() != expectedName {
		return path.Rename(expectedName)
	} else {
		p, err := path.Path()
		if err != nil {
			return err
		}
		// 不处理改名也不处理创建，仍要确保路径目录可用
		err = os.Mkdir(p, 0755)
		if err == nil || os.IsExist(err) {
			return nil
		}
		return err
	}
}

type UserEntity struct {
	entity *database.UserEntity
	db     *sqlx.DB
}

func NewUserEntity(db *sqlx.DB, uid uint64, parentDir string) *UserEntity {
	ue := UserEntity{}
	entity := database.UserEntity{}
	entity.Uid = uid
	entity.ParentDir = parentDir
	ue.db = db
	ue.entity = &entity
	return &ue
}

func (ue *UserEntity) Create() error {
	path, err := ue.Path()
	if err != nil {
		return err
	}
	if err := os.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	return database.CreateUserEntity(ue.db, ue.entity)
}

func (ue *UserEntity) Remove() error {
	path, err := ue.Path()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return database.DelUserEntity(ue.db, uint32(ue.entity.Id.Int32))
}

func (ue *UserEntity) Rename(title string) error {
	old, err := ue.Path()
	if err != nil {
		return err
	}
	newPath := filepath.Join(filepath.Dir(old), title)
	err = os.Rename(old, newPath)
	if os.IsNotExist(err) {
		err = os.Mkdir(newPath, 0755)
	}
	if err != nil && !os.IsExist(err) {
		return err
	}

	ue.entity.Name = title
	return database.UpdateUserEntity(ue.db, ue.entity)
}

func (ue *UserEntity) Path() (string, error) {
	return ue.entity.Path(), nil
}

func (ue *UserEntity) Name() string {
	return ue.entity.Name
}

func (ue *UserEntity) LoadRecordedName() (bool, error) {
	var entity *database.UserEntity
	var err error
	entity, err = database.LocateUserEntity(ue.db, ue.entity.Uid, ue.entity.ParentDir)

	if err != nil {
		return false, err
	}
	if entity == nil {
		return false, nil
	}
	ue.entity = entity
	return true, nil
}

func (ue *UserEntity) SetName(name string) {
	ue.entity.Name = name
}

func (ue *UserEntity) Id() int {
	return int(ue.entity.Id.Int32)
}

func (ue *UserEntity) LatestReleaseTime() time.Time {
	return ue.entity.LatestReleaseTime.Time
}

func (ue *UserEntity) SetLatestReleaseTime(t time.Time) error {
	err := database.SetUserEntityLatestReleaseTime(ue.db, int(ue.entity.Id.Int32), t)
	if err == nil {
		ue.entity.LatestReleaseTime.Scan(t)
	}
	return err
}

func (ue *UserEntity) Uid() uint64 {
	return ue.entity.Uid
}

type ListEntity struct {
	entity *database.LstEntity
	db     *sqlx.DB
}

func NewListEntity(db *sqlx.DB, lid int64, parentDir string) *ListEntity {
	le := ListEntity{}
	entity := database.LstEntity{}
	entity.LstId = lid
	entity.ParentDir = parentDir
	le.db = db
	le.entity = &entity
	return &le
}

func (ue *ListEntity) Create() error {
	path, _ := ue.Path()
	if err := os.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
		return nil
	}
	return database.CreateLstEntity(ue.db, ue.entity)
}

func (ue *ListEntity) Remove() error {
	path, _ := ue.Path()
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return database.DelLstEntity(ue.db, int(ue.entity.Id.Int32))
}

func (ue *ListEntity) Rename(title string) error {
	path, _ := ue.Path()
	newPath := filepath.Join(filepath.Dir(path), title)
	err := os.Rename(path, newPath)
	if os.IsNotExist(err) {
		err = os.Mkdir(newPath, 0755)
	}
	if err != nil && !os.IsExist(err) {
		return err
	}

	ue.entity.Name = title
	return database.UpdateLstEntity(ue.db, ue.entity)
}

func (ue *ListEntity) Path() (string, error) {
	return ue.entity.Path(), nil
}

func (ue ListEntity) Name() string {
	return ue.entity.Name
}

func (ue *ListEntity) LoadRecordedName() (bool, error) {
	entity, err := database.LocateLstEntity(ue.db, ue.entity.LstId, ue.entity.ParentDir)
	if err != nil {
		return false, err
	}
	if entity == nil {
		return false, nil
	}
	ue.entity = entity
	return true, nil
}

func (ue *ListEntity) SetName(name string) {
	ue.entity.Name = name
}

func (ue *ListEntity) Id() int {
	return int(ue.entity.Id.Int32)
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
