package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// TraceIDHeader HTTP Header 名称
	TraceIDHeader = "X-Trace-ID"
	// TraceIDKey Gin Context 中的键名
	TraceIDKey = "trace_id"
)

// TraceID 中间件:为每个请求生成唯一的 TraceID
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从请求头获取 TraceID
		traceID := c.GetHeader(TraceIDHeader)

		// 如果请求头中没有,生成新的 UUID
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
