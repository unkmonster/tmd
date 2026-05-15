package service

import (
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unkmonster/tmd/internal/config"
)

func TestDependencies_Struct(t *testing.T) {
	tempDir := t.TempDir()
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{resty.New(), resty.New()},
		Config:            &config.Config{RootPath: tempDir},
	}

	assert.NotNil(t, deps.Client)
	assert.Len(t, deps.AdditionalClients, 2)
	assert.NotNil(t, deps.Config)
	assert.Equal(t, tempDir, deps.Config.RootPath)
}

func TestDependencies_Validate(t *testing.T) {
	tests := []struct {
		name    string
		deps    *Dependencies
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid dependencies",
			deps: &Dependencies{
				Client: resty.New(),
				DB:     &sqlx.DB{},
				Config: &config.Config{RootPath: t.TempDir()},
			},
			wantErr: false,
		},
		{
			name:    "nil dependencies",
			deps:    nil,
			wantErr: true,
			errMsg:  "dependencies is nil",
		},
		{
			name: "nil client",
			deps: &Dependencies{
				Client: nil,
				DB:     &sqlx.DB{},
				Config: &config.Config{RootPath: t.TempDir()},
			},
			wantErr: true,
			errMsg:  "client is required",
		},
		{
			name: "nil config",
			deps: &Dependencies{
				Client: resty.New(),
				DB:     &sqlx.DB{},
				Config: nil,
			},
			wantErr: true,
			errMsg:  "config is required",
		},
		{
			name: "empty root path",
			deps: &Dependencies{
				Client: resty.New(),
				DB:     &sqlx.DB{},
				Config: &config.Config{RootPath: ""},
			},
			wantErr: true,
			errMsg:  "config.RootPath is required",
		},
		{
			name: "nil db",
			deps: &Dependencies{
				Client: resty.New(),
				Config: &config.Config{RootPath: t.TempDir()},
				DB:     nil,
			},
			wantErr: true,
			errMsg:  "db is required",
		},
		{
			name: "nil additional client",
			deps: &Dependencies{
				Client:            resty.New(),
				AdditionalClients: []*resty.Client{resty.New(), nil},
				DB:                &sqlx.DB{},
				Config:            &config.Config{RootPath: t.TempDir()},
			},
			wantErr: true,
			errMsg:  "additional client 1 is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.deps.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewDownloadService(t *testing.T) {
	tempDir := t.TempDir()
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                &sqlx.DB{},
		Config:            &config.Config{RootPath: tempDir},
	}

	service, err := NewDownloadService(deps)
	require.NoError(t, err)
	assert.NotNil(t, service)

	impl, ok := service.(*downloadServiceImpl)
	assert.True(t, ok)
	assert.NotNil(t, impl)
	assert.Equal(t, deps, impl.deps)
}

func TestNewDownloadService_WithNilDB(t *testing.T) {
	tempDir := t.TempDir()
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                nil,
		Config:            &config.Config{RootPath: tempDir},
	}

	service, err := NewDownloadService(deps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db is required")
	assert.Nil(t, service)
}

func TestNewDownloadService_WithNilClient(t *testing.T) {
	tempDir := t.TempDir()
	deps := &Dependencies{
		Client:            nil,
		AdditionalClients: []*resty.Client{},
		DB:                &sqlx.DB{},
		Config:            &config.Config{RootPath: tempDir},
	}

	service, err := NewDownloadService(deps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client is required")
	assert.Nil(t, service)
}

func TestNewDownloadService_WithNilConfig(t *testing.T) {
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                &sqlx.DB{},
		Config:            nil,
	}

	service, err := NewDownloadService(deps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
	assert.Nil(t, service)
}

func TestNewDownloadService_WithEmptyRootPath(t *testing.T) {
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                &sqlx.DB{},
		Config:            &config.Config{RootPath: ""},
	}

	service, err := NewDownloadService(deps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config.RootPath is required")
	assert.Nil(t, service)
}

func TestNewDownloadService_WithMultipleAdditionalClients(t *testing.T) {
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
	assert.NotNil(t, impl)
	assert.Len(t, impl.deps.AdditionalClients, 3)
}

func TestDependencies_EmptyAdditionalClients(t *testing.T) {
	tempDir := t.TempDir()
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                &sqlx.DB{},
		Config:            &config.Config{RootPath: tempDir},
	}

	assert.NotNil(t, deps)
	assert.Empty(t, deps.AdditionalClients)
}
