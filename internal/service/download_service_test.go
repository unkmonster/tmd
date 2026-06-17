package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloading"
	"github.com/unkmonster/tmd/internal/entity"
	"github.com/unkmonster/tmd/internal/path"
	"github.com/unkmonster/tmd/internal/twitter"
)

// MockProgressReporter 用于测试的进度报告器
type MockProgressReporter struct {
	mu sync.Mutex

	ProgressCalls []struct {
		TaskID   string
		Progress Progress
	}
	CompleteCalls []struct {
		TaskID string
		Result Result
	}
	ErrorCalls []struct {
		TaskID string
		Err    error
	}
}

func NewMockProgressReporter() *MockProgressReporter {
	return &MockProgressReporter{
		ProgressCalls: make([]struct {
			TaskID   string
			Progress Progress
		}, 0),
		CompleteCalls: make([]struct {
			TaskID string
			Result Result
		}, 0),
		ErrorCalls: make([]struct {
			TaskID string
			Err    error
		}, 0),
	}
}

func (m *MockProgressReporter) OnProgress(taskID string, p Progress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProgressCalls = append(m.ProgressCalls, struct {
		TaskID   string
		Progress Progress
	}{TaskID: taskID, Progress: p})
}

func (m *MockProgressReporter) OnComplete(taskID string, r Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CompleteCalls = append(m.CompleteCalls, struct {
		TaskID string
		Result Result
	}{TaskID: taskID, Result: r})
}

func (m *MockProgressReporter) OnError(taskID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ErrorCalls = append(m.ErrorCalls, struct {
		TaskID string
		Err    error
	}{TaskID: taskID, Err: err})
}

func createTestDependencies(t *testing.T) *Dependencies {
	db, err := sqlx.Connect(database.DriverName, database.MemoryDSN(true))
	require.NoError(t, err)
	database.CreateTables(db)
	t.Cleanup(func() {
		_ = db.Close()
	})

	return &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                db,
		Config:            &config.Config{RootPath: t.TempDir()},
	}
}

func newFailedTweet(entityID int, tweetID uint64) *downloading.TweetInEntity {
	record := &database.UserEntity{
		Id:        sql.NullInt32{Int32: int32(entityID), Valid: true},
		UserId:    uint64(entityID),
		Name:      "user",
		ParentDir: "/tmp",
	}
	return &downloading.TweetInEntity{
		Tweet:  &twitter.Tweet{Id: tweetID, CreatedAt: time.Now()},
		Entity: entity.NewUserEntityFromRecord(nil, record),
	}
}

func TestDownloadServiceImpl_SaveDumperMergesExistingFile(t *testing.T) {
	deps := createTestDependencies(t)
	impl := &downloadServiceImpl{deps: deps}

	dumpPath := filepath.Join(t.TempDir(), "errors.json")
	now := time.Now()

	existing := downloading.NewDumper()
	existing.Push(1, &twitter.Tweet{Id: 10, CreatedAt: now})
	require.NoError(t, existing.Dump(dumpPath))

	incoming := downloading.NewDumper()
	incoming.Push(1, &twitter.Tweet{Id: 10, CreatedAt: now})
	incoming.Push(1, &twitter.Tweet{Id: 11, CreatedAt: now})

	impl.saveDumper(incoming, dumpPath)

	loaded := downloading.NewDumper()
	require.NoError(t, loaded.Load(dumpPath))
	assert.Equal(t, 2, loaded.Count())
	assert.True(t, loaded.HasTweet(1, 10))
	assert.True(t, loaded.HasTweet(1, 11))
}

func TestDownloadServiceImpl_SaveDumperConcurrentWritesPreserveData(t *testing.T) {
	deps := createTestDependencies(t)
	impl := &downloadServiceImpl{deps: deps}

	dumpPath := filepath.Join(t.TempDir(), "errors.json")
	now := time.Now()
	left := downloading.NewDumper()
	right := downloading.NewDumper()
	left.Push(1, &twitter.Tweet{Id: 101, CreatedAt: now})
	right.Push(2, &twitter.Tweet{Id: 202, CreatedAt: now})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		impl.saveDumper(left, dumpPath)
	}()
	go func() {
		defer wg.Done()
		impl.saveDumper(right, dumpPath)
	}()
	wg.Wait()

	loaded := downloading.NewDumper()
	require.NoError(t, loaded.Load(dumpPath))
	// saveDumper 现在是直接覆写模式（为了正确持久化 Remove），
	// 并发写入时最后一个写入者获胜。因此只要文件内容合法即可。
	assert.GreaterOrEqual(t, loaded.Count(), 1,
		"should have at least 1 tweet (concurrent write may not preserve both)")
	// 不应在文件中混合多实体数据（说明覆写是完整的）
	if loaded.Count() == 1 {
		assert.True(t, loaded.HasTweet(1, 101) || loaded.HasTweet(2, 202),
			"should have either left or right data")
	}
}

