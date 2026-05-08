package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/scheduler"
	"github.com/unkmonster/tmd/internal/service"
	"github.com/unkmonster/tmd/internal/utils"
)

const (
	maxUploadRequestSize = 200 << 20
	maxUploadFileSize    = 50 << 20
	maxUploadMemory      = 32 << 20
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
	taskID := task.ID
	status := task.Status

	opts := service.DownloadOptions{
		AutoFollow:    req.AutoFollow,
		FollowMembers: req.FollowMembers,
		SkipProfile:   req.SkipProfile,
		NoRetry:       req.NoRetry,
	}

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.UserDownload(ctx, taskID, screenName, opts, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":        taskID,
		"status":         status,
		"screen_name":    req.ScreenName,
		"auto_follow":    req.AutoFollow,
		"follow_members": req.FollowMembers,
		"skip_profile":   req.SkipProfile,
		"no_retry":       req.NoRetry,
		"message":        "Download task queued successfully",
	}))
}

func (s *Server) handleUserProfile(w http.ResponseWriter, _ *http.Request, screenName string) {
	req := ProfileDownloadTaskData{ScreenName: screenName}

	task := s.taskManager.CreateTask(TaskTypeProfileDownload, &req)
	taskID := task.ID
	status := task.Status

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.ProfileDownload(ctx, taskID, []string{screenName}, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     taskID,
		"status":      status,
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
	taskID := task.ID
	status := task.Status

	markTime := formatTaskMarkTime(req.Timestamp)

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.MarkDownloaded(ctx, taskID, []string{screenName}, nil, nil, markTime, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     taskID,
		"status":      status,
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
	req.ListID = StringUint64(listID)

	task := s.taskManager.CreateTask(TaskTypeMarkDownloaded, &req)
	taskID := task.ID
	status := task.Status

	markTime := formatTaskMarkTime(req.Timestamp)

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.MarkDownloaded(ctx, taskID, nil, []uint64{listID}, nil, markTime, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":   taskID,
		"status":    status,
		"list_id":   StringUint64(listID),
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
	taskID := task.ID
	status := task.Status

	markTime := formatTaskMarkTime(req.Timestamp)

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.MarkDownloaded(ctx, taskID, nil, nil, []string{screenName}, markTime, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":     taskID,
		"status":      status,
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
	taskID := task.ID
	status := task.Status

	opts := service.DownloadOptions{
		AutoFollow:    req.AutoFollow,
		FollowMembers: req.FollowMembers,
		SkipProfile:   req.SkipProfile,
		NoRetry:       req.NoRetry,
	}

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.FollowingDownload(ctx, taskID, screenName, opts, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":        taskID,
		"status":         status,
		"screen_name":    req.ScreenName,
		"auto_follow":    req.AutoFollow,
		"follow_members": req.FollowMembers,
		"skip_profile":   req.SkipProfile,
		"no_retry":       req.NoRetry,
		"message":        "Following download task queued successfully",
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
	req.ListID = StringUint64(listID)

	task := s.taskManager.CreateTask(TaskTypeListDownload, &req)
	taskID := task.ID
	status := task.Status

	opts := service.DownloadOptions{
		AutoFollow:    req.AutoFollow,
		FollowMembers: req.FollowMembers,
		SkipProfile:   req.SkipProfile,
		NoRetry:       req.NoRetry,
	}

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.ListDownload(ctx, taskID, listID, opts, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":        taskID,
		"status":         status,
		"list_id":        StringUint64(listID),
		"skip_profile":   req.SkipProfile,
		"auto_follow":    req.AutoFollow,
		"follow_members": req.FollowMembers,
		"no_retry":       req.NoRetry,
		"message":        "List download task queued",
	}))
}

func (s *Server) handleListProfile(w http.ResponseWriter, _ *http.Request, listID uint64) {
	req := ListProfileTaskData{ListID: StringUint64(listID)}

	task := s.taskManager.CreateTask(TaskTypeListProfile, &req)
	taskID := task.ID
	status := task.Status

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.ListProfileDownload(ctx, taskID, listID, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id": taskID,
		"status":  status,
		"list_id": StringUint64(listID),
		"message": "List profile download task queued",
	}))
}

func (s *Server) handleJsonFileDownload(w http.ResponseWriter, r *http.Request) {
	if isMultipartRequest(r) {
		s.handleJsonFileUpload(w, r)
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
	taskID := task.ID
	status := task.Status
	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.JsonFileDownload(ctx, taskID, req.Paths, req.NoRetry, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":  taskID,
		"status":   status,
		"paths":    req.Paths,
		"no_retry": req.NoRetry,
		"message":  "JSON file download task queued",
	}))
}

