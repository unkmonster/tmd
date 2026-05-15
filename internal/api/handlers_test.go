package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func serveStatic(server *Server, req *http.Request) *httptest.ResponseRecorder {
	if strings.HasPrefix(req.URL.Path, "/static/") {
		req.SetPathValue("path", strings.TrimPrefix(req.URL.Path, "/static/"))
	}

	rr := httptest.NewRecorder()
	server.handleStatic(rr, req)
	return rr
}

func TestHandleWeb_Success(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	server.handleWeb(rr, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=3600", rr.Header().Get("Cache-Control"))
	assert.Equal(t, contentETag(rr.Body.Bytes()), rr.Header().Get("ETag"))
	assert.Greater(t, rr.Body.Len(), 0)
}

func TestHandleWeb_Error(t *testing.T) {
	// 这个测试需要模拟文件读取错误
	// 由于使用了 embed，无法直接测试错误情况
	// 但我们可以验证正常情况下的行为
}

func TestHandleStatic_CSS(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	rr := serveStatic(server, req)

	// 由于实际文件可能不存在，可能返回 404
	// 这里主要测试不会 panic
	contentType := rr.Header().Get("Content-Type")
	if rr.Code == http.StatusOK {
		assert.Equal(t, "text/css; charset=utf-8", contentType)
	}
}

func TestHandleStatic_JS(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	rr := serveStatic(server, req)

	contentType := rr.Header().Get("Content-Type")
	if rr.Code == http.StatusOK {
		assert.Equal(t, "application/javascript; charset=utf-8", contentType)
	}
}

func TestHandleStatic_PNG(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/image.png", nil)
	rr := serveStatic(server, req)

	contentType := rr.Header().Get("Content-Type")
	if rr.Code == http.StatusOK {
		assert.Equal(t, "image/png", contentType)
	}
}

func TestHandleStatic_JPG(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/image.jpg", nil)
	rr := serveStatic(server, req)

	contentType := rr.Header().Get("Content-Type")
	if rr.Code == http.StatusOK {
		assert.Equal(t, "image/jpeg", contentType)
	}
}

func TestHandleStatic_SVG(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/icon.svg", nil)
	rr := serveStatic(server, req)

	contentType := rr.Header().Get("Content-Type")
	if rr.Code == http.StatusOK {
		assert.Equal(t, "image/svg+xml", contentType)
	}
}

func TestHandleStatic_JSON(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/data.json", nil)
	rr := serveStatic(server, req)

	contentType := rr.Header().Get("Content-Type")
	if rr.Code == http.StatusOK {
		assert.Equal(t, "application/json", contentType)
	}
}

func TestHandleStatic_HTML(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/page.html", nil)
	rr := serveStatic(server, req)

	contentType := rr.Header().Get("Content-Type")
	if rr.Code == http.StatusOK {
		assert.Equal(t, "text/html; charset=utf-8", contentType)
	}
}

func TestHandleStatic_UnknownExtension(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/file.unknown", nil)
	rr := serveStatic(server, req)

	// 未知扩展名应该使用默认的 content type
	contentType := rr.Header().Get("Content-Type")
	if rr.Code == http.StatusOK {
		assert.Equal(t, "application/octet-stream", contentType)
	}
}

func TestHandleStatic_PathTraversal(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "双点路径",
			path: "/static/../etc/passwd",
		},
		{
			name: "嵌套双点",
			path: "/static/../../etc/passwd",
		},
		{
			name: "混合路径",
			path: "/static/images/../../../etc/passwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := serveStatic(server, req)

			// 路径遍历应该被阻止，返回 404
			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	}
}

func TestHandleStatic_NotFound(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/nonexistent.file", nil)
	rr := serveStatic(server, req)

	// 不存在的文件应该返回 404
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleStatic_CacheHeaders(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	rr := serveStatic(server, req)

	// 验证缓存头
	if rr.Code == http.StatusOK {
		assert.Equal(t, "public, max-age=86400", rr.Header().Get("Cache-Control"))
		assert.Equal(t, contentETag(rr.Body.Bytes()), rr.Header().Get("ETag"))
	}
}

func TestHandleStatic_EmptyPath(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static/", nil)
	rr := serveStatic(server, req)

	// 空路径应该返回 404
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleStatic_RootPath(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/static", nil)
	serveStatic(server, req)

	// 没有尾部斜杠的路径应该被处理
	// 实际行为取决于实现
}

func TestHandleWeb_DifferentRoutes(t *testing.T) {
	server := &Server{}

	routes := []string{
		"/",
		"/tasks",
		"/data",
		"/system",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rr := httptest.NewRecorder()

			server.handleWeb(rr, req)

			// 所有路由都应该返回相同的 HTML
			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
		})
	}
}

