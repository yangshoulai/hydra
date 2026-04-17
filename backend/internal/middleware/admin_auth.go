package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	adminService "github.com/yangshoulai/hydra/internal/service/admin"
)

// AdminAuthMiddleware 管理端认证中间件
type AdminAuthMiddleware struct {
	jwtService *adminService.JWTService
}

// NewAdminAuthMiddleware 创建管理端认证中间件
func NewAdminAuthMiddleware(jwtService *adminService.JWTService) *AdminAuthMiddleware {
	return &AdminAuthMiddleware{
		jwtService: jwtService,
	}
}

// Handle 执行管理端 JWT 认证
func (m *AdminAuthMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]
		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}
