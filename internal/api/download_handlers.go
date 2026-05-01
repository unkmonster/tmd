package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/unkmonster/tmd/internal/service"
	"github.com/unkmonster/tmd/internal/utils"
)

// executeDownloadTask 执行下载任务的通用辅助方法
func (s *Server) executeDownloadTask(task *Task, downloadFunc func() error) {
	taskID := task.ID
	go func() {
		if !s.acquireDownloadTaskSlot(task.Ctx) {
			return
		}
		defer s.releaseDownloadTaskSlot()

		if !s.taskManager.UpdateTaskStatus(taskID, TaskStatusRunning) {
			return
		}
		if err := downloadFunc(); err != nil {
			taskSnapshot, ok := s.taskManager.GetTask(taskID)
			if ok && !isTerminalStatus(taskSnapshot.Status) {
				s.taskManager.SetTaskError(taskID, err)
			}
		}
	}()
}

func (s *Server) acquireDownloadTaskSlot(ctx context.Context) bool {
	if s.downloadTaskSlots == nil {
		return true
	}
	select {
	case s.downloadTaskSlots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Server) releaseDownloadTaskSlot() {
	if s.downloadTaskSlots == nil {
		return
	}
	select {
	case <-s.downloadTaskSlots:
	default:
	}
}

func (s *Server) enqueueTask(task *Task, run func(ctx context.Context, taskID string, reporter service.ProgressReporter) error) {
	reporter := NewSSEProgressReporter(s)
	taskCtx := task.Ctx
	taskID := task.ID
	s.executeDownloadTask(task, func() error {
		return run(taskCtx, taskID, reporter)
	})
}

func formatTaskMarkTime(timestamp *time.Time) *string {
	if timestamp == nil {
		return nil
	}
	formatted := timestamp.Format("2006-01-02T15:04:05")
	return &formatted
}

func (s *Server) screenNameFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	screenName := utils.NormalizeScreenName(r.PathValue("screen_name"))

	if !utils.IsValidScreenName(screenName) {
		s.writeError(w, http.StatusBadRequest, "Invalid screen name format")
		return "", false
	}

	return screenName, true
}

func (s *Server) handleUserDownloadRoute(w http.ResponseWriter, r *http.Request) {
	screenName, ok := s.screenNameFromPath(w, r)
	if !ok {
		return
	}
	s.handleUserDownload(w, r, screenName)
}

func (s *Server) handleUserProfileRoute(w http.ResponseWriter, r *http.Request) {
	screenName, ok := s.screenNameFromPath(w, r)
	if !ok {
		return
	}
	s.handleUserProfile(w, r, screenName)
}

func (s *Server) handleUserMarkRoute(w http.ResponseWriter, r *http.Request) {
	screenName, ok := s.screenNameFromPath(w, r)
	if !ok {
		return
	}
	s.handleUserMark(w, r, screenName)
}

func (s *Server) handleFollowingDownloadRoute(w http.ResponseWriter, r *http.Request) {
	screenName, ok := s.screenNameFromPath(w, r)
	if !ok {
		return
	}
	s.handleFollowingDownload(w, r, screenName)
}

func (s *Server) handleFollowingMarkRoute(w http.ResponseWriter, r *http.Request) {
	screenName, ok := s.screenNameFromPath(w, r)
	if !ok {
		return
	}
	s.handleFollowingMark(w, r, screenName)
}

func (s *Server) handleUserDownload(w http.ResponseWriter, r *http.Request, screenName string) {
	var req UserDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = UserDownloadTaskData{}
	}
	req.ScreenName = screenName

	task := s.taskManager.CreateTask(TaskTypeUserDownload, &req)

	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.UserDownload(ctx, taskID, screenName, opts, reporter)
	})

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

func (s *Server) handleUserProfile(w http.ResponseWriter, _ *http.Request, screenName string) {
	req := ProfileDownloadTaskData{ScreenName: screenName}

	task := s.taskManager.CreateTask(TaskTypeProfileDownload, &req)

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.ProfileDownload(ctx, taskID, []string{screenName}, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"screen_name": req.ScreenName,
		"message":     "Profile download task queued",
	}))
}

func (s *Server) handleUserMark(w http.ResponseWriter, r *http.Request, screenName string) {
	var req MarkDownloadedTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = MarkDownloadedTaskData{}
	}
	req.ScreenName = screenName

	task := s.taskManager.CreateTask(TaskTypeMarkDownloaded, &req)

	markTime := formatTaskMarkTime(req.Timestamp)

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.MarkDownloaded(ctx, taskID, []string{screenName}, nil, nil, markTime, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"screen_name": req.ScreenName,
		"timestamp":   req.Timestamp,
		"message":     "Mark downloaded task queued",
	}))
}

func (s *Server) handleListMark(w http.ResponseWriter, r *http.Request, listID uint64) {
	var req ListMarkDownloadedTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = ListMarkDownloadedTaskData{}
	}
	req.ListID = listID

	task := s.taskManager.CreateTask(TaskTypeMarkDownloaded, &req)

	markTime := formatTaskMarkTime(req.Timestamp)

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.MarkDownloaded(ctx, taskID, nil, []uint64{listID}, nil, markTime, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":   task.ID,
		"status":    task.Status,
		"list_id":   listID,
		"timestamp": req.Timestamp,
		"message":   "Mark list downloaded task queued",
	}))
}

