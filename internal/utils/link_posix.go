//go:build !windows
// +build !windows

package utils

import "os"

func CreateLink(path string, lnk string) error {
	return os.Symlink(path, lnk)
}
