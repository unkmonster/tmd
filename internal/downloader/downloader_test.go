package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

// =============================================================================
// FileWriter.Write() 测试
// =============================================================================

func TestFileWriter_Write_Normal(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "filewriter_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建 FileWriter
	fw := NewFileWriter(nil)

	// 准备测试数据
	testData := []byte("hello world")
	testPath := filepath.Join(tempDir, "test.txt")

	// 执行写入
	req := WriteRequest{
		Path: testPath,
		Data: testData,
		Options: WriteOptions{
			CreateVersion: false,
			SkipUnchanged: false,
		},
	}
	result, err := fw.Write(req)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	// 验证结果
	if !result.Success {
		t.Error("期望 Success=true, 实际 false")
	}
	if result.Skipped {
		t.Error("期望 Skipped=false, 实际 true")
	}
	if result.NewSize != int64(len(testData)) {
		t.Errorf("期望 NewSize=%d, 实际 %d", len(testData), result.NewSize)
	}

	// 验证文件内容
	content, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("期望内容 %q, 实际 %q", string(testData), string(content))
	}
}

func TestFileWriter_Write_SkipUnchanged(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "filewriter_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建 FileWriter
	fw := NewFileWriter(nil)

	// 准备测试数据
	testData := []byte("hello world")
	testPath := filepath.Join(tempDir, "test.txt")

	// 第一次写入
	req := WriteRequest{
		Path: testPath,
		Data: testData,
		Options: WriteOptions{
			SkipUnchanged: true,
		},
	}
	result1, err := fw.Write(req)
	if err != nil {
		t.Fatalf("第一次写入失败: %v", err)
	}
	if result1.Skipped {
		t.Error("第一次写入不应该被跳过")
	}

	// 第二次写入相同内容（应该跳过）
	result2, err := fw.Write(req)
	if err != nil {
		t.Fatalf("第二次写入失败: %v", err)
	}

	// 验证跳过
	if !result2.Skipped {
		t.Error("期望第二次写入被跳过, 实际未跳过")
	}
	if !result2.Success {
		t.Error("跳过的写入仍应标记为成功")
	}

	// 第三次写入不同内容（不应该跳过）
	newData := []byte("hello world 2")
	req.Data = newData
	result3, err := fw.Write(req)
	if err != nil {
		t.Fatalf("第三次写入失败: %v", err)
	}
	if result3.Skipped {
		t.Error("写入不同内容不应该被跳过")
	}
}

func TestFileWriter_Write_CreateVersion(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "filewriter_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建 VersionManager 和 FileWriter
	vm := NewVersionManagerWithWriter(".versions", nil)
	fw := NewFileWriter(vm)

	// 准备测试数据
	testPath := filepath.Join(tempDir, "test.txt")
	oldData := []byte("old content")
	newData := []byte("new content")

	// 第一次写入（创建文件，不需要创建版本）
	req := WriteRequest{
		Path: testPath,
		Data: oldData,
		Options: WriteOptions{
			CreateVersion: true,
		},
	}
	_, err = fw.Write(req)
	if err != nil {
		t.Fatalf("第一次写入失败: %v", err)
	}

	// 第二次写入（应该创建版本）
	req.Data = newData
	result, err := fw.Write(req)
	if err != nil {
		t.Fatalf("第二次写入失败: %v", err)
	}
	if !result.Success {
		t.Error("写入应该成功")
	}

	// 验证版本文件存在
	versionsDir := filepath.Join(tempDir, ".versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		t.Fatalf("读取版本目录失败: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("期望存在版本文件, 但未找到")
	}

	// 验证版本文件内容是旧内容
	versionPath := filepath.Join(versionsDir, entries[0].Name())
	versionContent, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatalf("读取版本文件失败: %v", err)
	}
	if string(versionContent) != string(oldData) {
		t.Errorf("版本文件内容应为 %q, 实际 %q", string(oldData), string(versionContent))
	}

	// 验证当前文件内容是新内容
	currentContent, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("读取当前文件失败: %v", err)
	}
	if string(currentContent) != string(newData) {
		t.Errorf("当前文件内容应为 %q, 实际 %q", string(newData), string(currentContent))
	}
}

