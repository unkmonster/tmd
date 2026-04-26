package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"

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
	taskManager       *TaskManager
	downloadService   service.DownloadService
	sseMgr            *sseManager
}

// NewServer 创建 API Server
func NewServer(client *resty.Client, additionalClients []*resty.Client, db *sqlx.DB, config *config.Config, appRootPath string) *Server {
	s := &Server{
		client:            client,
		additionalClients: additionalClients,
		db:                db,
		config:            config,
		appRootPath:       appRootPath,
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

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server.ListenAndServe()
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
