package downloading

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

type mockTweetFileWriter struct {
	result downloader.WriteResult
	err    error
}

func (m *mockTweetFileWriter) Write(req downloader.WriteRequest) (downloader.WriteResult, error) {
	return m.result, m.err
}

type mockTweetDownloader struct {
	results map[string]mockTweetDownloadResult
}

type mockTweetDownloadResult struct {
	result *downloader.DownloadResult
	err    error
}

func (m *mockTweetDownloader) Download(req downloader.DownloadRequest) (*downloader.DownloadResult, error) {
	if m.results == nil {
		return &downloader.DownloadResult{Success: true}, nil
	}
	if result, ok := m.results[req.URL]; ok {
		return result.result, result.err
	}
	return &downloader.DownloadResult{Success: true}, nil
}

type panicTweetDownloader struct{}

func (p panicTweetDownloader) Download(req downloader.DownloadRequest) (*downloader.DownloadResult, error) {
	panic("download panic")
}

type testPackagedTweet struct {
	tweet *twitter.Tweet
	path  string
}

func (t testPackagedTweet) GetTweet() *twitter.Tweet { return t.tweet }
func (t testPackagedTweet) GetPath() string          { return t.path }

func TestCleanTweetJson(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid tweet JSON",
			input: `{
				"rest_id": "12345",
				"legacy": {
					"full_text": "Test tweet",
					"user_id_str": "98765",
					"id_str": "12345",
					"entities": {
						"media": [],
						"symbols": [],
						"urls": []
					}
				},
				"core": {
					"user_results": {
						"result": {
							"id": "user123",
							"legacy": {
								"profile_image_url_https": "http://example.com/image_normal.jpg"
							}
						}
					}
				}
			}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `not valid json`,
			wantErr: true,
		},
		{
			name:    "empty JSON",
			input:   `{}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cleanTweetJson([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("cleanTweetJson() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify result is valid JSON
				_, err := json.Marshal(result)
				if err != nil {
					t.Errorf("cleanTweetJson() result is not valid: %v", err)
				}
			}
		})
	}
}

func TestCleanMediaRecursive(t *testing.T) {
	input := map[string]any{
		"extended_entities": map[string]any{
			"media": []any{
				map[string]any{
					"type":            "photo",
					"media_url_https": "https://pbs.twimg.com/media/image.jpg",
					"media_results":   map[string]any{"some": "data"},
					"original_info": map[string]any{
						"focus_rects": []any{},
						"width":       100,
					},
					"features": map[string]any{
						"large":  map[string]any{},
						"medium": map[string]any{},
						"small":  map[string]any{},
						"orig":   map[string]any{},
					},
				},
			},
		},
	}

	cleanMediaRecursive(input)

	// Verify media_results was deleted
	media := input["extended_entities"].(map[string]any)["media"].([]any)[0].(map[string]any)
	if _, exists := media["media_results"]; exists {
		t.Error("media_results should be deleted")
	}

	// Verify focus_rects was deleted
	originalInfo := media["original_info"].(map[string]any)
	if _, exists := originalInfo["focus_rects"]; exists {
		t.Error("focus_rects should be deleted")
	}

	// Verify features were deleted
	features := media["features"].(map[string]any)
	if _, exists := features["large"]; exists {
		t.Error("large should be deleted")
	}
	if _, exists := features["medium"]; exists {
		t.Error("medium should be deleted")
	}
	if _, exists := features["small"]; exists {
		t.Error("small should be deleted")
	}
	// orig should remain
	if _, exists := features["orig"]; !exists {
		t.Error("orig should not be deleted")
	}

	// Verify photo URL was modified (only for twimg.com URLs)
	url := media["media_url_https"].(string)
	if url != "https://pbs.twimg.com/media/image.jpg?name=4096x4096" {
		t.Errorf("photo URL = %s, want https://pbs.twimg.com/media/image.jpg?name=4096x4096", url)
	}
}

