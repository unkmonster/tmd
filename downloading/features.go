package downloading

import (
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

var mutex sync.Mutex

// 任何一个 url 下载失败直接返回
func DownloadTweetMedia(client *resty.Client, dir string, tweet *twitter.Tweet) error {
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
	return nil
}

func BatchDownloadTweet(client *resty.Client, dir string, tweets []*twitter.Tweet) []*twitter.Tweet {
	var tokens = make(chan struct{}, 50)
	var errChan = make(chan *twitter.Tweet)
	errors := []*twitter.Tweet{}
	var wg sync.WaitGroup // number of working goroutines

	for _, tw := range tweets {
		wg.Add(1)
		go func(twe *twitter.Tweet) {
			defer wg.Done()
			tokens <- struct{}{}
			if err := DownloadTweetMedia(client, dir, twe); err != nil {
				errChan <- twe
				log.Println("failed to download tweet:", err)
			}
			<-tokens
		}(tw)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for range errChan {
		tw := <-errChan
		if tw != nil {
			errors = append(errors, tw)
		}
	}
	return errors
}
