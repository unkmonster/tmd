package profile

import (
	"fmt"
	"testing"
	"time"

	configpkg "github.com/unkmonster/tmd/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if !config.EnableVersioning {
		t.Error("EnableVersioning should be true by default")
	}

	if !config.SkipUnchanged {
		t.Error("SkipUnchanged should be true by default")
	}

	if config.AvatarQuality != "400x400" {
		t.Errorf("AvatarQuality = %s, want 400x400", config.AvatarQuality)
	}

	if config.FileDownloadTimeout != 40*time.Second {
		t.Errorf("FileDownloadTimeout = %v, want 40s", config.FileDownloadTimeout)
	}

	if config.MaxDownloadRoutine != configpkg.DefaultMaxDownloadRoutine() {
		t.Errorf("MaxDownloadRoutine = %d, want %d", config.MaxDownloadRoutine, configpkg.DefaultMaxDownloadRoutine())
	}
}

func TestFileStatus_String(t *testing.T) {
	tests := []struct {
		status   FileStatus
		expected string
	}{
		{StatusFailed, "failed"},
		{StatusDownloaded, "downloaded"},
		{FileStatus(999), "unknown"}, // invalid status
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.expected {
				t.Errorf("FileStatus(%d).String() = %s, want %s", tt.status, got, tt.expected)
			}
		})
	}
}

func TestFileResult(t *testing.T) {
	result := FileResult{
		FileType: FileTypeAvatar,
		FilePath: "/test/avatar.jpg",
		Status:   StatusDownloaded,
		OldSize:  1000,
		NewSize:  2000,
		Error:    fmt.Errorf("test error"),
	}

	if result.FileType != FileTypeAvatar {
		t.Errorf("FileType = %s, want avatar", result.FileType)
	}

	if result.FilePath != "/test/avatar.jpg" {
		t.Errorf("FilePath = %s, want /test/avatar.jpg", result.FilePath)
	}

	if result.Status != StatusDownloaded {
		t.Errorf("Status = %v, want StatusDownloaded", result.Status)
	}

	if result.OldSize != 1000 {
		t.Errorf("OldSize = %d, want 1000", result.OldSize)
	}

	if result.NewSize != 2000 {
		t.Errorf("NewSize = %d, want 2000", result.NewSize)
	}

	if result.Error == nil || result.Error.Error() != "test error" {
		t.Errorf("Error = %v, want test error", result.Error)
	}
}