func (s *Server) handleJsonFolderDownload(w http.ResponseWriter, r *http.Request) {
	if isMultipartRequest(r) {
		s.handleJsonFolderUpload(w, r)
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
	taskID := task.ID
	status := task.Status
	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.JsonFolderDownload(ctx, taskID, req.Paths, req.NoRetry, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":  taskID,
		"status":   status,
		"paths":    req.Paths,
		"no_retry": req.NoRetry,
		"message":  "JSON folder download task queued",
	}))
}

func (s *Server) handleJsonFileUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestSize)

	paths, uploadDir, err := s.saveUploadedJSONFiles(r, "json")
	if err != nil {
		if uploadDir != "" {
			_ = os.RemoveAll(uploadDir)
		}
		writeUploadError(w, err)
		return
	}

	req := JsonFileDownloadTaskData{
		Paths:   paths,
		NoRetry: parseMultipartNoRetry(r),
	}

	task := s.taskManager.CreateTask(TaskTypeJsonFileDownload, &req)
	taskID := task.ID
	status := task.Status
	s.enqueueTask(task, cleanupUploadDirAfterTask(uploadDir, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.JsonFileDownload(ctx, taskID, req.Paths, req.NoRetry, reporter)
	}))

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":    taskID,
		"status":     status,
		"file_count": len(paths),
		"no_retry":   req.NoRetry,
		"message":    "JSON file upload task queued",
	}))
}

func (s *Server) handleJsonFolderUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestSize)

	paths, uploadDir, err := s.saveUploadedJSONFiles(r, "loongtweet")
	if err != nil {
		if uploadDir != "" {
			_ = os.RemoveAll(uploadDir)
		}
		writeUploadError(w, err)
		return
	}

	req := JsonFolderDownloadTaskData{
		Paths:   []string{uploadDir},
		NoRetry: parseMultipartNoRetry(r),
	}

	task := s.taskManager.CreateTask(TaskTypeJsonFolderDownload, &req)
	taskID := task.ID
	status := task.Status
	s.enqueueTask(task, cleanupUploadDirAfterTask(uploadDir, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.JsonFolderDownload(ctx, taskID, req.Paths, req.NoRetry, reporter)
	}))

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":    taskID,
		"status":     status,
		"file_count": len(paths),
		"no_retry":   req.NoRetry,
		"message":    "LoongTweet upload task queued",
	}))
}

func isMultipartRequest(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && strings.EqualFold(mediaType, "multipart/form-data")
}

func parseMultipartNoRetry(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.FormValue("no_retry")), "true")
}

func cleanupUploadDirAfterTask(uploadDir string, task func(ctx context.Context, taskID string, reporter service.ProgressReporter) error) func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
	return func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		if uploadDir != "" {
			defer os.RemoveAll(uploadDir)
		}
		return task(ctx, taskID, reporter)
	}
}

func (s *Server) saveUploadedJSONFiles(r *http.Request, uploadKind string) ([]string, string, error) {
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		return nil, "", err
	}
	defer r.MultipartForm.RemoveAll()

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		return nil, "", fmt.Errorf("no files uploaded")
	}

	uploadParent := filepath.Join(s.appRootPath, "uploads", uploadKind)
	if err := os.MkdirAll(uploadParent, 0o755); err != nil {
		return nil, "", fmt.Errorf("failed to create upload directory: %w", err)
	}
	uploadDir, err := os.MkdirTemp(uploadParent, strconv.FormatInt(time.Now().UnixNano(), 10)+"-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	paths := make([]string, 0, len(files))
	usedNames := make(map[string]struct{}, len(files))
	for _, fileHeader := range files {
		name, err := validateUploadFile(fileHeader)
		if err != nil {
			return nil, uploadDir, err
		}

		dstPath := filepath.Join(uploadDir, uniqueUploadFileName(name, usedNames))
		if err := copyUploadedFile(fileHeader, dstPath); err != nil {
			return nil, uploadDir, err
		}
		paths = append(paths, dstPath)
	}

	return paths, uploadDir, nil
}

func validateUploadFile(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Size > maxUploadFileSize {
		return "", fmt.Errorf("uploaded file too large: %s exceeds %d bytes", fileHeader.Filename, maxUploadFileSize)
	}
	return validateUploadFileName(fileHeader.Filename)
}

func validateUploadFileName(fileName string) (string, error) {
	name := strings.TrimSpace(fileName)
	if name == "" {
		return "", fmt.Errorf("invalid file name")
	}
	if strings.ContainsAny(name, `/\<>:"|?*`) {
		return "", fmt.Errorf("invalid file name: %s", fileName)
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".json" {
		return name, nil
	}
	return "", fmt.Errorf("unsupported file type: %s", fileName)
}

func uniqueUploadFileName(fileName string, used map[string]struct{}) string {
	key := strings.ToLower(fileName)
	if _, exists := used[key]; !exists {
		used[key] = struct{}{}
		return fileName
	}

	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d%s", base, index, ext)
		key := strings.ToLower(candidate)
		if _, exists := used[key]; !exists {
			used[key] = struct{}{}
			return candidate
		}
	}
}

