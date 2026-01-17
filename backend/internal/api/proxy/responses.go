package proxy

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/service/proxy"
)

// ResponsesHandler Responses API 请求处理器
type ResponsesHandler struct {
	logger       *slog.Logger
	proxyService *proxy.ProxyService
}

// NewResponsesHandler 创建 Responses 处理器
func NewResponsesHandler(logger *slog.Logger, proxyService *proxy.ProxyService) *ResponsesHandler {
	return &ResponsesHandler{
		logger:       logger,
		proxyService: proxyService,
	}
}

// Handle 处理 POST /v1/responses 请求
func (h *ResponsesHandler) Handle(c *gin.Context) {
	startTime := time.Now()
	traceID := middleware.GetTraceID(c)

	h.logger.Info("responses request received",
		slog.String("trace_id", traceID),
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("client_ip", c.ClientIP()),
	)

	// 调用代理服务处理请求
	err := h.proxyService.ProxyResponses(c)

	duration := time.Since(startTime)

	if err != nil {
		h.logger.Error("responses request failed",
			slog.String("trace_id", traceID),
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

	h.logger.Info("responses request completed",
		slog.String("trace_id", traceID),
		slog.Duration("duration", duration),
		slog.Int("status_code", c.Writer.Status()),
	)
}
