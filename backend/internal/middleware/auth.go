package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/repository"
)

// Auth 访问令牌认证中间件(用于代理接口)
func Auth(tokenRepo *repository.AccessTokenRepository, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenValue string

		// 根据请求路径选择认证方式
		if c.Request.URL.Path == "/v1/messages" {
			// Anthropic Messages API 使用 X-Api-Key
			tokenValue = c.GetHeader("X-Api-Key")
			if tokenValue == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Missing X-Api-Key header",
				})
				c.Abort()
				return
			}
		} else {
			// OpenAI API 使用 Authorization Bearer
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Missing authorization header",
				})
				c.Abort()
				return
			}

			// 解析 Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid authorization header format",
				})
				c.Abort()
				return
			}

			tokenValue = parts[1]
		}

		// 验证 token
		token, err := tokenRepo.FindByToken(c.Request.Context(), tokenValue)
		if err != nil {
			traceID := GetTraceID(c)
			logger.Warn("token validation failed",
				slog.String("trace_id", traceID),
				slog.String("error", err.Error()),
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// 检查 token 是否启用
		if !token.IsActive() {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token is disabled",
			})
			c.Abort()
			return
		}

		// 更新最后使用时间(异步)
		go tokenRepo.UpdateLastUsed(c.Request.Context(), token.ID)

		// 将 token 信息存储到上下文
		c.Set("access_token_id", token.ID)
		c.Set("access_token_name", token.Name)

		c.Next()
	}
}
