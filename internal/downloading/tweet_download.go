package downloading

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/naming"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

var mediaMutex sync.Mutex

func saveTweetJson(cfg *workerConfig, dir string, tweet *twitter.Tweet, namingObj *naming.TweetNaming) {
	if dir == "" || tweet == nil || cfg.fileWriter == nil {
		return
	}

	go func() {
		defer utils.RecoverWithLog("saveTweetJson")

		loongDir := filepath.Join(dir, ".loongtweet")

		jsonPath, err := namingObj.FilePath(loongDir, ".json")
		if err != nil {
			return
		}

		data, err := cleanTweetJson([]byte(tweet.RawJSON))
		if err != nil {
			return
		}

		formatted, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return
		}

		writeReq := downloader.WriteRequest{
			Path: jsonPath,
			Data: formatted,
			Options: downloader.WriteOptions{
				ModTime: &tweet.CreatedAt,
			},
		}

		if _, err := cfg.fileWriter.Write(writeReq); err != nil {
			log.Warnf("failed to write tweet json: %v", err)
		}
	}()
}

func saveLoongTweet(cfg *workerConfig, dir string, tweet *twitter.Tweet, namingObj *naming.TweetNaming) {
	if dir == "" || tweet == nil || cfg.fileWriter == nil {
		return
	}

	go func() {
		defer utils.RecoverWithLog("saveLoongTweet")

		loongDir := filepath.Join(dir, ".loongtweet")

		txtPath, err := namingObj.FilePath(loongDir, ".txt")
		if err != nil {
			return
		}

		var screenName, text string
		var tweetID uint64
		var createdAt time.Time
		var mediaCount int

		if tweet.RawJSON != "" {
			result := gjson.Parse(tweet.RawJSON)
			tweetID = result.Get("rest_id").Uint()
			if createdAtStr := result.Get("legacy.created_at").String(); createdAtStr != "" {
				createdAt, _ = time.Parse(time.RubyDate, createdAtStr)
			}
			noteText := result.Get("note_tweet.note_tweet_results.result.text").String()
			if noteText != "" {
				text = noteText
			} else {
				text = result.Get("legacy.full_text").String()
			}
			screenName = result.Get("core.user_results.result.legacy.screen_name").String()
			if screenName == "" {
				screenName = "unknown"
			}
			if media := result.Get("legacy.extended_entities.media"); media.Exists() {
				mediaCount = len(media.Array())
			}
		} else {
			tweetID = tweet.Id
			createdAt = tweet.CreatedAt
			text = tweet.Text
			if tweet.Creator != nil && tweet.Creator.ScreenName != "" {
				screenName = tweet.Creator.ScreenName
			} else {
				screenName = "unknown"
			}
			mediaCount = len(tweet.Urls)
		}

		txtContent := fmt.Sprintf("time:%s\nurl:https://x.com/%s/status/%d\nmedia:%d\n\n%s",
			createdAt.Format("2006-01-02T15:04:05"),
			screenName,
			tweetID,
			mediaCount,
			text)

		writeReq := downloader.WriteRequest{
			Path: txtPath,
			Data: []byte(txtContent),
			Options: downloader.WriteOptions{
				ModTime: &createdAt,
			},
		}

		if _, err := cfg.fileWriter.Write(writeReq); err != nil {
			log.Warnf("failed to write loongtweet txt: %v", err)
		}
	}()
}

func cleanTweetJson(raw []byte) (any, error) {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	m, ok := data.(map[string]any)
	if !ok {
		return data, nil
	}

	if legacy, ok := m["legacy"].(map[string]any); ok {
		delete(legacy, "user_id_str")
		delete(legacy, "id_str")

		if entities, ok := legacy["entities"].(map[string]any); ok {
			delete(entities, "media")
			delete(entities, "symbols")
			delete(entities, "timestamps")
			delete(entities, "urls")
			delete(entities, "user_mentions")
		}
	}

	if core, ok := m["core"].(map[string]any); ok {
		if userResults, ok := core["user_results"].(map[string]any); ok {
			if result, ok := userResults["result"].(map[string]any); ok {
				delete(result, "id")
				if userLegacy, ok := result["legacy"].(map[string]any); ok {
					if profileImg, ok := userLegacy["profile_image_url_https"].(string); ok {
						userLegacy["profile_image_url_https"] = utils.StripAvatarSuffix(profileImg)
					}
				}
			}
		}
	}

	cleanMediaRecursive(m)

	return m, nil
}

func cleanMediaRecursive(data any) {
	switch v := data.(type) {
	case map[string]any:
		if media, ok := v["extended_entities"].(map[string]any); ok {
			if mediaList, ok := media["media"].([]any); ok {
				for _, item := range mediaList {
					if m, ok := item.(map[string]any); ok {
						delete(m, "media_results")

						if originalInfo, ok := m["original_info"].(map[string]any); ok {
							delete(originalInfo, "focus_rects")
						}

						if features, ok := m["features"].(map[string]any); ok {
							delete(features, "large")
							delete(features, "medium")
							delete(features, "small")
						}

						if mediaType, ok := m["type"].(string); ok && mediaType == "photo" {
							if rawUrl, ok := m["media_url_https"].(string); ok {
								if strings.Contains(rawUrl, "twimg.com") && !strings.Contains(rawUrl, "?name=") {
									m["media_url_https"] = rawUrl + "?name=4096x4096"
								}
							}
						}
					}
				}
			}
		}
		for _, val := range v {
			cleanMediaRecursive(val)
		}
	case []any:
		for _, item := range v {
			cleanMediaRecursive(item)
		}
	}
}

