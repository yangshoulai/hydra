package app

import (
	"log/slog"

	"github.com/yangshoulai/hydra/internal/repository"
	adminService "github.com/yangshoulai/hydra/internal/service/admin"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	modelsyncService "github.com/yangshoulai/hydra/internal/service/modelsync"
	proxyService "github.com/yangshoulai/hydra/internal/service/proxy"
	schedulerService "github.com/yangshoulai/hydra/internal/service/scheduler"
	"gorm.io/gorm"
)

// Repositories 应用内共享的仓储实例
type Repositories struct {
	AdminUserRepo     *repository.AdminUserRepository
	AccessTokenRepo   *repository.AccessTokenRepository
	ChannelRepo       *repository.ChannelRepository
	ChannelKeyRepo    *repository.ChannelKeyRepository
	ModelConfigRepo   *repository.ChannelModelConfigRepository
	SystemSettingRepo *repository.SystemSettingRepository
	ModelRepo         *repository.ModelRepository
	ProviderRepo      *repository.ProviderRepository
	RequestLogRepo    *repository.RequestLogRepository
}

// Services 应用内共享的服务实例
type Services struct {
	SettingService      *configService.SettingService
	CircuitManager      *circuit.CircuitManager
	CronScheduler       *schedulerService.CronScheduler
	ProxyService        *proxyService.ProxyService
	AuthService         *adminService.AuthService
	JWTService          *adminService.JWTService
	HealthCheckService  *adminService.HealthCheckService
	SyncService         *modelsyncService.SyncService
	DashboardService    *adminService.DashboardService
	ModelService        *adminService.ModelService
	ProviderService     *adminService.ProviderService
	CircuitProbeHandler *circuit.ProbeHandler
}

// Components 应用启动后共享的核心组件
type Components struct {
	DB       *gorm.DB
	Logger   *slog.Logger
	Repos    *Repositories
	Services *Services
}
