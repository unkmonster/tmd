package api

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
	"path"
	"strings"
	"sync"
)

//go:embed web/*
var webFS embed.FS

var (
	themeMu     sync.RWMutex
	frontendTheme = "web1" // web1 或 web2，运行时热切换
)

func getFrontendDir() string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return "web/" + frontendTheme
}

func readFrontendFile(name string) ([]byte, error) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return webFS.ReadFile("web/" + frontendTheme + "/" + name)
}

func setFrontendTheme(theme string) bool {
	if theme == "" || strings.ContainsAny(theme, "/\\..") {
		return false
	}
	// 验证目录在 embed FS 中真实存在（尝试读 index.html）
	themeMu.RLock()
	_, err := webFS.ReadFile("web/" + theme + "/index.html")
	themeMu.RUnlock()
	if err != nil {
		return false
	}

	themeMu.Lock()
	frontendTheme = theme
	themeMu.Unlock()
	return true
}

func getFrontendTheme() string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return frontendTheme
}

// handleWeb 返回 Web 管理页面
func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	data, err := readFrontendFile("index.html")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to load web page")
		return
	}
	s.writeCachedContent(w, r, data, "text/html; charset=utf-8", "public, max-age=3600")
}

// handleStatic 静态文件服务
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	reqPath := r.PathValue("path")
	if reqPath == "" {
		http.NotFound(w, r)
		return
	}

	// 使用 path.Clean 来规范化路径，自动处理掉所有的 "." 和 ".." 以及多余的斜杠
	cleanPath := path.Clean("/" + reqPath)

	// 确保规范化后的路径不会逃逸出根目录
	if strings.Contains(cleanPath, "..") {
		http.NotFound(w, r)
		return
	}

	cleanPath = strings.TrimPrefix(cleanPath, "/")

	data, err := readFrontendFile(cleanPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	contentType := "application/octet-stream"
	switch {
	case strings.HasSuffix(cleanPath, ".html"):
		contentType = "text/html; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".css"):
		contentType = "text/css; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".js"):
		contentType = "application/javascript; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".json"):
		contentType = "application/json"
	case strings.HasSuffix(cleanPath, ".png"):
		contentType = "image/png"
	case strings.HasSuffix(cleanPath, ".jpg"), strings.HasSuffix(cleanPath, ".jpeg"):
		contentType = "image/jpeg"
	case strings.HasSuffix(cleanPath, ".svg"):
		contentType = "image/svg+xml"
	}

	s.writeCachedContent(w, r, data, contentType, "public, max-age=86400")
}

func (s *Server) writeCachedContent(w http.ResponseWriter, r *http.Request, data []byte, contentType, cacheControl string) {
	etag := contentETag(data)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("ETag", etag)

	if ifNoneMatch(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if _, err := w.Write(data); err != nil {
		return
	}
}

func contentETag(data []byte) string {
	sum := sha256.Sum256(data)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func ifNoneMatch(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}
