package downloading

import (
	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/entity"
	"github.com/unkmonster/tmd/internal/naming"
	"github.com/unkmonster/tmd/internal/twitter"
)

func syncUser(db *sqlx.DB, user *twitter.User, accessible bool) error {
	return database.SyncUser(db, user.Id, user.Name, user.ScreenName, user.IsProtected, user.FriendsCount, accessible)
}

func syncUserAndEntity(db *sqlx.DB, user *twitter.User, dir string) (*entity.UserEntity, error) {
	if err := syncUser(db, user, true); err != nil {
		return nil, err
	}
	userNaming := naming.NewUserNaming(user.Name, user.ScreenName)
	expectedTitle := userNaming.SanitizedTitle()

	ent, err := entity.NewUserEntity(db, user.Id, dir)
	if err != nil {
		return nil, err
	}
	if err = entity.Sync(ent, expectedTitle); err != nil {
		return nil, err
	}
	return ent, nil
}

func shouldIgnoreUser(user *twitter.User) bool {
	if user == nil {
		return true
	}
	return user.Blocking || user.Muting
}