func TestFileWriter_Write_SetModTime(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "filewriter_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建 FileWriter
	fw := NewFileWriter(nil)

	// 准备测试数据
	testData := []byte("hello world")
	testPath := filepath.Join(tempDir, "test.txt")

	// 设置特定的修改时间
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// 执行写入
	req := WriteRequest{
		Path: testPath,
		Data: testData,
		Options: WriteOptions{
			ModTime: &modTime,
		},
	}
	_, err = fw.Write(req)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	// 验证修改时间
	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("获取文件信息失败: %v", err)
	}

	// 比较修改时间（允许1秒误差，因为文件系统精度可能不同）
	actualModTime := info.ModTime().UTC()
	diff := actualModTime.Sub(modTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("期望修改时间 %v, 实际 %v", modTime, actualModTime)
	}
}

func TestFileWriter_Write_NonExistentDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)

	testData := []byte("hello world")
	nonExistentDir := filepath.Join(tempDir, "nonexistent", "nested", "dir")
	testPath := filepath.Join(nonExistentDir, "test.txt")

	req := WriteRequest{
		Path: testPath,
		Data: testData,
	}
	_, err = fw.Write(req)
	if err != nil {
		t.Fatalf("写入到不存在的目录失败: %v", err)
	}

	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(data) != string(testData) {
		t.Errorf("期望内容 %q, 实际 %q", string(testData), string(data))
	}
}

// =============================================================================
// VersionManager.CreateVersion() 测试
// =============================================================================

func TestVersionManager_CreateVersion(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "versionmanager_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建 VersionManager
	vm := NewVersionManagerWithWriter(".versions", nil)

	// 创建源文件
	sourcePath := filepath.Join(tempDir, "document.txt")
	sourceData := []byte("source content")
	if err := os.WriteFile(sourcePath, sourceData, 0644); err != nil {
		t.Fatalf("创建源文件失败: %v", err)
	}

	// 创建版本
	versionPath, err := vm.CreateVersion(sourcePath)
	if err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}

	// 验证版本路径
	expectedDir := filepath.Join(tempDir, ".versions")
	if !strings.HasPrefix(versionPath, expectedDir) {
		t.Errorf("版本路径应在 %s 目录下, 实际: %s", expectedDir, versionPath)
	}

	// 验证版本文件命名格式: document_20060102_150405_NNN.txt
	versionFilename := filepath.Base(versionPath)
	pattern := `^document_\d{8}_\d{6}_\d{1,3}\.txt$`
	matched, err := regexp.MatchString(pattern, versionFilename)
	if err != nil {
		t.Fatalf("正则匹配失败: %v", err)
	}
	if !matched {
		t.Errorf("版本文件名格式不正确, 期望匹配 %s, 实际: %s", pattern, versionFilename)
	}

	// 验证版本文件内容
	versionContent, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatalf("读取版本文件失败: %v", err)
	}
	if string(versionContent) != string(sourceData) {
		t.Errorf("版本文件内容应为 %q, 实际 %q", string(sourceData), string(versionContent))
	}

	// 验证版本目录存在
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Error("版本目录应该存在")
	}
}

func TestVersionManager_CreateVersion_EmptyPath(t *testing.T) {
	vm := NewVersionManagerWithWriter(".versions", nil)

	_, err := vm.CreateVersion("")
	if err == nil {
		t.Error("期望空路径返回错误，但未返回")
	}
}

func TestVersionManager_CreateVersion_DotFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "versionmanager_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	vm := NewVersionManagerWithWriter(".versions", nil)

	sourcePath := filepath.Join(tempDir, ".gitignore")
	sourceData := []byte("gitignore content")
	if err := os.WriteFile(sourcePath, sourceData, 0644); err != nil {
		t.Fatalf("创建源文件失败: %v", err)
	}

	versionPath, err := vm.CreateVersion(sourcePath)
	if err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}

	versionFilename := filepath.Base(versionPath)
	pattern := `^_unknown_\d{8}_\d{6}_\d{1,3}\.gitignore$`
	matched, err := regexp.MatchString(pattern, versionFilename)
	if err != nil {
		t.Fatalf("正则匹配失败: %v", err)
	}
	if !matched {
		t.Errorf("点开头的文件名格式不正确, 期望匹配 %s, 实际: %s", pattern, versionFilename)
	}
}

// =============================================================================
// Downloader.Download() 测试
// =============================================================================

