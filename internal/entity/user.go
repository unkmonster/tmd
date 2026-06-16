package entity

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/database"
)

// UserEntity 用户实体
type UserEntity struct {
	record  *database.UserEntity
	db      *sqlx.DB
	created bool
}

// NewUserEntity 创建或加载用户实体
func NewUserEntity(db *sqlx.DB, uid uint64, parentDir string) (*UserEntity, error) {
	created := true
	record, err := database.LocateUserEntity(db, uid, parentDir)
	if err != nil {
		return nil, err
	}
	if record == nil {
		record = &database.UserEntity{}
		record.UserId = uid
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
		return fmt.Errorf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.UserId)
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
	return ue.record.Path()
}

// ParentDir 返回实体的父目录（用于在 Path 为空时生成 .loongtweet）
func (ue *UserEntity) ParentDir() string {
	if ue.record == nil {
		return ""
	}
	return ue.record.ParentDir
}

func (ue *UserEntity) Name() (string, error) {
	if ue.record.Name == "" {
		return "", fmt.Errorf("the name of user entity [%s:%d] was unset", ue.record.ParentDir, ue.record.UserId)
	}
	return ue.record.Name, nil
}

func (ue *UserEntity) Id() (int, error) {
	if !ue.created {
		return 0, fmt.Errorf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.UserId)
	}
	return int(ue.record.Id.Int32), nil
}

func (ue *UserEntity) LatestReleaseTime() (time.Time, error) {
	if !ue.created {
		return time.Time{}, fmt.Errorf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.UserId)
	}
	return ue.record.LatestReleaseTime.Time, nil
}

func (ue *UserEntity) SetLatestReleaseTime(t time.Time) error {
	if !ue.created {
		return fmt.Errorf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.UserId)
	}
	err := database.SetUserEntityLatestReleaseTime(ue.db, int(ue.record.Id.Int32), t)
	if err == nil {
		ue.record.LatestReleaseTime.Scan(t)
	}
	return err
}

func (ue *UserEntity) ClearLatestReleaseTime() error {
	if !ue.created {
		return fmt.Errorf("user entity [%s:%d] was not created", ue.record.ParentDir, ue.record.UserId)
	}
	err := database.ClearUserEntityLatestReleaseTime(ue.db, int(ue.record.Id.Int32))
	if err == nil {
		ue.record.LatestReleaseTime.Valid = false
	}
	return err
}

func (ue *UserEntity) UserId() uint64 {
	return ue.record.UserId
}

func (ue *UserEntity) Recorded() bool {
	return ue.created
}

func (ue *UserEntity) MediaCount() int32 {
	if !ue.record.MediaCount.Valid {
		return 0
	}
	return ue.record.MediaCount.Int32
}

// MediaCountValid 返回数据库中的 media_count 是否有值（非 NULL）。
// 当新创建的 entity 尚未同步时返回 false，调用方可据此避免使用 0 做错误推断。
func (ue *UserEntity) MediaCountValid() bool {
	return ue.record.MediaCount.Valid
}

// NewUserEntityFromRecord 从已有的数据库记录创建用户实体（用于恢复等场景）
func NewUserEntityFromRecord(db *sqlx.DB, record *database.UserEntity) *UserEntity {
	return &UserEntity{
		record:  record,
		db:      db,
		created: record.Id.Valid && record.Id.Int32 > 0,
	}
}
