package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	backupDirName        = "backups"
	maxConfigBackupCount = 10
)

// CreateBackup 在源文件同级的 backups/ 子目录下创建带纳秒时间戳的备份。
// 返回相对于源文件目录的备份路径（如 "backups/conf.yaml.backup.123456789"）。
// 文件不存在时返回 ("", nil)，不视为错误。
// 创建后自动修剪旧备份，保留最近 maxConfigBackupCount 份。
func CreateBackup(filePath string) (string, error) {
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

	pruneConfigBackups(backupDir)
	return filepath.Join(backupDirName, backupName), nil
}

// pruneConfigBackups 修剪 backups/ 目录中的旧备份，仅保留最近 maxConfigBackupCount 份。
// 文件名中的纳秒时间戳天然有序，按名排序即可。
func pruneConfigBackups(backupDir string) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		log.Warnf("Failed to list backup directory %q: %v", backupDir, err)
		return
	}

	backups := make([]os.DirEntry, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.Contains(entry.Name(), ".backup.") {
			continue
		}
		backups = append(backups, entry)
	}
	if len(backups) <= maxConfigBackupCount {
		return
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() < backups[j].Name()
	})

	for _, entry := range backups[:len(backups)-maxConfigBackupCount] {
		if err := os.Remove(filepath.Join(backupDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			log.Warnf("Failed to remove stale backup %q: %v", entry.Name(), err)
		}
	}
}
