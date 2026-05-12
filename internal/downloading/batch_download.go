package downloading

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/panjf2000/ants/v2"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/entity"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

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
		depth++
	}
	return depth
}

func BatchUserDownload(ctx context.Context, client *resty.Client, db *sqlx.DB, users []userInListEntity, dir string, autoFollow bool, additional []*resty.Client, dwn downloader.Downloader, fileWriter downloader.FileWriter, progress BatchProgressFunc) ([]*TweetInEntity, BatchDownloadSummary, error) {
	if len(users) == 0 {
		return nil, BatchDownloadSummary{}, nil
	}

	uidToUser := make(map[uint64]*twitter.User)
	for _, u := range users {
		if u.user == nil {
			continue
		}
		uidToUser[u.user.Id] = u.user
	}

	tweetChan := make(chan PackagedTweet, MaxDownloadRoutine)
	errChan := make(chan PackagedTweet)
	prodwg := sync.WaitGroup{}
	conswg := sync.WaitGroup{}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	syncState := newBatchSyncState()

	symlinkWarnCount := 0
	symlinkWarnMu := sync.Mutex{}

	panicHandler := func() {
		if r := recover(); r != nil {
			cancel(fmt.Errorf("%v", r))
			buf := make([]byte, 1<<16)
			n := runtime.Stack(buf, false)
			fmt.Printf("Recovered from panic: %v\n%s\n", r, buf[:n])
		}
	}

	missingTweets := 0
	depthByEntity := make(map[*entity.UserEntity]int)
	userEntityHeap := utils.NewHeap(func(lhs, rhs *entity.UserEntity) bool {
		luser, ruser := uidToUser[lhs.UserId()], uidToUser[rhs.UserId()]
		lOnlyMater := luser != nil && luser.IsProtected && luser.Followstate == twitter.FS_FOLLOWING
		rOnlyMaster := ruser != nil && ruser.IsProtected && ruser.Followstate == twitter.FS_FOLLOWING

		if lOnlyMater == rOnlyMaster {
			return depthByEntity[lhs] > depthByEntity[rhs]
		}
		return lOnlyMater
	})

	start := time.Now()

	// 统计未关注且受保护的账户（无法下载内容的用户）
	var protectedUnfollowedUsers []*twitter.User
	for _, u := range users {
		if u.user != nil && u.user.IsProtected && u.user.Followstate == twitter.FS_UNFOLLOW {
			protectedUnfollowedUsers = append(protectedUnfollowedUsers, u.user)
		}
	}
	if len(protectedUnfollowedUsers) > 0 {
		log.Infof("未关注且受保护的账户 (%d，无法下载内容):", len(protectedUnfollowedUsers))
		for _, u := range protectedUnfollowedUsers {
			log.Infof("  - %s(@%s)", u.Name, u.ScreenName)
		}
	}

	func() {
		defer panicHandler()
		log.Infoln("start pre processing users")

		for _, userInLST := range users {
			var pathEntity *entity.UserEntity
			var err error
			user := userInLST.user
			leid := userInLST.leid

			if shouldIgnoreUser(user) {
				continue
			}

			pathEntity, loaded := syncState.loadUser(user.Id)
			if !loaded {
				pathEntity, err = syncUserAndEntity(db, user, dir)
				if err != nil {
					log.Warnln("✗", user.Title(), "-", "failed to update user or entity", err)
					continue
				}
				syncState.storeUser(user.Id, pathEntity)

				upath, _ := pathEntity.Path()
				linkds, err := database.GetUserLinks(db, user.Id)
				if err != nil {
					log.Warnln("✗", user.Title(), "-", "failed to get links to user:", err)
				}
				for _, linkd := range linkds {
					if err = updateUserLink(linkd, db, upath); err != nil {
						symlinkWarnMu.Lock()
						symlinkWarnCount++
						if symlinkWarnCount == 1 {
							log.Warnln("✗", user.Title(), "-", "symlink permission denied (suppressing further warnings)")
						}
						symlinkWarnMu.Unlock()
					}
					syncState.markListUser(int(linkd.ParentLstEntityId), user.Id)
				}

				if user.MediaCount != 0 && user.IsVisiable() {
					missingTweets += max(0, user.MediaCount-int(pathEntity.MediaCount()))
					depthByEntity[pathEntity] = calcUserDepth(int(pathEntity.MediaCount()), user.MediaCount)
					userEntityHeap.Push(pathEntity)
				}

				if user.IsProtected && user.Followstate == twitter.FS_UNFOLLOW && autoFollow {
					if err := twitter.FollowUser(ctx, client, user); err != nil {
						log.Warnln("✗", user.Title(), "-", "failed to follow user:", err)
					} else {
						log.Debugln("✓", user.Title(), "-", "follow request has been sent")
					}
				}
			}

			if leid == 0 {
				continue
			}
			if !syncState.markListUser(leid, user.Id) {
				continue
			}

			upath, _ := pathEntity.Path()
			linkname, err := pathEntity.Name()
			if err != nil {
				log.Warnln("✗", user.Title(), "-", "failed to get entity name:", err)
				continue
			}

			curlink := &database.UserLink{}
			curlink.Name = linkname
			curlink.ParentLstEntityId = int32(leid)
			curlink.UserId = user.Id

			linkpath, err := curlink.Path(db)
			if err == nil {
				linkDir := filepath.Dir(linkpath)
				if mkdirErr := os.MkdirAll(linkDir, 0755); mkdirErr == nil {
					if err = os.Symlink(upath, linkpath); err == nil || os.IsExist(err) {
						err = database.CreateUserLink(db, curlink)
					}
				} else {
					err = mkdirErr
				}
			}
			if err != nil {
				symlinkWarnMu.Lock()
				symlinkWarnCount++
				if symlinkWarnCount == 1 {
					log.Warnln("✗", user.Title(), "-", "symlink permission denied (suppressing further warnings)")
				}
				symlinkWarnMu.Unlock()
			}
		}
	}()

	if userEntityHeap.Empty() {
		return nil, BatchDownloadSummary{}, nil
	}
	log.Debugln("preprocessing finish, elapsed:", time.Since(start))
	log.Debugln("real members:", userEntityHeap.Size())
	log.Debugln("missing tweets:", missingTweets)
	if symlinkWarnCount > 0 {
		log.Warnf("symlink permission denied: %d errors suppressed (run as admin to enable symlinks)", symlinkWarnCount)
	}

	totalUsers := userEntityHeap.Size()
	summary := BatchDownloadSummary{TotalEntities: totalUsers}
	var completedUsers atomic.Int64
	var failedTweets atomic.Int64
	type userProgressState struct {
		total     int
		completed int
		current   string
	}
	userProgress := make(map[int]*userProgressState)
	var progressMu sync.Mutex

	reportProgress := func(current string) {
		if progress == nil {
			return
		}
		progress(BatchProgress{
			Total:     totalUsers,
			Completed: int(completedUsers.Load()),
			Failed:    int(failedTweets.Load()),
			Current:   current,
		})
	}

	markUserDone := func(current string) {
		completedUsers.Add(1)
		reportProgress(current)
	}

	producer := func(ent *entity.UserEntity) {
		defer prodwg.Done()
		defer panicHandler()

		user := uidToUser[ent.UserId()]
		if user == nil {
			log.Warnln("✗", fmt.Sprintf("(uid:%d)", ent.UserId()), "-", "user not found in uidToUser, skipping")
			markUserDone("")
			return
		}

		entityName, nameErr := ent.Name()
		if nameErr != nil {
			log.Warnln("✗", user.Title(), "-", "failed to get entity name:", nameErr)
			markUserDone(user.ScreenName)
			return
		}

		cli := twitter.SelectClientMFQ(ctx, client, additional, user, "/i/api/graphql/MOLbHrtk8Ovu7DUNOLcXiA/UserMedia")
		if ctx.Err() != nil {
			userEntityHeap.Push(ent)
			return
		}
		if cli == nil {
			userEntityHeap.Push(ent)
			cancel(fmt.Errorf("no client available"))
			return
		}

		minTime, err := ent.LatestReleaseTime()
		if err != nil {
			log.Warnln("✗", entityName, "-", "failed to get latest release time:", err)
			markUserDone(user.ScreenName)
			return
		}
		tweets, err := user.GetMedias(ctx, cli, &utils.TimeRange{Min: minTime})
		if err == twitter.ErrWouldBlock {
			userEntityHeap.Push(ent)
			return
		}
		var apiErr *twitter.TwitterApiError
		if errors.As(err, &apiErr) {
			switch apiErr.Code {
			case twitter.ErrExceedPostLimit:
				twitter.SetClientError(cli, fmt.Errorf("reached the limit for seeing posts today"))
				userEntityHeap.Push(ent)
				return
			case twitter.ErrAccountLocked:
				twitter.SetClientError(cli, fmt.Errorf("account is locked"))
				userEntityHeap.Push(ent)
				return
			}
		}
		if ctx.Err() != nil {
			userEntityHeap.Push(ent)
			return
		}
		if err != nil {
			log.Warnln("✗", entityName, "-", "failed to get user medias:", err)
			markUserDone(user.ScreenName)
			return
		}

		eid, idErr := ent.Id()
		if idErr != nil {
			log.Warnln("✗", entityName, "-", "failed to get entity id:", idErr)
			markUserDone(user.ScreenName)
			return
		}

		if len(tweets) == 0 {
			if err := database.UpdateUserEntityMediCount(db, eid, user.MediaCount); err != nil {
				log.Errorln("✗", entityName, "-", "failed to update user medias count:", err)
			}
			markUserDone(user.ScreenName)
			return
		}
		progressMu.Lock()
		userProgress[eid] = &userProgressState{total: len(tweets), current: user.ScreenName}
		progressMu.Unlock()
		reportProgress(user.ScreenName)

		for _, tw := range tweets {
			pt := TweetInEntity{Tweet: tw, Entity: ent}
			select {
			case tweetChan <- &pt:
			case <-ctx.Done():
				return
			}
		}

		if err := database.UpdateUserEntityTweetStat(db, eid, tweets[0].CreatedAt, user.MediaCount); err != nil {
			log.Errorln("✗", entityName, "-", "failed to update user tweets stat:", err)
		}
	}

	config := workerConfig{
		ctx:        ctx,
		wg:         &conswg,
		cancel:     cancel,
		downloader: dwn,
		fileWriter: fileWriter,
		client:     client,
	}
	if progress != nil {
		config.onTweetDone = func(pt PackagedTweet, failed bool) {
			current := ""
			tweet := pt.GetTweet()
			if tweet != nil && tweet.Creator != nil {
				current = tweet.Creator.ScreenName
			}

			failedCount := int(failedTweets.Load())
			if failed {
				failedCount = int(failedTweets.Add(1))
			}

			userDone := false
			if tie, ok := pt.(*TweetInEntity); ok && tie.Entity != nil {
				if current == "" {
					if user := uidToUser[tie.Entity.UserId()]; user != nil {
						current = user.ScreenName
					}
				}
				if eid, err := tie.Entity.Id(); err == nil {
					progressMu.Lock()
					if state, ok := userProgress[eid]; ok {
						state.completed++
						if current == "" {
							current = state.current
						}
						if state.completed >= state.total {
							delete(userProgress, eid)
							userDone = true
						}
					}
					progressMu.Unlock()
				}
			}
			if userDone {
				completedUsers.Add(1)
			}
			progress(BatchProgress{
				Total:     totalUsers,
				Completed: int(completedUsers.Load()),
				Failed:    failedCount,
				Current:   current,
			})
		}
	}
	for i := 0; i < MaxDownloadRoutine; i++ {
		conswg.Add(1)
		go tweetDownloader(&config, errChan, tweetChan)
	}

	producerPool, err := ants.NewPool(min(userTweetMaxConcurrent, userEntityHeap.Size()))
	if err != nil {
		return nil, summary, err
	}
	defer producerPool.Release()

	go func() {
		for !userEntityHeap.Empty() && ctx.Err() == nil {
			selected := []int{}
			for count := 0; count < userTweetRateLimit && ctx.Err() == nil; {
				if userEntityHeap.Empty() {
					break
				}

				entity := userEntityHeap.Peek()
				depth := depthByEntity[entity]
				if depth > userTweetRateLimit {
					entityName, nameErr := entity.Name()
					if nameErr != nil {
						entityName = fmt.Sprintf("(uid:%d)", entity.UserId())
					}
					log.Warnln("user depth exceeds limit:", entityName, "- depth:", depth)
					userEntityHeap.Pop()
					markUserDone(entityName)
					continue
				}

				if depth+count > userTweetRateLimit {
					break
				}

				prodwg.Add(1)
				producerPool.Submit(func() {
					producer(entity)
				})
				selected = append(selected, depth)

				count += depth
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
	return fails, summary, context.Cause(ctx)
}
