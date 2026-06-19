package database

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"
)

// migrateExistingDatabase validates an existing SQLite file and rebuilds it
// from a backup if the current driver/schema cannot use it directly. This
// compatibility path exists for pre-modernc / pre-current-schema databases and
// can be removed after support for upgrading those legacy on-disk databases is
// explicitly dropped.
func migrateExistingDatabase(path string) error {
	sourceDB, err := openSQLiteFile(path, false)
	if err == nil {
		validateErr := validateDatabase(sourceDB)
		closeErr := sourceDB.Close()
		if validateErr == nil {
			if closeErr != nil {
				return fmt.Errorf("failed to close database %q after validation: %w", path, closeErr)
			}
			return nil
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close database %q before migration: %w", path, closeErr)
		}
	}

	backupPath, err := backupSQLiteFiles(path)
	if err != nil {
		return fmt.Errorf("failed to back up database %q before migration: %w", path, err)
	}

	// 迁移过程中清理备份；成功后保留作为最终安全网
	removeBackup := true
	defer func() {
		if removeBackup {
			removeSQLiteFiles(backupPath)
		}
	}()

	sourceDB, err = openSQLiteFile(backupPath, false)
	if err != nil {
		return fmt.Errorf("database %q requires migration but backup %q cannot be opened with %s: %w", path, backupPath, DriverName, err)
	}
	defer sourceDB.Close()

	tempPath := path + ".migrating"
	removeSQLiteFiles(tempPath)

	targetDB, err := openSQLiteFile(tempPath, false)
	if err != nil {
		return fmt.Errorf("failed to create migration target database %q: %w", tempPath, err)
	}

	configureSQLiteConnection(targetDB)
	CreateTables(targetDB)
	if err := MigrateDatabase(targetDB); err != nil {
		targetDB.Close()
		removeSQLiteFiles(tempPath)
		return fmt.Errorf("failed to prepare migration target database %q: %w", tempPath, err)
	}

	if err := copyAllData(sourceDB, targetDB); err != nil {
		targetDB.Close()
		removeSQLiteFiles(tempPath)
		return fmt.Errorf("failed to migrate data from %q to %q: %w", backupPath, tempPath, err)
	}

	if err := validateDatabase(targetDB); err != nil {
		targetDB.Close()
		removeSQLiteFiles(tempPath)
		return fmt.Errorf("migrated database %q failed validation: %w", tempPath, err)
	}

	if err := targetDB.Close(); err != nil {
		removeSQLiteFiles(tempPath)
		return fmt.Errorf("failed to close migration target database %q: %w", tempPath, err)
	}

	if err := replaceSQLiteFiles(path, tempPath); err != nil {
		return fmt.Errorf("failed to replace database %q with migrated copy %q: %w", path, tempPath, err)
	}

	removeBackup = false
	return nil
}

func backupSQLiteFiles(path string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	backupPath := path + ".backup." + timestamp

	if err := copyFile(path, backupPath); err != nil {
		return "", err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := copyFileIfExists(path+suffix, backupPath+suffix); err != nil {
			return "", err
		}
	}
	return backupPath, nil
}

func copyFile(srcPath string, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", srcPath, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", dstPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy %q to %q: %w", srcPath, dstPath, err)
	}
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync %q: %w", dstPath, err)
	}
	return nil
}

func copyFileIfExists(srcPath string, dstPath string) error {
	exists, err := fileExists(srcPath)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return copyFile(srcPath, dstPath)
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func removeSQLiteFiles(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
}

func replaceSQLiteFiles(path string, tempPath string) error {
	holdingPath := path + ".replacing"
	removeSQLiteFiles(holdingPath)

	if err := moveSQLiteFiles(path, holdingPath, true); err != nil {
		return fmt.Errorf("failed to move current database aside: %w", err)
	}

	if err := moveSQLiteFiles(tempPath, path, true); err != nil {
		if restoreErr := moveSQLiteFiles(holdingPath, path, true); restoreErr != nil {
			return fmt.Errorf("failed to move migrated database into place: %w; additionally failed to restore original database: %v", err, restoreErr)
		}
		return fmt.Errorf("failed to move migrated database into place; original database was restored: %w", err)
	}

	removeSQLiteFiles(holdingPath)
	return nil
}

func moveSQLiteFiles(srcPath string, dstPath string, requireMain bool) error {
	movedSuffixes := make([]string, 0, 3)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := srcPath + suffix
		dst := dstPath + suffix

		exists, err := fileExists(src)
		if err != nil {
			rollbackSQLiteFileMoves(dstPath, srcPath, movedSuffixes)
			return fmt.Errorf("failed to inspect %q: %w", src, err)
		}
		if !exists {
			if suffix == "" && requireMain {
				rollbackSQLiteFileMoves(dstPath, srcPath, movedSuffixes)
				return fmt.Errorf("required SQLite file %q does not exist", src)
			}
			continue
		}

		_ = os.Remove(dst)
		if err := os.Rename(src, dst); err != nil {
			rollbackSQLiteFileMoves(dstPath, srcPath, movedSuffixes)
			return fmt.Errorf("failed to move %q to %q: %w", src, dst, err)
		}
		movedSuffixes = append(movedSuffixes, suffix)
	}
	return nil
}

