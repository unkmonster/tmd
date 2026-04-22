package downloader

import (
	"context"
	"io"
	"time"

	"github.com/go-resty/resty/v2"
)

type DownloadRequest struct {
	Context     context.Context
	Client      *resty.Client
	URL         string
	Destination string
	Options     DownloadOptions
}

type DownloadOptions struct {
	QueryParams   map[string]string
	SkipUnchanged bool
	CreateVersion bool
	SetModTime    *time.Time
}

type DownloadResult struct {
	Success  bool
	Skipped  bool
	FilePath string
	FileSize int64
	OldSize  int64
	Error    error
}

type WriteRequest struct {
	Path    string
	Data    []byte    // 用于小文件（Buffer 模式）
	Reader  io.Reader // 用于大文件（Stream 模式）
	Size    int64     // 文件大小（Stream 模式需要）
	Options WriteOptions
}

// IsStream 判断是否使用流式模式
func (w WriteRequest) IsStream() bool {
	return w.Reader != nil
}

type WriteOptions struct {
	CreateVersion bool
	SkipUnchanged bool
	ModTime       *time.Time
}

type WriteResult struct {
	Success bool
	Skipped bool
	OldSize int64
	NewSize int64
}

type Downloader interface {
	Download(req DownloadRequest) (*DownloadResult, error)
}

type FileWriter interface {
	Write(req WriteRequest) (WriteResult, error)
}

type VersionManager interface {
	CreateVersion(sourcePath string) (string, error)
}
