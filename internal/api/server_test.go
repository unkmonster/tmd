package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/scheduler"
	"github.com/unkmonster/tmd/internal/service"
)

type batchDownloadCall struct {
	taskID         string
	users          []string
	listIDs        []uint64
	followingNames []string
	opts           service.DownloadOptions
}

type fakeDownloadService struct {
	batchCalls chan batchDownloadCall
}

func (f *fakeDownloadService) UserDownload(context.Context, string, string, service.DownloadOptions, service.ProgressReporter) error {
	return errors.New("unexpected UserDownload call")
}

func (f *fakeDownloadService) ListDownload(context.Context, string, uint64, service.DownloadOptions, service.ProgressReporter) error {
	return errors.New("unexpected ListDownload call")
}

func (f *fakeDownloadService) FollowingDownload(context.Context, string, string, service.DownloadOptions, service.ProgressReporter) error {
	return errors.New("unexpected FollowingDownload call")
}

func (f *fakeDownloadService) ProfileDownload(context.Context, string, []string, service.ProgressReporter) error {
	return errors.New("unexpected ProfileDownload call")
}

func (f *fakeDownloadService) ListProfileDownload(context.Context, string, uint64, service.ProgressReporter) error {
	return errors.New("unexpected ListProfileDownload call")
}

func (f *fakeDownloadService) MarkDownloaded(context.Context, string, []string, []uint64, []string, *string, service.ProgressReporter) error {
	return errors.New("unexpected MarkDownloaded call")
}

func (f *fakeDownloadService) JsonFileDownload(context.Context, string, []string, bool, service.ProgressReporter) error {
	return errors.New("unexpected JsonFileDownload call")
}

func (f *fakeDownloadService) JsonFolderDownload(context.Context, string, []string, bool, service.ProgressReporter) error {
	return errors.New("unexpected JsonFolderDownload call")
}

func (f *fakeDownloadService) BatchDownload(_ context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, opts service.DownloadOptions, _ service.ProgressReporter) error {
	if f.batchCalls != nil {
		f.batchCalls <- batchDownloadCall{
			taskID:         taskID,
			users:          append([]string(nil), screenNames...),
			listIDs:        append([]uint64(nil), listIDs...),
			followingNames: append([]string(nil), followingNames...),
			opts:           opts,
		}
	}
	return nil
}

func (f *fakeDownloadService) RetryAllFailed(context.Context, string, service.ProgressReporter) error {
	return nil
}

func (f *fakeDownloadService) ClearErrors() error {
	return nil
}

// setupTestServer 创建测试服务器
func setupTestServer(t *testing.T) (*Server, *sqlx.DB) {
	return setupTestServerWithAppRoot(t, "/app")
}

func setupTestServerWithAppRoot(t *testing.T, appRoot string) (*Server, *sqlx.DB) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// 创建必要的表
	database.CreateTables(db)

	cfg := &config.Config{
		RootPath:           t.TempDir(),
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}

	client := resty.New()
	server := NewServer(client, []*resty.Client{}, db, cfg, appRoot, nil)
	t.Cleanup(server.taskManager.Close)
	t.Cleanup(func() {
		if server.downloadQueue != nil {
			server.downloadQueue.CloseAndWait(2 * time.Second)
		}
	})

	return server, db
}

type multipartUploadFile struct {
	name    string
	content string
}

