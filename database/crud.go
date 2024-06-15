package database

import (
	"fmt"

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
	title VARCHAR NOT NULL, 
	parent_dir VARCHAR NOT NULL, 
	PRIMARY KEY (id), 
	UNIQUE (lst_id, parent_dir), 
	FOREIGN KEY(lst_id) REFERENCES lsts (id)
);

CREATE TABLE IF NOT EXISTS user_entities (
	id INTEGER NOT NULL, 
	user_id INTEGER NOT NULL, 
	title VARCHAR NOT NULL, 
	latest_release_time DATETIME, 
	parent_dir VARCHAR, 
	parent_lst_entity_id INTEGER, 
	PRIMARY KEY (id), 
	UNIQUE (user_id, parent_dir, parent_lst_entity_id), 
	CHECK (parent_dir IS NOT NULL OR parent_lst_entity_id IS NOT NULL), 
	CHECK (parent_dir IS NULL OR parent_lst_entity_id IS NULL), 
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
	result := User{}
	if err := db.Get(&result, stmt, uid); err != nil {
		return nil, err
	}
	return &result, nil
}

func UpdateUser(db *sqlx.DB, usr *User) error {
	stmt := fmt.Sprintf(`UPDATE users SET screen_name=:screen_name, name=:name, protected=:protected, friends_count=:friends_count WHERE id=%d`, usr.Id)
	_, err := db.NamedExec(stmt, usr)
	return err
}

func CreateUserEntity(db *sqlx.DB, entity *UserEntity) error {
	stmt := `INSERT INTO user_entities(user_id, title, parent_dir, parent_lst_entity_id) VALUES(:user_id, :title, :parent_dir, :parent_lst_entity_id)`
	_, err := db.NamedExec(stmt, entity)
	return err
}

func DelUserEntity(db *sqlx.DB, id uint32) error {
	stmt := `DELETE FROM user_entities WHERE id=?`
	_, err := db.Exec(stmt, id)
	return err
}

func LocateUserEntityInDir(db *sqlx.DB, uid uint64, parentDIr string) (*UserEntity, error) {
	stmt := `SELECT * FROM user_entities WHERE uid=? AND parent_dir=?`
	result := UserEntity{}
	if err := db.Get(&result, stmt, uid, parentDIr); err != nil {
		return nil, err
	}
	return &result, nil
}

func LocateUserEntityInLst(db *sqlx.DB, uid uint64, lstEid uint) (*UserEntity, error) {
	result := UserEntity{}
	stmt := `SELECT * FROM user_entities WHERE uid=? AND parent_lst_entity_id=?`
	if err := db.Get(&result, stmt, uid, lstEid); err != nil {
		return nil, err
	}
	return &result, nil
}

func GetUserEntity(db *sqlx.DB, id int) (*UserEntity, error) {
	result := UserEntity{}
	stmt := `SELECT * FROM user_entities WHERE id=?`
	if err := db.Get(&result, stmt, id); err != nil {
		return nil, err
	}
	return &result, nil
}

func UpdateUserEntity(db *sqlx.DB, entity *UserEntity) error {
	stmt := `UPDATE user_entities SET title=?, latest_release_time=? WHERE id=?`
	_, err := db.Exec(stmt, entity.Title, entity.LatestReleaseTime, entity.Id)
	return err
}
