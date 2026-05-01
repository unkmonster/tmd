package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLogLinesTail_ReturnsLastNLines(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "tmd2.log")
	content := "line1\nline2\nline3\nline4\nline5\n"
	require.NoError(t, os.WriteFile(logPath, []byte(content), 0600))

	lines, err := readLogLinesTail(logPath, 3)
	require.NoError(t, err)
	assert.Equal(t, []string{"line3", "line4", "line5"}, lines)
}

func TestReadLogLinesTail_DropsPartialLeadingLine(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "tmd2.log")
	content := "first line\nsecond line\nthird line without newline"
	require.NoError(t, os.WriteFile(logPath, []byte(content), 0600))

	lines, err := readLogLinesTail(logPath, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"second line", "third line without newline"}, lines)
}

func TestLogFollower_ReadNewLinesKeepsPartialLineUntilComplete(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "tmd2.log")
	require.NoError(t, os.WriteFile(logPath, []byte("existing\n"), 0600))

	follower, err := newLogFollower(logPath)
	require.NoError(t, err)
	defer follower.Close()

	lines, err := follower.ReadNewLines()
	require.NoError(t, err)
	assert.Empty(t, lines)

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, err)

	_, err = file.WriteString("partial")
	require.NoError(t, err)

	lines, err = follower.ReadNewLines()
	require.NoError(t, err)
	assert.Empty(t, lines)

	_, err = file.WriteString(" line\nnext line\n")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	lines, err = follower.ReadNewLines()
	require.NoError(t, err)
	assert.Equal(t, []string{"partial line", "next line"}, lines)
}

func TestHandleGetLogs_FiltersAndPaginatesTail(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	appRoot := t.TempDir()
	server.appRootPath = appRoot

	logPath := filepath.Join(appRoot, "tmd2.log")
	content := "" +
		"time=1 level=info msg=drop\n" +
		"time=2 level=error msg=keep-a\n" +
		"time=3 level=error msg=ignore\n" +
		"time=4 level=error msg=keep-b\n" +
		"time=5 level=error msg=keep-c\n"
	require.NoError(t, os.WriteFile(logPath, []byte(content), 0600))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?level=error&q=keep&page=1&pageSize=2", nil)
	rr := httptest.NewRecorder()
	server.buildHandler().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Success bool         `json:"success"`
		Data    LogsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, []string{
		"time=2 level=error msg=keep-a",
		"time=4 level=error msg=keep-b",
	}, resp.Data.Logs)
	assert.Equal(t, 3, resp.Data.Total)
	assert.Equal(t, 1, resp.Data.Page)
	assert.Equal(t, 2, resp.Data.PageSize)
	assert.Equal(t, 2, resp.Data.TotalPages)
}

func TestHandleGetLogs_UsesLogPaginationDefaults(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	appRoot := t.TempDir()
	server.appRootPath = appRoot

	logPath := filepath.Join(appRoot, "tmd2.log")
	content := "line1\nline2\nline3\n"
	require.NoError(t, os.WriteFile(logPath, []byte(content), 0600))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil)
	rr := httptest.NewRecorder()
	server.buildHandler().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Success bool         `json:"success"`
		Data    LogsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.Data.Page)
	assert.Equal(t, 100, resp.Data.PageSize)
	assert.Equal(t, 1, resp.Data.TotalPages)
	assert.Len(t, resp.Data.Logs, 3)
}
