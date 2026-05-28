package path

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== NewStorePath 测试 ====================

func TestNewStorePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_path_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		root        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "正常创建存储路径",
			root:    filepath.Join(tempDir, "valid_root"),
			wantErr: false,
		},
		{
			name:    "嵌套目录创建",
			root:    filepath.Join(tempDir, "level1", "level2", "level3"),
			wantErr: false,
		},
		{
			name:    "绝对路径",
			root:    tempDir,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := NewStorePath(tt.root)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, sp)
			expectedRoot := mustNormalizeRoot(t, tt.root)

			// 验证路径结构
			assert.Equal(t, expectedRoot, sp.Root)
			assert.Equal(t, filepath.Join(expectedRoot, "users"), sp.Users)
			assert.Equal(t, filepath.Join(expectedRoot, ".data"), sp.Data)
			assert.Equal(t, filepath.Join(expectedRoot, ".data", "foo.db"), sp.DB)
			assert.Equal(t, filepath.Join(expectedRoot, ".data", "errors.json"), sp.ErrorsPath)
			assert.Equal(t, filepath.Join(expectedRoot, ".data", "json_errors.json"), sp.JSONErrorsPath)

			// 验证目录是否实际创建
			assert.DirExists(t, sp.Root)
			assert.DirExists(t, sp.Users)
			assert.DirExists(t, sp.Data)
		})
	}
}

func TestNewStorePath_PathStructure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_path_structure_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	root := filepath.Join(tempDir, "test_store")
	sp, err := NewStorePath(root)
	require.NoError(t, err)
	expectedRoot := mustNormalizeRoot(t, root)

	// 验证路径拼接的正确性
	t.Run("验证路径拼接", func(t *testing.T) {
		assert.Equal(t, expectedRoot, sp.Root)
		assert.Equal(t, filepath.Join(expectedRoot, "users"), sp.Users)
		assert.Equal(t, filepath.Join(expectedRoot, ".data"), sp.Data)
		assert.Equal(t, filepath.Join(expectedRoot, ".data", "foo.db"), sp.DB)
		assert.Equal(t, filepath.Join(expectedRoot, ".data", "errors.json"), sp.ErrorsPath)
		assert.Equal(t, filepath.Join(expectedRoot, ".data", "json_errors.json"), sp.JSONErrorsPath)
	})

	// 验证目录权限和存在性
	t.Run("验证目录存在性", func(t *testing.T) {
		info, err := os.Stat(sp.Root)
		require.NoError(t, err)
		assert.True(t, info.IsDir())

		info, err = os.Stat(sp.Users)
		require.NoError(t, err)
		assert.True(t, info.IsDir())

		info, err = os.Stat(sp.Data)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func TestNewStorePath_ExistingDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_path_existing_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 预先创建目录结构
	root := filepath.Join(tempDir, "existing_store")
	require.NoError(t, os.MkdirAll(root, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "users"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".data"), 0755))

	// 在已存在的目录上创建 StorePath
	sp, err := NewStorePath(root)
	require.NoError(t, err)
	require.NotNil(t, sp)
	expectedRoot := mustNormalizeRoot(t, root)

	// 验证路径仍然正确
	assert.Equal(t, expectedRoot, sp.Root)
	assert.DirExists(t, sp.Root)
	assert.DirExists(t, sp.Users)
	assert.DirExists(t, sp.Data)
}

func TestNewStorePath_InvalidPath(t *testing.T) {
	tests := []struct {
		name        string
		root        string
		errContains string
	}{
		{
			name:        "空路径",
			root:        "",
			errContains: "root path cannot be empty",
		},
		{
			name:        "无效字符路径",
			root:        "/\x00invalid",
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := NewStorePath(tt.root)
			assert.Error(t, err)
			assert.Nil(t, sp)
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}
		})
	}
}

// ==================== StorePath 字段访问测试 ====================

func TestStorePath_FieldAccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_path_field_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	root := filepath.Join(tempDir, "field_test")
	sp, err := NewStorePath(root)
	require.NoError(t, err)
	expectedRoot := mustNormalizeRoot(t, root)

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{
			name:     "Root 字段",
			got:      sp.Root,
			expected: expectedRoot,
		},
		{
			name:     "Users 字段",
			got:      sp.Users,
			expected: filepath.Join(expectedRoot, "users"),
		},
		{
			name:     "Data 字段",
			got:      sp.Data,
			expected: filepath.Join(expectedRoot, ".data"),
		},
		{
			name:     "DB 字段",
			got:      sp.DB,
			expected: filepath.Join(expectedRoot, ".data", "foo.db"),
		},
		{
			name:     "ErrorsPath 字段",
			got:      sp.ErrorsPath,
			expected: filepath.Join(expectedRoot, ".data", "errors.json"),
		},
		{
			name:     "JSONErrorsPath 字段",
			got:      sp.JSONErrorsPath,
			expected: filepath.Join(expectedRoot, ".data", "json_errors.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.got)
		})
	}
}

// ==================== StorePath 并发测试 ====================

func TestNewStorePath_Concurrent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_path_concurrent_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 并发创建相同的目录
	concurrency := 10
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(index int) {
			defer func() { done <- true }()
			root := filepath.Join(tempDir, "concurrent_store")
			sp, err := NewStorePath(root)
			if err != nil {
				errors <- err
				return
			}
			if sp == nil {
				errors <- assert.AnError
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < concurrency; i++ {
		<-done
	}
	close(errors)

	// 检查是否有错误
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}

	// 并发创建相同目录应该都能成功（os.MkdirAll 是幂等的）
	assert.Equal(t, 0, errorCount, "并发创建目录不应该产生错误")

	// 验证目录存在
	assert.DirExists(t, filepath.Join(tempDir, "concurrent_store"))
}

