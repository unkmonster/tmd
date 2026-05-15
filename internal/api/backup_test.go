package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBackupWritesToSharedBackupDir(t *testing.T) {
	root := t.TempDir()
	confPath := filepath.Join(root, "conf.yaml")
	cookiesPath := filepath.Join(root, "additional_cookies.yaml")
	require.NoError(t, os.WriteFile(confPath, []byte("root_path: old"), 0600))
	require.NoError(t, os.WriteFile(cookiesPath, []byte("- auth_token: old"), 0600))

	confBackup, err := createBackup(confPath)
	require.NoError(t, err)
	cookiesBackup, err := createBackup(cookiesPath)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(confBackup, backupDirName+string(filepath.Separator)))
	assert.True(t, strings.HasPrefix(cookiesBackup, backupDirName+string(filepath.Separator)))
	assert.FileExists(t, filepath.Join(root, confBackup))
	assert.FileExists(t, filepath.Join(root, cookiesBackup))
}

func TestCreateBackupKeepsLatestTenInSharedBackupDir(t *testing.T) {
	root := t.TempDir()
	confPath := filepath.Join(root, "conf.yaml")
	cookiesPath := filepath.Join(root, "additional_cookies.yaml")

	for i := 0; i < maxConfigBackupCount+3; i++ {
		require.NoError(t, os.WriteFile(confPath, []byte{byte(i)}, 0600))
		_, err := createBackup(confPath)
		require.NoError(t, err)
	}
	for i := 0; i < 2; i++ {
		require.NoError(t, os.WriteFile(cookiesPath, []byte{byte(i)}, 0600))
		_, err := createBackup(cookiesPath)
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(filepath.Join(root, backupDirName))
	require.NoError(t, err)

	backups := 0
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".backup.") {
			backups++
		}
	}

	assert.Equal(t, maxConfigBackupCount, backups)
}

func TestCreateBackupMissingSourceIsNoop(t *testing.T) {
	backupName, err := createBackup(filepath.Join(t.TempDir(), "missing.yaml"))

	require.NoError(t, err)
	assert.Empty(t, backupName)
}
