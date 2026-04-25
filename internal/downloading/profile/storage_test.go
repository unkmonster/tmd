package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileStorageManager(t *testing.T) {
	tempDir := t.TempDir()

	fsm, err := NewFileStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewFileStorageManager() error = %v", err)
	}

	if fsm == nil {
		t.Fatal("NewFileStorageManager() returned nil")
	}

	if fsm.usersBasePath != tempDir {
		t.Errorf("usersBasePath = %s, want %s", fsm.usersBasePath, tempDir)
	}
}

func TestNewFileStorageManager_EmptyPath(t *testing.T) {
	fsm, err := NewFileStorageManager("")
	if err == nil {
		t.Error("NewFileStorageManager(\"\") should return error")
	}

	if fsm != nil {
		t.Error("NewFileStorageManager(\"\") should return nil")
	}
}

func TestNewFileStorageManager_InvalidPath(t *testing.T) {
	// Test with a path that cannot be created
	// On Windows, use a path with invalid characters or a drive that doesn't exist
	invalidPath := "Z:\\nonexistent\\path"
	fsm, err := NewFileStorageManager(invalidPath)
	// On Windows, this may or may not error depending on the drive
	// Just log the result
	t.Logf("NewFileStorageManager() with invalid path: err=%v, fsm=%v", err, fsm)
}

func TestFileStorageManager_EnsureDirectory(t *testing.T) {
	tempDir := t.TempDir()

	fsm, err := NewFileStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewFileStorageManager() error = %v", err)
	}

	userTitle := "TestUser"
	profileDir, err := fsm.EnsureDirectory(userTitle)
	if err != nil {
		t.Errorf("EnsureDirectory() error = %v", err)
	}

	if profileDir == "" {
		t.Error("EnsureDirectory() returned empty path")
	}

	// Verify directory was created
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		t.Errorf("Directory %s should exist", profileDir)
	}

	// Verify .loongtweet/.profile subdirectory was created
	expectedSubdir := filepath.Join(tempDir, userTitle, ".loongtweet", ".profile")
	if _, err := os.Stat(expectedSubdir); os.IsNotExist(err) {
		t.Errorf("Subdirectory %s should exist", expectedSubdir)
	}

	// Verify .versions subdirectory was created
	expectedVersionsDir := filepath.Join(expectedSubdir, ".versions")
	if _, err := os.Stat(expectedVersionsDir); os.IsNotExist(err) {
		t.Errorf("Versions directory %s should exist", expectedVersionsDir)
	}
}

func TestFileStorageManager_GetFilePath(t *testing.T) {
	tempDir := t.TempDir()

	fsm, err := NewFileStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewFileStorageManager() error = %v", err)
	}

	userTitle := "TestUser"

	tests := []struct {
		fileType FileType
		expected string
	}{
		{FileTypeAvatar, "avatar.jpg"},
		{FileTypeBanner, "banner.jpg"},
		{FileTypeDescription, "description.txt"},
		{FileTypeProfile, "profile.json"},
		{FileType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.fileType), func(t *testing.T) {
			got := fsm.GetFilePath(userTitle, tt.fileType)
			expected := filepath.Join(tempDir, userTitle, ".loongtweet", ".profile", tt.expected)
			if got != expected {
				t.Errorf("GetFilePath() = %s, want %s", got, expected)
			}
		})
	}
}

func TestFileStorageManager_GetFilePathWithExt(t *testing.T) {
	tempDir := t.TempDir()

	fsm, err := NewFileStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewFileStorageManager() error = %v", err)
	}

	userTitle := "TestUser"

	tests := []struct {
		fileType FileType
		ext      string
		expected string
	}{
		{FileTypeAvatar, ".png", "avatar.png"},
		{FileTypeBanner, ".png", "banner.png"},
		{FileTypeDescription, ".txt", "description.txt"},
		{FileTypeProfile, ".json", "profile.json"},
	}

	for _, tt := range tests {
		t.Run(string(tt.fileType), func(t *testing.T) {
			got := fsm.GetFilePathWithExt(userTitle, tt.fileType, tt.ext)
			expected := filepath.Join(tempDir, userTitle, ".loongtweet", ".profile", tt.expected)
			if got != expected {
				t.Errorf("GetFilePathWithExt() = %s, want %s", got, expected)
			}
		})
	}
}

func TestFileStorageManager_SetVersionManager(t *testing.T) {
	tempDir := t.TempDir()

	fsm, err := NewFileStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewFileStorageManager() error = %v", err)
	}

	// SetVersionManager should not panic with nil
	fsm.SetVersionManager(nil)

	// In a real test, we'd create a mock VersionManager
	// but for now we just verify the method exists and doesn't panic
}

func TestFileStorageManager_getUserProfilePath(t *testing.T) {
	tempDir := t.TempDir()

	fsm, err := NewFileStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewFileStorageManager() error = %v", err)
	}

	userTitle := "TestUser"
	got := fsm.getUserProfilePath(userTitle)
	expected := filepath.Join(tempDir, userTitle, ".loongtweet", ".profile")

	if got != expected {
		t.Errorf("getUserProfilePath() = %s, want %s", got, expected)
	}
}

func TestFileStorageManager_EnsureDirectory_SpecialChars(t *testing.T) {
	tempDir := t.TempDir()

	fsm, err := NewFileStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewFileStorageManager() error = %v", err)
	}

	// Test with user title containing special characters
	// On Windows, some characters like : / \ are not allowed in file names
	userTitle := "User - Special _ Chars"
	profileDir, err := fsm.EnsureDirectory(userTitle)
	if err != nil {
		t.Errorf("EnsureDirectory() error = %v", err)
	}

	if profileDir == "" {
		t.Error("EnsureDirectory() returned empty path")
	}

	// Verify directory was created
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		t.Errorf("Directory %s should exist", profileDir)
	}
}

func TestFileStorageManager_EnsureDirectory_Existing(t *testing.T) {
	tempDir := t.TempDir()

	fsm, err := NewFileStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewFileStorageManager() error = %v", err)
	}

	userTitle := "ExistingUser"

	// Create directory first
	profileDir1, err := fsm.EnsureDirectory(userTitle)
	if err != nil {
		t.Fatalf("First EnsureDirectory() error = %v", err)
	}

	// Call again - should not error
	profileDir2, err := fsm.EnsureDirectory(userTitle)
	if err != nil {
		t.Errorf("Second EnsureDirectory() error = %v", err)
	}

	if profileDir1 != profileDir2 {
		t.Errorf("Paths differ: %s vs %s", profileDir1, profileDir2)
	}
}
