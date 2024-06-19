package downloading

import (
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd2/database"
)

// 路径Plus
type EntityBase interface {
}

type UserEntity struct {
	dbentity *database.UserEntity
	db       *sqlx.DB
}

func (ue *UserEntity) Create() error {
	path, err := ue.Path()
	if err != nil {
		return err
	}
	if err := os.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	return database.CreateUserEntity(ue.db, ue.dbentity)
}

func (ue *UserEntity) Remove() error {
	path, err := ue.Path()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return database.DelUserEntity(ue.db, uint32(ue.dbentity.Id.Int32))
}

func (ue *UserEntity) Rename(title string) error {
	old, err := ue.Path()
	if err != nil {
		return err
	}
	newPath := filepath.Join(filepath.Dir(old), title)
	if err := os.Rename(old, newPath); err != nil {
		return err
	}

	ue.dbentity.Title = title
	return database.UpdateUserEntity(ue.db, ue.dbentity)
}

func (ue *UserEntity) Path() (string, error) {
	return ue.dbentity.Path(ue.db)
}

func (ue *UserEntity) Title() string {
	return ue.dbentity.Title
}

func (ue *UserEntity) LatestReleaseTime() time.Time {
	return ue.dbentity.LatestReleaseTime.Time
}

func (ue *UserEntity) SetLatestReleaseTime(t time.Time) error {
	err := database.SetUserEntityLatestReleaseTime(ue.db, int(ue.dbentity.Id.Int32), t)
	if err == nil {
		ue.dbentity.LatestReleaseTime.Scan(t)
	}
	return err
}

func (ue *UserEntity) Uid() uint64 {
	return ue.dbentity.Uid
}

type ListEntity struct {
	dbentity *database.LstEntity
	db       *sqlx.DB
}

func (ue ListEntity) Create() error {
	path, _ := ue.Path()
	if err := os.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
		return nil
	}
	return database.CreateLstEntity(ue.db, ue.dbentity)
}

func (ue ListEntity) Remove() error {
	path, _ := ue.Path()
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return database.DelLstEntity(ue.db, int(ue.dbentity.Id.Int32))
}

func (ue ListEntity) Rename(title string) error {
	path, _ := ue.Path()
	newPath := filepath.Join(filepath.Dir(path), title)
	err := os.Rename(path, newPath)
	if os.IsNotExist(err) {
		err = os.Mkdir(newPath, 0755)
	}
	if err != nil && !os.IsExist(err) {
		return err
	}

	ue.dbentity.Title = title
	return database.UpdateLstEntity(ue.db, ue.dbentity)
}

func (ue ListEntity) Path() (string, error) {
	return ue.dbentity.Path(), nil
}

func (ue ListEntity) Title() string {
	return ue.dbentity.Title
}
