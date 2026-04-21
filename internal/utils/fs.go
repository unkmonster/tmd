package utils

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
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

// reserve 8 bytes for suffix
// NTFS 单个文件名硬限制为 255 字符（与长路径设置无关）
const DefaultMaxFileNameLen = 158

// WinFileNameWithMaxLen 将无后缀的文件名更新为有效的 Windows 文件名
// maxLen 指定最大文件名长度
func WinFileNameWithMaxLen(name string, maxLen int) string {
	name = reUrl.ReplaceAllString(name, "")
	name = reWinNonSupport.ReplaceAllString(name, "")

	var buffer bytes.Buffer

	for _, ch := range name {
		switch ch {
		case '\r':
			continue
		case '\n':
			if buffer.Len()+1 > maxLen {
				break
			}
			buffer.WriteRune(' ')
		default:
			if buffer.Len()+utf8.RuneLen(ch) > maxLen {
				break
			}
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

func GetExtFromUrl(u string) (string, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	return filepath.Ext(pu.Path), nil
}
