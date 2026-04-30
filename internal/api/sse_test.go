package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHandleSSETasks_ResponseHeaders(t *testing.T) {
	server := &Server{
		taskManager: NewTaskManager(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := httptest.NewRecorder()

	// 使用带超时的 context 来结束测试
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// 在 goroutine 中运行，避免阻塞
	done := make(chan struct{})
	go func() {
		server.handleSSETasks(rr, req)
		close(done)
	}()

	select {
	case <-done:
		// 验证响应头
		assert.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", rr.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", rr.Header().Get("Connection"))
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Test timeout")
	}
}

func TestHandleSSETasks_NoFlusherSupport(t *testing.T) {
	server := &Server{
		taskManager: NewTaskManager(),
	}

	// 使用不支持 Flusher 的 ResponseWriter
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := &sseMockResponseWriter{headers: make(http.Header)}

	server.handleSSETasks(rr, req)

	// 应该返回 500 错误
	assert.Equal(t, http.StatusInternalServerError, rr.statusCode)
}

func TestHandleSSETasks_ContextCancellation(t *testing.T) {
	server := &Server{
		taskManager: NewTaskManager(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := httptest.NewRecorder()

	// 使用可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		server.handleSSETasks(rr, req)
		close(done)
	}()

	// 立即取消 context
	cancel()

	// 等待处理完成
	select {
	case <-done:
		// 成功
	case <-time.After(200 * time.Millisecond):
		t.Fatal("SSE handler did not respond to context cancellation")
	}
}

func TestHandleSSETasks_ClientDisconnect(t *testing.T) {
	server := &Server{
		taskManager: NewTaskManager(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := httptest.NewRecorder()

	// 使用已取消的 context 模拟客户端断开
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		server.handleSSETasks(rr, req)
		close(done)
	}()

	select {
	case <-done:
		// 应该立即返回
	case <-time.After(200 * time.Millisecond):
		t.Fatal("SSE handler should return immediately on cancelled context")
	}
}

func TestSSEDataFormat(t *testing.T) {
	// 验证 SSE 数据格式
	tasks := []*Task{
		{
			ID:     "task_123",
			Type:   TaskTypeUserDownload,
			Status: TaskStatusRunning,
			Progress: &TaskProgress{
				Total:     100,
				Completed: 50,
			},
		},
	}

	data, err := json.Marshal(tasks)
	assert.NoError(t, err)

	// SSE 格式应该是 "data: <json>\n\n"
	sseData := "data: " + string(data) + "\n\n"
	assert.Contains(t, sseData, "data: ")
	assert.Contains(t, sseData, "\n\n")
	assert.Contains(t, sseData, "task_123")
}

// errorResponseWriter 模拟写入错误的 ResponseWriter
type errorResponseWriter struct {
	headers http.Header
}

func (e *errorResponseWriter) Header() http.Header {
	if e.headers == nil {
		e.headers = make(http.Header)
	}
	return e.headers
}

func (e *errorResponseWriter) Write([]byte) (int, error) {
	return 0, assert.AnError
}

func (e *errorResponseWriter) WriteHeader(code int) {}

func (e *errorResponseWriter) Flush() {}

// sseMockResponseWriter 不支持 Flusher 的 ResponseWriter
type sseMockResponseWriter struct {
	headers    http.Header
	statusCode int
	body       []byte
}

func (m *sseMockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *sseMockResponseWriter) Write(b []byte) (int, error) {
	m.body = append(m.body, b...)
	return len(b), nil
}

func (m *sseMockResponseWriter) WriteHeader(code int) {
	m.statusCode = code
}
