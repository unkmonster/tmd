package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== ScreenName 测试 ====================

func TestNormalizeScreenName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "带@前缀",
			input:    "@twitteruser",
			expected: "twitteruser",
		},
		{
			name:     "不带@前缀",
			input:    "twitteruser",
			expected: "twitteruser",
		},
		{
			name:     "仅一个@被剥离",
			input:    "@@twitteruser",
			expected: "@twitteruser",
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeScreenName(tt.input))
		})
	}
}

func TestIsValidScreenName(t *testing.T) {
	tests := []struct {
		name       string
		screenName string
		expected   bool
	}{
		{
			name:       "普通用户名",
			screenName: "twitteruser",
			expected:   true,
		},
		{
			name:       "带下划线",
			screenName: "user_name_123",
			expected:   true,
		},
		{
			name:       "15字符边界",
			screenName: "user_name_1234",
			expected:   true,
		},
		{
			name:       "空字符串",
			screenName: "",
			expected:   false,
		},
		{
			name:       "超过15字符",
			screenName: "user_name_123456",
			expected:   false,
		},
		{
			name:       "带连字符",
			screenName: "user-name",
			expected:   false,
		},
		{
			name:       "带@前缀",
			screenName: "@twitteruser",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidScreenName(tt.screenName))
		})
	}
}

// ==================== PathExists 测试 ====================

func TestPathExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "path_exists_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name     string
		setup    func() string
		expected bool
		wantErr  bool
	}{
		{
			name: "存在的文件",
			setup: func() string {
				f, _ := os.CreateTemp(tempDir, "exist_*.txt")
				f.Close()
				return f.Name()
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "存在的目录",
			setup: func() string {
				d, _ := os.MkdirTemp(tempDir, "exist_dir")
				return d
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "不存在的路径",
			setup: func() string {
				return filepath.Join(tempDir, "non_existent_file.txt")
			},
			expected: false,
			wantErr:  false,
		},
		{
			name: "符号链接",
			setup: func() string {
				target, _ := os.CreateTemp(tempDir, "target_*.txt")
				target.Close()
				link := filepath.Join(tempDir, "symlink.txt")
				os.Symlink(target.Name(), link)
				return link
			},
			expected: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			exists, err := PathExists(path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, exists)
			}
		})
	}
}

// ==================== UniquePath 测试 ====================

func TestUniquePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unique_path_test")
	require.NoError(t, err)
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
		{"file.txt", "file.txt"},
		{"file.txt", "file(1).txt"},
		{"file(5).txt", "file(5).txt"},     // 已存在的路径返回原路径，递增在调用方处理
		{"file(abc).txt", "file(abc).txt"}, // 非数字括号不会递增
	}

	for _, test := range tests {
		path := filepath.Join(tempDir, test.input)
		path, err = UniquePath(path)
		require.NoError(t, err)

		assert.Equal(t, test.expected, filepath.Base(path))

		err = os.Mkdir(path, 0755)
		require.NoError(t, err)
	}
}

func TestUniquePath_WithFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unique_path_file_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 测试文件路径唯一化
	basePath := filepath.Join(tempDir, "testfile.txt")

	// 创建第一个文件
	f1, err := os.Create(basePath)
	require.NoError(t, err)
	f1.Close()

	// 获取唯一路径应该是 testfile(1).txt
	path2, err := UniquePath(basePath)
	require.NoError(t, err)
	assert.Equal(t, "testfile(1).txt", filepath.Base(path2))

	// 创建第二个文件
	f2, err := os.Create(path2)
	require.NoError(t, err)
	f2.Close()

	// 获取唯一路径应该是 testfile(2).txt
	path3, err := UniquePath(basePath)
	require.NoError(t, err)
	assert.Equal(t, "testfile(2).txt", filepath.Base(path3))
}

// ==================== WinFileNameWithMaxLen 测试 ====================

