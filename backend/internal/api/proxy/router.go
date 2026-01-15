package proxy

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
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
) {
	// 创建 repositories
	channelRepo := repository.NewChannelRepository(db)
	modelRepo := repository.NewModelRepository(db, logger)
	accessTokenRepo := repository.NewAccessTokenRepository(db)

	// 创建代理服务
	proxyService := proxy.NewProxyService(logger, channelRepo, circuitManager, auditLogger, proxyServiceConfig)

	// 创建 handlers
	chatCompletionsHandler := NewChatCompletionsHandler(logger, proxyService)
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
		v1.GET("/models", modelsHandler.Handle)
	}

	logger.Info("proxy routes registered",
		slog.String("prefix", "/v1"),
		slog.Int("routes_count", 2),
	)
}