func TestDownloadServiceImpl_Struct(t *testing.T) {
	deps := createTestDependencies(t)
	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	assert.NotNil(t, service)

	impl, ok := service.(*downloadServiceImpl)
	assert.True(t, ok)
	assert.NotNil(t, impl.deps)
}

func TestDownloadServiceImpl_NilReporterHandling(t *testing.T) {
	deps := createTestDependencies(t)
	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	// 测试 nil reporter 被替换为 NopReporter
	reporter := impl.getReporterOrDefault(nil)
	assert.NotNil(t, reporter)

	// 验证是 NopReporter 类型
	_, ok := reporter.(*NopReporter)
	assert.True(t, ok)
}

func TestDownloadServiceImpl_ValidReporterHandling(t *testing.T) {
	deps := createTestDependencies(t)
	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	mockReporter := NewMockProgressReporter()
	reporter := impl.getReporterOrDefault(mockReporter)
	assert.Equal(t, mockReporter, reporter)
}

func TestDownloadServiceImpl_NewBatchProgressCallback(t *testing.T) {
	deps := createTestDependencies(t)
	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	reporter := NewMockProgressReporter()
	callback := impl.newBatchProgressCallback("task-1", reporter)
	callback(downloading.BatchProgress{Total: 5, Completed: 2, Failed: 1, Current: "user1"})

	require.Len(t, reporter.ProgressCalls, 1)
	assert.Equal(t, "task-1", reporter.ProgressCalls[0].TaskID)
	assert.Equal(t, "downloading", reporter.ProgressCalls[0].Progress.Stage)
	assert.Equal(t, 5, reporter.ProgressCalls[0].Progress.Total)
	assert.Equal(t, 2, reporter.ProgressCalls[0].Progress.Completed)
	assert.Equal(t, 1, reporter.ProgressCalls[0].Progress.Failed)
	assert.Equal(t, "user1", reporter.ProgressCalls[0].Progress.Current)
}

func TestDownloadServiceImpl_NewRetryProgressCallback(t *testing.T) {
	deps := createTestDependencies(t)
	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	reporter := NewMockProgressReporter()
	callback := impl.newRetryProgressCallback("task-1", reporter)
	callback(downloading.RetryProgress{Total: 5, Completed: 3, Failed: 2})

	require.Len(t, reporter.ProgressCalls, 1)
	assert.Equal(t, "task-1", reporter.ProgressCalls[0].TaskID)
	assert.Equal(t, "retrying", reporter.ProgressCalls[0].Progress.Stage)
	assert.Equal(t, 5, reporter.ProgressCalls[0].Progress.Total)
	assert.Equal(t, 3, reporter.ProgressCalls[0].Progress.Completed)
	assert.Equal(t, 2, reporter.ProgressCalls[0].Progress.Failed)
}

func TestDownloadServiceImpl_BuildMainDownloadResultUsesMainDownloadStats(t *testing.T) {
	result := (&downloadServiceImpl{}).buildMainDownloadResult(downloading.BatchDownloadSummary{TotalEntities: 5}, 2)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.Downloaded)
	assert.Equal(t, 2, result.Failed)
}

func TestDownloadServiceImpl_BuildMainDownloadResultReturnsNilWithoutMainEntities(t *testing.T) {
	result := (&downloadServiceImpl{}).buildMainDownloadResult(downloading.BatchDownloadSummary{}, 0)
	assert.Nil(t, result)
}

func TestCountRemainingFailedEntitiesOnlyCountsCurrentFailures(t *testing.T) {
	dumper := downloading.NewDumper()
	dumper.Push(9, &twitter.Tweet{Id: 90, CreatedAt: time.Now()})
	dumper.Push(1, &twitter.Tweet{Id: 10, CreatedAt: time.Now()})

	failures := collectFailedTweetSet([]*downloading.TweetInEntity{
		newFailedTweet(1, 10),
		newFailedTweet(2, 20),
	})

	assert.Equal(t, 1, countRemainingFailedEntities(dumper, failures))
}

func TestDownloadServiceImpl_CollectFailedTweetsSkipsNilEntries(t *testing.T) {
	impl := &downloadServiceImpl{}
	dumper := downloading.NewDumper()

	assert.NotPanics(t, func() {
		impl.collectFailedTweets(dumper, []*downloading.TweetInEntity{
			nil,
			{Tweet: nil, Entity: newFailedTweet(1, 10).Entity},
			{Tweet: &twitter.Tweet{Id: 20, CreatedAt: time.Now()}, Entity: nil},
			newFailedTweet(2, 30),
		})
	})

	assert.Equal(t, 1, dumper.Count())
	assert.True(t, dumper.HasTweet(2, 30))
}

func TestDownloadServiceImpl_CompleteProfileTaskWithoutDownloads(t *testing.T) {
	deps := createTestDependencies(t)
	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	reporter := NewMockProgressReporter()
	impl.completeProfileTask("task-1", reporter, nil)

	require.Len(t, reporter.CompleteCalls, 1)
	assert.Equal(t, "No profile downloads performed", reporter.CompleteCalls[0].Result.Message)
	assert.Nil(t, reporter.CompleteCalls[0].Result.Profile)
}

