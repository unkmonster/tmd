package api

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
