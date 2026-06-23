package api

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"strings"
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

// Unwrap exposes the underlying writer for http.ResponseController.
func (rr *responseRecorder) Unwrap() http.ResponseWriter {
	return rr.ResponseWriter
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

// securityHeadersMiddleware 安全响应头中间件
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// authBearerPrefix is the prefix for Bearer tokens in the Authorization header.
const authBearerPrefix = "Bearer "

// publicPaths 是不需要认证的路径前缀。
// Web UI 页面必须公开（否则用户看不到登录界面），静态资源和健康检查同理。
var publicPathPrefixes = []string{
	"/api/v1/health",
	"/api/v1/config/theme", // theme 切换器由内联 JS 调用，不经过 api 对象
	"/static/",
}

// isPublicPath 检查路径是否属于公开路径白名单。
func isPublicPath(path string) bool {
	// 根路径和所有 SPA 页面路由
	if path == "/" || path == "/favicon.ico" {
		return true
	}
	// SPA 页面路由
	spaPages := []string{"/tasks", "/data", "/schedules", "/system", "/logs"}
	for _, p := range spaPages {
		if path == p {
			return true
		}
	}
	// 前缀匹配
	for _, prefix := range publicPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// extractBearerToken 从请求头中提取 Bearer token
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, authBearerPrefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(authBearerPrefix):])
}

// authMiddleware 认证中间件。
// 当 conf.api_key 为空时放行所有请求（向后兼容）。
// 当 conf.api_key 非空时检查 Authorization: Bearer <token> 头，
// SSE 端点可回退到 ?token= 查询参数。
// 公开路径（健康检查、Web UI 页面、静态文件）免认证。
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 公开路径直接放行
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// 读取当前配置中的 API Key
		s.configMu.RLock()
		apiKey := s.config.APIKey
		s.configMu.RUnlock()

		// API Key 为空时不做认证（向后兼容）
		if apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// 尝试从 Authorization 头提取 token
		token := extractBearerToken(r)
		// SSE 端点无法设置自定义头，回退到查询参数
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token != apiKey {
			w.Header().Set("WWW-Authenticate", `Bearer realm="TMD API"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(NewErrorResponse("unauthorized"))
			return
		}

		next.ServeHTTP(w, r)
	})
}
