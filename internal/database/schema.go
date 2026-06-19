package database

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER NOT NULL, 
	screen_name VARCHAR NOT NULL, 
	name VARCHAR NOT NULL, 
	protected BOOLEAN NOT NULL, 
	friends_count INTEGER NOT NULL, 
	is_accessible BOOLEAN NOT NULL DEFAULT 1,
	PRIMARY KEY (id), 
	UNIQUE (screen_name)
);

CREATE TABLE IF NOT EXISTS user_previous_names (
	id INTEGER NOT NULL, 
	user_id INTEGER NOT NULL, 
	screen_name VARCHAR NOT NULL, 
	name VARCHAR NOT NULL, 
	record_date DATE NOT NULL, 
	PRIMARY KEY (id), 
	FOREIGN KEY(user_id) REFERENCES users (id)
);

CREATE TABLE IF NOT EXISTS lsts (
	id INTEGER NOT NULL, 
	name VARCHAR NOT NULL, 
	owner_user_id INTEGER NOT NULL, 
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

const indexes = `
CREATE INDEX IF NOT EXISTS idx_users_screen_name ON users(screen_name);
CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
CREATE INDEX IF NOT EXISTS idx_users_accessible ON users(is_accessible);
CREATE INDEX IF NOT EXISTS idx_users_protected ON users(protected);
CREATE INDEX IF NOT EXISTS idx_lsts_name ON lsts(name);
CREATE INDEX IF NOT EXISTS idx_lsts_owner ON lsts(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_user_entities_user_id ON user_entities(user_id);
CREATE INDEX IF NOT EXISTS idx_user_entities_name ON user_entities(name);
CREATE INDEX IF NOT EXISTS idx_lst_entities_lst_id ON lst_entities(lst_id);
CREATE INDEX IF NOT EXISTS idx_user_links_user_id ON user_links(user_id);
CREATE INDEX IF NOT EXISTS idx_user_links_lst_entity ON user_links(parent_lst_entity_id);
CREATE INDEX IF NOT EXISTS idx_user_previous_names_user_id ON user_previous_names(user_id);
`

func CreateTables(db *sqlx.DB) {
	db.MustExec(schema)
	db.MustExec(indexes)
}


// renameMigration 描述一条 ALTER TABLE RENAME COLUMN 迁移。
type renameMigration struct {
	table      string // 表名
	newColumn  string // 目标列名（RENAME 之后的列名）
}

// MigrateDatabase keeps idempotent in-place schema upgrades for databases that
// are already readable by the current SQLite driver. These ALTER/RENAME steps
// can be removed after support for upgrading pre-is_accessible / pre-user_id /
// pre-owner_user_id databases is explicitly dropped.
func MigrateDatabase(db *sqlx.DB) error {
	type migrationStep struct {
		sql    string
		rename *renameMigration // 非 nil 表示这是 RENAME COLUMN 迁移
	}
	migrations := []migrationStep{
		{`ALTER TABLE users ADD COLUMN is_accessible BOOLEAN NOT NULL DEFAULT 1`, nil},
		{`ALTER TABLE user_previous_names RENAME COLUMN uid TO user_id`, &renameMigration{"user_previous_names", "user_id"}},
		{`ALTER TABLE lsts RENAME COLUMN owner_uid TO owner_user_id`, &renameMigration{"lsts", "owner_user_id"}},
	}

	for i, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			// ADD COLUMN 失败：列已存在（重复列名）
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			// 表不存在，跳过
			if strings.Contains(err.Error(), "no such table") {
				continue
			}
			// RENAME COLUMN 失败
			if m.rename != nil {
				// "cannot rename" 无法区分具体原因，安全跳过
				if strings.Contains(err.Error(), "cannot rename") {
					continue
				}
				// "no such column"：旧列不存在。验证目标列已存在来区分
				// "已重命名" 和 "真的 schema 损坏"。
				if strings.Contains(err.Error(), "no such column") && columnExists(db, m.rename.table, m.rename.newColumn) {
					continue
				}
			}
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}

// columnExists 检查指定表中是否存在指定列。
func columnExists(db *sqlx.DB, table, column string) bool {
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, table, column); err != nil {
		return false
	}
	return count > 0
}
