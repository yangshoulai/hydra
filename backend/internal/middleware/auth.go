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

		xApiKeyValue := c.GetHeader("X-Api-Key")
		authorizationValue := c.GetHeader("Authorization")
		googleApiKey := c.GetHeader("x-goog-api-key")
		if authorizationValue != "" {
			// 解析 Bearer token
			parts := strings.SplitN(authorizationValue, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
				c.Abort()
				return
			}
			tokenValue = parts[1]
		} else if xApiKeyValue != "" {
			tokenValue = xApiKeyValue
		} else {
			tokenValue = googleApiKey
		}
		if tokenValue == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			c.Abort()
			return
		}
		// 验证 token
		token, err := tokenRepo.FindByToken(c.Request.Context(), tokenValue)
		if err != nil {
			traceID := GetTraceID(c)
			logger.Warn("令牌验证失败", slog.String("trace_id", traceID), slog.String("error", err.Error()))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 检查 token 是否启用
		if !token.IsActive() {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token is disabled"})
			c.Abort()
			return
		}

		// 更新最后使用时间(异步)
		go func() {
			_ = tokenRepo.UpdateLastUsed(c.Request.Context(), token.ID)
		}()

		// 将 token 信息存储到上下文
		c.Set("access_token_id", token.ID)
		c.Set("access_token_name", token.Name)
		c.Set("access_token_allowed_models", []string(token.AllowedModels))

		c.Next()
	}
}