func TestDownloader_Download_Normal(t *testing.T) {
	// 创建模拟 HTTP 服务器
	testData := []byte("test file content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer server.Close()

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "downloader_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建 FileWriter 和 Downloader
	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	// 准备下载请求
	destPath := filepath.Join(tempDir, "downloaded.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/test.txt",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	// 执行下载
	result, err := dl.Download(req)
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}

	// 验证结果
	if !result.Success {
		t.Error("期望 Success=true")
	}
	if result.Skipped {
		t.Error("期望 Skipped=false")
	}
	if result.FilePath != destPath {
		t.Errorf("期望 FilePath=%s, 实际 %s", destPath, result.FilePath)
	}
	if result.FileSize != int64(len(testData)) {
		t.Errorf("期望 FileSize=%d, 实际 %d", len(testData), result.FileSize)
	}

	// 验证文件内容
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("期望内容 %q, 实际 %q", string(testData), string(content))
	}
}

func TestDownloader_Download_Error(t *testing.T) {
	// 创建模拟 HTTP 服务器（返回错误状态码）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "downloader_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建 FileWriter 和 Downloader
	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	// 准备下载请求
	destPath := filepath.Join(tempDir, "downloaded.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/notfound.txt",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	// 执行下载（HTTP 404 现在应返回错误）
	result, err := dl.Download(req)
	if err == nil {
		t.Error("期望 HTTP 404 返回错误，但未返回")
	}
	if result == nil {
		t.Fatal("期望 result 不为 nil")
	}
	if result.Error == nil {
		t.Error("期望 result.Error 不为 nil（HTTP 404 应被记录）")
	}
	if result.Success {
		t.Error("期望 Success=false（HTTP 非成功状态码）")
	}

	// 测试无效 URL
	req.URL = "://invalid-url"
	_, err = dl.Download(req)
	if err == nil {
		t.Error("期望无效 URL 返回错误")
	}
}

// =============================================================================
// Phase 1.1 验证: per-file sync.Map 锁正确性
// =============================================================================

func TestFileWriter_ConcurrentDifferentFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_concurrent_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)

	numFiles := 20
	var mu sync.Mutex
	var errors []error
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("content-%d", idx))
			path := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", idx))

			req := WriteRequest{
				Path: path,
				Data: data,
				Options: WriteOptions{
					CreateVersion: false,
					SkipUnchanged: false,
				},
			}
			result, writeErr := fw.Write(req)
			if writeErr != nil || !result.Success {
				mu.Lock()
				errors = append(errors, fmt.Errorf("file %d failed: %v", idx, writeErr))
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	if len(errors) > 0 {
		for _, e := range errors {
			t.Error(e)
		}
	}

	for i := 0; i < numFiles; i++ {
		path := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("读取文件 %d 失败: %v", i, err)
			continue
		}
		expected := fmt.Sprintf("content-%d", i)
		if string(content) != expected {
			t.Errorf("文件 %d 内容不匹配: 期望 %q, 实际 %q", i, expected, string(content))
		}
	}

	t.Logf("%d 个不同文件并发写入耗时: %v", numFiles, elapsed)
}

func TestFileWriter_ConcurrentSameFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_concurrent_same_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)

	samePath := filepath.Join(tempDir, "same_file.txt")
	numWriters := 10
	var wg sync.WaitGroup
	var successCount int64
	var mu sync.Mutex

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("writer-%d-data", idx))
			req := WriteRequest{
				Path: samePath,
				Data: data,
				Options: WriteOptions{
					CreateVersion: false,
					SkipUnchanged: false,
				},
			}
			result, writeErr := fw.Write(req)
			if writeErr == nil && result.Success {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if successCount != int64(numWriters) {
		t.Errorf("期望 %d 个写入全部成功, 实际成功 %d", numWriters, successCount)
	}

	// 验证最终文件存在且内容是最后一次写入的内容
	content, err := os.ReadFile(samePath)
	if err != nil {
		t.Fatalf("读取目标文件失败: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("文件内容为空")
	}

	validContent := false
	for i := 0; i < numWriters; i++ {
		expected := fmt.Sprintf("writer-%d-data", i)
		if string(content) == expected {
			validContent = true
			break
		}
	}
	if !validContent {
		t.Errorf("文件内容不是任何一次有效写入的结果, 实际: %q", string(content))
	}

	t.Logf("%d 个 goroutine 并发写入同一文件全部完成, 最终内容来自其中一个 writer", numWriters)
}

// =============================================================================
// Phase 1.3 验证: VersionManager 注入 FileWriter 后使用它写入版本文件
// =============================================================================

func TestVersionManager_WithFileWriter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "versionmanager_fw_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	vm := NewVersionManagerWithWriter(".versions", fw)

	sourcePath := filepath.Join(tempDir, "important_data.json")
	sourceData := []byte(`{"key": "value", "data": [1, 2, 3]}`)
	if err := os.WriteFile(sourcePath, sourceData, 0644); err != nil {
		t.Fatalf("创建源文件失败: %v", err)
	}

	versionPath, err := vm.CreateVersion(sourcePath)
	if err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}

	versionContent, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatalf("读取版本文件失败: %v", err)
	}
	if string(versionContent) != string(sourceData) {
		t.Errorf("版本文件内容不匹配\n期望: %s\n实际: %s", string(sourceData), string(versionContent))
	}

	versionsDir := filepath.Join(tempDir, ".versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		t.Fatalf("读取版本目录失败: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("版本目录为空")
	}
	t.Logf("注入 FileWriter 的 VersionManager 成功创建版本: %s", entries[0].Name())
}

