package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// TraceIDHeader HTTP Header 名称
	TraceIDHeader = "X-Trace-ID"
	// TraceIDKey Gin Context 中的键名
	TraceIDKey = "trace_id"
)

// TraceID 为每个请求注入唯一 TraceID。
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从请求头获取 TraceID
		traceID := c.GetHeader(TraceIDHeader)

		// 如果请求头中没有，生成新的 UUID
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// 存储到 Gin Context 中
		c.Set(TraceIDKey, traceID)

		// 设置响应头
		c.Header(TraceIDHeader, traceID)

		c.Next()
	}
}

// GetTraceID 从 Gin Context 中获取 TraceID
func GetTraceID(c *gin.Context) string {
	if traceID, exists := c.Get(TraceIDKey); exists {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	return ""
}

// RequestLogger 请求日志中间件
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		startTime := time.Now()

		// 获取 TraceID
		traceID := GetTraceID(c)

		// 调试日志：请求开始
		logger.Debug("请求开始",
			slog.String("trace_id", traceID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("client_ip", c.ClientIP()),
		)

		// 处理请求
		c.Next()

		// 计算请求耗时
		duration := time.Since(startTime)

		// 调试日志：请求完成
		logger.Debug("请求完成",
			slog.String("trace_id", traceID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status_code", c.Writer.Status()),
			slog.Int("response_size", c.Writer.Size()),
			slog.Duration("duration", duration),
		)
	}
}
