package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// mockResponseWriter 用于测试的 ResponseWriter
// 实现了 http.Flusher, http.Hijacker, http.Pusher 接口
type mockResponseWriter struct {
	headers    http.Header
	statusCode int
	body       []byte
	flushed    bool
	hijacked   bool
	written    bool
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		headers: make(http.Header),
	}
}

func (m *mockResponseWriter) Header() http.Header {
	return m.headers
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	if !m.written {
		m.statusCode = http.StatusOK
		m.written = true
	}
	m.body = append(m.body, b...)
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(code int) {
	if !m.written {
		m.statusCode = code
		m.written = true
	}
}

func (m *mockResponseWriter) Flush() {
	m.flushed = true
}

func (m *mockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijacked = true
	return nil, nil, nil
}

// mockHijacker 实现 http.Hijacker 接口
type mockHijacker struct {
	*mockResponseWriter
}

func (m *mockHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

// mockFlusher 实现 http.Flusher 接口
type mockFlusher struct {
	*mockResponseWriter
}

func (m *mockFlusher) Flush() {
	m.flushed = true
}

// mockPusher 实现 http.Pusher 接口
type mockPusher struct {
	*mockResponseWriter
	pushCalled bool
}

func (m *mockPusher) Push(target string, opts *http.PushOptions) error {
	m.pushCalled = true
	return nil
}

func TestResponseRecorder_WriteHeader(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		wantCode   int
	}{
		{
			name:     "设置 200",
			code:     http.StatusOK,
			wantCode: http.StatusOK,
		},
		{
			name:     "设置 404",
			code:     http.StatusNotFound,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "设置 500",
			code:     http.StatusInternalServerError,
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockResponseWriter()
			rr := &responseRecorder{
				ResponseWriter: mock,
				statusCode:     http.StatusOK,
			}

			rr.WriteHeader(tt.code)
			assert.Equal(t, tt.wantCode, rr.statusCode)
			assert.Equal(t, tt.wantCode, mock.statusCode)
		})
	}
}

func TestResponseRecorder_Flush(t *testing.T) {
	t.Run("底层支持 Flusher", func(t *testing.T) {
		mock := &mockFlusher{mockResponseWriter: newMockResponseWriter()}
		rr := &responseRecorder{ResponseWriter: mock}

		rr.Flush()
		assert.True(t, mock.flushed)
	})

	t.Run("底层支持 Flusher（通过 mockResponseWriter）", func(t *testing.T) {
		mock := newMockResponseWriter()
		rr := &responseRecorder{ResponseWriter: mock}

		// mockResponseWriter 实现了 Flush()，所以会被调用
		rr.Flush()
		assert.True(t, mock.flushed)
	})
}

func TestResponseRecorder_Hijack(t *testing.T) {
	t.Run("底层支持 Hijacker", func(t *testing.T) {
		mock := &mockHijacker{mockResponseWriter: newMockResponseWriter()}
		rr := &responseRecorder{ResponseWriter: mock}

		conn, rw, err := rr.Hijack()
		assert.NoError(t, err)
		assert.Nil(t, conn)
		assert.Nil(t, rw)
	})

	t.Run("底层不支持 Hijacker", func(t *testing.T) {
		// 使用 httptest.ResponseRecorder，它实现了 Hijacker
		mock := httptest.NewRecorder()
		rr := &responseRecorder{ResponseWriter: mock}

		conn, rw, err := rr.Hijack()
		// httptest.ResponseRecorder 实现了 Hijacker，但会返回错误
		// 因为 hijack 不支持
		assert.Error(t, err)
		assert.Nil(t, conn)
		assert.Nil(t, rw)
	})
}

func TestResponseRecorder_Push(t *testing.T) {
	t.Run("底层支持 Pusher", func(t *testing.T) {
		mock := &mockPusher{mockResponseWriter: newMockResponseWriter()}
		rr := &responseRecorder{ResponseWriter: mock}

		err := rr.Push("/test", nil)
		assert.NoError(t, err)
		assert.True(t, mock.pushCalled)
	})

	t.Run("底层不支持 Pusher", func(t *testing.T) {
		mock := newMockResponseWriter()
		rr := &responseRecorder{ResponseWriter: mock}

		err := rr.Push("/test", nil)
		assert.Error(t, err)
		assert.Equal(t, http.ErrNotSupported, err)
	})
}

func TestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
	}{
		{
			name:       "GET 请求 200",
			method:     http.MethodGet,
			path:       "/api/v1/health",
			statusCode: http.StatusOK,
		},
		{
			name:       "POST 请求 201",
			method:     http.MethodPost,
			path:       "/api/v1/users/test/download",
			statusCode: http.StatusAccepted,
		},
		{
			name:       "GET 请求 404",
			method:     http.MethodGet,
			path:       "/api/v1/notfound",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "GET 请求 500",
			method:     http.MethodGet,
			path:       "/api/v1/error",
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			middleware := loggingMiddleware(handler)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			rr := httptest.NewRecorder()

			middleware.ServeHTTP(rr, req)

			assert.Equal(t, tt.statusCode, rr.Code)
		})
	}
}

func TestLoggingMiddleware_MultipleRequests(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := loggingMiddleware(handler)

	// 发送多个请求
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
	}

	assert.Equal(t, 5, callCount)
}

func TestLoggingMiddleware_ResponseWriterWrapping(t *testing.T) {
	var capturedWriter http.ResponseWriter

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedWriter = w
		w.WriteHeader(http.StatusOK)
	})

	middleware := loggingMiddleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	// 验证 ResponseWriter 被包装
	assert.NotNil(t, capturedWriter)

	// 验证类型
	_, ok := capturedWriter.(*responseRecorder)
	assert.True(t, ok)
}

func TestResponseRecorder_RecordsStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	})

	middleware := loggingMiddleware(handler)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "Created", rr.Body.String())
}

func TestResponseRecorder_DefaultStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 不调用 WriteHeader，只写入数据
		w.Write([]byte("OK"))
	})

	middleware := loggingMiddleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	// 默认状态码应该是 200
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestResponseRecorder_InterfaceCompliance(t *testing.T) {
	// 测试 responseRecorder 实现了所有需要的接口
	var _ http.ResponseWriter = (*responseRecorder)(nil)
	var _ http.Flusher = (*responseRecorder)(nil)
	var _ http.Hijacker = (*responseRecorder)(nil)
	var _ http.Pusher = (*responseRecorder)(nil)
}

func TestLoggingMiddleware_WithDifferentMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := loggingMiddleware(handler)
			req := httptest.NewRequest(method, "/test", nil)
			rr := httptest.NewRecorder()

			middleware.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestResponseRecorder_WithRealHTTPServer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	middleware := loggingMiddleware(handler)
	server := httptest.NewServer(middleware)
	defer server.Close()

	resp, err := http.Get(server.URL + "/test")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// 读取响应体
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	assert.Contains(t, body, `"status":"ok"`)
}

func TestResponseRecorder_HeaderModification(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.Header().Add("X-Multi-Header", "value1")
		w.Header().Add("X-Multi-Header", "value2")
		w.WriteHeader(http.StatusOK)
	})

	middleware := loggingMiddleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	assert.Equal(t, "custom-value", rr.Header().Get("X-Custom-Header"))
	assert.Equal(t, []string{"value1", "value2"}, rr.Header()["X-Multi-Header"])
}

func TestResponseRecorder_LargeResponse(t *testing.T) {
	largeBody := strings.Repeat("x", 1024*1024) // 1MB

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	})

	middleware := loggingMiddleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, len(largeBody), rr.Body.Len())
}

// ============================================
// Auth Middleware Tests
// ============================================

func TestIsPublicPath(t *testing.T) {
	tests := []struct {
		path   string
		public bool
	}{
		// 公开路径
		{"/", true},
		{"/favicon.ico", true},
		{"/tasks", true},
		{"/data", true},
		{"/schedules", true},
		{"/system", true},
		{"/logs", true},
		{"/api/v1/health", true},
		{"/api/v1/health?foo=bar", true},
		{"/api/v1/config/theme", true},
		{"/api/v1/config/themes", true},
		{"/api/v1/config/theme?foo=bar", true},
		{"/static/app.js", true},
		{"/static/css/styles.css", true},
		{"/static/", true},
		// 非公开路径
		{"/api/v1/tasks", false},
		{"/api/v1/tasks/stats", false},
		{"/api/v1/users/elonmusk/download", false},
		{"/api/v1/config", false},
		{"/api/v1/db/users", false},
		{"/api/v1/sse/tasks", false},
		{"/api/v1/logs/stream", false},
		{"/api/v1/server/shutdown", false},
		{"/some/other/path", false},
		{"/not-static/file.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isPublicPath(tt.path)
			assert.Equal(t, tt.public, got, "isPublicPath(%q)", tt.path)
		})
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name  string
		auth  string
		token string
	}{
		{"无 Authorization 头", "", ""},
		{"非 Bearer 前缀", "Token mytoken", ""},
		{"小写 bearer", "bearer mytoken", ""},
		{"标准 Bearer", "Bearer mytoken123", "mytoken123"},
		{"Bearer 带多余空格", "Bearer   mytoken  ", "mytoken"},
		{"Bearer 空 token", "Bearer ", ""},
		{"Bearer 复杂 token", "Bearer sec-r3t_K#12!@", "sec-r3t_K#12!@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			got := extractBearerToken(req)
			assert.Equal(t, tt.token, got)
		})
	}
}