func (s *Server) handleFollowingMark(w http.ResponseWriter, r *http.Request, screenName string) {
	var req MarkDownloadedTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = MarkDownloadedTaskData{}
	}
	req.ScreenName = screenName

	task := s.taskManager.CreateTask(TaskTypeMarkDownloaded, &req)

	markTime := formatTaskMarkTime(req.Timestamp)

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.MarkDownloaded(ctx, taskID, nil, nil, []string{screenName}, markTime, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"screen_name": screenName,
		"timestamp":   req.Timestamp,
		"message":     "Mark following downloaded task queued",
	}))
}

func (s *Server) handleFollowingDownload(w http.ResponseWriter, r *http.Request, screenName string) {
	var req FollowingDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = FollowingDownloadTaskData{}
	}
	req.ScreenName = screenName

	task := s.taskManager.CreateTask(TaskTypeFollowingDownload, &req)

	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.FollowingDownload(ctx, taskID, screenName, opts, reporter)
	})

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

func (s *Server) listIDFromPath(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	listID, err := strconv.ParseUint(r.PathValue("list_id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid list ID")
		return 0, false
	}

	// 校验 listID 有效性（必须大于 0）
	if listID == 0 {
		s.writeError(w, http.StatusBadRequest, "Invalid list ID")
		return 0, false
	}

	return listID, true
}

func (s *Server) handleListDownloadRoute(w http.ResponseWriter, r *http.Request) {
	listID, ok := s.listIDFromPath(w, r)
	if !ok {
		return
	}
	s.handleListDownload(w, r, listID)
}

func (s *Server) handleListProfileRoute(w http.ResponseWriter, r *http.Request) {
	listID, ok := s.listIDFromPath(w, r)
	if !ok {
		return
	}
	s.handleListProfile(w, r, listID)
}

func (s *Server) handleListMarkRoute(w http.ResponseWriter, r *http.Request) {
	listID, ok := s.listIDFromPath(w, r)
	if !ok {
		return
	}
	s.handleListMark(w, r, listID)
}

func (s *Server) handleListDownload(w http.ResponseWriter, r *http.Request, listID uint64) {
	var req ListDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = ListDownloadTaskData{}
	}
	req.ListID = listID

	task := s.taskManager.CreateTask(TaskTypeListDownload, &req)

	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.ListDownload(ctx, taskID, listID, opts, reporter)
	})

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

func (s *Server) handleListProfile(w http.ResponseWriter, _ *http.Request, listID uint64) {
	req := ListProfileTaskData{ListID: listID}

	task := s.taskManager.CreateTask(TaskTypeListProfile, &req)

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.ListProfileDownload(ctx, taskID, listID, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id": task.ID,
		"status":  task.Status,
		"list_id": listID,
		"message": "List profile download task queued",
	}))
}

func (s *Server) handleJsonFileDownload(w http.ResponseWriter, r *http.Request) {
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
	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.JsonFileDownload(ctx, taskID, req.Paths, req.NoRetry, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":  task.ID,
		"status":   task.Status,
		"paths":    req.Paths,
		"no_retry": req.NoRetry,
		"message":  "JSON file download task queued",
	}))
}

func (s *Server) handleJsonFolderDownload(w http.ResponseWriter, r *http.Request) {
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
	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.JsonFolderDownload(ctx, taskID, req.Paths, req.NoRetry, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":  task.ID,
		"status":   task.Status,
		"paths":    req.Paths,
		"no_retry": req.NoRetry,
		"message":  "JSON folder download task queued",
	}))
}

func (s *Server) handleBatchDownload(w http.ResponseWriter, r *http.Request) {
	var req BatchDownloadTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Users) == 0 && len(req.Lists) == 0 && len(req.FollowingNames) == 0 {
		s.writeError(w, http.StatusBadRequest, "At least one of users, lists, or following_names is required")
		return
	}

	// 校验所有 screenName 格式
	for i, screenName := range req.Users {
		req.Users[i] = utils.NormalizeScreenName(screenName)
		if !utils.IsValidScreenName(req.Users[i]) {
			s.writeError(w, http.StatusBadRequest, "Invalid screen name format: "+screenName)
			return
		}
	}
	for i, screenName := range req.FollowingNames {
		req.FollowingNames[i] = utils.NormalizeScreenName(screenName)
		if !utils.IsValidScreenName(req.FollowingNames[i]) {
			s.writeError(w, http.StatusBadRequest, "Invalid screen name format: "+screenName)
			return
		}
	}

	// 校验所有 listID 有效性
	for _, listID := range req.Lists {
		if listID == 0 {
			s.writeError(w, http.StatusBadRequest, "Invalid list ID: must be greater than 0")
			return
		}
	}

	task := s.taskManager.CreateTask(TaskTypeBatchDownload, &req)

	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.BatchDownload(ctx, taskID, req.Users, req.Lists, req.FollowingNames, opts, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":         task.ID,
		"status":          task.Status,
		"users":           req.Users,
		"lists":           req.Lists,
		"following_names": req.FollowingNames,
		"user_count":      len(req.Users),
		"list_count":      len(req.Lists),
		"following_count": len(req.FollowingNames),
		"auto_follow":     req.AutoFollow,
		"skip_profile":    req.SkipProfile,
		"no_retry":        req.NoRetry,
		"message":         "Batch download task queued",
	}))
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.taskManager.GetAllTasks()
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(TaskListResponse{
		Tasks: tasks,
		Total: len(tasks),
	}))
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, ok := s.taskManager.GetTask(taskID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "Task not found")
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(task))
}

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
