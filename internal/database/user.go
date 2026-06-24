package database

import (
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// ErrUserNotFound 标记"用户不存在"错误，供调用方用 errors.Is 判断。
var ErrUserNotFound = errors.New("user not found")

func CreateUser(db *sqlx.DB, usr *User) error {
	stmt := `INSERT INTO Users(id, screen_name, name, protected, friends_count, is_accessible) VALUES(:id, :screen_name, :name, :protected, :friends_count, :is_accessible)`
	_, err := db.NamedExec(stmt, usr)
	if err != nil {
		return fmt.Errorf("failed to create user %d (%s): %w", usr.Id, usr.ScreenName, err)
	}
	return nil
}

func GetUserById(db *sqlx.DB, uid uint64) (*User, error) {
	stmt := `SELECT * FROM users WHERE id=?`
	result := &User{}
	err := db.Get(result, stmt, uid)
	return handleGetResult(result, err)
}

func SetUserAccessible(db *sqlx.DB, uid uint64, accessible bool) error {
	stmt := `UPDATE users SET is_accessible=? WHERE id=?`
	result, err := db.Exec(stmt, accessible, uid)
	if err != nil {
		return fmt.Errorf("failed to set accessible status for user %d: %w", uid, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for user %d: %w", uid, err)
	}
	if rows == 0 {
		return fmt.Errorf("user %d not found, cannot set accessible status: %w", uid, ErrUserNotFound)
	}
	return nil
}

func SetUserAccessibleByScreenName(db *sqlx.DB, screenName string, accessible bool) error {
	stmt := `UPDATE users SET is_accessible=? WHERE screen_name=?`
	result, err := db.Exec(stmt, accessible, screenName)
	if err != nil {
		return fmt.Errorf("failed to set accessible status for user %s: %w", screenName, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for user %s: %w", screenName, err)
	}
	if rows == 0 {
		return fmt.Errorf("user %s not found, cannot set accessible status: %w", screenName, ErrUserNotFound)
	}
	return nil
}

func UpdateUser(db *sqlx.DB, usr *User) error {
	stmt := `UPDATE users SET screen_name=:screen_name, name=:name, protected=:protected, friends_count=:friends_count, is_accessible=:is_accessible WHERE id=:id`
	res, err := db.NamedExec(stmt, usr)
	if err != nil {
		return fmt.Errorf("failed to update user %d: %w", usr.Id, err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	return nil
}

func SetUsersAccessible(db *sqlx.DB, uids []uint64) error {
	if len(uids) == 0 {
		return nil
	}

	query := `UPDATE users SET is_accessible=1 WHERE id IN (?)`
	query, args, err := sqlx.In(query, uids)
	if err != nil {
		return fmt.Errorf("failed to build batch update query: %w", err)
	}

	query = db.Rebind(query)
	_, err = db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to batch update users accessible status: %w", err)
	}
	return nil
}

func MarkListMembersAccessibleByIDs(db *sqlx.DB, uids []uint64) error {
	if len(uids) == 0 || db == nil {
		return nil
	}

	if err := SetUsersAccessible(db, uids); err != nil {
		return fmt.Errorf("failed to mark list members as accessible: %w", err)
	}
	return nil
}

func RecordUserPreviousName(db *sqlx.DB, uid uint64, name string, screenName string) error {
	stmt := `INSERT INTO user_previous_names(user_id, screen_name, name, record_date) VALUES(?, ?, ?, ?)`
	_, err := db.Exec(stmt, uid, screenName, name, time.Now())
	if err != nil {
		return fmt.Errorf("failed to record previous name for user %d (%s -> %s): %w", uid, screenName, name, err)
	}
	return nil
}

func MarkUserInaccessible(db *sqlx.DB, uid uint64, screenName string) {
	if uid > 0 {
		if markErr := SetUserAccessible(db, uid, false); markErr != nil {
			if !errors.Is(markErr, ErrUserNotFound) {
				log.Warnln("[db] Failed to mark user as inaccessible:", uid, markErr)
			}
		}
	} else if screenName != "" {
		if markErr := SetUserAccessibleByScreenName(db, screenName, false); markErr != nil {
			if !errors.Is(markErr, ErrUserNotFound) {
				log.Warnln("[db] Failed to mark user as inaccessible by screen_name:", screenName, markErr)
			}
		}
	}
}

func DelUser(db *sqlx.DB, uid uint64) (err error) {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec("DELETE FROM user_links WHERE user_id = ?", uid)
	if err != nil {
		return fmt.Errorf("failed to delete user links for user %d: %w", uid, err)
	}

	_, err = tx.Exec("DELETE FROM user_entities WHERE user_id = ?", uid)
	if err != nil {
		return fmt.Errorf("failed to delete user entities for user %d: %w", uid, err)
	}

	_, err = tx.Exec("DELETE FROM user_previous_names WHERE user_id = ?", uid)
	if err != nil {
		return fmt.Errorf("failed to delete previous names for user %d: %w", uid, err)
	}

	_, err = tx.Exec("DELETE FROM users WHERE id = ?", uid)
	if err != nil {
		return fmt.Errorf("failed to delete user %d: %w", uid, err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
