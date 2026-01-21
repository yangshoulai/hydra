package proxy

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/service/proxy"
)

// GenericHandler 通用端点处理器
type GenericHandler struct {
	logger       *slog.Logger
	proxyService *proxy.ProxyService
	endpointPath string
	endpointName string
}

// NewGenericHandler 创建通用端点处理器
func NewGenericHandler(logger *slog.Logger, proxyService *proxy.ProxyService, endpointPath string, endpointName string) *GenericHandler {
	return &GenericHandler{
		logger:       logger,
		proxyService: proxyService,
		endpointPath: endpointPath,
		endpointName: endpointName,
	}
}

// Handle 处理请求
func (h *GenericHandler) Handle(c *gin.Context) {
	startTime := time.Now()
	traceID := middleware.GetTraceID(c)

	h.logger.Info("收到请求",
		slog.String("trace_id", traceID),
		slog.String("endpoint", h.endpointName),
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("client_ip", c.ClientIP()),
	)

	// 调用代理服务处理请求
	err := h.proxyService.ProxyRequest(c, h.endpointPath)

	duration := time.Since(startTime)

	if err != nil {
		h.logger.Error("请求处理失败",
			slog.String("trace_id", traceID),
			slog.String("endpoint", h.endpointName),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)

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

	h.logger.Info("request completed",
		slog.String("trace_id", traceID),
		slog.String("endpoint", h.endpointName),
		slog.Duration("duration", duration),
		slog.Int("status_code", c.Writer.Status()),
	)
}
