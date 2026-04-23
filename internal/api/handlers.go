package api

import (
	"embed"
	"net/http"
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
	w.Write(data)
}

// handleStatic 静态文件服务
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	// 防止路径穿越：移除所有 ".."
	path = strings.ReplaceAll(path, "..", "")
	path = strings.TrimPrefix(path, "/")
	data, err := webFS.ReadFile("web/" + path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	contentType := "application/octet-stream"
	switch {
	case strings.HasSuffix(path, ".html"):
		contentType = "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		contentType = "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		contentType = "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		contentType = "application/json"
	case strings.HasSuffix(path, ".png"):
		contentType = "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		contentType = "image/jpeg"
	case strings.HasSuffix(path, ".svg"):
		contentType = "image/svg+xml"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("ETag", "\"v1.0.0\"")
	w.Write(data)
}
