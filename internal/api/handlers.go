package api

import (
	"embed"
	"net/http"
	"path"
	"strings"
)

//go:embed web/*
var webFS embed.FS

// handleWeb 返回 Web 管理页面
func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	data, err := webFS.ReadFile("web/index.html")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to load web page")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", "\"v1.0.0\"")
	if _, err := w.Write(data); err != nil {
		// 这里只记录日志，因为头部可能已经发送，无法再返回 HTTP 500
		return
	}
}

// handleStatic 静态文件服务
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimPrefix(r.URL.Path, "/static/")

	// 使用 path.Clean 来规范化路径，自动处理掉所有的 "." 和 ".." 以及多余的斜杠
	cleanPath := path.Clean("/" + reqPath)

	// 确保规范化后的路径不会逃逸出根目录
	if strings.Contains(cleanPath, "..") {
		http.NotFound(w, r)
		return
	}

	cleanPath = strings.TrimPrefix(cleanPath, "/")

	data, err := webFS.ReadFile("web/" + cleanPath)
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

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("ETag", "\"v1.0.0\"")
	if _, err := w.Write(data); err != nil {
		return
	}
}
