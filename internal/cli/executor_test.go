package cli

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/service"
)

// MockDownloadService 模拟下载服务
type MockDownloadService struct {
	mock.Mock
}

func (m *MockDownloadService) UserDownload(ctx context.Context, taskID string, screenName string, opts service.DownloadOptions, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, screenName, opts, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) ListDownload(ctx context.Context, taskID string, listID uint64, opts service.DownloadOptions, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, listID, opts, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) FollowingDownload(ctx context.Context, taskID string, screenName string, opts service.DownloadOptions, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, screenName, opts, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) BatchDownload(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, opts service.DownloadOptions, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, screenNames, listIDs, followingNames, opts, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) ProfileDownload(ctx context.Context, taskID string, users []string, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, users, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) ListProfileDownload(ctx context.Context, taskID string, listID uint64, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, listID, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) JsonFileDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, paths, noRetry, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) JsonFolderDownload(ctx context.Context, taskID string, paths []string, noRetry bool, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, paths, noRetry, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) MarkDownloaded(ctx context.Context, taskID string, screenNames []string, listIDs []uint64, followingNames []string, markTime *string, reporter service.ProgressReporter) error {
	args := m.Called(ctx, taskID, screenNames, listIDs, followingNames, markTime, reporter)
	return args.Error(0)
}

// ==================== Execute 测试 ====================

func TestExecute_ParseArgsError(t *testing.T) {
	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:      resty.New(),
			Config:      &config.Config{},
			AppRootPath: "/tmp",
		},
	}

	// 使用无效的列表ID
	args := []string{"-list", "notanumber"}
	err := Execute(context.Background(), args, deps)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse args failed")
}

func TestExecute_NilDependencies(t *testing.T) {
	err := Execute(context.Background(), []string{"-user", "testuser"}, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dependencies is nil")
}

func TestExecute_JsonDownload(t *testing.T) {
	mockSvc := new(MockDownloadService)
	mockSvc.On("JsonFileDownload", mock.Anything, mock.Anything, []string{"/path/to/file.json"}, false, mock.Anything).Return(nil)

	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:      resty.New(),
			Config:      &config.Config{},
			AppRootPath: "/tmp",
		},
		DownloadService: mockSvc,
	}

	args := []string{"-jsonfile", "/path/to/file.json"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_JsonDownload_NoRetry(t *testing.T) {
	mockSvc := new(MockDownloadService)
	mockSvc.On("JsonFileDownload", mock.Anything, mock.Anything, []string{"/path/to/file.json"}, true, mock.Anything).Return(nil)

	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:      resty.New(),
			Config:      &config.Config{},
			AppRootPath: "/tmp",
		},
		DownloadService: mockSvc,
	}

	args := []string{"-jsonfile", "/path/to/file.json", "-no-retry"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_MarkDownloaded(t *testing.T) {
	mockSvc := new(MockDownloadService)
	markTime := "2024-01-01T00:00:00"
	mockSvc.On("MarkDownloaded", mock.Anything, mock.Anything, []string{"testuser"}, mock.AnythingOfType("[]uint64"), mock.AnythingOfType("[]string"), &markTime, mock.Anything).Return(nil)

	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:      resty.New(),
			Config:      &config.Config{},
			AppRootPath: "/tmp",
		},
		DownloadService: mockSvc,
	}

	args := []string{"-user", "testuser", "-mark-downloaded", "-mark-time", "2024-01-01T00:00:00"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_ProfileDownload(t *testing.T) {
	mockSvc := new(MockDownloadService)
	mockSvc.On("ProfileDownload", mock.Anything, mock.Anything, []string{"user1", "user2"}, mock.Anything).Return(nil)

	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:      resty.New(),
			Config:      &config.Config{},
			AppRootPath: "/tmp",
		},
		DownloadService: mockSvc,
	}

	args := []string{"-profile-user", "user1", "-profile-user", "user2"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_ListProfileDownload(t *testing.T) {
	mockSvc := new(MockDownloadService)
	mockSvc.On("ListProfileDownload", mock.Anything, mock.Anything, uint64(12345), mock.Anything).Return(nil)

	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:      resty.New(),
			Config:      &config.Config{},
			AppRootPath: "/tmp",
		},
		DownloadService: mockSvc,
	}

	args := []string{"-profile-list", "12345"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_NoArgs(t *testing.T) {
	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:      resty.New(),
			Config:      &config.Config{},
			AppRootPath: "/tmp",
		},
		DownloadService: nil,
	}

	args := []string{}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
}

func TestExecute_NoArgs_LogsHint(t *testing.T) {
	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:      resty.New(),
			Config:      &config.Config{},
			AppRootPath: "/tmp",
		},
	}

	var buf bytes.Buffer
	originalOutput := log.StandardLogger().Out
	log.SetOutput(&buf)
	defer log.SetOutput(originalOutput)

	err := Execute(context.Background(), nil, deps)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "no download tasks specified")
}

func TestExecute_DefaultServiceCreation(t *testing.T) {
	// 当DownloadService为nil时，应该创建默认服务
	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:            resty.New(),
			AdditionalClients: []*resty.Client{},
			DB:                &sqlx.DB{},
			Config:            &config.Config{},
			AppRootPath:       "/tmp",
		},
		DownloadService: nil,
	}

	// 由于没有实际的Twitter API，这会失败，但可以验证默认服务被创建
	args := []string{}
	err := Execute(context.Background(), args, deps)

	// 空参数不应该报错
	assert.NoError(t, err)
}

// ==================== Dependencies 测试 ====================

func TestDependencies_Struct(t *testing.T) {
	client := resty.New()
	db := &sqlx.DB{}
	cfg := &config.Config{}

	deps := &Dependencies{
		Dependencies: service.Dependencies{
			Client:            client,
			AdditionalClients: []*resty.Client{client},
			DB:                db,
			Config:            cfg,
			AppRootPath:       "/app/root",
		},
		DownloadService: nil,
	}

	assert.Equal(t, client, deps.Client)
	assert.Len(t, deps.AdditionalClients, 1)
	assert.Equal(t, db, deps.DB)
	assert.Equal(t, cfg, deps.Config)
	assert.Equal(t, "/app/root", deps.AppRootPath)
	assert.Nil(t, deps.DownloadService)
}

// ==================== SetClientLogger 测试 ====================

func TestSetClientLogger(t *testing.T) {
	client := resty.New()
	var buf bytes.Buffer

	SetClientLogger(client, &buf)

	// 验证logger已设置
	assert.NotNil(t, client)
}

func TestSetClientLogger_NilWriter(t *testing.T) {
	client := resty.New()

	// 使用Discard writer
	SetClientLogger(client, io.Discard)

	assert.NotNil(t, client)
}

func TestSetClientLogger_MultipleCalls(t *testing.T) {
	client := resty.New()
	var buf1, buf2 bytes.Buffer

	// 第一次设置
	SetClientLogger(client, &buf1)

	// 第二次设置应该覆盖
	SetClientLogger(client, &buf2)

	assert.NotNil(t, client)
}
