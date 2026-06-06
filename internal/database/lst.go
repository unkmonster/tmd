package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

func CreateLst(db *sqlx.DB, lst *Lst) error {
	stmt := `INSERT INTO lsts(id, name, owner_user_id) VALUES(:id, :name, :owner_user_id)`
	_, err := db.NamedExec(stmt, &lst)
	if err != nil {
		return fmt.Errorf("failed to create list %d (%s): %w", lst.Id, lst.Name, err)
	}
	return nil
}

func GetLst(db *sqlx.DB, lid uint64) (*Lst, error) {
	stmt := `SELECT * FROM lsts WHERE id = ?`
	result := &Lst{}
	err := db.Get(result, stmt, lid)
	return handleGetResultWithContext(result, err, "failed to get list %d: %w", lid)
}

func UpdateLst(db *sqlx.DB, lst *Lst) error {
	stmt := `UPDATE lsts SET name=?, owner_user_id=? WHERE id=?`
	result, err := db.Exec(stmt, lst.Name, lst.OwnerUserId, lst.Id)
	if err != nil {
		return fmt.Errorf("failed to update list %d: %w", lst.Id, err)
	}
	_, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	return nil
}

func DelLst(db *sqlx.DB, lid uint64) (err error) {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec("DELETE FROM user_links WHERE parent_lst_entity_id IN (SELECT id FROM lst_entities WHERE lst_id = ?)", lid)
	if err != nil {
		return fmt.Errorf("failed to delete user links for list %d: %w", lid, err)
	}

	_, err = tx.Exec("DELETE FROM lst_entities WHERE lst_id = ?", lid)
	if err != nil {
		return fmt.Errorf("failed to delete lst entities for list %d: %w", lid, err)
	}

	_, err = tx.Exec("DELETE FROM lsts WHERE id = ?", lid)
	if err != nil {
		return fmt.Errorf("failed to delete list %d: %w", lid, err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
