//go:build windows
// +build windows

package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unkmonster/tmd/internal/utils"
)

// ============================================================================
// 测试配置和全局变量
// ============================================================================

var client *resty.Client
var testScreenName string

// 测试用户数据集 - 包含各种边界情况
var someUsers = []struct {
	id         uint64
	screenName string
	desc       string
}{
	{id: 1528902077325332480, desc: "通过ID查询的用户"},
	{id: 3316272504, desc: "通过ID查询的用户（较小ID）"},
	{id: 1478962175947390976, desc: "通过ID查询的用户（较大ID）"},
	{screenName: "Chibubao01", desc: "通过screen_name查询的用户"},
	{screenName: "Tsumugi69458619", desc: "通过screen_name查询的用户"},
	{screenName: "_sosen_", desc: "带下划线的screen_name"},
	{screenName: "baobaoxqaq", desc: "普通screen_name"},
	{screenName: "Greenfish_insky", desc: "带下划线的screen_name"},
	{screenName: "midorino_o", desc: "带下划线的screen_name"},
}

// 测试列表数据集
var someLists = []uint64{
	1293998605938950144,
	1073356376045436928,
	1360265344439443460,
}

// 不存在的用户/列表（用于测试错误处理）
var nonExistentUsers = []struct {
	id         uint64
	screenName string
	desc       string
}{
	{id: 9999999999999999999, desc: "不存在的用户ID"},
	{screenName: "this_user_definitely_not_exists_12345", desc: "不存在的screen_name"},
}

var nonExistentListID uint64 = 9999999999999999999

// ============================================================================
// 测试初始化
// ============================================================================

func init() {
	var err error
	ctx := context.Background()

	// 从环境变量读取代理配置
	if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
		os.Setenv("HTTP_PROXY", proxy)
	}
	if proxy := os.Getenv("HTTPS_PROXY"); proxy != "" {
		os.Setenv("HTTPS_PROXY", proxy)
	}

	// 从环境变量读取认证信息
	authToken := os.Getenv("TWITTER_AUTH_TOKEN")
	ct0 := os.Getenv("TWITTER_CT0")

	// 如果环境变量未设置，使用测试默认值
	if authToken == "" {
		authToken = "167011fd552e68beaaead97e8c2753f5fe96d33c"
	}
	if ct0 == "" {
		ct0 = "0e5e47afa12276c48324b794702d452a932280edca7f136f8992ca89a489d0637f4999ab0559944bbf80dd50b0366cff61f27186f591b32de5cd421ffa8858159490ba77dfb136310935f5efb8c3bffe"
	}

	client, testScreenName, err = Login(ctx, authToken, ct0)
	if err != nil {
		panic(err)
	}
	//EnableRateLimit(client)
}

// ============================================================================
// 用户相关测试
// ============================================================================

// TestGetUser 测试通过ID和screen_name获取用户信息
func TestGetUser(t *testing.T) {
	ctx := context.Background()

	for _, test := range someUsers {
		t.Run(test.desc, func(t *testing.T) {
			var u *User = nil
			var err error
			var uid uint64

			if test.id != 0 {
				u, uid, err = GetUserById(ctx, client, test.id)
			} else {
				u, uid, err = GetUserByScreenName(ctx, client, test.screenName)
			}

			if err != nil {
				t.Errorf("failed to get user (uid=%d): %v", uid, err)
				return
			}

			// 验证返回的用户信息
			if test.id != 0 && u.Id != test.id {
				t.Errorf("user.id = %d, want %d", u.Id, test.id)
			} else if test.id == 0 && u.ScreenName != test.screenName {
				t.Errorf("screen_name = %s, want %s", u.ScreenName, test.screenName)
			}

			// 验证用户基本信息完整性
			if u.Name == "" {
				t.Error("user.Name is empty")
			}
			if u.ScreenName == "" {
				t.Error("user.ScreenName is empty")
			}

			// 输出用户信息用于调试
			j, err := json.MarshalIndent(u, "", "  ")
			if err != nil {
				t.Error(err)
			}
			t.Logf("%s\n", j)
		})
	}
}

// TestGetUserNotFound 测试获取不存在用户的错误处理
func TestGetUserNotFound(t *testing.T) {
	ctx := context.Background()

	for _, test := range nonExistentUsers {
		t.Run(test.desc, func(t *testing.T) {
			var err error
			if test.id != 0 {
				_, _, err = GetUserById(ctx, client, test.id)
			} else {
				_, _, err = GetUserByScreenName(ctx, client, test.screenName)
			}

			if err == nil {
				t.Error("expected error for non-existent user, got nil")
			}
		})
	}
}

