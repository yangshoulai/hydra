package admin

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/repository"
	adminService "github.com/yangshoulai/hydra/internal/service/admin"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	modelsyncService "github.com/yangshoulai/hydra/internal/service/modelsync"
	"gorm.io/gorm"
)

// RegisterRoutes 注册管理后台路由
func RegisterRoutes(
	router *gin.Engine,
	db *gorm.DB,
	logger *slog.Logger,
	circuitManager *circuit.Manager,
	settingService *configService.SettingService,
) {
	// 创建 repositories
	adminUserRepo := repository.NewAdminUserRepository(db)
	accessTokenRepo := repository.NewAccessTokenRepository(db)
	channelRepo := repository.NewChannelRepository(db)
	keyRepo := repository.NewKeyRepository(db)
	modelConfigRepo := repository.NewChannelModelConfigRepository(db)
	requestLogRepo := repository.NewRequestLogRepository(db)
	systemSettingRepo := repository.NewSystemSettingRepository(db)
	modelRepo := repository.NewModelRepository(db, logger)
	providerRepo := repository.NewProviderRepository(db, logger)

	// 创建服务
	authService := adminService.NewAuthService(db, logger, adminUserRepo, accessTokenRepo)
	jwtService := adminService.NewJWTService()
	probeHandler := circuit.NewProbeHandler(circuitManager, logger)
	healthCheckService := adminService.NewHealthCheckService(logger, keyRepo, channelRepo, probeHandler)
	syncService := modelsyncService.NewSyncService(logger, channelRepo, modelConfigRepo, keyRepo)
	logQueryService := adminService.NewLogQueryService(logger, requestLogRepo)

	// 创建 Dashboard 服务
	dashboardService := adminService.NewDashboardService(logger, requestLogRepo, channelRepo, keyRepo)

	// 创建 Model 服务
	modelService := adminService.NewModelService(modelRepo, modelConfigRepo, logger)

	// 创建 Provider 服务
	providerService := adminService.NewProviderService(providerRepo, logger)

	// 创建 handlers
	authHandler := NewAuthHandler(authService, logger)
	channelHandler := NewChannelHandler(channelRepo, modelConfigRepo, db, logger, circuitManager)
	keyHandler := NewKeyHandler(keyRepo, channelRepo, healthCheckService, circuitManager, logger)
	modelConfigHandler := NewChannelModelHandler(modelConfigRepo, channelRepo, logger)
	modelSyncHandler := NewModelSyncHandler(syncService, modelConfigRepo, db, logger)
	logHandler := NewLogHandler(logQueryService, logger)
	dashboardHandler := NewDashboardHandler(dashboardService)
	settingsHandler := NewSettingsHandler(logger, systemSettingRepo, settingService)
	tokensHandler := NewTokensHandler(logger, accessTokenRepo)
	modelHandler := NewModelHandler(modelService, logger)
	providerHandler := NewProviderHandler(providerService, logger)

	// 注册路由
	adminAPI := router.Group("/admin/api")
	{
		// 认证路由(不需要中间件)
		auth := adminAPI.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
		}

		// 需要认证的路由
		protected := adminAPI.Group("")
		protected.Use(middleware.JWTAuth(jwtService))
		{
			// 认证相关（需要 JWT）
			protectedAuth := protected.Group("/auth")
			{
				protectedAuth.POST("/change-password", authHandler.ChangePassword)
			}

			channelHandler.RegisterRoutes(protected)
			keyHandler.RegisterRoutes(protected)
			modelConfigHandler.RegisterRoutes(protected)
			modelSyncHandler.RegisterRoutes(protected)
			logHandler.RegisterRoutes(protected)
			dashboardHandler.RegisterRoutes(protected)
			settingsHandler.RegisterRoutes(protected)
			tokensHandler.RegisterRoutes(protected)
			modelHandler.RegisterRoutes(protected)
			providerHandler.RegisterRoutes(protected)
		}
	}
	logger.Info("管理后台路由注册成功", slog.String("prefix", "/admin/api"))
}
