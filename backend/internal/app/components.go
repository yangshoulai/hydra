package app

import (
	"log/slog"

	"github.com/yangshoulai/hydra/internal/repository"
	adminService "github.com/yangshoulai/hydra/internal/service/admin"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	"github.com/yangshoulai/hydra/internal/service/logger"
	modelsyncService "github.com/yangshoulai/hydra/internal/service/modelsync"
	proxyService "github.com/yangshoulai/hydra/internal/service/proxy"
	schedulerService "github.com/yangshoulai/hydra/internal/service/scheduler"
	"gorm.io/gorm"
)

// Repositories 应用内共享的仓储实例
type Repositories struct {
	AdminUser     *repository.AdminUserRepository
	AccessToken   *repository.AccessTokenRepository
	Channel       *repository.ChannelRepository
	Key           *repository.KeyRepository
	ModelConfig   *repository.ChannelModelConfigRepository
	RequestLog    *repository.RequestLogRepository
	SystemSetting *repository.SystemSettingRepository
	Model         *repository.ModelRepository
	Provider      *repository.ProviderRepository
}

// Services 应用内共享的服务实例
type Services struct {
	Setting               *configService.SettingService
	CircuitManager        *circuit.Manager
	DebugModeManager      *logger.DebugModeManager
	CronScheduler         *schedulerService.CronScheduler
	ChannelModelScheduler *modelsyncService.ChannelModelSyncScheduler
	AuditLogger           *logger.AuditLogger
	LogCleanupService     *logger.LogCleanupService
	ProxyService          *proxyService.ProxyService
	AuthService           *adminService.AuthService
	JWTService            *adminService.JWTService
	HealthCheckService    *adminService.HealthCheckService
	SyncService           *modelsyncService.SyncService
	DashboardService      *adminService.DashboardService
	ModelService          *adminService.ModelService
	ProviderService       *adminService.ProviderService
	CircuitProbeHandler   *circuit.ProbeHandler
}

// Components 应用启动后共享的核心组件
type Components struct {
	DB       *gorm.DB
	Logger   *slog.Logger
	Repos    *Repositories
	Services *Services
}
