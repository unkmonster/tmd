package downloading

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/unkmonster/tmd/internal/database"
)

func TestNewListSyncManager(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)
	if manager == nil {
		t.Fatal("NewListSyncManager() returned nil")
	}
}

func TestListSyncManager_SyncListMembers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)
	ctx := context.Background()

	// Create a list entity first
	listEntity := &database.LstEntity{
		LstId:     12345,
		ParentDir: t.TempDir(),
		Name:      "TestList",
	}
	err := database.CreateLstEntity(db, listEntity)
	if err != nil {
		t.Fatalf("Failed to create list entity: %v", err)
	}

	// Create user entities and links
	for i := 1; i <= 3; i++ {
		userEntity := &database.UserEntity{
			Uid:       uint64(i),
			ParentDir: t.TempDir(),
			Name:      "User" + string(rune('0'+i)),
		}
		err := database.CreateUserEntity(db, userEntity)
		if err != nil {
			t.Fatalf("Failed to create user entity: %v", err)
		}

		link := &database.UserLink{
			UserId:            uint64(i),
			ParentLstEntityId: listEntity.Id.Int32,
			Name:              "User" + string(rune('0'+i)),
		}
		err = database.CreateUserLink(db, link)
		if err != nil {
			t.Fatalf("Failed to create user link: %v", err)
		}
	}

	// Sync with current members (all 3)
	currentMembers := []uint64{1, 2, 3}
	err = manager.SyncListMembers(ctx, int(listEntity.Id.Int32), "TestList", currentMembers)
	if err != nil {
		t.Errorf("SyncListMembers() error = %v", err)
	}

	// Verify all links still exist
	links, err := database.GetUserLinksByLstEntityId(db, int(listEntity.Id.Int32))
	if err != nil {
		t.Fatalf("Failed to get user links: %v", err)
	}

	if len(links) != 3 {
		t.Errorf("len(links) = %d, want 3", len(links))
	}

	// Sync with fewer members (remove user 3)
	currentMembers = []uint64{1, 2}
	err = manager.SyncListMembers(ctx, int(listEntity.Id.Int32), "TestList", currentMembers)
	if err != nil {
		t.Errorf("SyncListMembers() error = %v", err)
	}

	// Verify user 3's link was removed
	links, err = database.GetUserLinksByLstEntityId(db, int(listEntity.Id.Int32))
	if err != nil {
		t.Fatalf("Failed to get user links after sync: %v", err)
	}

	if len(links) != 2 {
		t.Errorf("len(links) after removal = %d, want 2", len(links))
	}

	for _, link := range links {
		if link.UserId == 3 {
			t.Error("User 3's link should have been removed")
		}
	}
}

func TestListSyncManager_SyncListMembers_EmptyCurrent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)
	ctx := context.Background()

	// Create a list entity
	listEntity := &database.LstEntity{
		LstId:     12345,
		ParentDir: t.TempDir(),
		Name:      "TestList",
	}
	err := database.CreateLstEntity(db, listEntity)
	if err != nil {
		t.Fatalf("Failed to create list entity: %v", err)
	}

	// Create user entities and links
	for i := 1; i <= 2; i++ {
		userEntity := &database.UserEntity{
			Uid:       uint64(i),
			ParentDir: t.TempDir(),
			Name:      "User" + string(rune('0'+i)),
		}
		err := database.CreateUserEntity(db, userEntity)
		if err != nil {
			t.Fatalf("Failed to create user entity: %v", err)
		}

		link := &database.UserLink{
			UserId:            uint64(i),
			ParentLstEntityId: listEntity.Id.Int32,
			Name:              "User" + string(rune('0'+i)),
		}
		err = database.CreateUserLink(db, link)
		if err != nil {
			t.Fatalf("Failed to create user link: %v", err)
		}
	}

	// Sync with empty current members (remove all)
	err = manager.SyncListMembers(ctx, int(listEntity.Id.Int32), "TestList", []uint64{})
	if err != nil {
		t.Errorf("SyncListMembers() error = %v", err)
	}

	// Verify all links were removed
	links, err := database.GetUserLinksByLstEntityId(db, int(listEntity.Id.Int32))
	if err != nil {
		t.Fatalf("Failed to get user links: %v", err)
	}

	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}

func TestListSyncManager_SyncListMembers_NewMembers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)
	ctx := context.Background()

	// Create a list entity
	listEntity := &database.LstEntity{
		LstId:     12345,
		ParentDir: t.TempDir(),
		Name:      "TestList",
	}
	err := database.CreateLstEntity(db, listEntity)
	if err != nil {
		t.Fatalf("Failed to create list entity: %v", err)
	}

	// Sync with new members (no existing links)
	currentMembers := []uint64{1, 2, 3}
	err = manager.SyncListMembers(ctx, int(listEntity.Id.Int32), "TestList", currentMembers)
	if err != nil {
		t.Errorf("SyncListMembers() error = %v", err)
	}

	// Verify no links exist (since we didn't create any)
	links, err := database.GetUserLinksByLstEntityId(db, int(listEntity.Id.Int32))
	if err != nil {
		t.Fatalf("Failed to get user links: %v", err)
	}

	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0 (no links should be created)", len(links))
	}
}

