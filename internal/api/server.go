package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gookit/color"
	"github.com/jmoiron/sqlx"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/service"
	"github.com/unkmonster/tmd/internal/twitter"
)

// Server API Server
type Server struct {
	client            *resty.Client
	additionalClients []*resty.Client
	db                *sqlx.DB
	config            *config.Config
	appRootPath       string
	configMu          sync.RWMutex
	taskManager       *TaskManager
	downloadService   service.DownloadService
	sseMgr            *sseManager
	logWriter         io.Closer
	httpServer        *http.Server
}

// NewServer 创建 API Server
func NewServer(client *resty.Client, additionalClients []*resty.Client, db *sqlx.DB, config *config.Config, appRootPath string, logWriter io.Closer) *Server {
	s := &Server{
		client:            client,
		additionalClients: additionalClients,
		db:                db,
		config:            config,
		appRootPath:       appRootPath,
		logWriter:         logWriter,
		taskManager:       NewTaskManager(),
		sseMgr:            newSSEManager(),
	}

	// 创建 Service 层
	s.downloadService = service.NewDownloadService(&service.Dependencies{
		Client:            client,
		AdditionalClients: additionalClients,
		DB:                db,
		Config:            config,
		AppRootPath:       appRootPath,
	})

	return s
}

// Start 启动服务器
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// 原有 API 端点
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/users/", s.handleUsers)
	mux.HandleFunc("/api/v1/lists/", s.handleLists)
	mux.HandleFunc("/api/v1/json/file/download", s.handleJsonFileDownload)
	mux.HandleFunc("/api/v1/json/folder/download", s.handleJsonFolderDownload)
	mux.HandleFunc("/api/v1/batch/download", s.handleBatchDownload)
	mux.HandleFunc("/api/v1/tasks", s.handleTasks)
	mux.HandleFunc("GET /api/v1/tasks/{task_id}", s.handleGetTask)
	mux.HandleFunc("POST /api/v1/tasks/{task_id}/cancel", s.handleCancelTask)

	// 新增 Web 与数据端点
	mux.HandleFunc("GET /{$}", s.handleWeb)
	mux.HandleFunc("GET /tasks", s.handleWeb)
	mux.HandleFunc("GET /data", s.handleWeb)
	mux.HandleFunc("GET /system", s.handleWeb)
	mux.HandleFunc("/static/", s.handleStatic)
	mux.HandleFunc("/api/v1/sse/tasks", s.handleSSETasks)

	// 数据库查询路由 - Users
	mux.HandleFunc("GET /api/v1/db/users", s.handleDBUsers)
	mux.HandleFunc("GET /api/v1/db/users/{id}", s.handleDBUserDetail)
	mux.HandleFunc("PUT /api/v1/db/users/{id}", s.handleDBUserUpdate)
	mux.HandleFunc("DELETE /api/v1/db/users/{id}", s.handleDBUserDelete)
	mux.HandleFunc("GET /api/v1/db/users/{id}/previous-names", s.handleDBUserPreviousNames)

	// 数据库查询路由 - Lists
	mux.HandleFunc("GET /api/v1/db/lists", s.handleDBLists)
	mux.HandleFunc("GET /api/v1/db/lists/{id}", s.handleDBListDetail)
	mux.HandleFunc("PUT /api/v1/db/lists/{id}", s.handleDBListUpdate)
	mux.HandleFunc("DELETE /api/v1/db/lists/{id}", s.handleDBListDelete)

	// 数据库查询路由 - User Entities
	mux.HandleFunc("GET /api/v1/db/user-entities", s.handleDBUserEntities)
	mux.HandleFunc("GET /api/v1/db/user-entities/{id}", s.handleDBUserEntityDetail)
	mux.HandleFunc("PUT /api/v1/db/user-entities/{id}", s.handleDBUserEntityUpdate)
	mux.HandleFunc("DELETE /api/v1/db/user-entities/{id}", s.handleDBUserEntityDelete)

	// 数据库查询路由 - List Entities（新增）
	mux.HandleFunc("GET /api/v1/db/list-entities", s.handleDBListEntities)
	mux.HandleFunc("GET /api/v1/db/list-entities/{id}", s.handleDBListEntityDetail)
	mux.HandleFunc("PUT /api/v1/db/list-entities/{id}", s.handleDBListEntityUpdate)
	mux.HandleFunc("DELETE /api/v1/db/list-entities/{id}", s.handleDBListEntityDelete)

	// 数据库查询路由 - User Links（新增）
	mux.HandleFunc("GET /api/v1/db/user-links", s.handleDBUserLinks)

	mux.HandleFunc("/api/v1/config", s.handleConfig)
	mux.HandleFunc("/api/v1/config/raw", s.handleConfigRaw)
	mux.HandleFunc("/api/v1/config/fields", s.handleConfigFields)
	mux.HandleFunc("/api/v1/cookies", s.handleCookies)
	mux.HandleFunc("/api/v1/cookies/raw", s.handleCookiesRaw)
	mux.HandleFunc("/api/v1/server/restart", s.handleServerRestart)
	mux.HandleFunc("/api/v1/server/shutdown", s.handleServerShutdown)
	mux.HandleFunc("/api/v1/logs", s.handleGetLogs)
	mux.HandleFunc("/api/v1/logs/stream", s.handleLogStream)

	// 中间件链
	var handler http.Handler = mux
	handler = loggingMiddleware(handler)
	handler = cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}).Handler(handler)

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

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 检查数据库连接
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

