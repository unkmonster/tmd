package cli

import (
	"context"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/twitter"
)

// ResolvedEntities 解析后的用户和列表实体
type ResolvedEntities struct {
	Users []*twitter.User
	Lists []twitter.ListBase
}

// ResolveUsersAndLists 统一解析用户和列表参数（避免 DRY 违反）
func ResolveUsersAndLists(ctx context.Context, client *resty.Client, db *sqlx.DB, usrArgs UserArgs, listArgs ListArgs, follArgs UserArgs) ResolvedEntities {
	var entities ResolvedEntities

	for _, screenName := range usrArgs.ScreenName {
		user, uid, err := twitter.GetUserByScreenName(ctx, client, screenName)
		if err != nil {
			if db != nil {
				database.MarkUserInaccessible(db, uid, screenName)
			}
			log.Warnf("Failed to get user %s: %v", screenName, err)
			continue
		}
		entities.Users = append(entities.Users, user)
	}

	for _, listID := range listArgs.ID {
		list, err := twitter.GetLst(ctx, client, listID)
		if err != nil {
			log.Warnf("Failed to get list %d: %v", listID, err)
			continue
		}
		entities.Lists = append(entities.Lists, list)
	}

	for _, screenName := range follArgs.ScreenName {
		user, uid, err := twitter.GetUserByScreenName(ctx, client, screenName)
		if err != nil {
			if db != nil {
				database.MarkUserInaccessible(db, uid, screenName)
			}
			log.Warnf("Failed to get user %s for following list: %v", screenName, err)
			continue
		}
		entities.Lists = append(entities.Lists, user.Following())
	}

	return entities
}
