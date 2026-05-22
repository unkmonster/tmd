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

func TestTweetDumper_HasTweet(t *testing.T) {
	dumper := NewDumper()
	dumper.Push(1, &twitter.Tweet{Id: 10, CreatedAt: time.Now()})

	if !dumper.HasTweet(1, 10) {
		t.Fatal("HasTweet() should find pushed tweet")
	}
	if dumper.HasTweet(1, 11) {
		t.Fatal("HasTweet() should reject unknown tweet")
	}
	if dumper.HasTweet(2, 10) {
		t.Fatal("HasTweet() should reject unknown entity")
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

func TestTweetDumper_Merge(t *testing.T) {
	left := NewDumper()
	right := NewDumper()
	now := time.Now()

	left.Push(1, &twitter.Tweet{Id: 10, CreatedAt: now})
	right.Push(1, &twitter.Tweet{Id: 10, CreatedAt: now})
	right.Push(1, &twitter.Tweet{Id: 11, CreatedAt: now})
	right.Push(2, &twitter.Tweet{Id: 20, CreatedAt: now})

	left.Merge(right)

	if left.Count() != 3 {
		t.Fatalf("Merge() count = %d, want 3", left.Count())
	}
	if !left.HasTweet(1, 10) || !left.HasTweet(1, 11) || !left.HasTweet(2, 20) {
		t.Fatal("Merge() did not include expected tweets")
	}
}

func TestTweetDumper_Remove(t *testing.T) {
	dumper := NewDumper()
	now := time.Now()

	dumper.Push(1, &twitter.Tweet{Id: 10, CreatedAt: now})
	dumper.Push(1, &twitter.Tweet{Id: 11, CreatedAt: now})
	dumper.Push(2, &twitter.Tweet{Id: 20, CreatedAt: now})

	if dumper.Count() != 3 {
		t.Fatalf("count before Remove = %d, want 3", dumper.Count())
	}

	removed := dumper.Remove(1, 10)
	if !removed {
		t.Fatal("Remove() should return true for existing tweet")
	}
	if dumper.Count() != 2 {
		t.Errorf("count after Remove = %d, want 2", dumper.Count())
	}
	if dumper.HasTweet(1, 10) {
		t.Error("Remove() should delete the tweet from set")
	}
	if len(dumper.data[1]) != 1 || dumper.data[1][0].Id != 11 {
		t.Error("Remove() should preserve remaining tweets in order")
	}

	removed = dumper.Remove(1, 11)
	if !removed {
		t.Fatal("Remove() should return true for existing tweet")
	}
	if dumper.Count() != 1 {
		t.Errorf("count after removing last tweet of entity = %d, want 1", dumper.Count())
	}
	if _, ok := dumper.data[1]; ok {
		t.Error("Remove() should clean up empty entity from data map")
	}
	if _, ok := dumper.set[1]; ok {
		t.Error("Remove() should clean up empty entity from set map")
	}

	removed = dumper.Remove(999, 99)
	if removed {
		t.Error("Remove() should return false for non-existing tweet")
	}
	if dumper.Count() != 1 {
		t.Errorf("count after failed remove = %d, want 1", dumper.Count())
	}
}

func TestTweetDumper_RemovePreservesOtherEntities(t *testing.T) {
	dumper := NewDumper()
	now := time.Now()

	dumper.Push(1, &twitter.Tweet{Id: 10, CreatedAt: now})
	dumper.Push(1, &twitter.Tweet{Id: 11, CreatedAt: now})
	dumper.Push(2, &twitter.Tweet{Id: 20, CreatedAt: now})
	dumper.Push(3, &twitter.Tweet{Id: 30, CreatedAt: now})

	dumper.Remove(1, 10)

	if dumper.Count() != 3 {
		t.Errorf("count = %d, want 3", dumper.Count())
	}
	if !dumper.HasTweet(1, 11) {
		t.Error("entity 1's other tweets should be preserved")
	}
	if !dumper.HasTweet(2, 20) {
		t.Error("entity 2 should not be affected")
	}
	if !dumper.HasTweet(3, 30) {
		t.Error("entity 3 should not be affected")
	}
}

func TestJsonDumper_PushAndCount(t *testing.T) {
	d := NewJsonDumper()
	now := time.Now()

	tw1 := &twitter.Tweet{Id: 10, CreatedAt: now}
	tw2 := &twitter.Tweet{Id: 11, CreatedAt: now}

	n := d.Push("path/a.json", "file", tw1)
	if n != 1 {
		t.Errorf("first Push returned %d, want 1", n)
	}
	n = d.Push("path/a.json", "file", tw2)
	if n != 1 {
		t.Errorf("second Push returned %d, want 1", n)
	}
	n = d.Push("path/b.json", "file", tw1)
	if n != 1 {
		t.Errorf("Push to different source returned %d, want 1", n)
	}

	if d.Count() != 3 {
		t.Errorf("count = %d, want 3", d.Count())
	}
	if d.EntryCount() != 2 {
		t.Errorf("entryCount = %d, want 2", d.EntryCount())
	}
	dupN := d.Push("path/a.json", "file", tw1)
	if dupN != 0 {
		t.Errorf("duplicate Push returned %d, want 0", dupN)
	}
}

func TestJsonDumper_HasTweet(t *testing.T) {
	d := NewJsonDumper()
	now := time.Now()
	d.Push("a.json", "file", &twitter.Tweet{Id: 10, CreatedAt: now})

	if !d.HasTweet("a.json", 10) {
		t.Error("HasTweet should return true for existing tweet")
	}
	if d.HasTweet("a.json", 99) {
		t.Error("HasTweet should return false for non-existing tweet")
	}
	if d.HasTweet("b.json", 10) {
		t.Error("HasTweet should return false for non-existing source")
	}
}

func TestJsonDumper_Remove(t *testing.T) {
	d := NewJsonDumper()
	now := time.Now()

	d.Push("a.json", "file",
		&twitter.Tweet{Id: 10, CreatedAt: now},
		&twitter.Tweet{Id: 11, CreatedAt: now},
	)
	d.Push("b.json", "folder",
		&twitter.Tweet{Id: 20, CreatedAt: now},
	)

	if d.Count() != 3 {
		t.Fatalf("count before Remove = %d, want 3", d.Count())
	}

	ok := d.Remove("a.json", 10)
	if !ok {
		t.Fatal("Remove should return true for existing")
	}
	if d.Count() != 2 {
		t.Errorf("count after Remove = %d, want 2", d.Count())
	}
	if d.HasTweet("a.json", 10) {
		t.Error("tweet should be removed from set")
	}
	if !d.HasTweet("a.json", 11) {
		t.Error("other tweet in same entry should remain")
	}
	if !d.HasTweet("b.json", 20) {
		t.Error("other entry should not be affected")
	}

	ok = d.Remove("a.json", 11)
	if !ok {
		t.Fatal("Remove of last tweet in entry should succeed")
	}
	if d.EntryCount() != 1 {
		t.Errorf("entryCount after removing last tweet = %d, want 1", d.EntryCount())
	}
	if _, exists := d.data["a.json"]; exists {
		t.Error("empty entry should be cleaned from data map")
	}

	ok = d.Remove("nonexist.json", 99)
	if ok {
		t.Error("Remove of non-existing should return false")
	}
}

func TestJsonDumper_Clear(t *testing.T) {
	d := NewJsonDumper()
	now := time.Now()
	d.Push("a.json", "file", &twitter.Tweet{Id: 10, CreatedAt: now})
	d.Clear()
	if d.Count() != 0 {
		t.Errorf("count after Clear = %d, want 0", d.Count())
	}
	if d.EntryCount() != 0 {
		t.Errorf("entryCount after Clear = %d, want 0", d.EntryCount())
	}
}

func TestJsonDumper_Merge(t *testing.T) {
	a := NewJsonDumper()
	b := NewJsonDumper()
	now := time.Now()

	a.Push("a.json", "file", &twitter.Tweet{Id: 10, CreatedAt: now})
	b.Push("b.json", "folder", &twitter.Tweet{Id: 20, CreatedAt: now})
	b.Push("a.json", "file", &twitter.Tweet{Id: 11, CreatedAt: now})

	a.Merge(b)

	if a.Count() != 3 {
		t.Errorf("count after Merge = %d, want 3", a.Count())
	}
	if !a.HasTweet("a.json", 10) || !a.HasTweet("a.json", 11) || !a.HasTweet("b.json", 20) {
		t.Error("Merge did not include all tweets")
	}
}

func TestJsonDumper_DumpLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "json_errors.json")

	d := NewJsonDumper()
	now := time.Now()
	d.Push("a.json", "file",
		&twitter.Tweet{Id: 10, CreatedAt: now, Urls: []string{"http://img1.jpg"}},
		&twitter.Tweet{Id: 11, CreatedAt: now},
	)
	d.Push("b/folder", "folder", &twitter.Tweet{Id: 20, CreatedAt: now})

	if err := d.Dump(path); err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	loaded := NewJsonDumper()
	if err := loaded.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Count() != d.Count() {
		t.Errorf("loaded count = %d, want %d", loaded.Count(), d.Count())
	}
	if loaded.EntryCount() != d.EntryCount() {
		t.Errorf("loaded entryCount = %d, want %d", loaded.EntryCount(), d.EntryCount())
	}
	if !loaded.HasTweet("a.json", 10) || !loaded.HasTweet("a.json", 11) || !loaded.HasTweet("b/folder", 20) {
		t.Error("loaded data missing expected tweets")
	}
	entryA := loaded.data["a.json"]
	if entryA == nil || entryA.Type != "file" {
		t.Error("entry type not preserved")
	}
}

