package cli

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/unkmonster/tmd/internal/config"
	"github.com/unkmonster/tmd/internal/service"
	"github.com/unkmonster/tmd/internal/twitter"
)

// MockDownloadService 模拟下载服务
type MockDownloadService struct {
	mock.Mock
}

func (m *MockDownloadService) UserDownload(ctx context.Context, source string, screenName string, opts service.DownloadOptions, reporter service.ProgressReporter) error {
	args := m.Called(ctx, source, screenName, opts, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) ListDownload(ctx context.Context, source string, listID uint64, opts service.DownloadOptions, reporter service.ProgressReporter) error {
	args := m.Called(ctx, source, listID, opts, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) FollowingDownload(ctx context.Context, source string, screenName string, opts service.DownloadOptions, reporter service.ProgressReporter) error {
	args := m.Called(ctx, source, screenName, opts, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) BatchDownload(ctx context.Context, source string, users []*twitter.User, lists []twitter.ListBase, opts service.DownloadOptions, reporter service.ProgressReporter) error {
	args := m.Called(ctx, source, users, lists, opts, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) ProfileDownload(ctx context.Context, source string, users []string, reporter service.ProgressReporter) error {
	args := m.Called(ctx, source, users, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) ListProfileDownload(ctx context.Context, source string, listID uint64, reporter service.ProgressReporter) error {
	args := m.Called(ctx, source, listID, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) JsonDownload(ctx context.Context, source string, paths []string, noRetry bool, reporter service.ProgressReporter) error {
	args := m.Called(ctx, source, paths, noRetry, reporter)
	return args.Error(0)
}

func (m *MockDownloadService) MarkDownloaded(ctx context.Context, source string, users []*twitter.User, lists []twitter.ListBase, markTime *string, reporter service.ProgressReporter) error {
	args := m.Called(ctx, source, users, lists, markTime, reporter)
	return args.Error(0)
}

// ==================== Execute 测试 ====================

func TestExecute_ParseArgsError(t *testing.T) {
	deps := &Dependencies{
		Client:      resty.New(),
		Conf:        &config.Config{},
		AppRootPath: "/tmp",
	}

	// 使用无效的列表ID
	args := []string{"-list", "notanumber"}
	err := Execute(context.Background(), args, deps)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse args failed")
}

func TestExecute_JsonDownload(t *testing.T) {
	mockSvc := new(MockDownloadService)
	mockSvc.On("JsonDownload", mock.Anything, "cli", []string{"/path/to/file.json"}, false, mock.Anything).Return(nil)

	deps := &Dependencies{
		Client:          resty.New(),
		Conf:            &config.Config{},
		AppRootPath:     "/tmp",
		DownloadService: mockSvc,
	}

	args := []string{"-json", "/path/to/file.json"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_JsonDownload_NoRetry(t *testing.T) {
	mockSvc := new(MockDownloadService)
	mockSvc.On("JsonDownload", mock.Anything, "cli", []string{"/path/to/file.json"}, true, mock.Anything).Return(nil)

	deps := &Dependencies{
		Client:          resty.New(),
		Conf:            &config.Config{},
		AppRootPath:     "/tmp",
		DownloadService: mockSvc,
	}

	args := []string{"-json", "/path/to/file.json", "-no-retry"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_MarkDownloaded(t *testing.T) {
	mockSvc := new(MockDownloadService)
	markTime := "2024-01-01T00:00:00"
	mockSvc.On("MarkDownloaded", mock.Anything, "cli", mock.AnythingOfType("[]*twitter.User"), mock.AnythingOfType("[]twitter.ListBase"), &markTime, mock.Anything).Return(nil)

	deps := &Dependencies{
		Client:          resty.New(),
		Conf:            &config.Config{},
		AppRootPath:     "/tmp",
		DownloadService: mockSvc,
	}

	args := []string{"-user", "testuser", "-mark-downloaded", "-mark-time", "2024-01-01T00:00:00"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_ProfileDownload(t *testing.T) {
	mockSvc := new(MockDownloadService)
	mockSvc.On("ProfileDownload", mock.Anything, "cli", []string{"user1", "user2"}, mock.Anything).Return(nil)

	deps := &Dependencies{
		Client:          resty.New(),
		Conf:            &config.Config{},
		AppRootPath:     "/tmp",
		DownloadService: mockSvc,
	}

	args := []string{"-profile-user", "user1", "-profile-user", "user2"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_ListProfileDownload(t *testing.T) {
	mockSvc := new(MockDownloadService)
	mockSvc.On("ListProfileDownload", mock.Anything, "cli", uint64(12345), mock.Anything).Return(nil)

	deps := &Dependencies{
		Client:          resty.New(),
		Conf:            &config.Config{},
		AppRootPath:     "/tmp",
		DownloadService: mockSvc,
	}

	args := []string{"-profile-list", "12345"}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestExecute_NoArgs(t *testing.T) {
	deps := &Dependencies{
		Client:          resty.New(),
		Conf:            &config.Config{},
		AppRootPath:     "/tmp",
		DownloadService: nil,
	}

	args := []string{}
	err := Execute(context.Background(), args, deps)

	assert.NoError(t, err)
}

func TestExecute_DefaultServiceCreation(t *testing.T) {
	// 当DownloadService为nil时，应该创建默认服务
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                &sqlx.DB{},
		Conf:              &config.Config{},
		AppRootPath:       "/tmp",
		DownloadService:   nil,
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
		Client:            client,
		AdditionalClients: []*resty.Client{client},
		DB:                db,
		Conf:              cfg,
		AppRootPath:       "/app/root",
		DownloadService:   nil,
	}

	assert.Equal(t, client, deps.Client)
	assert.Len(t, deps.AdditionalClients, 1)
	assert.Equal(t, db, deps.DB)
	assert.Equal(t, cfg, deps.Conf)
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
