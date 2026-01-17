package proxy

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
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

	// 配置 SnifferManager 以支持热更新
	snifferManager := configService.GetSnifferManager()
	snifferManager.SetSettingService(settingService)
	snifferManager.RegisterUpdater(proxySvc)

	// 从系统设置加载明文错误规则
	ctx := context.Background()
	keywords := settingService.GetPlainTextErrorRules(ctx)
	if len(keywords) > 0 {
		proxySvc.UpdateSnifferKeywords(keywords)
		logger.Info("plain text error rules loaded from system settings",
			slog.Int("count", len(keywords)),
		)
	}

	// 创建 handlers
	chatCompletionsHandler := NewChatCompletionsHandler(logger, proxySvc)
	responsesHandler := NewResponsesHandler(logger, proxySvc)
	modelsHandler := NewModelsHandler(logger, modelRepo)

	// 创建 v1 路由组
	v1 := router.Group("/v1")
	{
		// 应用中间件
		v1.Use(middleware.TraceID())
		v1.Use(middleware.RequestLogger(logger))
		v1.Use(middleware.Auth(accessTokenRepo, logger)) // 访问令牌认证

		// 注册路由
		v1.POST("/chat/completions", chatCompletionsHandler.Handle)
		v1.POST("/responses", responsesHandler.Handle)
		v1.GET("/models", modelsHandler.Handle)
	}

	logger.Info("proxy routes registered",
		slog.String("prefix", "/v1"),
		slog.Int("routes_count", 3),
	)
}
