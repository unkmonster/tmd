package downloading

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd2/database"
	"github.com/unkmonster/tmd2/internal/utils"
	"github.com/unkmonster/tmd2/twitter"
)

type TweetInEntity struct {
	Tweet  *twitter.Tweet
	Entity *UserEntity
}

func (pt TweetInEntity) GetTweet() *twitter.Tweet {
	return pt.Tweet
}

func (pt TweetInEntity) GetPath() string {
	path, err := pt.Entity.Path()
	if err != nil {
		return ""
	}
	return path
}

func batchUserDownload(client *resty.Client, db *sqlx.DB, users []*twitter.User, dir string, listEntityId int) []*TweetInEntity {
	getterCount := min(len(users), 25)
	downloaderCount := 60

	uidToUser := make(map[uint64]*twitter.User)
	userChan := make(chan *twitter.User, len(users))
	for _, u := range users {
		userChan <- u
		uidToUser[u.Id] = u
	}
	close(userChan)

	entityChan := make(chan *UserEntity, len(users))
	tweetChan := make(chan *TweetInEntity, 2*getterCount)
	failureTw := make(chan *TweetInEntity)
	abortChan := make(chan struct{})
	syncWg := sync.WaitGroup{}
	getterWg := sync.WaitGroup{}
	downloaderWg := sync.WaitGroup{}

	abort := sync.OnceFunc(func() {
		close(abortChan)
	})

	cancelled := func() bool {
		select {
		case <-abortChan:
			return true
		default:
			return false
		}
	}

	panicHandler := func() {
		if p := recover(); p != nil {
			fmt.Println(p)
			abort()
		}
	}

	userUpdater := func() {
		defer syncWg.Done()
		defer panicHandler()
		for u := range userChan {
			if cancelled() {
				break
			}
			pathEntity, err := syncUserAndEntityInDir(db, u, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[sync worker] %s: %v\n", u.Title(), err)
				continue
			}
			entityChan <- pathEntity

			// link
			linkEntity := NewUserEntityByParentLstPathId(db, u.Id, listEntityId)
			userLink := NewUserLink(linkEntity, pathEntity)
			err = syncPath(userLink, pathEntity.Name()+".lnk")
			if err != nil {
				fmt.Printf("failed to sync link: %v\n", err)
			}
		}
		fmt.Println("[updating worker]: bye")
	}

	tweetGetter := func() {
		defer getterWg.Done()
		defer panicHandler()
		for e := range entityChan {
			if cancelled() {
				break
			}
			tweets, err := getTweetAndUpdateLatestReleaseTime(client, uidToUser[e.Uid()], e)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[getting worker] %s: %v\n", e.Name(), err)
				continue
			}
			for _, tw := range tweets {
				pt := TweetInEntity{Tweet: tw, Entity: e}
				tweetChan <- &pt
			}
		}
	}

	tweetDownloader := func() {
		defer downloaderWg.Done()
		var pt *TweetInEntity
		defer func() {
			if p := recover(); p != nil {
				failureTw <- pt
				fmt.Println("[downloading worker]:", p)
			}
		}()
		for pt = range tweetChan {
			if cancelled() {
				failureTw <- pt
				continue
			}

			path, _ := pt.Entity.Path()
			// TODO: 判断可恢复及不可恢复
			err := downloadTweetMedia(client, path, pt.Tweet)
			if err != nil {
				failureTw <- pt
			}
		}
		fmt.Println("[downloading worker]: bye")
	}

	for i := 0; i < getterCount; i++ {
		syncWg.Add(1)
		go userUpdater()
		getterWg.Add(1)
		go tweetGetter()
	}

	for i := 0; i < downloaderCount; i++ {
		downloaderWg.Add(1)
		go tweetDownloader()
	}

	//closer
	go func() {
		syncWg.Wait()
		close(entityChan)
		log.Printf("entity chan has closed\n")

		getterWg.Wait()
		close(tweetChan)
		log.Printf("tweet chan has closed\n")

		downloaderWg.Wait()
		close(failureTw)
		log.Printf("failureTw chan has closed\n")
	}()

	failures := []*TweetInEntity{}
	for pt := range failureTw {
		failures = append(failures, pt)
	}
	return failures
}

func downloadList(client *resty.Client, db *sqlx.DB, list twitter.ListBase, dir string, realDir string) ([]*TweetInEntity, error) {
	expectedTitle := string(utils.WinFileName([]byte(list.Title())))
	entity := NewListEntity(db, list.GetId(), dir)
	if err := syncPath(entity, expectedTitle); err != nil {
		return nil, err
	}

	members, err := list.GetMembers(client)
	if err != nil || len(members) == 0 {
		return nil, err
	}

	return batchUserDownload(client, db, members, realDir, entity.Id()), nil
}

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

func DownloadList(client *resty.Client, db *sqlx.DB, list *twitter.List, dir string, realDir string) ([]*TweetInEntity, error) {
	if err := syncList(db, list); err != nil {
		return nil, err
	}
	return downloadList(client, db, list, dir, realDir)
}

func DownloadFollowing(client *resty.Client, db *sqlx.DB, list twitter.UserFollowing, dir string, realDir string) ([]*TweetInEntity, error) {
	return downloadList(client, db, list, dir, realDir)
}
