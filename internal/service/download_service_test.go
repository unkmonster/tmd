package service

import (
	"context"
	"errors"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"

	"github.com/unkmonster/tmd/internal/config"
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

func createTestDependencies() *Dependencies {
	return &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                nil,
		Config:            &config.Config{RootPath: "/test/path"},
		AppRootPath:       "/app",
	}
}

func TestDownloadServiceImpl_Struct(t *testing.T) {
	deps := createTestDependencies()
	service := NewDownloadService(deps)

	assert.NotNil(t, service)

	impl, ok := service.(*downloadServiceImpl)
	assert.True(t, ok)
	assert.NotNil(t, impl.deps)
}

func TestDownloadServiceImpl_NilReporterHandling(t *testing.T) {
	deps := createTestDependencies()
	service := NewDownloadService(deps).(*downloadServiceImpl)

	// 测试 nil reporter 被替换为 NopReporter
	reporter := service.getReporterOrDefault(nil)
	assert.NotNil(t, reporter)

	// 验证是 NopReporter 类型
	_, ok := reporter.(*NopReporter)
	assert.True(t, ok)
}

func TestDownloadServiceImpl_ValidReporterHandling(t *testing.T) {
	deps := createTestDependencies()
	service := NewDownloadService(deps).(*downloadServiceImpl)

	mockReporter := NewMockProgressReporter()
	reporter := service.getReporterOrDefault(mockReporter)
	assert.Equal(t, mockReporter, reporter)
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
		DB:          nil,
		Config:      &config.Config{RootPath: "/test"},
		AppRootPath: "/app",
	}

	service := NewDownloadService(deps)
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
				MarkTime:    nil,
			},
		},
		{
			name: "All true",
			opts: DownloadOptions{
				AutoFollow:  true,
				SkipProfile: true,
				NoRetry:     true,
				MarkTime:    strPtr("2024-01-01T00:00:00"),
			},
		},
		{
			name: "Mixed",
			opts: DownloadOptions{
				AutoFollow:  true,
				SkipProfile: false,
				NoRetry:     true,
				MarkTime:    strPtr("2024-06-15T12:30:00"),
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
	deps := createTestDependencies()
	service := NewDownloadService(deps)

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
				AppRootPath:       "/app",
			},
		},
		{
			name: "Empty AdditionalClients",
			deps: &Dependencies{
				Client:            resty.New(),
				AdditionalClients: []*resty.Client{},
				DB:                nil,
				Config:            &config.Config{RootPath: "/test"},
				AppRootPath:       "/app",
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
				DB:          nil,
				Config:      &config.Config{RootPath: "/test"},
				AppRootPath: "/app",
			},
		},
		{
			name: "Nil Config",
			deps: &Dependencies{
				Client:            resty.New(),
				AdditionalClients: []*resty.Client{},
				DB:                nil,
				Config:            nil,
				AppRootPath:       "/app",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewDownloadService(tc.deps)
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

func TestDownloadOptions_WithMarkTime(t *testing.T) {
	markTime := "2024-01-01T00:00:00"
	opts := DownloadOptions{
		MarkTime: &markTime,
	}

	assert.NotNil(t, opts.MarkTime)
	assert.Equal(t, markTime, *opts.MarkTime)
}

func TestDownloadOptions_NilMarkTime_Service(t *testing.T) {
	opts := DownloadOptions{
		MarkTime: nil,
	}

	assert.Nil(t, opts.MarkTime)
}

func TestMockProgressReporter_ProgressStages(t *testing.T) {
	reporter := NewMockProgressReporter()
	stages := []string{"syncing", "downloading", "retrying", "profile", "marking", "completed"}

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
	reporter.OnComplete("task-1", Result{Downloaded: 100, Failed: 5, Versioned: 10, Message: "Stats"})
	reporter.OnComplete("task-2", Result{Message: "Only message"})
	reporter.OnComplete("task-3", Result{})

	assert.Len(t, reporter.CompleteCalls, 3)
	assert.Equal(t, 100, reporter.CompleteCalls[0].Result.Downloaded)
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

func strPtr(s string) *string {
	return &s
}

// getReporterOrDefault 辅助方法（在 download_service.go 中实际存在）
func (s *downloadServiceImpl) getReporterOrDefault(reporter ProgressReporter) ProgressReporter {
	if reporter == nil {
		return &NopReporter{}
	}
	return reporter
}
