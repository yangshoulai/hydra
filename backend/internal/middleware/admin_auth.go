package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	// SessionKeyUserID 会话中存储的用户ID键名
	SessionKeyUserID = "user_id"
	// SessionKeyUsername 会话中存储的用户名键名
	SessionKeyUsername = "username"
)

// AdminAuth 管理后台会话认证中间件
func AdminAuth(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		// 获取会话中的用户信息
		userID := session.Get(SessionKeyUserID)
		if userID == nil {
			traceID := GetTraceID(c)
			logger.Warn("unauthorized admin access attempt",
				slog.String("trace_id", traceID),
				slog.String("path", c.Request.URL.Path),
			)

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("admin_user_id", userID)
		if username := session.Get(SessionKeyUsername); username != nil {
			c.Set("admin_username", username)
		}

		c.Next()
	}
}

// OptionalAdminAuth 可选的管理员认证(登录后有额外权限,未登录也可访问)
func OptionalAdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		// 尝试获取会话中的用户信息
		userID := session.Get(SessionKeyUserID)
		if userID != nil {
			c.Set("admin_user_id", userID)
			if username := session.Get(SessionKeyUsername); username != nil {
				c.Set("admin_username", username)
			}
		}

		c.Next()
	}
}
