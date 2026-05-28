package downloading

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"path/filepath"
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

func writeAuxiliaryTweetFile(fileWriter downloader.FileWriter, writeReq downloader.WriteRequest, fileType string) {
	// Auxiliary tweet metadata is best-effort: failures are logged but do not affect
	// the media download result.
	result, err := fileWriter.Write(writeReq)
	if err != nil {
		log.Warnf("failed to write %s: %v", fileType, err)
		return
	}
	if !result.Success {
		log.Warnf("%s write reported unsuccessful result: %s", fileType, writeReq.Path)
		return
	}
	if result.Versioned {
		log.Debugf("%s write created version backup: %s", fileType, writeReq.Path)
	}
}

func saveTweetJson(cfg *workerConfig, dir string, tweet *twitter.Tweet, namingObj *naming.TweetNaming) {
	if dir == "" || tweet == nil || cfg.fileWriter == nil {
		return
	}

	// Fire-and-forget by design: .json metadata should not block media downloads.
	go func() {
		defer utils.RecoverWithLog("saveTweetJson")

		loongDir := filepath.Join(dir, ".loongtweet")

		jsonPath, err := namingObj.FilePathWithResolver(loongDir, ".json", cfg.pathResolver)
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

		writeAuxiliaryTweetFile(cfg.fileWriter, writeReq, "tweet json")
	}()
}

