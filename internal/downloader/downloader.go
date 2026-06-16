package downloader

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/utils"
)

const streamThreshold = 10 * 1024 * 1024 // 10MB
const maxDownloadRetries = 2             // 最大重试次数
const retryDelay = 2 * time.Second       // 重试间隔

func waitRetryDelay(ctx context.Context, delay time.Duration) error {
	if ctx == nil {
		time.Sleep(delay)
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// DefaultDownloader 默认下载器实现，使用独立的 HTTP 客户端下载媒体文件，
// 不携带 Twitter API 鉴权凭据，以避免敏感信息泄漏到 CDN 服务器。
type DefaultDownloader struct {
	fileWriter     FileWriter
	logger         log.FieldLogger
	downloadClient *resty.Client // 专用于媒体文件下载的客户端，无 API 鉴权
}

func newHTTPStatusError(statusCode int, url string) error {
	return &utils.HttpStatusError{
		Code: statusCode,
		Msg:  url,
	}
}

func isNonRetriableStatusError(err error) bool {
	return utils.IsStatusCode(err, 403) || utils.IsStatusCode(err, 404)
}

// NewDownloader 创建下载器
func NewDownloader(fileWriter FileWriter) *DefaultDownloader {
	return &DefaultDownloader{
		fileWriter:     fileWriter,
		logger:         log.StandardLogger(),
		downloadClient: newDownloadClient(),
	}
}

// newDownloadClient 创建专用于媒体文件下载的 HTTP 客户端。
// 与 Twitter API 客户端不同，该客户端：
//   - 不携带任何鉴权凭据（无 Authorization、Cookie、X-Csrf-Token）
//   - 使用对大文件下载友好的超时配置
//   - 无重试逻辑（由 downloadStream 自行控制）
//   - 无 Twitter API 错误检查钩子
func newDownloadClient() *resty.Client {
	c := resty.New()
	c.SetTransport(&http.Transport{
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		Proxy:                 http.ProxyFromEnvironment,
	})
	c.SetTimeout(5 * time.Minute)
	return c
}

// Download 下载单个文件
func (d *DefaultDownloader) Download(req DownloadRequest) (*DownloadResult, error) {
	// 1. 获取文件大小（HEAD 请求）
	contentLength, err := d.getContentLength(req)
	if err != nil {
		// HEAD 失败，回退到 Buffer 模式
		d.logger.WithFields(log.Fields{
			"url":   req.URL,
			"error": err,
		}).Debug("HEAD request failed, fallback to buffer mode")
		return d.downloadBuffer(req)
	}

	// 2. 根据大小选择策略
	if contentLength > streamThreshold {
		// 大文件：流式下载（带重试）
		d.logger.WithFields(log.Fields{
			"url":  req.URL,
			"size": contentLength,
		}).Debug("using stream mode for large file")
		return d.downloadStream(req, contentLength)
	} else {
		// 小文件：Buffer 下载（支持 SkipUnchanged）
		d.logger.WithFields(log.Fields{
			"url":  req.URL,
			"size": contentLength,
		}).Debug("using buffer mode for small file")
		return d.downloadBuffer(req)
	}
}

// getContentLength 通过 HEAD 请求获取文件大小
func (d *DefaultDownloader) getContentLength(req DownloadRequest) (int64, error) {
	headReq := d.downloadClient.R().
		SetContext(req.Context).
		SetDoNotParseResponse(true)

	for k, v := range req.Options.QueryParams {
		headReq = headReq.SetQueryParam(k, v)
	}

	resp, err := headReq.Head(req.URL)
	if err != nil {
		return 0, err
	}

	// 先检查响应是否存在
	if resp.RawResponse == nil {
		return 0, fmt.Errorf("no response")
	}

	// 确保关闭响应体
	if resp.RawResponse.Body != nil {
		resp.RawResponse.Body.Close()
	}

	contentLength := resp.RawResponse.ContentLength
	if contentLength <= 0 {
		return 0, fmt.Errorf("unknown content length")
	}

	return contentLength, nil
}

// downloadBuffer 原有 Buffer 模式（小文件）
func (d *DefaultDownloader) downloadBuffer(req DownloadRequest) (*DownloadResult, error) {
	result := &DownloadResult{}

	r := d.downloadClient.R().SetContext(req.Context)
	for k, v := range req.Options.QueryParams {
		r = r.SetQueryParam(k, v)
	}

	resp, err := r.Get(req.URL)
	if err != nil {
		result.Error = err
		return result, err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		err := newHTTPStatusError(resp.StatusCode(), req.URL)
		result.Error = err
		d.logger.WithFields(log.Fields{
			"url":         req.URL,
			"status_code": resp.StatusCode(),
		}).Warn("download failed with non-2xx status")
		return result, err
	}

	writeReq := WriteRequest{
		Path: req.Destination,
		Data: resp.Body(),
		Options: WriteOptions{
			CreateVersion: req.Options.CreateVersion,
			SkipUnchanged: req.Options.SkipUnchanged,
			ModTime:       req.Options.SetModTime,
		},
	}
	writeResult, err := d.fileWriter.Write(writeReq)
	if err != nil {
		result.Error = err
		result.Success = false
		return result, err
	}

	result.Success = writeResult.Success
	result.FilePath = req.Destination
	result.FileSize = writeResult.NewSize
	result.OldSize = writeResult.OldSize
	result.Versioned = writeResult.Versioned

	return result, nil
}

// downloadStream 带重试的流式下载（大文件）
func (d *DefaultDownloader) downloadStream(req DownloadRequest, contentLength int64) (*DownloadResult, error) {
	var lastErr error

	for attempt := 1; attempt <= maxDownloadRetries; attempt++ {
		result, err := d.doDownloadStream(req, contentLength)
		if err == nil {
			// 下载成功
			if attempt > 1 {
				d.logger.WithFields(log.Fields{
					"url":     req.URL,
					"attempt": attempt,
				}).Info("download succeeded after retry")
			}
			return result, nil
		}

		lastErr = err

		if isNonRetriableStatusError(err) {
			return result, err
		}

		// 检查是否是可重试的错误（文件大小不匹配）
		if result != nil && result.Error != nil {
			// 如果是最后一次尝试，回退到 Buffer 模式
			if attempt == maxDownloadRetries {
				d.logger.WithFields(log.Fields{
					"url":        req.URL,
					"attempts":   maxDownloadRetries,
					"last_error": err,
				}).Warn("stream download failed after max retries, fallback to buffer mode")
				return d.downloadBuffer(req)
			}

			// 记录重试日志
			d.logger.WithFields(log.Fields{
				"url":         req.URL,
				"attempt":     attempt,
				"max_retries": maxDownloadRetries,
				"error":       err,
			}).Warn("download failed, retrying...")

			// 等待一段时间后重试
			if err := waitRetryDelay(req.Context, retryDelay*time.Duration(attempt)); err != nil {
				result.Error = err
				return result, err
			}
		} else {
			// 其他错误（如网络错误），直接返回，等会会记录到error.json
			return result, err
		}
	}

	return nil, lastErr
}

// doDownloadStream 执行单次流式下载
func (d *DefaultDownloader) doDownloadStream(req DownloadRequest, contentLength int64) (*DownloadResult, error) {
	result := &DownloadResult{}

	r := d.downloadClient.R().
		SetContext(req.Context).
		SetDoNotParseResponse(true) // 关键：不自动解析响应体

	for k, v := range req.Options.QueryParams {
		r = r.SetQueryParam(k, v)
	}

	resp, err := r.Get(req.URL)
	if err != nil {
		result.Error = err
		return result, err
	}

	// 检查 RawBody 是否为 nil
	if resp.RawResponse == nil || resp.RawResponse.Body == nil {
		result.Error = fmt.Errorf("no response body")
		return result, result.Error
	}
	defer resp.RawResponse.Body.Close()

	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		err := newHTTPStatusError(resp.StatusCode(), req.URL)
		result.Error = err
		d.logger.WithFields(log.Fields{
			"url":         req.URL,
			"status_code": resp.StatusCode(),
		}).Warn("download failed with non-2xx status")
		return result, err
	}

	writeReq := WriteRequest{
		Path:   req.Destination,
		Reader: resp.RawResponse.Body,
		Size:   contentLength,
		Options: WriteOptions{
			CreateVersion: req.Options.CreateVersion,
			SkipUnchanged: req.Options.SkipUnchanged,
			ModTime:       req.Options.SetModTime,
		},
	}
	writeResult, err := d.fileWriter.Write(writeReq)
	if err != nil {
		result.Error = err
		result.Success = false
		return result, err
	}

	// 验证文件大小是否与预期一致
	if writeResult.NewSize != contentLength {
		err := fmt.Errorf("file size mismatch: expected %d bytes, got %d bytes", contentLength, writeResult.NewSize)
		result.Error = err
		result.Success = false
		d.logger.WithFields(log.Fields{
			"url":           req.URL,
			"expected_size": contentLength,
			"actual_size":   writeResult.NewSize,
		}).Warn("download file size mismatch")

		// 删除不完整的文件
		if removeErr := os.Remove(req.Destination); removeErr != nil {
			d.logger.WithFields(log.Fields{
				"url":   req.URL,
				"error": removeErr,
			}).Warn("failed to remove incomplete file")
		}

		return result, err
	}

	result.Success = writeResult.Success
	result.FilePath = req.Destination
	result.FileSize = writeResult.NewSize
	result.OldSize = writeResult.OldSize
	result.Versioned = writeResult.Versioned

	return result, nil
}
