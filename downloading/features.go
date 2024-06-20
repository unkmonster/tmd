package downloading

import (
	"fmt"
	"io"
	"log"
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
	var tokens = make(chan struct{}, numToken)
	var errChan = make(chan PackgedTweet)
	errors := []PackgedTweet{}
	var wg sync.WaitGroup // number of working goroutines

	for _, pt := range pts {
		wg.Add(1)
		go func(pt PackgedTweet) {
			defer wg.Done()
			tokens <- struct{}{}
			// !! path 会不会为空字符串？
			if err := downloadTweetMedia(client, pt.GetPath(), pt.GetTweet()); err != nil {
				errChan <- pt
				log.Println("failed to download tweet:", err)
			}
			<-tokens
		}(pt)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for range errChan {
		pt := <-errChan
		errors = append(errors, pt)
	}
	return errors
}

var numToken = 20
