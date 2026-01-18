package proxy

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/service/proxy"
)

// ChatCompletionsHandler Chat Completions 请求处理器
type ChatCompletionsHandler struct {
	logger       *slog.Logger
	proxyService *proxy.ProxyService
}

// NewChatCompletionsHandler 创建 Chat Completions 处理器
func NewChatCompletionsHandler(logger *slog.Logger, proxyService *proxy.ProxyService) *ChatCompletionsHandler {
	return &ChatCompletionsHandler{
		logger:       logger,
		proxyService: proxyService,
	}
}

// Handle 处理 POST /v1/chat/completions 请求
func (h *ChatCompletionsHandler) Handle(c *gin.Context) {
	startTime := time.Now()
	traceID := middleware.GetTraceID(c)

	h.logger.Info("收到聊天补全请求",
		slog.String("trace_id", traceID),
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("client_ip", c.ClientIP()),
	)

	// 调用代理服务处理请求
	err := h.proxyService.ProxyChatCompletions(c)

	duration := time.Since(startTime)

	if err != nil {
		h.logger.Error("聊天补全请求失败",
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

	h.logger.Info("聊天补全请求完成",
		slog.String("trace_id", traceID),
		slog.Duration("duration", duration),
		slog.Int("status_code", c.Writer.Status()),
	)
}
