package downloading

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/twitter"
)

func TestNewDumper(t *testing.T) {
	dumper := NewDumper()
	if dumper == nil {
		t.Fatal("NewDumper() returned nil")
	}

	if dumper.data == nil {
		t.Error("data map should be initialized")
	}

	if dumper.set == nil {
		t.Error("set map should be initialized")
	}

	if dumper.count != 0 {
		t.Errorf("count should be 0, got %d", dumper.count)
	}
}

func TestTweetDumper_Push(t *testing.T) {
	dumper := NewDumper()

	tweets := []*twitter.Tweet{
		{Id: 1, Text: "tweet 1", CreatedAt: time.Now()},
		{Id: 2, Text: "tweet 2", CreatedAt: time.Now()},
		{Id: 3, Text: "tweet 3", CreatedAt: time.Now()},
	}

	eid := 42

	// Push tweets
	added := dumper.Push(eid, tweets...)
	if added != 3 {
		t.Errorf("Push() added = %d, want 3", added)
	}

	if dumper.count != 3 {
		t.Errorf("count = %d, want 3", dumper.count)
	}

	// Push duplicate tweets (should not increase count)
	added = dumper.Push(eid, tweets[0])
	if added != 0 {
		t.Errorf("Push() duplicate added = %d, want 0", added)
	}

	if dumper.count != 3 {
		t.Errorf("count after duplicate = %d, want 3", dumper.count)
	}

	// Push tweets with different entity ID
	eid2 := 43
	tweets2 := []*twitter.Tweet{
		{Id: 4, Text: "tweet 4", CreatedAt: time.Now()},
	}

	added = dumper.Push(eid2, tweets2...)
	if added != 1 {
		t.Errorf("Push() with different eid added = %d, want 1", added)
	}

	if dumper.count != 4 {
		t.Errorf("count = %d, want 4", dumper.count)
	}
}

func TestTweetDumper_Count(t *testing.T) {
	dumper := NewDumper()

	if dumper.Count() != 0 {
		t.Errorf("Count() = %d, want 0", dumper.Count())
	}

	dumper.Push(1, &twitter.Tweet{Id: 1, CreatedAt: time.Now()})
	dumper.Push(1, &twitter.Tweet{Id: 2, CreatedAt: time.Now()})

	if dumper.Count() != 2 {
		t.Errorf("Count() = %d, want 2", dumper.Count())
	}
}

func TestTweetDumper_Clear(t *testing.T) {
	dumper := NewDumper()

	tweets := []*twitter.Tweet{
		{Id: 1, Text: "tweet 1", CreatedAt: time.Now()},
		{Id: 2, Text: "tweet 2", CreatedAt: time.Now()},
	}

	dumper.Push(1, tweets...)

	if dumper.count != 2 {
		t.Fatalf("count before Clear() = %d, want 2", dumper.count)
	}

	dumper.Clear()

	if dumper.count != 0 {
		t.Errorf("count after Clear() = %d, want 0", dumper.count)
	}

	if len(dumper.data) != 0 {
		t.Error("data map should be empty after Clear()")
	}

	if len(dumper.set) != 0 {
		t.Error("set map should be empty after Clear()")
	}
}