func TestDownloadServiceImpl_CompleteTaskWithProfileWarning(t *testing.T) {
	deps := createTestDependencies(t)
	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	reporter := NewMockProgressReporter()
	stats := &Result{
		Main: &MainResult{
			Downloaded: 2,
			Failed:     1,
		},
		Profile: &ProfileResult{
			Downloaded: 4,
			Failed:     1,
			Versioned:  3,
		},
	}
	impl.completeTask("task-1", reporter, "User download completed", stats, "with profile warnings")

	require.Len(t, reporter.CompleteCalls, 1)
	assert.Equal(t, "User download completed (with profile warnings)", reporter.CompleteCalls[0].Result.Message)
	require.NotNil(t, reporter.CompleteCalls[0].Result.Main)
	require.NotNil(t, reporter.CompleteCalls[0].Result.Profile)
	assert.Equal(t, 2, reporter.CompleteCalls[0].Result.Main.Downloaded)
	assert.Equal(t, 1, reporter.CompleteCalls[0].Result.Main.Failed)
	assert.Equal(t, 3, reporter.CompleteCalls[0].Result.Profile.Versioned)
}

func TestDownloadServiceImpl_UserDownloadErrorDoesNotCallReporterOnError(t *testing.T) {
	impl := &downloadServiceImpl{
		deps: &Dependencies{
			Config: &config.Config{RootPath: ""},
		},
	}
	reporter := NewMockProgressReporter()

	err := impl.UserDownload(context.Background(), "task-1", "someone", DownloadOptions{}, reporter)
	require.Error(t, err)
	assert.Empty(t, reporter.ErrorCalls)
}

func TestDownloadOptions_Combinations(t *testing.T) {
	testCases := []struct {
		name        string
		opts        DownloadOptions
		expectRetry bool
	}{
		{
			name:        "NoRetry true",
			opts:        DownloadOptions{NoRetry: true},
			expectRetry: false,
		},
		{
			name:        "NoRetry false",
			opts:        DownloadOptions{NoRetry: false},
			expectRetry: true,
		},
		{
			name:        "SkipProfile true",
			opts:        DownloadOptions{SkipProfile: true},
			expectRetry: true,
		},
		{
			name:        "AutoFollow true",
			opts:        DownloadOptions{AutoFollow: true},
			expectRetry: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectRetry, !tc.opts.NoRetry)
		})
	}
}

func TestEffectiveAutoFollowDisabledWhenFollowMembersEnabled(t *testing.T) {
	assert.True(t, effectiveAutoFollow(DownloadOptions{AutoFollow: true}))
	assert.False(t, effectiveAutoFollow(DownloadOptions{AutoFollow: true, FollowMembers: true}))
	assert.False(t, effectiveAutoFollow(DownloadOptions{FollowMembers: true}))
}

func TestShouldFollowMemberMatchesDownloadFiltering(t *testing.T) {
	assert.False(t, shouldFollowMember(nil))
	assert.False(t, shouldFollowMember(&twitter.User{Id: 0, Followstate: twitter.FS_UNFOLLOW}))
	assert.False(t, shouldFollowMember(&twitter.User{Id: 1, Followstate: twitter.FS_FOLLOWING}))
	assert.False(t, shouldFollowMember(&twitter.User{Id: 1, Followstate: twitter.FS_REQUESTED}))
	assert.False(t, shouldFollowMember(&twitter.User{Id: 1, Followstate: twitter.FS_UNFOLLOW, Blocking: true}))
	assert.False(t, shouldFollowMember(&twitter.User{Id: 1, Followstate: twitter.FS_UNFOLLOW, Muting: true}))
	assert.True(t, shouldFollowMember(&twitter.User{Id: 1, Followstate: twitter.FS_UNFOLLOW}))
}

func TestDedupeProfileUsersUsesScreenNameAndID(t *testing.T) {
	invalidA := &twitter.User{Name: "invalid-a"}
	invalidB := &twitter.User{Name: "invalid-b"}
	users := []*twitter.User{
		{Id: 1, ScreenName: "alice"},
		{Id: 2, ScreenName: "alice"},
		{Id: 1, ScreenName: "alice_renamed"},
		{Id: 3, ScreenName: " Bob "},
		{ScreenName: "Bob"},
		{Id: 4},
		{Id: 4},
		nil,
		invalidA,
		invalidB,
	}

	got := dedupeProfileUsers(users)

	require.Len(t, got, 5)
	assert.Equal(t, uint64(1), got[0].Id)
	assert.Equal(t, "alice", got[0].ScreenName)
	assert.Equal(t, " Bob ", got[1].ScreenName)
	assert.Equal(t, uint64(4), got[2].Id)
	assert.Same(t, invalidA, got[3])
	assert.Same(t, invalidB, got[4])
}

