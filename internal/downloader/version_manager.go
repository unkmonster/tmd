package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultVersionManager 默认版本管理器实现
type DefaultVersionManager struct {
	versionsDirName string
	fileWriter      FileWriter
	fallbackWriter  FileWriter
}

func NewVersionManagerWithWriter(versionsDirName string, fw FileWriter) *DefaultVersionManager {
	if versionsDirName == "" {
		versionsDirName = ".versions"
	}
	return &DefaultVersionManager{
		versionsDirName: versionsDirName,
		fileWriter:      fw,
		fallbackWriter:  NewFileWriter(nil),
	}
}

func (vm *DefaultVersionManager) SetFileWriter(fw FileWriter) {
	vm.fileWriter = fw
}

// CreateVersion 创建版本备份
func (vm *DefaultVersionManager) CreateVersion(sourcePath string) (string, error) {
	// P1: 空路径校验
	if sourcePath == "" {
		return "", fmt.Errorf("source path cannot be empty")
	}

	dir := filepath.Dir(sourcePath)
	filename := filepath.Base(sourcePath)
	ext := filepath.Ext(filename)
	stem := filename[:len(filename)-len(ext)]

	// P2: 空 stem 边界保护（处理如 ".gitignore" 这类无主文件名的文件）
	if stem == "" {
		stem = "_unknown"
	}

	versionsDir := filepath.Join(dir, vm.versionsDirName)
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102_150405") + fmt.Sprintf("_%d", time.Now().Nanosecond()%1000)
	versionPath := filepath.Join(versionsDir, fmt.Sprintf("%s_%s%s", stem, timestamp, ext))

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}

	if vm.fileWriter != nil {
		_, err = vm.fileWriter.Write(WriteRequest{Path: versionPath, Data: data})
		return versionPath, err
	}

	if vm.fallbackWriter == nil {
		vm.fallbackWriter = NewFileWriter(nil)
	}

	_, err = vm.fallbackWriter.Write(WriteRequest{Path: versionPath, Data: data})
	return versionPath, err
}
