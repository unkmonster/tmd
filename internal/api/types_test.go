package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUserDownloadTaskData(t *testing.T) {
	tests := []struct {
		name       string
		data       UserDownloadTaskData
		wantJSON   string
		wantScreen string
	}{
		{
			name: "完整数据",
			data: UserDownloadTaskData{
				ScreenName:    "testuser",
				AutoFollow:    true,
				FollowMembers: true,
				SkipProfile:   false,
				NoRetry:       true,
			},
			wantJSON:   `{"screen_name":"testuser","auto_follow":true,"follow_members":true,"skip_profile":false,"no_retry":true}`,
			wantScreen: "testuser",
		},
		{
			name: "默认值",
			data: UserDownloadTaskData{
				ScreenName: "user2",
			},
			wantJSON:   `{"screen_name":"user2","auto_follow":false,"follow_members":false,"skip_profile":false,"no_retry":false}`,
			wantScreen: "user2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.data)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(bytes))
			assert.Equal(t, tt.wantScreen, tt.data.ScreenName)
		})
	}
}

func TestListDownloadTaskData(t *testing.T) {
	tests := []struct {
		name     string
		data     ListDownloadTaskData
		wantJSON string
	}{
		{
			name: "完整数据",
			data: ListDownloadTaskData{
				ListID:        12345,
				AutoFollow:    true,
				FollowMembers: true,
				SkipProfile:   true,
				NoRetry:       false,
			},
			wantJSON: `{"list_id":"12345","auto_follow":true,"follow_members":true,"skip_profile":true,"no_retry":false}`,
		},
		{
			name:     "零值",
			data:     ListDownloadTaskData{},
			wantJSON: `{"list_id":"0","auto_follow":false,"follow_members":false,"skip_profile":false,"no_retry":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.data)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(bytes))
		})
	}
}

func TestStringUint64JSON(t *testing.T) {
	const bigListID = "2033436439346905439"

	value := StringUint64(2033436439346905439)
	bytes, err := json.Marshal(value)
	assert.NoError(t, err)
	assert.Equal(t, `"`+bigListID+`"`, string(bytes))

	var decoded StringUint64
	err = json.Unmarshal([]byte(`"`+bigListID+`"`), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, value, decoded)
	assert.Equal(t, uint64(2033436439346905439), decoded.Uint64())

	err = json.Unmarshal([]byte(`12345`), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, StringUint64(12345), decoded)

	assert.Error(t, json.Unmarshal([]byte(`123.0`), &decoded))
	assert.Error(t, json.Unmarshal([]byte(`1e3`), &decoded))
	assert.Error(t, json.Unmarshal([]byte(`"18446744073709551616"`), &decoded))
}

func TestFollowingDownloadTaskData(t *testing.T) {
	data := FollowingDownloadTaskData{
		ScreenName:    "following_user",
		AutoFollow:    true,
		FollowMembers: true,
		SkipProfile:   false,
		NoRetry:       true,
	}

	bytes, err := json.Marshal(data)
	assert.NoError(t, err)

	var decoded FollowingDownloadTaskData
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, data, decoded)
}

func TestProfileDownloadTaskData(t *testing.T) {
	data := ProfileDownloadTaskData{
		ScreenName: "profile_user",
	}

	bytes, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"screen_name":"profile_user"}`, string(bytes))
}

func TestMarkDownloadedTaskData(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		data    MarkDownloadedTaskData
		hasTime bool
	}{
		{
			name: "带时间戳",
			data: MarkDownloadedTaskData{
				ScreenName: "mark_user",
				Timestamp:  &now,
			},
			hasTime: true,
		},
		{
			name: "不带时间戳",
			data: MarkDownloadedTaskData{
				ScreenName: "mark_user2",
			},
			hasTime: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.data)
			assert.NoError(t, err)

			var decoded map[string]interface{}
			err = json.Unmarshal(bytes, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, tt.data.ScreenName, decoded["screen_name"])

			if tt.hasTime {
				assert.NotNil(t, decoded["timestamp"])
			}
		})
	}
}

