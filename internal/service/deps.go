package service

import (
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

// NewDownloadService 创建下载服务
func NewDownloadService(deps *Dependencies) DownloadService {
	return &downloadServiceImpl{
		deps: deps,
	}
}
