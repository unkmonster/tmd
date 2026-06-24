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
	"/api/v1/auth/login", // login 端点需要在没有 JWT 时也能被调用
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
// 当 conf.api_key 非空时，支持两种认证方式：
//   1. JWT 会话令牌（首选） — 通过 validateSessionToken 验证
//   2. 原始 API Key（向后兼容） — 直接字符串比较
// 认证方式优先级：Authorization: Bearer <token> 头 > ?token= 查询参数（SSE 回退）
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

		if token == "" {
			writeAuth401(w, "missing")
			return
		}

		// Auth 管理端点（refresh/check）：允许过期 JWT 通过
		// 普通端点要求 JWT 完全有效（未过期）；auth 管理端点在 JWT 过期时也需要能处理
		// （客户端需要 refresh 端点来续期，需要 check 端点来查询状态）
		if isAuthManagementPath(r.URL.Path) {
			if _, err := validateSessionToken(token, apiKey); err == nil || isJWTExpiredError(err) {
				// JWT 签名有效（允许过期）→ 放行给 handler 处理
				next.ServeHTTP(w, r)
				return
			}
			// JWT 完全无效（签名错误/格式错误）→ 401
			writeAuth401(w, "invalid")
			return
		}

		// 方式一：尝试 JWT 验证（首选）
		jwtToken, jwtErr := validateSessionToken(token, apiKey)
		if jwtErr == nil && jwtToken != nil && jwtToken.Valid {
			next.ServeHTTP(w, r)
			return
		}

		// 方式二：回退到原始 API Key 比较（向后兼容）
		if token == apiKey {
			next.ServeHTTP(w, r)
			return
		}

		// 两者都失败 → 401
		//
		// X-Token-Type 帮助前端区分处理策略：
		//   "expired" → JWT 签名有效但已过期 —— 客户端应调用 /auth/refresh 续期
		//   "invalid" → JWT 完全无效或 API Key 错误 —— 客户端应弹登录框重新认证
		//
		// jwtErr 来自 validateSessionToken，isJWTExpiredError 识别过期。
		// 当一个签名有效的 JWT 过期后，token == apiKey 必然为 false
		//（JWT 字符串 != 短字符串 api_key），此时返回 "expired" 是安全的：
		// 前端先尝试 refresh，成功后恢复；若 refresh 也失败则 fallback 到登录。
		if isJWTExpiredError(jwtErr) {
			w.Header().Set("X-Token-Type", "expired")
		} else {
			w.Header().Set("X-Token-Type", "invalid")
		}
		writeAuth401(w, "invalid")
	})
}

// isAuthManagementPath 判断是否为认证管理端点（refresh/check）。
// 这些端点即使收到过期 JWT 也需要处理，因此 middleware 放行签名有效（允许过期）的 token。
func isAuthManagementPath(path string) bool {
	return path == "/api/v1/auth/refresh" || path == "/api/v1/auth/check"
}

// writeAuth401 writes a standard 401 Unauthorized response.
func writeAuth401(w http.ResponseWriter, tokenType string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="TMD API"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(NewErrorResponse("unauthorized"))
}
