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

func batchUserDownload(client *resty.Client, db *sqlx.DB, users []*twitter.User, dir string) []TweetInEntity {
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
	tweetChan := make(chan TweetInEntity, 2*getterCount)
	failureTw := make(chan TweetInEntity)

	syncWg := sync.WaitGroup{}
	getterWg := sync.WaitGroup{}

	for i := 0; i < getterCount; i++ {
		syncWg.Add(1)
		go func() {
			defer syncWg.Done()
			for u := range userChan {
				entity, err := syncUserAndEntityInDir(db, u, dir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[sync worker] %s: %v\n", u.Title(), err)
					continue
				}
				entityChan <- entity
			}
			fmt.Println("[sync worker]: bye")
		}()

		getterWg.Add(1)
		go func() {
			defer getterWg.Done()
			for e := range entityChan {
				tweets, err := getTweetAndUpdateLatestReleaseTime(client, uidToUser[e.Uid()], e)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[getting worker] %s: %v\n", e.Title(), err)
					//log.Printf("[getting worker] %s: %v", e.Title(), err)
					continue
				}
				for _, tw := range tweets {
					pt := TweetInEntity{Tweet: tw, Entity: e}
					tweetChan <- pt
				}
			}
			fmt.Println("[getting worker]: bye")
		}()
	}

	wg := sync.WaitGroup{}
	for i := 0; i < downloaderCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pt := range tweetChan {
				//fmt.Println(pt.Tweet.Text)
				path, _ := pt.Entity.Path()
				err := downloadTweetMedia(client, path, pt.Tweet)
				if err != nil {
					failureTw <- pt
				}
			}
			fmt.Println("[downloading worker]: bye")
		}()
	}

	//closer
	go func() {
		syncWg.Wait()
		close(entityChan)
		log.Printf("entity chan has closed\n")

		getterWg.Wait()
		close(tweetChan)
		log.Printf("tweet chan has closed\n")

		wg.Wait()
		close(failureTw)
		log.Printf("failureTw chan has closed\n")
	}()

	failures := []TweetInEntity{}
	for pt := range failureTw {
		failures = append(failures, pt)
	}
	return failures
}

func downloadList(client *resty.Client, db *sqlx.DB, list twitter.ListBase, dir string) ([]TweetInEntity, error) {
	expectedTitle := string(utils.WinFileName([]byte(list.Title())))
	entityDb, err := database.LocateLstEntity(db, list.GetId(), dir)
	if err != nil {
		return nil, err
	}
	if entityDb == nil {
		entityDb = &database.LstEntity{}
		entityDb.LstId = list.GetId()
		entityDb.ParentDir = dir
		entityDb.Title = string(expectedTitle)
	}

	var path string
	entity := ListEntity{entityDb, db}
	if !entity.dbentity.Id.Valid {
		if err := entity.Create(); err != nil {
			return nil, err
		}
	} else if entity.Title() != expectedTitle {
		if err := entity.Rename(expectedTitle); err != nil {
			return nil, err
		}
	} else {
		path, _ = entity.Path()
		os.Mkdir(path, 0755)
	}

	members, err := list.GetMembers(client)
	if err != nil || len(members) == 0 {
		return nil, err
	}

	return batchUserDownload(client, db, members, path), nil
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

func DownloadList(client *resty.Client, db *sqlx.DB, list *twitter.List, dir string) ([]TweetInEntity, error) {
	if err := syncList(db, list); err != nil {
		return nil, err
	}
	return downloadList(client, db, list, dir)
}

func DownloadFollowing(client *resty.Client, db *sqlx.DB, list twitter.UserFollowing, dir string) ([]TweetInEntity, error) {
	return downloadList(client, db, list, dir)
}
