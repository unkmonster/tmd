package downloading

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
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
	fmt.Printf("[%s] %s\n", tweet.Creator.Title(), text)
	return nil
}

func batchDownloadTweet(client *resty.Client, pts ...PackgedTweet) []PackgedTweet {
	var errChan = make(chan PackgedTweet)
	var tweetChan = make(chan PackgedTweet, len(pts))
	var abortChan = make(chan struct{})
	var wg sync.WaitGroup // number of working goroutines
	var numRoutine = min(len(pts), 20)

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

	for i := 0; i < numRoutine; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var pt PackgedTweet
			defer func() {
				if err := recover(); err != nil {
					abort()
					errChan <- pt
				}
			}()

			for pt = range tweetChan {
				if cancelled() {
					errChan <- pt
					continue
				}
				if err := downloadTweetMedia(client, pt.GetPath(), pt.GetTweet()); err != nil {
					errChan <- pt
				}
			}
		}()
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
