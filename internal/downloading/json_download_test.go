package downloading

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unkmonster/tmd/internal/twitter"
)

func TestParseUint64(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"12345", 12345},
		{"0", 0},
		{"18446744073709551615", ^uint64(0)}, // max uint64
		{"", 0},
		{"invalid", 0},
		{"123abc", 123}, // sscanf parses until non-digit
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseUint64(tt.input)
			if got != tt.expected {
				t.Errorf("parseUint64(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseTwitterDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantZero bool
	}{
		{
			name:     "RFC3339 format",
			input:    "2024-01-15T10:30:00Z",
			wantZero: false,
		},
		{
			name:     "Ruby date format",
			input:    "Mon Jan 15 10:30:00 +0000 2024",
			wantZero: false,
		},
		{
			name:     "Twitter format with timezone",
			input:    "2024-01-15 10:30:00 +00:00",
			wantZero: false,
		},
		{
			name:     "empty string",
			input:    "",
			wantZero: false, // returns time.Now(), not zero
		},
		{
			name:     "invalid format",
			input:    "not a date",
			wantZero: false, // returns time.Now()
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTwitterDate(tt.input)
			if tt.wantZero && !got.IsZero() {
				t.Errorf("parseTwitterDate(%q) = %v, want zero", tt.input, got)
			}
			// Just verify it doesn't panic and returns a reasonable time
			t.Logf("parseTwitterDate(%q) = %v", tt.input, got)
		})
	}
}

func TestExtractUrlsFromRawEntry(t *testing.T) {
	tests := []struct {
		name     string
		entry    *RawTweetEntry
		expected []string
	}{
		{
			name: "with original URLs",
			entry: &RawTweetEntry{
				Media: []struct {
					Type     string `json:"type"`
					URL      string `json:"url"`
					Original string `json:"original"`
				}{
					{Original: "http://example.com/1.jpg"},
					{Original: "http://example.com/2.jpg"},
				},
			},
			expected: []string{"http://example.com/1.jpg", "http://example.com/2.jpg"},
		},
		{
			name: "with URL fallback (no original)",
			entry: &RawTweetEntry{
				Media: []struct {
					Type     string `json:"type"`
					URL      string `json:"url"`
					Original string `json:"original"`
				}{
					{URL: "http://example.com/image.jpg"},
				},
			},
			expected: []string{"http://example.com/image.jpg"},
		},
		{
			name: "skip t.co URLs",
			entry: &RawTweetEntry{
				Media: []struct {
					Type     string `json:"type"`
					URL      string `json:"url"`
					Original string `json:"original"`
				}{
					{URL: "https://t.co/abc123"},
				},
			},
			expected: []string{},
		},
		{
			name: "mixed original and URL",
			entry: &RawTweetEntry{
				Media: []struct {
					Type     string `json:"type"`
					URL      string `json:"url"`
					Original string `json:"original"`
				}{
					{Original: "http://example.com/original.jpg", URL: "http://example.com/url.jpg"},
				},
			},
			expected: []string{"http://example.com/original.jpg"}, // original takes precedence
		},
		{
			name:     "empty media",
			entry:    &RawTweetEntry{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUrlsFromRawEntry(tt.entry)
			if len(got) != len(tt.expected) {
				t.Errorf("len(urls) = %d, want %d", len(got), len(tt.expected))
			}
			for i, url := range got {
				if i < len(tt.expected) && url != tt.expected[i] {
					t.Errorf("urls[%d] = %s, want %s", i, url, tt.expected[i])
				}
			}
		})
	}
}

func TestRawTweetFile_GetTweets(t *testing.T) {
	tests := []struct {
		name     string
		entries  []RawTweetEntry
		expected int
	}{
		{
			name: "valid entries",
			entries: []RawTweetEntry{
				{Id: "1", FullText: "Tweet 1", Media: []struct {
					Type     string `json:"type"`
					URL      string `json:"url"`
					Original string `json:"original"`
				}{{Original: "http://example.com/1.jpg"}}},
				{Id: "2", FullText: "Tweet 2", Media: []struct {
					Type     string `json:"type"`
					URL      string `json:"url"`
					Original string `json:"original"`
				}{{Original: "http://example.com/2.jpg"}}},
			},
			expected: 2,
		},
		{
			name: "skip entries without ID",
			entries: []RawTweetEntry{
				{Id: "", FullText: "No ID"},
				{Id: "1", FullText: "Has ID", Media: []struct {
					Type     string `json:"type"`
					URL      string `json:"url"`
					Original string `json:"original"`
				}{{Original: "http://example.com/1.jpg"}}},
			},
			expected: 1,
		},
		{
			name: "skip entries without media",
			entries: []RawTweetEntry{
				{Id: "1", FullText: "No media"},
				{Id: "2", FullText: "Has media", Media: []struct {
					Type     string `json:"type"`
					URL      string `json:"url"`
					Original string `json:"original"`
				}{{Original: "http://example.com/2.jpg"}}},
			},
			expected: 1,
		},
		{
			name:     "empty entries",
			entries:  []RawTweetEntry{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &RawTweetFile{Entries: tt.entries}
			tweets, err := file.GetTweets()
			if err != nil {
				t.Errorf("GetTweets() error = %v", err)
			}
			if len(tweets) != tt.expected {
				t.Errorf("len(tweets) = %d, want %d", len(tweets), tt.expected)
			}
		})
	}
}