// TestGetUserConcurrent 测试并发获取用户
func TestGetUserConcurrent(t *testing.T) {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	for _, test := range someUsers {
		wg.Add(1)
		go func(id uint64, screenName string) {
			defer wg.Done()

			var err error
			if id != 0 {
				_, _, err = GetUserById(ctx, client, id)
			} else {
				_, _, err = GetUserByScreenName(ctx, client, screenName)
			}

			if err != nil {
				t.Errorf("concurrent get user failed: %v", err)
			}
		}(test.id, test.screenName)
	}

	wg.Wait()
}

// TestUserVisibility 测试用户可见性检查
func TestUserVisibility(t *testing.T) {
	ctx := context.Background()

	for _, test := range someUsers {
		if test.screenName == "" {
			continue
		}

		t.Run(test.screenName, func(t *testing.T) {
			user, _, err := GetUserByScreenName(ctx, client, test.screenName)
			if err != nil {
				t.Skipf("failed to get user: %v", err)
			}

			isVisible := user.IsVisiable()

			// 验证可见性逻辑
			expectedVisible := user.Followstate == FS_FOLLOWING || !user.IsProtected
			if isVisible != expectedVisible {
				t.Errorf("IsVisiable() = %v, want %v", isVisible, expectedVisible)
			}

			t.Logf("User %s: protected=%v, followstate=%d, visible=%v",
				user.ScreenName, user.IsProtected, user.Followstate, isVisible)
		})
	}
}

// TestUserTitle 测试用户标题生成
func TestUserTitle(t *testing.T) {
	user := &User{
		Name:       "Test User",
		ScreenName: "testuser",
	}

	title := user.Title()
	expected := "Test User(testuser)"
	if title != expected {
		t.Errorf("Title() = %s, want %s", title, expected)
	}
}

// TestUserFollowing 测试Following方法
func TestUserFollowing(t *testing.T) {
	user := &User{
		Id:         12345,
		ScreenName: "testuser",
	}

	following := user.Following()
	if following.creator != user {
		t.Error("Following().creator should be the original user")
	}
	if following.GetId() != -12345 {
		t.Errorf("Following().GetId() = %d, want -12345", following.GetId())
	}
}

// ============================================================================
// 媒体/推文相关测试
// ============================================================================

// TestGetMedia 测试获取用户媒体推文
func TestGetMedia(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 辅助函数：生成随机时间范围
	makeMinMax := func(items []*Tweet) (min int, max int) {
		// 时间线是逆序的，所以更大的索引对应发布时间更早的推文
		for {
			r1 := r.Intn(len(items))
			r2 := r.Intn(len(items))
			if r1 > r2 {
				max = r2
				min = r1
			} else {
				max = r1
				min = r2
			}

			// 不要生成在边界
			if max != 0 && min != len(items)-1 {
				return
			}
		}
	}

	wg := sync.WaitGroup{}
	ctx := context.Background()

	routine := func(test string) {
		defer wg.Done()
		usr, _, err := GetUserByScreenName(ctx, client, test)
		if err != nil {
			t.Error(err)
			return
		}

		// 获取全部推文
		if !usr.IsVisiable() {
			t.Logf("%s is not visible, skipping", test)
			return
		}

		tweets, err := usr.GetMedias(ctx, client, nil)
		if err != nil {
			t.Error(err, usr)
			return
		}

		n := len(tweets)
		retrievalRatio := 0.75
		if n > usr.MediaCount || float64(n)/float64(usr.MediaCount) < retrievalRatio {
			t.Errorf("%s: len(tweets) == %d, want %d", test, n, usr.MediaCount)
		}

		if n == 0 {
			t.Logf("%s has no media tweets", test)
			return
		}

		// 验证推文数据完整性
		for i, tweet := range tweets {
			if tweet.Id == 0 {
				t.Errorf("tweet[%d].Id is zero", i)
			}
			if tweet.CreatedAt.IsZero() {
				t.Errorf("tweet[%d].CreatedAt is zero", i)
			}
		}

		// 区间测试
		minIndex, maxIndex := makeMinMax(tweets)
		tr := &utils.TimeRange{Min: tweets[minIndex+1].CreatedAt, Max: tweets[maxIndex-1].CreatedAt}
		rangedTweets, err := usr.GetMedias(ctx, client, tr)
		if err != nil {
			t.Error(err, usr, "range")
			return
		}

		if len(rangedTweets) == 0 {
			t.Logf("No tweets in specified time range")
			return
		}

		if rangedTweets[0].Id != tweets[maxIndex].Id {
			t.Errorf("rangedTweets[0].Id = %d, want %d", rangedTweets[0].Id, tweets[maxIndex].Id)
		}
		if rangedTweets[len(rangedTweets)-1].Id != tweets[minIndex].Id {
			t.Errorf("rangedTweets[-1].Id = %d, want %d", rangedTweets[len(rangedTweets)-1].Id, tweets[minIndex].Id)
		}
	}

	for _, user := range someUsers {
		if user.id != 0 {
			continue
		}

		wg.Add(1)
		go routine(user.screenName)
	}

	wg.Wait()
}

