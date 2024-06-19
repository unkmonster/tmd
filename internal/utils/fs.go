package utils

/*
#cgo CPPFLAGS: -DUNICODE=1
#cgo LDFLAGS: -luuid -lole32 -loleaut32
#include <Windows.h>
int msgbox(wchar_t* msg);
HRESULT CreateLink(wchar_t* lpszPathObj, wchar_t* lpszPathLink);
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unsafe"
)

var (
	reUrl           = regexp.MustCompile(`(https?|ftp|file)://[-A-Za-z0-9+&@#/%?=~_|!:,.;]+[-A-Za-z0-9+&@#/%=~_|]`)
	reEnter         = regexp.MustCompile(`\r?\n`)
	reWinNonSupport = regexp.MustCompile(`[/\\:*?"<>\|]`)
)

func PathExists(path string) (bool, error) {

	_, err := os.Stat(path)

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
		stem, _ := strings.CutPrefix(base, ext)
		stemlen := len(stem)

		// 处理已括号结尾的文件名
		if stem[stemlen-1] == ')' {
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

func CreateLink(path string, lnk string) int {
	u16Path := utf16.Encode([]rune(path))
	u16Lnk := utf16.Encode([]rune(lnk))
	pPath := unsafe.Pointer(&u16Path[0])
	pLnk := unsafe.Pointer(&u16Lnk[0])
	hr := C.CreateLink((*C.wchar_t)(pPath), (*C.wchar_t)(pLnk))
	return int(hr)
}