func copyUploadedFile(fileHeader *multipart.FileHeader, dstPath string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file %s: %w", fileHeader.Filename, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create upload file %s: %w", filepath.Base(dstPath), err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to save uploaded file %s: %w", fileHeader.Filename, err)
	}
	return nil
}

func writeUploadError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeJSONError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Upload too large: max %d bytes", maxBytesErr.Limit))
		return
	}

	statusCode := http.StatusBadRequest
	if strings.HasPrefix(err.Error(), "failed to create upload") ||
		strings.HasPrefix(err.Error(), "failed to save uploaded") ||
		strings.HasPrefix(err.Error(), "failed to open uploaded") {
		statusCode = http.StatusInternalServerError
	}

	writeJSONError(w, statusCode, err.Error())
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(NewErrorResponse(message))
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
	listIDs := stringUint64SliceToUint64(req.Lists)
	for _, listID := range listIDs {
		if listID == 0 {
			s.writeError(w, http.StatusBadRequest, "Invalid list ID: must be greater than 0")
			return
		}
	}

	task := s.taskManager.CreateTask(TaskTypeBatchDownload, &req)
	taskID := task.ID
	status := task.Status

	opts := service.DownloadOptions{
		AutoFollow:    req.AutoFollow,
		FollowMembers: req.FollowMembers,
		SkipProfile:   req.SkipProfile,
		NoRetry:       req.NoRetry,
	}

	s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		return s.downloadService.BatchDownload(ctx, taskID, req.Users, listIDs, req.FollowingNames, opts, reporter)
	})

	s.writeJSON(w, http.StatusAccepted, NewSuccessResponse(map[string]interface{}{
		"task_id":         taskID,
		"status":          status,
		"users":           req.Users,
		"lists":           req.Lists,
		"following_names": req.FollowingNames,
		"user_count":      len(req.Users),
		"list_count":      len(req.Lists),
		"following_count": len(req.FollowingNames),
		"auto_follow":     req.AutoFollow,
		"follow_members":  req.FollowMembers,
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

func (s *Server) scheduledDownload(entry scheduler.ScheduleEntry) string {
	opts := service.DownloadOptions{
		AutoFollow:    entry.AutoFollow,
		FollowMembers: entry.FollowMembers,
		SkipProfile:   entry.SkipProfile,
		NoRetry:       entry.NoRetry,
	}

	switch entry.Type {
	case scheduler.ScheduleTypeList:
		listID, err := strconv.ParseUint(entry.Target, 10, 64)
		if err != nil {
			log.Warnf("[scheduler] Invalid list_id %q: %v", entry.Target, err)
			return ""
		}
		if listID == 0 {
			log.Warnf("[scheduler] Invalid list_id %q: must be a positive integer", entry.Target)
			return ""
		}
		req := &ListDownloadTaskData{
			ListID:        StringUint64(listID),
			AutoFollow:    entry.AutoFollow,
			FollowMembers: entry.FollowMembers,
			SkipProfile:   entry.SkipProfile,
			NoRetry:       entry.NoRetry,
		}
		task := s.taskManager.CreateTask(TaskTypeListDownload, req)
		s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
			return s.downloadService.ListDownload(ctx, taskID, listID, opts, reporter)
		})
		return task.ID

	case scheduler.ScheduleTypeUser:
		req := &UserDownloadTaskData{
			ScreenName:    entry.Target,
			AutoFollow:    entry.AutoFollow,
			FollowMembers: entry.FollowMembers,
			SkipProfile:   entry.SkipProfile,
			NoRetry:       entry.NoRetry,
		}
		task := s.taskManager.CreateTask(TaskTypeUserDownload, req)
		s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
			return s.downloadService.UserDownload(ctx, taskID, entry.Target, opts, reporter)
		})
		return task.ID

	case scheduler.ScheduleTypeFollowing:
		req := &FollowingDownloadTaskData{
			ScreenName:    entry.Target,
			AutoFollow:    entry.AutoFollow,
			FollowMembers: entry.FollowMembers,
			SkipProfile:   entry.SkipProfile,
			NoRetry:       entry.NoRetry,
		}
		task := s.taskManager.CreateTask(TaskTypeFollowingDownload, req)
		s.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
			return s.downloadService.FollowingDownload(ctx, taskID, entry.Target, opts, reporter)
		})
		return task.ID

	default:
		log.Warnf("[scheduler] Unknown schedule type: %q", entry.Type)
	}

	return ""
}
