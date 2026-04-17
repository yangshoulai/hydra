package admin

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/app"
	"github.com/yangshoulai/hydra/internal/middleware"
)

// RegisterRoutes 注册管理后台路由
func RegisterRoutes(
	router *gin.Engine,
	components *app.Components,
) {
	logger := components.Logger
	db := components.DB
	repos := components.Repos
	services := components.Services

	authService := services.AuthService
	jwtService := services.JWTService
	healthCheckService := services.HealthCheckService
	syncService := services.SyncService
	dashboardService := services.DashboardService
	modelService := services.ModelService
	providerService := services.ProviderService
	settingService := services.SettingService
	circuitManager := services.CircuitManager

	// 创建 handlers
	authHandler := NewAuthHandler(authService, logger)
	channelHandler := NewChannelHandler(repos.ChannelRepo, repos.ModelConfigRepo, repos.ChannelKeyRepo, db, logger, circuitManager)
	channelKeyHandler := NewChannelKeyHandler(repos.ChannelKeyRepo, repos.ChannelRepo, healthCheckService, circuitManager, logger)
	modelConfigHandler := NewChannelModelHandler(repos.ModelConfigRepo, repos.ChannelRepo, logger, circuitManager)
	modelSyncHandler := NewModelSyncHandler(syncService, repos.ModelConfigRepo, settingService, db, logger)
	dashboardHandler := NewDashboardHandler(dashboardService)
	settingsHandler := NewSettingsHandler(logger, repos.SystemSettingRepo, settingService)
	tokensHandler := NewTokensHandler(logger, repos.AccessTokenRepo)
	modelHandler := NewModelHandler(modelService, logger)
	providerHandler := NewProviderHandler(providerService, logger)
	endpointHandler := NewEndpointHandler()
	requestLogHandler := NewRequestLogHandler(repos.RequestLogRepo, logger)

	// 注册路由
	adminAPI := router.Group("/admin/api")
	{
		// 认证路由(不需要中间件)
		auth := adminAPI.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)
		}

		// 需要认证的路由
		protected := adminAPI.Group("")
		protected.Use(middleware.NewAdminAuthMiddleware(jwtService).Handle())
		{
			// 认证相关（需要 JWT）
			protectedAuth := protected.Group("/auth")
			{
				protectedAuth.POST("/change-password", authHandler.ChangePassword)
			}

			// 端点管理
			protected.GET("/endpoints", endpointHandler.GetEndpoints)

			channelHandler.RegisterRoutes(protected)
			channelKeyHandler.RegisterRoutes(protected)
			modelConfigHandler.RegisterRoutes(protected)
			modelSyncHandler.RegisterRoutes(protected)
			dashboardHandler.RegisterRoutes(protected)
			settingsHandler.RegisterRoutes(protected)
			tokensHandler.RegisterRoutes(protected)
			modelHandler.RegisterRoutes(protected)
			providerHandler.RegisterRoutes(protected)
			requestLogHandler.RegisterRoutes(protected)
		}
	}
	logger.Info("管理后台路由注册成功", slog.String("prefix", "/admin/api"))
}
