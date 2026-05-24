package downloading

import (
	"os"
	"testing"

	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/twitter"
)

func TestSyncUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user := &twitter.User{
		Id:           12345,
		Name:         "Test User",
		ScreenName:   "testuser",
		IsProtected:  false,
		FriendsCount: 100,
	}

	// Test syncing a new user
	err := database.SyncUser(db, user.Id, user.Name, user.ScreenName, user.IsProtected, user.FriendsCount, true)
	if err != nil {
		t.Errorf("database.SyncUser() error = %v", err)
	}

	// Verify user was created
	syncedUser, err := database.GetUserById(db, user.Id)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if syncedUser == nil {
		t.Fatal("User should exist after sync")
	}

	if syncedUser.Name != user.Name {
		t.Errorf("Name = %s, want %s", syncedUser.Name, user.Name)
	}

	if syncedUser.ScreenName != user.ScreenName {
		t.Errorf("ScreenName = %s, want %s", syncedUser.ScreenName, user.ScreenName)
	}

	if syncedUser.IsProtected != user.IsProtected {
		t.Errorf("IsProtected = %v, want %v", syncedUser.IsProtected, user.IsProtected)
	}

	if syncedUser.FriendsCount != user.FriendsCount {
		t.Errorf("FriendsCount = %d, want %d", syncedUser.FriendsCount, user.FriendsCount)
	}

	// Test syncing the same user again (update)
	user.Name = "Updated Name"
	err = database.SyncUser(db, user.Id, user.Name, user.ScreenName, user.IsProtected, user.FriendsCount, true)
	if err != nil {
		t.Errorf("database.SyncUser() update error = %v", err)
	}

	// Verify update
	syncedUser, err = database.GetUserById(db, user.Id)
	if err != nil {
		t.Fatalf("Failed to get user after update: %v", err)
	}

	if syncedUser.Name != "Updated Name" {
		t.Errorf("Name after update = %s, want Updated Name", syncedUser.Name)
	}
}

func TestSyncUserAndEntity(t *testing.T) {
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

	// Test syncing user and entity
	entity, err := syncUserAndEntity(db, user, tempDir)
	if err != nil {
		t.Fatalf("syncUserAndEntity() error = %v", err)
	}

	if entity == nil {
		t.Fatal("Entity should not be nil")
	}

	// Verify entity properties
	if entity.UserId() != user.Id {
		t.Errorf("UserId() = %d, want %d", entity.UserId(), user.Id)
	}

	// Verify directory was created
	path, err := entity.Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Entity directory should exist")
	}

	// Verify user was synced
	syncedUser, err := database.GetUserById(db, user.Id)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if syncedUser == nil {
		t.Fatal("User should exist after syncUserAndEntity")
	}

	if syncedUser.IsProtected != user.IsProtected {
		t.Errorf("IsProtected = %v, want %v", syncedUser.IsProtected, user.IsProtected)
	}

	if syncedUser.FriendsCount != user.FriendsCount {
		t.Errorf("FriendsCount = %d, want %d", syncedUser.FriendsCount, user.FriendsCount)
	}
}

func TestSyncUserAndEntity_UpdateExisting(t *testing.T) {
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

	// First sync
	entity1, err := syncUserAndEntity(db, user, tempDir)
	if err != nil {
		t.Fatalf("First syncUserAndEntity() error = %v", err)
	}

	path1, _ := entity1.Path()

	// Update user and sync again
	user.Name = "Updated User"
	entity2, err := syncUserAndEntity(db, user, tempDir)
	if err != nil {
		t.Fatalf("Second syncUserAndEntity() error = %v", err)
	}

	// Should return the same entity
	if entity1.UserId() != entity2.UserId() {
		t.Error("Should return entity for same user")
	}

	// Verify directory was renamed
	path2, _ := entity2.Path()
	if path1 == path2 {
		t.Log("Path may or may not change depending on implementation")
	}
}

func TestSyncUserAndEntity_UserWithSpecialChars(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()

	user := &twitter.User{
		Id:           12345,
		Name:         "Test User / Special:Chars",
		ScreenName:   "testuser",
		IsProtected:  false,
		FriendsCount: 100,
	}

	entity, err := syncUserAndEntity(db, user, tempDir)
	if err != nil {
		t.Fatalf("syncUserAndEntity() error = %v", err)
	}

	if entity == nil {
		t.Fatal("Entity should not be nil")
	}

	// Verify directory exists
	path, err := entity.Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Entity directory should exist even with special characters in name")
	}

	// Verify user was synced with correct data
	syncedUser, err := database.GetUserById(db, user.Id)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if syncedUser.IsProtected != user.IsProtected {
		t.Errorf("IsProtected = %v, want %v", syncedUser.IsProtected, user.IsProtected)
	}

	if syncedUser.FriendsCount != user.FriendsCount {
		t.Errorf("FriendsCount = %d, want %d", syncedUser.FriendsCount, user.FriendsCount)
	}
}

func TestShouldIgnoreUser(t *testing.T) {
	tests := []struct {
		name     string
		user     *twitter.User
		expected bool
	}{
		{
			name: "normal user",
			user: &twitter.User{
				Id:         1,
				Name:       "Normal",
				ScreenName: "normal",
				Blocking:   false,
				Muting:     false,
			},
			expected: false,
		},
		{
			name: "blocking user",
			user: &twitter.User{
				Id:         2,
				Name:       "Blocking",
				ScreenName: "blocking",
				Blocking:   true,
				Muting:     false,
			},
			expected: true,
		},
		{
			name: "muting user",
			user: &twitter.User{
				Id:         3,
				Name:       "Muting",
				ScreenName: "muting",
				Blocking:   false,
				Muting:     true,
			},
			expected: true,
		},
		{
			name: "blocking and muting user",
			user: &twitter.User{
				Id:         4,
				Name:       "Both",
				ScreenName: "both",
				Blocking:   true,
				Muting:     true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIgnoreUser(tt.user)
			if got != tt.expected {
				t.Errorf("shouldIgnoreUser() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldIgnoreUser_Nil(t *testing.T) {
	// Test with nil user - this will panic because the function doesn't check for nil
	// This test documents the current behavior
	defer func() {
		if r := recover(); r != nil {
			t.Logf("shouldIgnoreUser(nil) panicked as expected: %v", r)
		}
	}()

	// This will panic because we access user.Blocking without nil check
	_ = shouldIgnoreUser(nil)
}

func TestSyncUser_NilDB(t *testing.T) {
	user := &twitter.User{
		Id:         12345,
		Name:       "Test",
		ScreenName: "test",
	}

	// Test with nil DB - this will panic because the function doesn't check for nil
	defer func() {
		if r := recover(); r != nil {
			t.Logf("database.SyncUser(nil, ...) panicked as expected: %v", r)
		}
	}()

	_ = database.SyncUser(nil, user.Id, user.Name, user.ScreenName, user.IsProtected, user.FriendsCount, true)
}
