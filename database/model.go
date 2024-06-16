package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
)

type User struct {
	Id           uint64 `db:"id"`
	ScreenName   string `db:"screen_name"`
	Name         string `db:"name"`
	IsProtected  bool   `db:"protected"`
	FriendsCount int    `db:"friends_count"`
}

type UserEntity struct {
	Id                sql.NullInt32  `db:"id"`
	Uid               uint64         `db:"user_id"`
	Title             string         `db:"title"`
	LatestReleaseTime time.Time      `db:"latest_release_time"`
	ParentDir         sql.NullString `db:"parent_dir"`
	ParentLstEntityId sql.NullInt32  `db:"parent_lst_entity_id"`
}

type Lst struct {
	Id      uint64 `db:"id"`
	Name    string `db:"name"`
	OwnerId uint64 `db:"owner_id"`
}

type LstEntity struct {
	Id        int    `db:"id"`
	LstId     uint64 `db:"lst_id"`
	Titile    string `db:"title"`
	ParentDir string `db:"parent_dir"`
}

func (le *LstEntity) Path() string {
	return filepath.Join(le.ParentDir, le.Titile)
}

func (ue *UserEntity) Path(db *sqlx.DB) (string, error) {
	if ue.ParentDir.Valid {
		return filepath.Join(ue.ParentDir.String, ue.Title), nil
	}

	if db != nil && ue.ParentLstEntityId.Valid {
		lstEntity, err := GetLstEntity(db, int(ue.ParentLstEntityId.Int32))
		if err != nil {
			return "", err
		}
		return filepath.Join(lstEntity.Path(), ue.Title), nil
	}
	return "", fmt.Errorf("no enough info to get path")
}
