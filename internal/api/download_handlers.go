package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/unkmonster/tmd/internal/service"
)

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
		if len(parts) >= 3 {
			switch parts[2] {
			case "download":
				s.handleFollowingDownload(w, r, screenName)
			case "mark":
				s.handleFollowingMark(w, r, screenName)
			default:
				s.writeError(w, http.StatusNotFound, "Not found")
			}
		} else {
			s.writeError(w, http.StatusNotFound, "Not found")
		}
	default:
		s.writeError(w, http.StatusNotFound, "Not found")
	}
}

func (s *Server) handleUserDownload(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

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

	reporter := NewSSEProgressReporter(s, task.ID)

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

func (s *Server) handleUserProfile(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	req := ProfileDownloadTaskData{ScreenName: screenName}

	task := s.taskManager.CreateTask(TaskTypeProfileDownload, &req)

	reporter := NewSSEProgressReporter(s, task.ID)

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

	task := s.taskManager.CreateTask(TaskTypeMarkDownloaded, &req)

	var markTime *string
	if req.Timestamp != nil {
		t := req.Timestamp.Format("2006-01-02T15:04:05")
		markTime = &t
	}

	reporter := NewSSEProgressReporter(s, task.ID)

	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.MarkDownloaded(task.Ctx, task.ID, []string{screenName}, nil, nil, markTime, reporter)
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

func (s *Server) handleListMark(w http.ResponseWriter, r *http.Request, listID uint64) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req ListMarkDownloadedTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = ListMarkDownloadedTaskData{}
	}
	req.ListID = listID

	task := s.taskManager.CreateTask(TaskTypeMarkDownloaded, &req)

	var markTime *string
	if req.Timestamp != nil {
		t := req.Timestamp.Format("2006-01-02T15:04:05")
		markTime = &t
	}

	reporter := NewSSEProgressReporter(s, task.ID)

	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.MarkDownloaded(task.Ctx, task.ID, nil, []uint64{listID}, nil, markTime, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":   task.ID,
		"status":    task.Status,
		"list_id":   listID,
		"timestamp": req.Timestamp,
		"message":   "Mark list downloaded task queued",
	}))
}

func (s *Server) handleFollowingMark(w http.ResponseWriter, r *http.Request, screenName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req MarkDownloadedTaskData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = MarkDownloadedTaskData{}
	}
	req.ScreenName = screenName

	task := s.taskManager.CreateTask(TaskTypeMarkDownloaded, &req)

	var markTime *string
	if req.Timestamp != nil {
		t := req.Timestamp.Format("2006-01-02T15:04:05")
		markTime = &t
	}

	reporter := NewSSEProgressReporter(s, task.ID)

	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.MarkDownloaded(task.Ctx, task.ID, nil, nil, []string{screenName}, markTime, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"screen_name": screenName,
		"timestamp":   req.Timestamp,
		"message":     "Mark following downloaded task queued",
	}))
}

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

	task := s.taskManager.CreateTask(TaskTypeFollowingDownload, &req)

	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	reporter := NewSSEProgressReporter(s, task.ID)

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
	case "mark":
		s.handleListMark(w, r, listID)
	default:
		s.writeError(w, http.StatusNotFound, "Not found")
	}
}

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

	task := s.taskManager.CreateTask(TaskTypeListDownload, &req)

	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	reporter := NewSSEProgressReporter(s, task.ID)

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

func (s *Server) handleListProfile(w http.ResponseWriter, r *http.Request, listID uint64) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	req := ListProfileTaskData{ListID: listID}

	task := s.taskManager.CreateTask(TaskTypeListProfile, &req)

	reporter := NewSSEProgressReporter(s, task.ID)

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

	if len(req.Users) == 0 && len(req.Lists) == 0 && len(req.FollowingNames) == 0 {
		s.writeError(w, http.StatusBadRequest, "At least one of users, lists, or following_names is required")
		return
	}

	task := s.taskManager.CreateTask(TaskTypeBatchDownload, &req)

	opts := service.DownloadOptions{
		AutoFollow:  req.AutoFollow,
		SkipProfile: req.SkipProfile,
		NoRetry:     req.NoRetry,
	}

	reporter := NewSSEProgressReporter(s, task.ID)

	go func() {
		s.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
		err := s.downloadService.BatchDownload(task.Ctx, task.ID, req.Users, req.Lists, req.FollowingNames, opts, reporter)
		if err != nil {
			s.taskManager.SetTaskError(task.ID, err)
		}
	}()

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
