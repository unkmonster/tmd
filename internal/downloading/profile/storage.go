package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/unkmonster/tmd/internal/downloader"
)

type FileStorageManager struct {
	usersBasePath  string
	versionManager downloader.VersionManager
}

func NewFileStorageManager(usersBasePath string) (*FileStorageManager, error) {
	if usersBasePath == "" {
		return nil, fmt.Errorf("usersBasePath cannot be empty")
	}

	if err := os.MkdirAll(usersBasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create users base directory: %w", err)
	}

	return &FileStorageManager{usersBasePath: usersBasePath}, nil
}

func (fsm *FileStorageManager) SetVersionManager(vm downloader.VersionManager) {
	fsm.versionManager = vm
}

func (fsm *FileStorageManager) getUserProfilePath(userTitle string) string {
	// userTitle 已经在外部清理过，直接使用
	return filepath.Join(fsm.usersBasePath, userTitle, profileDirName, profileSubDirName)
}

func (fsm *FileStorageManager) EnsureDirectory(userTitle string) (string, error) {
	userDir := filepath.Join(fsm.usersBasePath, userTitle)
	return ensureProfileDirs(userDir)
}

func (fsm *FileStorageManager) GetFilePath(userTitle string, fileType FileType) string {
	profilePath := fsm.getUserProfilePath(userTitle)

	switch fileType {
	case FileTypeAvatar:
		return filepath.Join(profilePath, "avatar.jpg")
	case FileTypeBanner:
		return filepath.Join(profilePath, "banner.jpg")
	case FileTypeDescription:
		return filepath.Join(profilePath, "description.txt")
	case FileTypeProfile:
		return filepath.Join(profilePath, "profile.json")
	default:
		return filepath.Join(profilePath, string(fileType))
	}
}

func (fsm *FileStorageManager) GetFilePathWithExt(userTitle string, fileType FileType, ext string) string {
	profilePath := fsm.getUserProfilePath(userTitle)

	switch fileType {
	case FileTypeAvatar:
		filename := "avatar" + ext
		return filepath.Join(profilePath, filename)
	case FileTypeBanner:
		filename := "banner" + ext
		return filepath.Join(profilePath, filename)
	default:
		return fsm.GetFilePath(userTitle, fileType)
	}
}
