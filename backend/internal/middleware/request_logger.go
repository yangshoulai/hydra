package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger 请求日志中间件
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		startTime := time.Now()

		// 获取 TraceID
		traceID := GetTraceID(c)

		// 记录请求基本信息
		logger.Info("request started",
			slog.String("trace_id", traceID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
		)

		// 处理请求
		c.Next()

		// 计算请求耗时
		duration := time.Since(startTime)

		// 记录响应信息
		logger.Info("request completed",
			slog.String("trace_id", traceID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status_code", c.Writer.Status()),
			slog.Int("response_size", c.Writer.Size()),
			slog.Duration("duration", duration),
		)
	}
}
