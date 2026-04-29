package proxy

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/app"
	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/middleware"
)

// RegisterRoutes 注册代理路由
func RegisterRoutes(
	router *gin.Engine,
	components *app.Components,
) {
	logger := components.Logger
	repos := components.Repos
	services := components.Services
	proxySvc := services.ProxyService
	settingService := services.SettingService

	// 从系统设置加载明文错误规则（用于响应嗅探）
	ctx := context.Background()
	keywords := settingService.GetPlainTextErrorRules(ctx)
	if len(keywords) > 0 {
		proxySvc.UpdateSnifferKeywords(keywords)
		logger.Info("从系统设置加载明文错误规则",
			slog.Int("count", len(keywords)),
		)
	}

	// 创建通用 handlers
	modelsHandler := NewModelsHandler(logger, repos.ModelRepo)
	bodyLimitMiddleware := middleware.MaxBodyBytes(func() int64 {
		return settingService.GetProxyMaxBodyBytes(context.Background())
	})
	rateLimiter := middleware.NewProxyRateLimiter(settingService, logger)

	// 创建 v1 路由组
	v1 := router.Group("/v1")
	v1beta := router.Group("/v1beta")
	{
		// 应用中间件
		v1.Use(middleware.TraceID())
		v1.Use(middleware.RequestLogger(logger))
		v1.Use(bodyLimitMiddleware)
		v1.Use(middleware.Auth(repos.AccessTokenRepo, logger)) // 访问令牌认证
		v1.Use(rateLimiter.Handle())

		v1beta.Use(middleware.TraceID())
		v1beta.Use(middleware.RequestLogger(logger))
		v1beta.Use(bodyLimitMiddleware)
		v1beta.Use(middleware.Auth(repos.AccessTokenRepo, logger)) // 访问令牌认证
		v1beta.Use(rateLimiter.Handle())

		// 从显式端点列表注册路由
		for _, ep := range endpoint.DefaultEndpoints() {
			epPath := ep.GetPath()

			// 创建通用 handler
			handler := NewGenericHandler(logger, proxySvc, ep)

			if strings.HasPrefix(epPath, "/v1beta/") {
				routePath := strings.TrimPrefix(epPath, "/v1beta")
				v1beta.POST(routePath, handler.Handle)
				logger.Info("注册端点路由",
					slog.String("name", ep.GetName()),
					slog.String("type", ep.GetType()),
					slog.String("path", "/v1beta"+routePath),
				)
				continue
			}

			// 去掉 /v1 前缀，因为已经在路由组中
			routePath := strings.TrimPrefix(epPath, "/v1")
			v1.POST(routePath, handler.Handle)

			logger.Info("注册端点路由",
				slog.String("name", ep.GetName()),
				slog.String("type", ep.GetType()),
				slog.String("path", "/v1"+routePath),
			)
		}

		// 注册 /models 端点
		v1.GET("/models", modelsHandler.Handle)
		v1beta.GET("/models", modelsHandler.HandleV1Beta)
	}

	logger.Info("代理路由注册成功", slog.String("prefix", "/v1"))
}
