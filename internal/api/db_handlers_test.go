package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/unkmonster/tmd/internal/database"
)

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// 创建必要的表
	database.CreateTables(db)
	return db
}

// createTestServer 创建测试服务器
func createTestServer(t *testing.T) (*Server, *sqlx.DB) {
	db := setupTestDB(t)
	server := &Server{
		db:          db,
		taskManager: NewTaskManager(nil),
	}
	t.Cleanup(server.taskManager.Close)
	return server, db
}

func TestHandleDBUsers_Empty(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users", nil)
	rr := httptest.NewRecorder()

	server.handleDBUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandleDBUsers_WithData(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建测试用户
	user := &database.User{
		Id:           1,
		ScreenName:   "testuser",
		Name:         "Test User",
		IsProtected:  false,
		FriendsCount: 100,
		IsAccessible: true,
	}
	err := database.CreateUser(db, user)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users", nil)
	rr := httptest.NewRecorder()

	server.handleDBUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	// 验证响应数据
	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(1), data["total"])
	items, ok := data["data"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, items, 1)
	item, ok := items[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "testuser", item["screen_name"])
}

func TestHandleDBUsers_WithSearch(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建测试用户
	user := &database.User{
		Id:           1,
		ScreenName:   "searchuser",
		Name:         "Search User",
		IsProtected:  false,
		FriendsCount: 100,
		IsAccessible: true,
	}
	err := database.CreateUser(db, user)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users?q=search", nil)
	rr := httptest.NewRecorder()

	server.handleDBUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUsers_WithFilters(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建测试用户
	user := &database.User{
		Id:           1,
		ScreenName:   "testuser",
		Name:         "Test User",
		IsProtected:  true,
		FriendsCount: 100,
		IsAccessible: false,
	}
	err := database.CreateUser(db, user)
	assert.NoError(t, err)

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "accessible filter true",
			url:  "/api/v1/db/users?accessible=true",
		},
		{
			name: "accessible filter false",
			url:  "/api/v1/db/users?accessible=false",
		},
		{
			name: "protected filter true",
			url:  "/api/v1/db/users?protected=true",
		},
		{
			name: "protected filter false",
			url:  "/api/v1/db/users?protected=false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rr := httptest.NewRecorder()

			server.handleDBUsers(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestHandleDBUserDetail_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	server.handleDBUserDetail(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBUserDetail_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users/invalid", nil)
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	server.handleDBUserDetail(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBUserDetail_Success(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建测试用户
	user := &database.User{
		Id:           1,
		ScreenName:   "detailuser",
		Name:         "Detail User",
		IsProtected:  false,
		FriendsCount: 50,
		IsAccessible: true,
	}
	err := database.CreateUser(db, user)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	server.handleDBUserDetail(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp APIResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandleDBUserUpdate_Success(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建测试用户
	user := &database.User{
		Id:           1,
		ScreenName:   "updateuser",
		Name:         "Update User",
		IsProtected:  false,
		FriendsCount: 50,
		IsAccessible: true,
	}
	err := database.CreateUser(db, user)
	assert.NoError(t, err)

	updateData := map[string]interface{}{
		"name":          "Updated Name",
		"friends_count": 100,
		"protected":     true,
		"is_accessible": false,
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/users/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBUserUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserUpdate_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	updateData := map[string]string{
		"name": "Updated Name",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/users/999", bytes.NewReader(body))
	req.SetPathValue("id", "999")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBUserUpdate(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBUserUpdate_InvalidBody(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建测试用户
	user := &database.User{
		Id:         1,
		ScreenName: "testuser",
		Name:       "Test User",
	}
	database.CreateUser(db, user)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/users/1", bytes.NewReader([]byte("invalid json")))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBUserUpdate(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBUserDelete_Success(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建测试用户
	user := &database.User{
		Id:         1,
		ScreenName: "deleteuser",
		Name:       "Delete User",
	}
	database.CreateUser(db, user)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/users/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	server.handleDBUserDelete(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify user was deleted
	deletedUser, err := database.GetUserById(db, 1)
	assert.NoError(t, err)
	assert.Nil(t, deletedUser)
}

func TestHandleDBUserDelete_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/users/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	server.handleDBUserDelete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBLists_Empty(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists", nil)
	rr := httptest.NewRecorder()

	server.handleDBLists(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBLists_WithData(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建测试列表
	lst := &database.Lst{
		Id:          1,
		Name:        "Test List",
		OwnerUserId: 100,
	}
	err := database.CreateLst(db, lst)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists", nil)
	rr := httptest.NewRecorder()

	server.handleDBLists(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBLists_WithSearch(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	lst := &database.Lst{
		Id:          1,
		Name:        "Search List",
		OwnerUserId: 100,
	}
	database.CreateLst(db, lst)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists?q=Search", nil)
	rr := httptest.NewRecorder()

	server.handleDBLists(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBLists_WithOwnerFilter(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	lst := &database.Lst{
		Id:          1,
		Name:        "Test List",
		OwnerUserId: 100,
	}
	_ = database.CreateLst(db, lst)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists?ownerId=100", nil)
	rr := httptest.NewRecorder()

	server.handleDBLists(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBLists_WithInvalidOwnerFilter(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists?ownerId=invalid", nil)
	rr := httptest.NewRecorder()

	server.handleDBLists(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBListDetail_Success(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	lst := &database.Lst{
		Id:          1,
		Name:        "Detail List",
		OwnerUserId: 100,
	}
	database.CreateLst(db, lst)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	server.handleDBListDetail(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBListDetail_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	server.handleDBListDetail(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBListDetail_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists/invalid", nil)
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	server.handleDBListDetail(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBListUpdate_Success(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	lst := &database.Lst{
		Id:          1,
		Name:        "Old Name",
		OwnerUserId: 100,
	}
	database.CreateLst(db, lst)

	updateData := map[string]string{
		"name":          "New Name",
		"owner_user_id": "200",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/lists/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBListUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBListUpdate_InvalidOwnerID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	lst := &database.Lst{
		Id:          1,
		Name:        "Test List",
		OwnerUserId: 100,
	}
	database.CreateLst(db, lst)

	updateData := map[string]string{
		"owner_user_id": "invalid",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/lists/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBListUpdate(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBListDelete_Success(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	lst := &database.Lst{
		Id:          1,
		Name:        "Delete List",
		OwnerUserId: 100,
	}
	database.CreateLst(db, lst)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/lists/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	server.handleDBListDelete(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBListDelete_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/lists/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	server.handleDBListDelete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBUserEntities_Empty(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/user-entities", nil)
	rr := httptest.NewRecorder()

	server.handleDBUserEntities(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserEntities_WithFilters(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "search filter",
			url:  "/api/v1/db/user-entities?q=test",
		},
		{
			name: "userId filter",
			url:  "/api/v1/db/user-entities?userId=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rr := httptest.NewRecorder()

			server.handleDBUserEntities(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestHandleDBUserEntities_InvalidUserIDFilter(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/user-entities?userId=not-number", nil)
	rr := httptest.NewRecorder()

	server.handleDBUserEntities(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBUserEntityDetail_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/user-entities/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	server.handleDBUserEntityDetail(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBUserEntityDetail_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/user-entities/invalid", nil)
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	server.handleDBUserEntityDetail(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBUserEntityUpdate_InvalidBody(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/user-entities/1", bytes.NewReader([]byte("invalid")))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBUserEntityUpdate(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBUserEntityDelete_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/user-entities/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	server.handleDBUserEntityDelete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBListEntities_Empty(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/list-entities", nil)
	rr := httptest.NewRecorder()

	server.handleDBListEntities(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBListEntities_WithFilters(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "search filter",
			url:  "/api/v1/db/list-entities?q=test",
		},
		{
			name: "listId filter",
			url:  "/api/v1/db/list-entities?listId=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rr := httptest.NewRecorder()

			server.handleDBListEntities(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestHandleDBListEntities_InvalidListIDFilter(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/list-entities?listId=not-number", nil)
	rr := httptest.NewRecorder()

	server.handleDBListEntities(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBListEntityDetail_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/list-entities/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	server.handleDBListEntityDetail(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBListEntityUpdate_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	updateData := map[string]string{
		"name": "New Name",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/list-entities/999", bytes.NewReader(body))
	req.SetPathValue("id", "999")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBListEntityUpdate(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBListEntityDelete_NotFound(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/list-entities/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	server.handleDBListEntityDelete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleDBUserLinks_Empty(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/user-links", nil)
	rr := httptest.NewRecorder()

	server.handleDBUserLinks(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserLinks_WithFilters(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "userId filter",
			url:  "/api/v1/db/user-links?userId=1",
		},
		{
			name: "listEntityId filter",
			url:  "/api/v1/db/user-links?listEntityId=1",
		},
		{
			name: "both filters",
			url:  "/api/v1/db/user-links?userId=1&listEntityId=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rr := httptest.NewRecorder()

			server.handleDBUserLinks(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestHandleDBUserLinks_InvalidNumericFilters(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	tests := []string{
		"/api/v1/db/user-links?userId=not-number",
		"/api/v1/db/user-links?listEntityId=not-number",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rr := httptest.NewRecorder()

			server.handleDBUserLinks(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestHandleDBUserPreviousNames_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users/invalid/previous-names", nil)
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	server.handleDBUserPreviousNames(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBUserPreviousNames_Empty(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建用户
	user := &database.User{
		Id:         1,
		ScreenName: "testuser",
		Name:       "Test User",
	}
	database.CreateUser(db, user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users/1/previous-names", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	server.handleDBUserPreviousNames(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestNullInt32(t *testing.T) {
	tests := []struct {
		name     string
		input    sql.NullInt32
		expected int32
	}{
		{
			name:     "正常值",
			input:    sql.NullInt32{Int32: 42, Valid: true},
			expected: 42,
		},
		{
			name:     "零值",
			input:    sql.NullInt32{Int32: 0, Valid: true},
			expected: 0,
		},
		{
			name:     "无效值",
			input:    sql.NullInt32{Valid: false},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nullInt32(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortFieldsMaps(t *testing.T) {
	t.Run("userSortFields", func(t *testing.T) {
		assert.Contains(t, userSortFields, "id")
		assert.Contains(t, userSortFields, "screen_name")
		assert.Contains(t, userSortFields, "name")
		assert.Contains(t, userSortFields, "friends_count")
		assert.Contains(t, userSortFields, "is_accessible")
	})

	t.Run("listSortFields", func(t *testing.T) {
		assert.Contains(t, listSortFields, "id")
		assert.Contains(t, listSortFields, "name")
		assert.Contains(t, listSortFields, "owner_id")
	})

	t.Run("entitySortFields", func(t *testing.T) {
		assert.Contains(t, entitySortFields, "id")
		assert.Contains(t, entitySortFields, "user_id")
		assert.Contains(t, entitySortFields, "name")
		assert.Contains(t, entitySortFields, "media_count")
		assert.Contains(t, entitySortFields, "latest_release_time")
	})
}

func TestHandleDBUsers_Pagination(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建多个用户
	for i := 1; i <= 25; i++ {
		user := &database.User{
			Id:           uint64(i),
			ScreenName:   fmt.Sprintf("user%d", i),
			Name:         fmt.Sprintf("User %d", i),
			IsProtected:  false,
			FriendsCount: i * 10,
			IsAccessible: true,
		}
		database.CreateUser(db, user)
	}

	tests := []struct {
		name         string
		query        string
		expectedCode int
	}{
		{
			name:         "默认分页",
			query:        "",
			expectedCode: http.StatusOK,
		},
		{
			name:         "第二页",
			query:        "page=2",
			expectedCode: http.StatusOK,
		},
		{
			name:         "自定义页面大小",
			query:        "pageSize=10",
			expectedCode: http.StatusOK,
		},
		{
			name:         "排序",
			query:        "sortBy=name&sortOrder=asc",
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/db/users"
			if tt.query != "" {
				url = url + "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rr := httptest.NewRecorder()

			server.handleDBUsers(rr, req)

			assert.Equal(t, tt.expectedCode, rr.Code)
		})
	}
}

func TestHandleDBLists_Pagination(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/lists?page=1&pageSize=10&sortBy=name&sortOrder=asc", nil)
	rr := httptest.NewRecorder()

	server.handleDBLists(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserEntities_Pagination(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/user-entities?page=1&pageSize=10&sortBy=name&sortOrder=desc", nil)
	rr := httptest.NewRecorder()

	server.handleDBUserEntities(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBListEntities_Pagination(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/list-entities?page=1&pageSize=10", nil)
	rr := httptest.NewRecorder()

	server.handleDBListEntities(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserLinks_Pagination(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/user-links?page=1&pageSize=10", nil)
	rr := httptest.NewRecorder()

	server.handleDBUserLinks(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserPreviousNames_Pagination(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建用户
	user := &database.User{
		Id:         1,
		ScreenName: "testuser",
		Name:       "Test User",
	}
	database.CreateUser(db, user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/users/1/previous-names?page=1&pageSize=10", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	server.handleDBUserPreviousNames(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserUpdate_PartialFields(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建用户
	user := &database.User{
		Id:           1,
		ScreenName:   "testuser",
		Name:         "Original Name",
		IsProtected:  false,
		FriendsCount: 50,
		IsAccessible: true,
	}
	database.CreateUser(db, user)

	// 只更新 name
	updateData := map[string]string{
		"name": "New Name Only",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/users/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBUserUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserUpdate_AllowsExplicitEmptyStrings(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	user := &database.User{
		Id:           1,
		ScreenName:   "testuser",
		Name:         "Original Name",
		IsProtected:  false,
		FriendsCount: 50,
		IsAccessible: true,
	}
	database.CreateUser(db, user)

	updateData := map[string]string{
		"screen_name": "",
		"name":        "",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/users/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBUserUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	updated, err := database.GetUserById(db, 1)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, "", updated.ScreenName)
	assert.Equal(t, "", updated.Name)
}

func TestHandleDBListUpdate_PartialFields(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建列表
	lst := &database.Lst{
		Id:          1,
		Name:        "Original Name",
		OwnerUserId: 100,
	}
	database.CreateLst(db, lst)

	// 只更新 name
	updateData := map[string]string{
		"name": "New List Name",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/lists/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBListUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserEntityUpdate_PartialFields(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建用户
	user := &database.User{
		Id:         1,
		ScreenName: "testuser",
		Name:       "Test User",
	}
	database.CreateUser(db, user)

	// 创建实体
	entity := &database.UserEntity{
		UserId:    1,
		Name:      "Original Name",
		ParentDir: "/data",
	}
	database.CreateUserEntity(db, entity)

	// 只更新 name
	updateData := map[string]string{
		"name": "New Entity Name",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/user-entities/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBUserEntityUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBListEntityUpdate_PartialFields(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 创建列表
	lst := &database.Lst{
		Id:          1,
		Name:        "Test List",
		OwnerUserId: 100,
	}
	database.CreateLst(db, lst)

	// 创建列表实体
	entity := &database.LstEntity{
		LstId:     1,
		Name:      "Original Name",
		ParentDir: "/data",
	}
	database.CreateLstEntity(db, entity)

	// 只更新 name
	updateData := map[string]string{
		"name": "New List Entity Name",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/list-entities/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBListEntityUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserEntityUpdate_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	updateData := map[string]string{
		"name": "New Name",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/user-entities/invalid", bytes.NewReader(body))
	req.SetPathValue("id", "invalid")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBUserEntityUpdate(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBListEntityUpdate_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	updateData := map[string]string{
		"name": "New Name",
	}
	body, _ := json.Marshal(updateData)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/db/list-entities/invalid", bytes.NewReader(body))
	req.SetPathValue("id", "invalid")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.handleDBListEntityUpdate(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBListEntityDetail_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/list-entities/invalid", nil)
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	server.handleDBListEntityDetail(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBUserEntityDelete_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/user-entities/invalid", nil)
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	server.handleDBUserEntityDelete(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBListEntityDelete_InvalidID(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/db/list-entities/invalid", nil)
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	server.handleDBListEntityDelete(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDBUsers_URLQuery(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	// 测试 URL 查询参数解析
	u := &url.URL{
		Path:     "/api/v1/db/users",
		RawQuery: "q=test&accessible=true&protected=false&page=1&pageSize=20&sortBy=name&sortOrder=asc",
	}

	req := httptest.NewRequest(http.MethodGet, u.String(), nil)
	rr := httptest.NewRecorder()

	server.handleDBUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBLists_URLQuery(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	u := &url.URL{
		Path:     "/api/v1/db/lists",
		RawQuery: "q=test&ownerId=100&page=1&pageSize=20",
	}

	req := httptest.NewRequest(http.MethodGet, u.String(), nil)
	rr := httptest.NewRecorder()

	server.handleDBLists(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserEntities_URLQuery(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	u := &url.URL{
		Path:     "/api/v1/db/user-entities",
		RawQuery: "q=test&userId=1&page=1&pageSize=20",
	}

	req := httptest.NewRequest(http.MethodGet, u.String(), nil)
	rr := httptest.NewRecorder()

	server.handleDBUserEntities(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBListEntities_URLQuery(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	u := &url.URL{
		Path:     "/api/v1/db/list-entities",
		RawQuery: "q=test&listId=1&page=1&pageSize=20",
	}

	req := httptest.NewRequest(http.MethodGet, u.String(), nil)
	rr := httptest.NewRecorder()

	server.handleDBListEntities(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleDBUserLinks_URLQuery(t *testing.T) {
	server, db := createTestServer(t)
	defer db.Close()

	u := &url.URL{
		Path:     "/api/v1/db/user-links",
		RawQuery: "userId=1&listEntityId=1&page=1&pageSize=20",
	}

	req := httptest.NewRequest(http.MethodGet, u.String(), nil)
	rr := httptest.NewRecorder()

	server.handleDBUserLinks(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
