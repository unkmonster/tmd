package downloading

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/unkmonster/tmd/internal/twitter"
)

func TestMarkSingleUserWithInfo(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()

	user := &twitter.User{
		Id:           12345,
		Name:         "Test User",
		ScreenName:   "testuser",
		IsProtected:  false,
		FriendsCount: 100,
	}

	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)

	// Test marking user with timestamp
	info := markSingleUserWithInfo(db, user, tempDir, &timestamp)

	if !info.Success {
		t.Errorf("Success = false, want true. Error: %s", info.Error)
	}

	if info.UserID != user.Id {
		t.Errorf("UserID = %d, want %d", info.UserID, user.Id)
	}

	if info.ScreenName != user.ScreenName {
		t.Errorf("ScreenName = %s, want %s", info.ScreenName, user.ScreenName)
	}

	if info.EntityID == 0 {
		t.Error("EntityID should not be 0")
	}
}

func TestMarkSingleUserWithInfo_NilTimestamp(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()

	user := &twitter.User{
		Id:           12345,
		Name:         "Test User",
		ScreenName:   "testuser",
		IsProtected:  false,
		FriendsCount: 100,
	}

	// Test marking user with nil timestamp (full download)
	info := markSingleUserWithInfo(db, user, tempDir, nil)

	if !info.Success {
		t.Errorf("Success = false, want true. Error: %s", info.Error)
	}

	if info.EntityID == 0 {
		t.Error("EntityID should not be 0")
	}
}

func TestMarkSingleUserWithInfo_NilUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)

	// Test with nil user
	info := markSingleUserWithInfo(db, nil, tempDir, &timestamp)

	if info.Success {
		t.Error("Success should be false for nil user")
	}

	if info.Error != "user is nil" {
		t.Errorf("Error = %s, want 'user is nil'", info.Error)
	}
}

func TestMarkUsersAsDownloaded(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	// Create mock list
	mockList := &MockList{
		id:   12345,
		name: "Test List",
		members: []*twitter.User{
			{Id: 1, Name: "User 1", ScreenName: "user1"},
			{Id: 2, Name: "User 2", ScreenName: "user2"},
		},
	}

	// Additional users
	additionalUsers := []*twitter.User{
		{Id: 3, Name: "User 3", ScreenName: "user3"},
	}

	// Test marking users with specific timestamp
	results, err := MarkUsersAsDownloaded(ctx, nil, db, []twitter.ListBase{mockList}, additionalUsers, tempDir, "2024-01-15T10:30:00")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	// Should have 3 results (2 from list + 1 additional)
	if len(results) != 3 {
		t.Errorf("len(results) = %d, want 3", len(results))
	}

	// Verify all succeeded
	successCount := 0
	for _, info := range results {
		if info.Success {
			successCount++
		}
	}

	if successCount != 3 {
		t.Errorf("successCount = %d, want 3", successCount)
	}
}

func TestMarkUsersAsDownloaded_EmptyTimestamp(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	user := &twitter.User{
		Id:         1,
		Name:       "Test User",
		ScreenName: "testuser",
	}

	// Test with empty timestamp (should use current time)
	results, err := MarkUsersAsDownloaded(ctx, nil, db, nil, []*twitter.User{user}, tempDir, "")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}

	if !results[0].Success {
		t.Errorf("Success = false, want true. Error: %s", results[0].Error)
	}
}

func TestMarkUsersAsDownloaded_NullTimestamp(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	user := &twitter.User{
		Id:         1,
		Name:       "Test User",
		ScreenName: "testuser",
	}

	// Test with "null" timestamp (should clear latest release time)
	results, err := MarkUsersAsDownloaded(ctx, nil, db, nil, []*twitter.User{user}, tempDir, "null")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}

	if !results[0].Success {
		t.Errorf("Success = false, want true. Error: %s", results[0].Error)
	}
}