func TestVersionManager_WithoutFileWriter_Fallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "versionmanager_nofw_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	vm := NewVersionManagerWithWriter(".versions", nil)

	sourcePath := filepath.Join(tempDir, "fallback_test.txt")
	sourceData := []byte("fallback content")
	if err := os.WriteFile(sourcePath, sourceData, 0644); err != nil {
		t.Fatalf("创建源文件失败: %v", err)
	}

	versionPath, err := vm.CreateVersion(sourcePath)
	if err != nil {
		t.Fatalf("创建版本失败(回退路径): %v", err)
	}

	versionContent, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatalf("读取版本文件失败: %v", err)
	}
	if string(versionContent) != string(sourceData) {
		t.Errorf("回退路径版本文件内容不匹配\n期望: %s\n实际: %s", string(sourceData), string(versionContent))
	}
	t.Log("无 FileWriter 注入时回退到 os.WriteFile 成功")
}

// =============================================================================
// Phase 3.1 验证: ExtractImageExtFromURL 工具函数
// =============================================================================

func TestExtractImageExtFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"标准 jpg URL", "https://pbs.twimg.com/media/photo.jpg", ".jpg"},
		{"大写 JPG URL", "https://example.com/IMAGE.JPG", ".jpg"},
		{"混合大小写 JPEG", "https://cdn.example.com/photo.JPEG", ".jpeg"},
		{"PNG 图片", "https://example.com/icon.PNG", ".png"},
		{"GIF 动图", "https://media.example.com/anim.GIF", ".gif"},
		{"WebP 格式", "https://cdn.example.com/img.webp", ".webp"},
		{"带查询参数的 jpg", "https://pbs.twimg.com/media/photo.jpg?name=4096x4096", ".jpg"},
		{"带路径段的 png", "https://cdn.example.com/a/b/c/image.png", ".png"},
		{"无扩展名默认 jpg", "https://pbs.twimg.com/media/noext", ".jpg"},
		{"空字符串默认 jpg", "", ".jpg"},
		{"未知扩展名默认 jpg", "https://example.com/file.xyz", ".jpg"},
		{"视频 URL 默认 jpg", "https://video.twimg.com/tweet_video/123.mp4", ".jpg"},
		{"tweet_video 路径", "https://tweet_video/abc.mp4", ".jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractImageExtFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractImageExtFromURL(%q) = %q, 期望 %q", tt.url, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Phase 4: 流式下载测试
// =============================================================================

func TestFileWriter_WriteStream_Normal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_stream_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)

	// 准备测试数据（模拟大文件内容）
	testData := []byte("this is stream test content for large file simulation")
	testPath := filepath.Join(tempDir, "stream_test.txt")
	expectedSize := int64(len(testData))

	// 使用 io.Reader 创建写入请求
	reader := strings.NewReader(string(testData))
	req := WriteRequest{
		Path:   testPath,
		Reader: reader,
		Size:   expectedSize,
		Options: WriteOptions{
			CreateVersion: false,
			SkipUnchanged: false,
		},
	}

	result, err := fw.Write(req)
	if err != nil {
		t.Fatalf("流式写入失败: %v", err)
	}

	// 验证结果
	if !result.Success {
		t.Error("期望 Success=true, 实际 false")
	}
	if result.Skipped {
		t.Error("期望 Skipped=false, 实际 true")
	}
	if result.NewSize != expectedSize {
		t.Errorf("期望 NewSize=%d, 实际 %d", expectedSize, result.NewSize)
	}

	// 验证文件内容
	content, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("期望内容 %q, 实际 %q", string(testData), string(content))
	}
}

