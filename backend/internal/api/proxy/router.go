package proxy

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	"github.com/yangshoulai/hydra/internal/service/logger"
	"github.com/yangshoulai/hydra/internal/service/proxy"
	"gorm.io/gorm"
)

// RegisterRoutes 注册代理路由
func RegisterRoutes(
	router *gin.Engine,
	db *gorm.DB,
	logger *slog.Logger,
	circuitManager *circuit.Manager,
	auditLogger *logger.AuditLogger,
	debugModeManager *logger.DebugModeManager,
	proxyServiceConfig *proxy.ProxyServiceConfig,
	settingService *configService.SettingService,
) {
	// 创建 repositories
	channelRepo := repository.NewChannelRepository(db)
	modelRepo := repository.NewModelRepository(db, logger)
	accessTokenRepo := repository.NewAccessTokenRepository(db)

	// 创建代理服务（传入 settingService 以支持配置热更新）
	proxySvc := proxy.NewProxyService(logger, channelRepo, circuitManager, auditLogger, proxyServiceConfig, settingService)

	// 注册 ProxyService 为配置监听器
	settingService.RegisterListener(proxySvc)

	// 从系统设置加载明文错误规则（用于非流式响应嗅探）
	ctx := context.Background()
	keywords := settingService.GetPlainTextErrorRules(ctx)
	if len(keywords) > 0 {
		proxySvc.UpdateSnifferKeywords(keywords)
		logger.Info("从系统设置加载明文错误规则",
			slog.Int("count", len(keywords)),
		)
	}

	// 创建通用 handlers
	modelsHandler := NewModelsHandler(logger, modelRepo)

	// 创建 v1 路由组
	v1 := router.Group("/v1")
	{
		// 应用中间件
		v1.Use(middleware.TraceID())
		v1.Use(middleware.RequestLogger(logger))
		v1.Use(middleware.Auth(accessTokenRepo, logger)) // 访问令牌认证

		// 从端点注册中心动态注册路由
		registry := endpoint.GetGlobalRegistry()
		for _, ep := range registry.GetAll() {
			epPath := ep.GetPath()
			// 去掉 /v1 前缀，因为已经在路由组中
			routePath := strings.TrimPrefix(epPath, "/v1")

			// 创建通用 handler
			handler := NewGenericHandler(logger, proxySvc, epPath, ep.GetName())
			v1.POST(routePath, handler.Handle)

			logger.Info("注册端点路由",
				slog.String("name", ep.GetName()),
				slog.String("type", ep.GetType()),
				slog.String("path", routePath),
			)
		}

		// 注册 /models 端点
		v1.GET("/models", modelsHandler.Handle)
	}

	logger.Info("代理路由注册成功", slog.String("prefix", "/v1"))
}