func TestCleanMediaRecursive_Video(t *testing.T) {
	input := map[string]any{
		"extended_entities": map[string]any{
			"media": []any{
				map[string]any{
					"type":            "video",
					"media_url_https": "http://example.com/video.jpg",
				},
			},
		},
	}

	cleanMediaRecursive(input)

	// Video URLs should not be modified
	media := input["extended_entities"].(map[string]any)["media"].([]any)[0].(map[string]any)
	url := media["media_url_https"].(string)
	if url != "http://example.com/video.jpg" {
		t.Errorf("video URL = %s, want http://example.com/video.jpg", url)
	}
}

func TestCleanMediaRecursive_Nested(t *testing.T) {
	input := map[string]any{
		"nested": map[string]any{
			"extended_entities": map[string]any{
				"media": []any{
					map[string]any{
						"type":            "photo",
						"media_url_https": "https://pbs.twimg.com/media/nested.jpg",
					},
				},
			},
		},
	}

	cleanMediaRecursive(input)

	// Verify nested media was processed (only for twimg.com URLs)
	nested := input["nested"].(map[string]any)["extended_entities"].(map[string]any)["media"].([]any)[0].(map[string]any)
	url := nested["media_url_https"].(string)
	if url != "https://pbs.twimg.com/media/nested.jpg?name=4096x4096" {
		t.Errorf("nested URL = %s, want https://pbs.twimg.com/media/nested.jpg?name=4096x4096", url)
	}
}

func TestBatchDownloadTweet_Empty(t *testing.T) {
	ctx := context.Background()

	// Test with empty input
	result := BatchDownloadTweet(ctx, nil, false, nil, nil, nil)

	if result != nil {
		t.Errorf("BatchDownloadTweet() with empty input = %v, want nil", result)
	}
}

func TestBatchDownloadTweet_WithTweets(t *testing.T) {
	ctx := context.Background()

	tweets := []PackagedTweet{
		&TweetInEntity{
			Tweet: &twitter.Tweet{
				Id:        1,
				Text:      "Test tweet",
				Urls:      []string{"http://example.com/image.jpg"},
				CreatedAt: time.Now(),
			},
			Entity: nil,
		},
	}

	// This will fail because we don't have real downloader/fileWriter
	// but it tests the function signature and basic flow
	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadTweet panicked as expected: %v", r)
		}
	}()

	_ = BatchDownloadTweet(ctx, nil, false, nil, nil, nil, tweets...)
}

func TestBatchDownloadTweet_ReportsFailedTweet(t *testing.T) {
	ctx := context.Background()
	var called bool
	var gotFailed bool
	var gotTweetID uint64

	tweets := []PackagedTweet{
		&TweetInEntity{
			Tweet: &twitter.Tweet{
				Id: 1,
				Creator: &twitter.User{
					ScreenName: "alice",
				},
			},
			Entity: nil,
		},
	}

	result := BatchDownloadTweet(ctx, nil, false, nil, nil, func(pt PackagedTweet, failed bool) {
		called = true
		gotFailed = failed
		tweet := pt.GetTweet()
		if tweet != nil {
			gotTweetID = tweet.Id
		}
	}, tweets...)

	if len(result) != 1 {
		t.Fatalf("BatchDownloadTweet() returned %d failed tweets, want 1", len(result))
	}
	if !called {
		t.Fatal("BatchDownloadTweet() should call progress callback")
	}
	if !gotFailed {
		t.Fatal("BatchDownloadTweet() should report failed tweet when path is empty")
	}
	if gotTweetID != 1 {
		t.Fatalf("BatchDownloadTweet() reported tweet id %d, want 1", gotTweetID)
	}
}

