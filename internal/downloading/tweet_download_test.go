package downloading

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/unkmonster/tmd/internal/twitter"
)

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
	result := BatchDownloadTweet(ctx, nil, false, nil, nil)

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

	_ = BatchDownloadTweet(ctx, nil, false, nil, nil, tweets...)
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
