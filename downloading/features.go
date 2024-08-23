package downloading

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd2/database"
	"github.com/unkmonster/tmd2/internal/utils"
	"github.com/unkmonster/tmd2/twitter"
)

type PackgedTweet interface {
	GetTweet() *twitter.Tweet
	GetPath() string
}

type TweetInDir struct {
	tweet *twitter.Tweet
	path  string
}

func (pt TweetInDir) GetTweet() *twitter.Tweet {
	return pt.tweet
}

func (pt TweetInDir) GetPath() string {
	return pt.path
}

var mutex sync.Mutex

// 任何一个 url 下载失败直接返回
// TODO: 要么全做，要么不做
func downloadTweetMedia(ctx context.Context, client *resty.Client, dir string, tweet *twitter.Tweet) error {
	text := utils.WinFileName(tweet.Text)

	for _, u := range tweet.Urls {
		ext, err := utils.GetExtFromUrl(u)
		if err != nil {
			return err
		}

		// 请求
		resp, err := client.R().SetContext(ctx).Get(u)
		if err != nil {
			return err
		}
		if err := utils.CheckRespStatus(resp); err != nil {
			return err
		}

		mutex.Lock()
		path, err := utils.UniquePath(filepath.Join(dir, text+ext))
		if err != nil {
			mutex.Unlock()
			return err
		}
		file, err := os.Create(path)
		mutex.Unlock()
		if err != nil {
			return err
		}

		defer os.Chtimes(path, time.Time{}, tweet.CreatedAt)
		defer file.Close()

		_, err = file.Write(resp.Body())
		if err != nil {
			return err
		}
	}

	fmt.Printf("%s %s\n", color.FgLightMagenta.Render("["+tweet.Creator.Title()+"]"), text)
	return nil
}

var MaxDownloadRoutine int

// TODO 多列表同时下载

// map[user_id]*UserEntity 记录本次程序运行已同步过的用户
var syncedUsers sync.Map

func init() {
	MaxDownloadRoutine = runtime.GOMAXPROCS(0) * 8
}

type balanceLoader struct {
	activeRoutines int
	maxRoutines    int
	routine        func()
	shouldNotify   func() bool
	requests       chan struct{}
}

func newBalanceLoader(activeRoutines int, maxRoutines int, routine func(), shouldNotify func() bool) *balanceLoader {
	bl := balanceLoader{}
	bl.activeRoutines = activeRoutines
	bl.maxRoutines = maxRoutines
	bl.routine = routine
	bl.shouldNotify = shouldNotify
	bl.requests = make(chan struct{})
	return &bl
}

func (bl *balanceLoader) do() {
	if bl.activeRoutines < bl.maxRoutines {
		//color.Debug.Tips("[Balance Loader] launched a new goroutine %d", bl.activeRoutines+1)
		go bl.routine()
		bl.activeRoutines++
	}
}

func (bl *balanceLoader) notify() {
	select {
	case bl.requests <- struct{}{}:
	default:
	}
}

type workerConfig struct {
	ctx    context.Context
	wg     *sync.WaitGroup
	bl     *balanceLoader
	cancel context.CancelCauseFunc
}

// 负责下载推文，保证 tweet chan 内的推文要么下载成功，要么推送至 error chan
func tweetDownloader(client *resty.Client, config *workerConfig, errch chan<- PackgedTweet, twech <-chan PackgedTweet) {
	var pt PackgedTweet
	var ok bool

	defer config.wg.Done()
	defer func() {
		if p := recover(); p != nil {
			config.cancel(fmt.Errorf("%v", p)) // panic 取消上下文，防止生产者死锁
			log.WithField("worker", "downloading").Errorln("panic:", p)

			if pt != nil {
				errch <- pt // push 正下载的推文
			}
			// 确保只有1个协程的情况下，未能下载完毕的推文仍然会全部推送到 errch
			for pt := range twech {
				errch <- pt
			}
		}
	}()

	for {
		select {
		case pt, ok = <-twech:
			if !ok {
				return
			}
		case <-time.After(150 * time.Millisecond):
			if config.bl != nil && config.bl.shouldNotify() {
				config.bl.notify()
			}
			continue
		case <-config.ctx.Done():
			for pt := range twech {
				errch <- pt
			}
			return
		}

		path := pt.GetPath()
		if path == "" {
			errch <- pt
			continue
		}
		err := downloadTweetMedia(config.ctx, client, path, pt.GetTweet())
		if err != nil && !utils.IsStatusCode(err, 404) {
			errch <- pt
		}
	}
}