// handleConfig 获取配置（脱敏）
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.configMu.RLock()
	defer s.configMu.RUnlock()

	// 返回脱敏后的配置
	resp := ConfigResponse{
		RootPath:           s.config.RootPath,
		MaxDownloadRoutine: s.config.MaxDownloadRoutine,
		MaxFileNameLen:     s.config.MaxFileNameLen,
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
}

// handleUsers 处理用户相关请求
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		s.writeError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	screenName := parts[0]
	action := parts[1]

	switch action {
	case "download":
		s.handleUserDownload(w, r, screenName)
	case "profile":
		s.handleUserProfile(w, r, screenName)
	case "mark":
		s.handleUserMark(w, r, screenName)
	case "following":
		if len(parts) >= 3 && parts[2] == "download" {
			s.handleFollowingDownload(w, r, screenName)
		} else {
			s.writeError(w, http.StatusNotFound, "Not found")
		}
	default:
		s.writeError(w, http.StatusNotFound, "Not found")
	}
}

// handleUserDownload 处理用户下载
func (s *Server) handleUserDownload(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req UserDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 使用默认值
		req = UserDownloadTaskData{}
	}
	req.ScreenName = screenName

	// 创建任务
	task := s.taskManager.CreateTask(TaskTypeUserDownload, &req)

	// 构建下载选项
	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	// 创建进度报告器
	reporter := NewSSEProgressReporter(s, task.ID)

	// 异步执行
	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.UserDownload(task.Ctx, task.ID, screenName, opts, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":      task.ID,
		"status":       task.Status,
		"screen_name":  req.ScreenName,
		"auto_follow":  req.AutoFollow,
		"skip_profile": req.SkipProfile,
		"no_retry":     req.NoRetry,
		"message":      "Download task queued successfully",
	}))
}

// handleUserProfile 处理用户 Profile 下载
func (s *Server) handleUserProfile(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	req := ProfileDownloadTaskData{ScreenName: screenName}

	// 创建任务
	task := s.taskManager.CreateTask(TaskTypeProfileDownload, &req)

	// 创建进度报告器
	reporter := NewSSEProgressReporter(s, task.ID)

	// 异步执行
	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.ProfileDownload(task.Ctx, task.ID, []string{screenName}, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"screen_name": req.ScreenName,
		"message":     "Profile download task queued",
	}))
}

// handleUserMark 处理标记已下载
func (s *Server) handleUserMark(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req MarkDownloadedTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = MarkDownloadedTaskData{}
	}
	req.ScreenName = screenName

	// 创建任务
	task := s.taskManager.CreateTask(TaskTypeMarkDownloaded, &req)

	// 构建 markTime
	var markTime *string
	if req.Timestamp != nil {
		t := req.Timestamp.Format("2006-01-02T15:04:05")
		markTime = &t
	}

	// 创建进度报告器
	reporter := NewSSEProgressReporter(s, task.ID)

	// 异步执行
	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)

		// 获取用户对象
		user, _, err := twitter.GetUserByScreenName(task.Ctx, s.client, screenName)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, fmt.Errorf("failed to get user %s: %w", screenName, err))
			return
		}

		err = s.downloadService.MarkDownloaded(task.Ctx, task.ID, []*twitter.User{user}, nil, markTime, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"screen_name": req.ScreenName,
		"timestamp":   req.Timestamp,
		"message":     "Mark downloaded task queued",
	}))
}

