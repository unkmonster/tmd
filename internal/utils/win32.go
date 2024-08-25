//go:build windows
// +build windows

package utils

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

func SetConsoleTitle(title string) error {
	// 获取 Windows API 的 SetConsoleTitleW 函数
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleTitle := kernel32.NewProc("SetConsoleTitleW")

	// 调用 SetConsoleTitleW 函数
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return err
	}

	_, _, err = setConsoleTitle.Call(uintptr(unsafe.Pointer(titlePtr)))
	if err != nil && err.Error() != "The operation completed successfully." {
		return err
	}
	return nil
}

func GetConsoleTitle() (string, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleTitle := kernel32.NewProc("GetConsoleTitleW")

	// 创建一个用于存储标题的缓冲区
	buf := make([]uint16, 512) // 512 是缓冲区大小

	// 调用 GetConsoleTitleW 函数
	ret, _, err := getConsoleTitle.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		return "", err
	}

	// 将 UTF-16 编码转换为字符串
	return string(utf16.Decode(buf[:ret])), nil
}
