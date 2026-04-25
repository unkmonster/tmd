package database

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/jmoiron/sqlx"
)

func CreateLstEntity(db *sqlx.DB, entity *LstEntity) error {
	abs, err := filepath.Abs(entity.ParentDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for parent dir %q: %w", entity.ParentDir, err)
	}
	entity.ParentDir = abs

	stmt := `INSERT INTO lst_entities(id, lst_id, name, parent_dir) VALUES(:id, :lst_id, :name, :parent_dir)`
	res, err := db.NamedExec(stmt, &entity)
	if err != nil {
		return fmt.Errorf("failed to create list entity for list %d in %q: %w", entity.LstId, entity.ParentDir, err)
	}
	return handleInsertWithId(res, err, func(id int64) { entity.Id.Scan(id) })
}

func DelLstEntity(db *sqlx.DB, id int) error {
	stmt := `DELETE FROM lst_entities WHERE id=?`
	_, err := db.Exec(stmt, id)
	if err != nil {
		return fmt.Errorf("failed to delete list entity %d: %w", id, err)
	}
	return nil
}

func GetLstEntity(db *sqlx.DB, id int) (*LstEntity, error) {
	return GetLstEntityWithTx(db, id)
}

// GetLstEntityWithTx 支持在事务中查询 lst_entities
func GetLstEntityWithTx(queryer interface {
	Get(dest interface{}, query string, args ...interface{}) error
}, id int) (*LstEntity, error) {
	stmt := `SELECT * FROM lst_entities WHERE id=?`
	result := &LstEntity{}
	err := queryer.Get(result, stmt, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get list entity %d: %w", id, err)
	}
	return handleGetResult(result, err)
}

func LocateLstEntity(db *sqlx.DB, lid int64, parentDir string) (*LstEntity, error) {
	parentDir, err := filepath.Abs(parentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for %q: %w", parentDir, err)
	}

	stmt := `SELECT * FROM lst_entities WHERE lst_id=? AND parent_dir=?`
	result := &LstEntity{}
	err = db.Get(result, stmt, lid, parentDir)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to locate list entity for list %d in %q: %w", lid, parentDir, err)
	}
	return handleGetResult(result, err)
}

func UpdateLstEntity(db *sqlx.DB, entity *LstEntity) error {
	stmt := `UPDATE lst_entities SET name=? WHERE id=?`
	_, err := db.Exec(stmt, entity.Name, entity.Id.Int32)
	if err != nil {
		return fmt.Errorf("failed to update list entity %d: %w", entity.Id.Int32, err)
	}
	return nil
}