// handleFollowingDownload 处理关注列表下载
func (s *Server) handleFollowingDownload(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req FollowingDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = FollowingDownloadTaskData{}
	}
	req.ScreenName = screenName

	// 创建任务
	task := s.taskManager.CreateTask(TaskTypeFollowingDownload, &req)

	// 构建下载选项
	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	// 创建进度报告器
	reporter := NewSSEProgressReporter(s, task.ID)

	// 异步执行
	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.FollowingDownload(task.Ctx, task.ID, screenName, opts, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":      task.ID,
		"status":       task.Status,
		"screen_name":  req.ScreenName,
		"auto_follow":  req.AutoFollow,
		"skip_profile": req.SkipProfile,
		"no_retry":     req.NoRetry,
		"message":      "Following download task queued successfully",
	}))
}

// handleLists 处理列表相关请求
func (s *Server) handleLists(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/lists/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		s.writeError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	listID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	action := parts[1]

	switch action {
	case "download":
		s.handleListDownload(w, r, listID)
	case "profile":
		s.handleListProfile(w, r, listID)
	default:
		s.writeError(w, http.StatusNotFound, "Not found")
	}
}

// handleListDownload 处理列表下载
func (s *Server) handleListDownload(w http.ResponseWriter, r *http.Request, listID uint64) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req ListDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = ListDownloadTaskData{}
	}
	req.ListID = listID

	// 创建任务
	task := s.taskManager.CreateTask(TaskTypeListDownload, &req)

	// 构建下载选项
	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	// 创建进度报告器
	reporter := NewSSEProgressReporter(s, task.ID)

	// 异步执行
	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.ListDownload(task.Ctx, task.ID, listID, opts, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":      task.ID,
		"status":       task.Status,
		"list_id":      listID,
		"skip_profile": req.SkipProfile,
		"auto_follow":  req.AutoFollow,
		"no_retry":     req.NoRetry,
		"message":      "List download task queued",
	}))
}

// handleListProfile 处理列表 Profile 下载
func (s *Server) handleListProfile(w http.ResponseWriter, r *http.Request, listID uint64) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	req := ListProfileTaskData{ListID: listID}

	task := s.taskManager.CreateTask(TaskTypeListProfile, &req)

	// 创建进度报告器
	reporter := NewSSEProgressReporter(s, task.ID)

	// 异步执行
	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.ListProfileDownload(task.Ctx, task.ID, listID, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id": task.ID,
		"status":  task.Status,
		"list_id": listID,
		"message": "List profile download task queued",
	}))
}

// handleJsonFileDownload 处理第三方工具JSON文件下载（用户资料）
func (s *Server) handleJsonFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req JsonFileDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Paths) == 0 {
		s.writeError(w, http.StatusBadRequest, "Paths are required")
		return
	}

	task := s.taskManager.CreateTask(TaskTypeJsonFileDownload, &req)
	reporter := NewSSEProgressReporter(s, task.ID)

	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.JsonFileDownload(task.Ctx, task.ID, req.Paths, req.NoRetry, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":  task.ID,
		"status":   task.Status,
		"paths":    req.Paths,
		"no_retry": req.NoRetry,
		"message":  "JSON file download task queued",
	}))
}

// handleJsonFolderDownload 处理loongtweet文件夹下载（推文媒体）
func (s *Server) handleJsonFolderDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req JsonFolderDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Paths) == 0 {
		s.writeError(w, http.StatusBadRequest, "Paths are required")
		return
	}

	task := s.taskManager.CreateTask(TaskTypeJsonFolderDownload, &req)
	reporter := NewSSEProgressReporter(s, task.ID)

	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.JsonFolderDownload(task.Ctx, task.ID, req.Paths, req.NoRetry, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":  task.ID,
		"status":   task.Status,
		"paths":    req.Paths,
		"no_retry": req.NoRetry,
		"message":  "JSON folder download task queued",
	}))
}