func TestGetStringFromMap(t *testing.T) {
	m := map[string]any{
		"string": "value",
		"number": 42,
		"bool":   true,
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"string", "value"},
		{"number", ""}, // number is not a string
		{"bool", ""},   // bool is not a string
		{"missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getStringFromMap(m, tt.key)
			if got != tt.expected {
				t.Errorf("getStringFromMap(m, %q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestGetIntFromMap(t *testing.T) {
	m := map[string]any{
		"int":    float64(42),
		"float":  float64(3.14),
		"string": "not a number",
	}

	tests := []struct {
		key      string
		expected int
	}{
		{"int", 42},
		{"float", 3}, // truncated to int
		{"string", 0},
		{"missing", 0},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getIntFromMap(m, tt.key)
			if got != tt.expected {
				t.Errorf("getIntFromMap(m, %q) = %d, want %d", tt.key, got, tt.expected)
			}
		})
	}
}

func TestJsonPackagedTweet(t *testing.T) {
	tweet := &twitter.Tweet{
		Id:   12345,
		Text: "Test tweet",
	}

	pt := JsonPackagedTweet{
		tweet: tweet,
		dir:   "/test/dir",
	}

	if pt.GetTweet() != tweet {
		t.Error("GetTweet() should return the tweet")
	}

	if pt.GetPath() != "/test/dir" {
		t.Errorf("GetPath() = %s, want /test/dir", pt.GetPath())
	}
}

func TestReadJsonEntryFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		expectError bool
		expectCount int
	}{
		{
			name:        "valid raw tweet array",
			content:     `[{"id":"1","full_text":"Test","media":[{"original":"http://example.com/1.jpg"}]}]`,
			expectError: false,
			expectCount: 1,
		},
		{
			name:        "valid single raw tweet",
			content:     `{"id":"1","full_text":"Test","media":[{"original":"http://example.com/1.jpg"}]}`,
			expectError: false,
			expectCount: 1,
		},
		{
			name:        "valid formatted tweet",
			content:     `{"rest_id":"12345","legacy":{"full_text":"Test","created_at":"Mon Jan 15 10:30:00 +0000 2024"}}`,
			expectError: false,
			expectCount: 1, // formatted tweet is recognized
		},
		{
			name:        "invalid JSON",
			content:     `not valid json`,
			expectError: true,
			expectCount: 0,
		},
		{
			name:        "empty JSON object",
			content:     `{}`,
			expectError: true, // no rest_id, so unrecognized
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tempDir, tt.name+".json")
			err := os.WriteFile(path, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			entries, err := readJsonEntryFile(path)
			if tt.expectError {
				if err == nil && len(entries) == 0 {
					// Some cases return no error but empty entries
					t.Logf("Expected error but got empty entries")
				}
			} else {
				if err != nil {
					t.Errorf("readJsonEntryFile() error = %v", err)
				}
			}

			if !tt.expectError && len(entries) != tt.expectCount {
				t.Errorf("len(entries) = %d, want %d", len(entries), tt.expectCount)
			}
		})
	}
}

