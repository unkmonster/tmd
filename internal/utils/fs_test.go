package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func generateTemp(num int) ([]*os.File, error) {
	temps := make([]*os.File, 0, num)
	for i := 0; i < 100; i++ {
		file, err := os.CreateTemp("", "")
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
			lnk := filepath.Join(tempDir, filepath.Base(path)+".lnk")
			hr := CreateLink(path, lnk)
			if hr != nil {
				t.Error(hr)
				return
			}
			ex, err := PathExists(lnk)
			if err != nil {
				t.Error(err)
			}
			if !ex {
				t.Errorf("create failed: %s -> %s", lnk, path)
			}

			if runtime.GOOS == "windows" {
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
		}(temp.Name())
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
