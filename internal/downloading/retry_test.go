package downloading

import (
	"context"
	"testing"
	"time"

	"github.com/unkmonster/tmd/internal/twitter"
)

func TestCountTotalUrls(t *testing.T) {
	tests := []struct {
		name     string
		tweets   []PackagedTweet
		expected int
	}{
		{
			name:     "empty list",
			tweets:   []PackagedTweet{},
			expected: 0,
		},
		{
			name: "single tweet with no URLs",
			tweets: []PackagedTweet{
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 1, Urls: []string{}}},
			},
			expected: 0,
		},
		{
			name: "single tweet with URLs",
			tweets: []PackagedTweet{
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 1, Urls: []string{"url1", "url2", "url3"}}},
			},
			expected: 3,
		},
		{
			name: "multiple tweets with URLs",
			tweets: []PackagedTweet{
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 1, Urls: []string{"url1", "url2"}}},
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 2, Urls: []string{"url3"}}},
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 3, Urls: []string{"url4", "url5", "url6"}}},
			},
			expected: 6,
		},
		{
			name: "mixed tweets with and without URLs",
			tweets: []PackagedTweet{
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 1, Urls: []string{"url1"}}},
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 2, Urls: []string{}}},
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 3, Urls: []string{"url2", "url3"}}},
				&TweetInEntity{Tweet: &twitter.Tweet{Id: 4, Urls: []string{}}},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countTotalUrls(tt.tweets)
			if got != tt.expected {
				t.Errorf("countTotalUrls() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestCountTotalUrls_NilTweets(t *testing.T) {
	// Test with tweets that have nil Urls (should handle gracefully)
	tweets := []PackagedTweet{
		&TweetInEntity{Tweet: &twitter.Tweet{Id: 1, Urls: nil}},
		&TweetInEntity{Tweet: &twitter.Tweet{Id: 2, Urls: []string{"url1"}}},
	}

	got := countTotalUrls(tweets)
	if got != 1 {
		t.Errorf("countTotalUrls() = %d, want 1", got)
	}
}

func TestRetryFailedTweets_EmptyDumper(t *testing.T) {
	ctx := context.Background()
	dumper := NewDumper()

	// Test with empty dumper - should return nil immediately
	// We can't fully test this without a database and other dependencies,
	// but we can verify it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RetryFailedTweets panicked with empty dumper: %v", r)
		}
	}()

	// Empty dumper should return immediately and never touch dependencies.
	_, _ = RetryFailedTweets(ctx, dumper, nil, nil, nil, nil, nil)
}

func TestRetryFailedTweets_EmptyDumperWithProgress(t *testing.T) {
	ctx := context.Background()
	dumper := NewDumper()
	called := false

	summary, err := RetryFailedTweets(ctx, dumper, nil, nil, nil, nil, func(progress RetryProgress) {
		called = true
	})

	if err != nil {
		t.Fatalf("RetryFailedTweets() error = %v", err)
	}
	if called {
		t.Fatal("RetryFailedTweets() should not call progress for empty dumper")
	}
	if summary.TotalEntities != 0 || summary.RemainingEntities != 0 {
		t.Fatalf("RetryFailedTweets() summary = %+v, want zero summary", summary)
	}
}

func TestRetryFailedTweets_WithTweets(t *testing.T) {
	dumper := NewDumper()

	// Add some tweets with URLs to retry
	tweets := []*twitter.Tweet{
		{Id: 1, Urls: []string{"http://example.com/1.jpg"}, CreatedAt: time.Now()},
		{Id: 2, Urls: []string{"http://example.com/2.jpg"}, CreatedAt: time.Now()},
	}

	dumper.Push(1, tweets...)

	// Verify dumper has tweets
	if dumper.Count() != 2 {
		t.Fatalf("dumper.Count() = %d, want 2", dumper.Count())
	}

	// We can't fully test the retry logic without proper dependencies,
	// but we verified the setup is correct
	t.Log("Dumper setup correctly with tweets")
}

func TestCountTotalUrls_LargeNumber(t *testing.T) {
	// Test with a larger number of tweets and URLs
	tweets := make([]PackagedTweet, 100)
	expectedCount := 0

	for i := 0; i < 100; i++ {
		urlCount := i % 5 // 0-4 URLs per tweet
		urls := make([]string, urlCount)
		for j := 0; j < urlCount; j++ {
			urls[j] = "http://example.com/" + string(rune('a'+i)) + ".jpg"
		}
		tweets[i] = &TweetInEntity{Tweet: &twitter.Tweet{Id: uint64(i), Urls: urls}}
		expectedCount += urlCount
	}

	got := countTotalUrls(tweets)
	if got != expectedCount {
		t.Errorf("countTotalUrls() = %d, want %d", got, expectedCount)
	}
}
