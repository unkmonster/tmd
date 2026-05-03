package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/consolelog"
	"github.com/unkmonster/tmd/internal/service"
)

const maxConcurrentDownloadTasks = 1

type Server struct {
	client            *resty.Client
	additionalClients []*resty.Client
	db                *sqlx.DB
	config            *config.Config
	appRootPath       string
	configMu          sync.RWMutex
	taskManager       *TaskManager
	downloadService   service.DownloadService
	downloadTaskSlots chan struct{}
	logWriter         io.Closer
	logHub            *consolelog.Hub
	httpServer        *http.Server
	shutdownOnce      sync.Once
	shutdownDone      chan struct{}
}

func NewServer(client *resty.Client, additionalClients []*resty.Client, db *sqlx.DB, config *config.Config, appRootPath string, logWriter io.Closer) *Server {
	return NewServerWithConsoleLogHub(client, additionalClients, db, config, appRootPath, logWriter, consolelog.DefaultHub())
}

func NewServerWithConsoleLogHub(client *resty.Client, additionalClients []*resty.Client, db *sqlx.DB, config *config.Config, appRootPath string, logWriter io.Closer, logHub *consolelog.Hub) *Server {
	if logHub == nil {
		logHub = consolelog.DefaultHub()
	}

	s := &Server{
		client:            client,
		additionalClients: additionalClients,
		db:                db,
		config:            config,
		appRootPath:       appRootPath,
		logWriter:         logWriter,
		logHub:            logHub,
		taskManager:       NewTaskManager(),
		downloadTaskSlots: make(chan struct{}, maxConcurrentDownloadTasks),
		shutdownDone:      make(chan struct{}),
	}

	downloadService, err := service.NewDownloadService(&service.Dependencies{
		Client:            client,
		AdditionalClients: additionalClients,
		DB:                db,
		Config:            config,
	})
	if err != nil {
		log.Fatalf("failed to create download service: %v", err)
	}
	s.downloadService = downloadService

	return s
}

func (s *Server) consoleLogHub() *consolelog.Hub {
	if s.logHub != nil {
		return s.logHub
	}
	return consolelog.DefaultHub()
}

