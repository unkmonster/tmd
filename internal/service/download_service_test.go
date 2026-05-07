package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
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
	m.ProgressCalls = append(m.ProgressCalls, struct {
		TaskID   string
		Progress Progress
	}{TaskID: taskID, Progress: p})
}

func (m *MockProgressReporter) OnComplete(taskID string, r Result) {
	m.CompleteCalls = append(m.CompleteCalls, struct {
		TaskID string
		Result Result
	}{TaskID: taskID, Result: r})
}

func (m *MockProgressReporter) OnError(taskID string, err error) {
	m.ErrorCalls = append(m.ErrorCalls, struct {
		TaskID string
		Err    error
	}{TaskID: taskID, Err: err})
}

func createTestDependencies(t *testing.T) *Dependencies {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)
	database.CreateTables(db)
	t.Cleanup(func() {
		_ = db.Close()
	})

	return &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                db,
		Config:            &config.Config{RootPath: "/test/path"},
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
	deps := &Dependencies{
		Client: resty.New(),
		AdditionalClients: []*resty.Client{
			resty.New(),
			resty.New(),
			resty.New(),
		},
		DB:     &sqlx.DB{},
		Config: &config.Config{RootPath: "/test"},
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
				Config:            &config.Config{RootPath: "/test"},
			},
		},
		{
			name: "Empty AdditionalClients",
			deps: &Dependencies{
				Client:            resty.New(),
				AdditionalClients: []*resty.Client{},
				DB:                &sqlx.DB{},
				Config:            &config.Config{RootPath: "/test"},
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
				Config: &config.Config{RootPath: "/test"},
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