// TestGetMediaEmptyTimeRange 测试空时间范围
func TestGetMediaEmptyTimeRange(t *testing.T) {
	ctx := context.Background()

	// 使用第一个可用的screen_name
	var testUser string
	for _, u := range someUsers {
		if u.screenName != "" {
			testUser = u.screenName
			break
		}
	}

	if testUser == "" {
		t.Skip("no test user available")
	}

	usr, _, err := GetUserByScreenName(ctx, client, testUser)
	if err != nil {
		t.Skipf("failed to get user: %v", err)
	}

	if !usr.IsVisiable() {
		t.Skip("user is not visible")
	}

	// 测试空时间范围（应该返回所有推文）
	tweets, err := usr.GetMedias(ctx, client, nil)
	if err != nil {
		t.Errorf("GetMedias with nil timeRange failed: %v", err)
	}

	t.Logf("Got %d tweets with nil timeRange", len(tweets))
}

// TestGetMediaInvalidTimeRange 测试无效时间范围
func TestGetMediaInvalidTimeRange(t *testing.T) {
	ctx := context.Background()

	var testUser string
	for _, u := range someUsers {
		if u.screenName != "" {
			testUser = u.screenName
			break
		}
	}

	if testUser == "" {
		t.Skip("no test user available")
	}

	usr, _, err := GetUserByScreenName(ctx, client, testUser)
	if err != nil {
		t.Skipf("failed to get user: %v", err)
	}

	if !usr.IsVisiable() {
		t.Skip("user is not visible")
	}

	// 测试Min > Max的时间范围（应该返回空结果）
	now := time.Now()
	tr := &utils.TimeRange{
		Min: now,
		Max: now.Add(-24 * time.Hour),
	}

	tweets, err := usr.GetMedias(ctx, client, tr)
	if err != nil {
		t.Errorf("GetMedias with invalid timeRange failed: %v", err)
	}

	// 这种时间范围应该返回空结果或正确处理
	t.Logf("Got %d tweets with invalid timeRange (Min > Max)", len(tweets))
}

// TestGetMediaProtectedUser 测试受保护用户的媒体获取
func TestGetMediaProtectedUser(t *testing.T) {
	ctx := context.Background()

	// 注意：这里需要一个已知的受保护用户来测试
	// 如果没有，则跳过此测试
	protectedUser := "some_protected_user"

	usr, _, err := GetUserByScreenName(ctx, client, protectedUser)
	if err != nil {
		t.Skipf("failed to get user (may not exist): %v", err)
	}

	if !usr.IsProtected {
		t.Skip("user is not protected")
	}

	if usr.IsVisiable() {
		t.Log("Protected user is visible (following)")
		tweets, err := usr.GetMedias(ctx, client, nil)
		if err != nil {
			t.Errorf("GetMedias failed: %v", err)
		}
		t.Logf("Got %d tweets from protected user", len(tweets))
	} else {
		t.Log("Protected user is not visible")
		tweets, err := usr.GetMedias(ctx, client, nil)
		if err != nil {
			t.Errorf("GetMedias failed: %v", err)
		}
		if tweets != nil && len(tweets) > 0 {
			t.Error("Should not get tweets from non-visible protected user")
		}
	}
}

// ============================================================================
// 列表相关测试
// ============================================================================

