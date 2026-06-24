package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/utils"
)

func Connect(path string) (*sqlx.DB, error) {
	exists, err := utils.PathExists(path)
	if err != nil {
		return nil, fmt.Errorf("failed to check if db file exists at %q: %w", path, err)
	}

	if exists {
		if err := migrateExistingDatabase(path); err != nil {
			return nil, err
		}
	}

	db, err := openSQLiteFile(path, false)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database at %q: %w", path, err)
	}

	configureSQLiteConnection(db)

	CreateTables(db)
	if err := MigrateDatabase(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database at %q: %w", path, err)
	}

	if err := validateDatabase(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("database at %q failed validation after initialization: %w", path, err)
	}

	if !exists {
		log.Debugln("[db] Created new db file", path)
	}
	return db, nil
}
