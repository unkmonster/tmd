package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if !result1.Success {
		t.Error("第一次写入应该成功")
	}

	// 第二次写入相同内容（应该跳过，但标记为成功）
	result2, err := fw.Write(req)
	if err != nil {
		t.Fatalf("第二次写入失败: %v", err)
	}

	// 验证跳过（成功但未写入新内容）
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
	if !result3.Success {
		t.Error("写入不同内容应该成功")
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
		t.Fatalf("创建版本失败(内部 fallback writer): %v", err)
	}

	versionContent, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatalf("读取版本文件失败: %v", err)
	}
	if string(versionContent) != string(sourceData) {
		t.Errorf("fallback writer 版本文件内容不匹配\n期望: %s\n实际: %s", string(sourceData), string(versionContent))
	}
	t.Log("无 FileWriter 注入时回退到内部 fallback writer 成功")
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
	if !result1.Success {
		t.Error("第一次写入应该成功")
	}

	// 第二次写入相同大小内容（应该跳过，但仍标记为成功）
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
	declaredSize := 11 * 1024 * 1024                                   // 11MB
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

// =============================================================================
// FileWriter 内部方法测试
// =============================================================================

func TestFileWriter_computeDataHash(t *testing.T) {
	fw := NewFileWriter(nil)

	tests := []struct {
		name string
		data []byte
	}{
		{"空数据", []byte{}},
		{"简单字符串", []byte("hello")},
		{"中文字符", []byte("你好世界")},
		{"特殊字符", []byte("!@#$%^&*()")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fw.computeDataHash(tt.data)
			// 验证哈希值是有效的 MD5（32位十六进制字符串）
			if len(result) != 32 {
				t.Errorf("computeDataHash(%q) 返回的哈希长度 %d 不是 32", tt.data, len(result))
			}
			// 验证相同输入产生相同输出
			result2 := fw.computeDataHash(tt.data)
			if result != result2 {
				t.Errorf("computeDataHash 对相同输入返回不同结果: %q vs %q", result, result2)
			}
		})
	}
}

func TestFileWriter_computeFileHash(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_hash_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)

	// 测试正常文件
	testFile := filepath.Join(tempDir, "test.txt")
	testData := []byte("hello world")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	hash, err := fw.computeFileHash(testFile)
	if err != nil {
		t.Fatalf("计算文件哈希失败: %v", err)
	}

	expectedHash := fw.computeDataHash(testData)
	if hash != expectedHash {
		t.Errorf("文件哈希不匹配: 得到 %q, 期望 %q", hash, expectedHash)
	}

	// 测试不存在的文件
	_, err = fw.computeFileHash(filepath.Join(tempDir, "nonexistent.txt"))
	if err == nil {
		t.Error("期望不存在的文件返回错误")
	}
}

func TestFileWriter_fileExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_exists_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)

	// 测试存在的文件
	testFile := filepath.Join(tempDir, "exists.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	exists, info, err := fw.fileExists(testFile)
	if err != nil {
		t.Fatalf("检查文件存在性失败: %v", err)
	}
	if !exists {
		t.Error("期望文件存在")
	}
	if info == nil {
		t.Error("期望返回文件信息")
	}
	if info.Size() != int64(len("content")) {
		t.Errorf("期望文件大小 %d, 实际 %d", len("content"), info.Size())
	}

	// 测试不存在的文件
	exists, info, err = fw.fileExists(filepath.Join(tempDir, "nonexistent.txt"))
	if err != nil {
		t.Fatalf("检查不存在的文件失败: %v", err)
	}
	if exists {
		t.Error("期望文件不存在")
	}
	if info != nil {
		t.Error("期望不存在的文件返回 nil 信息")
	}
}

