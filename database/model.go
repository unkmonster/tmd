package database

import (
	"database/sql"
	"time"
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
