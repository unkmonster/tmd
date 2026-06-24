package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/consolelog"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/scheduler"
	"github.com/unkmonster/tmd/internal/service"
)

type Server struct {
	client            *resty.Client
	additionalClients []*resty.Client
	db                *sqlx.DB
	config            *config.Config
	appRootPath       string
	configMu          sync.RWMutex
	taskManager       *TaskManager
	downloadService   service.DownloadService
	downloadQueue     *DownloadQueue
	logWriter         io.Closer
	logHub            *consolelog.Hub
	httpServer        *http.Server
	shutdownOnce      sync.Once
	shutdownDone      chan struct{}
	schedulesMu       sync.Mutex
	schedulerMu       sync.RWMutex
	scheduler         *scheduler.Scheduler
	eventBus          *EventBus
	authRateLimit     *authRateLimiter
}

func NewServer(client *resty.Client, additionalClients []*resty.Client, db *sqlx.DB, config *config.Config, appRootPath string, logWriter io.Closer) *Server {
	return NewServerWithConsoleLogHub(client, additionalClients, db, config, appRootPath, logWriter, consolelog.DefaultHub())
}

func NewServerWithConsoleLogHub(client *resty.Client, additionalClients []*resty.Client, db *sqlx.DB, config *config.Config, appRootPath string, logWriter io.Closer, logHub *consolelog.Hub) *Server {
	if logHub == nil {
		logHub = consolelog.DefaultHub()
	}

	eventBus := NewEventBus()

	s := &Server{
		client:            client,
		additionalClients: additionalClients,
		db:                db,
		config:            config,
		appRootPath:       appRootPath,
		logWriter:         logWriter,
		logHub:            logHub,
		taskManager:       NewTaskManager(eventBus),
		shutdownDone:      make(chan struct{}),
		eventBus:          eventBus,
		authRateLimit:     defaultAuthRateLimiter,
	}

	s.authRateLimit.startCleanupLoop()

	// 配置副本：使 service.Dependencies 持有独立的 Config 副本，
	// 避免与 Server.config 共享同一指针。这样 handleUpdateConfigRaw
	// 的 *s.config = *testConf 只影响 Server 显示用配置，不影响运行时依赖，
	// 确保所有运行时组件行为一致（均需重启后生效）。
	configCopy := *config
	downloadService, err := service.NewDownloadService(&service.Dependencies{
		Client:            client,
		AdditionalClients: additionalClients,
		DB:                db,
		Config:            &configCopy,
		ListSyncManager:   downloading.NewListSyncManager(db),
	})
	if err != nil {
		log.Fatalf("failed to create download service: %v", err)
	}
	s.downloadService = downloadService
	s.downloadQueue = NewDownloadQueue(s)

	schedulesPath := filepath.Join(appRootPath, "schedules.yaml")
	sched, err := scheduler.New(schedulesPath, s.scheduledDownload)
	if err != nil {
		log.Warnf("[scheduler] Failed to initialize scheduler: %v", err)
	} else {
		sched.OnStatusChange = s.handleScheduleStatusChange
		s.scheduler = sched
	}

	return s
}

func (s *Server) getScheduler() *scheduler.Scheduler {
	s.schedulerMu.RLock()
	defer s.schedulerMu.RUnlock()
	return s.scheduler
}

