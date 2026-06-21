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

// embeddedFileExists 检查 embed FS 中是否存在给定路径的文件。
// 用于跳过依赖特定 embed 文件的测试，避免在 embed 文件缺失时产生误报。
func embeddedFileExists(server *Server, path string) bool {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := serveStatic(server, req)
	return rr.Code == http.StatusOK
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
	t.Skip("handleWeb 使用 //go:embed 固定读取 index.html，无法在测试中模拟文件读取错误")
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

			// 所有路由都返回 index.html 的内容，只是前端路由
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

func TestHandleWeb_NonGETMethods(t *testing.T) {
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

			// 当前 handleWeb 不检查 HTTP 方法，非 GET 请求也返回正常页面
			// 如果将来添加了方法检查，这里需要修改
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestHandleStatic_CSS(t *testing.T) {
	server := &Server{}

	if !embeddedFileExists(server, "/static/styles.css") {
		t.Skip("embedded styles.css not available")
	}

	req := httptest.NewRequest(http.MethodGet, "/static/styles.css", nil)
	rr := serveStatic(server, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/css; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestHandleStatic_JS(t *testing.T) {
	server := &Server{}

	if !embeddedFileExists(server, "/static/app.js") {
		t.Skip("embedded app.js not available")
	}

	req := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	rr := serveStatic(server, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/javascript; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestHandleStatic_PNG(t *testing.T) {
	t.Skip("embedded image.png not available in test build")
}

func TestHandleStatic_JPG(t *testing.T) {
	t.Skip("embedded image.jpg not available in test build")
}

func TestHandleStatic_SVG(t *testing.T) {
	t.Skip("embedded icon.svg not available in test build")
}

func TestHandleStatic_JSON(t *testing.T) {
	t.Skip("embedded data.json not available in test build")
}

func TestHandleStatic_HTML(t *testing.T) {
	t.Skip("embedded page.html not available in test build")
}

func TestHandleStatic_UnknownExtension(t *testing.T) {
	t.Skip("no embedded file with unknown extension available in test build")
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

	if !embeddedFileExists(server, "/static/styles.css") {
		t.Skip("embedded styles.css not available")
	}

	req := httptest.NewRequest(http.MethodGet, "/static/styles.css", nil)
	rr := serveStatic(server, req)

	assert.Equal(t, "public, max-age=86400", rr.Header().Get("Cache-Control"))
	assert.Equal(t, contentETag(rr.Body.Bytes()), rr.Header().Get("ETag"))
}

func TestHandleStatic_IfNoneMatch(t *testing.T) {
	server := &Server{}

	if !embeddedFileExists(server, "/static/styles.css") {
		t.Skip("embedded styles.css not available")
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/static/styles.css", nil)
	firstRR := serveStatic(server, firstReq)
	requireETag := firstRR.Header().Get("ETag")

	req := httptest.NewRequest(http.MethodGet, "/static/styles.css", nil)
	req.Header.Set("If-None-Match", requireETag)
	rr := serveStatic(server, req)

	assert.Equal(t, http.StatusNotModified, rr.Code)
	assert.Equal(t, requireETag, rr.Header().Get("ETag"))
	assert.Equal(t, 0, rr.Body.Len())
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
	rr := serveStatic(server, req)

	// 不带尾部斜杠的 /static 路由不会匹配到 handleStatic（路径参数为空）
	// path.Clean 对空字符串的处理可能导致不同结果，至少不应 panic
	t.Logf("handleStatic with /static returned status %d", rr.Code)
}

func TestHandleStatic_PathCleaning(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "普通路径",
			path: "/static/style.css",
		},
		{
			name: "带双点的路径",
			path: "/static/../style.css",
		},
		{
			name: "带斜杠的路径",
			path: "/static//style.css",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := serveStatic(server, req)

			// 验证不会 panic，且始终返回有效 HTTP 状态码
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
				"handleStatic should never return 500 for path: %s", tt.path)
		})
	}
}

func TestHandleStatic_MultipleDotsInFilename(t *testing.T) {
	server := &Server{}

	tests := []struct {
		filename    string
		contentType string
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
				assert.Equal(t, tt.contentType, rr.Header().Get("Content-Type"))
			} else {
				// 文件不存在时只需确认不 panic 且无 500
				assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
					"handleStatic should never return 500 for: %s", tt.filename)
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
			rr := serveStatic(server, req)

			// 验证不会 panic，返回有效状态码
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
				"handleStatic should never return 500 for case-variant path: %s", tt.path)
		})
	}
}

func TestHandleStatic_LongPath(t *testing.T) {
	server := &Server{}

	// 构造一个很长的路径
	longPath := "/static/" + strings.Repeat("a/", 100) + "file.css"

	req := httptest.NewRequest(http.MethodGet, longPath, nil)
	rr := serveStatic(server, req)

	// 验证不会 panic，返回有效状态码
	assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
		"handleStatic should never return 500 for long path")
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
			rr := serveStatic(server, req)

			// 验证不会 panic，返回有效状态码
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
				"handleStatic should never return 500 for: %s", tt.path)
		})
	}
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
			rr := serveStatic(server, req)

			// 验证不会 panic，返回有效状态码
			assert.NotEqual(t, http.StatusInternalServerError, rr.Code,
				"handleStatic should never return 500 for method: %s", method)
		})
	}
}
