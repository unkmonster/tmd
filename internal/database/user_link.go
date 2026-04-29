package database

import (
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

func CreateUserLink(db *sqlx.DB, lnk *UserLink) error {
	stmt := `INSERT INTO user_links(user_id, name, parent_lst_entity_id) VALUES(:user_id, :name, :parent_lst_entity_id)`
	res, err := db.NamedExec(stmt, lnk)
	if err != nil {
		return fmt.Errorf("failed to create user link for user %d in list entity %d: %w", lnk.UserId, lnk.ParentLstEntityId, err)
	}
	return handleInsertWithId(res, err, func(id int64) { lnk.Id = int32(id) })
}

func GetUserLinks(db *sqlx.DB, uid uint64) ([]*UserLink, error) {
	stmt := `SELECT * FROM user_links WHERE user_id = ?`
	res := []*UserLink{}
	err := db.Select(&res, stmt, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to get user links for user %d: %w", uid, err)
	}
	return res, nil
}

func GetUserLink(db *sqlx.DB, uid uint64, parentLstEntityId int32) (*UserLink, error) {
	stmt := `SELECT * FROM user_links WHERE user_id = ? AND parent_lst_entity_id = ?`
	res := &UserLink{}
	err := db.Get(res, stmt, uid, parentLstEntityId)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get user link for user %d in list entity %d: %w", uid, parentLstEntityId, err)
	}
	return handleGetResult(res, err)
}

func UpdateUserLink(db *sqlx.DB, id int32, name string) error {
	stmt := `UPDATE user_links SET name = ? WHERE id = ?`
	_, err := db.Exec(stmt, name, id)
	if err != nil {
		return fmt.Errorf("failed to update user link %d: %w", id, err)
	}
	return nil
}

func GetUserLinkById(db *sqlx.DB, id int32) (*UserLink, error) {
	stmt := `SELECT * FROM user_links WHERE id = ?`
	res := &UserLink{}
	err := db.Get(res, stmt, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get user link %d: %w", id, err)
	}
	return handleGetResult(res, err)
}

func DelUserLink(db *sqlx.DB, id int32) error {
	stmt := `DELETE FROM user_links WHERE id=?`
	_, err := db.Exec(stmt, id)
	if err != nil {
		return fmt.Errorf("failed to delete user link %d: %w", id, err)
	}
	return nil
}

func GetUserLinksByLstEntityId(queryer interface {
	Select(dest interface{}, query string, args ...interface{}) error
}, lstEntityId int) ([]*UserLink, error) {
	stmt := `SELECT * FROM user_links WHERE parent_lst_entity_id = ?`
	res := []*UserLink{}
	err := queryer.Select(&res, stmt, lstEntityId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user links for list entity %d: %w", lstEntityId, err)
	}
	return res, nil
}
