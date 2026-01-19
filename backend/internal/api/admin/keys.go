package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/admin"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

// KeyHandler Key 管理处理器
type KeyHandler struct {
	keyRepo           *repository.KeyRepository
	channelRepo       *repository.ChannelRepository
	healthCheckSvc    *admin.HealthCheckService
	circuitManager    *circuit.Manager
	logger            *slog.Logger
}

// NewKeyHandler 创建 Key 处理器
func NewKeyHandler(
	keyRepo *repository.KeyRepository,
	channelRepo *repository.ChannelRepository,
	healthCheckSvc *admin.HealthCheckService,
	circuitManager *circuit.Manager,
	logger *slog.Logger,
) *KeyHandler {
	return &KeyHandler{
		keyRepo:        keyRepo,
		channelRepo:    channelRepo,
		healthCheckSvc: healthCheckSvc,
		circuitManager: circuitManager,
		logger:         logger,
	}
}

// CreateKeyRequest 创建 Key 请求
type CreateKeyRequest struct {
	ChannelID uint   `json:"channel_id" binding:"required"`
	KeyValue  string `json:"key_value" binding:"required,max=500"`
	Remark    string `json:"remark" binding:"omitempty,max=200"`
}

// BatchCreateKeysRequest 批量创建 Key 请求
type BatchCreateKeysRequest struct {
	ChannelID uint     `json:"channel_id" binding:"required"`
	KeyValues []string `json:"key_values" binding:"required,min=1,max=100"`
	Remark    string   `json:"remark" binding:"omitempty,max=200"`
}

// BatchCreateKeysResponse 批量创建 Key 响应
type BatchCreateKeysResponse struct {
	SuccessCount int      `json:"success_count"`
	FailedCount  int      `json:"failed_count"`
	Keys         []string `json:"keys,omitempty"`
	FailedKeys   []string `json:"failed_keys,omitempty"`
}

// CreateKey 添加 Key
// @Summary 添加 Key
// @Description 为指定渠道添加新的 API Key
// @Tags Key 管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateKeyRequest true "创建 Key 请求"
// @Success 201 {object} models.Key
// @Failure 400 {object} map[string]interface{}
// @Router /admin/api/keys [post]
func (h *KeyHandler) CreateKey(c *gin.Context) {
	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid create key request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// 验证渠道是否存在
	channel, err := h.channelRepo.FindByID(c.Request.Context(), req.ChannelID)
	if err != nil {
		h.logger.Error("failed to find channel",
			slog.Uint64("channel_id", uint64(req.ChannelID)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find channel",
		})
		return
	}

	if channel == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "channel not found",
		})
		return
	}

	// 创建 Key 对象
	key := &models.Key{
		ChannelID: req.ChannelID,
		KeyValue:  req.KeyValue,
		Status:    "active",
		Remark:    req.Remark,
	}

	// 保存到数据库
	if err := h.keyRepo.Create(c.Request.Context(), key); err != nil {
		h.logger.Error("failed to create key",
			slog.Uint64("channel_id", uint64(req.ChannelID)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create key",
		})
		return
	}

	h.logger.Info("key created",
		slog.Uint64("key_id", uint64(key.ID)),
		slog.Uint64("channel_id", uint64(req.ChannelID)),
	)

	c.JSON(http.StatusCreated, key)
}

// BatchCreateKeys 批量添加 Keys
// @Summary 批量添加 Keys
// @Description 为指定渠道批量添加多个 API Key
// @Tags Key 管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body BatchCreateKeysRequest true "批量创建 Keys 请求"
// @Success 200 {object} BatchCreateKeysResponse
// @Failure 400 {object} map[string]interface{}
// @Router /admin/api/keys/batch [post]
func (h *KeyHandler) BatchCreateKeys(c *gin.Context) {
	var req BatchCreateKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid batch create keys request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// 验证渠道是否存在
	channel, err := h.channelRepo.FindByID(c.Request.Context(), req.ChannelID)
	if err != nil {
		h.logger.Error("failed to find channel",
			slog.Uint64("channel_id", uint64(req.ChannelID)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find channel",
		})
		return
	}

	if channel == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "channel not found",
		})
		return
	}

	response := &BatchCreateKeysResponse{
		SuccessCount: 0,
		FailedCount:  0,
		Keys:         []string{},
		FailedKeys:   []string{},
	}

	// 批量创建 Keys
	for _, keyValue := range req.KeyValues {
		// 创建 Key 对象
		key := &models.Key{
			ChannelID: req.ChannelID,
			KeyValue:  keyValue,
			Status:    "active",
			Remark:    req.Remark,
		}

		// 保存到数据库
		if err := h.keyRepo.Create(c.Request.Context(), key); err != nil {
			h.logger.Warn("failed to create key",
				slog.Uint64("channel_id", uint64(req.ChannelID)),
				slog.String("error", err.Error()),
			)
			response.FailedCount++
			response.FailedKeys = append(response.FailedKeys, keyValue)
		} else {
			response.SuccessCount++
			response.Keys = append(response.Keys, keyValue)
		}
	}

	h.logger.Info("batch create keys completed",
		slog.Uint64("channel_id", uint64(req.ChannelID)),
		slog.Int("total", len(req.KeyValues)),
		slog.Int("success", response.SuccessCount),
		slog.Int("failed", response.FailedCount),
	)

	c.JSON(http.StatusOK, response)
}