// handleBatchDownload 处理批量下载
func (s *Server) handleBatchDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req BatchDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Users) == 0 && len(req.Lists) == 0 {
		s.writeError(w, http.StatusBadRequest, "At least one of users or lists is required")
		return
	}

	// 创建任务
	task := s.taskManager.CreateTask(TaskTypeBatchDownload, &req)

	// 构建下载选项
	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	// 创建进度报告器
	reporter := NewSSEProgressReporter(s, task.ID)

	// 异步执行
	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)

		// 获取用户对象
		var users []*twitter.User
		for _, screenName := range req.Users {
			user, _, err := twitter.GetUserByScreenName(task.Ctx, s.client, screenName)
			if err != nil {
				log.Warnf("Failed to get user %s: %v", screenName, err)
				continue
			}
			users = append(users, user)
		}

		// 获取列表对象
		var lists []twitter.ListBase
		for _, listID := range req.Lists {
			list, err := twitter.GetLst(task.Ctx, s.client, listID)
			if err != nil {
				log.Warnf("Failed to get list %d: %v", listID, err)
				continue
			}
			lists = append(lists, list)
		}

		// 检查是否全部解析失败
		if len(users) == 0 && len(lists) == 0 {
			s.taskManager.SetTaskError(task.ID, fmt.Errorf("all users and lists failed to resolve"))
			return
		}

		err := s.downloadService.BatchDownload(task.Ctx, task.ID, users, lists, opts, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":      task.ID,
		"status":       task.Status,
		"users":        req.Users,
		"lists":        req.Lists,
		"user_count":   len(req.Users),
		"list_count":   len(req.Lists),
		"auto_follow":  req.AutoFollow,
		"skip_profile": req.SkipProfile,
		"no_retry":     req.NoRetry,
		"message":      "Batch download task queued",
	}))
}

// handleTasks 处理任务列表
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	tasks := s.taskManager.GetAllTasks()
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(TaskListResponse{
		Tasks: tasks,
		Total: len(tasks),
	}))
}

// handleGetTask 获取任务详情
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, ok := s.taskManager.GetTask(taskID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "Task not found")
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(task))
}

// handleCancelTask 取消任务
func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	if !s.taskManager.CancelTask(taskID) {
		s.writeError(w, http.StatusBadRequest, "Task cannot be cancelled")
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Task cancelled",
	}))
}

// writeJSON 写入 JSON 响应
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Warnf("Failed to write response: %v", err)
	}
}

// writeError 写入错误响应
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, NewErrorResponse(message))
}

func (s *Server) handleConfigRaw(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfigRaw(w, r)
	case http.MethodPut:
		s.handleUpdateConfigRaw(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleGetConfigRaw(w http.ResponseWriter, _ *http.Request) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	confPath := filepath.Join(s.appRootPath, "conf.yaml")
	data, err := os.ReadFile(confPath)
	if err != nil {
		if os.IsNotExist(err) {
			defaultConf := config.Config{}
			yamlData, _ := yaml.Marshal(defaultConf)
			s.writeJSON(w, http.StatusOK, NewSuccessResponse(ConfigRawResponse{
				Content: string(yamlData),
				Path:    confPath,
				Exists:  false,
			}))
			return
		}
		s.writeError(w, http.StatusInternalServerError, "Failed to read config: "+err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(ConfigRawResponse{
		Content: string(data),
		Path:    confPath,
		Exists:  true,
	}))
}

func (s *Server) handleUpdateConfigRaw(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		s.writeError(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	var testConf config.Config
	if err := yaml.Unmarshal([]byte(req.Content), &testConf); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid YAML format: "+err.Error())
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	confPath := filepath.Join(s.appRootPath, "conf.yaml")

	backupPath := confPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if data, err := os.ReadFile(confPath); err == nil {
		if writeErr := os.WriteFile(backupPath, data, 0644); writeErr != nil {
			log.Warnf("Failed to create config backup: %v", writeErr)
		}
	}

	if err := os.WriteFile(confPath, []byte(req.Content), 0600); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to write config: "+err.Error())
		return
	}

	log.Infoln("[WebUI] config saved via raw editor")

	s.config = &testConf

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message":      "Configuration saved successfully",
		"backup":       filepath.Base(backupPath),
		"applied":      true,
		"yaml_preview": req.Content,
	}))
}

var configFieldDefCache []config.FieldDef
var configFieldMetaCache []configFieldMeta
var configFieldMetaOnce sync.Once

type configFieldMeta struct {
	Name        string
	Label       string
	Prompt      string
	Default     string
	Type        string
	Placeholder string
	Required    bool
	Group       string
	IsSensitive bool
}

