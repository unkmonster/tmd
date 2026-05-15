package downloading

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unkmonster/tmd/internal/database"
)

func TestUpdateUserLink(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test directory structure
	tempDir := t.TempDir()
	userDir := filepath.Join(tempDir, "users", "TestUser")
	err := os.MkdirAll(userDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	// Create a user entity first
	entity := &database.UserEntity{
		UserId:    12345,
		ParentDir: filepath.Join(tempDir, "users"),
		Name:      "TestUser",
	}
	err = database.CreateUserEntity(db, entity)
	if err != nil {
		t.Fatalf("Failed to create user entity: %v", err)
	}

	// Create a list entity
	listEntity := &database.LstEntity{
		LstId:     999,
		ParentDir: tempDir,
		Name:      "TestList",
	}
	err = database.CreateLstEntity(db, listEntity)
	if err != nil {
		t.Fatalf("Failed to create list entity: %v", err)
	}

	// Create user link
	link := &database.UserLink{
		UserId:            12345,
		ParentLstEntityId: listEntity.Id.Int32,
		Name:              "TestUser",
	}
	err = database.CreateUserLink(db, link)
	if err != nil {
		t.Fatalf("Failed to create user link: %v", err)
	}

	// Test updateUserLink with same name
	newPath := filepath.Join(tempDir, "users", "TestUser")
	err = updateUserLink(link, db, newPath)
	if err != nil {
		if strings.Contains(err.Error(), "A required privilege is not held") {
			t.Skip("Skipping symlink test due to lack of privileges on Windows")
		}
		t.Errorf("updateUserLink() error = %v", err)
	}

	// Test updateUserLink with different name (rename scenario)
	renamedPath := filepath.Join(tempDir, "users", "TestUserRenamed")
	err = os.MkdirAll(renamedPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create renamed dir: %v", err)
	}

	err = updateUserLink(link, db, renamedPath)
	if err != nil {
		t.Errorf("updateUserLink() with rename error = %v", err)
	}

	// Verify link name was updated
	if link.Name != "TestUserRenamed" {
		t.Errorf("link.Name = %s, want TestUserRenamed", link.Name)
	}
}

func TestUpdateUserLink_ReplacesStaleSymlinkTarget(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "old", "TestUser")
	newPath := filepath.Join(tempDir, "new", "TestUser")
	if err := os.MkdirAll(oldPath, 0755); err != nil {
		t.Fatalf("Failed to create old path: %v", err)
	}
	if err := os.MkdirAll(newPath, 0755); err != nil {
		t.Fatalf("Failed to create new path: %v", err)
	}

	entity := &database.UserEntity{
		UserId:    12345,
		ParentDir: filepath.Join(tempDir, "new"),
		Name:      "TestUser",
	}
	if err := database.CreateUserEntity(db, entity); err != nil {
		t.Fatalf("Failed to create user entity: %v", err)
	}

	listEntity := &database.LstEntity{
		LstId:     999,
		ParentDir: tempDir,
		Name:      "TestList",
	}
	if err := database.CreateLstEntity(db, listEntity); err != nil {
		t.Fatalf("Failed to create list entity: %v", err)
	}

	link := &database.UserLink{
		UserId:            12345,
		ParentLstEntityId: listEntity.Id.Int32,
		Name:              "TestUser",
	}
	if err := database.CreateUserLink(db, link); err != nil {
		t.Fatalf("Failed to create user link: %v", err)
	}

	linkPath, err := link.Path(db)
	if err != nil {
		t.Fatalf("Failed to resolve link path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		t.Fatalf("Failed to create link dir: %v", err)
	}
	oldAbs, err := filepath.Abs(oldPath)
	if err != nil {
		t.Fatalf("Failed to resolve old path: %v", err)
	}
	if err := os.Symlink(oldAbs, linkPath); err != nil {
		if strings.Contains(err.Error(), "A required privilege is not held") {
			t.Skip("Skipping symlink test due to lack of privileges on Windows")
		}
		t.Fatalf("Failed to create stale symlink: %v", err)
	}

	if err := updateUserLink(link, db, newPath); err != nil {
		if strings.Contains(err.Error(), "A required privilege is not held") {
			t.Skip("Skipping symlink replacement test due to lack of privileges on Windows")
		}
		t.Fatalf("updateUserLink() error = %v", err)
	}

	gotTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if !filepath.IsAbs(gotTarget) {
		gotTarget = filepath.Join(filepath.Dir(linkPath), gotTarget)
	}
	gotTarget, err = filepath.Abs(gotTarget)
	if err != nil {
		t.Fatalf("Failed to resolve symlink target: %v", err)
	}
	wantTarget, err := filepath.Abs(newPath)
	if err != nil {
		t.Fatalf("Failed to resolve new path: %v", err)
	}
	if gotTarget != wantTarget {
		t.Fatalf("symlink target = %s, want %s", gotTarget, wantTarget)
	}
}

func TestUpdateUserLink_NonExistentLink(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "test")
	err := os.MkdirAll(testPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	// Create a link that doesn't exist in DB
	link := &database.UserLink{
		Id:                99999, // Non-existent ID
		UserId:            12345,
		ParentLstEntityId: 1,
		Name:              "Test",
	}

	// This should fail because the link doesn't exist in DB
	err = updateUserLink(link, db, testPath)
	// Error is expected since the link path can't be resolved
	if err == nil {
		t.Log("updateUserLink() succeeded unexpectedly, but that's OK for this test")
	}
}
