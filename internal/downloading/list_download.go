package downloading

import (
	"context"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/entity"
	"github.com/unkmonster/tmd/internal/naming"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

func syncList(db *sqlx.DB, list *twitter.List) error {
	listdb, err := database.GetLst(db, list.Id)
	if err != nil {
		return err
	}
	if listdb == nil {
		return database.CreateLst(db, &database.Lst{Id: list.Id, Name: list.Name, OwnerId: list.Creator.Id})
	}
	return database.UpdateLst(db, &database.Lst{Id: list.Id, Name: list.Name, OwnerId: list.Creator.Id})
}

func syncListAndGetMembers(ctx context.Context, client *resty.Client, db *sqlx.DB, lst twitter.ListBase, dir string) (entities []userInListEntity, members []*twitter.User, err error) {
	if v, ok := lst.(*twitter.List); ok {
		if err = syncList(db, v); err != nil {
			return nil, nil, err
		}
	}

	expectedTitle := naming.NewListNamingFromBase(lst).SanitizedTitle()
	ent, err := entity.NewListEntity(db, lst.GetId(), dir)
	if err != nil {
		return nil, nil, err
	}
	if err = entity.Sync(ent, expectedTitle); err != nil {
		return nil, nil, err
	}

	membersResult, err := lst.GetMembers(ctx, client)
	if err != nil {
		return nil, nil, err
	}

	eid, err := ent.Id()
	if err != nil {
		return nil, nil, err
	}

	members = membersResult.Users
	if len(members) == 0 {
		return nil, nil, nil
	}

	memberIDs := utils.ExtractIDs(members, func(u *twitter.User) uint64 { return u.Id })
	database.MarkListMembersAccessibleByIDs(db, memberIDs)
	syncManager := NewListSyncManager(db)
	if err = syncManager.SyncListMembers(ctx, eid, lst.Title(), memberIDs); err != nil {
		log.Warnln("failed to sync list members for", lst.Title(), ":", err)
	}

	entities = make([]userInListEntity, 0, len(members))
	for _, user := range members {
		// 为每个用户创建独立的 eid 副本，避免共享局部变量地址
		leid := eid
		entities = append(entities, userInListEntity{user: user, leid: &leid})
	}
	return entities, members, nil
}