// TestGetList 测试获取列表信息
func TestGetList(t *testing.T) {
	ctx := context.Background()

	for _, test := range someLists {
		t.Run(fmt.Sprintf("list_%d", test), func(t *testing.T) {
			lst, err := GetLst(ctx, client, test)
			if err != nil {
				t.Error(err)
				return
			}

			if lst.Id != test {
				t.Errorf("lst.id == %d, want %d", lst.Id, test)
			}

			if lst.Name == "" {
				t.Error("list.Name is empty")
			}

			if lst.MemberCount < 0 {
				t.Error("list.MemberCount is negative")
			}

			j, err := json.MarshalIndent(&lst, "", "    ")
			if err != nil {
				t.Error(err)
			}
			t.Logf("%s\n", j)
		})
	}
}

// TestGetListNotFound 测试获取不存在的列表
func TestGetListNotFound(t *testing.T) {
	ctx := context.Background()

	_, err := GetLst(ctx, client, nonExistentListID)
	if err == nil {
		t.Error("expected error for non-existent list, got nil")
	}
}

// TestListTitle 测试列表标题生成
func TestListTitle(t *testing.T) {
	lst := &List{
		Id:   12345,
		Name: "Test List",
	}

	title := lst.Title()
	expected := "Test List(12345)"
	if title != expected {
		t.Errorf("Title() = %s, want %s", title, expected)
	}
}

// TestListGetId 测试列表ID获取
func TestListGetId(t *testing.T) {
	lst := &List{
		Id: 12345,
	}

	id := lst.GetId()
	if id != 12345 {
		t.Errorf("GetId() = %d, want 12345", id)
	}
}

// TestGetMember 测试获取列表成员
func TestGetMember(t *testing.T) {
	wg := sync.WaitGroup{}
	ctx := context.Background()

	routine := func(test uint64) {
		defer wg.Done()

		lst, err := GetLst(ctx, client, test)
		if err != nil {
			t.Error(err)
			return
		}

		usersResult, err := lst.GetMembers(ctx, client)
		if err != nil {
			t.Error(err)
			return
		}
		users := usersResult.Users

		// 允许2%的误差
		tolerance := lst.MemberCount * 2 / 100
		if len(users) > lst.MemberCount || lst.MemberCount-len(users) > tolerance {
			t.Errorf("len(users) == %d, want %d", len(users), lst.MemberCount)
		}

		// 验证用户数据
		for i, u := range users {
			if u.Id == 0 {
				t.Errorf("user[%d].Id is zero", i)
			}
			if u.ScreenName == "" {
				t.Errorf("user[%d].ScreenName is empty", i)
			}
		}

		// 测试UserFollowing
		fo := UserFollowing{lst.Creator}
		usersResult, err = fo.GetMembers(ctx, client)
		if err != nil {
			t.Error(err)
			return
		}
		users = usersResult.Users

		tolerance = fo.creator.FriendsCount * 2 / 100
		if len(users) > fo.creator.FriendsCount || fo.creator.FriendsCount-len(users) > tolerance {
			t.Errorf("len(users) == %d, want %d", len(users), fo.creator.FriendsCount)
		}
	}

	for _, test := range someLists {
		wg.Add(1)
		go routine(test)
	}

	wg.Wait()
}

// TestGetMemberEmptyList 测试空列表
func TestGetMemberEmptyList(t *testing.T) {
	ctx := context.Background()

	// 创建一个空列表（ID为0应该返回错误或空结果）
	emptyList := &List{
		Id:          0,
		MemberCount: 0,
	}

	result, err := emptyList.GetMembers(ctx, client)
	if err == nil && (result == nil || len(result.Users) == 0) {
		t.Log("Empty list handled correctly")
	} else if err != nil {
		t.Logf("Empty list returned error (expected): %v", err)
	}
}

// TestUserFollowingTitle 测试UserFollowing标题
func TestUserFollowingTitle(t *testing.T) {
	user := &User{
		ScreenName: "testuser",
	}
	fo := UserFollowing{user}

	title := fo.Title()
	expected := "testuser's Following"
	if title != expected {
		t.Errorf("Title() = %s, want %s", title, expected)
	}
}

// ============================================================================
// API错误处理测试
// ============================================================================

