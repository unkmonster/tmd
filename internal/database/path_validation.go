package database

import (
	"fmt"
	"path/filepath"
	"strings"
)

func normalizeEntityParentDir(parentDir string) (string, error) {
	if strings.ContainsRune(parentDir, '\x00') {
		return "", fmt.Errorf("parent dir contains NUL byte")
	}

	abs, err := filepath.Abs(parentDir)
	if err != nil {
		return "", err
	}
	return abs, nil
}