// DeleteKey 删除 Key
// @Summary 删除 Key
// @Description 删除指定的 API Key
// @Tags Key 管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Key ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/keys/{id} [delete]
func (h *KeyHandler) DeleteKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid key id",
		})
		return
	}

	// 先检查 Key 是否存在
	key, err := h.keyRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("failed to find key",
			slog.Uint64("key_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find key",
		})
		return
	}

	if key == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "key not found",
		})
		return
	}

	// 执行删除
	if err := h.keyRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		h.logger.Error("failed to delete key",
			slog.Uint64("key_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete key",
		})
		return
	}

	// 清理熔断器缓存
	if h.circuitManager != nil {
		h.circuitManager.RemoveKeyBreaker(uint(id))
	}

	h.logger.Info("key deleted",
		slog.Uint64("key_id", id),
		slog.Uint64("channel_id", uint64(key.ChannelID)),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "key deleted successfully",
	})
}

// ResetKeyRequest 重置 Key 请求
type ResetKeyRequest struct {
	Status string `json:"status" binding:"omitempty,oneof=active cooling disabled"`
}

// ResetKeyStatus 重置 Key 状态
// @Summary 重置 Key 状态
// @Description 手动重置 Key 的状态(从冷却或禁用恢复到激活)
// @Tags Key 管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Key ID"
// @Param request body ResetKeyRequest false "重置请求"
// @Success 200 {object} models.Key
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/keys/{id} [patch]
func (h *KeyHandler) ResetKeyStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid key id",
		})
		return
	}

	var req ResetKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid reset key request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// 查询 Key
	key, err := h.keyRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("failed to find key",
			slog.Uint64("key_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find key",
		})
		return
	}

	if key == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "key not found",
		})
		return
	}

	// 如果没有指定状态，默认重置为 active
	targetStatus := req.Status
	if targetStatus == "" {
		targetStatus = "active"
	}

	// 更新状态
	key.Status = targetStatus
	key.CoolingAt = nil // 清除冷却时间

	if err := h.keyRepo.Update(c.Request.Context(), key); err != nil {
		h.logger.Error("failed to update key status",
			slog.Uint64("key_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update key status",
		})
		return
	}

	// 同时重置熔断器状态
	if targetStatus == "active" {
		h.circuitManager.ResetKey(uint(id))
	}

	h.logger.Info("key status reset",
		slog.Uint64("key_id", id),
		slog.String("status", targetStatus),
	)

	c.JSON(http.StatusOK, key)
}

// TestChannelKeys 测试渠道所有 Key 的健康状态
// @Summary 测试渠道 Key 健康状态
// @Description 并发测试指定渠道所有 Key 的健康状态
// @Tags Key 管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "渠道ID"
// @Success 200 {object} admin.ChannelHealthCheckResult
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/channels/{id}/test-keys [post]
func (h *KeyHandler) TestChannelKeys(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid channel id",
		})
		return
	}

	// 执行健康检查
	result, err := h.healthCheckSvc.CheckChannelHealth(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("failed to check channel health",
			slog.Uint64("channel_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to check channel health",
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "channel not found",
		})
		return
	}

	h.logger.Info("channel health check completed",
		slog.Uint64("channel_id", id),
		slog.Int("total_keys", result.TotalKeys),
		slog.Int("healthy_keys", result.HealthyKeys),
	)

	c.JSON(http.StatusOK, result)
}

// TestSingleKey 测试单个 Key 的健康状态
// @Summary 测试单个 Key 健康状态
// @Description 测试指定 Key 的健康状态
// @Tags Key 管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Key ID"
// @Success 200 {object} admin.KeyHealthResult
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/keys/{id}/test [post]
func (h *KeyHandler) TestSingleKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid key id",
		})
		return
	}

	// 查询 Key
	key, err := h.keyRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("failed to find key",
			slog.Uint64("key_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find key",
		})
		return
	}

	if key == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "key not found",
		})
		return
	}

	// 查询渠道
	channel, err := h.channelRepo.FindByID(c.Request.Context(), key.ChannelID)
	if err != nil {
		h.logger.Error("failed to find channel",
			slog.Uint64("channel_id", uint64(key.ChannelID)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find channel",
		})
		return
	}

	if channel == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "channel not found",
		})
		return
	}

	// 执行健康检查
	result := h.healthCheckSvc.CheckSingleKey(c.Request.Context(), key, channel)

	h.logger.Info("single key health check completed",
		slog.Uint64("key_id", id),
		slog.String("status", result.Status),
	)

	c.JSON(http.StatusOK, result)
}

// RegisterRoutes 注册 Key 管理路由
func (h *KeyHandler) RegisterRoutes(r *gin.RouterGroup) {
	keys := r.Group("/keys")
	{
		keys.POST("", h.CreateKey)
		keys.POST("/batch", h.BatchCreateKeys)
		keys.DELETE("/:id", h.DeleteKey)
		keys.PATCH("/:id", h.ResetKeyStatus)
		keys.POST("/:id/test", h.TestSingleKey)
	}

	// 渠道测活路由
	r.POST("/channels/:id/test-keys", h.TestChannelKeys)
}