func downloadTweetMedia(cfg *workerConfig, dir string, tweet *twitter.Tweet, skipLoongTweet bool) error {
	var creatorTitle string
	if tweet.Creator != nil {
		creatorTitle = tweet.Creator.Title()
	} else {
		creatorTitle = "unknown"
	}
	tweetNaming := naming.NewTweetNaming(tweet.Text, tweet.Id, creatorTitle)

	if !skipLoongTweet {
		saveTweetJson(cfg, dir, tweet, tweetNaming)
		saveLoongTweet(cfg, dir, tweet, tweetNaming)
	}

	// 用于收集仍然失败的URL
	failedUrls := make([]string, 0)
	// 用于收集成功的URL（用于日志）
	successUrls := make([]string, 0)

	for _, u := range tweet.Urls {
		ext, err := utils.GetExtFromUrl(u)
		if err != nil {
			ext = ".jpg"
		}

		queryParams := make(map[string]string)
		if !strings.Contains(u, "tweet_video") && !strings.Contains(u, "video.twimg.com") && !strings.Contains(u, "?name=") {
			queryParams["name"] = "4096x4096"
		}

		mediaMutex.Lock()
		path, err := tweetNaming.FilePath(dir, ext)
		if err != nil {
			mediaMutex.Unlock()
			failedUrls = append(failedUrls, u)
			continue
		}
		mediaMutex.Unlock()

		req := downloader.DownloadRequest{
			Context:     cfg.ctx,
			Client:      cfg.client,
			URL:         u,
			Destination: path,
			Options: downloader.DownloadOptions{
				QueryParams: queryParams,
				SetModTime:  &tweet.CreatedAt,
			},
		}

		result, err := cfg.downloader.Download(req)
		if err != nil {
			log.Warnln("failed to download media:", u, "-", err)
			failedUrls = append(failedUrls, u)
			continue
		}
		if !result.Success {
			log.Warnln("media download reported failure:", u, "-", result.Error)
			failedUrls = append(failedUrls, u)
			continue
		}
		successUrls = append(successUrls, u)
	}

	// 更新 tweet.Urls：只保留失败的URL
	tweet.Urls = failedUrls

	// 只在至少一个媒体下载成功时才打印推文标题
	if len(successUrls) > 0 {
		fmt.Printf("%s", color.FgLightMagenta.Render(tweetNaming.LogFormat()))
		if len(failedUrls) > 0 {
			fmt.Printf(" [%d/%d succeeded]", len(successUrls), len(successUrls)+len(failedUrls))
		}
		fmt.Println()
	}

	// 只要有失败的URL，就返回错误，让推文进入重试队列
	if len(failedUrls) > 0 {
		return fmt.Errorf("%d media(s) failed to download", len(failedUrls))
	}
	return nil
}

func tweetDownloader(config *workerConfig, errch chan<- PackagedTweet, twech <-chan PackagedTweet) {
	var pt PackagedTweet
	var ok bool

	defer config.wg.Done()
	defer func() {
		if p := recover(); p != nil {
			config.cancel(fmt.Errorf("%v", p))
			log.Errorln("✗ [downloading] - panic:", p)

			if pt != nil {
				errch <- pt
			}
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
			if !config.skipLoongTweet {
				tweet := pt.GetTweet()
				if tweet != nil && tweet.Creator != nil {
					if tie, ok := pt.(TweetInEntity); ok && tie.Entity != nil {
						parentDir := tie.Entity.ParentDir()
						if parentDir != "" {
							userNaming := naming.NewUserNaming(tweet.Creator.Name, tweet.Creator.ScreenName)
							userDirName := userNaming.SanitizedTitle()
							userDir := filepath.Join(parentDir, userDirName)
							tweetNaming := naming.NewTweetNaming(tweet.Text, tweet.Id, tweet.Creator.Title())
							saveTweetJson(config, userDir, tweet, tweetNaming)
							saveLoongTweet(config, userDir, tweet, tweetNaming)
						}
					}
				}
			}
			errch <- pt
			continue
		}
		err := downloadTweetMedia(config, path, pt.GetTweet(), config.skipLoongTweet)
		if err != nil && !utils.IsStatusCode(err, 404) && !utils.IsStatusCode(err, 403) {
			errch <- pt
		}

		if errors.Is(err, syscall.ENOSPC) {
			config.cancel(err)
		}
	}
}

func BatchDownloadTweet(ctx context.Context, client *resty.Client, skipLoongTweet bool, dwn downloader.Downloader, fileWriter downloader.FileWriter, pts ...PackagedTweet) []PackagedTweet {
	if len(pts) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancelCause(ctx)

	var errChan = make(chan PackagedTweet)
	var tweetChan = make(chan PackagedTweet, len(pts))
	var wg sync.WaitGroup
	var numRoutine = min(len(pts), MaxDownloadRoutine)

	for _, pt := range pts {
		tweetChan <- pt
	}
	close(tweetChan)

	config := workerConfig{
		ctx:            ctx,
		cancel:         cancel,
		wg:             &wg,
		skipLoongTweet: skipLoongTweet,
		downloader:     dwn,
		fileWriter:     fileWriter,
		client:         client,
	}
	for i := 0; i < numRoutine; i++ {
		wg.Add(1)
		go tweetDownloader(&config, errChan, tweetChan)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	errors := []PackagedTweet{}
	for pt := range errChan {
		errors = append(errors, pt)
	}
	return errors
}