func initConfigFieldMeta() {
	fieldDefs := config.GetFieldDefs()
	configFieldDefCache = fieldDefs
	configFieldMetaCache = make([]configFieldMeta, len(fieldDefs))
	for i, fd := range fieldDefs {
		meta := configFieldMeta{
			Name:     fd.Name,
			Prompt:   fd.Prompt,
			Default:  fd.Default,
			Required: fd.Default == "",
		}
		switch fd.Name {
		case "root_path":
			meta.Label, meta.Type, meta.Group = "存储路径", "text", "basic"
		case "auth_token":
			meta.Label, meta.Type, meta.Group = "Auth Token", "password", "cookie"
			meta.IsSensitive = true
		case "ct0":
			meta.Label, meta.Type, meta.Group = "CT0", "password", "cookie"
			meta.IsSensitive = true
		case "max_download_routine":
			meta.Label, meta.Type, meta.Group = "最大并发下载", "number", "advanced"
			meta.Placeholder = fmt.Sprintf("1-100, 默认 %s", fd.Default)
		case "max_file_name_len":
			meta.Label, meta.Type, meta.Group = "最大文件名长度", "number", "advanced"
			meta.Placeholder = fmt.Sprintf("%d-%d, 默认 %s", config.MinFileNameLen, config.MaxFileNameLen, fd.Default)
		case "proxy_url":
			meta.Label, meta.Type, meta.Group, meta.Placeholder = "代理地址", "text", "advanced", "http://127.0.0.1:7897 或留空"
		default:
			meta.Label, meta.Type, meta.Group = fd.Name, "text", "basic"
		}
		configFieldMetaCache[i] = meta
	}
}

func buildConfigFieldItems(conf *config.Config) []ConfigFieldItem {
	configFieldMetaOnce.Do(initConfigFieldMeta)
	items := make([]ConfigFieldItem, 0, len(configFieldMetaCache))
	for i, m := range configFieldMetaCache {
		val := config.GetFieldValue(conf, configFieldDefCache[i])
		item := ConfigFieldItem{
			Name:        m.Name,
			Label:       m.Label,
			Prompt:      m.Prompt,
			Value:       val,
			Default:     m.Default,
			Type:        m.Type,
			Placeholder: m.Placeholder,
			Required:    m.Required,
			Group:       m.Group,
		}
		if m.IsSensitive {
			item.Value = maskSensitive(val)
		}
		items = append(items, item)
	}
	return items
}

func (s *Server) handleConfigFields(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfigFields(w, r)
	case http.MethodPut:
		s.handleSaveConfigFields(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleGetConfigFields(w http.ResponseWriter, _ *http.Request) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	confPath := filepath.Join(s.appRootPath, "conf.yaml")
	exists := true

	currentConf := s.config
	if currentConf == nil {
		var err error
		currentConf, err = config.ReadConf(confPath)
		if err != nil {
			if os.IsNotExist(err) {
				exists = false
				currentConf = &config.Config{}
			} else {
				s.writeError(w, http.StatusInternalServerError, "Failed to read config: "+err.Error())
				return
			}
		}
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(ConfigFieldsResponse{
		Exists: exists,
		Fields: buildConfigFieldItems(currentConf),
	}))
}

func (s *Server) handleSaveConfigFields(w http.ResponseWriter, r *http.Request) {
	var req ConfigFieldsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	newConf := &config.Config{}
	if s.config != nil {
		*newConf = *s.config
	}

	fieldDefs := config.GetFieldDefs()

	for _, fd := range fieldDefs {
		userVal, ok := req.Fields[fd.Name]
		if !ok || strings.TrimSpace(userVal) == "" || userVal == "__KEEP_OLD__" {
			userVal = config.GetFieldValue(newConf, fd)
			if userVal == "" {
				userVal = fd.Default
			}
		}

		if err := fd.Setter(newConf, userVal); err != nil {
			s.writeError(w, http.StatusBadRequest,
				fmt.Sprintf("字段 %s 无效: %s", fd.Name, err.Error()))
			return
		}
	}

	confPath := filepath.Join(s.appRootPath, "conf.yaml")

	backupPath := confPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if data, err := os.ReadFile(confPath); err == nil {
		if writeErr := os.WriteFile(backupPath, data, 0644); writeErr != nil {
			log.Warnf("Failed to create config backup: %v", writeErr)
		}
	}

	if err := config.WriteConf(confPath, newConf); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	log.Infoln("[WebUI] config saved via structured form")

	s.config = newConf

	yamlPreview, _ := yaml.Marshal(newConf)

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message":      "Configuration saved successfully",
		"backup":       filepath.Base(backupPath),
		"applied":      true,
		"yaml_preview": string(yamlPreview),
		"fields":       buildConfigFieldItems(newConf),
	}))
}