func rollbackSQLiteFileMoves(srcPath string, dstPath string, suffixes []string) {
	for i := len(suffixes) - 1; i >= 0; i-- {
		suffix := suffixes[i]
		_ = os.Rename(srcPath+suffix, dstPath+suffix)
	}
}

func copyAllData(sourceDB *sqlx.DB, targetDB *sqlx.DB) error {
	sourceTx, err := sourceDB.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin source transaction: %w", err)
	}
	defer sourceTx.Rollback()

	tx, err := targetDB.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin target transaction: %w", err)
	}
	defer tx.Rollback()

	if err := copyUsers(sourceTx, tx); err != nil {
		return err
	}
	if err := copyUserPreviousNames(sourceTx, tx); err != nil {
		return err
	}
	if err := copyLsts(sourceTx, tx); err != nil {
		return err
	}
	if err := copyUserEntities(sourceTx, tx); err != nil {
		return err
	}
	if err := copyLstEntities(sourceTx, tx); err != nil {
		return err
	}
	if err := copyUserLinks(sourceTx, tx); err != nil {
		return err
	}

	if err := validateRowCounts(sourceTx, tx); err != nil {
		return err
	}

	if err := sourceTx.Commit(); err != nil {
		return fmt.Errorf("failed to commit source transaction: %w", err)
	}
	return tx.Commit()
}

func copyUsers(sourceDB *sqlx.Tx, targetTx *sqlx.Tx) error {
	userColumns, err := getTableColumns(sourceDB, "users")
	if err != nil {
		return err
	}

	isAccessibleExpr := "1 AS is_accessible"
	if userColumns["is_accessible"] {
		isAccessibleExpr = "is_accessible"
	}

	query := fmt.Sprintf(
		"SELECT id, screen_name, name, protected, friends_count, %s FROM users ORDER BY id",
		isAccessibleExpr,
	)

	var users []*User
	if err := sourceDB.Select(&users, query); err != nil {
		return fmt.Errorf("failed to load users for migration: %w", err)
	}
	for _, user := range users {
		if _, err := targetTx.NamedExec(
			`INSERT INTO users(id, screen_name, name, protected, friends_count, is_accessible)
			 VALUES(:id, :screen_name, :name, :protected, :friends_count, :is_accessible)`,
			user,
		); err != nil {
			return fmt.Errorf("failed to insert user %d during migration: %w", user.Id, err)
		}
	}
	return nil
}

func copyUserPreviousNames(sourceDB *sqlx.Tx, targetTx *sqlx.Tx) error {
	exists, err := tableExists(sourceDB, "user_previous_names")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	columns, err := getTableColumns(sourceDB, "user_previous_names")
	if err != nil {
		return err
	}

	userIDColumn := "user_id"
	if columns["uid"] && !columns["user_id"] {
		userIDColumn = "uid"
	}

	query := fmt.Sprintf(
		"SELECT id, %s AS user_id, screen_name, name, record_date FROM user_previous_names ORDER BY id",
		userIDColumn,
	)

	var records []*UserPreviousName
	if err := sourceDB.Select(&records, query); err != nil {
		return fmt.Errorf("failed to load user_previous_names for migration: %w", err)
	}
	for _, record := range records {
		if _, err := targetTx.Exec(
			`INSERT INTO user_previous_names(id, user_id, screen_name, name, record_date)
			 VALUES(?, ?, ?, ?, ?)`,
			record.Id,
			record.UserId,
			record.ScreenName,
			record.Name,
			record.RecordDate,
		); err != nil {
			return fmt.Errorf("failed to insert user_previous_name %d during migration: %w", record.Id, err)
		}
	}
	return nil
}

func copyLsts(sourceDB *sqlx.Tx, targetTx *sqlx.Tx) error {
	exists, err := tableExists(sourceDB, "lsts")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	columns, err := getTableColumns(sourceDB, "lsts")
	if err != nil {
		return err
	}

	ownerUserIDColumn := "owner_user_id"
	if columns["owner_uid"] && !columns["owner_user_id"] {
		ownerUserIDColumn = "owner_uid"
	}

	query := fmt.Sprintf(
		"SELECT id, name, %s AS owner_user_id FROM lsts ORDER BY id",
		ownerUserIDColumn,
	)

	var lists []*Lst
	if err := sourceDB.Select(&lists, query); err != nil {
		return fmt.Errorf("failed to load lsts for migration: %w", err)
	}
	for _, lst := range lists {
		if _, err := targetTx.Exec(
			`INSERT INTO lsts(id, name, owner_user_id) VALUES(?, ?, ?)`,
			lst.Id,
			lst.Name,
			lst.OwnerUserId,
		); err != nil {
			return fmt.Errorf("failed to insert list %d during migration: %w", lst.Id, err)
		}
	}
	return nil
}

