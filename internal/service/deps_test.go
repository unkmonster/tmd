package service

import (
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/unkmonster/tmd/internal/config"
)

func TestDependencies_Struct(t *testing.T) {
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{resty.New(), resty.New()},
		Config:            &config.Config{RootPath: "/test/path"},
		AppRootPath:       "/app/root",
	}

	assert.NotNil(t, deps.Client)
	assert.Len(t, deps.AdditionalClients, 2)
	assert.NotNil(t, deps.Config)
	assert.Equal(t, "/test/path", deps.Config.RootPath)
	assert.Equal(t, "/app/root", deps.AppRootPath)
}

func TestNewDownloadService(t *testing.T) {
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                &sqlx.DB{},
		Config:            &config.Config{RootPath: "/test"},
		AppRootPath:       "/app",
	}

	service := NewDownloadService(deps)

	assert.NotNil(t, service)

	impl, ok := service.(*downloadServiceImpl)
	assert.True(t, ok)
	assert.NotNil(t, impl)
	assert.Equal(t, deps, impl.deps)
}

func TestNewDownloadService_WithNilDB(t *testing.T) {
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                nil,
		Config:            &config.Config{RootPath: "/test"},
		AppRootPath:       "/app",
	}

	service := NewDownloadService(deps)

	assert.NotNil(t, service)

	impl, ok := service.(*downloadServiceImpl)
	assert.True(t, ok)
	assert.NotNil(t, impl)
}

func TestNewDownloadService_WithMultipleAdditionalClients(t *testing.T) {
	deps := &Dependencies{
		Client: resty.New(),
		AdditionalClients: []*resty.Client{
			resty.New(),
			resty.New(),
			resty.New(),
		},
		DB:          &sqlx.DB{},
		Config:      &config.Config{RootPath: "/test"},
		AppRootPath: "/app",
	}

	service := NewDownloadService(deps)

	assert.NotNil(t, service)

	impl, ok := service.(*downloadServiceImpl)
	assert.True(t, ok)
	assert.NotNil(t, impl)
	assert.Len(t, impl.deps.AdditionalClients, 3)
}

func TestDependencies_EmptyAdditionalClients(t *testing.T) {
	deps := &Dependencies{
		Client:            resty.New(),
		AdditionalClients: []*resty.Client{},
		DB:                &sqlx.DB{},
		Config:            &config.Config{RootPath: "/test"},
		AppRootPath:       "/app",
	}

	assert.NotNil(t, deps)
	assert.Empty(t, deps.AdditionalClients)
}
