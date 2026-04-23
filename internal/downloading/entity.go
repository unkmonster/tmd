package downloading

import (
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
		err = os.Symlink(path, linkpath)
		if os.IsExist(err) {
			err = nil
		}
		return err
	}

	newlinkpath := filepath.Join(linkDir, name)

	if err = os.RemoveAll(linkpath); err != nil {
		return err
	}
	if err = os.Symlink(path, newlinkpath); err != nil && !os.IsExist(err) {
		return err
	}

	if err = database.UpdateUserLink(db, lnk.Id, name); err != nil {
		return err
	}

	lnk.Name = name
	return nil
}
