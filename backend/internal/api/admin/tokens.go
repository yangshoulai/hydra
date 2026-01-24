package admin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
)

// TokensHandler 访问令牌管理处理器
type TokensHandler struct {
	logger    *slog.Logger
	tokenRepo *repository.AccessTokenRepository
}

// NewTokensHandler 创建访问令牌管理处理器
func NewTokensHandler(
	logger *slog.Logger,
	tokenRepo *repository.AccessTokenRepository,
) *TokensHandler {
	return &TokensHandler{
		logger:    logger,
		tokenRepo: tokenRepo,
	}
}

// TokenListResponse 令牌列表响应
type TokenListResponse struct {
	ID               uint    `json:"id"`
	Name             string  `json:"name"`
	Token            string  `json:"token"`          // 明文令牌（用于复制）
	TokenPreview     string  `json:"token_preview"` // 脱敏令牌（前8位+***+后4位）
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	LastUsedAt       *string `json:"last_used_at,omitempty"`
	ExpiresAt        *string `json:"expires_at,omitempty"` // 过期时间
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
}

// TokenListRequest 令牌列表请求
type TokenListRequest struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=1000"`
	Name      string `form:"name" binding:"omitempty,max=20"`               // 名称过滤
	Status    string `form:"status" binding:"omitempty,oneof=active disabled"` // 状态过滤
	Token     string `form:"token" binding:"omitempty,max=255"`             // 令牌过滤
	SortBy    string `form:"sort_by" binding:"omitempty,oneof=id status created_at last_used_at"` // 排序字段
	SortOrder string `form:"sort_order" binding:"omitempty,oneof=asc desc"`  // 排序方向
}

// TokenListData 令牌列表数据响应
type TokenListData struct {
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Items    []TokenListResponse `json:"items"`
}

// CreateTokenRequest 创建令牌请求
type CreateTokenRequest struct {
	Name      string `json:"name" binding:"required,max=20"`
	ExpiresAt string `json:"expires_at,omitempty"` // 过期时间，格式：2006-01-02 15:04:05，空字符串表示永不过期
}

// CreateTokenResponse 创建令牌响应
type CreateTokenResponse struct {
	ID           uint    `json:"id"`
	Name         string  `json:"name"`
	TokenPreview string  `json:"token_preview"` // 脱敏令牌
	AccessToken  string  `json:"access_token"`  // 明文令牌，仅在创建时返回
	CreatedAt    string  `json:"created_at"`
	Message      string  `json:"message"`
}

// GetTokens 获取令牌列表
// GET /admin/api/tokens
func (h *TokensHandler) GetTokens(c *gin.Context) {
	var req TokenListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("invalid token list request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request parameters",
		})
		return
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	// 计算偏移量
	offset := (req.Page - 1) * req.PageSize

	// 构建过滤条件
	var filter *repository.AccessTokenFilter
	if req.Name != "" || req.Status != "" || req.Token != "" {
		filter = &repository.AccessTokenFilter{
			Name:   req.Name,
			Status: req.Status,
			Token:  req.Token,
		}
	}

	// 构建排序选项
	var sortOpts *repository.AccessTokenSortOptions
	if req.SortBy != "" {
		sortOpts = &repository.AccessTokenSortOptions{
			Field:     req.SortBy,
			Direction: req.SortOrder,
		}
		if sortOpts.Direction == "" {
			sortOpts.Direction = "desc" // 默认降序
		}
	}

	// 查询令牌列表
	tokens, total, err := h.tokenRepo.ListWithFilter(c.Request.Context(), offset, req.PageSize, filter, sortOpts)
	if err != nil {
		h.logger.Error("failed to get tokens", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get tokens",
		})
		return
	}

	// 转换为响应格式
	items := make([]TokenListResponse, 0, len(tokens))
	for _, token := range tokens {
		var lastUsedAt *string
		if token.LastUsedAt != nil {
			formatted := token.LastUsedAt.Format("2006-01-02 15:04:05")
			lastUsedAt = &formatted
		}

		var expiresAt *string
		if token.ExpiresAt != nil {
			formatted := token.ExpiresAt.Format("2006-01-02 15:04:05")
			expiresAt = &formatted
		}

		items = append(items, TokenListResponse{
			ID:               token.ID,
			Name:             token.Name,
			Token:            token.Token,
			TokenPreview:     token.TokenPreview,
			Status:           token.Status,
			CreatedAt:        token.CreatedAt.Format("2006-01-02 15:04:05"),
			LastUsedAt:       lastUsedAt,
			ExpiresAt:        expiresAt,
			PromptTokens:     token.PromptTokens,
			CompletionTokens: token.CompletionTokens,
		})
	}

	c.JSON(http.StatusOK, TokenListData{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	})
}

