package downloading

import (
	"fmt"
	"os"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
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
	userChan := make(chan *twitter.User, getterCount)
	for _, u := range users {
		userChan <- u
		uidToUser[u.Id] = u
	}
	close(userChan)

	entityChan := make(chan *UserEntity, getterCount)
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
		}()
	}

	wg := sync.WaitGroup{}
	for i := 0; i < downloaderCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pt := range tweetChan {
				path, _ := pt.Entity.Path()
				err := downloadTweetMedia(client, path, pt.Tweet)
				if err != nil {
					failureTw <- pt
				}
			}
		}()
	}

	//closer
	go func() {
		syncWg.Wait()
		close(entityChan)

		getterWg.Wait()
		close(tweetChan)

		wg.Wait()
		close(failureTw)
	}()

	failures := []TweetInEntity{}
	for pt := range failureTw {
		failures = append(failures, pt)
	}
	return failures
}