// ==================== StorePath 边界情况测试 ====================

func TestNewStorePath_EdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_path_edge_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name    string
		root    string
		wantErr bool
	}{
		{
			name:    "包含空格的路径",
			root:    filepath.Join(tempDir, "path with spaces"),
			wantErr: false,
		},
		{
			name:    "包含中文字符的路径",
			root:    filepath.Join(tempDir, "中文路径测试"),
			wantErr: false,
		},
		{
			name:    "包含特殊字符的路径",
			root:    filepath.Join(tempDir, "path-with_special.chars"),
			wantErr: false,
		},
		{
			name:    "非常长的路径",
			root:    filepath.Join(tempDir, "very", "long", "path", "with", "many", "nested", "directories", "that", "go", "deep"),
			wantErr: false,
		},
		{
			name:    "单字符路径",
			root:    filepath.Join(tempDir, "a"),
			wantErr: false,
		},
		{
			name:    "以点开头的路径",
			root:    filepath.Join(tempDir, ".hidden"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := NewStorePath(tt.root)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, sp)

			// 验证目录结构
			assert.DirExists(t, sp.Root)
			assert.DirExists(t, sp.Users)
			assert.DirExists(t, sp.Data)
		})
	}
}

// ==================== StorePath 路径分隔符测试 ====================

func TestNewStorePath_PathSeparators(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_path_sep_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	root := filepath.Join(tempDir, "sep_test")
	sp, err := NewStorePath(root)
	require.NoError(t, err)

	t.Run("使用 filepath.Join 确保正确的分隔符", func(t *testing.T) {
		// 验证路径使用了当前系统的正确分隔符
		sep := string(filepath.Separator)
		assert.Contains(t, sp.Root, sep)
		assert.Contains(t, sp.Users, sep)
		assert.Contains(t, sp.Data, sep)

		// 验证路径是有效的
		assert.True(t, filepath.IsAbs(sp.Root))
	})
}

func TestNewStorePath_NormalizesRoot(t *testing.T) {
	tempDir := t.TempDir()
	input := filepath.Join(tempDir, "base", ".", "child", "..", "target") + string(filepath.Separator)
	expectedRoot := mustNormalizeRoot(t, input)

	sp, err := NewStorePath(input)

	require.NoError(t, err)
	assert.Equal(t, expectedRoot, sp.Root)
	assert.Equal(t, filepath.Join(expectedRoot, "users"), sp.Users)
	assert.Equal(t, filepath.Join(expectedRoot, ".data"), sp.Data)
	assert.True(t, filepath.IsAbs(sp.Root))
	assert.NotContains(t, sp.Root, string(filepath.Separator)+"."+string(filepath.Separator))
}

func TestNewStorePath_RelativeRootBecomesAbsolute(t *testing.T) {
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(oldWD))
	})

	sp, err := NewStorePath(filepath.Join(".", "downloads", "..", "store"))

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, "store"), sp.Root)
	assert.DirExists(t, filepath.Join(tempDir, "store", "users"))
	assert.DirExists(t, filepath.Join(tempDir, "store", ".data"))
}

// ==================== StorePath 重复创建测试 ====================

func TestNewStorePath_MultipleCalls(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_path_multiple_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	root := filepath.Join(tempDir, "multiple_calls")

	// 第一次创建
	sp1, err := NewStorePath(root)
	require.NoError(t, err)
	require.NotNil(t, sp1)

	// 第二次创建相同路径
	sp2, err := NewStorePath(root)
	require.NoError(t, err)
	require.NotNil(t, sp2)

	// 验证两次创建的路径一致
	assert.Equal(t, sp1.Root, sp2.Root)
	assert.Equal(t, sp1.Users, sp2.Users)
	assert.Equal(t, sp1.Data, sp2.Data)
	assert.Equal(t, sp1.DB, sp2.DB)
	assert.Equal(t, sp1.ErrorsPath, sp2.ErrorsPath)
	assert.Equal(t, sp1.JSONErrorsPath, sp2.JSONErrorsPath)
}

// ==================== StorePath 权限测试 ====================

func TestNewStorePath_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("跳过 Windows 权限测试")
	}

	tempDir, err := os.MkdirTemp("", "store_path_perm_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	root := filepath.Join(tempDir, "perm_test")
	sp, err := NewStorePath(root)
	require.NoError(t, err)

	t.Run("验证目录权限", func(t *testing.T) {
		info, err := os.Stat(sp.Root)
		require.NoError(t, err)
		// 0755 权限
		assert.Equal(t, os.FileMode(0755)|os.ModeDir, info.Mode()&os.ModePerm|os.ModeDir)

		info, err = os.Stat(sp.Users)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0755)|os.ModeDir, info.Mode()&os.ModePerm|os.ModeDir)

		info, err = os.Stat(sp.Data)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0755)|os.ModeDir, info.Mode()&os.ModePerm|os.ModeDir)
	})
}

// ==================== Benchmark 测试 ====================

func BenchmarkNewStorePath(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "store_path_bench")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		root := filepath.Join(tempDir, "bench", string(rune(i)))
		_, _ = NewStorePath(root)
	}
}

func BenchmarkNewStorePath_Existing(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "store_path_bench_existing")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	root := filepath.Join(tempDir, "existing")
	_, err = NewStorePath(root)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewStorePath(root)
	}
}

func mustNormalizeRoot(t *testing.T, root string) string {
	t.Helper()

	normalized, err := normalizeRoot(root)
	require.NoError(t, err)
	return normalized
}
