package service

import (
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	"github.com/unkmonster/tmd/internal/config"
)

// Dependencies Service 依赖
type Dependencies struct {
	Client            *resty.Client
	AdditionalClients []*resty.Client
	DB                *sqlx.DB
	Config            *config.Config
	AppRootPath       string
}

// Validate 验证依赖项是否完整
func (d *Dependencies) Validate() error {
	if d == nil {
		return errors.New("dependencies is nil")
	}
	if d.Client == nil {
		return errors.New("client is required")
	}
	if d.DB == nil {
		return errors.New("db is required")
	}
	if d.Config == nil {
		return errors.New("config is required")
	}
	if d.Config.RootPath == "" {
		return errors.New("config.RootPath is required")
	}
	return nil
}

// NewDownloadService 创建下载服务
func NewDownloadService(deps *Dependencies) (DownloadService, error) {
	if err := deps.Validate(); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}
	return &downloadServiceImpl{
		deps: deps,
	}, nil
}