func TestBatchDownloadTweet_WorkerPanicDoesNotDeadlock(t *testing.T) {
	originalMaxDownloadRoutine := MaxDownloadRoutine
	MaxDownloadRoutine = 4
	t.Cleanup(func() {
		MaxDownloadRoutine = originalMaxDownloadRoutine
	})

	tempDir := t.TempDir()
	tweets := make([]PackagedTweet, 8)
	for i := range tweets {
		tweetID := uint64(i + 1)
		tweets[i] = testPackagedTweet{
			tweet: &twitter.Tweet{
				Id:        tweetID,
				Text:      "tweet",
				Urls:      []string{"https://example.com/panic.jpg"},
				CreatedAt: time.Now(),
				Creator: &twitter.User{
					Name:       "alice",
					ScreenName: "alice",
				},
			},
			path: filepath.Join(tempDir, "alice"),
		}
	}

	done := make(chan []PackagedTweet, 1)
	go func() {
		done <- BatchDownloadTweet(
			context.Background(),
			nil,
			true,
			panicTweetDownloader{},
			nil,
			nil,
			tweets...,
		)
	}()

	select {
	case failedTweets := <-done:
		if len(failedTweets) != len(tweets) {
			t.Fatalf("BatchDownloadTweet() returned %d failed tweets, want %d", len(failedTweets), len(tweets))
		}
		seen := make(map[uint64]struct{}, len(failedTweets))
		for _, pt := range failedTweets {
			tweet := pt.GetTweet()
			if tweet == nil {
				t.Fatal("failed tweet should contain tweet data")
			}
			if _, ok := seen[tweet.Id]; ok {
				t.Fatalf("tweet %d reported more than once", tweet.Id)
			}
			seen[tweet.Id] = struct{}{}
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("BatchDownloadTweet deadlocked after worker panic")
	}
}

func TestWorkerConfig(t *testing.T) {
	ctx := context.Background()

	config := &workerConfig{
		ctx:            ctx,
		skipLoongTweet: true,
	}

	if config.ctx != ctx {
		t.Error("ctx should match")
	}

	if !config.skipLoongTweet {
		t.Error("skipLoongTweet should be true")
	}
}

func TestTweetDownloader_Config(t *testing.T) {
	// This test verifies the workerConfig structure
	ctx := context.Background()
	config := &workerConfig{
		ctx:            ctx,
		skipLoongTweet: false,
	}

	if config.ctx != ctx {
		t.Error("ctx should match")
	}

	if config.skipLoongTweet {
		t.Error("skipLoongTweet should be false")
	}
}

func TestDownloadTweetMedia_SkipsNonRetriableStatus(t *testing.T) {
	tempDir := t.TempDir()
	tweet := &twitter.Tweet{
		Id:        1,
		Text:      "tweet",
		Urls:      []string{"https://example.com/404.jpg"},
		CreatedAt: time.Now(),
		Creator: &twitter.User{
			Name:       "alice",
			ScreenName: "alice",
		},
	}

	cfg := &workerConfig{
		ctx: context.Background(),
		downloader: &mockTweetDownloader{results: map[string]mockTweetDownloadResult{
			"https://example.com/404.jpg": {
				result: &downloader.DownloadResult{},
				err:    &utils.HttpStatusError{Code: 404, Msg: "missing"},
			},
		}},
	}

	err := downloadTweetMedia(cfg, tempDir, tweet, true)
	if err != nil {
		t.Fatalf("403/404 应视为不进入重试链路，got err=%v", err)
	}
	if len(tweet.Urls) != 0 {
		t.Fatalf("403/404 不应保留在 tweet.Urls，got %v", tweet.Urls)
	}
}

func TestDownloadTweetMedia_RetainsRetryableUrlsOnly(t *testing.T) {
	tempDir := t.TempDir()
	retryableURL := "https://example.com/500.jpg"
	tweet := &twitter.Tweet{
		Id:        2,
		Text:      "tweet",
		Urls:      []string{"https://example.com/404.jpg", retryableURL, "https://example.com/success.jpg"},
		CreatedAt: time.Now(),
		Creator: &twitter.User{
			Name:       "bob",
			ScreenName: "bob",
		},
	}

	cfg := &workerConfig{
		ctx: context.Background(),
		downloader: &mockTweetDownloader{results: map[string]mockTweetDownloadResult{
			"https://example.com/404.jpg": {
				result: &downloader.DownloadResult{},
				err:    &utils.HttpStatusError{Code: 404, Msg: "missing"},
			},
			retryableURL: {
				result: &downloader.DownloadResult{},
				err:    &utils.HttpStatusError{Code: 500, Msg: "server error"},
			},
			"https://example.com/success.jpg": {
				result: &downloader.DownloadResult{Success: true},
			},
		}},
	}

	err := downloadTweetMedia(cfg, tempDir, tweet, true)
	if err == nil {
		t.Fatal("500 应保留为可重试失败")
	}
	if !utils.IsStatusCode(err, 500) {
		t.Fatalf("期望保留首个可重试状态码根因，got %v", err)
	}
	if len(tweet.Urls) != 1 || tweet.Urls[0] != retryableURL {
		t.Fatalf("tweet.Urls 只应保留可重试 URL，got %v", tweet.Urls)
	}
}

func TestBatchDownloadTweet_404DoesNotReturnFailedTweet(t *testing.T) {
	tempDir := t.TempDir()
	tweet := &twitter.Tweet{
		Id:        3,
		Text:      "tweet",
		Urls:      []string{"https://example.com/404.jpg"},
		CreatedAt: time.Now(),
		Creator: &twitter.User{
			Name:       "carol",
			ScreenName: "carol",
		},
	}

	var callbackCalled bool
	var gotFailed bool
	failedTweets := BatchDownloadTweet(
		context.Background(),
		nil,
		true,
		&mockTweetDownloader{results: map[string]mockTweetDownloadResult{
			"https://example.com/404.jpg": {
				result: &downloader.DownloadResult{},
				err:    &utils.HttpStatusError{Code: 404, Msg: "missing"},
			},
		}},
		nil,
		func(pt PackagedTweet, failed bool) {
			callbackCalled = true
			gotFailed = failed
		},
		testPackagedTweet{
			tweet: tweet,
			path:  filepath.Join(tempDir, "carol"),
		},
	)

	if len(failedTweets) != 0 {
		t.Fatalf("404 不应进入失败队列，got %d", len(failedTweets))
	}
	if !callbackCalled {
		t.Fatal("应触发 onTweetDone 回调")
	}
	if gotFailed {
		t.Fatal("404 不应被标记为 failed")
	}
	if len(tweet.Urls) != 0 {
		t.Fatalf("404 不应保留在 tweet.Urls，got %v", tweet.Urls)
	}
}

func TestDownloadTweetMedia_PreservesWrappedRetryableCause(t *testing.T) {
	tempDir := t.TempDir()
	tweet := &twitter.Tweet{
		Id:        4,
		Text:      "tweet",
		Urls:      []string{"https://example.com/500.jpg"},
		CreatedAt: time.Now(),
		Creator: &twitter.User{
			Name:       "dave",
			ScreenName: "dave",
		},
	}

	cfg := &workerConfig{
		ctx: context.Background(),
		downloader: &mockTweetDownloader{results: map[string]mockTweetDownloadResult{
			"https://example.com/500.jpg": {
				result: &downloader.DownloadResult{},
				err:    &utils.HttpStatusError{Code: 500, Msg: "server error"},
			},
		}},
	}

	err := downloadTweetMedia(cfg, tempDir, tweet, true)
	if err == nil {
		t.Fatal("期望返回聚合错误")
	}
	var mediaErr *mediaDownloadError
	if !errors.As(err, &mediaErr) {
		t.Fatalf("期望错误可解析为 mediaDownloadError, got %T", err)
	}
	if mediaErr.failedCount != 1 {
		t.Fatalf("failedCount = %d, want 1", mediaErr.failedCount)
	}
	if !utils.IsStatusCode(err, 500) {
		t.Fatalf("聚合错误应保留底层状态码根因, got %v", err)
	}
}