func TestDownloadResult(t *testing.T) {
	result := &DownloadResult{
		ScreenName:   "testuser",
		Success:      true,
		Files:        []FileResult{{FileType: FileTypeAvatar, FilePath: "/test/avatar.jpg"}},
		Error:        fmt.Errorf("test error"),
		DownloadTime: time.Second,
		Profile: &ProfileInfo{
			ID:         12345,
			Name:       "Test User",
			ScreenName: "testuser",
		},
	}

	if result.ScreenName != "testuser" {
		t.Errorf("ScreenName = %s, want testuser", result.ScreenName)
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if len(result.Files) != 1 {
		t.Errorf("Files length = %d, want 1", len(result.Files))
	}

	if result.Error == nil || result.Error.Error() != "test error" {
		t.Errorf("Error = %v, want test error", result.Error)
	}

	if result.DownloadTime != time.Second {
		t.Errorf("DownloadTime = %v, want 1s", result.DownloadTime)
	}

	if result.Profile == nil {
		t.Error("Profile should not be nil")
	}

	if result.Profile.ID != 12345 {
		t.Errorf("Profile.ID = %d, want 12345", result.Profile.ID)
	}
}

func TestProfileInfo(t *testing.T) {
	profile := &ProfileInfo{
		ID:          12345,
		Name:        "Test User",
		ScreenName:  "testuser",
		Description: "Test description",
		AvatarURL:   "http://example.com/avatar.jpg",
		BannerURL:   "http://example.com/banner.jpg",
		URL:         "http://example.com",
		Location:    "Test Location",
		Verified:    true,
		Protected:   false,
		CreatedAt:   "2020-01-01",
	}

	if profile.ID != 12345 {
		t.Errorf("ID = %d, want 12345", profile.ID)
	}

	if profile.Name != "Test User" {
		t.Errorf("Name = %s, want Test User", profile.Name)
	}

	if profile.ScreenName != "testuser" {
		t.Errorf("ScreenName = %s, want testuser", profile.ScreenName)
	}

	if profile.Description != "Test description" {
		t.Errorf("Description = %s, want Test description", profile.Description)
	}

	if profile.AvatarURL != "http://example.com/avatar.jpg" {
		t.Errorf("AvatarURL = %s, want http://example.com/avatar.jpg", profile.AvatarURL)
	}

	if profile.BannerURL != "http://example.com/banner.jpg" {
		t.Errorf("BannerURL = %s, want http://example.com/banner.jpg", profile.BannerURL)
	}

	if profile.URL != "http://example.com" {
		t.Errorf("URL = %s, want http://example.com", profile.URL)
	}

	if profile.Location != "Test Location" {
		t.Errorf("Location = %s, want Test Location", profile.Location)
	}

	if !profile.Verified {
		t.Error("Verified should be true")
	}

	if profile.Protected {
		t.Error("Protected should be false")
	}

	if profile.CreatedAt != "2020-01-01" {
		t.Errorf("CreatedAt = %s, want 2020-01-01", profile.CreatedAt)
	}
}

func TestFileType_Constants(t *testing.T) {
	if FileTypeAvatar != "avatar" {
		t.Errorf("FileTypeAvatar = %s, want avatar", FileTypeAvatar)
	}

	if FileTypeBanner != "banner" {
		t.Errorf("FileTypeBanner = %s, want banner", FileTypeBanner)
	}

	if FileTypeDescription != "description" {
		t.Errorf("FileTypeDescription = %s, want description", FileTypeDescription)
	}

	if FileTypeProfile != "profile" {
		t.Errorf("FileTypeProfile = %s, want profile", FileTypeProfile)
	}
}

func TestDownloadRequest(t *testing.T) {
	req := DownloadRequest{
		ScreenName:  "testuser",
		UserTitle:   "Test User(testuser)",
		Name:        "Test User",
		UserID:      12345,
		AvatarURL:   "http://example.com/avatar.jpg",
		BannerURL:   "http://example.com/banner.jpg",
		Description: "Test description",
		Location:    "Test Location",
		URL:         "http://example.com",
		Verified:    true,
		Protected:   false,
		CreatedAt:   "2020-01-01",
	}

	if req.ScreenName != "testuser" {
		t.Errorf("ScreenName = %s, want testuser", req.ScreenName)
	}

	if req.UserTitle != "Test User(testuser)" {
		t.Errorf("UserTitle = %s, want Test User(testuser)", req.UserTitle)
	}

	if req.Name != "Test User" {
		t.Errorf("Name = %s, want Test User", req.Name)
	}

	if req.UserID != 12345 {
		t.Errorf("UserID = %d, want 12345", req.UserID)
	}

	if req.AvatarURL != "http://example.com/avatar.jpg" {
		t.Errorf("AvatarURL = %s, want http://example.com/avatar.jpg", req.AvatarURL)
	}

	if req.BannerURL != "http://example.com/banner.jpg" {
		t.Errorf("BannerURL = %s, want http://example.com/banner.jpg", req.BannerURL)
	}

	if req.Description != "Test description" {
		t.Errorf("Description = %s, want Test description", req.Description)
	}

	if req.Location != "Test Location" {
		t.Errorf("Location = %s, want Test Location", req.Location)
	}

	if req.URL != "http://example.com" {
		t.Errorf("URL = %s, want http://example.com", req.URL)
	}

	if !req.Verified {
		t.Error("Verified should be true")
	}

	if req.Protected {
		t.Error("Protected should be false")
	}

	if req.CreatedAt != "2020-01-01" {
		t.Errorf("CreatedAt = %s, want 2020-01-01", req.CreatedAt)
	}
}

func TestConfig_CustomValues(t *testing.T) {
	config := &Config{
		EnableVersioning:   false,
		SkipUnchanged:      false,
		AvatarQuality:      "200x200",
		MaxDownloadRoutine: 7,
	}

	if config.EnableVersioning {
		t.Error("EnableVersioning should be false")
	}

	if config.SkipUnchanged {
		t.Error("SkipUnchanged should be false")
	}

	if config.AvatarQuality != "200x200" {
		t.Errorf("AvatarQuality = %s, want 200x200", config.AvatarQuality)
	}

	if config.MaxDownloadRoutine != 7 {
		t.Errorf("MaxDownloadRoutine = %d, want 7", config.MaxDownloadRoutine)
	}
}