func (s *Server) buildHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
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
	mux.HandleFunc("GET /api/v1/tasks", s.handleTasks)
	mux.HandleFunc("GET /api/v1/tasks/{task_id}", s.handleGetTask)
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/cancel", s.handleCancelTask)

	mux.HandleFunc("GET /{$}", s.handleWeb)
	mux.HandleFunc("GET /tasks", s.handleWeb)
	mux.HandleFunc("GET /data", s.handleWeb)
	mux.HandleFunc("GET /system", s.handleWeb)
	mux.HandleFunc("GET /static/{$}", s.handleStatic)
	mux.HandleFunc("GET /static/{path...}", s.handleStatic)
	mux.HandleFunc("GET /api/v1/sse/tasks", s.handleSSETasks)

	mux.HandleFunc("GET /api/v1/db/users", s.handleDBUsers)
	mux.HandleFunc("GET /api/v1/db/users/{id}", s.handleDBUserDetail)
	mux.HandleFunc("PUT /api/v1/db/users/{id}", s.handleDBUserUpdate)
	mux.HandleFunc("DELETE /api/v1/db/users/{id}", s.handleDBUserDelete)
	mux.HandleFunc("GET /api/v1/db/users/{id}/previous-names", s.handleDBUserPreviousNames)

	mux.HandleFunc("GET /api/v1/db/lists", s.handleDBLists)
	mux.HandleFunc("GET /api/v1/db/lists/{id}", s.handleDBListDetail)
	mux.HandleFunc("PUT /api/v1/db/lists/{id}", s.handleDBListUpdate)
	mux.HandleFunc("DELETE /api/v1/db/lists/{id}", s.handleDBListDelete)

	mux.HandleFunc("GET /api/v1/db/user-entities", s.handleDBUserEntities)
	mux.HandleFunc("GET /api/v1/db/user-entities/{id}", s.handleDBUserEntityDetail)
	mux.HandleFunc("PUT /api/v1/db/user-entities/{id}", s.handleDBUserEntityUpdate)
	mux.HandleFunc("DELETE /api/v1/db/user-entities/{id}", s.handleDBUserEntityDelete)

	mux.HandleFunc("GET /api/v1/db/list-entities", s.handleDBListEntities)
	mux.HandleFunc("GET /api/v1/db/list-entities/{id}", s.handleDBListEntityDetail)
	mux.HandleFunc("PUT /api/v1/db/list-entities/{id}", s.handleDBListEntityUpdate)
	mux.HandleFunc("DELETE /api/v1/db/list-entities/{id}", s.handleDBListEntityDelete)

	mux.HandleFunc("GET /api/v1/db/user-links", s.handleDBUserLinks)
	mux.HandleFunc("GET /api/v1/db/user-links/{id}", s.handleDBUserLinkDetail)
	mux.HandleFunc("PUT /api/v1/db/user-links/{id}", s.handleDBUserLinkUpdate)
	mux.HandleFunc("DELETE /api/v1/db/user-links/{id}", s.handleDBUserLinkDelete)

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
	mux.HandleFunc("GET /api/v1/logs", s.handleGetLogs)
	mux.HandleFunc("GET /api/v1/logs/stream", s.handleLogStream)

	var handler http.Handler = mux

	// 注意中间件的包裹顺序：最外层是 Logging，里面一层是 CORS，最里面是 Mux。
	// 这样 Logging 就能记录到所有请求，包括那些被 CORS 拦截的 OPTIONS 预检请求。
	handler = cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}).Handler(handler)

	handler = loggingMiddleware(handler)
	return handler
}

func (s *Server) Start(port int) error {
	handler := s.buildHandler()

	addr := fmt.Sprintf(":%d", port)
	log.Infoln("API server starting on", addr)
	log.Infof("Visit %s to get started", color.FgLightBlue.Render(fmt.Sprintf("http://localhost%s/", addr)))

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(); err != nil {
		s.writeJSON(w, http.StatusServiceUnavailable, NewErrorResponse("Database unavailable"))
		return
	}

	resp := HealthResponse{
		Status:    "ok",
		Version:   "2.0.0",
		Timestamp: time.Now().UTC(),
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
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

func (s *Server) handleServerAction(w http.ResponseWriter) {
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

func (s *Server) handleServerShutdown(w http.ResponseWriter, _ *http.Request) {
	s.handleServerAction(w)
}

func (s *Server) WaitForShutdown() {
	<-s.shutdownDone
}

func (s *Server) GracefulShutdown(reason string) {
	s.shutdownOnce.Do(func() {
		log.Infof("[server] graceful shutdown started (reason: %s)", reason)

		if s.taskManager != nil {
			s.taskManager.CancelAllTasks()
			s.taskManager.Close()
			log.Infoln("[server] all running tasks cancelled")
			time.Sleep(1 * time.Second)
		}

		if s.httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := s.httpServer.Shutdown(ctx); err != nil {
				log.Warnf("[server] http server shutdown error: %v", err)
			} else {
				log.Infoln("[server] http server stopped gracefully")
			}
		}

		if s.db != nil {
			if err := s.db.Close(); err != nil {
				log.Warnf("[server] failed to close database: %v", err)
			} else {
				log.Infoln("[server] database connection closed")
			}
		}

		if s.logWriter != nil {
			if err := s.logWriter.Close(); err != nil {
				log.Warnf("[server] failed to close log writer: %v", err)
			} else {
				log.Infoln("[server] log writer closed")
			}
		}

		time.Sleep(100 * time.Millisecond)
		log.Infoln("[server] shutdown complete")
		close(s.shutdownDone)
	})
}