// CreateToken 创建新令牌
// POST /admin/api/tokens
func (h *TokensHandler) CreateToken(c *gin.Context) {
	var req CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
		return
	}

	// 检查名称是否已存在
	existingTokens, err := h.tokenRepo.List(c.Request.Context())
	if err == nil {
		for _, token := range existingTokens {
			if token.Name == req.Name {
				c.JSON(http.StatusConflict, gin.H{
					"error":   "Token name already exists",
					"message": fmt.Sprintf("Token name '%s' is already in use", req.Name),
				})
				return
			}
		}
	}

	// 生成随机令牌
	accessToken, err := generateAccessToken()
	if err != nil {
		h.logger.Error("failed to generate access token", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate access token",
		})
		return
	}

	// 生成脱敏预览
	tokenPreview := models.MaskToken(accessToken)

	// 解析过期时间
	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid expires_at format",
				"message": "expires_at must be in format: YYYY-MM-DD HH:MM:SS",
			})
			return
		}
		// 验证过期时间必须在未来
		if parsedTime.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid expires_at",
				"message": "expires_at must be in the future",
			})
			return
		}
		expiresAt = &parsedTime
	}

	// 创建令牌记录（存储哈希值和明文）
	token := &models.AccessToken{
		Token:        accessToken,
		TokenHash:    models.HashToken(accessToken),
		TokenPreview: tokenPreview,
		Status:       "active",
		Name:         req.Name,
		ExpiresAt:    expiresAt,
	}

	if err := h.tokenRepo.Create(c.Request.Context(), token); err != nil {
		h.logger.Error("failed to create token",
			slog.String("name", req.Name),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create token",
		})
		return
	}

	h.logger.Info("access token created",
		slog.Int64("token_id", int64(token.ID)),
		slog.String("name", token.Name),
		slog.Bool("has_expiration", expiresAt != nil),
	)

	// 返回明文令牌（仅在创建时返回一次）
	c.JSON(http.StatusCreated, gin.H{
		"data": CreateTokenResponse{
			ID:           token.ID,
			Name:         token.Name,
			TokenPreview: tokenPreview,
			AccessToken:  accessToken, // 明文令牌
			CreatedAt:    token.CreatedAt.Format("2006-01-02 15:04:05"),
			Message:      "Token created successfully",
		},
	})
}

// DeleteToken 删除令牌
// DELETE /admin/api/tokens/:id
func (h *TokensHandler) DeleteToken(c *gin.Context) {
	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid token ID",
		})
		return
	}

	// 检查令牌是否存在
	token, err := h.tokenRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Token not found",
		})
		return
	}

	// 删除令牌
	if err := h.tokenRepo.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("failed to delete token",
			slog.Int64("token_id", int64(id)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete token",
		})
		return
	}

	h.logger.Info("access token deleted",
		slog.Int64("token_id", int64(id)),
		slog.String("name", token.Name),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Token deleted successfully",
	})
}

// ToggleTokenStatus 切换令牌状态
// PATCH /admin/api/tokens/:id/toggle
func (h *TokensHandler) ToggleTokenStatus(c *gin.Context) {
	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid token ID",
		})
		return
	}

	// 获取令牌
	token, err := h.tokenRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Token not found",
		})
		return
	}

	// 切换状态
	newStatus := "active"
	if token.Status == "active" {
		newStatus = "disabled"
	}

	if err := h.tokenRepo.ToggleStatus(c.Request.Context(), id, newStatus); err != nil {
		h.logger.Error("failed to toggle token status",
			slog.Int64("token_id", int64(id)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to toggle token status",
		})
		return
	}

	h.logger.Info("access token status toggled",
		slog.Int64("token_id", int64(id)),
		slog.String("new_status", newStatus),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Token status updated successfully",
		"data": gin.H{
			"id":     token.ID,
			"status": newStatus,
		},
	})
}

// RegisterRoutes 注册路由
func (h *TokensHandler) RegisterRoutes(router *gin.RouterGroup) {
	tokens := router.Group("/tokens")
	{
		tokens.GET("", h.GetTokens)
		tokens.POST("", h.CreateToken)
		tokens.DELETE("/:id", h.DeleteToken)
		tokens.PATCH("/:id/toggle", h.ToggleTokenStatus)
	}
}

// generateAccessToken 生成随机访问令牌
func generateAccessToken() (string, error) {
	// 生成 32 字节随机数据
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// Base64 编码
	accessToken := base64.URLEncoding.EncodeToString(randomBytes)

	// 添加前缀 "hydra-"
	return "hydra-" + accessToken, nil
}
