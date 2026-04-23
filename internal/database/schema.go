package database

import (
	"strings"
	"fmt"

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

-- 索引
CREATE INDEX IF NOT EXISTS idx_users_screen_name ON users(screen_name);
CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
CREATE INDEX IF NOT EXISTS idx_users_accessible ON users(is_accessible);
CREATE INDEX IF NOT EXISTS idx_users_protected ON users(protected);
CREATE INDEX IF NOT EXISTS idx_lsts_name ON lsts(name);
CREATE INDEX IF NOT EXISTS idx_lsts_owner ON lsts(owner_uid);
CREATE INDEX IF NOT EXISTS idx_user_entities_user_id ON user_entities(user_id);
CREATE INDEX IF NOT EXISTS idx_user_entities_name ON user_entities(name);
CREATE INDEX IF NOT EXISTS idx_lst_entities_lst_id ON lst_entities(lst_id);
CREATE INDEX IF NOT EXISTS idx_user_links_user_id ON user_links(user_id);
CREATE INDEX IF NOT EXISTS idx_user_links_lst_entity ON user_links(parent_lst_entity_id);
CREATE INDEX IF NOT EXISTS idx_user_previous_names_uid ON user_previous_names(uid);
`

func CreateTables(db *sqlx.DB) {
	db.MustExec(schema)
}

func MigrateDatabase(db *sqlx.DB) error {
	migrations := []string{
		`ALTER TABLE users ADD COLUMN is_accessible BOOLEAN NOT NULL DEFAULT 1`,
	}

	for i, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("migration %d failed: %w", i+1, err)
			}
		}
	}
	return nil
}
