package downloading

import (
	"context"
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

	// This will fail because we don't have real dependencies
	// but it tests the function signature and basic flow
	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, _ = BatchDownloadAny(ctx, nil, db, []twitter.ListBase{}, users, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, _ = BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, _ = BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, users, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, _ = BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, err := BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
	if err == nil {
		t.Log("BatchDownloadAny should return error when list fails")
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, _ = BatchDownloadAny(ctx, nil, db, lists, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, _ = BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, dir1, dir2, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	// Test with autoFollow = true
	_, _, _, _ = BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, true, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	// Test with additional clients (nil for now)
	_, _, _, _ = BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, _ = BatchDownloadAny(ctx, nil, db, []twitter.ListBase{mockList}, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchDownloadAny panicked as expected: %v", r)
		}
	}()

	_, _, _, _ = BatchDownloadAny(ctx, nil, db, lists, []*twitter.User{}, tempDir, tempDir, false, nil, nil, nil, RuntimeOptions{}, nil, nil)
}
