package downloading

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/naming"
	"github.com/unkmonster/tmd/internal/twitter"
)

// ThirdPartyTweetEntry 第三方工具导出的推文条目（对应 twitter-搜索结果-xxx.json 格式）
// 只包含实际需要的字段
type ThirdPartyTweetEntry struct {
	ID         string            `json:"id"`
	CreatedAt  string            `json:"created_at"`
	FullText   string            `json:"full_text"`
	ScreenName string            `json:"screen_name"`
	Name       string            `json:"name"`
	Media      []ThirdPartyMedia `json:"media"`
	Metadata   json.RawMessage   `json:"metadata"` // 原始 metadata JSON（用于保存完整推文信息）
}

// ThirdPartyMedia 第三方工具导出的媒体信息
// 只使用 Original 字段作为下载链接
type ThirdPartyMedia struct {
	Original string `json:"original"` // 原图/视频链接（唯一需要的字段）
}

// ThirdPartyTweetResult 单个 JSON 文件下载结果
type ThirdPartyTweetResult struct {
	Path       string        `json:"path"`
	TweetID    string        `json:"tweet_id"`
	ScreenName string        `json:"screen_name"`
	Success    bool          `json:"success"`
	MediaCount int           `json:"media_count"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// ParseThirdPartyTweetFile 读取并解析第三方推文 JSON 文件。
// 支持推文搜索结果格式（包含 media 数组）。
func ParseThirdPartyTweetFile(path string) ([]ThirdPartyTweetEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var entries []ThirdPartyTweetEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse JSON file %s: %w", path, err)
	}

	return entries, nil
}

// convertToTwitterTweet 将第三方推文条目转换为 twitter.Tweet，
// 使其能够通过 BatchDownloadTweet 进行统一处理（复用 -user/-jsonfolder 的下载逻辑）。
// 同时将新格式的 metadata 转换为旧格式（兼容 TMD）。
func convertToTwitterTweet(entry ThirdPartyTweetEntry) *twitter.Tweet {
	tweetID, _ := strconv.ParseUint(entry.ID, 10, 64)

	urls := make([]string, 0, len(entry.Media))
	for _, m := range entry.Media {
		if m.Original != "" {
			urls = append(urls, m.Original)
		}
	}

	// 转换 metadata 为兼容格式（新格式 → 旧格式）
	cleanedMetadata := entry.Metadata
	if converted, err := ConvertThirdPartyTweetJSON(entry.Metadata); err == nil {
		cleanedMetadata = converted
	} else {
		log.Warnf("failed to convert metadata for tweet %s, using original: %v", entry.ID, err)
	}

	return &twitter.Tweet{
		Id:   tweetID,
		Text: entry.FullText,
		Urls: urls,
		Creator: &twitter.User{
			Name:       entry.Name,
			ScreenName: entry.ScreenName,
		},
		CreatedAt: parseTwitterDate(entry.CreatedAt),
		RawJSON:   string(cleanedMetadata), // 使用转换后的 metadata
	}
}

// DownloadThirdPartyTweets 并发处理多个第三方工具导出的推文 JSON 文件，
// 通过 convertToTwitterTweet 转换后使用 BatchDownloadTweet 统一处理，
// 复用 -user/-jsonfolder 的完整下载逻辑（文件命名、并发控制、重试、txt/json保存）。
// usersDir 应该是 pathHelper.Users（即 Root/users）
func DownloadThirdPartyTweets(
	ctx context.Context,
	client *resty.Client,
	usersDir string,
	dwn downloader.Downloader,
	fileWriter downloader.FileWriter,
	filePaths ...string,
) []ThirdPartyTweetResult {
	results := make([]ThirdPartyTweetResult, 0, len(filePaths))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(fp string) {
			defer wg.Done()
			start := time.Now()

			result := ThirdPartyTweetResult{Path: fp}
			entries, err := ParseThirdPartyTweetFile(fp)
			if err != nil {
				result.Error = err.Error()
				result.Duration = time.Since(start)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			if len(entries) == 0 {
				result.Success = true
				result.Duration = time.Since(start)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			// 记录第一个推文的信息用于结果统计
			result.TweetID = entries[0].ID
			result.ScreenName = entries[0].ScreenName

			// 转换为 twitter.Tweet 并打包为 PackagedTweet
			// 每个推文有自己的用户目录，按用户分组保存
			pts := make([]PackagedTweet, 0, len(entries))
			for i := range entries {
				tweet := convertToTwitterTweet(entries[i])

				// 构建用户目录：usersDir/{ sanitizedUserName }/
				userNaming := naming.NewUserNaming(entries[i].Name, entries[i].ScreenName)
				userDir := filepath.Join(usersDir, userNaming.SanitizedTitle())
				if err := os.MkdirAll(userDir, 0755); err != nil {
					log.Warnf("failed to create user dir for tweet %s: %v", entries[i].ID, err)
				}

				pts = append(pts, JsonPackagedTweet{tweet: tweet, dir: userDir})
			}

			// 使用 BatchDownloadTweet 统一处理下载
			// skipLoongTweet=false：需要保存 txt 和 json 元数据文件
			failedTweets := BatchDownloadTweet(ctx, client, false, dwn, fileWriter, pts...)

			// 统计媒体文件总数（用于结果报告）
			totalMedia := 0
			for _, pt := range pts {
				totalMedia += len(pt.GetTweet().Urls)
			}

			if len(failedTweets) > 0 {
				result.Error = fmt.Sprintf("%d/%d tweets failed", len(failedTweets), len(pts))
			}
			result.Success = len(failedTweets) == 0
			result.MediaCount = totalMedia
			result.Duration = time.Since(start)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(filePath)
	}

	wg.Wait()
	return results
}