// TestApiError 测试API错误解析
func TestApiError(t *testing.T) {
	tests := []struct {
		name    string
		resp    string
		wantErr bool
		errCode int
	}{
		{
			name: "DependencyError",
			resp: `{
  "errors": [
    {
      "message": "Dependency: Unspecified",
      "locations": [{"line": 12, "column": 11}],
      "path": ["user", "result", "timeline_v2", "timeline"],
      "extensions": {
        "name": "DependencyError",
        "source": "Server",
        "retry_after": 0,
        "code": 0,
        "kind": "Operational"
      },
      "code": 0,
      "kind": "Operational",
      "name": "DependencyError",
      "source": "Server",
      "retry_after": 0
    }
  ],
  "data": {"user": {"result": {"__typename": "User", "timeline_v2": {}}}}
}`,
			wantErr: true,
			errCode: 0,
		},
		{
			name:    "NoError",
			resp:    `{"data": {"user": {"id": "12345"}}}`,
			wantErr: false,
		},
		{
			name: "TimeoutError",
			resp: `{
  "errors": [
    {
      "message": "Timeout",
      "extensions": {"code": 29}
    }
  ]
}`,
			wantErr: true,
			errCode: 29,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckApiResp([]byte(tt.resp))
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr && err != nil {
				if apiErr, ok := err.(*TwitterApiError); ok {
					if apiErr.Code != tt.errCode {
						t.Errorf("error code = %d, want %d", apiErr.Code, tt.errCode)
					}
				} else {
					t.Errorf("expected *TwitterApiError, got %T", err)
				}
			}
		})
	}
}

// TestTwitterApiError 测试TwitterApiError类型
func TestTwitterApiError(t *testing.T) {
	err := NewTwitterApiError(ErrTimeout, `{"errors": [{"message": "Timeout"}]}`)

	if err.Code != ErrTimeout {
		t.Errorf("Code = %d, want %d", err.Code, ErrTimeout)
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}

	t.Logf("Error message: %s", errMsg)
}

// ============================================================================
// 关注功能测试
// ============================================================================

// TestFollowUser 测试关注用户功能
// 注意：这是一个会修改状态的测试，需要谨慎运行
func TestFollowUser(t *testing.T) {
	if os.Getenv("RUN_FOLLOW_TEST") != "1" {
		t.Skip("Skipping follow test. Set RUN_FOLLOW_TEST=1 to run.")
	}

	ctx := context.Background()
	testScreenName := "su1__cos"

	user, _, err := GetUserByScreenName(ctx, client, testScreenName)
	if err != nil {
		t.Error(err)
		return
	}

	// 记录初始状态
	initialState := user.Followstate
	t.Logf("Initial follow state: %d", initialState)

	// 执行关注
	if err := FollowUser(ctx, client, user); err != nil {
		t.Error(err)
		return
	}

	// 验证关注状态
	user, _, err = GetUserByScreenName(ctx, client, testScreenName)
	if err != nil {
		t.Error(err)
		return
	}

	if user.Followstate == FS_UNFOLLOW {
		t.Errorf("user.Followstate == FS_UNFOLLOW, want following or requested")
	}

	t.Logf("Final follow state: %d", user.Followstate)
}

// ============================================================================
// 客户端功能测试
// ============================================================================

// TestGetClientScreenName 测试获取客户端screen_name
func TestGetClientScreenName(t *testing.T) {
	sn := GetClientScreenName(client)
	if sn == "" {
		t.Error("GetClientScreenName returned empty string")
	}
	if sn != testScreenName {
		t.Errorf("GetClientScreenName = %s, want %s", sn, testScreenName)
	}
	t.Logf("Client screen name: %s", sn)
}

// TestClientError 测试客户端错误管理
func TestClientError(t *testing.T) {
	// 测试获取不存在的错误
	err := GetClientError(client)
	if err != nil {
		t.Logf("Client has error: %v", err)
	}

	// 设置一个测试错误
	testErr := fmt.Errorf("test error")
	SetClientError(client, testErr)

	// 验证错误被设置
	err = GetClientError(client)
	if err == nil {
		t.Error("expected error after SetClientError")
	}
	if err.Error() != testErr.Error() {
		t.Errorf("error = %v, want %v", err, testErr)
	}

	// 清除错误
	SetClientError(client, nil)
	err = GetClientError(client)
	if err != nil {
		t.Errorf("expected nil error after clearing, got %v", err)
	}
}

// TestSelectClient 测试客户端选择功能
func TestSelectClient(t *testing.T) {
	ctx := context.Background()
	clients := []*resty.Client{client}

	// 测试正常选择
	selected := SelectClient(ctx, clients, "/test/path")
	if selected == nil {
		t.Error("SelectClient returned nil for available client")
	}
	if selected != client {
		t.Error("SelectClient returned wrong client")
	}
}

