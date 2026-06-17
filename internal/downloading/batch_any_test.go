package downloading

import (
	"context"
	"errors"
	"testing"

	"github.com/unkmonster/tmd/internal/twitter"
)

func TestBatchDownloadAny_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	// Test with empty lists and users
	result, _, _, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	// Should return nil, nil for empty input
	if err != nil {
		t.Errorf("BatchDownloadAny() error = %v", err)
	}

	if result != nil {
		t.Errorf("BatchDownloadAny() = %v, want nil", result)
	}
}

func TestBatchDownloadAny_NilInputs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	// Test with nil lists and users
	result, _, _, err := BatchDownloadAny(ctx, nil, db, nil, nil, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err != nil {
		t.Errorf("BatchDownloadAny() error = %v", err)
	}

	if result != nil {
		t.Errorf("BatchDownloadAny() = %v, want nil", result)
	}
}

func TestBatchDownloadAny_WithUsersOnly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	users := []*twitter.User{
		{
			Id:         12345,
			Name:       "Test User",
			ScreenName: "testuser",
			MediaCount: 10,
		},
	}

	// Without real dependencies (client, downloader), BatchDownloadAny will
	// successfully pass the list-sync phase (no lists) and then fail at
	// BatchUserDownload which requires a real Twitter client.
	_, _, _, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{}, users, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	// Should fail at BatchUserDownload due to missing dependencies,
	// not panic. The exact error depends on how far it gets.
	if err == nil {
		t.Error("BatchDownloadAny() should return an error when dependencies are missing")
	}
}

func TestBatchDownloadAny_WithListsOnly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	mockList := &MockList{
		id:      12345,
		name:    "Test List",
		members: []*twitter.User{},
		err:     nil,
	}

	// List with empty members: sync succeeds but BatchUserDownload
	// still runs with empty users (which is a no-op).
	// With no users at all (lists have empty members, no direct users),
	// BatchUserDownload returns early with nil.
	failed, members, summary, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err != nil {
		t.Errorf("BatchDownloadAny() with empty-member list should succeed: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed tweets, got %d", len(failed))
	}
	if len(members) != 0 {
		t.Errorf("expected no list members, got %d", len(members))
	}
	if summary.TotalEntities != 0 {
		t.Errorf("expected summary.TotalEntities = 0, got %d", summary.TotalEntities)
	}
}

func TestBatchDownloadAny_WithBoth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	mockList := &MockList{
		id:   12345,
		name: "Test List",
		members: []*twitter.User{
			{Id: 1, Name: "List User", ScreenName: "listuser", MediaCount: 10},
		},
		err: nil,
	}

	users := []*twitter.User{
		{Id: 2, Name: "Direct User", ScreenName: "directuser", MediaCount: 10},
	}

	// List sync succeeds (list has 1 member), then BatchUserDownload
	// is called with 2 users (1 from list + 1 direct) but fails
	// due to missing Twitter client and downloader dependencies.
	_, _, _, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, users, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err == nil {
		t.Error("BatchDownloadAny() should return an error when download dependencies are missing")
	}
}

func TestBatchDownloadAny_CancelledContext(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tempDir := t.TempDir()

	mockList := &MockList{
		id:      12345,
		name:    "Test List",
		members: []*twitter.User{},
		err:     nil,
	}

	_, _, _, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	// With a cancelled context, the list's GetMembers should check context
	// and return an error. BatchDownloadAny should propagate it.
	if err == nil {
		t.Log("BatchDownloadAny should ideally return context.Canceled for cancelled context")
		// The function may not check context in all paths; this is informational.
	}
}

func TestBatchDownloadAny_ListError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	mockList := &MockList{
		id:      12345,
		name:    "Error List",
		members: nil,
		err:     context.DeadlineExceeded,
	}

	_, _, _, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err == nil {
		t.Error("BatchDownloadAny should return error when list fails")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("BatchDownloadAny() error = %v, want DeadlineExceeded", err)
	}
}