func TestAuthMiddleware_NoKey_PassesThrough(t *testing.T) {
	// api_key = "" 时，所有请求应直接放行
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = ""

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("passed"))
	}))

	// 无需 token 的请求
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "passed", rr.Body.String())
}

func TestAuthMiddleware_CorrectBearerToken_Succeeds(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "my-test-key"

	// 获取 JWT（原始 API Key 不再直接支持，必须用 JWT）
	tokenStr, err := generateSessionToken("my-test-key")
	assert.NoError(t, err)

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authenticated"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "authenticated", rr.Body.String())
}

func TestAuthMiddleware_WrongBearerToken_Returns401(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "my-test-key"

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	tests := []struct {
		name  string
		token string
	}{
		{"无 token", ""},
		{"错误 token", "wrong-key"},
		{"空 Bearer", "Bearer "},
		{"缺少 Bearer 前缀", "my-test-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", tt.token)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			assert.Equal(t, `Bearer realm="TMD API"`, rr.Header().Get("WWW-Authenticate"))

			var resp APIResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.False(t, resp.Success)
			assert.Equal(t, "unauthorized", resp.Error)
		})
	}
}

func TestAuthMiddleware_SSEQueryParam_Succeeds(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "sse-key"

	// SSE uses ?token= with JWT (raw API Key no longer accepted)
	tokenStr, err := generateSessionToken("sse-key")
	assert.NoError(t, err)

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("sse-authenticated"))
	}))

	// SSE 使用 ?token=<jwt> 而非 Authorization 头
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks?token="+tokenStr, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "sse-authenticated", rr.Body.String())
}

func TestAuthMiddleware_SSEQueryParam_WrongToken_Returns401(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "sse-key"

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks?token=wrong", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_PublicPaths_BypassAuth(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "secret"

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("public-path-reached"))
	}))

	publicPaths := []string{
		"/",
		"/favicon.ico",
		"/tasks",
		"/data",
		"/schedules",
		"/system",
		"/logs",
		"/api/v1/health",
		"/api/v1/health?foo=bar",
		"/api/v1/config/theme",
		"/api/v1/config/themes",
		"/static/app.js",
		"/static/css/styles.css",
	}

	for _, p := range publicPaths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			// 故意不带 token，验证公开路径可以绕过
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code, "public path %q should bypass auth", p)
			assert.Equal(t, "public-path-reached", rr.Body.String())
		})
	}
}

func TestAuthMiddleware_ProtectedPaths_RequireAuth(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "secret"

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for", r.URL.Path)
	}))

	protectedPaths := []string{
		"/api/v1/tasks",
		"/api/v1/tasks/stats",
		"/api/v1/users/elonmusk/download",
		"/api/v1/config",
		"/api/v1/db/users",
		"/api/v1/sse/tasks",
		"/api/v1/logs/stream",
		"/api/v1/server/shutdown",
	}

	for _, p := range protectedPaths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusUnauthorized, rr.Code, "protected path %q should require auth", p)
		})
	}
}

func TestAuthMiddleware_BuildHandlerIntegration(t *testing.T) {
	// 集成测试：通过 buildHandler 验证完整的中间件链
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "integ-test-key"

	handler := server.buildHandler()

	t.Run("公开路径无需 token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Web UI 路径无需 token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("API 路径无 token 返回 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		var resp APIResponse
		json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.False(t, resp.Success)
		assert.Equal(t, "unauthorized", resp.Error)
	})

	t.Run("API 路径带正确 JWT 返回 200", func(t *testing.T) {
		// 生成 JWT 替代原始 API Key
		token, err := generateSessionToken("integ-test-key")
		assert.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestAuthMiddleware_OPTIONS_PreflightPasses(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "secret"

	handler := server.buildHandler()

	// OPTIONS 预检请求应不被 auth 拦截：auth 在 CORS 内层
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/tasks", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// CORS 中间件处理 OPTIONS 返回 204（No Content），auth 不被触发
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}

// ============================================
// JWT Auth Tests
// ============================================

func TestGenerateAndValidateJWT(t *testing.T) {
	apiKey := "test-api-key"

	// Generate token
	tokenStr, err := generateSessionToken(apiKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	// Validate with correct key
	token, err := validateSessionToken(tokenStr, apiKey)
	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.True(t, token.Valid)

	// Validate with wrong key should fail
	_, err = validateSessionToken(tokenStr, "wrong-key")
	assert.Error(t, err)

	// Validate garbage string
	_, err = validateSessionToken("not-a-jwt", apiKey)
	assert.Error(t, err)
}


func TestAuthMiddleware_JWT_Succeeds(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "jwt-test-key"

	// Generate a valid JWT
	tokenStr, err := generateSessionToken("jwt-test-key")
	assert.NoError(t, err)

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("jwt-authenticated"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "jwt-authenticated", rr.Body.String())
}

func TestAuthMiddleware_JWT_SSEQueryParam_Succeeds(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "jwt-sse-key"

	tokenStr, err := generateSessionToken("jwt-sse-key")
	assert.NoError(t, err)

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("jwt-sse-authenticated"))
	}))

	// SSE uses ?token= query param
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks?token="+tokenStr, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "jwt-sse-authenticated", rr.Body.String())
}