func (s *Server) buildHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.handleAuthRefresh)
	mux.HandleFunc("GET /api/v1/auth/check", s.handleAuthCheck)
	mux.HandleFunc("POST /api/v1/users/{screen_name}/download", s.handleUserDownloadRoute)
	mux.HandleFunc("POST /api/v1/users/{screen_name}/profile", s.handleUserProfileRoute)
	mux.HandleFunc("POST /api/v1/users/{screen_name}/mark", s.handleUserMarkRoute)
	mux.HandleFunc("POST /api/v1/users/{screen_name}/following/download", s.handleFollowingDownloadRoute)
	mux.HandleFunc("POST /api/v1/users/{screen_name}/following/mark", s.handleFollowingMarkRoute)
	mux.HandleFunc("POST /api/v1/lists/{list_id}/download", s.handleListDownloadRoute)
	mux.HandleFunc("POST /api/v1/lists/{list_id}/profile", s.handleListProfileRoute)
	mux.HandleFunc("POST /api/v1/lists/{list_id}/mark", s.handleListMarkRoute)
	mux.HandleFunc("POST /api/v1/json/file/download", s.handleJsonFileDownload)
	mux.HandleFunc("POST /api/v1/json/folder/download", s.handleJsonFolderDownload)
	mux.HandleFunc("POST /api/v1/batch/download", s.handleBatchDownload)
	mux.HandleFunc("POST /api/v1/batch/mark", s.handleBatchMark)
	mux.HandleFunc("GET /api/v1/tasks", s.handleTasks)
	mux.HandleFunc("GET /api/v1/tasks/stats", s.handleTaskStats)
	mux.HandleFunc("GET /api/v1/tasks/{task_id}", s.handleGetTask)
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/cancel", s.handleCancelTask)
	mux.HandleFunc("POST /api/v1/tasks/cancel-queued", s.handleCancelQueuedTasks)
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/retry", s.handleRetryTask)
	mux.HandleFunc("DELETE /api/v1/tasks/{task_id}", s.handleDeleteTask)
	mux.HandleFunc("GET /api/v1/errors", s.handleErrors)
	mux.HandleFunc("POST /api/v1/errors/retry", s.handleRetryAllFailed)
	mux.HandleFunc("DELETE /api/v1/errors", s.handleClearErrors)

	mux.HandleFunc("GET /{$}", s.handleWeb)
	mux.HandleFunc("GET /favicon.ico", s.handleFavicon)
	mux.HandleFunc("GET /tasks", s.handleWeb)
	mux.HandleFunc("GET /data", s.handleWeb)
	mux.HandleFunc("GET /schedules", s.handleWeb)
	mux.HandleFunc("GET /system", s.handleWeb)
	mux.HandleFunc("GET /logs", s.handleWeb)
	mux.HandleFunc("GET /static/{$}", s.handleStatic)
	mux.HandleFunc("GET /static/{path...}", s.handleStatic)
	mux.HandleFunc("GET /api/v1/sse/tasks", s.handleSSETasks)

	mux.HandleFunc("GET /api/v1/db/users", s.handleDBUsers)
	mux.HandleFunc("GET /api/v1/db/users/{id}", s.handleDBUserDetail)
	mux.HandleFunc("PATCH /api/v1/db/users/{id}", s.handleDBUserUpdate)
	mux.HandleFunc("DELETE /api/v1/db/users/{id}", s.handleDBUserDelete)
	mux.HandleFunc("GET /api/v1/db/users/{id}/previous-names", s.handleDBUserPreviousNames)
	mux.HandleFunc("GET /api/v1/db/users/{id}/entities", s.handleDBUserEntitiesByUserID)
	mux.HandleFunc("GET /api/v1/db/users/{id}/links", s.handleDBUserLinksByUserID)

	mux.HandleFunc("GET /api/v1/db/lists", s.handleDBLists)
	mux.HandleFunc("GET /api/v1/db/lists/{id}", s.handleDBListDetail)
	mux.HandleFunc("PATCH /api/v1/db/lists/{id}", s.handleDBListUpdate)
	mux.HandleFunc("DELETE /api/v1/db/lists/{id}", s.handleDBListDelete)
	mux.HandleFunc("GET /api/v1/db/lists/{id}/entities", s.handleDBLstEntitiesByListID)

	mux.HandleFunc("GET /api/v1/db/user-entities", s.handleDBUserEntities)
	mux.HandleFunc("GET /api/v1/db/user-entities/{id}", s.handleDBUserEntityDetail)
	mux.HandleFunc("PATCH /api/v1/db/user-entities/{id}", s.handleDBUserEntityUpdate)
	mux.HandleFunc("DELETE /api/v1/db/user-entities/{id}", s.handleDBUserEntityDelete)

	mux.HandleFunc("GET /api/v1/db/list-entities", s.handleDBListEntities)
	mux.HandleFunc("GET /api/v1/db/list-entities/{id}", s.handleDBListEntityDetail)
	mux.HandleFunc("PATCH /api/v1/db/list-entities/{id}", s.handleDBListEntityUpdate)
	mux.HandleFunc("DELETE /api/v1/db/list-entities/{id}", s.handleDBListEntityDelete)

	mux.HandleFunc("GET /api/v1/db/user-links", s.handleDBUserLinks)
	mux.HandleFunc("GET /api/v1/db/user-links/{id}", s.handleDBUserLinkDetail)
	mux.HandleFunc("PATCH /api/v1/db/user-links/{id}", s.handleDBUserLinkUpdate)
	mux.HandleFunc("DELETE /api/v1/db/user-links/{id}", s.handleDBUserLinkDelete)
	mux.HandleFunc("GET /api/v1/db/user-previous-names", s.handleDBPreviousNames)
	mux.HandleFunc("GET /api/v1/db/stats", s.handleDBStats)

	mux.HandleFunc("GET /api/v1/config/theme", s.handleGetTheme)
	mux.HandleFunc("POST /api/v1/config/theme", s.handleSetTheme)
	mux.HandleFunc("GET /api/v1/config/themes", s.handleGetThemes)
	mux.HandleFunc("GET /api/v1/config", s.handleConfig)
	mux.HandleFunc("GET /api/v1/config/raw", s.handleGetConfigRaw)
	mux.HandleFunc("PUT /api/v1/config/raw", s.handleUpdateConfigRaw)
	mux.HandleFunc("GET /api/v1/config/fields", s.handleGetConfigFields)
	mux.HandleFunc("PUT /api/v1/config/fields", s.handleSaveConfigFields)
	mux.HandleFunc("GET /api/v1/cookies", s.handleGetCookies)
	mux.HandleFunc("PUT /api/v1/cookies", s.handleSaveCookies)
	mux.HandleFunc("GET /api/v1/cookies/raw", s.handleGetCookiesRaw)
	mux.HandleFunc("PUT /api/v1/cookies/raw", s.handleUpdateCookiesRaw)
	mux.HandleFunc("POST /api/v1/server/shutdown", s.handleServerShutdown)
	mux.HandleFunc("GET /api/v1/queue/status", s.handleQueueStatus)
	mux.HandleFunc("GET /api/v1/logs", s.handleGetLogs)
	mux.HandleFunc("GET /api/v1/logs/stats", s.handleLogStats)
	mux.HandleFunc("GET /api/v1/logs/export", s.handleLogExport)
	mux.HandleFunc("GET /api/v1/logs/stream", s.handleLogStream)

	mux.HandleFunc("GET /api/v1/schedules", s.handleGetSchedules)
	mux.HandleFunc("PUT /api/v1/schedules", s.handleReplaceSchedules)
	mux.HandleFunc("POST /api/v1/schedules", s.handleCreateSchedule)
	mux.HandleFunc("GET /api/v1/schedules/raw", s.handleGetSchedulesRaw)
	mux.HandleFunc("PUT /api/v1/schedules/raw", s.handleUpdateSchedulesRaw)
	mux.HandleFunc("POST /api/v1/schedules/reload", s.handleReloadSchedules)
	mux.HandleFunc("POST /api/v1/schedules/validate", s.handleValidateSchedule)
	mux.HandleFunc("POST /api/v1/schedules/trigger-all", s.handleTriggerAllSchedules)
	mux.HandleFunc("GET /api/v1/schedules/stats", s.handleScheduleStats)
	mux.HandleFunc("PUT /api/v1/schedules/{id}", s.handleUpdateSchedule)
	mux.HandleFunc("DELETE /api/v1/schedules/{id}", s.handleDeleteSchedule)
	mux.HandleFunc("PATCH /api/v1/schedules/{id}/enabled", s.handleSetScheduleEnabled)
	mux.HandleFunc("POST /api/v1/schedules/{id}/trigger", s.handleTriggerSchedule)

	var handler http.Handler = mux

	// authMiddleware 在 CORS 内层：OPTIONS 预检请求由 CORS 直接处理，不经过 auth。
	// 当 api_key 为空时 authMiddleware 直接放行，不改变现有行为。
	handler = s.authMiddleware(handler)

	// 注意中间件的包裹顺序：最外层是 Logging，里面一层是 CORS，最里面是 Mux + Auth。
	// 这样 Logging 就能记录到所有请求，包括那些被 CORS 拦截的 OPTIONS 预检请求。
	handler = cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}).Handler(handler)

	// API 版本头中间件：在所有响应中添加 API-Version 头，为未来版本迁移预留
	corsHandler := handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("API-Version", "v1")
		corsHandler.ServeHTTP(w, r)
	})

	handler = loggingMiddleware(handler)
	handler = securityHeadersMiddleware(handler)
	return handler
}

