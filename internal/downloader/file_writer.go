package downloader

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type DefaultFileWriter struct {
	versionManager VersionManager
	locks          [256]sync.Mutex
}

func (fw *DefaultFileWriter) getLock(path string) *sync.Mutex {
	var hash uint32
	for i := 0; i < len(path); i++ {
		hash = hash*31 + uint32(path[i])
	}
	return &fw.locks[hash%256]
}

// NewFileWriter 创建文件写入器
func NewFileWriter(versionManager VersionManager) *DefaultFileWriter {
	return &DefaultFileWriter{
		versionManager: versionManager,
	}
}

// Write 写入文件
func (fw *DefaultFileWriter) Write(req WriteRequest) (WriteResult, error) {
	// 如果提供了 Reader，使用流式模式
	if req.IsStream() {
		return fw.writeStream(req.Path, req.Reader, req.Size, req.Options)
	}

	result := WriteResult{NewSize: int64(len(req.Data))}

	lock := fw.getLock(req.Path)
	lock.Lock()
	defer lock.Unlock()

	// 1. 检查是否需要跳过
	if req.Options.SkipUnchanged {
		exists, fileInfo, err := fw.fileExists(req.Path)
		if err != nil {
			return result, err
		}
		if exists {
			result.OldSize = fileInfo.Size()
			if fileInfo.Size() == result.NewSize {
				oldHash, hashErr := fw.computeFileHash(req.Path)
				if hashErr != nil {
				log.Warnf("[downloader] Failed to compute file hash for SkipUnchanged check: %v, path: %s", hashErr, req.Path)
					return result, hashErr
				}
				newHash := fw.computeDataHash(req.Data)
				if oldHash == newHash {
					// 文件未变化，跳过写入
					result.Success = true
					return result, nil
				}
			}
		}
	}

	// 2. 创建版本备份（如果需要）
		if req.Options.CreateVersion && fw.versionManager != nil {
			if _, err := os.Stat(req.Path); err == nil {
				_, err = fw.versionManager.CreateVersion(req.Path)
				if err != nil {
					return result, err
				}
				result.Versioned = true
			}
		}

	// 3. 原子写入
	written, err := fw.atomicWriteFromReader(req.Path, bytes.NewReader(req.Data))
	if err != nil {
		return result, err
	}
	result.NewSize = written

	// 4. 设置修改时间
	if req.Options.ModTime != nil {
		if err := os.Chtimes(req.Path, time.Time{}, *req.Options.ModTime); err != nil {
			log.Warnf("[downloader] Failed to set modification time for %s: %v", req.Path, err)
		}
	}

	result.Success = true
	return result, nil
}

// writeStream 流式写入文件
func (fw *DefaultFileWriter) writeStream(path string, reader io.Reader, size int64, options WriteOptions) (WriteResult, error) {
	result := WriteResult{NewSize: size}

	lock := fw.getLock(path)
	lock.Lock()
	defer lock.Unlock()

	// 流式模式不支持 SkipUnchanged（无法预先计算 hash）
	// 但可以通过文件大小快速判断
	if options.SkipUnchanged && size > 0 {
		exists, fileInfo, err := fw.fileExists(path)
		if err != nil {
			return result, err
		}
		if exists && fileInfo.Size() == size {
			// 大小相同，假设内容相同（跳过重下载）
			result.OldSize = fileInfo.Size()
			result.Success = true
			return result, nil
		}
	}

	// 创建版本备份（如果需要）
		if options.CreateVersion && fw.versionManager != nil {
			if _, err := os.Stat(path); err == nil {
				_, err = fw.versionManager.CreateVersion(path)
				if err != nil {
					return result, err
				}
				result.Versioned = true
			}
		}

	// 原子流式写入
	written, err := fw.atomicWriteFromReader(path, reader)
	if err != nil {
		return result, err
	}

	// 更新实际写入的字节数
	result.NewSize = written

	// 设置修改时间
	if options.ModTime != nil {
		if err := os.Chtimes(path, time.Time{}, *options.ModTime); err != nil {
			log.Warnf("[downloader] Failed to set modification time for %s: %v", path, err)
		}
	}

	result.Success = true
	return result, nil
}

// atomicWriteFromReader 原子写入 reader 内容
func (fw *DefaultFileWriter) atomicWriteFromReader(path string, reader io.Reader) (int64, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	tempFile, err := os.CreateTemp(dir, ".tmp_*")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	defer os.Remove(tempPath)

	// 使用缓冲区复制
	buf := make([]byte, 64*1024) // 64KB 缓冲区
	written, err := io.CopyBuffer(tempFile, reader, buf)
	if err != nil {
		tempFile.Close()
		return 0, fmt.Errorf("failed to write to temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return 0, fmt.Errorf("failed to close temp file: %w", err)
	}

	log.Debugf("[downloader] Atomic write: written %d bytes to %s", written, path)

	if err := os.Rename(tempPath, path); err != nil {
		return 0, fmt.Errorf("failed to rename temp file: %w", err)
	}

	return written, nil
}

// fileExists 检查文件是否存在
func (fw *DefaultFileWriter) fileExists(path string) (bool, os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, info, nil
}

// computeFileHash 计算文件 Hash
func (fw *DefaultFileWriter) computeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fw.computeDataHash(data), nil
}

// computeDataHash 计算数据 Hash
func (fw *DefaultFileWriter) computeDataHash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}
