package api

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	backupDirName        = "backups"
	maxConfigBackupCount = 10
)

func createBackup(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	backupDir := filepath.Join(filepath.Dir(filePath), backupDirName)
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", err
	}

	backupName := fmt.Sprintf("%s.backup.%d", filepath.Base(filePath), time.Now().UnixNano())
	backupPath := filepath.Join(backupDir, backupName)
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return "", err
	}

	if err := pruneConfigBackups(backupDir); err != nil {
		return filepath.Join(backupDirName, backupName), err
	}
	return filepath.Join(backupDirName, backupName), nil
}

func pruneConfigBackups(backupDir string) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}

	backups := make([]os.DirEntry, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.Contains(entry.Name(), ".backup.") {
			continue
		}
		backups = append(backups, entry)
	}
	if len(backups) <= maxConfigBackupCount {
		return nil
	}

	sort.Slice(backups, func(i, j int) bool {
		left, leftErr := backups[i].Info()
		right, rightErr := backups[j].Info()
		if leftErr != nil || rightErr != nil {
			return backups[i].Name() < backups[j].Name()
		}
		if left.ModTime().Equal(right.ModTime()) {
			return backups[i].Name() < backups[j].Name()
		}
		return left.ModTime().Before(right.ModTime())
	})

	for _, entry := range backups[:len(backups)-maxConfigBackupCount] {
		if err := os.Remove(filepath.Join(backupDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