func TestMarkUsersAsDownloaded_InvalidTimestamp(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	user := &twitter.User{
		Id:         1,
		Name:       "Test User",
		ScreenName: "testuser",
	}

	// Test with invalid timestamp format
	_, err := MarkUsersAsDownloaded(ctx, nil, db, nil, []*twitter.User{user}, tempDir, "invalid-timestamp")
	if err == nil {
		t.Error("MarkUsersAsDownloaded() should return error for invalid timestamp")
	}
}

func TestMarkUsersAsDownloaded_CancelledContext(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mockList := &MockList{
		id:      12345,
		name:    "Test List",
		members: []*twitter.User{},
	}

	_, err := MarkUsersAsDownloaded(ctx, nil, db, []twitter.ListBase{mockList}, nil, tempDir, "")
	if err == nil {
		t.Error("MarkUsersAsDownloaded() with cancelled context should return error")
	}
}

func TestMarkUsersAsDownloaded_NilUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	// Test with nil users in list
	mockList := &MockList{
		id:   12345,
		name: "Test List",
		members: []*twitter.User{
			nil, // nil user
			{Id: 1, Name: "User 1", ScreenName: "user1"},
		},
	}

	results, err := MarkUsersAsDownloaded(ctx, nil, db, []twitter.ListBase{mockList}, nil, tempDir, "")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	// Should have 1 successful result (nil user is skipped)
	successCount := 0
	for _, info := range results {
		if info.Success {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("successCount = %d, want 1", successCount)
	}
}

func TestMarkUsersAsDownloaded_ListError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	// Create mock list that returns a generic error (not "not accessible")
	mockList := &MockList{
		id:   12345,
		name: "Test List",
		err:  context.DeadlineExceeded,
	}

	results, err := MarkUsersAsDownloaded(ctx, nil, db, []twitter.ListBase{mockList}, nil, tempDir, "")
	// Generic errors should not cause function to fail, just skip the list
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() should not return error for generic list error: %v", err)
	}
	// Should have 0 results since list was skipped
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestMarkUsersAsDownloaded_ListNotAccessible(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	// Create mock list that returns "does not exist or is not accessible" error
	mockList := &MockList{
		id:   12345,
		name: "Test List",
		err:  fmt.Errorf("list does not exist or is not accessible"),
	}

	// This should return an error because the list is not accessible
	_, err := MarkUsersAsDownloaded(ctx, nil, db, []twitter.ListBase{mockList}, nil, tempDir, "")
	if err == nil {
		t.Error("MarkUsersAsDownloaded() should return error for inaccessible list")
	}
	if !strings.Contains(err.Error(), "does not exist or is not accessible") {
		t.Errorf("Error message should contain 'does not exist or is not accessible', got: %v", err)
	}
}

func TestMarkSingleUserWithInfo_PanicRecovery(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// This test verifies panic recovery works
	// We can't easily trigger a panic, but we can verify the function returns properly

	tempDir := t.TempDir()
	user := &twitter.User{
		Id:         12345,
		Name:       "Test User",
		ScreenName: "testuser",
	}

	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)

	// This should not panic
	info := markSingleUserWithInfo(db, user, tempDir, &timestamp)

	// The result depends on whether the sync succeeded
	t.Logf("Result: Success=%v, Error=%s", info.Success, info.Error)
}

func TestMarkedUserInfo_Fields(t *testing.T) {
	info := MarkedUserInfo{
		UserID:     12345,
		ScreenName: "testuser",
		EntityID:   42,
		Success:    true,
	}

	if info.UserID != 12345 {
		t.Errorf("UserID = %d, want 12345", info.UserID)
	}

	if info.ScreenName != "testuser" {
		t.Errorf("ScreenName = %s, want testuser", info.ScreenName)
	}

	if info.EntityID != 42 {
		t.Errorf("EntityID = %d, want 42", info.EntityID)
	}

	if !info.Success {
		t.Error("Success should be true")
	}
}