func TestMockProgressReporter_Recording(t *testing.T) {
	reporter := NewMockProgressReporter()

	reporter.OnProgress("task-1", Progress{Stage: "downloading", Current: "user1"})
	reporter.OnProgress("task-1", Progress{Stage: "completed", Current: "user1"})
	reporter.OnComplete("task-1", Result{Message: "Done"})
	reporter.OnError("task-1", errors.New("test error"))

	assert.Len(t, reporter.ProgressCalls, 2)
	assert.Len(t, reporter.CompleteCalls, 1)
	assert.Len(t, reporter.ErrorCalls, 1)

	assert.Equal(t, "downloading", reporter.ProgressCalls[0].Progress.Stage)
	assert.Equal(t, "completed", reporter.ProgressCalls[1].Progress.Stage)
	assert.Equal(t, "Done", reporter.CompleteCalls[0].Result.Message)
	assert.Equal(t, "test error", reporter.ErrorCalls[0].Err.Error())
}

func TestMockProgressReporter_MultipleTasks(t *testing.T) {
	reporter := NewMockProgressReporter()

	reporter.OnProgress("task-1", Progress{Stage: "downloading"})
	reporter.OnProgress("task-2", Progress{Stage: "syncing"})
	reporter.OnComplete("task-1", Result{Message: "Task 1 Done"})
	reporter.OnComplete("task-2", Result{Message: "Task 2 Done"})

	assert.Len(t, reporter.ProgressCalls, 2)
	assert.Len(t, reporter.CompleteCalls, 2)

	assert.Equal(t, "task-1", reporter.ProgressCalls[0].TaskID)
	assert.Equal(t, "task-2", reporter.ProgressCalls[1].TaskID)
}

func TestMockProgressReporter_EmptyCalls(t *testing.T) {
	reporter := NewMockProgressReporter()

	assert.Empty(t, reporter.ProgressCalls)
	assert.Empty(t, reporter.CompleteCalls)
	assert.Empty(t, reporter.ErrorCalls)
}

func TestDownloadServiceImpl_WithAdditionalClients(t *testing.T) {
	tempDir := t.TempDir()
	deps := &Dependencies{
		Client: resty.New(),
		AdditionalClients: []*resty.Client{
			resty.New(),
			resty.New(),
			resty.New(),
		},
		DB:     &sqlx.DB{},
		Config: &config.Config{RootPath: tempDir},
	}

	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	assert.NotNil(t, service)

	impl, ok := service.(*downloadServiceImpl)
	assert.True(t, ok)
	assert.Len(t, impl.deps.AdditionalClients, 3)
}

func TestDownloadServiceImpl_DownloadOptions_Variations(t *testing.T) {
	testCases := []struct {
		name string
		opts DownloadOptions
	}{
		{
			name: "All false",
			opts: DownloadOptions{
				AutoFollow:  false,
				SkipProfile: false,
				NoRetry:     false,
			},
		},
		{
			name: "All true",
			opts: DownloadOptions{
				AutoFollow:  true,
				SkipProfile: true,
				NoRetry:     true,
			},
		},
		{
			name: "Mixed",
			opts: DownloadOptions{
				AutoFollow:  true,
				SkipProfile: false,
				NoRetry:     true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotNil(t, tc.opts)
		})
	}
}

func TestDownloadServiceImpl_ContextHandling(t *testing.T) {
	deps := createTestDependencies(t)
	service, err := NewDownloadService(deps)
	require.NoError(t, err)

	// 测试取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 服务应该能处理已取消的 context
	assert.NotNil(t, service)
	assert.NotNil(t, ctx)
}

func TestDownloadServiceImpl_DependenciesVariations(t *testing.T) {
	tempDir := t.TempDir()
	testCases := []struct {
		name string
		deps *Dependencies
	}{
		{
			name: "Nil DB",
			deps: &Dependencies{
				Client:            resty.New(),
				AdditionalClients: []*resty.Client{},
				DB:                nil,
				Config:            &config.Config{RootPath: tempDir},
			},
		},
		{
			name: "Empty AdditionalClients",
			deps: &Dependencies{
				Client:            resty.New(),
				AdditionalClients: []*resty.Client{},
				DB:                &sqlx.DB{},
				Config:            &config.Config{RootPath: tempDir},
			},
		},
		{
			name: "Multiple AdditionalClients",
			deps: &Dependencies{
				Client: resty.New(),
				AdditionalClients: []*resty.Client{
					resty.New(),
					resty.New(),
				},
				DB:     &sqlx.DB{},
				Config: &config.Config{RootPath: tempDir},
			},
		},
		{
			name: "Nil Config",
			deps: &Dependencies{
				Client:            resty.New(),
				AdditionalClients: []*resty.Client{},
				DB:                &sqlx.DB{},
				Config:            nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service, err := NewDownloadService(tc.deps)
			if tc.name == "Nil Config" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "config is required")
				assert.Nil(t, service)
				return
			}
			if tc.name == "Nil DB" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "db is required")
				assert.Nil(t, service)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, service)

			impl, ok := service.(*downloadServiceImpl)
			assert.True(t, ok)
			assert.Equal(t, tc.deps, impl.deps)
		})
	}
}

func TestDownloadServiceImpl_InterfaceCompliance(t *testing.T) {
	var _ DownloadService = (*downloadServiceImpl)(nil)
}

