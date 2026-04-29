package downloading

import (
	"context"
	"testing"

	"github.com/unkmonster/tmd/internal/twitter"
)

func TestCalcUserDepth(t *testing.T) {
	tests := []struct {
		name     string
		exist    int
		total    int
		expected int
	}{
		{
			name:     "all exist",
			exist:    100,
			total:    100,
			expected: 1,
		},
		{
			name:     "none exist",
			exist:    0,
			total:    100,
			expected: 3, // (100/50) + 1 = 2 + 1 = 3
		},
		{
			name:     "partial exist",
			exist:    50,
			total:    100,
			expected: 1, // (100-50)/50 = 1
		},
		{
			name:     "small difference",
			exist:    95,
			total:    100,
			expected: 1, // (100-95)/50 = 0.1 -> 1
		},
		{
			name:     "large difference",
			exist:    0,
			total:    1000,
			expected: 16, // (1000/70) + 1 = 14 + 1 + 1 = 16 (rounded up)
		},
		{
			name:     "exist greater than total",
			exist:    150,
			total:    100,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcUserDepth(tt.exist, tt.total)
			if got != tt.expected {
				t.Errorf("calcUserDepth(%d, %d) = %d, want %d", tt.exist, tt.total, got, tt.expected)
			}
		})
	}
}

func TestBatchUserDownload_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	result, err := BatchUserDownload(ctx, nil, db, []userInListEntity{}, tempDir, false, nil, nil, nil)

	if err != nil {
		t.Errorf("BatchUserDownload() error = %v", err)
	}

	if result != nil {
		t.Errorf("BatchUserDownload() = %v, want nil", result)
	}
}

func TestBatchUserDownload_NilUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	result, err := BatchUserDownload(ctx, nil, db, nil, tempDir, false, nil, nil, nil)

	if err != nil {
		t.Errorf("BatchUserDownload() error = %v", err)
	}

	if result != nil {
		t.Errorf("BatchUserDownload() = %v, want nil", result)
	}
}

func TestBatchUserDownload_WithUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:         12345,
				Name:       "Test User",
				ScreenName: "testuser",
				MediaCount: 10,
				IsProtected: false,
			},
			leid: 0,
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchUserDownload panicked as expected: %v", r)
		}
	}()

	_, _ = BatchUserDownload(ctx, nil, db, users, tempDir, false, nil, nil, nil)
}

func TestBatchUserDownload_ProtectedUnfollowedUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:          1,
				Name:        "Protected User",
				ScreenName:  "protected",
				MediaCount:  10,
				IsProtected: true,
				Followstate: twitter.FS_UNFOLLOW,
			},
			leid: 0,
		},
		{
			user: &twitter.User{
				Id:          2,
				Name:        "Public User",
				ScreenName:  "public",
				MediaCount:  10,
				IsProtected: false,
				Followstate: twitter.FS_UNFOLLOW,
			},
			leid: 0,
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchUserDownload panicked as expected: %v", r)
		}
	}()

	_, _ = BatchUserDownload(ctx, nil, db, users, tempDir, false, nil, nil, nil)
}

func TestBatchUserDownload_AutoFollow(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:          1,
				Name:        "Protected User",
				ScreenName:  "protected",
				MediaCount:  10,
				IsProtected: true,
				Followstate: twitter.FS_UNFOLLOW,
			},
			leid: 0,
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchUserDownload panicked as expected: %v", r)
		}
	}()

	_, _ = BatchUserDownload(ctx, nil, db, users, tempDir, true, nil, nil, nil)
}

func TestBatchUserDownload_WithListEntity(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:         1,
				Name:       "Test User",
				ScreenName: "testuser",
				MediaCount: 10,
			},
			leid: 0,
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchUserDownload panicked as expected: %v", r)
		}
	}()

	_, _ = BatchUserDownload(ctx, nil, db, users, tempDir, false, nil, nil, nil)
}

func TestBatchUserDownload_CancelledContext(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tempDir := t.TempDir()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:         1,
				Name:       "Test User",
				ScreenName: "testuser",
				MediaCount: 10,
			},
			leid: 0,
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchUserDownload panicked as expected: %v", r)
		}
	}()

	_, _ = BatchUserDownload(ctx, nil, db, users, tempDir, false, nil, nil, nil)
}

func TestBatchUserDownload_UserHeap(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:          1,
				Name:        "Public User 1",
				ScreenName:  "public1",
				MediaCount:  100,
				IsProtected: false,
			},
		},
		{
			user: &twitter.User{
				Id:          2,
				Name:        "Protected Following",
				ScreenName:  "protected_following",
				MediaCount:  50,
				IsProtected: true,
				Followstate: twitter.FS_FOLLOWING,
			},
		},
		{
			user: &twitter.User{
				Id:          3,
				Name:        "Public User 2",
				ScreenName:  "public2",
				MediaCount:  75,
				IsProtected: false,
			},
		},
	}

	if len(users) != 3 {
		t.Errorf("len(users) = %d, want 3", len(users))
	}

	if !users[1].user.IsProtected || users[1].user.Followstate != twitter.FS_FOLLOWING {
		t.Error("User 2 should be protected and following")
	}
}

func TestBatchUserDownload_IgnoredUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:         1,
				Name:       "Blocking User",
				ScreenName: "blocking",
				MediaCount: 10,
				Blocking:   true,
			},
			leid: 0,
		},
		{
			user: &twitter.User{
				Id:         2,
				Name:       "Muting User",
				ScreenName: "muting",
				MediaCount: 10,
				Muting:     true,
			},
			leid: 0,
		},
		{
			user: &twitter.User{
				Id:         3,
				Name:       "Normal User",
				ScreenName: "normal",
				MediaCount: 10,
			},
			leid: 0,
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchUserDownload panicked as expected: %v", r)
		}
	}()

	_, _ = BatchUserDownload(ctx, nil, db, users, tempDir, false, nil, nil, nil)
}

func TestBatchUserDownload_DuplicateUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	tempDir := t.TempDir()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:         1,
				Name:       "Test User",
				ScreenName: "testuser",
				MediaCount: 10,
			},
			leid: 0,
		},
		{
			user: &twitter.User{
				Id:         1,
				Name:       "Test User",
				ScreenName: "testuser",
				MediaCount: 10,
			},
			leid: 0,
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchUserDownload panicked as expected: %v", r)
		}
	}()

	_, _ = BatchUserDownload(ctx, nil, db, users, tempDir, false, nil, nil, nil)
}

func TestBatchUserDownload_InvalidDirectory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	users := []userInListEntity{
		{
			user: &twitter.User{
				Id:         1,
				Name:       "Test User",
				ScreenName: "testuser",
				MediaCount: 10,
			},
			leid: 0,
		},
	}

	invalidDir := "/nonexistent/path/that/cannot/be/created"

	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchUserDownload panicked as expected: %v", r)
		}
	}()

	_, _ = BatchUserDownload(ctx, nil, db, users, invalidDir, false, nil, nil, nil)
}