func newMultipartUploadRequest(t *testing.T, target string, files []multipartUploadFile, noRetry bool) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.name)
		if err != nil {
			t.Fatalf("failed to create multipart form file: %v", err)
		}
		if _, err := part.Write([]byte(file.content)); err != nil {
			t.Fatalf("failed to write multipart form file: %v", err)
		}
	}
	if err := writer.WriteField("no_retry", strconv.FormatBool(noRetry)); err != nil {
		t.Fatalf("failed to write multipart field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, target, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func serveAPI(server *Server, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	server.buildHandler().ServeHTTP(rr, req)
	return rr
}

func TestNewServer(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	assert.NoError(t, err)
	defer db.Close()

	cfg := &config.Config{
		RootPath:           t.TempDir(),
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
	assert.NotNil(t, server.downloadQueue)
	assert.Equal(t, "/app", server.appRootPath)
}

func TestHandleUpdateSchedulesRawInitializesSchedulerAfterStartupParseFailure(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	assert.NoError(t, err)
	defer db.Close()
	database.CreateTables(db)

	appRoot := t.TempDir()
	err = os.WriteFile(filepath.Join(appRoot, "schedules.yaml"), []byte("schedules:\n  - type: ["), 0600)
	assert.NoError(t, err)

	cfg := &config.Config{
		RootPath:           t.TempDir(),
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}
	server := NewServer(resty.New(), []*resty.Client{}, db, cfg, appRoot, nil)
	defer server.taskManager.Close()
	assert.Nil(t, server.scheduler)

	body := `{"content":"schedules:\n  - type: user\n    target: alice\n    name: Alice\n    schedule: \"interval:1h\"\n    enabled: false\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/raw", strings.NewReader(body))
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, server.scheduler)
	statuses := server.scheduler.GetStatuses()
	if assert.Len(t, statuses, 1) {
		assert.Equal(t, "alice", statuses[0].Entry.Target)
	}

	reloadReq := httptest.NewRequest(http.MethodPost, "/api/v1/schedules/reload", nil)
	reloadRR := serveAPI(server, reloadReq)
	assert.Equal(t, http.StatusOK, reloadRR.Code)
}

func TestHandleGetSchedulesReturnsFrontendFieldNames(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	assert.NoError(t, err)
	defer db.Close()
	database.CreateTables(db)

	appRoot := t.TempDir()
	content := `schedules:
  - type: user
    target: alice
    name: Alice
    schedule: "interval:1h"
    enabled: true
    run_on_start: true
    auto_follow: true
  - type: list
    target: "12345"
    name: List
    schedule: "daily:07:00"
    enabled: false
    skip_profile: true
`
	err = os.WriteFile(filepath.Join(appRoot, "schedules.yaml"), []byte(content), 0600)
	assert.NoError(t, err)

	cfg := &config.Config{
		RootPath:           t.TempDir(),
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}
	server := NewServer(resty.New(), []*resty.Client{}, db, cfg, appRoot, nil)
	defer server.taskManager.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `"type":"user"`)
	assert.Contains(t, body, `"target":"alice"`)
	assert.Contains(t, body, `"run_on_start":true`)
	assert.Contains(t, body, `"auto_follow":true`)
	assert.Contains(t, body, `"type":"list"`)
	assert.Contains(t, body, `"skip_profile":true`)
	assert.NotContains(t, body, `"Type"`)
	assert.NotContains(t, body, `"AutoFollow"`)
}

func TestStructuredScheduleCRUDUsesStableID(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	assert.NoError(t, err)
	defer db.Close()
	database.CreateTables(db)

	appRoot := t.TempDir()
	cfg := &config.Config{
		RootPath:           t.TempDir(),
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}
	server := NewServer(resty.New(), []*resty.Client{}, db, cfg, appRoot, nil)
	defer server.taskManager.Close()

	createBody := `{"type":"list","target":"12345","name":"List A","schedule":"interval:1h","enabled":true,"run_on_start":false,"auto_follow":true}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", strings.NewReader(createBody))
	createRR := serveAPI(server, createReq)
	assert.Equal(t, http.StatusCreated, createRR.Code)

	var createResp APIResponse
	err = json.Unmarshal(createRR.Body.Bytes(), &createResp)
	assert.NoError(t, err)
	createData := createResp.Data.(map[string]interface{})
	createEntry := createData["entry"].(map[string]interface{})
	id := createEntry["id"].(string)
	assert.NotEmpty(t, id)

	updateBody := `{"type":"user","target":"alice","name":"Alice","schedule":"daily:07:00","enabled":false,"run_on_start":true,"skip_profile":true}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/"+id, strings.NewReader(updateBody))
	updateRR := serveAPI(server, updateReq)
	assert.Equal(t, http.StatusOK, updateRR.Code)

	enableReq := httptest.NewRequest(http.MethodPatch, "/api/v1/schedules/"+id+"/enabled", strings.NewReader(`{"enabled":true}`))
	enableRR := serveAPI(server, enableReq)
	assert.Equal(t, http.StatusOK, enableRR.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	getRR := serveAPI(server, getReq)
	assert.Equal(t, http.StatusOK, getRR.Code)
	getBody := getRR.Body.String()
	assert.Contains(t, getBody, `"id":"`+id+`"`)
	assert.Contains(t, getBody, `"type":"user"`)
	assert.Contains(t, getBody, `"target":"alice"`)
	assert.Contains(t, getBody, `"enabled":true`)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/schedules/"+id, nil)
	deleteRR := serveAPI(server, deleteReq)
	assert.Equal(t, http.StatusOK, deleteRR.Code)

	finalRR := serveAPI(server, getReq)
	assert.Equal(t, http.StatusOK, finalRR.Code)
	assert.Contains(t, finalRR.Body.String(), `"total":0`)
}

func TestReplaceSchedulesStartsStoppedSchedulerWhenEnabled(t *testing.T) {
	server, db := setupTestServerWithAppRoot(t, t.TempDir())
	defer db.Close()
	t.Cleanup(func() {
		if server.scheduler != nil {
			server.scheduler.Stop()
		}
	})

	require.NotNil(t, server.scheduler)
	require.False(t, server.scheduler.IsRunning())

	body := `{"entries":[{"type":"user","target":"alice","name":"Alice","schedule":"interval:1h","enabled":true}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules", strings.NewReader(body))
	rr := serveAPI(server, req)
	require.Equal(t, http.StatusOK, rr.Code)
	require.True(t, server.scheduler.IsRunning())

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	getRR := serveAPI(server, getReq)
	require.Equal(t, http.StatusOK, getRR.Code)
	assert.Contains(t, getRR.Body.String(), `"scheduler_running":true`)
	assert.Contains(t, getRR.Body.String(), `"enabled":true`)
}

func TestStructuredScheduleCRUDSupportsMixedAndNormalizesShape(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	assert.NoError(t, err)
	defer db.Close()
	database.CreateTables(db)

	appRoot := t.TempDir()
	cfg := &config.Config{
		RootPath:           t.TempDir(),
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}
	server := NewServer(resty.New(), []*resty.Client{}, db, cfg, appRoot, nil)
	defer server.taskManager.Close()

	createBody := `{"type":"mixed","target":"should-drop","users":["@alice"],"lists":["12345"],"following_names":[" bob "],"name":"Mixed A","schedule":"interval:1h","enabled":true}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", strings.NewReader(createBody))
	createRR := serveAPI(server, createReq)
	assert.Equal(t, http.StatusCreated, createRR.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	getRR := serveAPI(server, getReq)
	assert.Equal(t, http.StatusOK, getRR.Code)
	getBody := getRR.Body.String()
	assert.Contains(t, getBody, `"type":"mixed"`)
	assert.Contains(t, getBody, `"users":["alice"]`)
	assert.Contains(t, getBody, `"lists":["12345"]`)
	assert.Contains(t, getBody, `"following_names":["bob"]`)
	assert.NotContains(t, getBody, `"target":"should-drop"`)

	var createResp APIResponse
	err = json.Unmarshal(createRR.Body.Bytes(), &createResp)
	assert.NoError(t, err)
	createData := createResp.Data.(map[string]interface{})
	createEntry := createData["entry"].(map[string]interface{})
	id := createEntry["id"].(string)

	updateBody := `{"type":"user","target":"alice","users":["ghost"],"lists":["999"],"following_names":["noop"],"schedule":"daily:07:00","enabled":false}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/"+id, strings.NewReader(updateBody))
	updateRR := serveAPI(server, updateReq)
	assert.Equal(t, http.StatusOK, updateRR.Code)

	getRR = serveAPI(server, getReq)
	assert.Equal(t, http.StatusOK, getRR.Code)
	getBody = getRR.Body.String()
	assert.Contains(t, getBody, `"type":"user"`)
	assert.Contains(t, getBody, `"target":"alice"`)
	assert.NotContains(t, getBody, `"users":["ghost"]`)
	assert.NotContains(t, getBody, `"lists":["999"]`)
	assert.NotContains(t, getBody, `"following_names":["noop"]`)
}

func TestReplaceSchedulesBulkNormalizesAndReplacesAll(t *testing.T) {
	server, db := setupTestServerWithAppRoot(t, t.TempDir())
	defer db.Close()

	body := `{"entries":[
		{"type":"user","target":"alice","name":"Alice","schedule":"interval:1h","enabled":true},
		{"type":"mixed","target":"drop-me","users":["@bob"],"lists":["12345"],"following_names":[" carol "],"name":"Mixed","schedule":"daily:07:00","enabled":false}
	]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules", strings.NewReader(body))
	rr := serveAPI(server, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	respBody := rr.Body.String()
	assert.Contains(t, respBody, `"entries":[`)
	assert.Contains(t, respBody, `"id":"`)
	assert.Contains(t, respBody, `"target":"alice"`)
	assert.Contains(t, respBody, `"users":["bob"]`)
	assert.Contains(t, respBody, `"lists":["12345"]`)
	assert.Contains(t, respBody, `"following_names":["carol"]`)
	assert.NotContains(t, respBody, `"target":"drop-me"`)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	getRR := serveAPI(server, getReq)
	assert.Equal(t, http.StatusOK, getRR.Code)
	getBody := getRR.Body.String()
	assert.Contains(t, getBody, `"total":2`)
	assert.Contains(t, getBody, `"target":"alice"`)
	assert.Contains(t, getBody, `"users":["bob"]`)

	replaceBody := `{"entries":[{"type":"list","target":"67890","name":"Replacement","schedule":"interval:2h","enabled":true}]}`
	replaceReq := httptest.NewRequest(http.MethodPut, "/api/v1/schedules", strings.NewReader(replaceBody))
	replaceRR := serveAPI(server, replaceReq)
	assert.Equal(t, http.StatusOK, replaceRR.Code)

	finalRR := serveAPI(server, getReq)
	assert.Equal(t, http.StatusOK, finalRR.Code)
	finalBody := finalRR.Body.String()
	assert.Contains(t, finalBody, `"total":1`)
	assert.Contains(t, finalBody, `"target":"67890"`)
	assert.NotContains(t, finalBody, `"target":"alice"`)
	assert.NotContains(t, finalBody, `"users":["bob"]`)
}

func TestReplaceSchedulesBulkRejectsInvalidWithoutOverwriting(t *testing.T) {
	server, db := setupTestServerWithAppRoot(t, t.TempDir())
	defer db.Close()

	validBody := `{"entries":[{"type":"user","target":"alice","name":"Alice","schedule":"interval:1h","enabled":true}]}`
	validReq := httptest.NewRequest(http.MethodPut, "/api/v1/schedules", strings.NewReader(validBody))
	validRR := serveAPI(server, validReq)
	assert.Equal(t, http.StatusOK, validRR.Code)

	invalidBody := `{"entries":[{"type":"mixed","users":["bad-name"],"schedule":"interval:1h","enabled":true}]}`
	invalidReq := httptest.NewRequest(http.MethodPut, "/api/v1/schedules", strings.NewReader(invalidBody))
	invalidRR := serveAPI(server, invalidReq)
	assert.Equal(t, http.StatusBadRequest, invalidRR.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	getRR := serveAPI(server, getReq)
	assert.Equal(t, http.StatusOK, getRR.Code)
	getBody := getRR.Body.String()
	assert.Contains(t, getBody, `"total":1`)
	assert.Contains(t, getBody, `"target":"alice"`)
	assert.NotContains(t, getBody, `"bad-name"`)
}

func TestValidateScheduleRejectsInvalidMixedScreenName(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	body := `{"entries":[{"type":"mixed","users":["bad-name"],"schedule":"interval:1h"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedules/validate", strings.NewReader(body))
	rr := serveAPI(server, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"valid":false`)
}

func TestSchedulesRawSupportsMixed(t *testing.T) {
	server, db := setupTestServerWithAppRoot(t, t.TempDir())
	defer db.Close()

	body := `{"content":"schedules:\n  - type: mixed\n    users:\n      - alice\n    lists:\n      - 12345\n    following_names:\n      - bob\n    schedule: interval:1h\n    enabled: true\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/raw", strings.NewReader(body))
	rr := serveAPI(server, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	rawReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedules/raw", nil)
	rawRR := serveAPI(server, rawReq)
	assert.Equal(t, http.StatusOK, rawRR.Code)
	rawBody := rawRR.Body.String()
	assert.Contains(t, rawBody, "users:")
	assert.Contains(t, rawBody, "lists:")
	assert.Contains(t, rawBody, "following_names:")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	getRR := serveAPI(server, getReq)
	assert.Equal(t, http.StatusOK, getRR.Code)
	assert.Contains(t, getRR.Body.String(), `"type":"mixed"`)
}

func TestHandleHealth_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	oldVersion := Version
	Version = "test-version"
	t.Cleanup(func() { Version = oldVersion })

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
	assert.Equal(t, "test-version", data["version"])
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

func TestServer_QueueSkipsCancelledTask(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
	assert.Equal(t, CancelTaskResultCancelled, server.taskManager.CancelTask(task.ID))

	executed := make(chan struct{}, 1)
	server.enqueueTask(task, func(context.Context, string, service.ProgressReporter) error {
		executed <- struct{}{}
		return nil
	})

	select {
	case <-executed:
		t.Fatal("cancelled task should not execute download function")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestServer_QueueSerializesRunningTasks(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	firstTask := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
	secondTask := server.taskManager.CreateTask(TaskTypeUserDownload, nil)

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})

	server.enqueueTask(firstTask, func(context.Context, string, service.ProgressReporter) error {
		close(firstStarted)
		<-releaseFirst
		server.taskManager.CompleteTask(firstTask.ID, &TaskResult{Message: "first done"})
		return nil
	})

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not start")
	}

	server.enqueueTask(secondTask, func(context.Context, string, service.ProgressReporter) error {
		close(secondStarted)
		server.taskManager.CompleteTask(secondTask.ID, &TaskResult{Message: "second done"})
		return nil
	})

	select {
	case <-secondStarted:
		t.Fatal("second task should wait for the task slot")
	case <-time.After(100 * time.Millisecond):
	}

	secondSnapshot, ok := server.taskManager.GetTask(secondTask.ID)
	if !ok {
		t.Fatal("second task not found")
	}
	assert.Equal(t, TaskStatusQueued, secondSnapshot.Status)

	close(releaseFirst)

	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("second task did not start after slot was released")
	}
}

func TestServer_QueueSkipsCancelledPendingTaskAndRunsNextSameTarget(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	firstTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})
	cancelledTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})
	nextTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	cancelledStarted := make(chan struct{}, 1)
	nextStarted := make(chan struct{})

	server.enqueueTask(firstTask, func(context.Context, string, service.ProgressReporter) error {
		close(firstStarted)
		<-releaseFirst
		server.taskManager.CompleteTask(firstTask.ID, &TaskResult{Message: "first done"})
		return nil
	})

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not start")
	}

	server.enqueueTask(cancelledTask, func(context.Context, string, service.ProgressReporter) error {
		cancelledStarted <- struct{}{}
		return nil
	})
	server.enqueueTask(nextTask, func(context.Context, string, service.ProgressReporter) error {
		close(nextStarted)
		server.taskManager.CompleteTask(nextTask.ID, &TaskResult{Message: "next done"})
		return nil
	})

	assert.Equal(t, CancelTaskResultCancelled, server.taskManager.CancelTask(cancelledTask.ID))

	close(releaseFirst)

	select {
	case <-nextStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("next same-target task did not start after cancelled pending task was skipped")
	}

	select {
	case <-cancelledStarted:
		t.Fatal("cancelled pending task should not execute download function")
	default:
	}
}

func TestServer_QueueAllowsDifferentTargetWhenCancelledTaskDetached(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.downloadQueue.cancelGrace = 50 * time.Millisecond

	firstTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})
	secondTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "bob"})

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})

	server.enqueueTask(firstTask, func(context.Context, string, service.ProgressReporter) error {
		close(firstStarted)
		<-releaseFirst
		return nil
	})

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not start")
	}

	server.enqueueTask(secondTask, func(context.Context, string, service.ProgressReporter) error {
		close(secondStarted)
		server.taskManager.CompleteTask(secondTask.ID, &TaskResult{Message: "second done"})
		return nil
	})

	select {
	case <-secondStarted:
		t.Fatal("second task should wait while first task holds the slot")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Equal(t, CancelTaskResultCancelled, server.taskManager.CancelTask(firstTask.ID))

	select {
	case <-secondStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("second task did not start after first task was cancelled and detached")
	}

	close(releaseFirst)
}

func TestServer_QueueBlocksSameTargetWhenCancelledTaskDetached(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.downloadQueue.cancelGrace = 50 * time.Millisecond

	firstTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})
	secondTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})

	server.enqueueTask(firstTask, func(context.Context, string, service.ProgressReporter) error {
		close(firstStarted)
		<-releaseFirst
		return nil
	})

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not start")
	}

	server.enqueueTask(secondTask, func(context.Context, string, service.ProgressReporter) error {
		close(secondStarted)
		server.taskManager.CompleteTask(secondTask.ID, &TaskResult{Message: "second done"})
		return nil
	})

	assert.Equal(t, CancelTaskResultCancelled, server.taskManager.CancelTask(firstTask.ID))

	select {
	case <-secondStarted:
		t.Fatal("second task should remain blocked while detached task holds same target")
	case <-time.After(300 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case <-secondStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("second task did not start after detached same-target task exited")
	}
}

func TestServer_QueueCancelledTaskExitsWithinGraceWithoutDetach(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.downloadQueue.cancelGrace = 300 * time.Millisecond

	firstTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})
	secondTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})

	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})

	server.enqueueTask(firstTask, func(ctx context.Context, _ string, _ service.ProgressReporter) error {
		close(firstStarted)
		<-ctx.Done()
		return context.Canceled
	})

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not start")
	}

	server.enqueueTask(secondTask, func(context.Context, string, service.ProgressReporter) error {
		close(secondStarted)
		server.taskManager.CompleteTask(secondTask.ID, &TaskResult{Message: "second done"})
		return nil
	})

	assert.Equal(t, CancelTaskResultCancelled, server.taskManager.CancelTask(firstTask.ID))

	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("second task did not start after first task exited within grace")
	}

	server.downloadQueue.mu.Lock()
	detachedCount := len(server.downloadQueue.detached)
	server.downloadQueue.mu.Unlock()
	assert.Equal(t, 0, detachedCount)
}

func TestServer_QueueWildcardTargetBlocksSpecificTarget(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	firstTask := server.taskManager.CreateTask(TaskTypeListDownload, &ListDownloadTaskData{ListID: 1})
	secondTask := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})

	server.enqueueTask(firstTask, func(context.Context, string, service.ProgressReporter) error {
		close(firstStarted)
		<-releaseFirst
		server.taskManager.CompleteTask(firstTask.ID, &TaskResult{Message: "first done"})
		return nil
	})

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not start")
	}

	server.enqueueTask(secondTask, func(context.Context, string, service.ProgressReporter) error {
		close(secondStarted)
		server.taskManager.CompleteTask(secondTask.ID, &TaskResult{Message: "second done"})
		return nil
	})

	select {
	case <-secondStarted:
		t.Fatal("second task should be blocked by wildcard target")
	case <-time.After(200 * time.Millisecond):
	}

	close(releaseFirst)
	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("second task did not start after wildcard task finished")
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
	assert.NotEmpty(t, data["root_path"])
	assert.Equal(t, float64(5), data["max_download_routine"])
	assert.Equal(t, float64(100), data["max_file_name_len"])
}

func TestServer_PutRoutesForConfigFieldsAndCookies(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
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

func TestServer_UpdateConfigRawRejectsInvalidSemanticConfig(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	assert.NoError(t, err)
	defer db.Close()
	database.CreateTables(db)

	appRoot := t.TempDir()
	cfg := &config.Config{RootPath: appRoot}

	server := NewServer(resty.New(), []*resty.Client{}, db, cfg, appRoot, nil)
	defer server.taskManager.Close()
	handler := server.buildHandler()

	body := `{"content":"root_path: ` + strings.ReplaceAll(appRoot, `\`, `/`) + `\nproxy_url: ftp://127.0.0.1:21\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/raw", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid config")
	assert.Contains(t, rr.Body.String(), "invalid proxy_url")
}

func TestServer_UpdateConfigRawPersistsNormalizedConfig(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	assert.NoError(t, err)
	defer db.Close()
	database.CreateTables(db)

	appRoot := t.TempDir()
	cfg := &config.Config{RootPath: appRoot}

	server := NewServer(resty.New(), []*resty.Client{}, db, cfg, appRoot, nil)
	defer server.taskManager.Close()
	handler := server.buildHandler()

	rootPath := filepath.Join(appRoot, "downloads")
	body := `{"content":"root_path: ` + strings.ReplaceAll(filepath.ToSlash(rootPath), `\`, `/`) + `\nproxy_url: '  http://127.0.0.1:7897  '\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/raw", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	confPath := filepath.Join(appRoot, "conf.yaml")
	saved, err := config.ReadConf(confPath)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, rootPath, saved.RootPath)
	assert.Equal(t, "http://127.0.0.1:7897", saved.ProxyURL)

	data, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "proxy_url: http://127.0.0.1:7897")
	assert.NotContains(t, string(data), "  http://127.0.0.1:7897  ")
}

func TestServer_SaveCookiesFailsWhenExistingCookiesUnreadable(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
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

func TestServer_SaveCookiesKeepsOldValuesByOriginalIndex(t *testing.T) {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
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
	require.NoError(t, config.WriteAdditionalCookies(cookiesPath, []*config.Cookie{
		{AuthToken: "auth-0", Ct0: "ct0-0"},
		{AuthToken: "auth-1", Ct0: "ct0-1"},
		{AuthToken: "auth-2", Ct0: "ct0-2"},
	}))

	cookiesBody := `{"cookies":[{"index":0,"auth_token":"__KEEP_OLD__","ct0":"__KEEP_OLD__"},{"index":2,"auth_token":"__KEEP_OLD__","ct0":"ct0-new"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cookies", bytes.NewBufferString(cookiesBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	saved, err := config.ReadAdditionalCookies(cookiesPath)
	require.NoError(t, err)
	require.Len(t, saved, 2)
	assert.Equal(t, "auth-0", saved[0].AuthToken)
	assert.Equal(t, "ct0-0", saved[0].Ct0)
	assert.Equal(t, "auth-2", saved[1].AuthToken)
	assert.Equal(t, "ct0-new", saved[1].Ct0)
}

func TestHandleUsers_InvalidPath(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/", nil)
	rr := serveAPI(server, req)

	// 无效路径现在返回 404 而不是 400
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleUsers_UnknownAction(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/unknown", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleUserDownload_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := UserDownloadTaskData{
		ScreenName:    "testuser",
		AutoFollow:    true,
		FollowMembers: true,
		SkipProfile:   false,
		NoRetry:       false,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := serveAPI(server, req)

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
	assert.Equal(t, true, data["follow_members"])
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
	rr := serveAPI(server, req)

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
	rr := serveAPI(server, req)

	// 空 body 应该使用默认值
	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleUserProfile_WrongMethod(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/profile", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleUserProfile_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/profile", nil)
	rr := serveAPI(server, req)

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
	rr := serveAPI(server, req)

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
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleFollowingDownload_WrongMethod(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/following/download", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleFollowingDownload_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := FollowingDownloadTaskData{
		ScreenName:    "testuser",
		AutoFollow:    true,
		FollowMembers: true,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/following/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandleFollowingDownload_InvalidPath(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/testuser/following", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleLists_InvalidPath(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/", nil)
	rr := serveAPI(server, req)

	// 无效路径现在返回 404 而不是 400
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleLists_InvalidListID(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/invalid/download", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleLists_UnknownAction(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/123/unknown", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleListDownload_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := ListDownloadTaskData{
		ListID:        123,
		AutoFollow:    true,
		FollowMembers: true,
		SkipProfile:   false,
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/123/download", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["task_id"])
	assert.Equal(t, "123", data["list_id"])
}

func TestHandleListProfile_Success(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lists/123/profile", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["task_id"])
	assert.Equal(t, "123", data["list_id"])
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

func TestHandleJsonFileDownload_MultipartSuccess(t *testing.T) {
	appRoot := t.TempDir()
	server, db := setupTestServerWithAppRoot(t, appRoot)
	defer db.Close()

	req := newMultipartUploadRequest(t, "/api/v1/json/file/download", []multipartUploadFile{
		{name: "tweets.json", content: "{}"},
		{name: "notes.json", content: "{}"},
	}, true)
	rr := httptest.NewRecorder()

	server.handleJsonFileDownload(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(2), data["file_count"])
	assert.Equal(t, true, data["no_retry"])

	tasks := server.taskManager.GetAllTasks()
	if assert.Len(t, tasks, 1) {
		taskData, ok := tasks[0].Data.(*JsonFileDownloadTaskData)
		assert.True(t, ok)
		assert.Len(t, taskData.Paths, 2)
		assert.True(t, strings.HasPrefix(taskData.Paths[0], filepath.Join(appRoot, "uploads", "json")))
		assert.True(t, taskData.NoRetry)
	}
}

func TestHandleJsonFileDownload_MultipartEmpty(t *testing.T) {
	appRoot := t.TempDir()
	server, db := setupTestServerWithAppRoot(t, appRoot)
	defer db.Close()

	req := newMultipartUploadRequest(t, "/api/v1/json/file/download", nil, false)
	rr := httptest.NewRecorder()

	server.handleJsonFileDownload(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleJsonFileDownload_MultipartRejectsUnsupportedFile(t *testing.T) {
	appRoot := t.TempDir()
	server, db := setupTestServerWithAppRoot(t, appRoot)
	defer db.Close()

	req := newMultipartUploadRequest(t, "/api/v1/json/file/download", []multipartUploadFile{
		{name: "notes.txt", content: "not-json"},
	}, false)
	rr := httptest.NewRecorder()

	server.handleJsonFileDownload(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleJsonFileDownload_MultipartRejectsLargeFile(t *testing.T) {
	_, err := validateUploadFile(&multipart.FileHeader{
		Filename: "large.json",
		Size:     maxUploadFileSize + 1,
	})

	assert.Error(t, err)
}

func TestHandleJsonFileDownload_MultipartRejectsInvalidFileName(t *testing.T) {
	_, err := validateUploadFile(&multipart.FileHeader{
		Filename: `bad:name.json`,
		Size:     1,
	})

	assert.Error(t, err)
}

func TestHandleJsonFileDownload_MultipartAvoidsNameCollisions(t *testing.T) {
	appRoot := t.TempDir()
	server, db := setupTestServerWithAppRoot(t, appRoot)
	defer db.Close()

	req := newMultipartUploadRequest(t, "/api/v1/json/file/download", []multipartUploadFile{
		{name: "tweets-2.json", content: "{}"},
		{name: "tweets.json", content: "{}"},
		{name: "tweets.json", content: "{}"},
		{name: "TWEETS.json", content: "{}"},
	}, false)
	rr := httptest.NewRecorder()

	server.handleJsonFileDownload(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	tasks := server.taskManager.GetAllTasks()
	if assert.Len(t, tasks, 1) {
		taskData, ok := tasks[0].Data.(*JsonFileDownloadTaskData)
		assert.True(t, ok)
		assert.Len(t, taskData.Paths, 4)

		seen := map[string]bool{}
		for _, path := range taskData.Paths {
			name := strings.ToLower(filepath.Base(path))
			assert.False(t, seen[name], "duplicate upload file name: %s", name)
			seen[name] = true
		}
	}
}

func TestHandleJsonFileDownload_MultipartCreateDirFailureDoesNotRemoveParent(t *testing.T) {
	appRoot := t.TempDir()
	uploadParent := filepath.Join(appRoot, "uploads")
	assert.NoError(t, os.MkdirAll(uploadParent, 0o755))
	blockingPath := filepath.Join(uploadParent, "json")
	assert.NoError(t, os.WriteFile(blockingPath, []byte("not a directory"), 0o644))

	server, db := setupTestServerWithAppRoot(t, appRoot)
	defer db.Close()

	req := newMultipartUploadRequest(t, "/api/v1/json/file/download", []multipartUploadFile{
		{name: "tweets.json", content: "{}"},
	}, false)
	rr := httptest.NewRecorder()

	server.handleJsonFileDownload(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	_, err := os.Stat(blockingPath)
	assert.NoError(t, err)
}

func TestHandleJsonFolderDownload_MultipartSuccess(t *testing.T) {
	appRoot := t.TempDir()
	server, db := setupTestServerWithAppRoot(t, appRoot)
	defer db.Close()

	req := newMultipartUploadRequest(t, "/api/v1/json/folder/download", []multipartUploadFile{
		{name: "tweet-1.json", content: "{}"},
		{name: "tweet-2.json", content: "{}"},
	}, false)
	rr := httptest.NewRecorder()

	server.handleJsonFolderDownload(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(2), data["file_count"])
	assert.Equal(t, false, data["no_retry"])

	tasks := server.taskManager.GetAllTasks()
	if assert.Len(t, tasks, 1) {
		taskData, ok := tasks[0].Data.(*JsonFolderDownloadTaskData)
		assert.True(t, ok)
		assert.Equal(t, 1, len(taskData.Paths))
		assert.True(t, strings.HasPrefix(taskData.Paths[0], filepath.Join(appRoot, "uploads", "loongtweet")))
	}
}

func TestHandleJsonFolderDownload_MultipartRejectsTextFile(t *testing.T) {
	appRoot := t.TempDir()
	server, db := setupTestServerWithAppRoot(t, appRoot)
	defer db.Close()

	req := newMultipartUploadRequest(t, "/api/v1/json/folder/download", []multipartUploadFile{
		{name: "tweet.txt", content: "{}"},
	}, false)
	rr := httptest.NewRecorder()

	server.handleJsonFolderDownload(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
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
		Lists: []StringUint64{},
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
		Users:         []string{"user1", "user2"},
		Lists:         []StringUint64{},
		AutoFollow:    true,
		FollowMembers: true,
		SkipProfile:   false,
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
		Lists: []StringUint64{100, 200, 300},
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
		Users:         []string{"user1"},
		Lists:         []StringUint64{100},
		AutoFollow:    true,
		FollowMembers: true,
		SkipProfile:   true,
		NoRetry:       true,
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
	assert.Equal(t, true, data["follow_members"])
	assert.Equal(t, true, data["skip_profile"])
	assert.Equal(t, true, data["no_retry"])
}

func TestHandleUserDownload_InvalidJSONReturnsBadRequest(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/alice/download", strings.NewReader("{"))
	rr := httptest.NewRecorder()

	server.handleUserDownload(rr, req, "alice")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Len(t, server.taskManager.GetAllTasks(), 0)
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

func TestScheduledDownloadMixedCreatesBatchTaskAndCallsService(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	fakeService := &fakeDownloadService{batchCalls: make(chan batchDownloadCall, 1)}
	server.downloadService = fakeService

	taskID := server.scheduledDownload(scheduler.ScheduleEntry{
		Type:           scheduler.ScheduleTypeMixed,
		Users:          []string{"@user1"},
		Lists:          []string{"123", "456"},
		FollowingNames: []string{" @user2 "},
		AutoFollow:     true,
		FollowMembers:  true,
		SkipProfile:    true,
		NoRetry:        true,
	})
	assert.NotEmpty(t, taskID)

	task, ok := server.taskManager.GetTask(taskID)
	assert.True(t, ok)
	data := task.Data.(*BatchDownloadTaskData)
	assert.Equal(t, []string{"user1"}, data.Users)
	assert.Equal(t, []StringUint64{123, 456}, data.Lists)
	assert.Equal(t, []string{"user2"}, data.FollowingNames)
	assert.True(t, data.AutoFollow)
	assert.True(t, data.FollowMembers)
	assert.True(t, data.SkipProfile)
	assert.True(t, data.NoRetry)

	select {
	case call := <-fakeService.batchCalls:
		assert.Equal(t, taskID, call.taskID)
		assert.Equal(t, []string{"user1"}, call.users)
		assert.Equal(t, []uint64{123, 456}, call.listIDs)
		assert.Equal(t, []string{"user2"}, call.followingNames)
		assert.True(t, call.opts.AutoFollow)
		assert.True(t, call.opts.FollowMembers)
		assert.True(t, call.opts.SkipProfile)
		assert.True(t, call.opts.NoRetry)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for batch download call")
	}
}

func TestScheduledDownloadUser_NormalizesAndValidatesTarget(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	taskID := server.scheduledDownload(scheduler.ScheduleEntry{
		Type:   scheduler.ScheduleTypeUser,
		Target: " @Alice ",
	})
	assert.NotEmpty(t, taskID)

	task, ok := server.taskManager.GetTask(taskID)
	assert.True(t, ok)
	data, ok := task.Data.(*UserDownloadTaskData)
	assert.True(t, ok)
	assert.Equal(t, "Alice", data.ScreenName)

	invalidTaskID := server.scheduledDownload(scheduler.ScheduleEntry{
		Type:   scheduler.ScheduleTypeUser,
		Target: "坏名字",
	})
	assert.Empty(t, invalidTaskID)
}

func TestScheduledDownloadFollowing_NormalizesAndValidatesTarget(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	taskID := server.scheduledDownload(scheduler.ScheduleEntry{
		Type:   scheduler.ScheduleTypeFollowing,
		Target: " @Bob ",
	})
	assert.NotEmpty(t, taskID)

	task, ok := server.taskManager.GetTask(taskID)
	assert.True(t, ok)
	data, ok := task.Data.(*FollowingDownloadTaskData)
	assert.True(t, ok)
	assert.Equal(t, "Bob", data.ScreenName)

	invalidTaskID := server.scheduledDownload(scheduler.ScheduleEntry{
		Type:   scheduler.ScheduleTypeFollowing,
		Target: "??bad??",
	})
	assert.Empty(t, invalidTaskID)
}

func TestCleanupUploadDirAfterTask_CleansAfterTaskReturns(t *testing.T) {
	uploadDir := filepath.Join(t.TempDir(), "upload")
	require.NoError(t, os.MkdirAll(uploadDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(uploadDir, "a.json"), []byte("{}"), 0o644))

	taskStarted := make(chan struct{})
	runDone := make(chan struct{})
	unblock := make(chan struct{})
	wrapped := cleanupUploadDirAfterTask(uploadDir, func(ctx context.Context, taskID string, reporter service.ProgressReporter) error {
		close(taskStarted)
		<-unblock
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = wrapped(ctx, "task-1", nil)
		close(runDone)
	}()

	select {
	case <-taskStarted:
	case <-time.After(time.Second):
		t.Fatal("wrapped task did not start")
	}

	cancel()

	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(uploadDir); err != nil {
		t.Fatalf("upload dir should remain while task is still running, got: %v", err)
	}

	close(unblock)
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("wrapped task did not exit")
	}

	deadline := time.After(2 * time.Second)
	for {
		_, err := os.Stat(uploadDir)
		if os.IsNotExist(err) {
			break
		}
		select {
		case <-deadline:
			t.Fatal("upload dir was not cleaned after context cancel")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
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
	tasks, ok := data["tasks"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, tasks, 2)
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

	assert.Equal(t, http.StatusNotFound, rr.Code)
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

	assert.Equal(t, http.StatusConflict, rr.Code)
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

func TestServer_GracefulShutdownWaitsForDetachedTaskExit(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()
	server.downloadQueue.cancelGrace = 50 * time.Millisecond

	task := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "alice"})
	started := make(chan struct{})
	release := make(chan struct{})

	server.enqueueTask(task, func(context.Context, string, service.ProgressReporter) error {
		close(started)
		<-release
		return nil
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not start")
	}

	shutdownDone := make(chan struct{})
	go func() {
		server.GracefulShutdown("test")
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		t.Fatal("shutdown completed before detached task exited")
	case <-time.After(300 * time.Millisecond):
	}

	close(release)

	select {
	case <-shutdownDone:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not complete after detached task exited")
	}
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
		Main: &TaskMainResult{
			Downloaded: 95,
			Failed:     5,
		},
		Profile: &TaskProfileResult{
			Downloaded: 7,
			Failed:     1,
			Versioned:  10,
		},
		Message: "Done",
	}
	ok = server.taskManager.CompleteTask(task.ID, result)
	assert.True(t, ok)

	task, _ = server.taskManager.GetTask(task.ID)
	assert.NotNil(t, task.Result.Main)
	assert.Equal(t, 95, task.Result.Main.Downloaded)
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
	result := server.taskManager.CancelTask(task.ID)
	assert.Equal(t, CancelTaskResultCancelled, result)

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

func TestHandleTaskStats(t *testing.T) {
	t.Run("空统计", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/stats", nil)
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp APIResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)

		data, ok := resp.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(0), data["queued"])
		assert.Equal(t, float64(0), data["running"])
		assert.Equal(t, float64(0), data["completed"])
		assert.Equal(t, float64(0), data["failed"])
		assert.Equal(t, float64(0), data["cancelled"])
		assert.Equal(t, float64(0), data["total"])
	})

	t.Run("有任务统计", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		// 创建不同状态的任务
		server.taskManager.CreateTask(TaskTypeUserDownload, nil) // queued
		running := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
		server.taskManager.UpdateTaskStatus(running.ID, TaskStatusRunning)
		completed := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
		server.taskManager.UpdateTaskStatus(completed.ID, TaskStatusCompleted)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/stats", nil)
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp APIResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)

		data, ok := resp.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(1), data["queued"])
		assert.Equal(t, float64(1), data["running"])
		assert.Equal(t, float64(1), data["completed"])
		assert.Equal(t, float64(3), data["total"])
	})
}

func TestHandleCancelQueuedTasks(t *testing.T) {
	t.Run("取消排队任务", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		server.taskManager.CreateTask(TaskTypeUserDownload, nil)
		server.taskManager.CreateTask(TaskTypeListDownload, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/cancel-queued", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp APIResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)

		data, ok := resp.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(2), data["cancelled_count"])
	})

	t.Run("空队列", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/cancel-queued", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp APIResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)

		data, ok := resp.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(0), data["cancelled_count"])
	})
}

func TestHandleDeleteTask(t *testing.T) {
	t.Run("删除终态任务", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
		server.taskManager.UpdateTaskStatus(task.ID, TaskStatusCompleted)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+task.ID, nil)
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp APIResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("删除不存在任务", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/non_existent", nil)
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("删除运行中任务", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
		server.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+task.ID, nil)
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
	})
}

func TestHandleRetryTask(t *testing.T) {
	t.Run("重试失败任务", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		task := server.taskManager.CreateTask(TaskTypeUserDownload, &UserDownloadTaskData{ScreenName: "testuser"})
		server.taskManager.SetTaskError(task.ID, assert.AnError)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+task.ID+"/retry", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusAccepted, rr.Code)

		var resp APIResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Success)

		data, ok := resp.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, data["task_id"])
		assert.NotEqual(t, task.ID, data["task_id"])
		assert.Equal(t, task.ID, data["original_id"])
	})

	t.Run("重试不存在任务", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/non_existent/retry", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("重试运行中任务", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		task := server.taskManager.CreateTask(TaskTypeUserDownload, nil)
		server.taskManager.UpdateTaskStatus(task.ID, TaskStatusRunning)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+task.ID+"/retry", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("重试上传来源的JSON任务被拒绝", func(t *testing.T) {
		server, db := setupTestServer(t)
		defer db.Close()

		task := server.taskManager.CreateTask(TaskTypeJsonFileDownload, &JsonFileDownloadTaskData{
			Paths:      []string{"/tmp/uploaded.json"},
			FromUpload: true,
		})
		server.taskManager.SetTaskError(task.ID, assert.AnError)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+task.ID+"/retry", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rr := serveAPI(server, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
	})
}

func TestAPIVersionHeader(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"健康检查", http.MethodGet, "/api/v1/health"},
		{"任务列表", http.MethodGet, "/api/v1/tasks"},
		{"任务统计", http.MethodGet, "/api/v1/tasks/stats"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := serveAPI(server, req)

			assert.Equal(t, "v1", rr.Header().Get("API-Version"))
		})
	}
}

func TestHandleErrors(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/errors", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	// regular 和 json 字段都存在
	assert.NotNil(t, data["regular"])
	assert.NotNil(t, data["json"])
}

func TestHandleBatchMark(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	reqData := BatchMarkDownloadedTaskData{
		Users:          []string{"user1"},
		Lists:          []StringUint64{100},
		FollowingNames: []string{"user2"},
	}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/mark", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	tasks := server.taskManager.GetAllTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, TaskTypeMarkDownloaded, tasks[0].Type)
}

func TestHandleRetryAllFailed(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/errors/retry", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandleClearErrors(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/errors", nil)
	rr := serveAPI(server, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandleGetTheme_ReturnsDefault(t *testing.T) {
	server := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/theme", nil)
	rr := httptest.NewRecorder()
	server.handleGetTheme(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "web1", data["theme"])
}

func TestHandleSetTheme_SwitchesToWeb2(t *testing.T) {
	orig := getFrontendTheme()
	defer setFrontendTheme(orig)

	server := &Server{}
	body := `{"theme":"web2"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/theme", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleSetTheme(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "web2", data["theme"])
	assert.Equal(t, "web2", getFrontendTheme())
}

func TestHandleSetTheme_InvalidBody(t *testing.T) {
	server := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/theme", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleSetTheme(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.False(t, resp.Success)
}

func TestHandleSetTheme_InvalidTheme(t *testing.T) {
	orig := getFrontendTheme()
	defer setFrontendTheme(orig)

	server := &Server{}
	body := `{"theme":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/theme", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleSetTheme(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, orig, getFrontendTheme())
}

func TestHandleGetThemes_ReturnsList(t *testing.T) {
	// 重置 theme 避免后续测试干扰
	orig := getFrontendTheme()
	defer setFrontendTheme(orig)

	server := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/themes", nil)
	rr := httptest.NewRecorder()
	server.handleGetThemes(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)

	themes, ok := data["themes"].([]interface{})
	assert.True(t, ok)
	assert.Contains(t, themes, "web1")
	assert.Contains(t, themes, "web2")

	current, ok := data["current"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, current)
}
