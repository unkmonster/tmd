package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestUniquePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(tempDir)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello", "hello(1)"},
		{"hello", "hello(2)"},
		{"hello(2))", "hello(2))"},
		{"hello(2))", "hello(2))(1)"},
	}

	for _, test := range tests {
		path := filepath.Join(tempDir, test.input)
		path, err = UniquePath(path)
		if err != nil {
			t.Error(err)
			continue
		}

		if filepath.Base(path) != test.expected {
			t.Errorf("UniquePath(path) = %s, want %s", filepath.Base(path), test.expected)
			continue
		}

		if err = os.Mkdir(path, 0755); err != nil {
			t.Error(err)
		}
	}
}

func generateTemp(num int) ([]string, error) {
	temps := make([]string, 0, num)
	for i := 0; i < 100; i++ {
		file, err := os.CreateTemp("", "")
		if err != nil {
			return nil, err
		}
		temps = append(temps, file.Name())
	}
	return temps, nil
}

func generateTempDir(num int) ([]string, error) {
	temps := make([]string, 0, num)
	for i := 0; i < 100; i++ {
		file, err := os.MkdirTemp("", "")
		if err != nil {
			return nil, err
		}
		temps = append(temps, file)
	}
	return temps, nil
}

func TestLink(t *testing.T) {
	temps, err := generateTemp(500)
	if err != nil {
		t.Error(err)
		return
	}
	tempdirs, err := generateTempDir(20)
	if err != nil {
		t.Error(err)
		return
	}
	temps = append(temps, tempdirs...)

	wg := sync.WaitGroup{}
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(tempDir)
	fmt.Println("temp dir:", tempDir)

	// 在临时文件夹中创建指向临时文件的快捷方式
	for _, temp := range temps {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			lnk := filepath.Join(tempDir, filepath.Base(path))
			hr := os.Symlink(path, lnk)
			if hr != nil {
				t.Error(hr)
				return
			}

			target, err := os.Readlink(lnk)
			if err != nil {
				t.Error(err)
				return
			}
			if filepath.Clean(target) != filepath.Clean(path) {
				t.Errorf("%s -> %s, want %s", lnk, target, path)
			}
		}(temp)
	}

	wg.Wait()
}

func TestSafeDirName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// 移除URL并处理无效字符
		{input: "https://example.com/file?name=test", expected: ""},

		// 移除无效字符
		{input: "invalid|file?name", expected: "invalidfilename"},

		// 移除 \r 并将 \n 替换为空格
		{input: "filename_with\nnewlines\r", expected: "filename_with newlines"},

		// 处理组合情况
		{input: "https://example.com/path/file|name\nwith_invalid\rchars", expected: " with_invalidchars"},

		// 处理无效字符的组合
		{input: `file<name>:invalid|chars`, expected: "filenameinvalidchars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := WinFileName(tt.input)
			if got != tt.expected {
				t.Errorf("WinFileName(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetExtFromUrl(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedExt string
		expectError bool
	}{
		{
			name:        "Valid URL with extension",
			url:         "http://example.com/file.jpg",
			expectedExt: ".jpg",
			expectError: false,
		},
		{
			name:        "Valid URL with multiple extensions",
			url:         "http://example.com/archive.tar.gz",
			expectedExt: ".gz",
			expectError: false,
		},
		{
			name:        "Valid URL without extension",
			url:         "http://example.com/file",
			expectedExt: "",
			expectError: false,
		},
		{
			name:        "URL with query string and extension",
			url:         "http://example.com/file.jpg?version=1.2",
			expectedExt: ".jpg",
			expectError: false,
		},
		{
			name:        "Invalid URL format",
			url:         "://invalid-url",
			expectedExt: "",
			expectError: true,
		},
		{
			name:        "URL with path but no file extension",
			url:         "http://example.com/path/to/resource/",
			expectedExt: "",
			expectError: false,
		},
		{
			name:        "URL with special characters and extension",
			url:         "http://example.com/file%20name.txt",
			expectedExt: ".txt",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := GetExtFromUrl(tt.url)
			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}
			if ext != tt.expectedExt {
				t.Errorf("expected extension: %s, got: %s", tt.expectedExt, ext)
			}
		})
	}
}

func TestParseCookie(t *testing.T) {
	tests := []struct {
		name        string
		cookie      string
		expectedMap map[string]string
		expectError bool
	}{
		{
			name:        "Valid single cookie",
			cookie:      "name=value",
			expectedMap: map[string]string{"name": "value"},
			expectError: false,
		},
		{
			name:        "Valid multiple cookies",
			cookie:      "name=value; token=abc123; path=/",
			expectedMap: map[string]string{"name": "value", "token": "abc123", "path": "/"},
			expectError: false,
		},
		{
			name:        "Valid cookies with spaces",
			cookie:      "name=value; token=abc123; path=/; secure=true",
			expectedMap: map[string]string{"name": "value", "token": "abc123", "path": "/", "secure": "true"},
			expectError: false,
		},
		{
			name:        "Empty cookie string",
			cookie:      "",
			expectedMap: map[string]string{},
			expectError: false,
		},
		{
			name:        "Cookie with empty value",
			cookie:      "name=",
			expectedMap: map[string]string{"name": ""},
			expectError: false,
		},
		{
			name:        "Cookie with no equal sign",
			cookie:      "namevalue",
			expectedMap: nil,
			expectError: true,
		},
		{
			name:        "Cookie with multiple equal signs",
			cookie:      "name=value=extra",
			expectedMap: map[string]string{"name": "value=extra"},
			expectError: false,
		},
		{
			name:        "Cookie with semicolon but no key-value pair",
			cookie:      "name=value;;token=abc123",
			expectedMap: map[string]string{"name": "value", "token": "abc123"},
			expectError: false,
		},
		{
			name:        "Cookie with special characters",
			cookie:      "name=value; token=abc@123; path=/home/user",
			expectedMap: map[string]string{"name": "value", "token": "abc@123", "path": "/home/user"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCookie(tt.cookie)
			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}
			if !compareMaps(result, tt.expectedMap) {
				t.Errorf("expected map: %v, got: %v", tt.expectedMap, result)
			}
		})
	}
}

func compareMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}
	return true
}
