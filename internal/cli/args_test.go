package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== UserArgs 测试 ====================

func TestUserArgs_Set(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		expectedScreens    []string
		expectEmptyScreens bool
		expectError        bool
	}{
		{
			name:               "带@的screenName",
			input:              "@twitteruser",
			expectedScreens:    []string{"twitteruser"},
			expectEmptyScreens: false,
			expectError:        false,
		},
		{
			name:               "不带@的screenName",
			input:              "twitteruser",
			expectedScreens:    []string{"twitteruser"},
			expectEmptyScreens: false,
			expectError:        false,
		},
		{
			name:               "带连字符的screenName",
			input:              "@user_name-123",
			expectedScreens:    nil,
			expectEmptyScreens: true,
			expectError:        true, // 连字符 - 不被允许
		},
		{
			name:               "带下划线的screenName",
			input:              "@user_name_123",
			expectedScreens:    []string{"user_name_123"},
			expectEmptyScreens: false,
			expectError:        false, // 下划线 _ 是允许的
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UserArgs{}
			err := u.Set(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectEmptyScreens {
					assert.Empty(t, u.ScreenName)
				} else if len(tt.expectedScreens) > 0 {
					assert.Equal(t, tt.expectedScreens, u.ScreenName)
				}
			}
		})
	}
}

func TestUserArgs_Set_Multiple(t *testing.T) {
	u := &UserArgs{}

	// 添加多个值
	err := u.Set("@user1")
	require.NoError(t, err)

	err = u.Set("user2")
	require.NoError(t, err)

	err = u.Set("@user3")
	require.NoError(t, err)

	assert.Equal(t, []string{"user1", "user2", "user3"}, u.ScreenName)
}

func TestUserArgs_Set_Duplicate(t *testing.T) {
	u := &UserArgs{}

	require.NoError(t, u.Set("@user1"))
	err := u.Set("user1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate screen name")
	assert.Equal(t, []string{"user1"}, u.ScreenName)
}

func TestUserArgs_String(t *testing.T) {
	tests := []struct {
		name     string
		args     UserArgs
		expected string
	}{
		{
			name:     "空值",
			args:     UserArgs{},
			expected: "screenNames=[]",
		},
		{
			name:     "只有screenName",
			args:     UserArgs{ScreenName: []string{"user1", "user2"}},
			expected: "screenNames=[user1 user2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== IntArgs 测试 ====================

func TestIntArgs_Set(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedIDs []uint64
		expectError bool
	}{
		{
			name:        "有效数字",
			input:       "12345",
			expectedIDs: []uint64{12345},
			expectError: false,
		},
		{
			name:        "零",
			input:       "0",
			expectedIDs: nil,
			expectError: true, // 0 是无效ID，必须大于0
		},
		{
			name:        "最大有效ID",
			input:       "9223372036854775807",
			expectedIDs: []uint64{MaxNumericID},
			expectError: false,
		},
		{
			name:        "超过最大有效ID",
			input:       "9223372036854775808",
			expectedIDs: nil,
			expectError: true,
		},
		{
			name:        "无效字符串",
			input:       "notanumber",
			expectedIDs: nil,
			expectError: true,
		},
		{
			name:        "带字母的混合",
			input:       "123abc",
			expectedIDs: nil,
			expectError: true,
		},
		{
			name:        "负数",
			input:       "-123",
			expectedIDs: nil,
			expectError: true,
		},
		{
			name:        "浮点数",
			input:       "123.45",
			expectedIDs: nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &IntArgs{}
			err := i.Set(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIDs, i.ID)
			}
		})
	}
}

func TestIntArgs_Set_Multiple(t *testing.T) {
	i := &IntArgs{}

	err := i.Set("100")
	require.NoError(t, err)

	err = i.Set("200")
	require.NoError(t, err)

	err = i.Set("300")
	require.NoError(t, err)

	assert.Equal(t, []uint64{100, 200, 300}, i.ID)
}

func TestIntArgs_Set_Duplicate(t *testing.T) {
	i := &IntArgs{}

	require.NoError(t, i.Set("100"))
	err := i.Set("100")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate ID")
	assert.Equal(t, []uint64{100}, i.ID)
}

