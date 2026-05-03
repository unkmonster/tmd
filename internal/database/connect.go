package database

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/utils"
	log "github.com/sirupsen/logrus"
)

func Connect(path string) (*sqlx.DB, error) {
	ex, err := utils.PathExists(path)
	if err != nil {
		return nil, fmt.Errorf("failed to check if db file exists at %q: %w", path, err)
	}

	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&busy_timeout=30000", path)
	db, err := sqlx.Connect("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database at %q: %w", path, err)
	}

	// 配置连接池：SQLite 为文件型数据库，限制连接数以避免并发问题
	db.SetMaxOpenConns(1)                  // 最多 1 个打开的连接（SQLite 并发写入限制）
	db.SetMaxIdleConns(1)                  // 保持 1 个空闲连接
	db.SetConnMaxLifetime(30 * time.Minute) // 连接最大生命周期
	db.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接最大保持时间

	CreateTables(db)
	if err := MigrateDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database at %q: %w", path, err)
	}
	CreateIndexes(db)

	if !ex {
		log.Debugln("created new db file", path)
	}
	return db, nil
}
