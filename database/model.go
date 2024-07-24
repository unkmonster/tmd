package database

import (
	"database/sql"
	"path/filepath"

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
	Id                sql.NullInt32 `db:"id"`
	Uid               uint64        `db:"user_id"`
	Name              string        `db:"name"`
	LatestReleaseTime sql.NullTime  `db:"latest_release_time"`
	ParentDir         string        `db:"parent_dir"`
}

type UserLink struct {
	Id                sql.NullInt32 `db:"id"`
	Uid               uint64        `db:"user_id"`
	Name              string        `db:"name"`
	ParentLstEntityId int32         `db:"parent_lst_entity_id"`
}

type Lst struct {
	Id      uint64 `db:"id"`
	Name    string `db:"name"`
	OwnerId uint64 `db:"owner_uid"`
}

type LstEntity struct {
	Id        sql.NullInt32 `db:"id"`
	LstId     int64         `db:"lst_id"`
	Name      string        `db:"name"`
	ParentDir string        `db:"parent_dir"`
}

func (le *LstEntity) Path() string {
	return filepath.Join(le.ParentDir, le.Name)
}

func (ue *UserEntity) Path(db *sqlx.DB) string {
	return filepath.Join(ue.ParentDir, ue.Name)
}
