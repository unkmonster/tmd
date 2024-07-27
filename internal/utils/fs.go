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
	reEnter         = regexp.MustCompile(`\r?\n`)
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
func WinFileName(name []byte) []byte {
	result := reUrl.ReplaceAll(name, []byte(""))
	result = reWinNonSupport.ReplaceAll(result, []byte(""))
	result = reEnter.ReplaceAll(result, []byte(" "))
	return result
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
