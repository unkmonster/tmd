package naming

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/unkmonster/tmd/internal/utils"
)

// ============================================================================
// TweetNaming 测试
// ============================================================================

func TestTweetNaming_LogFormat(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		tweetID  uint64
		creator  string
		expected string
	}{
		{
			name:     "normal case",
			text:     "比基尼",
			tweetID:  1355100264760393735,
			creator:  "吕布(QqiRru)",
			expected: "[吕布(QqiRru)] 比基尼_1355100264760393735",
		},
		{
			name:     "empty text",
			text:     "",
			tweetID:  123,
			creator:  "test",
			expected: "[test] tweet_123",
		},
		{
			name:     "special chars",
			text:     "hello\nworld",
			tweetID:  456,
			creator:  "user",
			expected: "[user] hello world_456",
		},
		{
			name:     "text with carriage return",
			text:     "hello\r\nworld",
			tweetID:  789,
			creator:  "user",
			expected: "[user] hello world_789",
		},
		{
			name:     "text with tab",
			text:     "hello\tworld",
			tweetID:  101,
			creator:  "user",
			expected: "[user] hello\tworld_101",
		},
		{
			name:     "text with URL",
			text:     "Check out https://example.com/path",
			tweetID:  202,
			creator:  "user",
			expected: "[user] Check out _202",
		},
		{
			name:     "text with windows invalid chars",
			text:     "test<>:\"/\\|?*file",
			tweetID:  303,
			creator:  "user",
			expected: "[user] testfile_303",
		},
		{
			name:     "unicode text",
			text:     "🎉🎊🎁 庆祝活动 🎂🎈",
			tweetID:  404,
			creator:  "用户",
			expected: "[用户] 🎉🎊🎁 庆祝活动 🎂🎈_404",
		},
		{
			name:     "very long text",
			text:     strings.Repeat("a", 200),
			tweetID:  505,
			creator:  "user",
			expected: "[user] " + strings.Repeat("a", MaxFileNameLen-len("_505")-ExtReserveLen) + "_505",
		},
		{
			name:     "max tweetID",
			text:     "test",
			tweetID:  ^uint64(0),
			creator:  "user",
			expected: "[user] test_18446744073709551615",
		},
		{
			name:     "zero tweetID",
			text:     "test",
			tweetID:  0,
			creator:  "user",
			expected: "[user] test_0",
		},
		{
			name:     "empty creator",
			text:     "test",
			tweetID:  606,
			creator:  "",
			expected: "[] test_606",
		},
		{
			name:     "creator with special chars",
			text:     "test",
			tweetID:  707,
			creator:  "user<name>",
			expected: "[user<name>] test_707",
		},
		{
			name:     "multiple newlines",
			text:     "line1\nline2\nline3",
			tweetID:  808,
			creator:  "user",
			expected: "[user] line1 line2 line3_808",
		},
		{
			name:     "text with only whitespace",
			text:     "   \n\t  ",
			tweetID:  909,
			creator:  "user",
			expected: "[user]     \t  _909",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tn := NewTweetNaming(tt.text, tt.tweetID, tt.creator)
			if got := tn.LogFormat(); got != tt.expected {
				t.Errorf("LogFormat() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTweetNaming_FileName(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		tweetID  uint64
		ext      string
		expected string
	}{
		{
			name:     "normal case with jpg",
			text:     "比基尼",
			tweetID:  1355100264760393735,
			ext:      ".jpg",
			expected: "比基尼_1355100264760393735.jpg",
		},
		{
			name:     "empty text with json",
			text:     "",
			tweetID:  123,
			ext:      ".json",
			expected: "tweet_123.json",
		},
		{
			name:     "with mp4 extension",
			text:     "video",
			tweetID:  456,
			ext:      ".mp4",
			expected: "video_456.mp4",
		},
		{
			name:     "with empty extension",
			text:     "file",
			tweetID:  789,
			ext:      "",
			expected: "file_789",
		},
		{
			name:     "with dot only extension",
			text:     "file",
			tweetID:  111,
			ext:      ".",
			expected: "file_111.",
		},
		{
			name:     "long extension",
			text:     "file",
			tweetID:  222,
			ext:      ".tar.gz",
			expected: "file_222.tar.gz",
		},
		{
			name:     "text with path separator",
			text:     "path/to/file",
			tweetID:  333,
			ext:      ".txt",
			expected: "pathtofile_333.txt",
		},
		{
			name:     "text with multiple dots",
			text:     "file.name.test",
			tweetID:  444,
			ext:      ".ext",
			expected: "file.name.test_444.ext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tn := NewTweetNaming(tt.text, tt.tweetID, "creator")
			if got := tn.FileName(tt.ext); got != tt.expected {
				t.Errorf("FileName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTweetNaming_FilePath(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		tweetID    uint64
		dir        string
		ext        string
		wantPrefix string
		wantSuffix string
		wantErr    bool
	}{
		{
			name:       "normal path",
			text:       "test",
			tweetID:    123,
			dir:        "downloads",
			ext:        ".jpg",
			wantPrefix: filepath.Join("downloads", "test_123"),
			wantSuffix: ".jpg",
			wantErr:    false,
		},
		{
			name:       "nested directory",
			text:       "test",
			tweetID:    456,
			dir:        filepath.Join("downloads", "user", "tweets"),
			ext:        ".mp4",
			wantPrefix: filepath.Join("downloads", "user", "tweets", "test_456"),
			wantSuffix: ".mp4",
			wantErr:    false,
		},
		{
			name:       "absolute path",
			text:       "test",
			tweetID:    789,
			dir:        filepath.Join("C:", "downloads"),
			ext:        ".json",
			wantPrefix: filepath.Join("C:", "downloads", "test_789"),
			wantSuffix: ".json",
			wantErr:    false,
		},
		{
			name:       "empty directory",
			text:       "test",
			tweetID:    101,
			dir:        "",
			ext:        ".txt",
			wantPrefix: "test_101",
			wantSuffix: ".txt",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tn := NewTweetNaming(tt.text, tt.tweetID, "creator")
			got, err := tn.FilePath(tt.dir, tt.ext)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("FilePath() = %q, want prefix %q", got, tt.wantPrefix)
			}
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("FilePath() = %q, want suffix %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestTweetNaming_FilePathWithResolver(t *testing.T) {
	tempDir := t.TempDir()
	tn := NewTweetNaming("test", 123, "creator")
	resolver := utils.NewUniquePathResolver()

	first, err := tn.FilePathWithResolver(tempDir, ".jpg", resolver)
	if err != nil {
		t.Fatalf("FilePathWithResolver() error = %v", err)
	}
	second, err := tn.FilePathWithResolver(tempDir, ".jpg", resolver)
	if err != nil {
		t.Fatalf("FilePathWithResolver() error = %v", err)
	}

	if filepath.Base(first) != "test_123.jpg" {
		t.Fatalf("first path = %q, want %q", filepath.Base(first), "test_123.jpg")
	}
	if filepath.Base(second) != "test_123(1).jpg" {
		t.Fatalf("second path = %q, want %q", filepath.Base(second), "test_123(1).jpg")
	}
}

func TestTweetNaming_baseName_Truncation(t *testing.T) {
	// 测试文件名截断逻辑
	tests := []struct {
		name         string
		text         string
		tweetID      uint64
		maxLen       int
		wantContains string
	}{
		{
			name:         "short text no truncation",
			text:         "short",
			tweetID:      123,
			maxLen:       100,
			wantContains: "short_123",
		},
		{
			name:         "long text with truncation",
			text:         strings.Repeat("a", 200),
			tweetID:      123456789012345,
			maxLen:       50,
			wantContains: "_123456789012345",
		},
		{
			name:         "unicode text truncation",
			text:         strings.Repeat("中", 100),
			tweetID:      999,
			maxLen:       30,
			wantContains: "_999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 临时修改 MaxFileNameLen
			oldMaxLen := MaxFileNameLen
			MaxFileNameLen = tt.maxLen
			defer func() { MaxFileNameLen = oldMaxLen }()

			tn := NewTweetNaming(tt.text, tt.tweetID, "creator")
			got := tn.FileName(".ext")
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("FileName() = %q, should contain %q", got, tt.wantContains)
			}
			if len(got) > tt.maxLen {
				t.Errorf("FileName() length = %d, exceeds maxLen %d", len(got), tt.maxLen)
			}
		})
	}
}

// ============================================================================
// UserNaming 测试
// ============================================================================

func TestUserNaming_SanitizedTitle(t *testing.T) {
	tests := []struct {
		name       string
		userName   string
		screenName string
		expected   string
	}{
		{
			name:       "normal case",
			userName:   "吕布",
			screenName: "QqiRru",
			expected:   "吕布(QqiRru)",
		},
		{
			name:       "empty userName",
			userName:   "",
			screenName: "screen",
			expected:   "(screen)",
		},
		{
			name:       "empty screenName",
			userName:   "User",
			screenName: "",
			expected:   "User()",
		},
		{
			name:       "both empty",
			userName:   "",
			screenName: "",
			expected:   "()",
		},
		{
			name:       "with special chars",
			userName:   "User<Name>",
			screenName: "screen:name",
			expected:   "UserName(screenname)",
		},
		{
			name:       "with newlines",
			userName:   "User\nName",
			screenName: "screen\r\n",
			expected:   "User Name(screen )",
		},
		{
			name:       "with URL",
			userName:   "User https://x.com/name",
			screenName: "screen",
			expected:   "User (screen)",
		},
		{
			name:       "unicode names",
			userName:   "🎉用户🎊",
			screenName: "user_123",
			expected:   "🎉用户🎊(user_123)",
		},
		{
			name:       "very long names",
			userName:   strings.Repeat("a", 100),
			screenName: strings.Repeat("b", 100),
			expected:   strings.Repeat("a", 100) + "(" + strings.Repeat("b", 57),
		},
		{
			name:       "with path separators",
			userName:   "user/name",
			screenName: "screen\\name",
			expected:   "username(screenname)",
		},
		{
			name:       "with quotes",
			userName:   `User"Quoted"`,
			screenName: "screen'test'",
			expected:   "UserQuoted(screen'test')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			un := NewUserNaming(tt.userName, tt.screenName)
			if got := un.SanitizedTitle(); got != tt.expected {
				t.Errorf("SanitizedTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestUserNaming_Truncation(t *testing.T) {
	// 测试长名称截断
	oldMaxLen := MaxFileNameLen
	MaxFileNameLen = 30
	defer func() { MaxFileNameLen = oldMaxLen }()

	longName := strings.Repeat("a", 50)
	longScreen := strings.Repeat("b", 50)
	un := NewUserNaming(longName, longScreen)

	got := un.SanitizedTitle()
	if len(got) > MaxFileNameLen {
		t.Errorf("SanitizedTitle() length = %d, exceeds maxLen %d", len(got), MaxFileNameLen)
	}
}

// ============================================================================
// ListNaming 测试
// ============================================================================

type mockListBase struct {
	id    int64
	title string
}

func (m *mockListBase) GetId() int64  { return m.id }
func (m *mockListBase) Title() string { return m.title }

func TestListNaming_SanitizedTitle(t *testing.T) {
	tests := []struct {
		name     string
		listBase *mockListBase
		expected string
	}{
		{
			name:     "normal case",
			listBase: &mockListBase{id: 9876543210, title: "Test List(9876543210)"},
			expected: "Test List(9876543210)",
		},
		{
			name:     "empty title",
			listBase: &mockListBase{id: 1, title: ""},
			expected: "",
		},
		{
			name:     "title with special chars",
			listBase: &mockListBase{id: 2, title: "List<Name>: Test"},
			expected: "ListName Test",
		},
		{
			name:     "title with newlines",
			listBase: &mockListBase{id: 3, title: "Line1\nLine2\r\nLine3"},
			expected: "Line1 Line2 Line3",
		},
		{
			name:     "title with URL",
			listBase: &mockListBase{id: 4, title: "My List https://x.com/i/lists/123"},
			expected: "My List ",
		},
		{
			name:     "unicode title",
			listBase: &mockListBase{id: 5, title: "我的列表 📝📋"},
			expected: "我的列表 📝📋",
		},
		{
			name:     "title with path separators",
			listBase: &mockListBase{id: 6, title: "path/to/list"},
			expected: "pathtolist",
		},
		{
			name:     "title with quotes",
			listBase: &mockListBase{id: 7, title: `"Quoted" List`},
			expected: "Quoted List",
		},
		{
			name:     "title with asterisk",
			listBase: &mockListBase{id: 8, title: "List *important*"},
			expected: "List important",
		},
		{
			name:     "title with question mark",
			listBase: &mockListBase{id: 9, title: "What? Why?"},
			expected: "What Why",
		},
		{
			name:     "very long title",
			listBase: &mockListBase{id: 10, title: strings.Repeat("a", 200)},
			expected: strings.Repeat("a", MaxFileNameLen),
		},
		{
			name:     "negative id",
			listBase: &mockListBase{id: -1, title: "Test List"},
			expected: "Test List",
		},
		{
			name:     "zero id",
			listBase: &mockListBase{id: 0, title: "Test List"},
			expected: "Test List",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln := NewListNamingFromBase(tt.listBase)
			if got := ln.SanitizedTitle(); got != tt.expected {
				t.Errorf("SanitizedTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestListNaming_Truncation(t *testing.T) {
	oldMaxLen := MaxFileNameLen
	MaxFileNameLen = 20
	defer func() { MaxFileNameLen = oldMaxLen }()

	longTitle := strings.Repeat("x", 100)
	ln := NewListNamingFromBase(&mockListBase{id: 1, title: longTitle})

	got := ln.SanitizedTitle()
	if len(got) > MaxFileNameLen {
		t.Errorf("SanitizedTitle() length = %d, exceeds maxLen %d", len(got), MaxFileNameLen)
	}
	if got != strings.Repeat("x", MaxFileNameLen) {
		t.Errorf("SanitizedTitle() = %q, want %q", got, strings.Repeat("x", MaxFileNameLen))
	}
}

// ============================================================================
// 边界条件测试
// ============================================================================

func TestMaxFileNameLenModification(t *testing.T) {
	// 测试修改 MaxFileNameLen 后的行为
	originalLen := MaxFileNameLen
	defer func() { MaxFileNameLen = originalLen }()

	// 设置一个较小的限制
	MaxFileNameLen = 30

	tn := NewTweetNaming("this is a very long text that should be truncated", 12345, "creator")
	fileName := tn.FileName(".jpg")

	if len(fileName) > MaxFileNameLen {
		t.Errorf("FileName length %d exceeds MaxFileNameLen %d", len(fileName), MaxFileNameLen)
	}
}

func TestExtReserveLen(t *testing.T) {
	// 验证 ExtReserveLen 常量
	if ExtReserveLen != 8 {
		t.Errorf("ExtReserveLen = %d, want 8", ExtReserveLen)
	}
}

func TestDefaultMaxFileNameLen(t *testing.T) {
	// 验证默认值
	fromUtils := 158 // utils.DefaultMaxFileNameLen
	if MaxFileNameLen != fromUtils {
		t.Logf("Note: MaxFileNameLen = %d (from utils)", MaxFileNameLen)
	}
}

// ============================================================================
// 并发安全测试
// ============================================================================

func TestTweetNaming_Concurrent(t *testing.T) {
	tn := NewTweetNaming("test tweet", 12345, "testuser")

	// 并发调用各种方法
	done := make(chan bool, 3)

	go func() {
		for i := 0; i < 100; i++ {
			_ = tn.LogFormat()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = tn.FileName(".jpg")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_, _ = tn.FilePath("temp", ".txt")
		}
		done <- true
	}()

	for i := 0; i < 3; i++ {
		<-done
	}
}

func TestUserNaming_Concurrent(t *testing.T) {
	un := NewUserNaming("User", "screen")

	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 100; i++ {
			_ = un.SanitizedTitle()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = un.SanitizedTitle()
		}
		done <- true
	}()

	for i := 0; i < 2; i++ {
		<-done
	}
}

func TestListNaming_Concurrent(t *testing.T) {
	ln := NewListNamingFromBase(&mockListBase{id: 1, title: "Test List"})

	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 100; i++ {
			_ = ln.SanitizedTitle()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = ln.SanitizedTitle()
		}
		done <- true
	}()

	for i := 0; i < 2; i++ {
		<-done
	}
}

// ============================================================================
// 集成测试
// ============================================================================

func TestNamingIntegration(t *testing.T) {
	// 模拟实际使用场景
	tweet := NewTweetNaming(
		"Check out this photo! 📸 #photography",
		1355100264760393735,
		"PhotoMaster(@photom)",
	)

	// 验证 LogFormat
	logFormat := tweet.LogFormat()
	if logFormat == "" {
		t.Error("LogFormat() returned empty string")
	}
	t.Logf("LogFormat: %s", logFormat)

	// 验证 FileName
	fileName := tweet.FileName(".jpg")
	if !strings.HasSuffix(fileName, ".jpg") {
		t.Errorf("FileName() = %q, should end with .jpg", fileName)
	}
	t.Logf("FileName: %s", fileName)

	// 验证 FilePath
	path, err := tweet.FilePath(filepath.Join("downloads", "tweets"), ".jpg")
	if err != nil {
		t.Errorf("FilePath() error = %v", err)
	}
	if !strings.HasSuffix(path, ".jpg") {
		t.Errorf("FilePath() = %q, should end with .jpg", path)
	}
	t.Logf("FilePath: %s", path)
}

func TestUserAndTweetNamingIntegration(t *testing.T) {
	// 模拟创建用户和推文命名的场景
	user := NewUserNaming("张三", "zhangsan123")
	userTitle := user.SanitizedTitle()

	tweet := NewTweetNaming("Hello World!", 9876543210, userTitle)

	logFormat := tweet.LogFormat()
	expectedCreator := "张三(zhangsan123)"
	if !strings.Contains(logFormat, expectedCreator) {
		t.Errorf("LogFormat() = %q, should contain %q", logFormat, expectedCreator)
	}
}

// ============================================================================
// 性能基准测试
// ============================================================================

func BenchmarkTweetNaming_LogFormat(b *testing.B) {
	tn := NewTweetNaming("Test tweet content here", 123456789, "testuser")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tn.LogFormat()
	}
}

func BenchmarkTweetNaming_FileName(b *testing.B) {
	tn := NewTweetNaming("Test tweet content here", 123456789, "testuser")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tn.FileName(".jpg")
	}
}

func BenchmarkTweetNaming_FilePath(b *testing.B) {
	tn := NewTweetNaming("Test tweet content here", 123456789, "testuser")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tn.FilePath("downloads", ".jpg")
	}
}

func BenchmarkUserNaming_SanitizedTitle(b *testing.B) {
	un := NewUserNaming("User Name", "screen_name")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = un.SanitizedTitle()
	}
}

func BenchmarkListNaming_SanitizedTitle(b *testing.B) {
	ln := NewListNamingFromBase(&mockListBase{id: 1, title: "My Test List"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ln.SanitizedTitle()
	}
}

func BenchmarkNewTweetNaming(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewTweetNaming("Test content", 123456789, "user")
	}
}

func BenchmarkNewUserNaming(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewUserNaming("Name", "screen")
	}
}