func TestFileWriter_WriteStream_SkipUnchanged(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_stream_skip_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)

	testData := []byte("stream skip unchanged test content")
	testPath := filepath.Join(tempDir, "stream_skip.txt")
	expectedSize := int64(len(testData))

	// 第一次写入
	reader1 := strings.NewReader(string(testData))
	req1 := WriteRequest{
		Path:   testPath,
		Reader: reader1,
		Size:   expectedSize,
		Options: WriteOptions{
			SkipUnchanged: true,
		},
	}
	result1, err := fw.Write(req1)
	if err != nil {
		t.Fatalf("第一次流式写入失败: %v", err)
	}
	if result1.Skipped {
		t.Error("第一次写入不应该被跳过")
	}

	// 第二次写入相同大小内容（应该跳过）
	reader2 := strings.NewReader(string(testData))
	req2 := WriteRequest{
		Path:   testPath,
		Reader: reader2,
		Size:   expectedSize,
		Options: WriteOptions{
			SkipUnchanged: true,
		},
	}
	result2, err := fw.Write(req2)
	if err != nil {
		t.Fatalf("第二次流式写入失败: %v", err)
	}
	if !result2.Skipped {
		t.Error("期望第二次写入被跳过（大小相同）")
	}
	if !result2.Success {
		t.Error("跳过的写入仍应标记为成功")
	}
}

func TestDownloader_Download_StreamMode(t *testing.T) {
	// 创建模拟 HTTP 服务器，返回大于 10MB 的内容（触发流式模式）
	largeContent := make([]byte, 11*1024*1024) // 11MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(largeContent)))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(largeContent)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_stream_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "large_file.bin")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/large.bin",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	result, err := dl.Download(req)
	if err != nil {
		t.Fatalf("流式下载失败: %v", err)
	}

	if !result.Success {
		t.Error("期望 Success=true")
	}
	if result.FileSize != int64(len(largeContent)) {
		t.Errorf("期望 FileSize=%d, 实际 %d", len(largeContent), result.FileSize)
	}

	// 验证文件内容
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != string(largeContent) {
		t.Error("文件内容不匹配")
	}
}

func TestDownloader_Download_BufferMode(t *testing.T) {
	// 创建模拟 HTTP 服务器，返回小于 10MB 的内容（触发 Buffer 模式）
	smallContent := []byte("small file content for buffer mode test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(smallContent)))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(smallContent)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_buffer_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "small_file.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/small.txt",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	result, err := dl.Download(req)
	if err != nil {
		t.Fatalf("Buffer 模式下载失败: %v", err)
	}

	if !result.Success {
		t.Error("期望 Success=true")
	}
	if result.FileSize != int64(len(smallContent)) {
		t.Errorf("期望 FileSize=%d, 实际 %d", len(smallContent), result.FileSize)
	}

	// 验证文件内容
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != string(smallContent) {
		t.Errorf("期望内容 %q, 实际 %q", string(smallContent), string(content))
	}
}

func TestDownloader_Download_HeadRequestFail(t *testing.T) {
	// 创建模拟 HTTP 服务器，HEAD 请求失败但 GET 成功
	testContent := []byte("content when head fails")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(testContent)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_head_fail_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "head_fail.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/test.txt",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	result, err := dl.Download(req)
	if err != nil {
		t.Fatalf("HEAD 失败后下载失败: %v", err)
	}

	if !result.Success {
		t.Error("期望 Success=true")
	}

	// 验证文件内容
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("期望内容 %q, 实际 %q", string(testContent), string(content))
	}
}

func TestDownloader_Download_StreamSizeMismatch(t *testing.T) {
	// 创建模拟 HTTP 服务器，返回的内容大小与 Content-Length 不符
	// 使用大于 10MB 的大小触发流式模式
	declaredSize := 11 * 1024 * 1024 // 11MB
	actualContent := []byte("short content from buffer mode fallback") // 实际内容

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", declaredSize))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(actualContent)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_size_mismatch_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "mismatch.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/mismatch.txt",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	result, err := dl.Download(req)
	// 流式下载失败后回退到 Buffer 模式，应该成功
	if err != nil {
		t.Errorf("期望回退到 Buffer 模式后成功，但返回错误: %v", err)
	}
	if !result.Success {
		t.Error("期望 Success=true")
	}

	// 验证文件内容（应该是 Buffer 模式下载的内容）
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != string(actualContent) {
		t.Errorf("期望内容 %q, 实际 %q", string(actualContent), string(content))
	}
}