func TestTweetDumper_DumpAndLoad(t *testing.T) {
	dumper := NewDumper()

	tweets := []*twitter.Tweet{
		{Id: 1, Text: "tweet 1", CreatedAt: time.Now()},
		{Id: 2, Text: "tweet 2", CreatedAt: time.Now()},
		{Id: 3, Text: "tweet 3", CreatedAt: time.Now()},
	}

	dumper.Push(42, tweets...)

	// Create temp file
	tempFile := filepath.Join(t.TempDir(), "test_dumper.json")

	// Dump
	err := dumper.Dump(tempFile)
	if err != nil {
		t.Fatalf("Dump() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatal("Dump file does not exist")
	}

	// Create new dumper and load
	newDumper := NewDumper()
	err = newDumper.Load(tempFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if newDumper.count != 3 {
		t.Errorf("count after Load() = %d, want 3", newDumper.count)
	}

	// Verify data integrity
	if len(newDumper.data[42]) != 3 {
		t.Errorf("data[42] length = %d, want 3", len(newDumper.data[42]))
	}
}

func TestTweetDumper_Load_NonExistentFile(t *testing.T) {
	dumper := NewDumper()

	nonExistentFile := filepath.Join(t.TempDir(), "non_existent.json")

	err := dumper.Load(nonExistentFile)
	if err != nil {
		t.Errorf("Load() non-existent file error = %v, want nil", err)
	}

	if dumper.count != 0 {
		t.Errorf("count = %d, want 0", dumper.count)
	}
}

func TestTweetDumper_Load_InvalidJSON(t *testing.T) {
	dumper := NewDumper()

	tempFile := filepath.Join(t.TempDir(), "invalid.json")
	err := os.WriteFile(tempFile, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	err = dumper.Load(tempFile)
	if err == nil {
		t.Error("Load() should return error for invalid JSON")
	}
}

func TestTweetDumper_Dump_FileContent(t *testing.T) {
	dumper := NewDumper()

	tweet := &twitter.Tweet{
		Id:        12345,
		Text:      "test content",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	dumper.Push(1, tweet)

	tempFile := filepath.Join(t.TempDir(), "content_test.json")
	err := dumper.Dump(tempFile)
	if err != nil {
		t.Fatalf("Dump() error = %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Verify it's valid JSON
	var data map[string][]*twitter.Tweet
	err = json.Unmarshal(content, &data)
	if err != nil {
		t.Errorf("Dumped content is not valid JSON: %v", err)
	}

	// Verify content
	if len(data["1"]) != 1 {
		t.Errorf("Expected 1 tweet, got %d", len(data["1"]))
	}

	if data["1"][0].Id != 12345 {
		t.Errorf("Expected tweet ID 12345, got %d", data["1"][0].Id)
	}
}

func TestTweetDumper_GetTotal(t *testing.T) {
	// This test requires a database connection
	// We'll test the basic structure without DB
	dumper := NewDumper()

	tweets := []*twitter.Tweet{
		{Id: 1, Text: "tweet 1", CreatedAt: time.Now()},
		{Id: 2, Text: "tweet 2", CreatedAt: time.Now()},
	}

	dumper.Push(1, tweets...)

	// GetTotal requires a database, so we just verify it doesn't panic
	// with a nil DB (it will return an error or panic depending on implementation)
	defer func() {
		if r := recover(); r != nil {
			// Expected panic with nil DB
		}
	}()

	// This will fail because we don't have a real DB, but that's expected
	_, _ = dumper.GetTotal((*sqlx.DB)(nil))
}

func TestTweetDumper_MultipleEntities(t *testing.T) {
	dumper := NewDumper()

	// Add tweets to multiple entities
	dumper.Push(1, &twitter.Tweet{Id: 1, CreatedAt: time.Now()})
	dumper.Push(1, &twitter.Tweet{Id: 2, CreatedAt: time.Now()})
	dumper.Push(2, &twitter.Tweet{Id: 3, CreatedAt: time.Now()})
	dumper.Push(3, &twitter.Tweet{Id: 4, CreatedAt: time.Now()})
	dumper.Push(3, &twitter.Tweet{Id: 5, CreatedAt: time.Now()})
	dumper.Push(3, &twitter.Tweet{Id: 6, CreatedAt: time.Now()})

	if dumper.count != 6 {
		t.Errorf("count = %d, want 6", dumper.count)
	}

	if len(dumper.data) != 3 {
		t.Errorf("number of entities = %d, want 3", len(dumper.data))
	}

	if len(dumper.data[1]) != 2 {
		t.Errorf("entity 1 tweet count = %d, want 2", len(dumper.data[1]))
	}

	if len(dumper.data[2]) != 1 {
		t.Errorf("entity 2 tweet count = %d, want 1", len(dumper.data[2]))
	}

	if len(dumper.data[3]) != 3 {
		t.Errorf("entity 3 tweet count = %d, want 3", len(dumper.data[3]))
	}
}

func TestTweetDumper_Push_ReturnValue(t *testing.T) {
	dumper := NewDumper()

	// Push single tweet
	added := dumper.Push(1, &twitter.Tweet{Id: 1, CreatedAt: time.Now()})
	if added != 1 {
		t.Errorf("Push() = %d, want 1", added)
	}

	// Push multiple tweets
	added = dumper.Push(1,
		&twitter.Tweet{Id: 2, CreatedAt: time.Now()},
		&twitter.Tweet{Id: 3, CreatedAt: time.Now()},
		&twitter.Tweet{Id: 4, CreatedAt: time.Now()},
	)
	if added != 3 {
		t.Errorf("Push() = %d, want 3", added)
	}

	// Push with duplicates
	added = dumper.Push(1,
		&twitter.Tweet{Id: 1, CreatedAt: time.Now()}, // duplicate
		&twitter.Tweet{Id: 5, CreatedAt: time.Now()}, // new
	)
	if added != 1 {
		t.Errorf("Push() with duplicate = %d, want 1", added)
	}
}
