package database

import (
	"database/sql"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER NOT NULL, 
	screen_name VARCHAR NOT NULL, 
	name VARCHAR NOT NULL, 
	protected BOOLEAN NOT NULL, 
	friends_count INTEGER NOT NULL, 
	PRIMARY KEY (id), 
	UNIQUE (screen_name)
);

CREATE TABLE IF NOT EXISTS user_previous_names (
	id INTEGER NOT NULL, 
	uid INTEGER NOT NULL, 
	screen_name VARCHAR NOT NULL, 
	name VARCHAR NOT NULL, 
	record_date DATE NOT NULL, 
	PRIMARY KEY (id), 
	FOREIGN KEY(uid) REFERENCES users (id)
);

CREATE TABLE IF NOT EXISTS lsts (
	id INTEGER NOT NULL, 
	name VARCHAR NOT NULL, 
	owner_uid INTEGER NOT NULL, 
	PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS lst_entities (
	id INTEGER NOT NULL, 
	lst_id INTEGER NOT NULL, 
	name VARCHAR NOT NULL, 
	parent_dir VARCHAR NOT NULL COLLATE NOCASE, 
	PRIMARY KEY (id), 
	UNIQUE (lst_id, parent_dir)
);

CREATE TABLE IF NOT EXISTS user_entities (
	id INTEGER NOT NULL, 
	user_id INTEGER NOT NULL, 
	name VARCHAR NOT NULL, 
	latest_release_time DATETIME, 
	parent_dir VARCHAR COLLATE NOCASE NOT NULL, 
	media_count INTEGER,
	PRIMARY KEY (id), 
	UNIQUE (user_id, parent_dir), 
	FOREIGN KEY(user_id) REFERENCES users (id)
);

CREATE TABLE IF NOT EXISTS user_links (
	id INTEGER NOT NULL,
	user_id INTEGER NOT NULL, 
	name VARCHAR NOT NULL, 
	parent_lst_entity_id INTEGER NOT NULL,
	PRIMARY KEY (id),
	UNIQUE (user_id, parent_lst_entity_id),
	FOREIGN KEY(user_id) REFERENCES users (id), 
	FOREIGN KEY(parent_lst_entity_id) REFERENCES lst_entities (id)
);
`

func CreateTables(db *sqlx.DB) {
	db.MustExec(schema)
}

func CreateUser(db *sqlx.DB, usr *User) error {
	stmt := `INSERT INTO Users(id, screen_name, name, protected, friends_count) VALUES(:id, :screen_name, :name, :protected, :friends_count)`
	_, err := db.NamedExec(stmt, usr)
	return err
}

func DelUser(db *sqlx.DB, uid uint64) error {
	stmt := `DELETE FROM users WHERE id=?`
	_, err := db.Exec(stmt, uid)
	return err
}

func GetUserById(db *sqlx.DB, uid uint64) (*User, error) {
	stmt := `SELECT * FROM users WHERE id=?`
	result := &User{}
	err := db.Get(result, stmt, uid)
	if err == sql.ErrNoRows {
		result = nil
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func UpdateUser(db *sqlx.DB, usr *User) error {
	stmt := `UPDATE users SET screen_name=:screen_name, name=:name, protected=:protected, friends_count=:friends_count WHERE id=:id`
	_, err := db.NamedExec(stmt, usr)
	return err
}

func CreateUserEntity(db *sqlx.DB, entity *UserEntity) error {
	abs, err := filepath.Abs(entity.ParentDir)
	if err != nil {
		return err
	}
	entity.ParentDir = abs

	stmt := `INSERT INTO user_entities(user_id, name, parent_dir) VALUES(:user_id, :name, :parent_dir)`
	de, err := db.NamedExec(stmt, entity)
	if err != nil {
		return err
	}
	lastId, err := de.LastInsertId()
	if err != nil {
		return err
	}

	entity.Id.Scan(lastId)
	return nil
}

func DelUserEntity(db *sqlx.DB, id uint32) error {
	stmt := `DELETE FROM user_entities WHERE id=?`
	_, err := db.Exec(stmt, id)
	return err
}

func LocateUserEntity(db *sqlx.DB, uid uint64, parentDIr string) (*UserEntity, error) {
	parentDIr, err := filepath.Abs(parentDIr)
	if err != nil {
		return nil, err
	}

	stmt := `SELECT * FROM user_entities WHERE user_id=? AND parent_dir=?`
	result := &UserEntity{}
	err = db.Get(result, stmt, uid, parentDIr)
	if err == sql.ErrNoRows {
		err = nil
		result = nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetUserEntity(db *sqlx.DB, id int) (*UserEntity, error) {
	result := &UserEntity{}
	stmt := `SELECT * FROM user_entities WHERE id=?`
	err := db.Get(result, stmt, id)
	if err == sql.ErrNoRows {
		result = nil
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func UpdateUserEntity(db *sqlx.DB, entity *UserEntity) error {
	stmt := `UPDATE user_entities SET name=?, latest_release_time=?, media_count=? WHERE id=?`
	_, err := db.Exec(stmt, entity.Name, entity.LatestReleaseTime, entity.MediaCount, entity.Id)
	return err
}

func UpdateUserEntityMediCount(db *sqlx.DB, eid int, count int) error {
	stmt := `UPDATE user_entities SET media_count=? WHERE id=?`
	_, err := db.Exec(stmt, count, eid)
	return err
}

func UpdateUserEntityTweetStat(db *sqlx.DB, eid int, baseline time.Time, count int) error {
	stmt := `UPDATE user_entities SET latest_release_time=?, media_count=? WHERE id=?`
	_, err := db.Exec(stmt, baseline, count, eid)
	return err
}

func CreateLst(db *sqlx.DB, lst *Lst) error {
	stmt := `INSERT INTO lsts(id, name, owner_uid) VALUES(:id, :name, :owner_uid)`
	_, err := db.NamedExec(stmt, &lst)
	return err
}

func DelLst(db *sqlx.DB, lid uint64) error {
	stmt := `DELETE FROM lsts WHERE id=?`
	_, err := db.Exec(stmt, lid)
	return err
}

func GetLst(db *sqlx.DB, lid uint64) (*Lst, error) {
	stmt := `SELECT * FROM lsts WHERE id = ?`
	result := &Lst{}
	err := db.Get(result, stmt, lid)
	if err == sql.ErrNoRows {
		err = nil
		result = nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func UpdateLst(db *sqlx.DB, lst *Lst) error {
	stmt := `UPDATE lsts SET name=? WHERE id=?`
	_, err := db.Exec(stmt, lst.Name, lst.Id)
	return err
}

func CreateLstEntity(db *sqlx.DB, entity *LstEntity) error {
	abs, err := filepath.Abs(entity.ParentDir)
	if err != nil {
		return err
	}
	entity.ParentDir = abs

	stmt := `INSERT INTO lst_entities(id, lst_id, name, parent_dir) VALUES(:id, :lst_id, :name, :parent_dir)`
	r, err := db.NamedExec(stmt, &entity)
	if err != nil {
		return err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return err
	}
	entity.Id.Scan(id)
	return nil
}

func DelLstEntity(db *sqlx.DB, id int) error {
	stmt := `DELETE FROM lst_entities WHERE id=?`
	_, err := db.Exec(stmt, id)
	return err
}

func GetLstEntity(db *sqlx.DB, id int) (*LstEntity, error) {
	stmt := `SELECT * FROM lst_entities WHERE id=?`
	result := &LstEntity{}
	err := db.Get(result, stmt, id)
	if err == sql.ErrNoRows {
		err = nil
		result = nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func LocateLstEntity(db *sqlx.DB, lid int64, parentDir string) (*LstEntity, error) {
	parentDir, err := filepath.Abs(parentDir)
	if err != nil {
		return nil, err
	}

	stmt := `SELECT * FROM lst_entities WHERE lst_id=? AND parent_dir=?`
	result := &LstEntity{}
	err = db.Get(result, stmt, lid, parentDir)
	if err == sql.ErrNoRows {
		err = nil
		result = nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
func UpdateLstEntity(db *sqlx.DB, entity *LstEntity) error {
	stmt := `UPDATE lst_entities SET name=? WHERE id=?`
	_, err := db.Exec(stmt, entity.Name, entity.Id.Int32)
	return err
}

func SetUserEntityLatestReleaseTime(db *sqlx.DB, id int, t time.Time) error {
	stmt := `UPDATE user_entities SET latest_release_time=? WHERE id=?`
	_, err := db.Exec(stmt, t, id)
	return err
}

func RecordUserPreviousName(db *sqlx.DB, uid uint64, name string, screenName string) error {
	stmt := `INSERT INTO user_previous_names(uid, screen_name, name, record_date) VALUES(?, ?, ?, ?)`
	_, err := db.Exec(stmt, uid, screenName, name, time.Now())
	return err
}

func CreateUserLink(db *sqlx.DB, lnk *UserLink) error {
	stmt := `INSERT INTO user_links(user_id, name, parent_lst_entity_id) VALUES(:user_id, :name, :parent_lst_entity_id)`
	res, err := db.NamedExec(stmt, lnk)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	lnk.Id.Scan(id)
	return nil
}

func DelUserLink(db *sqlx.DB, id int32) error {
	stmt := `DELETE FROM user_links WHERE id = ?`
	_, err := db.Exec(stmt, id)
	return err
}

func GetUserLinks(db *sqlx.DB, uid uint64) ([]*UserLink, error) {
	stmt := `SELECT * FROM user_links WHERE user_id = ?`
	res := []*UserLink{}
	err := db.Select(&res, stmt, uid)
	return res, err
}

func GetUserLink(db *sqlx.DB, uid uint64, parentLstEntityId int32) (*UserLink, error) {
	stmt := `SELECT * FROM user_links WHERE user_id = ? AND parent_lst_entity_id = ?`
	res := &UserLink{}
	err := db.Get(res, stmt, uid, parentLstEntityId)
	if err == sql.ErrNoRows {
		err = nil
		res = nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func UpdateUserLink(db *sqlx.DB, id int32, name string) error {
	stmt := `UPDATE user_links SET name = ? WHERE id = ?`
	_, err := db.Exec(stmt, name, id)
	return err
}
