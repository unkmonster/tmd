package downloading

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/entity"
	"github.com/unkmonster/tmd/internal/twitter"
)

// ==================== TweetInEntity 测试 ====================

func TestTweetInEntity_GetTweet(t *testing.T) {
	tweet := &twitter.Tweet{
		Id:   12345,
		Text: "test tweet",
	}

	t.Run("获取推文", func(t *testing.T) {
		tie := TweetInEntity{
			Tweet:  tweet,
			Entity: nil,
		}

		got := tie.GetTweet()
		assert.Equal(t, tweet, got)
	})

	t.Run("nil推文", func(t *testing.T) {
		tie := TweetInEntity{
			Tweet:  nil,
			Entity: nil,
		}

		got := tie.GetTweet()
		assert.Nil(t, got)
	})
}

func TestTweetInEntity_GetPath(t *testing.T) {
	t.Run("nil entity返回空字符串", func(t *testing.T) {
		tie := TweetInEntity{
			Tweet:  &twitter.Tweet{Id: 1},
			Entity: nil,
		}
		got := tie.GetPath()
		assert.Empty(t, got)
	})

	t.Run("非nil entity委托给Entity.Path()", func(t *testing.T) {
		// GetPath 在 Entity 非 nil 时调用 Entity.Path() 并返回其结果。
		// 完整构造 UserEntity 需要数据库连接，此处验证代码路径可达。
		// Entity.Path() 的具体行为由 entity 包测试覆盖。
		t.Skip("构造完整 UserEntity 需要数据库连接，跳过")
	})
}

// ==================== PackagedTweet 接口合规测试 ====================

func TestPackagedTweet_Interface(t *testing.T) {
	// 编译期验证 TweetInEntity 实现了 PackagedTweet 接口
	var _ PackagedTweet = TweetInEntity{}

	tweet := &twitter.Tweet{
		Id:   12345,
		Text: "test",
	}

	pt := TweetInEntity{
		Tweet:  tweet,
		Entity: nil,
	}

	assert.NotNil(t, pt.GetTweet())
	assert.Empty(t, pt.GetPath())
}

// ==================== MaxDownloadRoutine 测试 ====================

func TestNormalizeMaxDownloadRoutine_DefaultValue(t *testing.T) {
	got := (RuntimeOptions{}).normalizedMaxDownloadRoutine()
	assert.Equal(t, config.DefaultMaxDownloadRoutine(), got)
	assert.Greater(t, got, 0, "MaxDownloadRoutine should be greater than 0")
	assert.LessOrEqual(t, got, 100, "MaxDownloadRoutine should be <= 100")
}

func TestNormalizeMaxDownloadRoutine_CustomValue(t *testing.T) {
	assert.Equal(t, 7, (RuntimeOptions{MaxDownloadRoutine: 7}).normalizedMaxDownloadRoutine())
}

// ==================== workerConfig 测试 ====================

func TestWorkerConfig_Fields(t *testing.T) {
	t.Run("完整配置", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)

		var wg sync.WaitGroup
		cfg := &workerConfig{
			ctx:            ctx,
			wg:             &wg,
			cancel:         cancel,
			skipLoongTweet: true,
		}

		assert.NotNil(t, cfg.ctx)
		assert.NotNil(t, cfg.wg)
		assert.NotNil(t, cfg.cancel)
		assert.True(t, cfg.skipLoongTweet)
	})

	t.Run("跳过长推文为false", func(t *testing.T) {
		cfg := &workerConfig{
			skipLoongTweet: false,
		}
		assert.False(t, cfg.skipLoongTweet)
	})
}

// ==================== userInListEntity 测试 ====================

func TestUserInListEntity(t *testing.T) {
	user := &twitter.User{
		Id:         12345,
		Name:       "Test User",
		ScreenName: "testuser",
	}

	t.Run("带leid", func(t *testing.T) {
		uile := userInListEntity{
			user: user,
			leid: 42,
		}

		assert.Equal(t, user, uile.user)
		assert.Equal(t, 42, uile.leid)
	})

	t.Run("zero leid", func(t *testing.T) {
		uile := userInListEntity{
			user: user,
			leid: 0,
		}

		assert.Equal(t, user, uile.user)
		assert.Equal(t, 0, uile.leid)
	})

	t.Run("nil user", func(t *testing.T) {
		uile := userInListEntity{
			user: nil,
			leid: 1,
		}

		assert.Nil(t, uile.user)
		assert.Equal(t, 1, uile.leid)
	})
}

// ==================== 常量测试 ====================

func TestConstants(t *testing.T) {
	t.Run("速率限制常量", func(t *testing.T) {
		assert.Greater(t, userTweetRateLimit, 0, "userTweetRateLimit should be positive")
		assert.GreaterOrEqual(t, userTweetRateLimit, 100, "userTweetRateLimit seems too low")

		assert.Greater(t, userTweetMaxConcurrent, 0, "userTweetMaxConcurrent should be positive")
		assert.GreaterOrEqual(t, userTweetMaxConcurrent, 1, "userTweetMaxConcurrent should be at least 1")
	})

	t.Run("合理值范围", func(t *testing.T) {
		// 验证速率限制在合理范围内
		assert.LessOrEqual(t, userTweetRateLimit, 10000, "userTweetRateLimit seems too high")
		assert.LessOrEqual(t, userTweetMaxConcurrent, 100, "userTweetMaxConcurrent seems too high")
	})
}

// ==================== batchSyncState 测试 ====================

func TestBatchSyncState_UserCache(t *testing.T) {
	state := newBatchSyncState()
	userID := uint64(12345)

	_, ok := state.loadUser(userID)
	assert.False(t, ok)

	ent := &entity.UserEntity{}
	state.storeUser(userID, ent)

	got, ok := state.loadUser(userID)
	assert.True(t, ok)
	assert.Same(t, ent, got)
}

func TestBatchSyncState_ListUserDedup(t *testing.T) {
	state := newBatchSyncState()

	assert.True(t, state.markListUser(1, 100))
	assert.False(t, state.markListUser(1, 100))
	assert.True(t, state.markListUser(1, 101))
	assert.True(t, state.markListUser(2, 100))
}
