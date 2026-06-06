package downloading

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/database"
)

func updateUserLink(lnk *database.UserLink, db *sqlx.DB, path string) error {
	name := filepath.Base(path)

	linkpath, err := lnk.Path(db)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	linkDir := filepath.Dir(linkpath)
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		return err
	}

	if lnk.Name == name {
		return ensureUserSymlink(path, linkpath)
	}

	newlinkpath := filepath.Join(linkDir, name)

	if err = os.RemoveAll(linkpath); err != nil {
		return err
	}
	if err = ensureUserSymlink(path, newlinkpath); err != nil {
		return err
	}

	if err = database.UpdateUserLink(db, lnk.Id, name); err != nil {
		return err
	}

	lnk.Name = name
	return nil
}

func ensureUserSymlink(targetPath, linkPath string) error {
	if err := os.Symlink(targetPath, linkPath); err != nil {
		if !os.IsExist(err) {
			return err
		}
		return replaceStaleUserSymlink(targetPath, linkPath, err)
	}
	return nil
}

func replaceStaleUserSymlink(targetPath, linkPath string, existErr error) error {
	currentTarget, err := os.Readlink(linkPath)
	if err != nil {
		return existErr
	}
	if !filepath.IsAbs(currentTarget) {
		currentTarget = filepath.Join(filepath.Dir(linkPath), currentTarget)
	}
	currentTarget, err = filepath.Abs(currentTarget)
	if err != nil {
		return err
	}
	targetPath, err = filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	if currentTarget == targetPath {
		return nil
	}

	backupPath := linkPath + ".stale"
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(linkPath, backupPath); err != nil {
		return err
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		if restoreErr := os.Rename(backupPath, linkPath); restoreErr != nil {
			os.Remove(backupPath) // best-effort cleanup of .stale
			return fmt.Errorf("symlink failed: %w (restore rename also failed: %v)", err, restoreErr)
		}
		return err
	}
	return os.Remove(backupPath)
}
