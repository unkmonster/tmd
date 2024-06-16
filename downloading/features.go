package downloading

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/unkmonster/tmd2/internal/utils"
	"github.com/unkmonster/tmd2/twitter"
)

// 任何一个媒体下载失败直接返回
func DownloadTweetMedia(client *http.Client, dir string, tweet *twitter.Tweet) error {
	text := string(utils.WinFileName([]byte(tweet.Text)))

	for _, u := range tweet.Urls {
		// 提取扩展名从 Url
		ext, err := utils.GetExtFromUrl(u)
		if err != nil {
			return err
		}

		// 请求
		resp, err := client.Get(u)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if err := utils.CheckRespStatus(resp); err != nil {
			return err
		}

		// 转储
		path, err := utils.UniquePath(filepath.Join(dir, text+ext))
		if err != nil {
			return err
		}
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer os.Chtimes(path, time.Time{}, tweet.CreatedAt)
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return err
		}
	}
	return nil
}