func TestFileWriter_getLock(t *testing.T) {
	fw := NewFileWriter(nil)

	// 测试获取不同路径的锁
	lock1 := fw.getLock("/path/to/file1.txt")
	lock2 := fw.getLock("/path/to/file2.txt")
	lock1Again := fw.getLock("/path/to/file1.txt")

	if lock1 == nil {
		t.Error("期望 lock1 不为 nil")
	}
	if lock2 == nil {
		t.Error("期望 lock2 不为 nil")
	}
	if lock1 == lock2 {
		t.Error("不同路径应该返回不同的锁")
	}
	if lock1 != lock1Again {
		t.Error("相同路径应该返回相同的锁")
	}
}

func TestFileWriter_WriteStream_WithVersion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_stream_version_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	vm := NewVersionManagerWithWriter(".versions", nil)
	fw := NewFileWriter(vm)

	testPath := filepath.Join(tempDir, "test.txt")
	oldData := []byte("old stream content")
	newData := []byte("new stream content")

	// 第一次写入（创建文件）
	reader1 := strings.NewReader(string(oldData))
	req1 := WriteRequest{
		Path:   testPath,
		Reader: reader1,
		Size:   int64(len(oldData)),
		Options: WriteOptions{
			CreateVersion: true,
		},
	}
	_, err = fw.Write(req1)
	if err != nil {
		t.Fatalf("第一次流式写入失败: %v", err)
	}

	// 第二次写入（应该创建版本）
	reader2 := strings.NewReader(string(newData))
	req2 := WriteRequest{
		Path:   testPath,
		Reader: reader2,
		Size:   int64(len(newData)),
		Options: WriteOptions{
			CreateVersion: true,
		},
	}
	result, err := fw.Write(req2)
	if err != nil {
		t.Fatalf("第二次流式写入失败: %v", err)
	}
	if !result.Success {
		t.Error("期望写入成功")
	}

	// 验证版本文件存在
	versionsDir := filepath.Join(tempDir, ".versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		t.Fatalf("读取版本目录失败: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("期望存在版本文件")
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

func TestFileWriter_WriteStream_WithModTime(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filewriter_stream_modtime_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)

	testData := []byte("stream content with modtime")
	testPath := filepath.Join(tempDir, "modtime.txt")
	modTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)

	reader := strings.NewReader(string(testData))
	req := WriteRequest{
		Path:   testPath,
		Reader: reader,
		Size:   int64(len(testData)),
		Options: WriteOptions{
			ModTime: &modTime,
		},
	}
	_, err = fw.Write(req)
	if err != nil {
		t.Fatalf("流式写入失败: %v", err)
	}

	// 验证修改时间
	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("获取文件信息失败: %v", err)
	}

	actualModTime := info.ModTime().UTC()
	diff := actualModTime.Sub(modTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("期望修改时间 %v, 实际 %v", modTime, actualModTime)
	}
}

// =============================================================================
// Downloader 边界情况测试
// =============================================================================

func TestDownloader_Download_ContentLengthZero(t *testing.T) {
	// 创建模拟 HTTP 服务器，返回 Content-Length 为 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_zero_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "empty.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/empty.txt",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	result, err := dl.Download(req)
	if err != nil {
		t.Fatalf("下载空文件失败: %v", err)
	}
	if !result.Success {
		t.Error("期望 Success=true")
	}
	if result.FileSize != 0 {
		t.Errorf("期望 FileSize=0, 实际 %d", result.FileSize)
	}

	// 验证文件存在且为空
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("获取文件信息失败: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("期望空文件，实际大小 %d", info.Size())
	}
}

func TestDownloader_Download_QueryParams(t *testing.T) {
	var receivedParams url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "13")
			w.WriteHeader(http.StatusOK)
			return
		}
		// 记录接收到的查询参数
		receivedParams = r.URL.Query()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("query params!"))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_query_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "query.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/test",
		Destination: destPath,
		Options: DownloadOptions{
			QueryParams: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	_, err = dl.Download(req)
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}

	if receivedParams.Get("key1") != "value1" {
		t.Errorf("期望 key1=value1, 实际 %s", receivedParams.Get("key1"))
	}
	if receivedParams.Get("key2") != "value2" {
		t.Errorf("期望 key2=value2, 实际 %s", receivedParams.Get("key2"))
	}
}

