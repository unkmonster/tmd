package downloading

import (
	"context"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/entity"
	"github.com/unkmonster/tmd/internal/twitter"
)

type PackagedTweet interface {
	GetTweet() *twitter.Tweet
	GetPath() string
}

type TweetInEntity struct {
	Tweet  *twitter.Tweet
	Entity *entity.UserEntity
}

func (pt TweetInEntity) GetTweet() *twitter.Tweet {
	return pt.Tweet
}

func (pt TweetInEntity) GetPath() string {
	if pt.Entity == nil {
		return ""
	}
	path, err := pt.Entity.Path()
	if err != nil {
		return ""
	}
	return path
}

type userInListEntity struct {
	user *twitter.User
	leid int
}

var MaxDownloadRoutine int

var syncedUsers sync.Map

var syncedListUsers sync.Map

func init() {
	// Keep the runtime default in one place so config prompts and execution agree.
	MaxDownloadRoutine = config.DefaultMaxDownloadRoutine()
}

type workerConfig struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	cancel         context.CancelCauseFunc
	skipLoongTweet bool
	downloader     downloader.Downloader
	fileWriter     downloader.FileWriter
	client         *resty.Client
	onTweetDone    func(tweet *twitter.Tweet, failed bool)
}

const userTweetRateLimit = 1500
const userTweetMaxConcurrent = 35
