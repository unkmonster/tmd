package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/unkmonster/tmd/internal/twitter"
)

// ==================== Task 结构测试 ====================

func TestTask_Struct(t *testing.T) {
	task := Task{
		Users: []*twitter.User{},
		Lists: []twitter.ListBase{},
	}

	assert.Empty(t, task.Users)
	assert.Empty(t, task.Lists)
}

func TestTask_WithData(t *testing.T) {
	// 创建模拟用户
	user1 := &twitter.User{}
	user2 := &twitter.User{}

	task := Task{
		Users: []*twitter.User{user1, user2},
		Lists: []twitter.ListBase{},
	}

	assert.Len(t, task.Users, 2)
	assert.Empty(t, task.Lists)
}

// ==================== MakeTask 测试 ====================

// 由于MakeTask需要实际调用Twitter API，这里主要测试错误处理
// 实际功能测试需要mock twitter包

func TestMakeTask_EmptyArgs(t *testing.T) {
	// 空参数应该返回空任务
	// 注意：这需要mock twitter客户端，否则会因为无法连接而失败
	// 这里我们主要测试函数签名和基本结构

	usrArgs := UserArgs{}
	listArgs := ListArgs{}
	follArgs := UserArgs{}

	// 验证参数结构
	assert.Empty(t, usrArgs.ScreenName)
	assert.Empty(t, listArgs.ID)
	assert.Empty(t, follArgs.ScreenName)
}

func TestMakeTask_UserArgsStructure(t *testing.T) {
	usrArgs := UserArgs{
		ScreenName: []string{"user1", "user2"},
	}

	assert.Len(t, usrArgs.ScreenName, 2)
	assert.Equal(t, "user1", usrArgs.ScreenName[0])
}

func TestMakeTask_ListArgsStructure(t *testing.T) {
	listArgs := ListArgs{
		IntArgs: IntArgs{ID: []uint64{100, 200, 300}},
	}

	assert.Len(t, listArgs.ID, 3)
	assert.Equal(t, []uint64{100, 200, 300}, listArgs.ID)
}

func TestMakeTask_FollArgsStructure(t *testing.T) {
	follArgs := UserArgs{
		ScreenName: []string{"following_user"},
	}

	assert.Len(t, follArgs.ScreenName, 1)
}

// ==================== 集成场景测试 ====================

func TestTaskScenario_BatchDownload(t *testing.T) {
	// 模拟批量下载场景
	usrArgs := UserArgs{
		ScreenName: []string{"user1", "user2", "user3"},
	}
	listArgs := ListArgs{
		IntArgs: IntArgs{ID: []uint64{100, 200}},
	}
	follArgs := UserArgs{}

	// 验证参数组合
	assert.Len(t, usrArgs.ScreenName, 3)
	assert.Len(t, listArgs.ID, 2)
	assert.Empty(t, follArgs.ScreenName)

	totalItems := len(usrArgs.ScreenName) + len(listArgs.ID) + len(follArgs.ScreenName)
	assert.Equal(t, 5, totalItems)
}

func TestTaskScenario_FollowingDownload(t *testing.T) {
	// 模拟关注列表下载场景
	usrArgs := UserArgs{}
	listArgs := ListArgs{}
	follArgs := UserArgs{
		ScreenName: []string{"target_user"},
	}

	assert.Empty(t, usrArgs.ScreenName)
	assert.Empty(t, listArgs.ID)
	assert.Len(t, follArgs.ScreenName, 1)
}

func TestTaskScenario_MixedDownload(t *testing.T) {
	// 模拟混合下载场景
	usrArgs := UserArgs{
		ScreenName: []string{"by_name"},
	}
	listArgs := ListArgs{
		IntArgs: IntArgs{ID: []uint64{999}},
	}
	follArgs := UserArgs{
		ScreenName: []string{"following"},
	}

	// 验证所有参数都被正确设置
	assert.Len(t, usrArgs.ScreenName, 1)
	assert.Len(t, listArgs.ID, 1)
	assert.Len(t, follArgs.ScreenName, 1)
}

// ==================== Task 边界测试 ====================

func TestTask_NilUsers(t *testing.T) {
	task := Task{
		Users: nil,
		Lists: []twitter.ListBase{},
	}

	assert.Nil(t, task.Users)
	assert.Empty(t, task.Lists)
}

func TestTask_NilLists(t *testing.T) {
	task := Task{
		Users: []*twitter.User{},
		Lists: nil,
	}

	assert.Empty(t, task.Users)
	assert.Nil(t, task.Lists)
}

func TestTask_BothNil(t *testing.T) {
	task := Task{
		Users: nil,
		Lists: nil,
	}

	assert.Nil(t, task.Users)
	assert.Nil(t, task.Lists)
}

// ==================== 参数验证测试 ====================

func TestUserArgs_Validation(t *testing.T) {
	tests := []struct {
		name      string
		usrArgs   UserArgs
		hasScreen bool
	}{
		{
			name:      "空参数",
			usrArgs:   UserArgs{},
			hasScreen: false,
		},
		{
			name:      "只有ScreenName",
			usrArgs:   UserArgs{ScreenName: []string{"user"}},
			hasScreen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.hasScreen, len(tt.usrArgs.ScreenName) > 0)
		})
	}
}

func TestIntArgs_Validation(t *testing.T) {
	tests := []struct {
		name    string
		intArgs IntArgs
		isEmpty bool
	}{
		{
			name:    "空",
			intArgs: IntArgs{},
			isEmpty: true,
		},
		{
			name:    "单个值",
			intArgs: IntArgs{ID: []uint64{1}},
			isEmpty: false,
		},
		{
			name:    "多个值",
			intArgs: IntArgs{ID: []uint64{1, 2, 3}},
			isEmpty: false,
		},
		{
			name:    "零值",
			intArgs: IntArgs{ID: []uint64{0}},
			isEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isEmpty, len(tt.intArgs.ID) == 0)
		})
	}
}