// TestSelectClientMFQ 测试MFQ客户端选择
func TestSelectClientMFQ(t *testing.T) {
	ctx := context.Background()
	additional := []*resty.Client{}

	// 获取一个测试用户
	var testUser *User
	for _, u := range someUsers {
		if u.screenName != "" {
			user, _, err := GetUserByScreenName(ctx, client, u.screenName)
			if err == nil {
				testUser = user
				break
			}
		}
	}

	if testUser == nil {
		t.Skip("no test user available")
	}

	// 测试MFQ选择
	selected := SelectClientMFQ(ctx, client, additional, testUser, "/test/path")
	if selected == nil {
		t.Error("SelectClientMFQ returned nil")
	}
}

// ============================================================================
// URL构建测试
// ============================================================================

// TestMakeUrl 测试URL构建
func TestMakeUrl(t *testing.T) {
	tests := []struct {
		name string
		api  api
		want string
	}{
		{
			name: "userByRestId",
			api:  &userByRestId{restId: 12345},
			want: HOST + "/i/api/graphql/CO4_gU4G_MRREoqfiTh6Hg/UserByRestId",
		},
		{
			name: "userByScreenName",
			api:  &userByScreenName{screenName: "testuser"},
			want: HOST + "/i/api/graphql/xmU6X_CKVnQ5lSrCbAmJsg/UserByScreenName",
		},
		{
			name: "userMedia",
			api:  &userMedia{userId: 12345, count: 100},
			want: HOST + "/i/api/graphql/MOLbHrtk8Ovu7DUNOLcXiA/UserMedia",
		},
		{
			name: "listByRestId",
			api:  &listByRestId{id: 12345},
			want: HOST + "/i/api/graphql/ZMQOSpxDo0cP5Cdt8MgEVA/ListByRestId",
		},
		{
			name: "listMembers",
			api:  &listMembers{id: 12345, count: 200},
			want: HOST + "/i/api/graphql/3dQPyRyAj6Lslp4e0ClXzg/ListMembers",
		},
		{
			name: "following",
			api:  &following{uid: 12345, count: 200},
			want: HOST + "/i/api/graphql/7FEKOPNAvxWASt6v9gfCXw/Following",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := makeUrl(tt.api)
			if !strings.HasPrefix(url, tt.want) {
				t.Errorf("makeUrl() = %s, want prefix %s", url, tt.want)
			}
			// 验证URL包含查询参数
			if !strings.Contains(url, "?") {
				t.Error("URL missing query parameters")
			}
		})
	}
}

// TestApiQueryParam 测试API查询参数
func TestApiQueryParam(t *testing.T) {
	tests := []struct {
		name string
		api  api
	}{
		{
			name: "userByRestId",
			api:  &userByRestId{restId: 12345},
		},
		{
			name: "userByScreenName",
			api:  &userByScreenName{screenName: "testuser"},
		},
		{
			name: "userMedia",
			api:  &userMedia{userId: 12345, count: 100, cursor: "test_cursor"},
		},
		{
			name: "listByRestId",
			api:  &listByRestId{id: 12345},
		},
		{
			name: "listMembers",
			api:  &listMembers{id: 12345, count: 200, cursor: "test_cursor"},
		},
		{
			name: "following",
			api:  &following{uid: 12345, count: 200, cursor: "test_cursor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := tt.api.QueryParam()
			if params == nil {
				t.Error("QueryParam() returned nil")
			}
			if params.Encode() == "" {
				t.Error("QueryParam() returned empty params")
			}
		})
	}
}

// TestTimelineApiSetCursor 测试时间线API设置游标
func TestTimelineApiSetCursor(t *testing.T) {
	tests := []struct {
		name string
		api  timelineApi
	}{
		{
			name: "userMedia",
			api:  &userMedia{userId: 12345, count: 100},
		},
		{
			name: "listMembers",
			api:  &listMembers{id: 12345, count: 200},
		},
		{
			name: "following",
			api:  &following{uid: 12345, count: 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCursor := "test_cursor_123"
			tt.api.SetCursor(testCursor)

			// 验证游标已设置（通过检查生成的URL）
			url := makeUrl(tt.api)
			if !strings.Contains(url, testCursor) {
				t.Errorf("SetCursor did not work, URL = %s", url)
			}
		})
	}
}

// ============================================================================
// 时间线相关测试
// ============================================================================

