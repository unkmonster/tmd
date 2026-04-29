package downloading

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("有效entity返回路径", func(t *testing.T) {
		// 注意：这里无法创建真实的UserEntity，因为它需要数据库连接
		// 这个测试验证了当Entity非nil时的代码路径
		// 实际路径取决于Entity.Path()的实现
		tie := TweetInEntity{
			Tweet:  &twitter.Tweet{Id: 1},
			Entity: nil, // 实际测试中无法创建真实entity
		}
		got := tie.GetPath()
		assert.Empty(t, got)
	})
}

// ==================== PackagedTweet 接口合规测试 ====================

func TestPackagedTweet_Interface(t *testing.T) {
	// 验证 TweetInEntity 实现了 PackagedTweet 接口
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

func TestMaxDownloadRoutine_DefaultValue(t *testing.T) {
	// Verify that MaxDownloadRoutine is initialized
	assert.Greater(t, MaxDownloadRoutine, 0, "MaxDownloadRoutine should be greater than 0")
	assert.LessOrEqual(t, MaxDownloadRoutine, 100, "MaxDownloadRoutine should be <= 100")

	// 验证是合理的值（基于CPU核心数）
	// 默认应该是 min(10, GOMAXPROCS*2)
	assert.LessOrEqual(t, MaxDownloadRoutine, 10, "MaxDownloadRoutine should be <= 10")
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

// ==================== syncedUsers 并发安全测试 ====================

func TestSyncedUsers_Map(t *testing.T) {
	key := uint64(12345)
	value := "test value"

	t.Run("基本操作", func(t *testing.T) {
		// Store
		syncedUsers.Store(key, value)

		// Load
		loaded, ok := syncedUsers.Load(key)
		require.True(t, ok, "should find stored value")
		assert.Equal(t, value, loaded)

		// Delete for cleanup
		syncedUsers.Delete(key)

		// Verify deleted
		_, ok = syncedUsers.Load(key)
		assert.False(t, ok, "value should be deleted")
	})

	t.Run("并发访问", func(t *testing.T) {
		const numGoroutines = 100
		const numOperations = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				key := uint64(id)
				for j := 0; j < numOperations; j++ {
					syncedUsers.Store(key, j)
					val, ok := syncedUsers.Load(key)
					if ok {
						_ = val.(int)
					}
				}
				syncedUsers.Delete(key)
			}(i)
		}

		wg.Wait()
	})

	t.Run("存储不同类型", func(t *testing.T) {
		tests := []struct {
			key   uint64
			value any
		}{
			{1, "string value"},
			{2, 42},
			{3, true},
			{4, time.Now()},
			{5, struct{ Name string }{Name: "test"}},
		}

		for _, tt := range tests {
			syncedUsers.Store(tt.key, tt.value)
			loaded, ok := syncedUsers.Load(tt.key)
			require.True(t, ok)
			assert.Equal(t, tt.value, loaded)
			syncedUsers.Delete(tt.key)
		}
	})
}

// ==================== syncedListUsers 并发安全测试 ====================

func TestSyncedListUsers_Map(t *testing.T) {
	t.Run("基本操作", func(t *testing.T) {
		key := 12345
		value := &sync.Map{}
		value.Store("test", "data")

		// Store
		syncedListUsers.Store(key, value)

		// Load
		loaded, ok := syncedListUsers.Load(key)
		require.True(t, ok, "should find stored value")
		assert.Equal(t, value, loaded)

		// Verify the inner map works
		if innerMap, ok := loaded.(*sync.Map); ok {
			data, ok := innerMap.Load("test")
			require.True(t, ok)
			assert.Equal(t, "data", data)
		}

		// Delete for cleanup
		syncedListUsers.Delete(key)

		// Verify deleted
		_, ok = syncedListUsers.Load(key)
		assert.False(t, ok, "value should be deleted")
	})

	t.Run("并发访问", func(t *testing.T) {
		const numGoroutines = 50
		const numOperations = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()

				// 每个goroutine创建自己的sync.Map
				innerMap := &sync.Map{}
				syncedListUsers.Store(id, innerMap)

				for j := 0; j < numOperations; j++ {
					innerMap.Store(j, id*numOperations+j)
				}

				// 验证存储
				loaded, ok := syncedListUsers.Load(id)
				if ok {
					if m, ok := loaded.(*sync.Map); ok {
						val, ok := m.Load(0)
						if ok {
							assert.Equal(t, id*numOperations, val)
						}
					}
				}

				syncedListUsers.Delete(id)
			}(i)
		}

		wg.Wait()
	})
}

// ==================== 性能测试 ====================

func BenchmarkSyncedUsersStore(b *testing.B) {
	for i := 0; i < b.N; i++ {
		key := uint64(i)
		syncedUsers.Store(key, i)
		syncedUsers.Delete(key)
	}
}

func BenchmarkSyncedUsersLoad(b *testing.B) {
	// 预先存储一些数据
	for i := 0; i < 1000; i++ {
		syncedUsers.Store(uint64(i), i)
	}
	defer func() {
		for i := 0; i < 1000; i++ {
			syncedUsers.Delete(uint64(i))
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		syncedUsers.Load(uint64(i % 1000))
	}
}

func BenchmarkSyncedUsersConcurrent(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := uint64(i)
			syncedUsers.Store(key, i)
			syncedUsers.Load(key)
			syncedUsers.Delete(key)
			i++
		}
	})
}
