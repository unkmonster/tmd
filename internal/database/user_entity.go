package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

func CreateUserEntity(db *sqlx.DB, entity *UserEntity) error {
	abs, err := normalizeEntityParentDir(entity.ParentDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for parent dir %q: %w", entity.ParentDir, err)
	}
	entity.ParentDir = abs

	stmt := `INSERT INTO user_entities(user_id, name, parent_dir) VALUES(:user_id, :name, :parent_dir)`
	res, err := db.NamedExec(stmt, entity)
	if err != nil {
		return fmt.Errorf("failed to create user entity for user %d in %q: %w", entity.UserId, entity.ParentDir, err)
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
	parentDIr, err := normalizeEntityParentDir(parentDIr)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for %q: %w", parentDIr, err)
	}

	stmt := `SELECT * FROM user_entities WHERE user_id=? AND parent_dir=?`
	result := &UserEntity{}
	err = db.Get(result, stmt, uid, parentDIr)
	return handleGetResultWithContext(result, err, "failed to locate user entity for user %d in %q: %w", uid, parentDIr)
}

func GetUserEntity(db *sqlx.DB, id int) (*UserEntity, error) {
	result := &UserEntity{}
	stmt := `SELECT * FROM user_entities WHERE id=?`
	err := db.Get(result, stmt, id)
	return handleGetResultWithContext(result, err, "failed to get user entity %d: %w", id)
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

// UpdateUserEntityFields 仅更新 name 和 media_count，不碰 latest_release_time，
// 避免 PATCH 语义下因读取-写入窗口覆盖其他进程对该字段的修改（丢失更新）。
func UpdateUserEntityFields(db *sqlx.DB, id int, name string, mediaCount sql.NullInt32) error {
	stmt := `UPDATE user_entities SET name=?, media_count=? WHERE id=?`
	_, err := db.Exec(stmt, name, mediaCount, id)
	if err != nil {
		return fmt.Errorf("failed to update user entity %d fields: %w", id, err)
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
	stmt := `UPDATE user_entities
SET latest_release_time=CASE
		WHEN latest_release_time IS NULL OR latest_release_time < ? THEN ?
		ELSE latest_release_time
	END,
	media_count=CASE
		WHEN media_count IS NULL OR media_count < ? THEN ?
		ELSE media_count
	END
WHERE id=?`
	result, err := db.Exec(stmt, baseline, baseline, count, count, eid)
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