func (s *Server) Start(port int) error {
	handler := s.buildHandler()

	addr := fmt.Sprintf(":%d", port)
	log.Infoln("API server starting on", addr)
	log.Infof("Visit %s to get started", color.FgLightBlue.Render(fmt.Sprintf("http://localhost%s/", addr)))

	s.httpServer = &http.Server{
		Addr:        addr,
		Handler:     handler,
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	if sched := s.getScheduler(); sched != nil {
		sched.Start()
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) handleGetTheme(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]string{
		"theme": getFrontendTheme(),
	}))
}

func (s *Server) handleSetTheme(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Theme string `json:"theme"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if !setFrontendTheme(req.Theme) {
		s.writeError(w, http.StatusBadRequest, "Invalid theme: directory not found or missing index.html")
		return
	}

	log.Infof("[theme] switched to %s", req.Theme)
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]string{
		"theme": getFrontendTheme(),
	}))
}

func (s *Server) handleGetThemes(w http.ResponseWriter, r *http.Request) {
	themes := listThemes()
	if themes == nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to list themes")
		return
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"themes":  themes,
		"current": getFrontendTheme(),
	}))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(); err != nil {
		s.writeJSON(w, http.StatusServiceUnavailable, NewErrorResponse("Database unavailable"))
		return
	}

	resp := HealthResponse{
		Status:    "ok",
		Version:   buildVersion(),
		Timestamp: time.Now().UTC(),
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#2563eb"><path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"/><line x1="4" y1="22" x2="4" y2="15"/></svg>`))
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Warnf("Failed to write response: %v", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, NewErrorResponse(message))
}