func TestIntArgs_String(t *testing.T) {
	tests := []struct {
		name     string
		args     IntArgs
		expected string
	}{
		{
			name:     "空值",
			args:     IntArgs{},
			expected: "[]",
		},
		{
			name:     "单个值",
			args:     IntArgs{ID: []uint64{42}},
			expected: "[42]",
		},
		{
			name:     "多个值",
			args:     IntArgs{ID: []uint64{1, 2, 3, 4, 5}},
			expected: "[1 2 3 4 5]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== JsonFilePathsArgs 测试 ====================

func TestJsonFilePathsArgs_Set(t *testing.T) {
	j := &JsonFilePathsArgs{}

	tests := []struct {
		input    string
		expected []string
	}{
		{"/path/to/file1.json", []string{"/path/to/file1.json"}},
		{"/path/to/file2.json", []string{"/path/to/file1.json", "/path/to/file2.json"}},
		{"relative/path.json", []string{"/path/to/file1.json", "/path/to/file2.json", "relative/path.json"}},
	}

	for _, tt := range tests {
		err := j.Set(tt.input)
		require.NoError(t, err)
	}

	assert.Equal(t, []string{"/path/to/file1.json", "/path/to/file2.json", "relative/path.json"}, j.Paths)
}

func TestJsonFilePathsArgs_Set_Empty(t *testing.T) {
	j := &JsonFilePathsArgs{}

	err := j.Set("  ")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jsonfile path cannot be empty")
	assert.Empty(t, j.Paths)
}

func TestJsonFolderPathArgs_Set_Empty(t *testing.T) {
	j := &JsonFolderPathArgs{}

	err := j.Set("  ")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jsonfolder path cannot be empty")
	assert.Empty(t, j.Paths)
}

func TestJsonFolderPathArgs_SetAndString(t *testing.T) {
	j := &JsonFolderPathArgs{}

	require.NoError(t, j.Set("/path/to/folder1"))
	require.NoError(t, j.Set("relative/folder2"))

	assert.Equal(t, []string{"/path/to/folder1", "relative/folder2"}, j.GetPaths())
	assert.Equal(t, "/path/to/folder1,relative/folder2", j.String())
}

func TestJsonFilePathsArgs_String(t *testing.T) {
	tests := []struct {
		name     string
		args     JsonFilePathsArgs
		expected string
	}{
		{
			name:     "空值",
			args:     JsonFilePathsArgs{},
			expected: "",
		},
		{
			name:     "单个路径",
			args:     JsonFilePathsArgs{Paths: []string{"/path/to/file.json"}},
			expected: "/path/to/file.json",
		},
		{
			name:     "多个路径",
			args:     JsonFilePathsArgs{Paths: []string{"/path/1.json", "/path/2.json", "/path/3.json"}},
			expected: "/path/1.json,/path/2.json,/path/3.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJsonFilePathsArgs_GetPaths(t *testing.T) {
	tests := []struct {
		name     string
		args     JsonFilePathsArgs
		expected []string
	}{
		{
			name:     "空值",
			args:     JsonFilePathsArgs{},
			expected: nil,
		},
		{
			name:     "有路径",
			args:     JsonFilePathsArgs{Paths: []string{"a.json", "b.json"}},
			expected: []string{"a.json", "b.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.GetPaths()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== ParseArgs 测试 ====================

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		validate    func(t *testing.T, cfg *CLIConfig)
	}{
		{
			name:        "空参数",
			args:        []string{},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Empty(t, cfg.UsrArgs.ScreenName)
				assert.False(t, cfg.AutoFollow)
				assert.False(t, cfg.FollowMembers)
				assert.False(t, cfg.NoRetry)
			},
		},
		{
			name:        "单个用户",
			args:        []string{"-user", "twitteruser"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, []string{"twitteruser"}, cfg.UsrArgs.ScreenName)
			},
		},
		{
			name:        "多个用户",
			args:        []string{"-user", "user1", "-user", "@user2", "-user", "user3"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, []string{"user1", "user2", "user3"}, cfg.UsrArgs.ScreenName)
			},
		},
		{
			name:        "列表参数",
			args:        []string{"-list", "123", "-list", "456"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, []uint64{123, 456}, cfg.ListArgs.ID)
			},
		},
		{
			name:        "关注参数",
			args:        []string{"-foll", "@user"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, []string{"user"}, cfg.FollArgs.ScreenName)
			},
		},
		{
			name:        "profile用户",
			args:        []string{"-profile-user", "user1", "-profile-user", "user2"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, []string{"user1", "user2"}, cfg.ProfileUsers.ScreenName)
			},
		},
		{
			name:        "profile列表",
			args:        []string{"-profile-list", "789"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, []uint64{789}, cfg.ProfileList.ID)
			},
		},
		{
			name:        "JSON参数",
			args:        []string{"-jsonfile", "/path/1.json", "-jsonfile", "/path/2.json"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, []string{"/path/1.json", "/path/2.json"}, cfg.JsonFileArgs.Paths)
			},
		},
		{
			name:        "布尔标志",
			args:        []string{"-auto-follow", "-follow-members", "-no-retry", "-mark-downloaded", "-noprofile"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.True(t, cfg.AutoFollow)
				assert.True(t, cfg.FollowMembers)
				assert.True(t, cfg.NoRetry)
				assert.True(t, cfg.MarkDownloaded)
				assert.True(t, cfg.NoProfile)
			},
		},
		{
			name:        "mark-time参数",
			args:        []string{"-mark-time", "2024-01-01T00:00:00"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, "2024-01-01T00:00:00", cfg.MarkTime)
			},
		},
		{
			name:        "mark-time允许null",
			args:        []string{"-mark-time", "null"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, "null", cfg.MarkTime)
			},
		},
		{
			name:        "mark-time格式错误",
			args:        []string{"-mark-time", "2024-01-01"},
			expectError: true,
		},
		{
			name:        "组合参数",
			args:        []string{"-user", "user1", "-list", "123", "-auto-follow", "-no-retry"},
			expectError: false,
			validate: func(t *testing.T, cfg *CLIConfig) {
				assert.Equal(t, []string{"user1"}, cfg.UsrArgs.ScreenName)
				assert.Equal(t, []uint64{123}, cfg.ListArgs.ID)
				assert.True(t, cfg.AutoFollow)
				assert.True(t, cfg.NoRetry)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseArgs(tt.args)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestParseArgs_InvalidListID(t *testing.T) {
	// 无效的列表ID应该返回错误
	args := []string{"-list", "notanumber"}
	_, err := ParseArgs(args)
	assert.Error(t, err)
}

func TestParseArgs_TooLargeListID(t *testing.T) {
	_, err := ParseArgs([]string{"-list", "9223372036854775808"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ID")
}

func TestParseArgs_UnknownFlagUsesFriendlyError(t *testing.T) {
	_, err := ParseArgs([]string{"-unknown", "value"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown CLI flag -unknown")
}

func TestParseArgs_MissingFlagValueUsesFriendlyError(t *testing.T) {
	_, err := ParseArgs([]string{"-user"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CLI flag -user requires a value")
}

func TestCLIConfig_DefaultValues(t *testing.T) {
	cfg, err := ParseArgs([]string{})
	require.NoError(t, err)

	// 验证所有默认值
	assert.Empty(t, cfg.UsrArgs.ScreenName)
	assert.Empty(t, cfg.ListArgs.ID)
	assert.Empty(t, cfg.FollArgs.ScreenName)
	assert.Empty(t, cfg.ProfileUsers.ScreenName)
	assert.Empty(t, cfg.ProfileList.ID)
	assert.Empty(t, cfg.JsonFileArgs.Paths)
	assert.Empty(t, cfg.JsonFolderArgs.Paths)

	// 布尔值默认值
	assert.False(t, cfg.AutoFollow)
	assert.False(t, cfg.FollowMembers)
	assert.False(t, cfg.NoRetry)
	assert.False(t, cfg.MarkDownloaded)
	assert.Empty(t, cfg.MarkTime)
	assert.False(t, cfg.NoProfile)
}