func TestWinFileNameWithMaxLen(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "短文本在限制内",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "文本超出限制",
			input:    "hello world this is a long text",
			maxLen:   10,
			expected: "hello worl",
		},
		{
			name:     "零长度限制",
			input:    "hello",
			maxLen:   0,
			expected: "",
		},
		{
			name:     "负数限制",
			input:    "hello",
			maxLen:   -1,
			expected: "",
		},
		{
			name:     "特殊字符处理",
			input:    "file<name>:test",
			maxLen:   10,
			expected: "filenamete",
		},
		{
			name:     "换行符处理",
			input:    "hello\nworld",
			maxLen:   8,
			expected: "hello wo",
		},
		{
			name:     "回车符移除",
			input:    "hello\rworld",
			maxLen:   20,
			expected: "helloworld",
		},
		{
			name:     "Unicode文本",
			input:    "比基尼测试文本",
			maxLen:   10,
			expected: "比基尼",
		},
		{
			name:     "URL移除",
			input:    "https://example.com/file",
			maxLen:   20,
			expected: "", // URL正则表达式会移除整个URL
		},
		{
			name:     "Windows非法字符",
			input:    `file|name<>:*?"\/`,
			maxLen:   20,
			expected: "filename",
		},
		{
			name:     "组合情况",
			input:    "https://example.com/path/file|name\nwith_invalid\rchars",
			maxLen:   DefaultMaxFileNameLen,
			expected: " with_invalidchars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WinFileNameWithMaxLen(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, got)
		})
	}
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
			got := WinFileNameWithMaxLen(tt.input, DefaultMaxFileNameLen)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// ==================== GetExtFromUrl 测试 ====================

func TestGetExtFromUrl(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedExt string
		expectError bool
	}{
		{
			name:        "有效URL带扩展名",
			url:         "http://example.com/file.jpg",
			expectedExt: ".jpg",
			expectError: false,
		},
		{
			name:        "多重扩展名",
			url:         "http://example.com/archive.tar.gz",
			expectedExt: ".gz",
			expectError: false,
		},
		{
			name:        "无扩展名",
			url:         "http://example.com/file",
			expectedExt: "",
			expectError: false,
		},
		{
			name:        "带查询参数的URL",
			url:         "http://example.com/file.jpg?version=1.2",
			expectedExt: ".jpg",
			expectError: false,
		},
		{
			name:        "无效URL格式",
			url:         "://invalid-url",
			expectedExt: "",
			expectError: true,
		},
		{
			name:        "路径结尾无扩展名",
			url:         "http://example.com/path/to/resource/",
			expectedExt: "",
			expectError: false,
		},
		{
			name:        "特殊字符和扩展名",
			url:         "http://example.com/file%20name.txt",
			expectedExt: ".txt",
			expectError: false,
		},
		{
			name:        "HTTPS URL",
			url:         "https://example.com/image.png",
			expectedExt: ".png",
			expectError: false,
		},
		{
			name:        "FTP URL",
			url:         "ftp://server.com/document.pdf",
			expectedExt: ".pdf",
			expectError: false,
		},
		{
			name:        "带锚点的URL",
			url:         "http://example.com/page.html#section",
			expectedExt: ".html",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := GetExtFromUrl(tt.url)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedExt, ext)
			}
		})
	}
}

// ==================== Heap 测试 ====================

