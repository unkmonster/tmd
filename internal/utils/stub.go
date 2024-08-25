//go:build !windows
// +build !windows

package utils

func SetConsoleTitle(title string) error {
	return nil
}

func GetConsoleTitle() (string, error) {
	return "", nil
}