func copyUserEntities(sourceDB *sqlx.Tx, targetTx *sqlx.Tx) error {
	exists, err := tableExists(sourceDB, "user_entities")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	var entities []*UserEntity
	if err := sourceDB.Select(&entities, "SELECT id, user_id, name, latest_release_time, parent_dir, media_count FROM user_entities ORDER BY id"); err != nil {
		return fmt.Errorf("failed to load user_entities for migration: %w", err)
	}
	for _, entity := range entities {
		if _, err := targetTx.Exec(
			`INSERT INTO user_entities(id, user_id, name, latest_release_time, parent_dir, media_count)
			 VALUES(?, ?, ?, ?, ?, ?)`,
			nullInt32Value(entity.Id),
			entity.UserId,
			entity.Name,
			nullTimeValue(entity.LatestReleaseTime),
			entity.ParentDir,
			nullInt32Value(entity.MediaCount),
		); err != nil {
			return fmt.Errorf("failed to insert user entity %d during migration: %w", entity.Id.Int32, err)
		}
	}
	return nil
}

func copyLstEntities(sourceDB *sqlx.Tx, targetTx *sqlx.Tx) error {
	exists, err := tableExists(sourceDB, "lst_entities")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	var entities []*LstEntity
	if err := sourceDB.Select(&entities, "SELECT id, lst_id, name, parent_dir FROM lst_entities ORDER BY id"); err != nil {
		return fmt.Errorf("failed to load lst_entities for migration: %w", err)
	}
	for _, entity := range entities {
		if _, err := targetTx.Exec(
			`INSERT INTO lst_entities(id, lst_id, name, parent_dir) VALUES(?, ?, ?, ?)`,
			nullInt32Value(entity.Id),
			entity.LstId,
			entity.Name,
			entity.ParentDir,
		); err != nil {
			return fmt.Errorf("failed to insert list entity %d during migration: %w", entity.Id.Int32, err)
		}
	}
	return nil
}

func copyUserLinks(sourceDB *sqlx.Tx, targetTx *sqlx.Tx) error {
	exists, err := tableExists(sourceDB, "user_links")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	var links []*UserLink
	if err := sourceDB.Select(&links, "SELECT id, user_id, name, parent_lst_entity_id FROM user_links ORDER BY id"); err != nil {
		return fmt.Errorf("failed to load user_links for migration: %w", err)
	}
	for _, link := range links {
		if _, err := targetTx.Exec(
			`INSERT INTO user_links(id, user_id, name, parent_lst_entity_id) VALUES(?, ?, ?, ?)`,
			link.Id,
			link.UserId,
			link.Name,
			link.ParentLstEntityId,
		); err != nil {
			return fmt.Errorf("failed to insert user link %d during migration: %w", link.Id, err)
		}
	}
	return nil
}

func validateRowCounts(sourceDB *sqlx.Tx, targetQueryer interface {
	Get(dest interface{}, query string, args ...interface{}) error
}) error {
	tableNames := make([]string, 0, len(requiredSchema))
	for tableName := range requiredSchema {
		tableNames = append(tableNames, tableName)
	}
	sort.Strings(tableNames)

	for _, tableName := range tableNames {
		sourceCount := 0
		exists, err := tableExists(sourceDB, tableName)
		if err != nil {
			return err
		}
		if exists {
			if err := sourceDB.Get(&sourceCount, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteSQLiteIdentifier(tableName))); err != nil {
				return fmt.Errorf("failed to count rows in source table %q: %w", tableName, err)
			}
		}

		var targetCount int
		if err := targetQueryer.Get(&targetCount, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteSQLiteIdentifier(tableName))); err != nil {
			return fmt.Errorf("failed to count rows in target table %q: %w", tableName, err)
		}

		if sourceCount != targetCount {
			return fmt.Errorf("row count mismatch for table %q: source=%d target=%d", tableName, sourceCount, targetCount)
		}
	}
	return nil
}

func nullInt32Value(value sql.NullInt32) interface{} {
	if value.Valid {
		return value.Int32
	}
	return nil
}

func nullTimeValue(value sql.NullTime) interface{} {
	if value.Valid {
		return value.Time
	}
	return nil
}