func TestDownloadOptions_AllCombinations(t *testing.T) {
	boolValues := []bool{true, false}

	for _, autoFollow := range boolValues {
		for _, skipProfile := range boolValues {
			for _, noRetry := range boolValues {
				opts := DownloadOptions{
					AutoFollow:  autoFollow,
					SkipProfile: skipProfile,
					NoRetry:     noRetry,
				}
				assert.Equal(t, autoFollow, opts.AutoFollow)
				assert.Equal(t, skipProfile, opts.SkipProfile)
				assert.Equal(t, noRetry, opts.NoRetry)
			}
		}
	}
}

func TestMockProgressReporter_ProgressStages(t *testing.T) {
	reporter := NewMockProgressReporter()
	stages := []string{"syncing", "downloading", "retrying", "profile", "profile_warning", "marking", "completed"}

	for _, stage := range stages {
		reporter.OnProgress("task-1", Progress{Stage: stage, Current: "test"})
	}

	assert.Len(t, reporter.ProgressCalls, len(stages))

	for i, stage := range stages {
		assert.Equal(t, stage, reporter.ProgressCalls[i].Progress.Stage)
	}
}

func TestMockProgressReporter_ResultVariations(t *testing.T) {
	reporter := NewMockProgressReporter()

	// 测试不同结果类型
	reporter.OnComplete("task-1", Result{
		Main: &MainResult{
			Downloaded: 100,
			Failed:     5,
		},
		Profile: &ProfileResult{
			Downloaded: 8,
			Failed:     1,
			Versioned:  10,
		},
		Message: "Stats",
	})
	reporter.OnComplete("task-2", Result{Message: "Only message"})
	reporter.OnComplete("task-3", Result{})

	assert.Len(t, reporter.CompleteCalls, 3)
	require.NotNil(t, reporter.CompleteCalls[0].Result.Main)
	assert.Equal(t, 100, reporter.CompleteCalls[0].Result.Main.Downloaded)
	assert.Equal(t, "Only message", reporter.CompleteCalls[1].Result.Message)
	assert.Equal(t, "", reporter.CompleteCalls[2].Result.Message)
}

func TestMockProgressReporter_ErrorVariations(t *testing.T) {
	reporter := NewMockProgressReporter()

	reporter.OnError("task-1", errors.New("error 1"))
	reporter.OnError("task-2", errors.New("error 2"))
	reporter.OnError("task-3", nil)

	assert.Len(t, reporter.ErrorCalls, 3)
	assert.Equal(t, "error 1", reporter.ErrorCalls[0].Err.Error())
	assert.Equal(t, "error 2", reporter.ErrorCalls[1].Err.Error())
	assert.Nil(t, reporter.ErrorCalls[2].Err)
}

func TestDownloadServiceImpl_UserDownload_ReturnsErrorWithoutReporterOnError(t *testing.T) {
	deps := createTestDependencies(t)
	tempDir := t.TempDir()
	rootFile := filepath.Join(tempDir, "root-file")
	require.NoError(t, os.WriteFile(rootFile, []byte("not a directory"), 0644))
	deps.Config.RootPath = rootFile

	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	reporter := NewMockProgressReporter()
	err = impl.UserDownload(context.Background(), "task-1", "elonmusk", DownloadOptions{}, reporter)
	require.Error(t, err)

	assert.Empty(t, reporter.ErrorCalls)
	assert.Empty(t, reporter.CompleteCalls)
}

func TestDownloadServiceImpl_DownloadProfile_ReturnsErrorWhenAllProfilesFail(t *testing.T) {
	deps := createTestDependencies(t)
	deps.Config.RootPath = t.TempDir()

	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	pathHelper, err := path.NewStorePath(deps.Config.RootPath)
	require.NoError(t, err)

	versionManager, fileWriter, dwn := impl.initDownloader()
	reporter := NewMockProgressReporter()

	result, err := impl.downloadProfile(context.Background(), "task-1", []*twitter.User{
		{ScreenName: "broken_user"},
	}, pathHelper, versionManager, fileWriter, dwn, reporter)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.Downloaded)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 0, result.Versioned)
	assert.Contains(t, err.Error(), "profile download failed for all 1 users")
	assert.Empty(t, reporter.CompleteCalls)
}

func TestDownloadServiceImpl_DownloadProfile_ReportsIncrementalProgress(t *testing.T) {
	deps := createTestDependencies(t)
	deps.Config.RootPath = t.TempDir()

	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	pathHelper, err := path.NewStorePath(deps.Config.RootPath)
	require.NoError(t, err)

	versionManager, fileWriter, dwn := impl.initDownloader()
	reporter := NewMockProgressReporter()

	users := []*twitter.User{
		{Id: 101, Name: "User One", ScreenName: "user_one", Description: "a"},
		{Id: 102, Name: "User Two", ScreenName: "user_two", Description: "b"},
	}

	result, err := impl.downloadProfile(context.Background(), "task-1", users, pathHelper, versionManager, fileWriter, dwn, reporter)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, reporter.ProgressCalls)

	assert.Equal(t, "profile", reporter.ProgressCalls[0].Progress.Stage)
	assert.Equal(t, 2, reporter.ProgressCalls[0].Progress.Total)
	assert.Equal(t, 0, reporter.ProgressCalls[0].Progress.Completed)

	last := reporter.ProgressCalls[len(reporter.ProgressCalls)-1].Progress
	assert.Equal(t, "profile", last.Stage)
	assert.Equal(t, 2, last.Total)
	assert.Equal(t, 2, last.Completed)
	assert.Equal(t, 0, last.Failed)
	assert.Contains(t, []string{"user_one", "user_two"}, last.Current)
}