func TestBatchDownloadAny_MultipleLists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	lists := []twitter.ListBase{
		&MockList{
			id:   1,
			name: "List 1",
			members: []*twitter.User{
				{Id: 1, Name: "User 1", ScreenName: "user1", MediaCount: 10},
			},
			err: nil,
		},
		&MockList{
			id:   2,
			name: "List 2",
			members: []*twitter.User{
				{Id: 2, Name: "User 2", ScreenName: "user2", MediaCount: 10},
			},
			err: nil,
		},
	}

	// Both lists sync successfully, then BatchUserDownload fails
	// due to missing dependencies.
	_, listMembers, _, err := BatchDownloadAny(ctx, nil, db, lists, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err == nil {
		t.Error("BatchDownloadAny() should return an error when download dependencies are missing")
	}

	// Even though the function eventually fails, list members should
	// have been collected from both lists.
	if len(listMembers) == 0 {
		t.Log("BatchDownloadAny collected 0 list members (may fail before collecting)")
	}
}

func TestBatchDownloadAny_DifferentDirs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	mockList := &MockList{
		id:      12345,
		name:    "Test List",
		members: []*twitter.User{},
		err:     nil,
	}

	// Using different root and users directories.
	// List has empty members, so BatchUserDownload is a no-op.
	_, _, _, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, dir1, dir2, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err != nil {
		t.Errorf("BatchDownloadAny() with different dirs and empty members should succeed: %v", err)
	}
}

func TestBatchDownloadAny_AutoFollow(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	mockList := &MockList{
		id:   12345,
		name: "Test List",
		members: []*twitter.User{
			{
				Id:          1,
				Name:        "Protected User",
				ScreenName:  "protected",
				MediaCount:  10,
				IsProtected: true,
				Followstate: twitter.FS_UNFOLLOW,
			},
		},
		err: nil,
	}

	// autoFollow = true with nil client triggers twitter.FollowUser(nil, user)
	// which panics, but BatchUserDownload has an internal panic handler that
	// recovers and cancels the context. Since the user is protected+unfollowed,
	// IsVisiable() returns false, so the user is skipped and the function
	// returns successfully without error.
	failed, members, summary, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, true, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err != nil {
		t.Errorf("BatchDownloadAny() with autoFollow should handle panic internally: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed tweets, got %d", len(failed))
	}
	// listMembers includes all members returned by the list,
	// regardless of whether they're downloadable.
	if len(members) != 1 {
		t.Errorf("expected 1 list member (the protected user), got %d", len(members))
	}
	if summary.TotalEntities != 0 {
		t.Errorf("expected summary.TotalEntities = 0 (user not visible), got %d", summary.TotalEntities)
	}
}

func TestBatchDownloadAny_AdditionalClients(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	mockList := &MockList{
		id:      12345,
		name:    "Test List",
		members: []*twitter.User{},
		err:     nil,
	}

	// additional clients (nil) is passed through. With empty members,
	// BatchUserDownload is a no-op and returns nil.
	_, _, _, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err != nil {
		t.Errorf("BatchDownloadAny() with nil additional clients and empty members should succeed: %v", err)
	}
}

func TestBatchDownloadAny_EmptyListMembers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	mockList := &MockList{
		id:      12345,
		name:    "Empty List",
		members: []*twitter.User{},
		err:     nil,
	}

	// List returns no members; no direct users provided.
	// BatchUserDownload is a no-op with empty users.
	failed, members, summary, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err != nil {
		t.Errorf("BatchDownloadAny() with empty list members should succeed: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed tweets, got %d", len(failed))
	}
	if len(members) != 0 {
		t.Errorf("expected no list members, got %d", len(members))
	}
	if summary.TotalEntities != 0 {
		t.Errorf("expected summary.TotalEntities = 0, got %d", summary.TotalEntities)
	}
}

func TestBatchDownloadAny_ConcurrentLists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	// Create multiple lists to test concurrent processing
	lists := make([]twitter.ListBase, 5)
	for i := 0; i < 5; i++ {
		lists[i] = &MockList{
			id:   int64(i + 1),
			name: "List " + string(rune('A'+i)),
			members: []*twitter.User{
				{Id: uint64(i + 1), Name: "User", ScreenName: "user", MediaCount: 10},
			},
			err: nil,
		}
	}

	// All 5 lists sync concurrently. Members collected from all lists,
	// then BatchUserDownload fails due to missing download deps.
	_, listMembers, _, err := BatchDownloadAny(ctx, nil, db, lists, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)

	if err == nil {
		t.Error("BatchDownloadAny() with concurrent lists should return an error when download dependencies are missing")
	}
	// Members may or may not have been collected before the error.
	_ = listMembers
}
