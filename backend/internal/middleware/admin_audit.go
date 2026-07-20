package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AdminAuditLogger 输出管理后台请求审计日志。
//
// 策略：
//   - 写操作与异常响应使用 Info/Warn/Error，便于审计；
//   - 成功 GET/HEAD/OPTIONS 降为 Debug，避免管理后台列表查询刷屏；
//   - 不记录请求体、响应体和 Authorization，避免敏感信息进入文件日志。
func AdminAuditLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		traceID := GetTraceID(c)

		c.Next()

		if logger == nil {
			return
		}

		statusCode := c.Writer.Status()
		routePath := c.FullPath()
		if strings.TrimSpace(routePath) == "" {
			routePath = c.Request.URL.Path
		}

		attrs := []any{
			slog.String("component", "admin"),
			slog.String("event", "admin.request.completed"),
			slog.String("trace_id", traceID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("route", routePath),
			slog.Int("status_code", statusCode),
			slog.Int("response_size", c.Writer.Size()),
			slog.Duration("duration", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
		}
		if userID, ok := c.Get("user_id"); ok {
			attrs = append(attrs, slog.Any("user_id", userID))
		}
		if username, ok := c.Get("username"); ok {
			if value, ok := username.(string); ok && value != "" {
				attrs = append(attrs, slog.String("username", value))
			}
		}

		msg := "管理后台请求完成"
		switch {
		case statusCode >= http.StatusInternalServerError:
			logger.Error(msg, attrs...)
		case statusCode >= http.StatusBadRequest:
			logger.Warn(msg, attrs...)
		case isAdminAuditMethod(c.Request.Method):
			logger.Info(msg, attrs...)
		default:
			logger.Debug(msg, attrs...)
		}
	}
}

func isAdminAuditMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