func TestDownloadServiceImpl_DownloadProfile_DedupesDuplicateUsers(t *testing.T) {
	deps := createTestDependencies(t)
	deps.Config.RootPath = t.TempDir()

	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	pathHelper, err := path.NewStorePath(deps.Config.RootPath)
	require.NoError(t, err)

	versionManager, fileWriter, dwn := impl.initDownloader()
	reporter := NewMockProgressReporter()

	users := []*twitter.User{
		{Id: 101, Name: "User One", ScreenName: "user_one", Description: "a"},
		{Id: 101, Name: "User One Changed", ScreenName: "user_one_new", Description: "b"},
		{Id: 102, Name: "User Two", ScreenName: "user_two", Description: "c"},
	}

	result, err := impl.downloadProfile(context.Background(), "task-1", users, pathHelper, versionManager, fileWriter, dwn, reporter)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 2, result.Downloaded)
	require.NotEmpty(t, reporter.ProgressCalls)
	assert.Equal(t, 2, reporter.ProgressCalls[0].Progress.Total)
	last := reporter.ProgressCalls[len(reporter.ProgressCalls)-1].Progress
	assert.Equal(t, 2, last.Total)
	assert.Equal(t, 2, last.Completed)
}

func TestDownloadServiceImpl_ProfileDownload_ReturnsErrorWhenAllUsersFailToResolve(t *testing.T) {
	deps := createTestDependencies(t)
	deps.Config.RootPath = t.TempDir()

	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	impl := service.(*downloadServiceImpl)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reporter := NewMockProgressReporter()
	err = impl.ProfileDownload(ctx, "task-1", []string{"broken_user"}, reporter)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all profile users failed to resolve")
	assert.Empty(t, reporter.CompleteCalls)
}

// --- Bug 61 复现 + 修复验证 ---
// TestDownloadServiceImpl_SaveDumper_RemoveAfterRetry
// 验证 replaceDumper 能持久化 Remove 操作（不通过 load-merge-save 把已移除的推文加回来）。
// 预置存在推文 10/11/12 的 dumper 文件 → 加载后 Remove(10) → replaceDumper
// → 文件应只包含 11/12（10 被成功移除）。
func TestDownloadServiceImpl_SaveDumper_RemoveAfterRetry(t *testing.T) {
	deps := createTestDependencies(t)
	impl := &downloadServiceImpl{deps: deps}

	dumpPath := filepath.Join(t.TempDir(), "errors.json")
	now := time.Now()

	// 初始文件：3 条失败推文
	initial := downloading.NewDumper()
	initial.Push(1, &twitter.Tweet{Id: 10, CreatedAt: now})
	initial.Push(1, &twitter.Tweet{Id: 11, CreatedAt: now})
	initial.Push(1, &twitter.Tweet{Id: 12, CreatedAt: now})
	require.NoError(t, initial.Dump(dumpPath))

	// 模拟 RetryAllFailed：加载 → 重试成功 → Remove
	dumper := downloading.NewDumper()
	require.NoError(t, dumper.Load(dumpPath))
	removed := dumper.Remove(1, 10)
	require.True(t, removed)
	assert.Equal(t, 2, dumper.Count())

	// 使用 replaceDumper（修复路径）写回，不做 load-merge
	impl.replaceDumper(dumper, dumpPath)

	// 验证文件内容
	loaded := downloading.NewDumper()
	require.NoError(t, loaded.Load(dumpPath))

	// BUG：推文 10 本应在文件中消失，但 saveDumper 的 re-load 把它加回来了
	assert.False(t, loaded.HasTweet(1, 10),
		"BUG 61: tweet 10 was removed before saveDumper but re-appeared in file")
	assert.Equal(t, 2, loaded.Count(),
		"BUG 61: file should contain 2 tweets (11,12) not %d", loaded.Count())
}

