package utils

/*
#cgo CPPFLAGS: -DUNICODE=1
#cgo windows LDFLAGS: -luuid -lole32 -loleaut32
#cgo linux LDFLAGS: -static-libstdc++ -static-libgcc
#include <stdlib.h>
int CreateSymLink(const char* path, const char* sympath);
*/
import "C"
import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unsafe"
)

var (
	reUrl           = regexp.MustCompile(`(https?|ftp|file)://[-A-Za-z0-9+&@#/%?=~_|!:,.;]+[-A-Za-z0-9+&@#/%=~_|]`)
	reWinNonSupport = regexp.MustCompile(`[/\\:*?"<>\|]`)
)

func PathExists(path string) (bool, error) {
	_, err := os.Lstat(path)

	if err == nil {

		return true, nil

	}

	if os.IsNotExist(err) {

		return false, nil

	}
	return false, err
}

// 将无后缀的文件名更新为有效的 Windows 文件名
func WinFileName(name string) string {
	// 将字节切片转换为字符串
	// 使用正则表达式进行替换
	name = reUrl.ReplaceAllString(name, "")
	name = reWinNonSupport.ReplaceAllString(name, "")

	// 创建一个缓冲区，避免多次分配
	var buffer bytes.Buffer

	// 遍历字符串，对字符进行处理
	for _, ch := range name {
		switch ch {
		case '\r':
			// 跳过 \r
			continue
		case '\n':
			// 将 \n 替换为空格
			buffer.WriteRune(' ')
		default:
			// 其他字符直接添加到缓冲区
			buffer.WriteRune(ch)
		}
	}

	return buffer.String()
}

func UniquePath(path string) (string, error) {
	for {
		exist, err := PathExists(path)
		if err != nil {
			return "", err
		}
		if !exist {
			return path, nil
		}

		dir := filepath.Dir(path)
		base := filepath.Base(path)
		ext := filepath.Ext(path)
		stem, _ := strings.CutSuffix(base, ext)
		stemlen := len(stem)

		// 处理已括号结尾的文件名
		if stemlen > 0 && stem[stemlen-1] == ')' {
			if left := strings.LastIndex(stem, "("); left != -1 {

				index, err := strconv.Atoi(stem[left+1 : stemlen-1])
				if err == nil {
					index++
					stem = fmt.Sprintf("%s(%d)", stem[:left], index)
					path = filepath.Join(dir, stem+ext)
					continue
				}
			}
		}

		path = filepath.Join(dir, stem+"(1)"+ext)
	}
}

func CreateLink(path string, lnk string) error {
	cpath := C.CString(path)
	clnk := C.CString(lnk)
	defer C.free(unsafe.Pointer(cpath))
	defer C.free(unsafe.Pointer(clnk))
	hr := C.CreateSymLink(cpath, clnk)
	if hr != 0 {
		return fmt.Errorf("failed to create link: hresult/errno = %d", hr)
	}
	return nil
}