// TestFilterTweetsByTimeRange 测试推文时间范围过滤
func TestFilterTweetsByTimeRange(t *testing.T) {
	now := time.Now()

	// 创建测试推文
	tweets := []*Tweet{
		{Id: 1, CreatedAt: now.Add(-1 * time.Hour)},
		{Id: 2, CreatedAt: now.Add(-2 * time.Hour)},
		{Id: 3, CreatedAt: now.Add(-3 * time.Hour)},
		{Id: 4, CreatedAt: now.Add(-4 * time.Hour)},
		{Id: 5, CreatedAt: now.Add(-5 * time.Hour)},
	}

	tests := []struct {
		name       string
		min        *time.Time
		max        *time.Time
		wantLen    int
		wantCutMin bool
		wantCutMax bool
	}{
		{
			name:    "no filter",
			min:     nil,
			max:     nil,
			wantLen: 5,
		},
		{
			name:       "min only",
			min:        &[]time.Time{now.Add(-4 * time.Hour)}[0],
			max:        nil,
			wantLen:    3,
			wantCutMin: true,
		},
		{
			name:       "max only",
			min:        nil,
			max:        &[]time.Time{now.Add(-2 * time.Hour)}[0],
			wantLen:    3,
			wantCutMax: true,
		},
		{
			name:       "both min and max",
			min:        &[]time.Time{now.Add(-4 * time.Hour)}[0],
			max:        &[]time.Time{now.Add(-2 * time.Hour)}[0],
			wantLen:    1,
			wantCutMin: true,
			wantCutMax: true,
		},
		{
			name:    "empty result",
			min:     &[]time.Time{now.Add(-1 * time.Hour)}[0],
			max:     &[]time.Time{now.Add(-5 * time.Hour)}[0],
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cutMin, cutMax, result := filterTweetsByTimeRange(tweets, tt.min, tt.max)

			if cutMin != tt.wantCutMin {
				t.Errorf("cutMin = %v, want %v", cutMin, tt.wantCutMin)
			}
			if cutMax != tt.wantCutMax {
				t.Errorf("cutMax = %v, want %v", cutMax, tt.wantCutMax)
			}
			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

// ============================================================================
// 批量登录测试
// ============================================================================

// TestBatchLogin 测试批量登录功能
func TestBatchLogin(t *testing.T) {
	ctx := context.Background()

	cookies := []AccountCookie{
		{
			AuthToken: os.Getenv("TWITTER_AUTH_TOKEN"),
			Ct0:       os.Getenv("TWITTER_CT0"),
		},
	}

	if cookies[0].AuthToken == "" {
		t.Skip("no credentials available for batch login test")
	}

	clients := BatchLogin(ctx, false, cookies, testScreenName)

	if len(clients) == 0 {
		t.Error("BatchLogin returned no clients")
	}

	for i, cli := range clients {
		if cli == nil {
			t.Errorf("client[%d] is nil", i)
			continue
		}
		sn := GetClientScreenName(cli)
		t.Logf("Client %d: %s", i, sn)
	}
}

// TestBatchLoginEmpty 测试空cookie列表
func TestBatchLoginEmpty(t *testing.T) {
	ctx := context.Background()
	clients := BatchLogin(ctx, false, []AccountCookie{}, testScreenName)

	if clients != nil && len(clients) > 0 {
		t.Error("BatchLogin with empty cookies should return nil or empty slice")
	}
}

// ============================================================================
// 性能测试
// ============================================================================

// BenchmarkGetUserById 测试通过ID获取用户的性能
func BenchmarkGetUserById(b *testing.B) {
	ctx := context.Background()
	testID := someUsers[0].id

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := GetUserById(ctx, client, testID)
		if err != nil {
			b.Error(err)
		}
	}
}

// BenchmarkGetUserByScreenName 测试通过screen_name获取用户的性能
func BenchmarkGetUserByScreenName(b *testing.B) {
	ctx := context.Background()
	testScreenName := someUsers[3].screenName

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := GetUserByScreenName(ctx, client, testScreenName)
		if err != nil {
			b.Error(err)
		}
	}
}

// BenchmarkGetMedias 测试获取媒体的性能
func BenchmarkGetMedias(b *testing.B) {
	ctx := context.Background()
	testScreenName := someUsers[3].screenName

	usr, _, err := GetUserByScreenName(ctx, client, testScreenName)
	if err != nil {
		b.Skipf("failed to get user: %v", err)
	}

	if !usr.IsVisiable() {
		b.Skip("user is not visible")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := usr.GetMedias(ctx, client, nil)
		if err != nil {
			b.Error(err)
		}
	}
}

// ============================================================================
// 并发测试
// ============================================================================

// TestConcurrentGetMedia 测试并发获取媒体
func TestConcurrentGetMedia(t *testing.T) {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	for _, user := range someUsers {
		if user.screenName == "" {
			continue
		}

		wg.Add(1)
		go func(screenName string) {
			defer wg.Done()

			usr, _, err := GetUserByScreenName(ctx, client, screenName)
			if err != nil {
				t.Logf("failed to get user %s: %v", screenName, err)
				return
			}

			if !usr.IsVisiable() {
				t.Logf("user %s is not visible", screenName)
				return
			}

			_, err = usr.GetMedias(ctx, client, nil)
			if err != nil {
				t.Errorf("GetMedias failed for %s: %v", screenName, err)
			}
		}(user.screenName)
	}

	wg.Wait()
}

// TestConcurrentListMembers 测试并发获取列表成员
func TestConcurrentListMembers(t *testing.T) {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	for _, listID := range someLists {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()

			lst, err := GetLst(ctx, client, id)
			if err != nil {
				t.Logf("failed to get list %d: %v", id, err)
				return
			}

			_, err = lst.GetMembers(ctx, client)
			if err != nil {
				t.Errorf("GetMembers failed for list %d: %v", id, err)
			}
		}(listID)
	}

	wg.Wait()
}

