package path

import (
	"os"
	"path/filepath"
)

// StorePath 存储路径
type StorePath struct {
	Root   string
	Users  string
	Data   string
	DB     string
	ErrorJ string
}

// NewStorePath 创建存储路径
func NewStorePath(root string) (*StorePath, error) {
	sp := &StorePath{Root: root}
	sp.Users = filepath.Join(root, "users")
	sp.Data = filepath.Join(root, ".data")
	sp.DB = filepath.Join(sp.Data, "foo.db")
	sp.ErrorJ = filepath.Join(sp.Data, "errors.json")

	if err := os.MkdirAll(sp.Root, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(sp.Users, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(sp.Data, 0755); err != nil {
		return nil, err
	}
	return sp, nil
}
