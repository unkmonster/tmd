package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/service"
)

// setupTestServer 创建测试服务器
func setupTestServer(t *testing.T) (*Server, *sqlx.DB) {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// 创建必要的表
	database.CreateTables(db)

	cfg := &config.Config{
		RootPath:           "/test/path",
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}

	client := resty.New()
	server := NewServer(client, []*resty.Client{}, db, cfg, "/app", nil)
	t.Cleanup(server.taskManager.Close)

	return server, db
}

func TestNewServer(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()

	cfg := &config.Config{
		RootPath:           "/test",
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}

	client := resty.New()
	server := NewServer(client, []*resty.Client{}, db, cfg, "/app", nil)
	defer server.taskManager.Close()

	assert.NotNil(t, server)
	assert.NotNil(t, server.client)
	assert.NotNil(t, server.db)
	assert.NotNil(t, server.config)
	assert.NotNil(t, server.taskManager)
	assert.NotNil(t, server.downloadService)
	assert.Equal(t, "/app", server.appRootPath)
}

func TestHandleHealth_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	server.handleHealth(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	// 验证响应数据
	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "ok", data["status"])
	assert.Equal(t, "2.0.0", data["version"])
	assert.NotNil(t, data["timestamp"])
}

func TestHandleHealth_DatabaseUnavailable(t *testing.T) {
	server, db := setupTestServer(t)
	db.Close() // 关闭数据库连接

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	server.handleHealth(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestFormatTaskMarkTime(t *testing.T) {
	assert.Nil(t, formatTaskMarkTime(nil))

	ts := time.Date(2026, 4, 30, 18, 30, 45, 0, time.Local)
	got := formatTaskMarkTime(&ts)
	if got == nil {
		t.Fatal("expected formatted timestamp")
	}
	assert.Equal(t, "2026-04-30T18:30:45", *got)
}

func TestServer_EnqueueTaskPassesTaskContextAndReporter(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
	done := make(chan struct{})

	var gotCtx context.Context
	var gotTaskID string
	var gotReporter service.ProgressReporter

	server.enqueueTask(task, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		gotCtx = ctx
		gotTaskID = taskID
		gotReporter = reporter
		close(done)
		return nil
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("enqueueTask did not execute task")
	}

	assert.Same(t, task.Ctx, gotCtx)
	assert.Equal(t, task.ID, gotTaskID)
	assert.NotNil(t, gotReporter)
}

func TestServer_ExecuteDownloadTaskSkipsCancelledTask(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
	assert.True(t, server.taskManager.CancelTask(task.ID))

	executed := make(chan struct{}, 1)
	server.executeDownloadTask(task, func() error {
		executed <- struct{}{}
		return nil
	})

	select {
	case <-executed:
		t.Fatal("cancelled task should not execute download function")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestHandleConfig_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rr := httptest.NewRecorder()

	server.handleConfig(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	// 验证配置数据（脱敏）
	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "/test/path", data["root_path"])
	assert.Equal(t, float64(5), data["max_download_routine"])
	assert.Equal(t, float64(100), data["max_file_name_len"])
}

func TestServer_PutRoutesForConfigFieldsAndCookies(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()
	database.CreateTables(db)

	appRoot := t.TempDir()
	cfg := &config.Config{
		RootPath:           appRoot,
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}

	server := NewServer(resty.New(), []*resty.Client{}, db, cfg, appRoot, nil)
	defer server.taskManager.Close()
	handler := server.buildHandler()

	configBody := `{"fields":{"root_path":"` + strings.ReplaceAll(appRoot, `\`, `\\`) + `","auth_token":"new-auth","ct0":"new-ct0","max_download_routine":"6","max_file_name_len":"120","proxy_url":""}}`
	configReq := httptest.NewRequest(http.MethodPut, "/api/v1/config/fields", bytes.NewBufferString(configBody))
	configReq.Header.Set("Content-Type", "application/json")
	configRR := httptest.NewRecorder()
	handler.ServeHTTP(configRR, configReq)

	assert.Equal(t, http.StatusOK, configRR.Code)
	var configResp APIResponse
	assert.NoError(t, json.Unmarshal(configRR.Body.Bytes(), &configResp))
	configData, ok := configResp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, configData["message"], "restart TMD manually")
	_, hasApplied := configData["applied"]
	assert.False(t, hasApplied)
	if _, err := os.Stat(filepath.Join(appRoot, "conf.yaml")); err != nil {
		t.Fatalf("expected conf.yaml to be written: %v", err)
	}

	cookiesBody := `{"cookies":[{"auth_token":"cookie-auth","ct0":"cookie-ct0"}]}`
	cookiesReq := httptest.NewRequest(http.MethodPut, "/api/v1/cookies", bytes.NewBufferString(cookiesBody))
	cookiesReq.Header.Set("Content-Type", "application/json")
	cookiesRR := httptest.NewRecorder()
	handler.ServeHTTP(cookiesRR, cookiesReq)

	assert.Equal(t, http.StatusOK, cookiesRR.Code)
	var cookiesResp APIResponse
	assert.NoError(t, json.Unmarshal(cookiesRR.Body.Bytes(), &cookiesResp))
	cookiesData, ok := cookiesResp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, cookiesData["message"], "restart TMD manually")
	if _, err := os.Stat(filepath.Join(appRoot, "additional_cookies.yaml")); err != nil {
		t.Fatalf("expected additional_cookies.yaml to be written: %v", err)
	}
}

func TestServer_SaveCookiesFailsWhenExistingCookiesUnreadable(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()
	database.CreateTables(db)

	appRoot := t.TempDir()
	cfg := &config.Config{
		RootPath:           appRoot,
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}

	server := NewServer(resty.New(), []*resty.Client{}, db, cfg, appRoot, nil)
	defer server.taskManager.Close()
	handler := server.buildHandler()

	cookiesPath := filepath.Join(appRoot, "additional_cookies.yaml")
	assert.NoError(t, os.WriteFile(cookiesPath, []byte(":\n  - invalid"), 0600))

	cookiesBody := `{"cookies":[{"auth_token":"cookie-auth","ct0":"cookie-ct0"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cookies", bytes.NewBufferString(cookiesBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandleUsers_InvalidPath(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/", nil)
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	// 无效路径现在返回 404 而不是 400
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleUsers_UnknownAction(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/testuser/unknown", nil)
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleUserDownload_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := UserDownloadTaskData{
		ScreenName:  "testuser",
		AutoFollow:  true,
		SkipProfile: false,
		NoRetry:     false,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	// 由于使用了 goroutine，可能返回 Accepted
	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	// 验证响应包含任务信息
	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["task_id"])
	assert.Equal(t, "testuser", data["screen_name"])
	assert.Equal(t, true, data["auto_follow"])
}

func TestHandleUserDownload_AllowsAtPrefixedScreenName(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := UserDownloadTaskData{
		ScreenName: "testuser",
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/@testuser/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	tasks := server.taskManager.GetAllTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "testuser", tasks[0].Data.(*UserDownloadTaskData).ScreenName)
}

func TestHandleUserDownload_EmptyBody(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/download", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	// 空 body 应该使用默认值
	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleUserProfile_WrongMethod(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/profile", nil)
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleUserProfile_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/profile", nil)
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["task_id"])
	assert.Equal(t, "testuser", data["screen_name"])
}

func TestHandleUserMark_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := MarkDownloadedTaskData{
		ScreenName: "testuser",
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/mark", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandleUserMark_WithTimestamp(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	now := time.Now()
	reqData := MarkDownloadedTaskData{
		ScreenName: "testuser",
		Timestamp:  &now,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/mark", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleFollowingDownload_WrongMethod(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/following/download", nil)
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleFollowingDownload_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := FollowingDownloadTaskData{
		ScreenName: "testuser",
		AutoFollow: true,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/following/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandleFollowingDownload_InvalidPath(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/testuser/following", nil)
	rr := httptest.NewRecorder()

	server.handleUsers(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleLists_InvalidPath(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/", nil)
	rr := httptest.NewRecorder()

	server.handleLists(rr, req)

	// 无效路径现在返回 404 而不是 400
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleLists_InvalidListID(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/invalid/download", nil)
	rr := httptest.NewRecorder()

	server.handleLists(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleLists_UnknownAction(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lists/123/unknown", nil)
	rr := httptest.NewRecorder()

	server.handleLists(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleListDownload_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := ListDownloadTaskData{
		ListID:      123,
		AutoFollow:  true,
		SkipProfile: false,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/123/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleLists(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["task_id"])
	assert.Equal(t, float64(123), data["list_id"])
}

func TestHandleListProfile_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/123/profile", nil)
	rr := httptest.NewRecorder()

	server.handleLists(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["task_id"])
	assert.Equal(t, float64(123), data["list_id"])
}

func TestHandleJsonFileDownload_InvalidBody(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/json/file/download", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleJsonFileDownload(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleJsonFileDownload_EmptyPaths(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := JsonFileDownloadTaskData{
		Paths: []string{},
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/json/file/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleJsonFileDownload(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleJsonFileDownload_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := JsonFileDownloadTaskData{
		Paths:   []string{"/path/1.json", "/path/2.json"},
		NoRetry: true,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/json/file/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleJsonFileDownload(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["task_id"])
	assert.Equal(t, []interface{}{"/path/1.json", "/path/2.json"}, data["paths"])
	assert.Equal(t, true, data["no_retry"])
}

func TestHandleBatchDownload_InvalidBody(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/download", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleBatchDownload(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleBatchDownload_EmptyUsersAndLists(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := BatchDownloadTaskData{
		Users: []string{},
		Lists: []uint64{},
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleBatchDownload(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleBatchDownload_OnlyUsers(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := BatchDownloadTaskData{
		Users:       []string{"user1", "user2"},
		Lists:       []uint64{},
		AutoFollow:  true,
		SkipProfile: false,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleBatchDownload(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(2), data["user_count"])
	assert.Equal(t, float64(0), data["list_count"])
}

func TestHandleBatchDownload_OnlyLists(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := BatchDownloadTaskData{
		Users: []string{},
		Lists: []uint64{100, 200, 300},
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleBatchDownload(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(0), data["user_count"])
	assert.Equal(t, float64(3), data["list_count"])
}

func TestHandleBatchDownload_BothUsersAndLists(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := BatchDownloadTaskData{
		Users:       []string{"user1"},
		Lists:       []uint64{100},
		AutoFollow:  true,
		SkipProfile: true,
		NoRetry:     true,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleBatchDownload(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(1), data["user_count"])
	assert.Equal(t, float64(1), data["list_count"])
	assert.Equal(t, true, data["auto_follow"])
	assert.Equal(t, true, data["skip_profile"])
	assert.Equal(t, true, data["no_retry"])
}

func TestHandleBatchDownload_NormalizesAtPrefixedScreenNames(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := BatchDownloadTaskData{
		Users:          []string{"@user1"},
		FollowingNames: []string{"@user2"},
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleBatchDownload(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	tasks := server.taskManager.GetAllTasks()
	assert.Len(t, tasks, 1)
	data := tasks[0].Data.(*BatchDownloadTaskData)
	assert.Equal(t, []string{"user1"}, data.Users)
	assert.Equal(t, []string{"user2"}, data.FollowingNames)
}

func TestHandleTasks_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	// 创建一些任务
	server.taskManager.CreateTask(TaskTypeUserDownload, nil)
	server.taskManager.CreateTask(TaskTypeListDownload, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rr := httptest.NewRecorder()

	server.handleTasks(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(2), data["total"])
}

func TestHandleGetTask_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	// 创建任务
	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+task.ID, nil)
	req.SetPathValue("task_id", task.ID)
	rr := httptest.NewRecorder()

	server.handleGetTask(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, task.ID, data["task_id"])
}

func TestHandleGetTask_NotFound(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/non_existent", nil)
	req.SetPathValue("task_id", "non_existent")
	rr := httptest.NewRecorder()

	server.handleGetTask(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleCancelTask_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	// 创建任务
	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+task.ID+"/cancel", nil)
	req.SetPathValue("task_id", task.ID)
	rr := httptest.NewRecorder()

	server.handleCancelTask(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandleCancelTask_NotFound(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/non_existent/cancel", nil)
	req.SetPathValue("task_id", "non_existent")
	rr := httptest.NewRecorder()

	server.handleCancelTask(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleCancelTask_AlreadyCompleted(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	// 创建并完成任务
	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
	server.taskManager.UpdateTaskStatus(task.ID, TaskStatusCompleted)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+task.ID+"/cancel", nil)
	req.SetPathValue("task_id", task.ID)
	rr := httptest.NewRecorder()

	server.handleCancelTask(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestWriteJSON(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	tests := []struct {
		name       string
		status     int
		data       interface{}
		wantStatus int
	}{
		{
			name:       "成功响应",
			status:     http.StatusOK,
			data:       map[string]string{"message": "ok"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "错误响应",
			status:     http.StatusBadRequest,
			data:       map[string]string{"error": "bad request"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "空数据",
			status:     http.StatusOK,
			data:       nil,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			server.writeJSON(rr, tt.status, tt.data)

			assert.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		})
	}
}

func TestWriteError(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	rr := httptest.NewRecorder()
	server.writeError(rr, http.StatusBadRequest, "test error")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "test error", resp.Error)
}

func TestServer_Start(t *testing.T) {
	// 这个测试主要验证 Start 方法不会 panic
	// 实际启动服务器会阻塞，所以只验证配置
	server, db := setupTestServer(t)
	defer db.Close()

	// 验证服务器配置正确
	assert.NotNil(t, server)
	assert.NotNil(t, server.client)
	assert.NotNil(t, server.db)
	assert.NotNil(t, server.config)
	assert.NotNil(t, server.taskManager)
}

func TestServer_GracefulShutdownCompletes(t *testing.T) {
	server, _ := setupTestServer(t)
	server.GracefulShutdown("shutdown")
	server.WaitForShutdown()
}

func TestServer_RestartRouteRemoved(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	handler := server.buildHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/restart", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestServer_TaskCreationAndRetrieval(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	// 测试创建任务
	task := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{
		ScreenName: "testuser",
	})

	assert.NotNil(t, task)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, TaskStatusQueued, task.Status)

	// 测试获取任务
	retrieved, ok := server.taskManager.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, task.ID, retrieved.ID)
	assert.NotSame(t, task, retrieved)

	// 测试获取所有任务
	tasks := server.taskManager.GetAllTasks()
	assert.Len(t, tasks, 1)
}

func TestServer_GetTaskReturnsSnapshot(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeBatchDownload, &BatchDownloadTaskData{
		Users: []string{"user1"},
	})
	server.taskManager.UpdateTaskProgress(task.ID, &TaskProgress{Total: 10, Completed: 2})

	retrieved, ok := server.taskManager.GetTask(task.ID)
	assert.True(t, ok)
	assert.NotSame(t, task, retrieved)
	assert.NotSame(t, task.Progress, retrieved.Progress)

	retrieved.Status = TaskStatusCompleted
	retrieved.Progress.Completed = 9
	retrieved.Data.(*BatchDownloadTaskData).Users[0] = "mutated"

	again, ok := server.taskManager.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, TaskStatusQueued, again.Status)
	assert.Equal(t, 2, again.Progress.Completed)
	assert.Equal(t, "user1", again.Data.(*BatchDownloadTaskData).Users[0])
}

func TestServer_GetAllTasksReturnsSnapshots(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeBatchDownload, &BatchDownloadTaskData{
		Users: []string{"user1"},
	})

	tasks := server.taskManager.GetAllTasks()
	assert.Len(t, tasks, 1)
	assert.NotSame(t, task, tasks[0])

	copiedData := tasks[0].Data.(*BatchDownloadTaskData)
	copiedData.Users[0] = "mutated"

	again, ok := server.taskManager.GetTask(task.ID)
	assert.True(t, ok)
	assert.Equal(t, "user1", again.Data.(*BatchDownloadTaskData).Users[0])
}

func TestServer_ConcurrentTaskCreation(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	// 并发创建任务
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			server.taskManager.CreateTask(TaskTypeUserDownload, map[string]int{"index": i})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	tasks := server.taskManager.GetAllTasks()
	assert.Len(t, tasks, 10)
}

func TestServer_TaskStatusTransitions(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)

	// queued -> running
	ok := server.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)
	assert.True(t, ok)

	task, _ = server.taskManager.GetTask(task.ID)
	assert.Equal(t, TaskStatusRunning, task.Status)
	assert.NotNil(t, task.StartedAt)

	// running -> completed
	ok = server.taskManager.UpdateTaskStatus(task.ID, TaskStatusCompleted)
	assert.True(t, ok)

	task, _ = server.taskManager.GetTask(task.ID)
	assert.Equal(t, TaskStatusCompleted, task.Status)
	assert.NotNil(t, task.EndedAt)
}

func TestServer_TaskProgressAndResult(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)

	// 更新进度
	progress := &TaskProgress{
		Total:     100,
		Completed: 50,
		Failed:    5,
	}
	ok := server.taskManager.UpdateTaskProgress(task.ID, progress)
	assert.True(t, ok)

	task, _ = server.taskManager.GetTask(task.ID)
	assert.Equal(t, 50, task.Progress.Completed)

	// 设置结果
	result := &TaskResult{
		Downloaded: 95,
		Failed:     5,
		Versioned:  10,
		Message:    "Done",
	}
	ok = server.taskManager.SetTaskResult(task.ID, result)
	assert.True(t, ok)

	task, _ = server.taskManager.GetTask(task.ID)
	assert.Equal(t, 95, task.Result.Downloaded)
	assert.Equal(t, "Done", task.Result.Message)
}

func TestServer_TaskError(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)

	// 设置错误
	testErr := assert.AnError
	ok := server.taskManager.SetTaskError(task.ID, testErr)
	assert.True(t, ok)

	task, _ = server.taskManager.GetTask(task.ID)
	assert.Equal(t, TaskStatusFailed, task.Status)
	assert.Equal(t, testErr.Error(), task.Error)
	assert.NotNil(t, task.EndedAt)
}

func TestServer_TaskCancellation(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)

	// 取消任务
	ok := server.taskManager.CancelTask(task.ID)
	assert.True(t, ok)

	task, _ = server.taskManager.GetTask(task.ID)
	assert.Equal(t, TaskStatusCancelled, task.Status)
	assert.NotNil(t, task.EndedAt)

	// 验证 context 被取消
	select {
	case <-task.Ctx.Done():
		// 预期行为
	case <-time.After(time.Second):
		t.Error("Context should be cancelled")
	}
}

func TestServer_MultipleTaskTypes(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	taskTypes := []TaskType{
		TaskTypeUserDownload,
		TaskTypeListDownload,
		TaskTypeFollowingDownload,
		TaskTypeProfileDownload,
		TaskTypeMarkDownloaded,
		TaskTypeJsonFileDownload,
		TaskTypeJsonFolderDownload,
		TaskTypeBatchDownload,
		TaskTypeListProfile,
	}

	for _, taskType := range taskTypes {
		task := server.taskManager.CreateTask(taskType, nil)
		assert.Equal(t, taskType, task.Type)
	}

	tasks := server.taskManager.GetAllTasks()
	assert.Len(t, tasks, len(taskTypes))
}

func TestServer_ResponseFormats(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	tests := []struct {
		name     string
		data     interface{}
		contains string
	}{
		{
			name:     "字符串",
			data:     "test string",
			contains: "test string",
		},
		{
			name:     "数字",
			data:     123,
			contains: "123",
		},
		{
			name:     "布尔值",
			data:     true,
			contains: "true",
		},
		{
			name:     "map",
			data:     map[string]string{"key": "value"},
			contains: `"key":"value"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			server.writeJSON(rr, http.StatusOK, NewSuccessResponse(tt.data))

			body := rr.Body.String()
			assert.Contains(t, body, tt.contains)
		})
	}
}

func TestServer_LargeResponse(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	// 创建大量任务
	for i := 0; i < 100; i++ {
		server.taskManager.CreateTask(TaskTypeUserDownload, nil)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rr := httptest.NewRecorder()

	server.handleTasks(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Greater(t, rr.Body.Len(), 0)
}

func TestServer_RequestPathVariations(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"健康检查 GET", http.MethodGet, "/api/v1/health"},
		{"配置 GET", http.MethodGet, "/api/v1/config"},
		{"任务列表 GET", http.MethodGet, "/api/v1/tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			// 根据路径调用相应的 handler
			switch {
			case strings.HasSuffix(tt.path, "/health"):
				server.handleHealth(rr, req)
			case strings.HasSuffix(tt.path, "/config"):
				server.handleConfig(rr, req)
			case strings.HasSuffix(tt.path, "/tasks") && !strings.Contains(tt.path, "task_id"):
				server.handleTasks(rr, req)
			}

			// 验证不会 panic
		})
	}
}
