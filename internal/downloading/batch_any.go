package downloading

import (
	"context"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/twitter"
)

func BatchDownloadAny(ctx context.Context, client *resty.Client, db *sqlx.DB, lists []twitter.ListBase, users []*twitter.User, dir string, realDir string, autoFollow bool, additional []*resty.Client, dwn downloader.Downloader, fileWriter downloader.FileWriter) (failedTweets []*TweetInEntity, listMembers []*twitter.User, err error) {
	log.Debugln("start collecting users")
	packgedUsers := make([]userInListEntity, 0)
	listMembers = make([]*twitter.User, 0)
	wg := sync.WaitGroup{}
	mtx := sync.Mutex{}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	for _, lst := range lists {
		wg.Add(1)
		go func(lst twitter.ListBase) {
			defer wg.Done()
			res, members, e := syncListAndGetMembers(ctx, client, db, lst, dir)
			if e != nil {
				cancel(e)
				return
			}
			log.Debugf("members of %s: %d", lst.Title(), len(res))
			mtx.Lock()
			defer mtx.Unlock()
			packgedUsers = append(packgedUsers, res...)
			listMembers = append(listMembers, members...)
		}(lst)
	}
	wg.Wait()
	if err = context.Cause(ctx); err != nil {
		return nil, nil, err
	}

	for _, usr := range users {
		packgedUsers = append(packgedUsers, userInListEntity{user: usr, leid: 0})
	}

	log.Debugln("collected users:", len(packgedUsers))
	failedTweets, err = BatchUserDownload(ctx, client, db, packgedUsers, realDir, autoFollow, additional, dwn, fileWriter)
	return failedTweets, listMembers, err
}