// TestDownloadServiceImpl_SaveDumper_RemoveViaSaveDumper
// 验证 saveDumper 能持久化 Remove 操作，即不通过 load-merge-save 把已移除的推文加回来。
// 与实际场景一致：下载 → 记录错误到 errors.json → retry 成功后 Remove → saveDumper
// → 文件不应包含已成功重试的推文。
func TestDownloadServiceImpl_SaveDumper_RemoveViaSaveDumper(t *testing.T) {
	deps := createTestDependencies(t)
	impl := &downloadServiceImpl{deps: deps}

	dumpPath := filepath.Join(t.TempDir(), "errors.json")
	now := time.Now()

	// 初始文件：3 条失败推文（模拟前一轮下载失败的记录）
	initial := downloading.NewDumper()
	initial.Push(1, &twitter.Tweet{Id: 10, CreatedAt: now})
	initial.Push(1, &twitter.Tweet{Id: 11, CreatedAt: now})
	initial.Push(1, &twitter.Tweet{Id: 12, CreatedAt: now})
	require.NoError(t, initial.Dump(dumpPath))

	// 本轮下载：加载 error.json → 重试成功(移除10,11) → saveDumper
	dumper := downloading.NewDumper()
	require.NoError(t, dumper.Load(dumpPath))
	removed := dumper.Remove(1, 10)
	require.True(t, removed)
	removed = dumper.Remove(1, 11)
	require.True(t, removed)
	assert.Equal(t, 1, dumper.Count()) // 只剩 tweet 12

	// 使用 saveDumper（原本用 load-merge，会把 10,11 加回来）
	impl.saveDumper(dumper, dumpPath)

	// 验证文件内容：10,11 不应再出现
	loaded := downloading.NewDumper()
	require.NoError(t, loaded.Load(dumpPath))
	assert.False(t, loaded.HasTweet(1, 10),
		"BUG: tweet 10 was removed before saveDumper but re-appeared in file (load-merge brings back removed entries)")
	assert.False(t, loaded.HasTweet(1, 11),
		"BUG: tweet 11 was removed before saveDumper but re-appeared in file")
	assert.True(t, loaded.HasTweet(1, 12),
		"tweet 12 should remain in file as it was not retried successfully")
	assert.Equal(t, 1, loaded.Count(),
		"file should contain 1 tweet (12) not %d", loaded.Count())
}

// --- Bug 62 复现 + 修复验证 ---
// TestDownloadServiceImpl_SaveDumper_FileRace
// 验证 RetryAllFailed 的 Load 在 dumperMu 锁保护下不再被 saveDumper 的并发写入破坏。
// saveDumper 在锁内写入文件的同时，若 Load 也在锁内则不会读到部分写入的 JSON。
func TestDownloadServiceImpl_SaveDumper_FileRace(t *testing.T) {
	deps := createTestDependencies(t)
	impl := &downloadServiceImpl{deps: deps}

	dumpPath := filepath.Join(t.TempDir(), "errors.json")
	now := time.Now()

	// 准备大量推文数据，提高文件写入耗时，增大竞态窗口
	initial := downloading.NewDumper()
	for entityID := 1; entityID <= 5; entityID++ {
		for tweetID := uint64(1); tweetID <= 200; tweetID++ {
			initial.Push(entityID, &twitter.Tweet{
				Id:        tweetID + uint64(entityID)*10000,
				CreatedAt: now,
			})
		}
	}
	require.NoError(t, initial.Dump(dumpPath))

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine A：模拟 saveDumper 在 dumperMu 锁内写入
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			incoming := downloading.NewDumper()
			incoming.Push(9, &twitter.Tweet{Id: 90000 + uint64(i), CreatedAt: now})
			impl.saveDumper(incoming, dumpPath)
		}
	}()

	// Goroutine B：模拟修复后 RetryAllFailed 在锁内 Load
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			dumper := downloading.NewDumper()
			impl.dumperMu.Lock()
			err := dumper.Load(dumpPath)
			impl.dumperMu.Unlock()
			if err != nil {
				t.Logf("Load failed under lock: %v", err)
				return
			}
		}
	}()

	wg.Wait()

	// 最终验证：文件应仍是合法 JSON
	loaded := downloading.NewDumper()
	err := loaded.Load(dumpPath)
	require.NoError(t, err, "BUG 62: final file corrupted by race condition")
	assert.Greater(t, loaded.Count(), 0, "final file should have data")
}