func TestBatchDownloadTaskData(t *testing.T) {
	tests := []struct {
		name     string
		data     BatchDownloadTaskData
		wantJSON string
	}{
		{
			name: "用户和列表",
			data: BatchDownloadTaskData{
				Users:         []string{"user1", "user2"},
				Lists:         []StringUint64{100, 200},
				AutoFollow:    true,
				FollowMembers: true,
				SkipProfile:   false,
				NoRetry:       true,
			},
			wantJSON: `{"users":["user1","user2"],"lists":["100","200"],"following_names":null,"auto_follow":true,"follow_members":true,"skip_profile":false,"no_retry":true}`,
		},
		{
			name: "仅用户",
			data: BatchDownloadTaskData{
				Users: []string{"user3"},
			},
			wantJSON: `{"users":["user3"],"lists":null,"following_names":null,"auto_follow":false,"follow_members":false,"skip_profile":false,"no_retry":false}`,
		},
		{
			name: "仅列表",
			data: BatchDownloadTaskData{
				Lists: []StringUint64{300},
			},
			wantJSON: `{"users":null,"lists":["300"],"following_names":null,"auto_follow":false,"follow_members":false,"skip_profile":false,"no_retry":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.data)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(bytes))
		})
	}
}

func TestListProfileTaskData(t *testing.T) {
	data := ListProfileTaskData{
		ListID: 99999,
	}

	bytes, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"list_id":"99999"}`, string(bytes))
}

func TestNewSuccessResponse(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		wantJSON string
	}{
		{
			name:     "字符串数据",
			data:     "test data",
			wantJSON: `{"success":true,"data":"test data"}`,
		},
		{
			name:     "map数据",
			data:     map[string]int{"count": 10},
			wantJSON: `{"success":true,"data":{"count":10}}`,
		},
		{
			name:     "nil数据",
			data:     nil,
			wantJSON: `{"success":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewSuccessResponse(tt.data)
			assert.True(t, resp.Success)
			assert.Equal(t, tt.data, resp.Data)
			assert.Empty(t, resp.Error)

			bytes, err := json.Marshal(resp)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(bytes))
		})
	}
}

func TestNewErrorResponse(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		wantError string
	}{
		{
			name:      "普通错误",
			errMsg:    "something went wrong",
			wantError: "something went wrong",
		},
		{
			name:      "空错误",
			errMsg:    "",
			wantError: "",
		},
		{
			name:      "长错误信息",
			errMsg:    "this is a very long error message with details",
			wantError: "this is a very long error message with details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewErrorResponse(tt.errMsg)
			assert.False(t, resp.Success)
			assert.Nil(t, resp.Data)
			assert.Equal(t, tt.wantError, resp.Error)

			bytes, err := json.Marshal(resp)
			assert.NoError(t, err)
			assert.Contains(t, string(bytes), `"success":false`)
			assert.Contains(t, string(bytes), tt.wantError)
		})
	}
}

func TestHealthResponse(t *testing.T) {
	now := time.Now()
	resp := HealthResponse{
		Status:    "ok",
		Version:   "2.0.0",
		Timestamp: now,
	}

	bytes, err := json.Marshal(resp)
	assert.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "ok", decoded["status"])
	assert.Equal(t, "2.0.0", decoded["version"])
	assert.NotNil(t, decoded["timestamp"])
}

func TestTaskListResponse(t *testing.T) {
	resp := TaskListResponse{
		Tasks: []*Task{},
	}

	bytes, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"tasks":[]}`, string(bytes))

	// 测试带任务的情况
	task := &Task{
		ID:     "task_123",
		Type:   TaskTypeUserDownload,
		Status: TaskStatusQueued,
	}
	resp.Tasks = append(resp.Tasks, task)

	bytes, err = json.Marshal(resp)
	assert.NoError(t, err)
	assert.Contains(t, string(bytes), `"tasks":[`)
	assert.NotContains(t, string(bytes), `"total"`)
}

func TestDBUserItem(t *testing.T) {
	item := DBUserItem{
		ID:           "1",
		ScreenName:   "dbuser",
		Name:         "DB User",
		IsProtected:  true,
		FriendsCount: 100,
		IsAccessible: false,
	}

	bytes, err := json.Marshal(item)
	assert.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "1", decoded["id"])
	assert.Equal(t, "dbuser", decoded["screen_name"])
	assert.Equal(t, "DB User", decoded["name"])
	assert.Equal(t, true, decoded["protected"])
	assert.Equal(t, float64(100), decoded["friends_count"])
	assert.Equal(t, false, decoded["is_accessible"])
}

func TestDBListItem(t *testing.T) {
	item := DBListItem{
		ID:      "100",
		Name:    "Test List",
		OwnerID: "200",
	}

	bytes, err := json.Marshal(item)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"id":"100","name":"Test List","owner_user_id":"200"}`, string(bytes))
}

func TestDBEntityItem(t *testing.T) {
	item := DBEntityItem{
		ID:                "1",
		UserID:            "123",
		Name:              "entity_name",
		LatestReleaseTime: "2024-01-01 12:00:00",
		ParentDir:         "/data",
		MediaCount:        50,
	}

	bytes, err := json.Marshal(item)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"id":"1","user_id":"123","name":"entity_name","latest_release_time":"2024-01-01 12:00:00","parent_dir":"/data","media_count":50}`, string(bytes))
}

func TestDBListEntityItem(t *testing.T) {
	item := DBListEntityItem{
		ID:        "1",
		LstID:     "100",
		Name:      "list_entity",
		ParentDir: "/lists",
	}

	bytes, err := json.Marshal(item)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"id":"1","lst_id":"100","name":"list_entity","parent_dir":"/lists"}`, string(bytes))
}

func TestDBUserLinkItem(t *testing.T) {
	item := DBUserLinkItem{
		ID:                "1",
		UserID:            "123",
		Name:              "link_name",
		ParentLstEntityID: "456",
	}

	bytes, err := json.Marshal(item)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"id":"1","user_id":"123","name":"link_name","parent_lst_entity_id":"456"}`, string(bytes))
}

func TestDBUserPreviousNameItem(t *testing.T) {
	item := DBUserPreviousNameItem{
		ID:         "1",
		Uid:        "123",
		ScreenName: "old_name",
		Name:       "Old Name",
		RecordDate: "2024-01-01",
	}

	bytes, err := json.Marshal(item)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"id":"1","user_id":"123","screen_name":"old_name","name":"Old Name","record_date":"2024-01-01"}`, string(bytes))
}

func TestConfigResponse(t *testing.T) {
	resp := ConfigResponse{
		RootPath:           "/data",
		MaxDownloadRoutine: 5,
		MaxFileNameLen:     100,
	}

	bytes, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"root_path":"/data","max_download_routine":5,"max_file_name_len":100}`, string(bytes))
}

func TestAPIResponse_JSONMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		resp APIResponse
	}{
		{
			name: "成功响应",
			resp: APIResponse{
				Success: true,
				Data:    map[string]string{"message": "ok"},
			},
		},
		{
			name: "错误响应",
			resp: APIResponse{
				Success: false,
				Error:   "error message",
			},
		},
		{
			name: "空响应",
			resp: APIResponse{
				Success: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.resp)
			assert.NoError(t, err)

			var decoded APIResponse
			err = json.Unmarshal(bytes, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, tt.resp.Success, decoded.Success)
			assert.Equal(t, tt.resp.Error, decoded.Error)
		})
	}
}
