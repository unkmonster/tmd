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

func init() {
	// Keep the runtime default in one place so config prompts and execution agree.
	MaxDownloadRoutine = config.DefaultMaxDownloadRoutine()
}

type batchSyncState struct {
	users     map[uint64]*entity.UserEntity
	listUsers map[int]map[uint64]struct{}
}

func newBatchSyncState() *batchSyncState {
	return &batchSyncState{
		users:     make(map[uint64]*entity.UserEntity),
		listUsers: make(map[int]map[uint64]struct{}),
	}
}

func (s *batchSyncState) loadUser(userID uint64) (*entity.UserEntity, bool) {
	ent, ok := s.users[userID]
	return ent, ok
}

func (s *batchSyncState) storeUser(userID uint64, ent *entity.UserEntity) {
	s.users[userID] = ent
}

func (s *batchSyncState) markListUser(listEntityID int, userID uint64) bool {
	users, ok := s.listUsers[listEntityID]
	if !ok {
		users = make(map[uint64]struct{})
		s.listUsers[listEntityID] = users
	}

	if _, exists := users[userID]; exists {
		return false
	}
	users[userID] = struct{}{}
	return true
}

type workerConfig struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	cancel         context.CancelCauseFunc
	skipLoongTweet bool
	downloader     downloader.Downloader
	fileWriter     downloader.FileWriter
	client         *resty.Client
	onTweetDone    func(pt PackagedTweet, failed bool)
}

const userTweetRateLimit = 1500
const userTweetMaxConcurrent = 35