func TestDownloader_Download_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_server_error_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "error.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/error",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	result, err := dl.Download(req)
	if err == nil {
		t.Error("期望服务器错误返回错误")
	}
	if result == nil {
		t.Fatal("期望 result 不为 nil")
	}
	if result.Success {
		t.Error("期望 Success=false")
	}
	if result.Error == nil {
		t.Error("期望 result.Error 不为 nil")
	}
}

func TestDownloader_Download_RetrySuccess(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", 11*1024*1024))
			w.WriteHeader(http.StatusOK)
			return
		}
		attemptCount++
		if attemptCount == 1 {
			// 第一次返回错误
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// 第二次成功
		largeContent := make([]byte, 11*1024*1024)
		for i := range largeContent {
			largeContent[i] = byte(i % 256)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(largeContent)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_retry_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "retry.bin")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/retry",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	result, err := dl.Download(req)
	if err != nil {
		t.Fatalf("重试后下载失败: %v", err)
	}
	if !result.Success {
		t.Error("期望 Success=true")
	}
	if attemptCount < 2 {
		t.Errorf("期望至少重试一次，实际尝试次数 %d", attemptCount)
	}
}

func TestDownloader_Download_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			return
		}
		// 模拟慢速响应
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow response"))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_cancel_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	// 创建一个会被立即取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	destPath := filepath.Join(tempDir, "cancel.txt")
	req := DownloadRequest{
		Context:     ctx,
		Client:      resty.New(),
		URL:         server.URL + "/cancel",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	_, err = dl.Download(req)
	// 由于 HEAD 请求在取消前完成，可能会返回结果或错误
	// 这里主要验证不会 panic
	t.Logf("上下文取消后的结果: err=%v", err)
}

func TestWaitRetryDelay_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := waitRetryDelay(ctx, 2*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("waitRetryDelay should return quickly after cancellation, elapsed=%v", elapsed)
	}
}

func TestDownloader_DownloadStream_CancelDuringRetryDelay(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", 11*1024*1024))
			w.WriteHeader(http.StatusOK)
			return
		}
		attemptCount++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "downloader_retry_cancel_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	destPath := filepath.Join(tempDir, "retry-cancel.bin")
	req := DownloadRequest{
		Context:     ctx,
		Client:      resty.New(),
		URL:         server.URL + "/retry-cancel",
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	start := time.Now()
	result, err := dl.Download(req)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if result == nil || !errors.Is(result.Error, context.Canceled) {
		t.Fatalf("expected result.Error to be context.Canceled, got %#v", result)
	}
	if elapsed >= retryDelay {
		t.Fatalf("expected cancellation before full retry delay, elapsed=%v", elapsed)
	}
	if attemptCount != 1 {
		t.Fatalf("expected only one GET attempt before cancellation, got %d", attemptCount)
	}
}

func TestDownloader_Download_NetworkError(t *testing.T) {
	// 使用一个无效的端口来模拟网络错误
	tempDir, err := os.MkdirTemp("", "downloader_network_error_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "network_error.txt")
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         "http://localhost:1/test.txt", // 几乎肯定失败的地址
		Destination: destPath,
		Options:     DownloadOptions{},
	}

	result, err := dl.Download(req)
	if err == nil {
		t.Error("期望网络错误返回错误")
	}
	if result == nil {
		t.Fatal("期望 result 不为 nil")
	}
	if result.Success {
		t.Error("期望 Success=false")
	}
}

// =============================================================================
// VersionManager 边界情况测试
// =============================================================================

func TestVersionManager_CreateVersion_NonExistentFile(t *testing.T) {
	vm := NewVersionManagerWithWriter(".versions", nil)

	_, err := vm.CreateVersion("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("期望不存在的文件返回错误")
	}
}

