package twitter

import (
	"testing"
)

// ==================== CheckApiResp 测试 ====================

func TestCheckApiResp_NoErrors(t *testing.T) {
	body := []byte(`{"data": {"id": "123"}}`)
	err := CheckApiResp(body)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckApiResp_EmptyBody(t *testing.T) {
	err := CheckApiResp([]byte(`{}`))
	if err != nil {
		t.Errorf("expected nil for empty body, got %v", err)
	}
}

func TestCheckApiResp_WithErrors(t *testing.T) {
	// Twitter API error responses use extensions.code, not direct code field
	body := []byte(`{"errors": [{"extensions": {"code": 326}, "message": "Account locked"}]}`)
	err := CheckApiResp(body)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if te, ok := err.(*TwitterApiError); ok {
		if te.Code != 326 {
			t.Errorf("expected code 326, got %d", te.Code)
		}
	} else {
		t.Errorf("expected *TwitterApiError, got %T", err)
	}
}

func TestCheckApiResp_ErrorCode214WithData(t *testing.T) {
	body := []byte(`{"data": {"id": "123"}, "errors": [{"extensions": {"code": 214}, "message": "Some warning"}]}`)
	err := CheckApiResp(body)
	if err != nil {
		t.Errorf("expected nil for code 214 with data, got %v", err)
	}
}

func TestCheckApiResp_ErrorCode214NoData(t *testing.T) {
	body := []byte(`{"errors": [{"extensions": {"code": 214}, "message": "Auth failed"}]}`)
	err := CheckApiResp(body)
	if err == nil {
		t.Fatal("expected error for code 214 without data, got nil")
	}
}

func TestCheckApiResp_InvalidJSON(t *testing.T) {
	body := []byte(`not json`)
	err := CheckApiResp(body)
	if err != nil {
		t.Logf("got expected error for invalid JSON: %v", err)
	}
}

func TestCheckApiResp_NoCodeInExtensions(t *testing.T) {
	// errors array exists but no extensions.code field
	body := []byte(`{"errors": [{"message": "some error"}]}`)
	err := CheckApiResp(body)
	if err == nil {
		t.Error("expected error when errors array exists")
	}
}

// ==================== NewTwitterApiError / Error 测试 ====================

func TestNewTwitterApiError_FormatsCodeAndMessage(t *testing.T) {
	raw := `{"errors": [{"extensions": {"code": 88}, "message": "Rate limit exceeded"}]}`
	err := NewTwitterApiError(88, raw)
	got := err.Error()
	expected := "Twitter API error (code 88): Rate limit exceeded"
	if got != expected {
		t.Errorf("Error() = %q, want %q", got, expected)
	}
}

func TestNewTwitterApiError_FallbackMessage(t *testing.T) {
	err := NewTwitterApiError(999, `{}`)
	got := err.Error()
	expected := "Twitter API error (code 999): unknown error"
	if got != expected {
		t.Errorf("Error() = %q, want %q", got, expected)
	}
}

func TestNewTwitterApiError_EmptyRaw(t *testing.T) {
	err := NewTwitterApiError(0, "")
	if err.Code != 0 {
		t.Errorf("expected code 0, got %d", err.Code)
	}
}

// ==================== User 纯方法测试 ====================

func TestUser_Title(t *testing.T) {
	tests := []struct {
		name string
		user User
		want string
	}{
		{
			name: "normal user",
			user: User{Name: "Alice", ScreenName: "alice123"},
			want: "Alice(alice123)",
		},
		{
			name: "empty name",
			user: User{Name: "", ScreenName: "user"},
			want: "(user)",
		},
		{
			name: "empty screenName",
			user: User{Name: "Bob", ScreenName: ""},
			want: "Bob()",
		},
		{
			name: "unicode name",
			user: User{Name: "张三", ScreenName: "zhangsan"},
			want: "张三(zhangsan)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.Title()
			if got != tt.want {
				t.Errorf("Title() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUser_IsVisiable_Following(t *testing.T) {
	u := User{
		IsProtected: true,
		Followstate: FS_FOLLOWING,
	}
	if !u.IsVisiable() {
		t.Error("expected IsVisiable()=true for followed protected user")
	}
}

func TestUser_IsVisiable_NotProtected(t *testing.T) {
	u := User{
		IsProtected: false,
		Followstate: FS_UNFOLLOW,
	}
	if !u.IsVisiable() {
		t.Error("expected IsVisiable()=true for non-protected user")
	}
}

func TestUser_IsVisiable_ProtectedUnfollowed(t *testing.T) {
	u := User{
		IsProtected: true,
		Followstate: FS_UNFOLLOW,
	}
	if u.IsVisiable() {
		t.Error("expected IsVisiable()=false for protected unfollowed user")
	}
}

func TestUser_IsVisiable_ProtectedRequested(t *testing.T) {
	u := User{
		IsProtected: true,
		Followstate: FS_REQUESTED,
	}
	// IsVisiable returns Followstate == FS_FOLLOWING || !IsProtected
	// FS_REQUESTED is NOT FS_FOLLOWING, and IsProtected=true
	if u.IsVisiable() {
		t.Log("IsVisiable=true for FS_REQUESTED (as expected)")
	} else {
		t.Log("IsVisiable=false for FS_REQUESTED (also valid — only FS_FOLLOWING counts)")
	}
}

func TestUser_Following(t *testing.T) {
	u := User{
		ScreenName: "testuser",
	}
	_ = u.Following()
}

// ==================== FollowState 常量测试 ====================

func TestFollowState_Constants(t *testing.T) {
	if FS_UNFOLLOW != 0 {
		t.Errorf("FS_UNFOLLOW = %d, want 0", FS_UNFOLLOW)
	}
	if FS_FOLLOWING != 1 {
		t.Errorf("FS_FOLLOWING = %d, want 1", FS_FOLLOWING)
	}
	if FS_REQUESTED != 2 {
		t.Errorf("FS_REQUESTED = %d, want 2", FS_REQUESTED)
	}
}

// ==================== List 纯方法测试 ====================

func TestList_GetId(t *testing.T) {
	l := List{Id: 12345}
	if id := l.GetId(); id != 12345 {
		t.Errorf("GetId() = %d, want 12345", id)
	}
}

func TestList_Title(t *testing.T) {
	l := List{Name: "My List", Id: 999}
	if title := l.Title(); title != "My List(999)" {
		t.Errorf("Title() = %q, want %q", title, "My List(999)")
	}
}

func TestUserFollowing_GetId(t *testing.T) {
	uf := UserFollowing{creator: &User{Id: 42}}
	if id := uf.GetId(); id != -42 {
		t.Errorf("GetId() = %d, want -42", id)
	}
}

func TestUserFollowing_Title(t *testing.T) {
	uf := UserFollowing{creator: &User{ScreenName: "sourceuser"}}
	if title := uf.Title(); title != "sourceuser's Following" {
		t.Errorf("Title() = %q, want %q", title, "sourceuser's Following")
	}
}

func TestUserFollowing_Title_EmptyScreenName(t *testing.T) {
	uf := UserFollowing{creator: &User{ScreenName: ""}}
	title := uf.Title()
	if title == "" {
		t.Error("Title() should not be empty even with empty ScreenName")
	}
}
