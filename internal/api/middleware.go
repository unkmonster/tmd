package api

import (
	"bufio"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// responseRecorder 包装 http.ResponseWriter 以记录状态码
// 同时实现了 http.Flusher、http.Hijacker 和 http.Pusher 接口
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

// Flush 实现 http.Flusher 接口（SSE 需要）
func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack 实现 http.Hijacker 接口（WebSocket 需要）
func (rr *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rr.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push 实现 http.Pusher 接口（HTTP/2 Server Push 需要）
func (rr *responseRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := rr.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// loggingMiddleware 请求日志中间件
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rr, r)
		log.Infof("[%s] %s %s %d (%v)", r.Method, r.URL.Path, r.RemoteAddr, rr.statusCode, time.Since(start))
	})
}
