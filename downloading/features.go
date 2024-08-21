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

// TODO 多列表同时下载仍会重复同步用户

// 记录本次程序运行已同步过的用户
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

// 负责下载推文，重试，转储，不能让收到的推文丢失
func tweetDownloader(ctx context.Context, client *resty.Client, wg *sync.WaitGroup, errch chan<- PackgedTweet, twech <-chan PackgedTweet, bl *balanceLoader) {
	var pt PackgedTweet
	var ok bool
	defer wg.Done()
	defer func() {
		if p := recover(); p != nil {
			if pt != nil {
				errch <- pt // push 正下载的推文
			}
			// 确保只有1个协程的情况下，未能下载完毕的推文仍然会全部推送到 errch
			for pt := range twech {
				errch <- pt
			}
			color.Error.Tips("[downloading worker]: %v", p)
		}
	}()

	for {
		select {
		case pt, ok = <-twech:
			if !ok {
				return
			}
		case <-time.After(150 * time.Millisecond):
			if bl != nil && bl.shouldNotify() {
				bl.notify()
			}
			continue
		case <-ctx.Done():
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
		err := downloadTweetMedia(ctx, client, path, pt.GetTweet())
		if err != nil && !utils.IsStatusCode(err, 404) {
			errch <- pt
		}
	}
}

func BatchDownloadTweet(ctx context.Context, client *resty.Client, pts ...PackgedTweet) []PackgedTweet {
	if len(pts) == 0 {
		return nil
	}

	var errChan = make(chan PackgedTweet)
	var tweetChan = make(chan PackgedTweet, len(pts))
	var wg sync.WaitGroup // number of working goroutines
	var numRoutine = min(len(pts), MaxDownloadRoutine)

	for _, pt := range pts {
		tweetChan <- pt
	}
	close(tweetChan)

	for i := 0; i < numRoutine; i++ {
		wg.Add(1)
		go tweetDownloader(ctx, client, &wg, errChan, tweetChan, nil)
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
