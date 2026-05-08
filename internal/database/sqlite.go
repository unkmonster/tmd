package database

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const DriverName = "sqlite"

const (
	sqliteBusyTimeoutMs = 30000
	sqliteTimeFormat    = "sqlite"
)

func MemoryDSN(sharedCache bool) string {
	query := url.Values{}
	if sharedCache {
		query.Set("cache", "shared")
	}
	addSQLiteQueryDefaults(query)
	return "file::memory:?" + query.Encode()
}

func FileDSN(path string, sharedCache bool) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %q: %w", path, err)
	}
	return sqliteFileDSN(absPath, sharedCache), nil
}

func MustFileDSN(path string, sharedCache bool) string {
	dsn, err := FileDSN(path, sharedCache)
	if err != nil {
		panic(err)
	}
	return dsn
}

func sqliteFileDSN(path string, sharedCache bool) string {
	query := url.Values{}
	if sharedCache {
		query.Set("cache", "shared")
	}
	addSQLiteQueryDefaults(query)

	normalizedPath := filepath.ToSlash(path)
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}

	return (&url.URL{
		Scheme:   "file",
		Path:     normalizedPath,
		RawQuery: query.Encode(),
	}).String()
}

func addSQLiteQueryDefaults(query url.Values) {
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", sqliteBusyTimeoutMs))
	query.Set("_time_format", sqliteTimeFormat)
}

func openSQLiteFile(path string, sharedCache bool) (*sqlx.DB, error) {
	dsn, err := FileDSN(path, sharedCache)
	if err != nil {
		return nil, err
	}
	return sqlx.Connect(DriverName, dsn)
}

func configureSQLiteConnection(db *sqlx.DB) {
	// SQLite 为文件型数据库，限制连接数以避免并发问题。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(10 * time.Minute)
}
