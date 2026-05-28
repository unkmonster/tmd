package path

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StorePath 存储路径
type StorePath struct {
	Root           string
	Users          string
	Data           string
	DB             string
	ErrorsPath     string
	JSONErrorsPath string
}

// NewStorePath 创建存储路径
func NewStorePath(root string) (*StorePath, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}

	sp := &StorePath{Root: root}
	sp.Users = filepath.Join(root, "users")
	sp.Data = filepath.Join(root, ".data")
	sp.DB = filepath.Join(sp.Data, "foo.db")
	sp.ErrorsPath = filepath.Join(sp.Data, "errors.json")
	sp.JSONErrorsPath = filepath.Join(sp.Data, "json_errors.json")

	for _, dir := range []string{sp.Root, sp.Users, sp.Data} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	return sp, nil
}

func normalizeRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("root path cannot be empty")
	}

	abs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	return abs, nil
}
