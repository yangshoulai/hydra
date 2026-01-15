package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Recovery 错误恢复中间件
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取 TraceID
				traceID := GetTraceID(c)

				// 记录 panic 错误
				logger.Error("panic recovered",
					slog.String("trace_id", traceID),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
					slog.Any("error", err),
				)

				// 返回 500 错误
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
					"trace_id": traceID,
				})

				// 中止请求
				c.Abort()
			}
		}()

		c.Next()
	}
}
