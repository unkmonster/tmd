package path

import (
	"os"
	"path/filepath"
)

// StorePath 存储路径
type StorePath struct {
	Root       string
	Users      string
	Data       string
	DB         string
	ErrorJ     string
	JsonErrorJ string
}

// NewStorePath 创建存储路径
func NewStorePath(root string) (*StorePath, error) {
	sp := &StorePath{Root: root}
	sp.Users = filepath.Join(root, "users")
	sp.Data = filepath.Join(root, ".data")
	sp.DB = filepath.Join(sp.Data, "foo.db")
	sp.ErrorJ = filepath.Join(sp.Data, "errors.json")
	sp.JsonErrorJ = filepath.Join(sp.Data, "json_errors.json")

	for _, dir := range []string{sp.Root, sp.Users, sp.Data} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	return sp, nil
}