func TestMarkUsersAsDownloaded_EmptyListsAndUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	// Test with empty lists and empty users
	results, err := MarkUsersAsDownloaded(ctx, nil, db, []twitter.ListBase{}, []*twitter.User{}, tempDir, "")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestMarkUsersAsDownloaded_NilLists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	user := &twitter.User{
		Id:         1,
		Name:       "Test User",
		ScreenName: "testuser",
	}

	// Test with nil lists (should not panic)
	results, err := MarkUsersAsDownloaded(ctx, nil, db, nil, []*twitter.User{user}, tempDir, "")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}

	if !results[0].Success {
		t.Errorf("Success = false, want true. Error: %s", results[0].Error)
	}
}

func TestMarkUsersAsDownloaded_NilListInSlice(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	// Create mock list with users
	mockList := &MockList{
		id:   12345,
		name: "Test List",
		members: []*twitter.User{
			{Id: 1, Name: "User 1", ScreenName: "user1"},
		},
	}

	// Test with nil list in slice (should skip nil list)
	results, err := MarkUsersAsDownloaded(ctx, nil, db, []twitter.ListBase{nil, mockList}, nil, tempDir, "")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	// Should have 1 result from the non-nil list
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}
}

func TestMarkUsersAsDownloaded_MultipleLists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	// Create multiple mock lists
	mockList1 := &MockList{
		id:   1,
		name: "List 1",
		members: []*twitter.User{
			{Id: 1, Name: "User 1", ScreenName: "user1"},
		},
	}
	mockList2 := &MockList{
		id:   2,
		name: "List 2",
		members: []*twitter.User{
			{Id: 2, Name: "User 2", ScreenName: "user2"},
			{Id: 3, Name: "User 3", ScreenName: "user3"},
		},
	}

	results, err := MarkUsersAsDownloaded(ctx, nil, db, []twitter.ListBase{mockList1, mockList2}, nil, tempDir, "2024-01-15T10:30:00")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	// Should have 3 results (1 from list1 + 2 from list2)
	if len(results) != 3 {
		t.Errorf("len(results) = %d, want 3", len(results))
	}

	// Verify all succeeded
	for i, info := range results {
		if !info.Success {
			t.Errorf("results[%d].Success = false, want true. Error: %s", i, info.Error)
		}
	}
}

func TestMarkSingleUserWithInfo_DuplicateUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)

	// Same user marked twice
	user := &twitter.User{
		Id:         12345,
		Name:       "Test User",
		ScreenName: "testuser",
	}

	// First mark
	info1 := markSingleUserWithInfo(db, user, tempDir, &timestamp)
	if !info1.Success {
		t.Errorf("First mark failed: %s", info1.Error)
	}

	// Second mark (should succeed, updating the same user)
	info2 := markSingleUserWithInfo(db, user, tempDir, &timestamp)
	if !info2.Success {
		t.Errorf("Second mark failed: %s", info2.Error)
	}

	// EntityID should be the same for both
	if info1.EntityID != info2.EntityID {
		t.Errorf("EntityID changed: first=%d, second=%d", info1.EntityID, info2.EntityID)
	}
}

func TestMarkUsersAsDownloaded_CaseInsensitiveNull(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	ctx := context.Background()

	user := &twitter.User{
		Id:         1,
		Name:       "Test User",
		ScreenName: "testuser",
	}

	// Test with uppercase "NULL"
	results, err := MarkUsersAsDownloaded(ctx, nil, db, nil, []*twitter.User{user}, tempDir, "NULL")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}

	if !results[0].Success {
		t.Errorf("Success = false, want true. Error: %s", results[0].Error)
	}

	// Test with mixed case "Null"
	results2, err := MarkUsersAsDownloaded(ctx, nil, db, nil, []*twitter.User{user}, tempDir, "Null")
	if err != nil {
		t.Errorf("MarkUsersAsDownloaded() error = %v", err)
	}

	if len(results2) != 1 {
		t.Errorf("len(results2) = %d, want 1", len(results2))
	}

	if !results2[0].Success {
		t.Errorf("Success = false, want true. Error: %s", results2[0].Error)
	}
}
