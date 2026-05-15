package utils

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	if maxLen <= 0 {
		return ""
	}

	var buffer bytes.Buffer

	for _, ch := range name {
		switch ch {
		case '\r':
			continue
		case '\n':
			if buffer.Len()+1 > maxLen {
				return buffer.String()
			}
			buffer.WriteRune(' ')
		default:
			if buffer.Len()+utf8.RuneLen(ch) > maxLen {
				return buffer.String()
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

		path = nextUniquePathCandidate(path)
	}
}

type UniquePathResolver struct {
	mu       sync.Mutex
	reserved map[string]struct{}
}

func NewUniquePathResolver() *UniquePathResolver {
	return &UniquePathResolver{
		reserved: make(map[string]struct{}),
	}
}

func (r *UniquePathResolver) UniquePath(path string) (string, error) {
	if r == nil {
		return UniquePath(path)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for {
		if _, exists := r.reserved[path]; exists {
			path = nextUniquePathCandidate(path)
			continue
		}

		exist, err := PathExists(path)
		if err != nil {
			return "", err
		}
		if !exist {
			r.reserved[path] = struct{}{}
			return path, nil
		}

		path = nextUniquePathCandidate(path)
	}
}

func nextUniquePathCandidate(path string) string {
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
				return filepath.Join(dir, stem+ext)
			}
		}
	}

	return filepath.Join(dir, stem+"(1)"+ext)
}

func GetExtFromUrl(u string) (string, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	// 使用 path.Ext 而不是 filepath.Ext，因为 URL path 总是使用正斜杠
	// filepath.Ext 在 Windows 上会使用反斜杠作为分隔符，导致无法正确提取扩展名
	return path.Ext(pu.Path), nil
}

// GetTwitterImageQualityParams 根据 URL 返回 Twitter 图片质量参数
// 规则：
//   - 视频（包含 tweet_video 或 video.twimg.com）：返回空 map
//   - 已有 name=orig 或 name=4096x4096：返回空 map（已是最高质量）
//   - 已有 name=small/medium/large 等：返回 {"name": "4096x4096"}
//   - 无 name 参数：返回 {"name": "4096x4096"}
func GetTwitterImageQualityParams(u string) map[string]string {
	// 视频跳过
	if strings.Contains(u, "tweet_video") || strings.Contains(u, "video.twimg.com") {
		return nil
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return map[string]string{"name": "4096x4096"}
	}

	nameValue := parsed.Query().Get("name")
	switch nameValue {
	case "orig", "4096x4096":
		// 已是最高质量，不添加参数
		return nil
	default:
		// 无 name 参数或为 small/medium/large 等，统一替换为 4096x4096
		return map[string]string{"name": "4096x4096"}
	}
}
