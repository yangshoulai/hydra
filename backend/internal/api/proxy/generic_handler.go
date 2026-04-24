package proxy

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/service/proxy"
)

// GenericHandler 通用端点处理器
type GenericHandler struct {
	logger       *slog.Logger
	proxyService *proxy.ProxyService
	endpoint     endpoint.Endpoint
}

// NewGenericHandler 创建通用端点处理器
func NewGenericHandler(logger *slog.Logger, proxyService *proxy.ProxyService, ep endpoint.Endpoint) *GenericHandler {
	return &GenericHandler{
		logger:       logger,
		proxyService: proxyService,
		endpoint:     ep,
	}
}

// Handle 处理请求
func (h *GenericHandler) Handle(c *gin.Context) {
	traceID := middleware.GetTraceID(c)

	// 调用代理服务处理请求
	err := h.proxyService.ProxyRequest(c, h.endpoint)

	if err != nil {
		if !proxy.ShouldSuppressProxyLogging(c) {
			h.logger.Debug("代理处理返回错误",
				slog.String("trace_id", traceID),
				slog.String("endpoint", h.endpoint.GetName()),
				slog.String("error", err.Error()),
			)
		}

		// 如果响应还没有发送，返回错误
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message":  "Internal server error",
					"type":     "internal_error",
					"trace_id": traceID,
				},
			})
		}
		return
	}
}
