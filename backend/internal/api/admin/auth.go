package admin

import (
	"errors"
	"log/slog"
	"net/http"

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
// @Failure 400 {object} map[string]any
// @Failure 401 {object} map[string]any
// @Router /admin/api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req admin.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("无效的登录请求", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
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

		h.logger.Error("登录失败",
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

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 修改当前登录管理员的密码
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body admin.ChangePasswordRequest true "修改密码请求"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 401 {object} map[string]any
// @Router /admin/api/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	// 从 JWT 中间件设置的上下文中获取用户 ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req admin.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("无效的修改密码请求",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), userID.(uint), &req); err != nil {
		if errors.Is(err, admin.ErrInvalidOldPassword) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "当前密码不正确",
			})
			return
		}

		h.logger.Error("修改密码失败", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

// RefreshToken 刷新访问令牌
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌和刷新令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body admin.RefreshTokenRequest true "刷新令牌请求"
// @Success 200 {object} admin.RefreshTokenResponse
// @Failure 400 {object} map[string]any
// @Failure 401 {object} map[string]any
// @Router /admin/api/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req admin.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("无效的刷新令牌请求", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}

	resp, err := h.authService.RefreshToken(c.Request.Context(), &req)
	if err != nil {
		h.logger.Warn("刷新令牌失败", slog.String("error", err.Error()))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