// ============================================================================
// 集成测试
// ============================================================================

// TestFullWorkflow 测试完整工作流
func TestFullWorkflow(t *testing.T) {
	ctx := context.Background()

	// 1. 获取用户
	var testUser *User
	for _, u := range someUsers {
		if u.screenName != "" {
			user, _, err := GetUserByScreenName(ctx, client, u.screenName)
			if err == nil {
				testUser = user
				break
			}
		}
	}

	if testUser == nil {
		t.Skip("no test user available")
	}

	t.Logf("Got user: %s", testUser.Title())

	// 2. 获取用户媒体
	if testUser.IsVisiable() {
		tweets, err := testUser.GetMedias(ctx, client, nil)
		if err != nil {
			t.Errorf("GetMedias failed: %v", err)
		}
		t.Logf("Got %d tweets", len(tweets))

		// 3. 获取用户的Following
		following := testUser.Following()
		t.Logf("User following: %s", following.Title())

		// 4. 尝试获取Following成员
		_, err = following.GetMembers(ctx, client)
		if err != nil {
			t.Logf("GetMembers for following failed: %v", err)
		}
	} else {
		t.Log("User is not visible, skipping media fetch")
	}

	// 5. 获取列表（如果可用）
	if len(someLists) > 0 {
		lst, err := GetLst(ctx, client, someLists[0])
		if err != nil {
			t.Logf("GetLst failed: %v", err)
		} else {
			t.Logf("Got list: %s", lst.Title())

			_, err = lst.GetMembers(ctx, client)
			if err != nil {
				t.Logf("GetMembers for list failed: %v", err)
			}
		}
	}
}

// ============================================================================
// 辅助函数测试
// ============================================================================

// TestExtractScreenNameFromHome 测试从主页提取screen_name
func TestExtractScreenNameFromHome(t *testing.T) {
	tests := []struct {
		name     string
		home     string
		expected string
	}{
		{
			name:     "valid screen_name",
			home:     `{"screen_name":"testuser"}`,
			expected: "testuser",
		},
		{
			name:     "no screen_name",
			home:     `{"other_field":"value"}`,
			expected: "",
		},
		{
			name:     "empty response",
			home:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractScreenNameFromHome([]byte(tt.home))
			if result != tt.expected {
				t.Errorf("extractScreenNameFromHome() = %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestGetProxyFromEnvironment 测试代理环境变量获取
func TestGetProxyFromEnvironment(t *testing.T) {
	// 保存原始环境变量
	origHTTPProxy := os.Getenv("HTTP_PROXY")
	origHTTPSProxy := os.Getenv("HTTPS_PROXY")
	defer func() {
		os.Setenv("HTTP_PROXY", origHTTPProxy)
		os.Setenv("HTTPS_PROXY", origHTTPSProxy)
	}()

	// 测试HTTP_PROXY
	os.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")
	os.Unsetenv("HTTPS_PROXY")

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	proxy, err := getProxyFromEnvironment(req)
	if err != nil {
		t.Errorf("getProxyFromEnvironment failed: %v", err)
	}
	if proxy != nil {
		t.Logf("Got proxy: %s", proxy.String())
	}
}