func TestAuthMiddleware_JWT_WrongKey_Returns401(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "real-key"

	// Generate JWT with a different key
	tokenStr, err := generateSessionToken("wrong-key")
	assert.NoError(t, err)

	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_RawAPIKey_NowRejected(t *testing.T) {
	// After JWT-only migration, raw API Key should return 401
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "my-test-key"

	called := false
	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer my-test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "raw API Key should be rejected after JWT-only migration")
	assert.False(t, called, "handler should not be called for raw API Key")

	var resp APIResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "unauthorized", resp.Error)
}

func TestIsJWTExpiredError(t *testing.T) {
	// Test the error type detection
	// (actual expired JWT is tested in TestAuthMiddleware_BuildHandler_JWT_Integration)
	assert.False(t, isJWTExpiredError(nil))
	assert.False(t, isJWTExpiredError(fmt.Errorf("some other error")))
}

func TestAuthLogin_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "login-test-key"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleAuthLogin(w, r)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.Header.Set("Authorization", "Bearer login-test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
			ExpiresIn int    `json:"expires_in"`
		} `json:"data"`
	}
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Data.Token)
	assert.Greater(t, resp.Data.ExpiresIn, 3500) // ~1 hour
}

func TestAuthLogin_WrongKey_Returns401(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "real-key"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleAuthLogin(w, r)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthLogin_NoKey_Returns401(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "real-key"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleAuthLogin(w, r)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthRefresh_ValidJWT_Succeeds(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "refresh-test-key"

	// First get a valid JWT
	tokenStr, err := generateSessionToken("refresh-test-key")
	assert.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleAuthRefresh(w, r)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
			ExpiresIn int    `json:"expires_in"`
		} `json:"data"`
	}
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Data.Token)
	assert.Greater(t, resp.Data.ExpiresIn, 3500) // ~1 hour
}

func TestAuthCheck_ValidJWT_ReturnsAuthenticated(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "check-key"

	tokenStr, err := generateSessionToken("check-key")
	assert.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleAuthCheck(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/check", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Authenticated bool `json:"authenticated"`
			Valid         bool `json:"valid"`
		} `json:"data"`
	}
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.True(t, resp.Data.Authenticated)
	assert.True(t, resp.Data.Valid)
}

func TestAuthCheck_NoToken_ReturnsUnauthenticated(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "check-key"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleAuthCheck(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/check", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Authenticated bool `json:"authenticated"`
		} `json:"data"`
	}
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.False(t, resp.Data.Authenticated)
}

func TestAuthMiddleware_BuildHandler_JWT_Integration(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.config.APIKey = "integ-jwt-key"

	tokenStr, err := generateSessionToken("integ-jwt-key")
	assert.NoError(t, err)

	handler := server.buildHandler()

	t.Run("JWT 认证访问受保护路径", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("JWT 过期后返回 401", func(t *testing.T) {
		// This test generates a token with an expired timestamp manually
		// to verify the middleware detects expiry
		apiKey := "integ-jwt-key"
		now := time.Now()
		claims := jwtClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    jwtIssuer,
				Subject:   jwtSubject,
				IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
				ID:        fmt.Sprintf("%d", now.UnixNano()),
			},
		}
		expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		secret := deriveJWTSecret(apiKey)
		expiredStr, err := expiredToken.SignedString(secret)
		assert.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Bearer "+expiredStr)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Equal(t, "expired", rr.Header().Get("X-Token-Type"))
	})

	t.Run("原始 API Key 不再可用（纯 JWT 模式）", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Bearer integ-jwt-key")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "raw API Key should be rejected after JWT-only migration")
	})
}
