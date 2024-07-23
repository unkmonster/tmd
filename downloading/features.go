package downloading

import (
	"fmt"
	"io"
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
func downloadTweetMedia(client *resty.Client, dir string, tweet *twitter.Tweet) error {
	text := string(utils.WinFileName([]byte(tweet.Text)))

	for _, u := range tweet.Urls {
		ext, err := utils.GetExtFromUrl(u)
		if err != nil {
			return err
		}

		// 请求
		resp, err := client.R().Get(u)
		if err != nil {
			return err
		}
		if err := utils.CheckRespStatus(resp); err != nil {
			return err
		}

		// 转储
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

		_, err = io.WriteString(file, resp.String())
		if err != nil {
			return err
		}
	}

	fmt.Printf("%s %s\n", color.FgLightMagenta.Render("["+tweet.Creator.Title()+"]"), text)
	return nil
}

var MaxDownloadRoutine int

// TODO 多列表同时下载仍会重复同步用户
var syncedUsers sync.Map

func init() {
	MaxDownloadRoutine = runtime.GOMAXPROCS(0) * 5
	//color.Info.Tips("MAX_DOWNLOAD_ROUTINE: %d\n", maxDownloadRoutine)
}

type workerController struct {
	Wg        *sync.WaitGroup
	cancelled func() bool
	abort     func()
}

func tweetDownloader(client *resty.Client, wc workerController, errch chan<- PackgedTweet, twech <-chan PackgedTweet) {
	var pt PackgedTweet
	defer wc.Wg.Done()
	defer func() {
		if p := recover(); p != nil {
			wc.abort()
			if pt != nil {
				errch <- pt
				color.Error.Tips("[downloading worker]: %v", p)
			}
		}
	}()

	for pt = range twech {
		if wc.cancelled() {
			errch <- pt
			continue
		}

		path := pt.GetPath()
		if path == "" {
			errch <- pt
			continue
		}
		err := downloadTweetMedia(client, path, pt.GetTweet())
		if err != nil && !utils.IsStatusCode(err, 404) {
			errch <- pt
		}
	}
}

func BatchDownloadTweet(client *resty.Client, pts ...PackgedTweet) []PackgedTweet {
	var errChan = make(chan PackgedTweet)
	var tweetChan = make(chan PackgedTweet, len(pts))
	var abortChan = make(chan struct{})
	var wg sync.WaitGroup // number of working goroutines
	var numRoutine = min(len(pts), MaxDownloadRoutine)

	for _, pt := range pts {
		tweetChan <- pt
	}
	close(tweetChan)

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

	wc := workerController{}
	wc.abort = abort
	wc.cancelled = cancelled
	wc.Wg = &wg

	for i := 0; i < numRoutine; i++ {
		wg.Add(1)
		go tweetDownloader(client, wc, errChan, tweetChan)
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