func TestVersionManager_CreateVersion_LongFilename(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "versionmanager_longname_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	vm := NewVersionManagerWithWriter(".versions", nil)

	// 创建一个很长的文件名
	longName := "very_long_filename_" + strings.Repeat("a", 100) + ".txt"
	sourcePath := filepath.Join(tempDir, longName)
	sourceData := []byte("content")
	if err := os.WriteFile(sourcePath, sourceData, 0644); err != nil {
		t.Fatalf("创建源文件失败: %v", err)
	}

	versionPath, err := vm.CreateVersion(sourcePath)
	if err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}

	// 验证版本文件存在且内容正确
	versionContent, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatalf("读取版本文件失败: %v", err)
	}
	if string(versionContent) != string(sourceData) {
		t.Error("版本文件内容不匹配")
	}
}

func TestVersionManager_CreateVersion_NoExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "versionmanager_noext_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	vm := NewVersionManagerWithWriter(".versions", nil)

	// 创建没有扩展名的文件
	sourcePath := filepath.Join(tempDir, "README")
	sourceData := []byte("readme content")
	if err := os.WriteFile(sourcePath, sourceData, 0644); err != nil {
		t.Fatalf("创建源文件失败: %v", err)
	}

	versionPath, err := vm.CreateVersion(sourcePath)
	if err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}

	// 验证版本文件名格式
	versionFilename := filepath.Base(versionPath)
	pattern := `^README_\d{8}_\d{6}_\d{1,3}$`
	matched, _ := regexp.MatchString(pattern, versionFilename)
	if !matched {
		t.Errorf("版本文件名格式不正确: %s", versionFilename)
	}
}

// =============================================================================
// 集成测试
// =============================================================================

func TestIntegration_FullDownloadWorkflow(t *testing.T) {
	// 创建模拟 HTTP 服务器
	testData := []byte("integration test content for full workflow")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "integration_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建完整的组件链
	vm := NewVersionManagerWithWriter(".versions", nil)
	fw := NewFileWriter(vm)
	dl := NewDownloader(fw)

	destPath := filepath.Join(tempDir, "integrated.txt")

	// 第一次下载
	req := DownloadRequest{
		Context:     context.Background(),
		Client:      resty.New(),
		URL:         server.URL + "/test.txt",
		Destination: destPath,
		Options: DownloadOptions{
			CreateVersion: true,
			SkipUnchanged: true,
		},
	}

	result1, err := dl.Download(req)
	if err != nil {
		t.Fatalf("第一次下载失败: %v", err)
	}
	if !result1.Success {
		t.Error("期望第一次下载成功")
	}

	// 第二次下载相同文件（应该被跳过，但仍标记为成功）
	result2, err := dl.Download(req)
	if err != nil {
		t.Fatalf("第二次下载失败: %v", err)
	}
	if !result2.Success {
		t.Error("期望第二次下载成功（跳过未变化的文件）")
	}

	// 验证文件内容
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("文件内容不匹配: 期望 %q, 实际 %q", string(testData), string(content))
	}

	t.Log("完整下载工作流测试通过")
}

func TestIntegration_ConcurrentDownloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			return
		}
		content := []byte("concurrent test content - " + r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "integration_concurrent_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fw := NewFileWriter(nil)
	dl := NewDownloader(fw)

	numDownloads := 10
	var wg sync.WaitGroup
	errChan := make(chan error, numDownloads)

	for i := 0; i < numDownloads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			destPath := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", idx))
			req := DownloadRequest{
				Context:     context.Background(),
				Client:      resty.New(),
				URL:         fmt.Sprintf("%s/file%d", server.URL, idx),
				Destination: destPath,
				Options:     DownloadOptions{},
			}
			result, err := dl.Download(req)
			if err != nil {
				errChan <- fmt.Errorf("下载 %d 失败: %v", idx, err)
				return
			}
			if !result.Success {
				errChan <- fmt.Errorf("下载 %d 未成功", idx)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Error(err)
	}

	// 验证所有文件都已下载
	for i := 0; i < numDownloads; i++ {
		destPath := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", i))
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			t.Errorf("文件 %d 未下载", i)
		}
	}

	t.Logf("并发下载 %d 个文件测试通过", numDownloads)
}