func TestListSyncManager_SyncListMembers_CancelledContext(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := manager.SyncListMembers(ctx, 1, "TestList", []uint64{1, 2})
	if err == nil {
		t.Error("SyncListMembers() with cancelled context should return error")
	}
}

func TestListSyncManager_removeUserLinkInTx(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)

	// Create a list entity
	listEntity := &database.LstEntity{
		LstId:     12345,
		ParentDir: t.TempDir(),
		Name:      "TestList",
	}
	err := database.CreateLstEntity(db, listEntity)
	if err != nil {
		t.Fatalf("Failed to create list entity: %v", err)
	}

	// Create user entity
	userEntity := &database.UserEntity{
		Uid:       1,
		ParentDir: t.TempDir(),
		Name:      "TestUser",
	}
	err = database.CreateUserEntity(db, userEntity)
	if err != nil {
		t.Fatalf("Failed to create user entity: %v", err)
	}

	// Create a symlink for the link
	linkDir := filepath.Join(listEntity.ParentDir, "TestList")
	err = os.MkdirAll(linkDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create link dir: %v", err)
	}

	linkPath := filepath.Join(linkDir, "TestUser")
	err = os.Symlink(userEntity.ParentDir, linkPath)
	if err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create user link in DB
	link := &database.UserLink{
		Id:                1,
		UserId:            1,
		ParentLstEntityId: listEntity.Id.Int32,
		Name:              "TestUser",
	}
	err = database.CreateUserLink(db, link)
	if err != nil {
		t.Fatalf("Failed to create user link: %v", err)
	}

	// Start transaction
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Remove the link
	err = manager.removeUserLinkInTx(tx, link, int(listEntity.Id.Int32))
	if err != nil {
		t.Errorf("removeUserLinkInTx() error = %v", err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify symlink was removed
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("Symlink should have been removed")
	}

	// Verify link was removed from DB
	links, err := database.GetUserLinksByLstEntityId(db, int(listEntity.Id.Int32))
	if err != nil {
		t.Fatalf("Failed to get user links: %v", err)
	}

	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}

func TestListSyncManager_removeUserLinkInTx_InvalidId(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)

	// Start transaction
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Try to remove link with invalid ID
	link := &database.UserLink{
		Id:                0, // Invalid ID
		UserId:            1,
		ParentLstEntityId: 1,
		Name:              "Test",
	}

	err = manager.removeUserLinkInTx(tx, link, 1)
	if err == nil {
		t.Error("removeUserLinkInTx() with invalid ID should return error")
	}
}

func TestListSyncManager_removeUserLinkInTx_NonExistentSymlink(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)

	// Create a list entity
	listEntity := &database.LstEntity{
		LstId:     12345,
		ParentDir: t.TempDir(),
		Name:      "TestList",
	}
	err := database.CreateLstEntity(db, listEntity)
	if err != nil {
		t.Fatalf("Failed to create list entity: %v", err)
	}

	// Create user link in DB (no actual symlink)
	link := &database.UserLink{
		Id:                1,
		UserId:            1,
		ParentLstEntityId: listEntity.Id.Int32,
		Name:              "TestUser",
	}
	err = database.CreateUserLink(db, link)
	if err != nil {
		t.Fatalf("Failed to create user link: %v", err)
	}

	// Start transaction
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Remove the link (symlink doesn't exist, should not error)
	err = manager.removeUserLinkInTx(tx, link, int(listEntity.Id.Int32))
	if err != nil {
		t.Errorf("removeUserLinkInTx() error = %v (non-existent symlink should be OK)", err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}
}

func TestListSyncManager_ConcurrentAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewListSyncManager(db)
	ctx := context.Background()

	// Create a list entity
	listEntity := &database.LstEntity{
		LstId:     12345,
		ParentDir: t.TempDir(),
		Name:      "TestList",
	}
	err := database.CreateLstEntity(db, listEntity)
	if err != nil {
		t.Fatalf("Failed to create list entity: %v", err)
	}

	// Run multiple syncs concurrently
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func(idx int) {
			members := []uint64{uint64(idx + 1), uint64(idx + 2)}
			err := manager.SyncListMembers(ctx, int(listEntity.Id.Int32), "TestList", members)
			if err != nil {
				t.Logf("SyncListMembers error: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify no deadlock occurred and DB is still accessible
	links, err := database.GetUserLinksByLstEntityId(db, int(listEntity.Id.Int32))
	if err != nil {
		t.Fatalf("Failed to get user links after concurrent access: %v", err)
	}

	// Result may vary due to race conditions, but shouldn't crash
	t.Logf("Links after concurrent syncs: %d", len(links))
}