func TestJsonDumper_GetTotal(t *testing.T) {
	d := NewJsonDumper()
	now := time.Now()
	d.Push("a.json", "file", &twitter.Tweet{Id: 10, CreatedAt: now, Urls: []string{"url1"}})

	total := d.GetTotal()
	if len(total) != 1 {
		t.Fatalf("GetTotal returned %d items, want 1", len(total))
	}
	if total[0].GetTweet().Id != 10 {
		t.Error("GetTotal returned wrong tweet")
	}
}

func TestJsonDumper_DumpLoadRoundTrip_WithDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "json_errors.json")

	d := NewJsonDumper()
	now := time.Now()
	tweet10 := &twitter.Tweet{Id: 10, CreatedAt: now, Urls: []string{"http://img1.jpg"}}
	tweet11 := &twitter.Tweet{Id: 11, CreatedAt: now}
	tweet20 := &twitter.Tweet{Id: 20, CreatedAt: now}

	d.PushWithDir("a.json", "file", "users/user_a", tweet10)
	d.PushWithDir("a.json", "file", "users/user_a", tweet11)
	d.PushWithDir("b/folder", "folder", "users/user_b", tweet20)

	if err := d.Dump(path); err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	loaded := NewJsonDumper()
	if err := loaded.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Count() != d.Count() {
		t.Errorf("loaded count = %d, want %d", loaded.Count(), d.Count())
	}

	entryA := loaded.data["a.json"]
	if entryA == nil {
		t.Fatal("entry a.json missing after Load")
	}
	if dirA, ok := entryA.Dirs[10]; !ok || dirA != "users/user_a" {
		t.Errorf("Dirs[10] = %q, want users/user_a", dirA)
	}
	if dirA11, ok := entryA.Dirs[11]; !ok || dirA11 != "users/user_a" {
		t.Errorf("Dirs[11] = %q, want users/user_a", dirA11)
	}

	entryB := loaded.data["b/folder"]
	if entryB == nil {
		t.Fatal("entry b/folder missing after Load")
	}
	if dirB, ok := entryB.Dirs[20]; !ok || dirB != "users/user_b" {
		t.Errorf("Dirs[20] = %q, want users/user_b", dirB)
	}

	total := loaded.GetTotal()
	if len(total) != 3 {
		t.Fatalf("GetTotal returned %d items, want 3", len(total))
	}
	for _, pt := range total {
		if pt.Dir == "" {
			t.Errorf("tweet %d has empty Dir after Load", pt.GetTweet().Id)
		}
	}
}
