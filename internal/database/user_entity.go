package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
)

func CreateUserEntity(db *sqlx.DB, entity *UserEntity) error {
	abs, err := filepath.Abs(entity.ParentDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for parent dir %q: %w", entity.ParentDir, err)
	}
	entity.ParentDir = abs

	stmt := `INSERT INTO user_entities(user_id, name, parent_dir) VALUES(:user_id, :name, :parent_dir)`
	res, err := db.NamedExec(stmt, entity)
	if err != nil {
		return fmt.Errorf("failed to create user entity for user %d in %q: %w", entity.Uid, entity.ParentDir, err)
	}
	return handleInsertWithId(res, err, func(id int64) { entity.Id.Scan(id) })
}

func DelUserEntity(db *sqlx.DB, id uint32) error {
	stmt := `DELETE FROM user_entities WHERE id=?`
	_, err := db.Exec(stmt, id)
	if err != nil {
		return fmt.Errorf("failed to delete user entity %d: %w", id, err)
	}
	return nil
}

func LocateUserEntity(db *sqlx.DB, uid uint64, parentDIr string) (*UserEntity, error) {
	parentDIr, err := filepath.Abs(parentDIr)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for %q: %w", parentDIr, err)
	}

	stmt := `SELECT * FROM user_entities WHERE user_id=? AND parent_dir=?`
	result := &UserEntity{}
	err = db.Get(result, stmt, uid, parentDIr)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to locate user entity for user %d in %q: %w", uid, parentDIr, err)
	}
	return handleGetResult(result, err)
}

func GetUserEntity(db *sqlx.DB, id int) (*UserEntity, error) {
	result := &UserEntity{}
	stmt := `SELECT * FROM user_entities WHERE id=?`
	err := db.Get(result, stmt, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get user entity %d: %w", id, err)
	}
	return handleGetResult(result, err)
}

func UpdateUserEntity(db *sqlx.DB, entity *UserEntity) error {
	stmt := `UPDATE user_entities SET name=?, latest_release_time=?, media_count=? WHERE id=?`
	result, err := db.Exec(stmt, entity.Name, entity.LatestReleaseTime, entity.MediaCount, entity.Id)
	if err != nil {
		return fmt.Errorf("failed to update user entity %d: %w", entity.Id.Int32, err)
	}
	_, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	return nil
}

func UpdateUserEntityMediCount(db *sqlx.DB, eid int, count int) error {
	stmt := `UPDATE user_entities SET media_count=? WHERE id=?`
	result, err := db.Exec(stmt, count, eid)
	if err != nil {
		return fmt.Errorf("failed to update media count for user entity %d: %w", eid, err)
	}
	_, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	return nil
}

func UpdateUserEntityTweetStat(db *sqlx.DB, eid int, baseline time.Time, count int) error {
	stmt := `UPDATE user_entities SET latest_release_time=?, media_count=? WHERE id=?`
	result, err := db.Exec(stmt, baseline, count, eid)
	if err != nil {
		return fmt.Errorf("failed to update tweet stat for user entity %d: %w", eid, err)
	}
	_, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	return nil
}

func SetUserEntityLatestReleaseTime(db *sqlx.DB, id int, t time.Time) error {
	stmt := `UPDATE user_entities SET latest_release_time=? WHERE id=?`
	result, err := db.Exec(stmt, t, id)
	if err != nil {
		return fmt.Errorf("failed to set latest release time for user entity %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for user entity %d: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("no user entity found with id %d", id)
	}
	return nil
}

func ClearUserEntityLatestReleaseTime(db *sqlx.DB, id int) error {
	stmt := `UPDATE user_entities SET latest_release_time=NULL WHERE id=?`
	result, err := db.Exec(stmt, id)
	if err != nil {
		return fmt.Errorf("failed to clear latest release time for user entity %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for user entity %d: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("no user entity found with id %d", id)
	}
	return nil
}
