package downloading

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/naming"
	"github.com/unkmonster/tmd/internal/twitter"
)

// FormattedTweetEntry TMD 生成的 loongtweet JSON 格式
type FormattedTweetEntry = map[string]any

// JsonPackagedTweet 实现了 PackagedTweet 接口
type JsonPackagedTweet struct {
	tweet *twitter.Tweet
	dir   string
}

func (pt JsonPackagedTweet) GetTweet() *twitter.Tweet {
	return pt.tweet
}

func (pt JsonPackagedTweet) GetPath() string {
	return pt.dir
}

// parseFormattedEntry 将 FormattedTweetEntry（TMD 保存的 loongtweet JSON）解析为 twitter.Tweet
// 提取推文内容、媒体链接和作者信息
func parseFormattedEntry(entry *FormattedTweetEntry) *twitter.Tweet {
	if entry == nil {
		return nil
	}

	restId := getStringFromMap(*entry, "rest_id")
	if restId == "" {
		return nil
	}

	tweet := &twitter.Tweet{
		Id: parseUint64(restId),
		RawJSON: func() string {
			if b, err := json.Marshal(entry); err == nil {
				return string(b)
			}
			return ""
		}(),
	}

	// 解析 legacy 字段：推文文本、创建时间、媒体
	if legacy, ok := (*entry)["legacy"].(map[string]any); ok {
		tweet.Text = getStringFromMap(legacy, "full_text")
		if tweet.Text == "" {
			tweet.Text = getStringFromMap(legacy, "text")
		}
		if createdAt := getStringFromMap(legacy, "created_at"); createdAt != "" {
			tweet.CreatedAt = parseTwitterDate(createdAt)
		}

		// 提取媒体链接（视频选最高码率，图片直接取链接）
		if extendedEntities, ok := legacy["extended_entities"].(map[string]any); ok {
			if mediaList, ok := extendedEntities["media"].([]any); ok {
				for _, m := range mediaList {
					if mm, ok := m.(map[string]any); ok {
						mediaType := getStringFromMap(mm, "type")
						switch mediaType {
						case "video", "animated_gif":
							// 选择最高码率的视频变体
							if variants, ok := mm["video_info"].(map[string]any); ok {
								if variantList, ok := variants["variants"].([]any); ok {
									var bestURL string
									var maxBitrate int
									for _, v := range variantList {
										if vv, ok := v.(map[string]any); ok {
											if url := getStringFromMap(vv, "url"); url != "" {
												if bitrate := getIntFromMap(vv, "bitrate"); bitrate > maxBitrate {
													maxBitrate = bitrate
													bestURL = url
												}
											}
										}
									}
									if bestURL != "" {
										tweet.Urls = append(tweet.Urls, bestURL)
									}
								}
							}
						case "photo":
							if url := getStringFromMap(mm, "media_url_https"); url != "" {
								tweet.Urls = append(tweet.Urls, url)
							}
						}
					}
				}
			}
		}
	}

	// 解析 core 字段：作者信息
	if core, ok := (*entry)["core"].(map[string]any); ok {
		if userResults, ok := core["user_results"].(map[string]any); ok {
			if result, ok := userResults["result"].(map[string]any); ok {
				tweet.Creator = &twitter.User{}

				if legacy, ok := result["legacy"].(map[string]any); ok {
					tweet.Creator.Name = html.UnescapeString(getStringFromMap(legacy, "name"))
					tweet.Creator.ScreenName = getStringFromMap(legacy, "screen_name")
				}
			}
		}
	}

	// 无媒体的推文返回 nil（跳过）
	if len(tweet.Urls) == 0 {
		return nil
	}

	return tweet
}

func getStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getIntFromMap(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func parseUint64(s string) uint64 {
	var v uint64
	fmt.Sscanf(s, "%d", &v)
	return v
}

func parseTwitterDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Now()
	}

	layouts := []string{
		"2006-01-02 15:04:05 -07:00",
		"2006-01-02 15:04:05 +08:00",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		time.RubyDate,
		"Mon Jan 02 15:04:05 -0700 2006",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t
		}
	}

	return time.Now()
}