func (s *Server) writeErrorDetail(w http.ResponseWriter, status int, message string, detail string) {
	s.writeJSON(w, status, NewErrorResponseWithDetail(message, detail))
}
func (s *Server) handleServerShutdown(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Server shutting down...",
		"action":  "shutdown",
	}))

	log.Infoln("[server] received shutdown request, performing graceful shutdown...")

	go func() {
		time.Sleep(500 * time.Millisecond)
		s.GracefulShutdown("shutdown")
	}()
}

func (s *Server) WaitForShutdown() {
	<-s.shutdownDone
}

func (s *Server) GracefulShutdown(reason string) {
	s.shutdownOnce.Do(func() {
		if s.eventBus != nil {
			s.eventBus.PublishServerShutdown("服务器正在关闭: " + reason)
			time.Sleep(100 * time.Millisecond)
		}

		log.Infof("[server] graceful shutdown started (reason: %s)", reason)

		if s.taskManager != nil {
			s.taskManager.CancelAllTasks()
			s.taskManager.Close()
			log.Infoln("[server] all running tasks cancelled")
			time.Sleep(1 * time.Second)
		}
		if s.downloadQueue != nil {
			s.downloadQueue.CloseAndWait(15 * time.Second)
		}

		if sched := s.getScheduler(); sched != nil {
			sched.Stop()
		}

		// Close SSE connections so httpServer.Shutdown() doesn't wait 30s for them.
		if s.eventBus != nil {
			s.eventBus.Close()
		}
		if s.logHub != nil {
			s.logHub.Close()
		}

		if s.authRateLimit != nil {
			s.authRateLimit.Stop()
		}

		if s.httpServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
				log.Warnf("[server] http server shutdown error: %v", err)
			} else {
				log.Infoln("[server] http server stopped gracefully")
			}
		}

		if s.logWriter != nil {
			if err := s.logWriter.Close(); err != nil {
				log.Warnf("[server] failed to close log writer: %v", err)
			} else {
				log.Infoln("[server] log writer closed")
			}
		}

		if s.db != nil {
			if err := s.db.Close(); err != nil {
				log.Warnf("[server] failed to close database: %v", err)
			} else {
				log.Infoln("[server] database connection closed")
			}
		}

		time.Sleep(100 * time.Millisecond)
		log.Infoln("[server] shutdown complete")
		close(s.shutdownDone)
	})
}

func (s *Server) handleScheduleStatusChange(statuses []scheduler.ScheduleStatus) {
	if s.eventBus == nil {
		return
	}
	sched := s.getScheduler()
	schedulerRunning := sched != nil && sched.IsRunning()
	s.eventBus.Publish("schedules", map[string]interface{}{
		"scheduler_running": schedulerRunning,
		"entries":           statuses,
	})

	for _, st := range statuses {
		if st.ConsecutiveFailures == 1 || (st.ConsecutiveFailures >= 3 && st.ConsecutiveFailures%3 == 0) {
			s.eventBus.PublishNotification(
				"schedule_warning",
				fmt.Sprintf("调度 %q 连续失败 %d 次", st.Entry.Name, st.ConsecutiveFailures),
				map[string]interface{}{
					"schedule_id":          st.Entry.ID,
					"consecutive_failures": st.ConsecutiveFailures,
				},
			)
		}
	}
}