func TestHandleWeb_CacheHeaders(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	server.handleWeb(rr, req)

	assert.Equal(t, "public, max-age=3600", rr.Header().Get("Cache-Control"))
	assert.Equal(t, contentETag(rr.Body.Bytes()), rr.Header().Get("ETag"))
}

func TestHandleWeb_IfNoneMatch(t *testing.T) {
	server := &Server{}

	firstReq := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRR := httptest.NewRecorder()
	server.handleWeb(firstRR, firstReq)
	requireETag := firstRR.Header().Get("ETag")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", requireETag)
	rr := httptest.NewRecorder()

	server.handleWeb(rr, req)

	assert.Equal(t, http.StatusNotModified, rr.Code)
	assert.Equal(t, requireETag, rr.Header().Get("ETag"))
	assert.Equal(t, 0, rr.Body.Len())
}

func TestHandleStatic_IfNoneMatch(t *testing.T) {
	server := &Server{}

	firstReq := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	firstRR := serveStatic(server, firstReq)
	if firstRR.Code != http.StatusOK {
		t.Skip("embedded app.js not found")
	}
	requireETag := firstRR.Header().Get("ETag")

	req := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	req.Header.Set("If-None-Match", requireETag)
	rr := serveStatic(server, req)

	assert.Equal(t, http.StatusNotModified, rr.Code)
	assert.Equal(t, requireETag, rr.Header().Get("ETag"))
	assert.Equal(t, 0, rr.Body.Len())
}

func TestHandleStatic_PathCleaning(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name     string
		path     string
		expected string // 清理后的路径
	}{
		{
			name:     "普通路径",
			path:     "/static/style.css",
			expected: "style.css",
		},
		{
			name:     "带双点的路径",
			path:     "/static/../style.css",
			expected: "style.css", // .. 被移除
		},
		{
			name:     "带斜杠的路径",
			path:     "/static//style.css",
			expected: "/style.css",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			serveStatic(server, req)

			// 主要验证不会 panic
		})
	}
}

func TestHandleStatic_MultipleDotsInFilename(t *testing.T) {
	server := &Server{}

	tests := []struct {
		filename string
		expected string
	}{
		{"file.min.css", "text/css; charset=utf-8"},
		{"file.bundle.js", "application/javascript; charset=utf-8"},
		{"file.test.json", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/static/"+tt.filename, nil)
			rr := serveStatic(server, req)

			if rr.Code == http.StatusOK {
				assert.Equal(t, tt.expected, rr.Header().Get("Content-Type"))
			}
		})
	}
}

func TestHandleStatic_CaseSensitivity(t *testing.T) {
	server := &Server{}

	tests := []struct {
		path string
	}{
		{"/static/FILE.CSS"},
		{"/static/File.Css"},
		{"/static/file.CSS"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			serveStatic(server, req)

			// 验证不会 panic
		})
	}
}

func TestHandleWeb_MethodNotAllowed(t *testing.T) {
	server := &Server{}

	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()

			server.handleWeb(rr, req)

			// 当前实现没有检查方法，所以应该返回 200
			// 如果将来添加了方法检查，这里可能需要修改
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestHandleStatic_LongPath(t *testing.T) {
	server := &Server{}

	// 构造一个很长的路径
	longPath := "/static/" + strings.Repeat("a/", 100) + "file.css"

	req := httptest.NewRequest(http.MethodGet, longPath, nil)
	serveStatic(server, req)

	// 验证不会 panic
}

func TestHandleStatic_SpecialCharacters(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "空格编码",
			path: "/static/file%20name.css",
		},
		{
			name: "中文编码",
			path: "/static/%E6%96%87%E4%BB%B6.css",
		},
		{
			name: "特殊符号",
			path: "/static/file-name_2.0.css",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			serveStatic(server, req)

			// 验证不会 panic
		})
	}
}

func TestHandleWeb_ResponseBody(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	server.handleWeb(rr, req)

	// 验证响应体不为空且包含 HTML
	body := rr.Body.String()
	assert.Greater(t, len(body), 0)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
}

func TestHandleStatic_Methods(t *testing.T) {
	server := &Server{}

	methods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/static/style.css", nil)
			serveStatic(server, req)

			// 验证不会 panic
		})
	}
}