func maskSensitive(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= 6 {
		return "***"
	}
	return string(runes[:3]) + "•••" + string(runes[len(runes)-3:])
}

func (s *Server) handleCookiesRaw(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetCookiesRaw(w, r)
	case http.MethodPut:
		s.handleUpdateCookiesRaw(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleGetCookiesRaw(w http.ResponseWriter, _ *http.Request) {
	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")
	data, err := os.ReadFile(cookiesPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.writeJSON(w, http.StatusOK, NewSuccessResponse(CookiesRawResponse{
				Content: "",
				Path:    cookiesPath,
				Exists:  false,
			}))
			return
		}
		s.writeError(w, http.StatusInternalServerError, "Failed to read cookies: "+err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(CookiesRawResponse{
		Content: string(data),
		Path:    cookiesPath,
		Exists:  true,
	}))
}

func (s *Server) handleUpdateCookiesRaw(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		s.writeError(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	var testCookies []*config.Cookie
	if err := yaml.Unmarshal([]byte(req.Content), &testCookies); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid YAML format: "+err.Error())
		return
	}

	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")

	backupPath := cookiesPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if data, err := os.ReadFile(cookiesPath); err == nil {
		if writeErr := os.WriteFile(backupPath, data, 0644); writeErr != nil {
			log.Warnf("Failed to create cookies backup: %v", writeErr)
		}
	}

	if err := config.WriteAdditionalCookies(cookiesPath, testCookies); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to write cookies: "+err.Error())
		return
	}

	log.Infoln("[WebUI] additional cookies saved via raw editor")

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Additional cookies saved successfully",
		"backup":  filepath.Base(backupPath),
	}))
}

func (s *Server) handleCookies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetCookies(w, r)
	case http.MethodPut:
		s.handleSaveCookies(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleGetCookies(w http.ResponseWriter, _ *http.Request) {
	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")
	exists := true

	cookies, err := config.ReadAdditionalCookies(cookiesPath)
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
			cookies = nil
		} else {
			s.writeError(w, http.StatusInternalServerError, "Failed to read cookies: "+err.Error())
			return
		}
	}

	items := make([]CookieItem, 0, len(cookies))
	for i, c := range cookies {
		items = append(items, CookieItem{
			Index:     i,
			AuthToken: maskSensitive(c.AuthToken),
			Ct0:       maskSensitive(c.Ct0),
		})
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"exists": exists,
		"items":  items,
	}))
}

func (s *Server) handleSaveCookies(w http.ResponseWriter, r *http.Request) {
	var req CookiesSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")

	existingCookies, _ := config.ReadAdditionalCookies(cookiesPath)

	cookies := make([]*config.Cookie, 0, len(req.Cookies))
	for i, c := range req.Cookies {
		authToken := c["auth_token"]
		ct0 := c["ct0"]

		if strings.TrimSpace(authToken) == "" && strings.TrimSpace(ct0) == "" {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("账户 #%d 的 Auth Token 和 CT0 不能同时为空", i+1))
			return
		}

		if authToken == "__KEEP_OLD__" && existingCookies != nil && i < len(existingCookies) {
			authToken = existingCookies[i].AuthToken
		}
		if ct0 == "__KEEP_OLD__" && existingCookies != nil && i < len(existingCookies) {
			ct0 = existingCookies[i].Ct0
		}

		cookies = append(cookies, &config.Cookie{
			AuthToken: authToken,
			Ct0:       ct0,
		})
	}

	backupPath := cookiesPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if data, err := os.ReadFile(cookiesPath); err == nil {
		if writeErr := os.WriteFile(backupPath, data, 0644); writeErr != nil {
			log.Warnf("Failed to create cookies backup: %v", writeErr)
		}
	}

	if err := config.WriteAdditionalCookies(cookiesPath, cookies); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to save cookies: "+err.Error())
		return
	}

	log.Infoln("[WebUI] additional cookies saved via structured form")

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Additional cookies saved successfully",
		"backup":  filepath.Base(backupPath),
	}))
}

