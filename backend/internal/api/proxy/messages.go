package proxy

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/service/proxy"
)

// MessagesHandler Messages API 请求处理器
type MessagesHandler struct {
	logger       *slog.Logger
	proxyService *proxy.ProxyService
}

// NewMessagesHandler 创建 Messages 处理器
func NewMessagesHandler(logger *slog.Logger, proxyService *proxy.ProxyService) *MessagesHandler {
	return &MessagesHandler{
		logger:       logger,
		proxyService: proxyService,
	}
}

// Handle 处理 POST /v1/messages 请求
func (h *MessagesHandler) Handle(c *gin.Context) {
	startTime := time.Now()
	traceID := middleware.GetTraceID(c)

	h.logger.Info("messages request received",
		slog.String("trace_id", traceID),
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("client_ip", c.ClientIP()),
	)

	// 调用代理服务处理请求
	err := h.proxyService.ProxyMessages(c)

	duration := time.Since(startTime)

	if err != nil {
		h.logger.Error("messages request failed",
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

	h.logger.Info("messages request completed",
		slog.String("trace_id", traceID),
		slog.Duration("duration", duration),
		slog.Int("status_code", c.Writer.Status()),
	)
}