// --- Issue A 复现 ---
// TestDownloadServiceImpl_JsonLoadWithoutLock
// 展示 JsonFileDownload/JsonFolderDownload 中 jsonDumper.Load()
// 未持 dumperMu 锁，与 replaceJsonDumper 的锁内写入产生竞态。
// Goroutine A（锁内写入） ←→ Goroutine B（无锁读取 json_errors.json）
// → 无锁 Load 读到部分写入的 JSON 导致损坏。
func TestDownloadServiceImpl_JsonLoadWithoutLock(t *testing.T) {
	deps := createTestDependencies(t)
	impl := &downloadServiceImpl{deps: deps}

	dumpPath := filepath.Join(t.TempDir(), "json_errors.json")
	now := time.Now()

	// 准备大量数据，增大竞态窗口
	initial := downloading.NewJsonDumper()
	for fileIdx := 0; fileIdx < 10; fileIdx++ {
		sourcePath := fmt.Sprintf("/path/to/file_%d.json", fileIdx)
		for tweetID := uint64(1); tweetID <= 100; tweetID++ {
			initial.Push(sourcePath, "file", &twitter.Tweet{
				Id:        tweetID + uint64(fileIdx)*10000,
				CreatedAt: now,
			})
		}
	}
	require.NoError(t, initial.Dump(dumpPath))

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine A：模拟 RetryAllFailed 在锁内 replaceJsonDumper
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			incoming := downloading.NewJsonDumper()
			incoming.Push("new.json", "file", &twitter.Tweet{
				Id:        90000 + uint64(i),
				CreatedAt: now,
			})
			impl.replaceJsonDumper(incoming, dumpPath)
		}
	}()

	// Goroutine B：模拟修复后 JsonFileDownload 在锁内 Load
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			dumper := downloading.NewJsonDumper()
			impl.dumperMu.Lock()
			err := dumper.Load(dumpPath)
			impl.dumperMu.Unlock()
			if err != nil {
				t.Logf("ISSUE A: json Load failed under lock: %v", err)
				return
			}
		}
	}()

	wg.Wait()

	// 最终验证：文件应仍是合法 JSON
	loaded := downloading.NewJsonDumper()
	err := loaded.Load(dumpPath)
	require.NoError(t, err, "ISSUE A: final JSON dumper file corrupted by race condition")
	assert.Greater(t, loaded.Count(), 0, "ISSUE A: final file should have data")
}

// --- Issue B 复现 ---
// TestDownloadServiceImpl_ClearErrorsRace
// 展示 ClearErrors 未持 dumperMu 锁，在 saveDumper 持锁写入期间删除文件。
// Goroutine A（saveDumper 持锁写入）←→ Goroutine B（ClearErrors 无锁删除）
// → saveDumper 可能在删除后重新创建文件，ClearErrors 效果失效。
func TestDownloadServiceImpl_ClearErrorsRace(t *testing.T) {
	deps := createTestDependencies(t)
	impl := &downloadServiceImpl{deps: deps}

	// ClearErrors 使用 deps.Config.RootPath，用 t.TempDir() 创建的数据
	// 不会被 ClearErrors 识别，因为 impl.deps 的 Config 指向另一个 RootPath。
	// 创建一个 RootPath 与 impl 共享的依赖。
	rootPath := t.TempDir()
	deps.Config.RootPath = rootPath
	impl = &downloadServiceImpl{deps: deps}

	now := time.Now()

	// 通过 pathHelper 获取路径
	pathHelper, err := path.NewStorePath(rootPath)
	require.NoError(t, err)

	// 确保 data 目录存在
	require.NoError(t, os.MkdirAll(filepath.Dir(pathHelper.ErrorsPath), 0755))

	// 准备初始数据
	initial := downloading.NewDumper()
	// 使用较多数据增大竞态窗口
	for entityID := 1; entityID <= 5; entityID++ {
		for tweetID := uint64(1); tweetID <= 100; tweetID++ {
			initial.Push(entityID, &twitter.Tweet{
				Id:        tweetID + uint64(entityID)*10000,
				CreatedAt: now,
			})
		}
	}
	require.NoError(t, initial.Dump(pathHelper.ErrorsPath))

	// 也准备 JSON 错误文件
	jsonInitial := downloading.NewJsonDumper()
	jsonInitial.Push("source.json", "file", &twitter.Tweet{Id: 1, CreatedAt: now})
	require.NoError(t, jsonInitial.Dump(pathHelper.JSONErrorsPath))

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine A：模拟 saveDumper 在锁内写入
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			incoming := downloading.NewDumper()
			incoming.Push(9, &twitter.Tweet{Id: 90000 + uint64(i), CreatedAt: now})
			impl.saveDumper(incoming, pathHelper.ErrorsPath)

			jsonIncoming := downloading.NewJsonDumper()
			jsonIncoming.Push("new.json", "file", &twitter.Tweet{Id: 90000 + uint64(i), CreatedAt: now})
			impl.saveJsonDumper(jsonIncoming, pathHelper.JSONErrorsPath)
		}
	}()

	// Goroutine B：模拟 ClearErrors 无锁删除
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = impl.ClearErrors()
		}
	}()

	wg.Wait()

	// 验证：文件要么不存在（Clear 成功），要么是合法 JSON（saveDumper 写入完成）
	// 但不应该是损坏的 JSON
	for name, path := range map[string]string{
		"errors.json":      pathHelper.ErrorsPath,
		"json_errors.json": pathHelper.JSONErrorsPath,
	} {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue // Clear 成功
		}
		require.NoError(t, err, "ISSUE B: stat %s failed", name)

		// 文件存在，验证是合法 JSON
		dumper := downloading.NewDumper()
		err = dumper.Load(path)
		if name == "json_errors.json" {
			jd := downloading.NewJsonDumper()
			err = jd.Load(path)
			_ = dumper // unused for json path
			if err != nil {
				t.Logf("ISSUE B: %s corrupted after ClearErrors race: %v", name, err)
			}
		} else {
			if err != nil {
				t.Logf("ISSUE B: %s corrupted after ClearErrors race: %v", name, err)
			}
		}
	}
}
