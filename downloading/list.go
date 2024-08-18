package downloading

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
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

const userTweetApiLimit = 500

// var syncedListUsers = make(map[uint64]map[int64]struct{})
var syncedListUsers sync.Map //leid -> uid -> struct{}

func BatchUserDownload(client *resty.Client, db *sqlx.DB, users []*twitter.User, dir string, listEntityId *int) []*TweetInEntity {
	if len(users) == 0 {
		return nil
	}

	uidToUser := make(map[uint64]*twitter.User)
	userChan := make(chan *twitter.User, len(users))
	for _, u := range users {
		userChan <- u
		uidToUser[u.Id] = u
	}
	close(userChan)

	// num of worker
	getterCount := min(len(users), MaxDownloadRoutine/3)
	getterCount = min(userTweetApiLimit, getterCount)
	// channels
	entityChan := make(chan *UserEntity, len(users))
	tweetChan := make(chan PackgedTweet, MaxDownloadRoutine)
	errch := make(chan PackgedTweet)
	abortChan := make(chan struct{})
	// WG
	updaterWg := sync.WaitGroup{}
	getterWg := sync.WaitGroup{}
	downloaderWg := sync.WaitGroup{}

	// 导致 cancelled() 返回真，后续调用无效
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

	panicHandler := func(name string) {
		if p := recover(); p != nil {
			color.Danger.Printf("[%s] panic: %v\n", name, p)
			abort()
		}
	}

	// 信号相关
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigChan)

	// cpu 密集型任务
	userUpdater := func() {
		defer updaterWg.Done()
		defer panicHandler("sync worker")
		for u := range userChan {
			if cancelled() {
				break
			}

			var pathEntity *UserEntity
			var err error

			pe, loaded := syncedUsers.Load(u.Id)
			if !loaded {
				pathEntity, err = syncUserAndEntity(db, u, dir)
				if err != nil {
					color.Error.Tips("[sync worker] failed to sync user or entity: %v", err)
					continue
				}
				syncedUsers.Store(u.Id, pathEntity)
				entityChan <- pathEntity

				// 同步所有现存的指向此用户的符号链接
				upath, _ := pathEntity.Path()
				linkds, err := database.GetUserLinks(db, u.Id)
				if err != nil {
					color.Error.Tips("[sync worker] failed to get links to %s: %v", u.Title(), err)
					continue
				}

				for _, linkd := range linkds {
					if err = updateUserLink(linkd, db, upath); err != nil {
						color.Error.Tips("[sync worker] failed to sync link: %v", err)
					}
					sl, _ := syncedListUsers.LoadOrStore(int(linkd.ParentLstEntityId), &sync.Map{})
					syncedList := sl.(*sync.Map)
					syncedList.Store(u.Id, struct{}{})
				}

			} else {
				pathEntity = pe.(*UserEntity)
				color.Note.Printf("[sync worker] skiped synced user '%s'\n", u.Title())
			}

			// 即便同步一个用户时也同步了所有指向此用户的链接，
			// 但此用户仍可能会是一个新的 “列表-用户”，所以判断此用户链接是否同步过，
			// 如果否，那么创建一个属于此列表的用户链接
			if listEntityId == nil {
				continue
			}
			sl, _ := syncedListUsers.LoadOrStore(*listEntityId, &sync.Map{})
			syncedList := sl.(*sync.Map)
			_, loaded = syncedList.LoadOrStore(u.Id, struct{}{})
			if loaded {
				//color.Note.Printf("[sync worker] skiped synced list user %d/%s\n", *listEntityId, u.Title())
				continue
			}

			// 为当前列表的新用户创建符号链接
			upath, _ := pathEntity.Path()
			var linkname = pathEntity.Name()

			curlink := &database.UserLink{}
			curlink.Name = linkname
			curlink.ParentLstEntityId = int32(*listEntityId)
			curlink.Uid = u.Id

			linkpath, err := curlink.Path(db)
			if err == nil {
				if err = os.Symlink(upath, linkpath); err == nil || os.IsExist(err) {
					err = database.CreateUserLink(db, curlink)
				}
			}
			if err != nil {
				color.Error.Tips("[sync worker] failed to create link: %v", err)
			}
		}
	}

	// 首批推文推送至 tweet chan 时调用，意味流水线已启动
	startedDownload := &atomic.Bool{}
	triggerStart := sync.OnceFunc(func() { startedDownload.Store(true) })

	tweetGetter := func() {
		defer getterWg.Done()
		defer panicHandler("getting worker")

		for entity := range entityChan {
			if cancelled() {
				break
			}
			user := uidToUser[entity.Uid()]
			tweets, err := user.GetMeidas(client, &utils.TimeRange{Min: entity.LatestReleaseTime()})
			if utils.IsStatusCode(err, 429) {
				// json 版本的响应 {"errors":[{"code":88,"message":"Rate limit exceeded."}]} 代表达到看帖上限
				// text 版本的响应 Rate limit exceeded. 代表暂时达到速率限制
				v := err.(*utils.HttpStatusError)
				if v.Msg[0] == '{' && v.Msg[len(v.Msg)-1] == '}' {
					color.Error.Tips("[getting worker]: You have reached the limit for seeing posts today.")
				abort()
				} else {
					color.Warn.Tips("[getting worker]: %v", err)
				}
				continue
			}
			if err != nil {
				color.Warn.Tips("[getting worker] %s: %v", entity.Name(), err)
				continue
			}
			if len(tweets) == 0 {
				continue
			}

			// 确保该用户所有推文已推送并更新用户的最新发布时间
			for _, tw := range tweets {
				pt := TweetInEntity{Tweet: tw, Entity: entity}
				tweetChan <- &pt
			}

			if err := entity.SetLatestReleaseTime(tweets[0].CreatedAt); err != nil {
				color.Error.Tips("[getting worker] %s: %v", entity.Name(), err)
				continue
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

	wc := workerController{}
	wc.abort = abort
	wc.cancelled = cancelled
	wc.Wg = &downloaderWg

	newUpstream := func() {
		getterWg.Add(1)
		tweetGetter()
	}
	bl := newBalanceLoader(getterCount, min(userTweetApiLimit, len(users)), newUpstream, func() bool {
		return !twitter.GetClientBlockState(client) && startedDownload.Load()
	})

	for i := 0; i < MaxDownloadRoutine; i++ {
		downloaderWg.Add(1)
		go tweetDownloader(client, wc, errch, tweetChan, bl)
	}

	//closer
	go func() {
		updaterWg.Wait()
		close(entityChan)
		color.Debug.Tips("[sync worker] shutdown elapsed %v", time.Since(start))

		getterWg.Wait()
		close(tweetChan)
		color.Debug.Tips("[getting worker] shutdown elapsed %v", time.Since(start))

		downloaderWg.Wait()
		close(errch)
	}()

	fails := []*TweetInEntity{}

	for {
		select {
		case pt, ok := <-errch:
			if !ok {
				// errch 已关闭，退出循环
				return fails
			}
			fails = append(fails, pt.(*TweetInEntity))
		case sig := <-sigChan:
			// TODO: windows 下，接收到信号并不会中断慢速系统调用，例如IO, Sleep..., unix 暂未测试
			abort() // 接收到信号，执行中断操作
			color.Warn.Tips("caught signal: %v", sig)
		case <-bl.requests:
			bl.do()
		}
	}
}

func downloadList(client *resty.Client, db *sqlx.DB, list twitter.ListBase, dir string, realDir string) ([]*TweetInEntity, error) {
	expectedTitle := utils.WinFileName(list.Title())
	entity := NewListEntity(db, list.GetId(), dir)
	if err := syncPath(entity, expectedTitle); err != nil {
		return nil, err
	}

	members, err := list.GetMembers(client)
	if err != nil || len(members) == 0 {
		return nil, err
	}

	eid := entity.Id()
	color.Debug.Println("members:", len(members))
	return BatchUserDownload(client, db, members, realDir, &eid), nil
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

func DownloadList(client *resty.Client, db *sqlx.DB, list twitter.ListBase, dir string, realDir string) ([]*TweetInEntity, error) {
	tlist, ok := list.(*twitter.List)
	if ok {
		if err := syncList(db, tlist); err != nil {
			return nil, err
		}
	}
	return downloadList(client, db, list, dir, realDir)
}