// LoongTweetResult loongtweet 下载结果
type LoongTweetResult struct {
	Path       string        `json:"path"`
	Success    bool          `json:"success"`
	TweetCount int           `json:"tweet_count"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// ParseLoongTweetFiles 遍历 folderPath 下所有 .json 子文件（递归），
// 使用 FormattedTweetEntry 格式解析每个文件，返回有效推文列表、文件路径列表和错误。
func ParseLoongTweetFiles(folderPath string) ([]*twitter.Tweet, []string, error) {
	jsonFiles, err := collectJsonFiles(folderPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to collect json files from %s: %w", folderPath, err)
	}

	if len(jsonFiles) == 0 {
		return nil, nil, fmt.Errorf("no .json files found in %s", folderPath)
	}

	var allTweets []*twitter.Tweet
	for _, path := range jsonFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Warnf("failed to read file %s: %v", path, err)
			continue
		}

		var entry FormattedTweetEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			log.Warnf("failed to parse FormattedTweetEntry from %s: %v", path, err)
			continue
		}

		tweet := parseFormattedEntry(&entry)
		if tweet == nil {
			// 无媒体的推文被 parseFormattedEntry 返回 nil，跳过
			continue
		}

		allTweets = append(allTweets, tweet)
	}

	if len(allTweets) == 0 {
		return nil, jsonFiles, fmt.Errorf("no tweets with media found in %s", folderPath)
	}

	return allTweets, jsonFiles, nil
}

// collectJsonFiles 递归收集 folderPath 下所有 .json 文件路径。
func collectJsonFiles(folderPath string) ([]string, error) {
	info, err := os.Stat(folderPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", folderPath)
	}

	var files []string
	err = filepath.WalkDir(folderPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// DownloadFromLoongTweetFolder 从 TMD 生成的 .loongtweet 文件夹下载推文媒体。
// 并发处理多个文件夹路径，skipLoongTweet=true 不保存 .json/.txt/.profile 元数据文件。
// usersDir 应该是 pathHelper.Users（即 Root/users）
func DownloadFromLoongTweetFolder(ctx context.Context, client *resty.Client, usersDir string, dwn downloader.Downloader, fileWriter downloader.FileWriter, folderPaths ...string) []LoongTweetResult {
	results := make([]LoongTweetResult, 0, len(folderPaths))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, folderPath := range folderPaths {
		wg.Add(1)
		go func(fp string) {
			defer wg.Done()
			start := time.Now()
			result := LoongTweetResult{Path: fp}

			allTweets, _, err := ParseLoongTweetFiles(fp)
			if err != nil {
				result.Error = err.Error()
				result.Duration = time.Since(start)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			// 构建 PackagedTweet 列表，按用户分组保存
			pts := make([]JsonPackagedTweet, 0, len(allTweets))
			for _, tw := range allTweets {
				userDir := usersDir
				if tw.Creator != nil {
					// 构建用户目录：usersDir/{ sanitizedUserName }/
					userNaming := naming.NewUserNaming(tw.Creator.Name, tw.Creator.ScreenName)
					userDir = filepath.Join(usersDir, userNaming.SanitizedTitle())
					if err := os.MkdirAll(userDir, 0755); err != nil {
						log.Warnf("failed to create user dir %s: %v", userDir, err)
					}
				}
				pts = append(pts, JsonPackagedTweet{tweet: tw, dir: userDir})
			}

			result.TweetCount = len(pts)

			// 转换为 PackagedTweet 接口类型
			packged := make([]PackagedTweet, len(pts))
			for i, pt := range pts {
				packged[i] = pt
			}

			// 使用 BatchDownloadTweet 统一处理下载
			// skipLoongTweet=true：不保存 txt/json（这些文件已存在）
			failedTweets := BatchDownloadTweet(ctx, client, true, dwn, fileWriter, packged...)

			result.Success = len(failedTweets) == 0
			result.Duration = time.Since(start)

			// 输出文件夹级别的成功/失败统计
			if result.Success {
				fmt.Printf("%s ✓ %s: %d tweets processed\n", color.FgCyan.Render("[jsonfolder]"), filepath.Base(fp), result.TweetCount)
			} else {
				result.Error = fmt.Sprintf("%d/%d tweets failed", len(failedTweets), len(pts))
				fmt.Printf("%s ✗ %s: %v\n", color.FgCyan.Render("[jsonfolder]"), filepath.Base(fp), result.Error)
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(folderPath)
	}

	wg.Wait()
	return results
}