// 批量下载推文并返回下载失败的推文，可以保证推文被成功下载或被返回
func BatchDownloadTweet(ctx context.Context, client *resty.Client, pts ...PackgedTweet) []PackgedTweet {
	if len(pts) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancelCause(ctx)

	var errChan = make(chan PackgedTweet)
	var tweetChan = make(chan PackgedTweet, len(pts))
	var wg sync.WaitGroup // number of working goroutines
	var numRoutine = min(len(pts), MaxDownloadRoutine)

	for _, pt := range pts {
		tweetChan <- pt
	}
	close(tweetChan)

	config := workerConfig{
		ctx:    ctx,
		bl:     nil,
		cancel: cancel,
		wg:     &wg,
	}
	for i := 0; i < numRoutine; i++ {
		wg.Add(1)
		go tweetDownloader(client, &config, errChan, tweetChan)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	errors := []PackgedTweet{}
	for pt := range errChan {
		errors = append(errors, pt)
	}
	return errors
}

// 更新数据库中对用户的记录
func syncUser(db *sqlx.DB, user *twitter.User) error {
	renamed := false
	isNew := false
	usrdb, err := database.GetUserById(db, user.Id)
	if err != nil {
		return err
	}

	if usrdb == nil {
		isNew = true
		usrdb = &database.User{}
		usrdb.Id = user.Id
	} else {
		renamed = usrdb.Name != user.Name || usrdb.ScreenName != user.ScreenName
	}

	usrdb.FriendsCount = user.FriendsCount
	usrdb.IsProtected = user.IsProtected
	usrdb.Name = user.Name
	usrdb.ScreenName = user.ScreenName

	if isNew {
		err = database.CreateUser(db, usrdb)
	} else {
		err = database.UpdateUser(db, usrdb)
	}
	if err != nil {
		return err
	}
	if renamed || isNew {
		err = database.RecordUserPreviousName(db, user.Id, user.Name, user.ScreenName)
	}
	return err
}

func getTweetAndUpdateLatestReleaseTime(ctx context.Context, client *resty.Client, user *twitter.User, entity *UserEntity) ([]*twitter.Tweet, error) {
	tweets, err := user.GetMeidas(ctx, client, &utils.TimeRange{Min: entity.LatestReleaseTime()})
	if err != nil || len(tweets) == 0 {
		return nil, err
	}
	if err := entity.SetLatestReleaseTime(tweets[0].CreatedAt); err != nil {
		return nil, err
	}
	return tweets, nil
}

func DownloadUser(ctx context.Context, db *sqlx.DB, client *resty.Client, user *twitter.User, dir string) ([]PackgedTweet, error) {
	if user.Blocking || user.Muting {
		return nil, nil
	}

	_, loaded := syncedUsers.Load(user.Id)
	if loaded {
		log.WithField("user", user.Title()).Debugln("skiped downloaded user")
		return nil, nil
	}
	entity, err := syncUserAndEntity(db, user, dir)
	if err != nil {
		return nil, err
	}

	syncedUsers.Store(user.Id, entity)
	tweets, err := getTweetAndUpdateLatestReleaseTime(ctx, client, user, entity)
	if err != nil || len(tweets) == 0 {
		return nil, err
	}

	// 打包推文
	pts := make([]PackgedTweet, 0, len(tweets))
	for _, tw := range tweets {
		pts = append(pts, TweetInEntity{Tweet: tw, Entity: entity})
	}

	return BatchDownloadTweet(ctx, client, pts...), nil
}

func syncUserAndEntity(db *sqlx.DB, user *twitter.User, dir string) (*UserEntity, error) {
	if err := syncUser(db, user); err != nil {
		return nil, err
	}
	expectedTitle := utils.WinFileName(user.Title())

	entity, err := NewUserEntity(db, user.Id, dir)
	if err != nil {
		return nil, err
	}
	if err = syncPath(entity, expectedTitle); err != nil {
		return nil, err
	}
	return entity, nil
}

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

const userTweetApiLimit = 500

// var syncedListUsers = make(map[uint64]map[int64]struct{})
var syncedListUsers sync.Map //leid -> uid -> struct{}

func BatchUserDownload(ctx context.Context, client *resty.Client, db *sqlx.DB, users []*twitter.User, dir string, listEntityId *int) ([]*TweetInEntity, error) {
	if len(users) == 0 {
		return nil, nil
	}

	utils.Shuffle(users)
	uidToUser := make(map[uint64]*twitter.User)
	userChan := make(chan *twitter.User, len(users))
	for _, u := range users {
		if u.Blocking || u.Muting {
			continue
		}

		userChan <- u
		uidToUser[u.Id] = u
	}
	close(userChan)

	numUsers := len(userChan)

	// num of worker
	getterCount := min(numUsers, MaxDownloadRoutine/3)
	getterCount = min(userTweetApiLimit, getterCount)
	getterCount = max(1, getterCount) // 确保至少有一个 getting worker
	// channels
	entityChan := make(chan *UserEntity, numUsers)
	tweetChan := make(chan PackgedTweet, MaxDownloadRoutine)
	errChan := make(chan PackgedTweet)
	// WG
	updaterWg := sync.WaitGroup{}
	getterWg := sync.WaitGroup{}
	downloaderWg := sync.WaitGroup{}
	// ctx
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	// logger
	updaterLogger := log.WithField("worker", "sync")
	getterLogger := log.WithField("worker", "getting")

	panicHandler := func() {
		if p := recover(); p != nil {
			cancel(fmt.Errorf("%v", p))
		}
	}

	userUpdater := func() {
		defer updaterWg.Done()
		defer panicHandler()

		var user *twitter.User
		var ok bool

		for {
			select {
			case user, ok = <-userChan:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			var pathEntity *UserEntity
			var err error

			pe, loaded := syncedUsers.Load(user.Id)
			if !loaded {
				pathEntity, err = syncUserAndEntity(db, user, dir)
				if err != nil {
					log.WithField("worker", "sync").Warnln("failed to sync user or entity", err)
					continue
				}
				syncedUsers.Store(user.Id, pathEntity)

				select {
				case entityChan <- pathEntity:
				case <-ctx.Done():
					return
				}

				// 同步所有现存的指向此用户的符号链接
				upath, _ := pathEntity.Path()
				linkds, err := database.GetUserLinks(db, user.Id)
				if err != nil {
					updaterLogger.WithField("user", user.Title()).Warnln("failed to get links to user:", err)
					continue
				}

				for _, linkd := range linkds {
					if err = updateUserLink(linkd, db, upath); err != nil {
						updaterLogger.WithField("user", user.Title()).Warnln("failed to sync link:", err)
					}
					sl, _ := syncedListUsers.LoadOrStore(int(linkd.ParentLstEntityId), &sync.Map{})
					syncedList := sl.(*sync.Map)
					syncedList.Store(user.Id, struct{}{})
				}

			} else {
				pathEntity = pe.(*UserEntity)
				log.WithField("user", user.Title()).Debugln("skiped synced user")
			}

			// 即便同步一个用户时也同步了所有指向此用户的链接，
			// 但此用户仍可能会是一个新的 “列表-用户”，所以判断此用户链接是否同步过，
			// 如果否，那么创建一个属于此列表的用户链接
			if listEntityId == nil {
				continue
			}
			sl, _ := syncedListUsers.LoadOrStore(*listEntityId, &sync.Map{})
			syncedList := sl.(*sync.Map)
			_, loaded = syncedList.LoadOrStore(user.Id, struct{}{})
			if loaded {
				continue
			}

			// 为当前列表的新用户创建符号链接
			upath, _ := pathEntity.Path()
			var linkname = pathEntity.Name()

			curlink := &database.UserLink{}
			curlink.Name = linkname
			curlink.ParentLstEntityId = int32(*listEntityId)
			curlink.Uid = user.Id

			linkpath, err := curlink.Path(db)
			if err == nil {
				if err = os.Symlink(upath, linkpath); err == nil || os.IsExist(err) {
					err = database.CreateUserLink(db, curlink)
				}
			}
			if err != nil {
				updaterLogger.WithField("user", user.Title()).Warnln("failed to create link for user:", err)
			}
		}
	}

	// 首批推文推送至 tweet chan 时调用，意味流水线已启动
	startedDownload := &atomic.Bool{}
	triggerStart := sync.OnceFunc(func() { startedDownload.Store(true) })

	tweetGetter := func() {
		defer getterWg.Done()
		defer panicHandler()

		var entity *UserEntity
		var ok bool

		for {
			select {
			case <-ctx.Done():
				return
			case entity, ok = <-entityChan:
				if !ok {
					return
				}
			}

			user := uidToUser[entity.Uid()]
			tweets, err := user.GetMeidas(ctx, client, &utils.TimeRange{Min: entity.LatestReleaseTime()})
			if utils.IsStatusCode(err, 429) {
				// json 版本的响应 {"errors":[{"code":88,"message":"Rate limit exceeded."}]} 代表达到看帖上限
				// text 版本的响应 Rate limit exceeded. 代表暂时达到速率限制
				v := err.(*utils.HttpStatusError)
				if v.Msg[0] == '{' && v.Msg[len(v.Msg)-1] == '}' {
					cancel(fmt.Errorf("reached the limit for seeing posts today"))
					continue
				}
			}
			if ctx.Err() != nil {
				continue
			}
			if err != nil {
				getterLogger.WithField("user", entity.Name()).Warnln("failed to get user medias:", err)
				continue
			}
			if len(tweets) == 0 {
				continue
			}

			// 确保该用户所有推文已推送并更新用户的最新发布时间
			for _, tw := range tweets {
				pt := TweetInEntity{Tweet: tw, Entity: entity}
				select {
				case tweetChan <- &pt:
				case <-ctx.Done():
					return // 防止无消费者导致死锁
				}
			}

			if err := entity.SetLatestReleaseTime(tweets[0].CreatedAt); err != nil {
				// 影响程序的正确性，必须 Panic
				getterLogger.WithField("user", entity.Name()).Panicln("failed to set latest release time for user:", err)
			}

			triggerStart()
		}
	}

	// launch all worker
	start := time.Now()

	updaterWg.Add(1)
	go userUpdater() // only 1 updating worker required

	for i := 0; i < getterCount; i++ {
		getterWg.Add(1)
		go tweetGetter()
	}

	// downloader config
	newUpstream := func() {
		getterWg.Add(1)
		tweetGetter()
	}
	bl := newBalanceLoader(getterCount, min(userTweetApiLimit, numUsers), newUpstream, func() bool {
		return !twitter.GetClientBlockState(client) && startedDownload.Load()
	})
	config := workerConfig{
		ctx:    ctx,
		bl:     bl,
		wg:     &downloaderWg,
		cancel: cancel,
	}
	for i := 0; i < MaxDownloadRoutine; i++ {
		downloaderWg.Add(1)
		go tweetDownloader(client, &config, errChan, tweetChan)
	}

	//closer
	go func() {
		updaterWg.Wait()
		close(entityChan)
		updaterLogger.WithField("elapsed", time.Since(start)).Debugln("shutdown")

		getterWg.Wait()
		close(tweetChan)
		getterLogger.WithField("elapsed", time.Since(start)).Debugln("shutdown")

		downloaderWg.Wait()
		close(errChan)
	}()

	fails := []*TweetInEntity{}

	for {
		select {
		case pt, ok := <-errChan:
			if !ok {
				// errch 已关闭，退出循环
				return fails, context.Cause(ctx)
			}
			fails = append(fails, pt.(*TweetInEntity))
		case <-bl.requests:
			bl.do()
		}
	}
}

func downloadList(ctx context.Context, client *resty.Client, db *sqlx.DB, list twitter.ListBase, dir string, realDir string) ([]*TweetInEntity, error) {
	expectedTitle := utils.WinFileName(list.Title())
	entity, err := NewListEntity(db, list.GetId(), dir)
	if err != nil {
		return nil, err
	}
	if err := syncPath(entity, expectedTitle); err != nil {
		return nil, err
	}

	members, err := list.GetMembers(ctx, client)
	if err != nil || len(members) == 0 {
		return nil, err
	}

	eid := entity.Id()
	log.Debugln("members:", len(members))
	return BatchUserDownload(ctx, client, db, members, realDir, &eid)
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

func DownloadList(ctx context.Context, client *resty.Client, db *sqlx.DB, list twitter.ListBase, dir string, realDir string) ([]*TweetInEntity, error) {
	tlist, ok := list.(*twitter.List)
	if ok {
		if err := syncList(db, tlist); err != nil {
			return nil, err
		}
	}
	return downloadList(ctx, client, db, list, dir, realDir)
}