func saveLoongTweet(cfg *workerConfig, dir string, tweet *twitter.Tweet, namingObj *naming.TweetNaming) {
	if dir == "" || tweet == nil || cfg.fileWriter == nil {
		return
	}

	// Fire-and-forget by design: .txt metadata should not block media downloads.
	go func() {
		defer utils.RecoverWithLog("saveLoongTweet")

		loongDir := filepath.Join(dir, ".loongtweet")

		txtPath, err := namingObj.FilePathWithResolver(loongDir, ".txt", cfg.pathResolver)
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
				text = html.UnescapeString(noteText)
			} else {
				text = html.UnescapeString(result.Get("legacy.full_text").String())
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

		writeAuxiliaryTweetFile(cfg.fileWriter, writeReq, "loongtweet txt")
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
								m["media_url_https"] = utils.EnsurePhotoHighQuality(rawUrl)
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

type mediaDownloadError struct {
	failedCount int
	cause       error
}

func (e *mediaDownloadError) Error() string {
	return fmt.Sprintf("%d media(s) failed to download", e.failedCount)
}

func (e *mediaDownloadError) Unwrap() error {
	return e.cause
}

func isNonRetriableMediaError(err error) bool {
	return utils.IsStatusCode(err, 403) || utils.IsStatusCode(err, 404)
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

	// tweet.Urls 最终只保留需要进入重试链路的 URL。
	urls := tweet.Urls
	tweet.Urls = tweet.Urls[:0]
	skippedUrls := make([]string, 0)
	successUrls := make([]string, 0)
	var firstRetryableErr error

	for _, u := range urls {
		ext, err := utils.GetExtFromUrl(u)
		if err != nil || ext == "" {
			ext = ".jpg"
		}

		downloadURL := utils.EnsurePhotoHighQuality(u)

		path, err := tweetNaming.FilePathWithResolver(dir, ext, cfg.pathResolver)
		if err != nil {
			log.Warnln("failed to build media path:", u, "-", err)
			tweet.Urls = append(tweet.Urls, u)
			if firstRetryableErr == nil {
				firstRetryableErr = err
			}
			continue
		}

		req := downloader.DownloadRequest{
			Context:     cfg.ctx,
			Client:      cfg.client,
			URL:         downloadURL,
			Destination: path,
			Options: downloader.DownloadOptions{
				SetModTime: &tweet.CreatedAt,
			},
		}

		result, err := cfg.downloader.Download(req)
		if err != nil {
			if isNonRetriableMediaError(err) {
				log.Infof("skip non-retriable media: %s - %v", u, err)
				skippedUrls = append(skippedUrls, u)
				continue
			}
			log.Warnln("failed to download media:", u, "-", err)
			tweet.Urls = append(tweet.Urls, u)
			if firstRetryableErr == nil {
				firstRetryableErr = err
			}
			continue
		}
		if result == nil {
			err = fmt.Errorf("download returned nil result")
			log.Warnln("media download returned nil result:", u)
			tweet.Urls = append(tweet.Urls, u)
			if firstRetryableErr == nil {
				firstRetryableErr = err
			}
			continue
		}
		if !result.Success {
			if result.Error != nil && isNonRetriableMediaError(result.Error) {
				log.Infof("skip non-retriable media: %s - %v", u, result.Error)
				skippedUrls = append(skippedUrls, u)
				continue
			}
			log.Warnln("media download reported failure:", u, "-", result.Error)
			tweet.Urls = append(tweet.Urls, u)
			if firstRetryableErr == nil {
				if result.Error != nil {
					firstRetryableErr = result.Error
				} else {
					firstRetryableErr = fmt.Errorf("media download reported unsuccessful result")
				}
			}
			continue
		}
		successUrls = append(successUrls, u)
	}

	// 只在至少一个媒体下载成功时才打印推文标题
	if len(successUrls) > 0 {
		fmt.Printf("%s", color.FgLightMagenta.Render(tweetNaming.LogFormat()))
		totalAttempted := len(successUrls) + len(tweet.Urls) + len(skippedUrls)
		if totalAttempted > len(successUrls) {
			fmt.Printf(" [%d/%d succeeded", len(successUrls), totalAttempted)
			if len(skippedUrls) > 0 {
				fmt.Printf(", %d skipped", len(skippedUrls))
			}
			fmt.Printf("]")
		}
		fmt.Println()
	}

	// 只有可重试的失败才进入后续失败链路。
	if len(tweet.Urls) > 0 {
		return &mediaDownloadError{
			failedCount: len(tweet.Urls),
			cause:       firstRetryableErr,
		}
	}
	return nil
}

func tweetDownloader(config *workerConfig, errch chan<- PackagedTweet, twech <-chan PackagedTweet) {
	var pt PackagedTweet
	var ok bool
	reportedCurrent := false

	defer config.wg.Done()
	defer func() {
		if p := recover(); p != nil {
			config.cancel(fmt.Errorf("%v", p))
			log.Errorln("✗ [downloading] - panic:", p)

			safeSend := func(pt PackagedTweet) {
				// Recover path only: if the consumer already stopped after cancellation,
				// dropping overflow items is preferable to blocking this goroutine forever.
				select {
				case errch <- pt:
				default:
				}
			}

			if pt != nil && !reportedCurrent {
				safeSend(pt)
			}
			for pt := range twech {
				safeSend(pt)
			}
		}
	}()

	for {
		select {
		case pt, ok = <-twech:
			if !ok {
				return
			}
			reportedCurrent = false
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
			reportedCurrent = true
			if config.onTweetDone != nil {
				config.onTweetDone(pt, true)
			}
			continue
		}
		err := downloadTweetMedia(config, path, pt.GetTweet(), config.skipLoongTweet)
		failed := err != nil
		if failed {
			errch <- pt
			reportedCurrent = true
		}
		if config.onTweetDone != nil {
			config.onTweetDone(pt, failed)
		}

		if errors.Is(err, syscall.ENOSPC) {
			config.cancel(err)
		}
	}
}

func BatchDownloadTweet(ctx context.Context, client *resty.Client, skipLoongTweet bool, dwn downloader.Downloader, fileWriter downloader.FileWriter, opts RuntimeOptions, onTweetDone func(pt PackagedTweet, failed bool), pts ...PackagedTweet) []PackagedTweet {
	if len(pts) == 0 {
		return nil
	}
	maxDownloadRoutine := opts.normalizedMaxDownloadRoutine()

	ctx, cancel := context.WithCancelCause(ctx)

	var errChan = make(chan PackagedTweet, len(pts))
	var tweetChan = make(chan PackagedTweet, len(pts))
	var wg sync.WaitGroup
	var numRoutine = min(len(pts), maxDownloadRoutine)

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
		onTweetDone:    onTweetDone,
		pathResolver:   utils.NewUniquePathResolver(),
	}
	for i := 0; i < numRoutine; i++ {
		wg.Add(1)
		go tweetDownloader(&config, errChan, tweetChan)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	failedTweets := []PackagedTweet{}
	for pt := range errChan {
		failedTweets = append(failedTweets, pt)
	}
	return failedTweets
}
