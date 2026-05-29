package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHandleSSETasks_ResponseHeaders(t *testing.T) {
	eventBus := NewEventBus()
	server := &Server{
		taskManager: NewTaskManager(eventBus),
		eventBus:    eventBus,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		server.handleSSETasks(rr, req)
		close(done)
	}()

	select {
	case <-done:
		assert.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", rr.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", rr.Header().Get("Connection"))
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Test timeout")
	}
}

func TestHandleSSETasks_NoFlusherSupport(t *testing.T) {
	eventBus := NewEventBus()
	server := &Server{
		taskManager: NewTaskManager(eventBus),
		eventBus:    eventBus,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := &sseMockResponseWriter{headers: make(http.Header)}

	server.handleSSETasks(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.statusCode)
}

func TestHandleSSETasks_ContextCancellation(t *testing.T) {
	eventBus := NewEventBus()
	server := &Server{
		taskManager: NewTaskManager(eventBus),
		eventBus:    eventBus,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		server.handleSSETasks(rr, req)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("SSE handler did not respond to context cancellation")
	}
}

func TestHandleSSETasks_ClientDisconnect(t *testing.T) {
	eventBus := NewEventBus()
	server := &Server{
		taskManager: NewTaskManager(eventBus),
		eventBus:    eventBus,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		server.handleSSETasks(rr, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("SSE handler should return immediately on cancelled context")
	}
}

func TestHandleSSETasks_InitialPush(t *testing.T) {
	eventBus := NewEventBus()
	server := &Server{
		taskManager: NewTaskManager(eventBus),
		eventBus:    eventBus,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	server.handleSSETasks(rr, req)

	body := rr.Body.String()
	assert.Contains(t, body, "event: tasks")
}

func TestHandleSSETasks_EventBusPush(t *testing.T) {
	eventBus := NewEventBus()
	server := &Server{
		taskManager: NewTaskManager(eventBus),
		eventBus:    eventBus,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/tasks", nil)
	rr := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		server.handleSSETasks(rr, req)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	eventBus.PublishNotification("task_completed", "test done", nil)

	<-done

	body := rr.Body.String()
	assert.Contains(t, body, "event: notification")
	assert.Contains(t, body, "test done")
}

func TestSSENamedEventFormat(t *testing.T) {
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

	sseData := "event: tasks\ndata: " + string(data) + "\n\n"
	assert.True(t, strings.HasPrefix(sseData, "event: tasks\n"))
	assert.Contains(t, sseData, "data: ")
	assert.Contains(t, sseData, "\n\n")
	assert.Contains(t, sseData, "task_123")
}

func TestWriteSSENamedEventReturnsMarshalError(t *testing.T) {
	server := &Server{}
	rr := httptest.NewRecorder()

	err := server.writeSSENamedEvent(rr, rr, "bad", func() {})

	assert.Error(t, err)
	assert.Empty(t, rr.Body.String())
}

func TestWriteSSEHeartbeat(t *testing.T) {
	rr := httptest.NewRecorder()

	err := writeSSEHeartbeat(rr)

	assert.NoError(t, err)
	assert.Equal(t, ": heartbeat\n\n", rr.Body.String())
}

func TestWriteSSENamedEventReturnsWriteError(t *testing.T) {
	server := &Server{}
	rr := &errorResponseWriter{}

	err := server.writeSSENamedEvent(rr, rr, "tasks", []*Task{})

	assert.Error(t, err)
}

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
