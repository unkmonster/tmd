package downloading

import (
	"context"
	"runtime"
	"sync"

	"github.com/go-resty/resty/v2"
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
	path, err := pt.Entity.Path()
	if err != nil {
		return ""
	}
	return path
}

type userInListEntity struct {
	user *twitter.User
	leid *int
}

var MaxDownloadRoutine int

var syncedUsers sync.Map

var syncedListUsers sync.Map

func init() {
	// 降低默认并发数，减少内存占用
	// 流式下载后，内存不再是瓶颈，但过多并发会增加网络和磁盘压力
	MaxDownloadRoutine = min(10, runtime.GOMAXPROCS(0)*2)
}

type workerConfig struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	cancel         context.CancelCauseFunc
	skipLoongTweet bool
	downloader     downloader.Downloader
	fileWriter     downloader.FileWriter
	client         *resty.Client
}

const userTweetRateLimit = 1500
const userTweetMaxConcurrent = 35
