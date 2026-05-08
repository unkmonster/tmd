package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

var requiredSchema = map[string][]string{
	"users":               {"id", "screen_name", "name", "protected", "friends_count", "is_accessible"},
	"user_previous_names": {"id", "user_id", "screen_name", "name", "record_date"},
	"lsts":                {"id", "name", "owner_user_id"},
	"lst_entities":        {"id", "lst_id", "name", "parent_dir"},
	"user_entities":       {"id", "user_id", "name", "latest_release_time", "parent_dir", "media_count"},
	"user_links":          {"id", "user_id", "name", "parent_lst_entity_id"},
}

func validateDatabase(db *sqlx.DB) error {
	for tableName, columns := range requiredSchema {
		existingColumns, err := getTableColumns(db, tableName)
		if err != nil {
			return err
		}
		if len(existingColumns) == 0 {
			return fmt.Errorf("table %q is missing", tableName)
		}
		for _, columnName := range columns {
			if !existingColumns[columnName] {
				return fmt.Errorf("table %q is missing column %q", tableName, columnName)
			}
		}
	}
	return nil
}

func getTableColumns(queryer interface {
	Queryx(query string, args ...interface{}) (*sqlx.Rows, error)
}, tableName string) (map[string]bool, error) {
	rows, err := queryer.Queryx(fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdentifier(tableName)))
	if err != nil {
		return nil, fmt.Errorf("failed to inspect schema for table %q: %w", tableName, err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var (
			cid      int
			name     string
			dataType string
			notNull  int
			defaultV sql.NullString
			primaryK int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultV, &primaryK); err != nil {
			return nil, fmt.Errorf("failed to scan schema metadata for table %q: %w", tableName, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while reading schema metadata for table %q: %w", tableName, err)
	}
	return columns, nil
}

func tableExists(queryer interface {
	Get(dest interface{}, query string, args ...interface{}) error
}, tableName string) (bool, error) {
	var count int
	if err := queryer.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName); err != nil {
		return false, fmt.Errorf("failed to inspect table %q: %w", tableName, err)
	}
	return count > 0, nil
}

func quoteSQLiteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
