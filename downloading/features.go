package downloading

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	"github.com/panjf2000/ants/v2"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/database"
	"github.com/unkmonster/tmd/internal/utils"
	"github.com/unkmonster/tmd/twitter"
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

// map[user_id]*UserEntity 记录本次程序运行已同步过的用户
var syncedUsers sync.Map

func init() {
	MaxDownloadRoutine = runtime.GOMAXPROCS(0) * 10
}

type workerConfig struct {
	ctx    context.Context
	wg     *sync.WaitGroup
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
		// 403: Dmcaed
		if err != nil && !utils.IsStatusCode(err, 404) && !utils.IsStatusCode(err, 403) {
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

const userTweetRateLimit = 500
const userTweetMaxConcurrent = 100 // avoid DownstreamOverCapacityError

// var syncedListUsers = make(map[uint64]map[int64]struct{})
var syncedListUsers sync.Map //leid -> uid -> struct{}

// 需要请求多少次时间线才能获取完毕用户的推文？
func calcUserDepth(exist int, total int) int {
	if exist >= total {
		return 1
	}

	miss := total - exist
	depth := miss / twitter.AvgTweetsPerPage
	if miss%twitter.AvgTweetsPerPage != 0 {
		depth++
	}
	if exist == 0 {
		depth++ //对于新用户，需要多获取一个空页
	}
	return depth
}

type userInLstEntity struct {
	user *twitter.User
	leid *int
}

func shouldIngoreUser(user *twitter.User) bool {
	return user.Blocking || user.Muting
}

func BatchUserDownload(ctx context.Context, client *resty.Client, db *sqlx.DB, users []userInLstEntity, dir string, autoFollow bool, additional []*resty.Client) ([]*TweetInEntity, error) {
	if len(users) == 0 {
		return nil, nil
	}

	uidToUser := make(map[uint64]*twitter.User)
	for _, u := range users {
		uidToUser[u.user.Id] = u.user
	}

	// channels
	tweetChan := make(chan PackgedTweet, MaxDownloadRoutine)
	errChan := make(chan PackgedTweet)
	// WG
	prodwg := sync.WaitGroup{}
	conswg := sync.WaitGroup{}
	// ctx
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	// logger
	updaterLogger := log.WithField("worker", "updating")
	getterLogger := log.WithField("worker", "getting")

	panicHandler := func() {
		if r := recover(); r != nil {
			cancel(fmt.Errorf("%v", r))
			buf := make([]byte, 1<<16)
			n := runtime.Stack(buf, false)
			fmt.Printf("Recovered from panic: %v\n%s\n", r, buf[:n])
		}
	}

	missingTweets := 0
	depthByEntity := make(map[*UserEntity]int)
	// 大顶堆，以用户深度
	userEntityHeap := utils.NewHeap(func(lhs, rhs *UserEntity) bool {
		luser, ruser := uidToUser[lhs.Uid()], uidToUser[rhs.Uid()]
		lOnlyMater := luser.IsProtected && luser.Followstate == twitter.FS_FOLLOWING
		rOnlyMaster := ruser.IsProtected && ruser.Followstate == twitter.FS_FOLLOWING
		if lOnlyMater && !rOnlyMaster {
			return true // 优先让 master 获取只有他能看到的
		}
		return depthByEntity[lhs] > depthByEntity[rhs]
	})

	start := time.Now()

	// pre-process
	func() {
		defer panicHandler()
		log.Debugln("start pre processing users")

		for _, userInLST := range users {
			var pathEntity *UserEntity
			var err error
			user := userInLST.user
			leid := userInLST.leid

			if shouldIngoreUser(user) {
				continue
			}

			pe, loaded := syncedUsers.Load(user.Id)
			if !loaded {
				pathEntity, err = syncUserAndEntity(db, user, dir)
				if err != nil {
					updaterLogger.WithField("user", user.Title()).Warnln("failed to update user or entity", err)
					continue
				}
				syncedUsers.Store(user.Id, pathEntity)

				// 同步所有现存的指向此用户的符号链接
				upath, _ := pathEntity.Path()
				linkds, err := database.GetUserLinks(db, user.Id)
				if err != nil {
					updaterLogger.WithField("user", user.Title()).Warnln("failed to get links to user:", err)
				}
				for _, linkd := range linkds {
					if err = updateUserLink(linkd, db, upath); err != nil {
						updaterLogger.WithField("user", user.Title()).Warnln("failed to update link:", err)
					}
					sl, _ := syncedListUsers.LoadOrStore(int(linkd.ParentLstEntityId), &sync.Map{})
					syncedList := sl.(*sync.Map)
					syncedList.Store(user.Id, struct{}{})
				}

				// 计算深度
				if user.MediaCount != 0 && user.IsVisiable() {
					missingTweets += max(0, user.MediaCount-int(pathEntity.record.MediaCount.Int32))
					depthByEntity[pathEntity] = calcUserDepth(int(pathEntity.record.MediaCount.Int32), user.MediaCount)
					userEntityHeap.Push(pathEntity)
				}

				// 自动关注
				if user.IsProtected && user.Followstate == twitter.FS_UNFOLLOW && autoFollow {
					if err := twitter.FollowUser(ctx, client, user); err != nil {
						log.WithField("user", user.Title()).Warnln("failed to follow user:", err)
					} else {
						log.WithField("user", user.Title()).Debugln("follow request has been sent")
					}
				}
			} else {
				pathEntity = pe.(*UserEntity)
			}

			// 即便同步一个用户时也同步了所有指向此用户的链接，
			// 但此用户仍可能会是一个新的 “列表-用户”，所以判断此用户链接是否同步过，
			// 如果否，那么创建一个属于此列表的用户链接
			if leid == nil {
				continue
			}
			sl, _ := syncedListUsers.LoadOrStore(*leid, &sync.Map{})
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
			curlink.ParentLstEntityId = int32(*leid)
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
	}()

	if userEntityHeap.Empty() {
		return nil, nil
	}
	log.Debugln("preprocessing finish, elapsed:", time.Since(start))
	log.Debugln("real members:", userEntityHeap.Size())
	log.Debugln("missing tweets:", missingTweets)
	log.Debugln("deepest:", depthByEntity[userEntityHeap.Peek()])

	clients := make([]*resty.Client, 0)
	clients = append(clients, client)
	clients = append(clients, additional...)

	producer := func(entity *UserEntity) {
		defer prodwg.Done()
		defer panicHandler()

		user := uidToUser[entity.Uid()]
		cli := twitter.SelectUserMediaClient(ctx, clients)
		if ctx.Err() != nil {
			userEntityHeap.Push(entity)
			return
		}
		if cli == nil {
			userEntityHeap.Push(entity)
			cancel(fmt.Errorf("no client available"))
			return
		}

		tweets, err := user.GetMeidas(ctx, cli, &utils.TimeRange{Min: entity.LatestReleaseTime()})
		if err == twitter.ErrWouldBlock {
			userEntityHeap.Push(entity)
			return
		}
		if v, ok := err.(*twitter.TwitterApiError); ok {
			// 客户端不再可用
			if v.Code == twitter.ErrExceedPostLimit {
				twitter.SetClientError(cli, fmt.Errorf("reached the limit for seeing posts today"))
				userEntityHeap.Push(entity)
				return
			} else if v.Code == twitter.ErrAccountLocked {
				twitter.SetClientError(cli, fmt.Errorf("account is locked"))
				userEntityHeap.Push(entity)
				return
			}
		}
		if ctx.Err() != nil {
			userEntityHeap.Push(entity)
			return
		}
		if err != nil {
			getterLogger.WithField("user", entity.Name()).Warnln("failed to get user medias:", err)
			return
		}

		if len(tweets) == 0 {
			if err := database.UpdateUserEntityMediCount(db, entity.Id(), user.MediaCount); err != nil {
				getterLogger.WithField("user", entity.Name()).Panicln("failed to update user medias count:", err)
			}
			return
		}

		// 确保该用户所有推文已推送并更新用户推文状态
		for _, tw := range tweets {
			pt := TweetInEntity{Tweet: tw, Entity: entity}
			select {
			case tweetChan <- &pt:
			case <-ctx.Done():
				return // 防止无消费者导致死锁
			}
		}

		if err := database.UpdateUserEntityTweetStat(db, entity.Id(), tweets[0].CreatedAt, user.MediaCount); err != nil {
			// 影响程序的正确性，必须 Panic
			getterLogger.WithField("user", entity.Name()).Panicln("failed to update user tweets stat:", err)
		}
	}

	// launch worker
	config := workerConfig{
		ctx:    ctx,
		wg:     &conswg,
		cancel: cancel,
	}
	for i := 0; i < MaxDownloadRoutine; i++ {
		conswg.Add(1)
		go tweetDownloader(client, &config, errChan, tweetChan)
	}

	producerPool, err := ants.NewPool(min(userTweetMaxConcurrent, userEntityHeap.Size()))
	if err != nil {
		return nil, err
	}
	defer ants.Release()

	//closer
	go func() {
		// 按批次调用生产者
		for !userEntityHeap.Empty() && ctx.Err() == nil {
			selected := []int{}
			for count := 0; count < userTweetRateLimit && ctx.Err() == nil; {
				if userEntityHeap.Empty() {
					break
				}

				entity := userEntityHeap.Peek()
				depth := depthByEntity[entity]
				if depth+count > userTweetRateLimit {
					break
				}

				prodwg.Add(1)
				producerPool.Submit(func() {
					producer(entity)
				})
				selected = append(selected, depth)

				count += depth
				//delete(depthByEntity, entity)
				userEntityHeap.Pop()
			}
			log.Debugln(selected)
			prodwg.Wait()
		}
		close(tweetChan)
		log.Debugf("getting tweets completed, elapsed time: %v", time.Since(start))

		conswg.Wait()
		close(errChan)
	}()

	fails := []*TweetInEntity{}
	for pt := range errChan {
		fails = append(fails, pt.(*TweetInEntity))
	}
	log.Debugf("%d users unable to start", userEntityHeap.Size())
	return fails, context.Cause(ctx)
}

func downloadList(ctx context.Context, client *resty.Client, db *sqlx.DB, list twitter.ListBase, dir string, realDir string, autoFollow bool, additional []*resty.Client) ([]*TweetInEntity, error) {
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
	packgedUsers := make([]userInLstEntity, len(members))
	for i, user := range members {
		packgedUsers[i] = userInLstEntity{user: user, leid: &eid}
	}
	return BatchUserDownload(ctx, client, db, packgedUsers, realDir, autoFollow, additional)
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

func DownloadList(ctx context.Context, client *resty.Client, db *sqlx.DB, list twitter.ListBase, dir string, realDir string, autoFollow bool, additional []*resty.Client) ([]*TweetInEntity, error) {
	tlist, ok := list.(*twitter.List)
	if ok {
		if err := syncList(db, tlist); err != nil {
			return nil, err
		}
	}
	return downloadList(ctx, client, db, list, dir, realDir, autoFollow, additional)
}

func syncLstAndGetMembers(ctx context.Context, client *resty.Client, db *sqlx.DB, lst twitter.ListBase, dir string) ([]userInLstEntity, error) {
	if v, ok := lst.(*twitter.List); ok {
		if err := syncList(db, v); err != nil {
			return nil, err
		}
	}

	// update lst path and record
	expectedTitle := utils.WinFileName(lst.Title())
	entity, err := NewListEntity(db, lst.GetId(), dir)
	if err != nil {
		return nil, err
	}
	if err := syncPath(entity, expectedTitle); err != nil {
		return nil, err
	}

	// get all members
	members, err := lst.GetMembers(ctx, client)
	if err != nil || len(members) == 0 {
		return nil, err
	}

	// bind lst entity to users for creating symlink
	packgedUsers := make([]userInLstEntity, 0, len(members))
	eid := entity.Id()
	for _, user := range members {
		packgedUsers = append(packgedUsers, userInLstEntity{user: user, leid: &eid})
	}
	return packgedUsers, nil
}

func BatchDownloadAny(ctx context.Context, client *resty.Client, db *sqlx.DB, lists []twitter.ListBase, users []*twitter.User, dir string, realDir string, autoFollow bool, additional []*resty.Client) ([]*TweetInEntity, error) {
	log.Debugln("start collecting users")
	packgedUsers := make([]userInLstEntity, 0)
	wg := sync.WaitGroup{}
	mtx := sync.Mutex{}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	for _, lst := range lists {
		wg.Add(1)
		go func(lst twitter.ListBase) {
			defer wg.Done()
			res, err := syncLstAndGetMembers(ctx, client, db, lst, dir)
			if err != nil {
				cancel(err)
			}
			log.Debugf("members of %s: %d", lst.Title(), len(res))
			mtx.Lock()
			defer mtx.Unlock()
			packgedUsers = append(packgedUsers, res...)
		}(lst)
	}
	wg.Wait()
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}

	for _, usr := range users {
		packgedUsers = append(packgedUsers, userInLstEntity{user: usr, leid: nil})
	}

	log.Debugln("collected users:", len(packgedUsers))
	return BatchUserDownload(ctx, client, db, packgedUsers, realDir, autoFollow, additional)
}
