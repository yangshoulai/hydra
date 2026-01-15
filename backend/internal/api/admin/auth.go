package admin

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/service/admin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService *admin.AuthService
	logger      *slog.Logger
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authService *admin.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

// Login 管理员登录
// @Summary 管理员登录
// @Description 管理员用户登录
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body admin.LoginRequest true "登录请求"
// @Success 200 {object} admin.LoginResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /admin/api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req admin.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid login request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		if err == admin.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid username or password",
			})
			return
		}
		if err == admin.ErrUserDisabled {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "user is disabled",
			})
			return
		}

		h.logger.Error("login failed",
			slog.String("username", req.Username),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "login failed",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Logout 管理员登出
// @Summary 管理员登出
// @Description 管理员用户登出
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /admin/api/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "missing authentication token",
		})
		return
	}

	if err := h.authService.Logout(c.Request.Context(), token); err != nil {
		h.logger.Error("logout failed",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "logout failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "logged out successfully",
	})
}

// Me 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录的管理员信息
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} admin.AdminUserDetail
// @Failure 401 {object} map[string]interface{}
// @Router /admin/api/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "missing authentication token",
		})
		return
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), token)
	if err != nil {
		if err == admin.ErrInvalidToken {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		h.logger.Error("failed to get current user",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get user info",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// RegisterRoutes 注册认证路由
func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)
		auth.GET("/me", h.Me)
	}
}

// extractToken 从请求中提取令牌
func extractToken(c *gin.Context) string {
	// 从 Authorization header 提取
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// 格式: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// 从 query 参数提取 (备选方案)
	if token := c.Query("token"); token != "" {
		return token
	}

	return ""
}