// handleServerRestart 触发服务器优雅重启
func (s *Server) handleServerRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Server restarting...",
		"action":  "restart",
	}))

	log.Infoln("[WebUI] received restart request, performing graceful shutdown...")

	go func() {
		time.Sleep(500 * time.Millisecond)
		s.gracefulShutdown("restart")
	}()
}

// handleServerShutdown 关闭服务器
func (s *Server) handleServerShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Server shutting down...",
		"action":  "shutdown",
	}))

	log.Infoln("[WebUI] received shutdown request, performing graceful shutdown...")

	go func() {
		time.Sleep(500 * time.Millisecond)
		s.gracefulShutdown("shutdown")
	}()
}

// gracefulShutdown 优雅关闭所有资源
func (s *Server) gracefulShutdown(reason string) {
	log.Infof("[WebUI] graceful shutdown started (reason: %s)", reason)

	// 1. 关闭 HTTP 服务器（停止接受新连接，等待现有请求完成）
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Warnf("[WebUI] http server shutdown error: %v", err)
		} else {
			log.Infoln("[WebUI] http server stopped gracefully")
		}
	}

	// 2. 关闭数据库连接
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Warnf("[WebUI] failed to close database: %v", err)
		} else {
			log.Infoln("[WebUI] database connection closed")
		}
	}

	// 3. 关闭日志写入器（确保所有日志刷新到磁盘）
	if s.logWriter != nil {
		if err := s.logWriter.Close(); err != nil {
			log.Warnf("[WebUI] failed to close log writer: %v", err)
		} else {
			log.Infoln("[WebUI] log writer closed")
		}
	}

	time.Sleep(100 * time.Millisecond)
	log.Infoln("[WebUI] shutdown complete")
	os.Exit(0)
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	query := r.URL.Query()
	levelStr := query.Get("level")
	search := query.Get("q")
	page, _ := strconv.Atoi(query.Get("page"))
	pageSize, _ := strconv.Atoi(query.Get("pageSize"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 100
	}

	logPath := filepath.Join(s.appRootPath, "tmd2.log")
	lines, err := readLogLinesTail(logPath, 5000)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to read logs: "+err.Error())
		return
	}

	filtered := filterLogLines(lines, levelStr, search)

	total := len(filtered)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		filtered = []string{}
	} else if end > total {
		filtered = filtered[start:]
	} else {
		filtered = filtered[start:end]
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(LogsResponse{
		Logs:       filtered,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}))
}

func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	levelStr := r.URL.Query().Get("level")
	ctx := r.Context()
	logPath := filepath.Join(s.appRootPath, "tmd2.log")

	var lastOffset int64 = 0
	if fi, err := os.Stat(logPath); err == nil {
		lastOffset = fi.Size()
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi, err := os.Stat(logPath)
			if err != nil {
				continue
			}

			currentSize := fi.Size()

			if currentSize < lastOffset {
				lastOffset = 0
				continue
			}

			if currentSize == lastOffset {
				fmt.Fprint(w, ": ping\n\n")
				flusher.Flush()
				continue
			}

			file, err := os.Open(logPath)
			if err != nil {
				continue
			}

			_, err = file.Seek(lastOffset, io.SeekStart)
			if err != nil {
				file.Close()
				continue
			}

			reader := bufio.NewReader(file)
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if levelStr != "" && !matchLogLevel(line, levelStr) {
					continue
				}
				line = stripAnsiCodes(line)
				fmt.Fprintf(w, "data: %s\n\n", jsonEscape(line))
				flusher.Flush()
			}

			newOffset, _ := file.Seek(0, io.SeekCurrent)
			file.Close()

			if newOffset > lastOffset {
				lastOffset = newOffset
			}
		}
	}
}

func readLogLinesTail(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	}
	return lines, scanner.Err()
}

func filterLogLines(lines []string, level, search string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if level != "" && !matchLogLevel(line, level) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(search)) {
			continue
		}
		result = append(result, line)
	}
	return result
}

func matchLogLevel(line, level string) bool {
	target := "level=" + level
	return strings.Contains(line, target+" ") ||
		strings.Contains(line, target+"\n") ||
		strings.Contains(line, target+"\t")
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsiCodes(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func jsonEscape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	return string(b[1 : len(b)-1])
}