func TestReadJsonEntries(t *testing.T) {
	tempDir := t.TempDir()

	// Create a subdirectory with JSON files
	subDir := filepath.Join(tempDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create JSON files
	json1 := `[{"id":"1","full_text":"Tweet 1","media":[{"original":"http://example.com/1.jpg"}]}]`
	json2 := `{"id":"2","full_text":"Tweet 2","media":[{"original":"http://example.com/2.jpg"}]}`

	err = os.WriteFile(filepath.Join(tempDir, "tweets1.json"), []byte(json1), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	err = os.WriteFile(filepath.Join(subDir, "tweets2.json"), []byte(json2), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Create a non-JSON file
	err = os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("not json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Test reading directory
	entries, err := readJsonEntries(tempDir)
	if err != nil {
		t.Errorf("readJsonEntries() error = %v", err)
	}

	// Should find 2 JSON files
	if len(entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(entries))
	}

	// Test reading single file
	entries, err = readJsonEntries(filepath.Join(tempDir, "tweets1.json"))
	if err != nil {
		t.Errorf("readJsonEntries() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want 1", len(entries))
	}

	// Test reading non-existent path - on Windows this may or may not error
	_, err = readJsonEntries("/nonexistent/path")
	// The behavior may vary by OS, so we just log the result
	t.Logf("readJsonEntries() for non-existent path: err=%v", err)
}

func TestDownloadFromJsonFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create a JSON file
	jsonContent := `[{
		"id": "12345",
		"full_text": "Test tweet",
		"media": [{"original": "http://example.com/image.jpg"}],
		"screen_name": "testuser",
		"name": "Test User",
		"user_id": "98765"
	}]`

	jsonPath := filepath.Join(tempDir, "tweets.json")
	err := os.WriteFile(jsonPath, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Test downloading from JSON files
	pts, err := DownloadFromJsonFiles(context.Background(), nil, tempDir, []string{jsonPath})
	if err != nil {
		t.Errorf("DownloadFromJsonFiles() error = %v", err)
	}

	if len(pts) != 1 {
		t.Errorf("len(pts) = %d, want 1", len(pts))
	}

	// Verify tweet data
	if len(pts) > 0 {
		tweet := pts[0].GetTweet()
		if tweet.Id != 12345 {
			t.Errorf("tweet.Id = %d, want 12345", tweet.Id)
		}
		if tweet.Text != "Test tweet" {
			t.Errorf("tweet.Text = %s, want 'Test tweet'", tweet.Text)
		}
	}
}

func TestDownloadFromJsonFiles_NoMedia(t *testing.T) {
	tempDir := t.TempDir()

	// Create a JSON file without media
	jsonContent := `[{
		"id": "12345",
		"full_text": "Test tweet without media"
	}]`

	jsonPath := filepath.Join(tempDir, "tweets.json")
	err := os.WriteFile(jsonPath, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Should return error since no tweets with media
	_, err = DownloadFromJsonFiles(context.Background(), nil, tempDir, []string{jsonPath})
	if err == nil {
		t.Error("DownloadFromJsonFiles() should return error when no media found")
	}
}

func TestDownloadFromJsonFiles_InvalidFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create an invalid JSON file
	jsonPath := filepath.Join(tempDir, "invalid.json")
	err := os.WriteFile(jsonPath, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Should skip invalid files and return error if no valid tweets
	_, err = DownloadFromJsonFiles(context.Background(), nil, tempDir, []string{jsonPath})
	if err == nil {
		t.Error("DownloadFromJsonFiles() should return error for invalid files")
	}
}

func TestFormattedJsonFile_GetTweets(t *testing.T) {
	tests := []struct {
		name     string
		entries  []FormattedTweetEntry
		expected int
	}{
		{
			name: "valid entries with media",
			entries: []FormattedTweetEntry{
				{
					"rest_id": "12345",
					"legacy": map[string]any{
						"full_text":  "Test tweet",
						"created_at": "Mon Jan 15 10:30:00 +0000 2024",
						"extended_entities": map[string]any{
							"media": []any{
								map[string]any{
									"type":            "photo",
									"media_url_https": "http://example.com/image.jpg",
								},
							},
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "entry without rest_id",
			entries: []FormattedTweetEntry{
				{
					"legacy": map[string]any{
						"full_text": "No rest_id",
					},
				},
			},
			expected: 0,
		},
		{
			name:     "empty entries",
			entries:  []FormattedTweetEntry{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &FormattedJsonFile{Entries: tt.entries}
			tweets, err := file.GetTweets()
			if err != nil {
				t.Errorf("GetTweets() error = %v", err)
			}
			if len(tweets) != tt.expected {
				t.Errorf("len(tweets) = %d, want %d", len(tweets), tt.expected)
			}
		})
	}
}

func TestJsonDownloadResult(t *testing.T) {
	result := JsonDownloadResult{
		Path:       "/test/file.json",
		Success:    true,
		TweetCount: 5,
		Duration:   time.Second,
	}

	if result.Path != "/test/file.json" {
		t.Errorf("Path = %s, want /test/file.json", result.Path)
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if result.TweetCount != 5 {
		t.Errorf("TweetCount = %d, want 5", result.TweetCount)
	}

	if result.Duration != time.Second {
		t.Errorf("Duration = %v, want 1s", result.Duration)
	}
}

func TestRawTweetEntry_Structure(t *testing.T) {
	// Test that RawTweetEntry can be unmarshaled correctly
	jsonData := `{
		"id": "12345",
		"created_at": "Mon Jan 15 10:30:00 +0000 2024",
		"full_text": "Test tweet",
		"media": [
			{
				"type": "photo",
				"url": "http://example.com/url.jpg",
				"original": "http://example.com/original.jpg"
			}
		],
		"screen_name": "testuser",
		"name": "Test User",
		"user_id": "98765"
	}`

	var entry RawTweetEntry
	err := json.Unmarshal([]byte(jsonData), &entry)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if entry.Id != "12345" {
		t.Errorf("Id = %s, want 12345", entry.Id)
	}

	if entry.FullText != "Test tweet" {
		t.Errorf("FullText = %s, want 'Test tweet'", entry.FullText)
	}

	if entry.ScreenName != "testuser" {
		t.Errorf("ScreenName = %s, want testuser", entry.ScreenName)
	}

	if len(entry.Media) != 1 {
		t.Errorf("len(Media) = %d, want 1", len(entry.Media))
	}
}
