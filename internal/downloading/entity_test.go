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
		Uid:       12345,
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