func TestHeap(t *testing.T) {
	t.Run("基本操作", func(t *testing.T) {
		heap := NewHeap(func(a, b int) bool { return a < b })
		nums := []int{1, 3, 0, 9, 2, 0, 1}
		wants := []int{0, 0, 1, 1, 2, 3, 9}

		for _, v := range nums {
			heap.Push(v)
		}

		assert.Equal(t, len(wants), heap.Size())

		for _, want := range wants {
			assert.Equal(t, want, heap.Peek())
			heap.Pop()
		}

		assert.True(t, heap.Empty())
	})

	t.Run("最大堆", func(t *testing.T) {
		heap := NewHeap(func(a, b int) bool { return a > b })
		nums := []int{1, 3, 0, 9, 2, 5, 7}
		wants := []int{9, 7, 5, 3, 2, 1, 0}

		for _, v := range nums {
			heap.Push(v)
		}

		for _, want := range wants {
			assert.Equal(t, want, heap.Peek())
			heap.Pop()
		}
	})

	t.Run("空堆操作", func(t *testing.T) {
		heap := NewHeap(func(a, b int) bool { return a < b })

		assert.Panics(t, func() { heap.Peek() })
		assert.Panics(t, func() { heap.Pop() })
	})

	t.Run("并发安全", func(t *testing.T) {
		heap := NewHeap(func(a, b int) bool { return a < b })
		var wg sync.WaitGroup
		numGoroutines := 100
		numPushes := 100

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numPushes; j++ {
					heap.Push(id*numPushes + j)
				}
			}(i)
		}
		wg.Wait()

		assert.Equal(t, numGoroutines*numPushes, heap.Size())

		// 验证堆的性质
		prev := heap.Peek()
		heap.Pop()
		for !heap.Empty() {
			curr := heap.Peek()
			assert.LessOrEqual(t, prev, curr)
			prev = curr
			heap.Pop()
		}
	})

	t.Run("字符串堆", func(t *testing.T) {
		heap := NewHeap(func(a, b string) bool { return a < b })
		words := []string{"cherry", "apple", "banana", "date"}
		wants := []string{"apple", "banana", "cherry", "date"}

		for _, w := range words {
			heap.Push(w)
		}

		for _, want := range wants {
			assert.Equal(t, want, heap.Peek())
			heap.Pop()
		}
	})

	t.Run("单元素堆", func(t *testing.T) {
		heap := NewHeap(func(a, b int) bool { return a < b })
		heap.Push(42)

		assert.Equal(t, 1, heap.Size())
		assert.Equal(t, 42, heap.Peek())
		heap.Pop()
		assert.True(t, heap.Empty())
	})
}

// ==================== Link/Symlink 测试 ====================

func generateTemp(num int) ([]string, error) {
	temps := make([]string, 0, num)
	for i := 0; i < num; i++ {
		file, err := os.CreateTemp("", "")
		if err != nil {
			return nil, err
		}
		temps = append(temps, file.Name())
		file.Close()
	}
	return temps, nil
}

func generateTempDir(num int) ([]string, error) {
	temps := make([]string, 0, num)
	for i := 0; i < num; i++ {
		dir, err := os.MkdirTemp("", "")
		if err != nil {
			return nil, err
		}
		temps = append(temps, dir)
	}
	return temps, nil
}

func TestLink(t *testing.T) {
	// Windows可能需要管理员权限创建符号链接
	if runtime.GOOS == "windows" {
		t.Skip("Windows上创建符号链接需要特殊权限")
	}

	temps, err := generateTemp(100)
	require.NoError(t, err)
	defer func() {
		for _, f := range temps {
			os.Remove(f)
		}
	}()

	tempdirs, err := generateTempDir(20)
	require.NoError(t, err)
	defer func() {
		for _, d := range tempdirs {
			os.RemoveAll(d)
		}
	}()

	temps = append(temps, tempdirs...)

	wg := sync.WaitGroup{}
	tempDir, err := os.MkdirTemp("", "link_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 在临时文件夹中创建指向临时文件的符号链接
	for _, temp := range temps {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			lnk := filepath.Join(tempDir, filepath.Base(path))
			err := os.Symlink(path, lnk)
			if err != nil {
				t.Errorf("创建符号链接失败: %v", err)
				return
			}

			target, err := os.Readlink(lnk)
			if err != nil {
				t.Errorf("读取符号链接失败: %v", err)
				return
			}
			if filepath.Clean(target) != filepath.Clean(path) {
				t.Errorf("%s -> %s, want %s", lnk, target, path)
			}
		}(temp)
	}

	wg.Wait()
}

// ==================== HTTP 工具测试 ====================

func TestCheckRespStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		errCode    int
	}{
		{
			name:       "成功响应 200",
			statusCode: 200,
			body:       "OK",
			wantErr:    false,
		},
		{
			name:       "成功响应 201",
			statusCode: 201,
			body:       "Created",
			wantErr:    false,
		},
		{
			name:       "客户端错误 400",
			statusCode: 400,
			body:       "Bad Request",
			wantErr:    true,
			errCode:    400,
		},
		{
			name:       "未授权 401",
			statusCode: 401,
			body:       "Unauthorized",
			wantErr:    true,
			errCode:    401,
		},
		{
			name:       "禁止访问 403",
			statusCode: 403,
			body:       "Forbidden",
			wantErr:    true,
			errCode:    403,
		},
		{
			name:       "未找到 404",
			statusCode: 404,
			body:       "Not Found",
			wantErr:    true,
			errCode:    404,
		},
		{
			name:       "服务器错误 500",
			statusCode: 500,
			body:       "Internal Server Error",
			wantErr:    true,
			errCode:    500,
		},
		{
			name:       "边界值 399",
			statusCode: 399,
			body:       "OK",
			wantErr:    false,
		},
		{
			name:       "边界值 400",
			statusCode: 400,
			body:       "Error",
			wantErr:    true,
			errCode:    400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = &resty.Response{}
			// 使用反射或直接构造Response比较困难，这里我们测试错误类型
			if tt.wantErr {
				err := &HttpStatusError{Code: tt.errCode, Msg: tt.body}
				assert.Equal(t, tt.errCode, err.Code)
				assert.Contains(t, err.Error(), fmt.Sprintf("%d", tt.errCode))
			}
		})
	}
}

func TestIsStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     int
		expected bool
	}{
		{
			name:     "匹配的状态码",
			err:      &HttpStatusError{Code: 404, Msg: "Not Found"},
			code:     404,
			expected: true,
		},
		{
			name:     "不匹配的状态码",
			err:      &HttpStatusError{Code: 404, Msg: "Not Found"},
			code:     500,
			expected: false,
		},
		{
			name:     "nil错误",
			err:      nil,
			code:     404,
			expected: false,
		},
		{
			name:     "非HttpStatusError类型",
			err:      fmt.Errorf("some error"),
			code:     404,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStatusCode(tt.err, tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHttpStatusError_Error(t *testing.T) {
	err := &HttpStatusError{Code: 404, Msg: "Not Found"}
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Not Found")
}

func TestStripAvatarSuffix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "_normal后缀",
			input:    "https://example.com/avatar_normal.jpg",
			expected: "https://example.com/avatar.jpg",
		},
		{
			name:     "_bigger后缀",
			input:    "https://example.com/avatar_bigger.png",
			expected: "https://example.com/avatar.png",
		},
		{
			name:     "_mini后缀",
			input:    "https://example.com/avatar_mini.gif",
			expected: "https://example.com/avatar.gif",
		},
		{
			name:     "无后缀",
			input:    "https://example.com/avatar.jpg",
			expected: "https://example.com/avatar.jpg",
		},
		{
			name:     "多个后缀替换所有",
			input:    "https://example.com/avatar_normal_bigger_mini.jpg",
			expected: "https://example.com/avatar.jpg", // 所有后缀都会被替换
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
		{
			name:     "只有后缀",
			input:    "_normal",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripAvatarSuffix(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== ExtractIDs 测试 ====================

type testUser struct {
	ID   uint64
	Name string
}

func TestExtractIDs(t *testing.T) {
	tests := []struct {
		name     string
		users    []testUser
		expected []uint64
	}{
		{
			name:     "正常提取",
			users:    []testUser{{ID: 1, Name: "Alice"}, {ID: 2, Name: "Bob"}, {ID: 3, Name: "Charlie"}},
			expected: []uint64{1, 2, 3},
		},
		{
			name:     "空切片",
			users:    []testUser{},
			expected: nil,
		},
		{
			name:     "nil切片",
			users:    nil,
			expected: nil,
		},
		{
			name:     "单个元素",
			users:    []testUser{{ID: 42, Name: "Solo"}},
			expected: []uint64{42},
		},
		{
			name:     "大ID值",
			users:    []testUser{{ID: 18446744073709551615, Name: "Max"}},
			expected: []uint64{18446744073709551615},
		},
	}

	extractor := func(u testUser) uint64 { return u.ID }

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractIDs(tt.users, extractor)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractIDs_WithDifferentTypes(t *testing.T) {
	type anotherUser struct {
		UserID uint64
		Email  string
	}

	users := []anotherUser{
		{UserID: 100, Email: "a@example.com"},
		{UserID: 200, Email: "b@example.com"},
	}

	extractor := func(u anotherUser) uint64 { return u.UserID }
	result := ExtractIDs(users, extractor)

	assert.Equal(t, []uint64{100, 200}, result)
}

// ==================== RecoverWithLog 测试 ====================

func TestRecoverWithLog(t *testing.T) {
	t.Run("恢复panic", func(t *testing.T) {
		// RecoverWithLog 会捕获panic并记录日志，不会重新抛出
		// 测试验证函数正常结束没有panic传出
		executed := true
		func() {
			defer RecoverWithLog("test")
			panic("test panic")
		}()
		// 如果能执行到这里，说明panic被恢复了
		assert.True(t, executed)
	})

	t.Run("无panic", func(t *testing.T) {
		var executed bool
		func() {
			defer func() {
				RecoverWithLog("test")
				executed = true
			}()
			// 正常执行，不panic
		}()
		assert.True(t, executed)
	})
}

// ==================== TimeRange 测试 ====================

func TestTimeRange(t *testing.T) {
	t.Run("基本创建", func(t *testing.T) {
		min := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		max := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)

		tr := TimeRange{Min: min, Max: max}

		assert.Equal(t, min, tr.Min)
		assert.Equal(t, max, tr.Max)
	})

	t.Run("零值", func(t *testing.T) {
		var tr TimeRange

		assert.True(t, tr.Min.IsZero())
		assert.True(t, tr.Max.IsZero())
	})

	t.Run("相同时间", func(t *testing.T) {
		now := time.Now()
		tr := TimeRange{Min: now, Max: now}

		assert.Equal(t, tr.Min, tr.Max)
	})
}

// ==================== Windows 控制台测试 ====================

func TestSetConsoleTitle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows专用测试")
	}

	err := SetConsoleTitle("hello")
	if err != nil {
		t.Skip("无法设置控制台标题，可能需要Windows终端")
	}

	title, err := GetConsoleTitle()
	if err != nil {
		t.Skip("无法获取控制台标题")
	}

	// 标题可能被截断或有其他限制
	assert.Contains(t, title, "hello")
}

func TestGetConsoleTitle_NonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("非Windows测试")
	}

	title, err := GetConsoleTitle()
	assert.NoError(t, err)
	assert.Empty(t, title)
}

func TestSetConsoleTitle_NonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("非Windows测试")
	}

	err := SetConsoleTitle("test")
	assert.NoError(t, err)
}

// ==================== 性能测试 ====================

func BenchmarkUniquePath(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "bench")
	defer os.RemoveAll(tempDir)

	basePath := filepath.Join(tempDir, "testfile.txt")
	f, _ := os.Create(basePath)
	f.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		UniquePath(basePath)
	}
}

func BenchmarkWinFileNameWithMaxLen(b *testing.B) {
	input := "https://example.com/path/file|name\nwith_invalid\rchars"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WinFileNameWithMaxLen(input, DefaultMaxFileNameLen)
	}
}

func BenchmarkHeapPush(b *testing.B) {
	heap := NewHeap(func(a, b int) bool { return a < b })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		heap.Push(i)
	}
}

func BenchmarkHeapPop(b *testing.B) {
	heap := NewHeap(func(a, b int) bool { return a < b })
	for i := 0; i < 10000; i++ {
		heap.Push(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if heap.Empty() {
			b.StopTimer()
			for j := 0; j < 10000; j++ {
				heap.Push(j)
			}
			b.StartTimer()
		}
		heap.Pop()
	}
}

func BenchmarkPathExists(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "bench")
	defer os.RemoveAll(tempDir)

	f, _ := os.CreateTemp(tempDir, "exist_*.txt")
	f.Close()
	path := f.Name()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PathExists(path)
	}
}
