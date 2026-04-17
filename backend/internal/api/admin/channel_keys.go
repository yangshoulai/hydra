package admin

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	adminService "github.com/yangshoulai/hydra/internal/service/admin"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

// ChannelKeyHandler 渠道密钥管理处理器
type ChannelKeyHandler struct {
	channelKeyRepo *repository.ChannelKeyRepository
	channelRepo    *repository.ChannelRepository
	healthCheckSvc *adminService.HealthCheckService
	circuitManager *circuit.CircuitManager
	logger         *slog.Logger
}

// NewChannelKeyHandler 创建渠道密钥处理器
func NewChannelKeyHandler(
	channelKeyRepo *repository.ChannelKeyRepository,
	channelRepo *repository.ChannelRepository,
	healthCheckSvc *adminService.HealthCheckService,
	circuitManager *circuit.CircuitManager,
	logger *slog.Logger,
) *ChannelKeyHandler {
	return &ChannelKeyHandler{
		channelKeyRepo: channelKeyRepo,
		channelRepo:    channelRepo,
		healthCheckSvc: healthCheckSvc,
		circuitManager: circuitManager,
		logger:         logger,
	}
}

// CreateChannelKeyRequest 创建渠道密钥请求
type CreateChannelKeyRequest struct {
	ChannelID       uint   `json:"channel_id" binding:"required"`
	ChannelKeyValue string `json:"channel_key_value" binding:"required,max=500"`
	Remark          string `json:"remark" binding:"omitempty,max=200"`
	ChannelKeyGroup string `json:"channel_key_group" binding:"omitempty,max=100"`
}

// BatchCreateChannelKeysRequest 批量创建渠道密钥请求
type BatchCreateChannelKeysRequest struct {
	ChannelID        uint     `json:"channel_id" binding:"required"`
	ChannelKeyValues []string `json:"channel_key_values" binding:"required,min=1,max=100"`
	Remark           string   `json:"remark" binding:"omitempty,max=200"`
	ChannelKeyGroup  string   `json:"channel_key_group" binding:"omitempty,max=100"`
}

// BatchCreateChannelKeysResponse 批量创建渠道密钥响应
type BatchCreateChannelKeysResponse struct {
	SuccessCount      int      `json:"success_count"`
	FailedCount       int      `json:"failed_count"`
	ChannelKeys       []string `json:"channel_keys,omitempty"`
	FailedChannelKeys []string `json:"failed_channel_keys,omitempty"`
}

// CreateChannelKey 添加渠道密钥
func (h *ChannelKeyHandler) CreateChannelKey(c *gin.Context) {
	var req CreateChannelKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("报文格式不正确", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "报文格式不正确"})
		return
	}

	channel, err := h.channelRepo.FindByID(c.Request.Context(), req.ChannelID)
	if err != nil {
		h.logger.Error("查询渠道异常", slog.Uint64("channel_id", uint64(req.ChannelID)), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询渠道异常"})
		return
	}
	if channel == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "渠道不存在"})
		return
	}

	channelKeyGroup := strings.TrimSpace(req.ChannelKeyGroup)
	if channelKeyGroup == "" {
		channelKeyGroup = "Default"
	}

	channelKey := &models.ChannelKey{
		ChannelID:       req.ChannelID,
		ChannelKeyValue: req.ChannelKeyValue,
		Status:          "active",
		ChannelKeyGroup: channelKeyGroup,
		Remark:          req.Remark,
	}

	if err := h.channelKeyRepo.Create(c.Request.Context(), channelKey); err != nil {
		h.logger.Error("创建渠道密钥异常", slog.Uint64("channel_id", uint64(req.ChannelID)), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建渠道密钥异常"})
		return
	}

	h.logger.Info("渠道密钥已创建",
		slog.Uint64("channel_key_id", uint64(channelKey.ID)),
		slog.Uint64("channel_id", uint64(req.ChannelID)),
	)
	c.JSON(http.StatusCreated, channelKey)
}

// BatchCreateChannelKeys 批量添加渠道密钥
func (h *ChannelKeyHandler) BatchCreateChannelKeys(c *gin.Context) {
	var req BatchCreateChannelKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("请求报文格式不正确", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}

	channel, err := h.channelRepo.FindByID(c.Request.Context(), req.ChannelID)
	if err != nil {
		h.logger.Error("查询渠道异常", slog.Uint64("channel_id", uint64(req.ChannelID)), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询渠道异常"})
		return
	}
	if channel == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "渠道不存在"})
		return
	}

	resp := &BatchCreateChannelKeysResponse{
		ChannelKeys:       []string{},
		FailedChannelKeys: []string{},
	}

	channelKeyGroup := strings.TrimSpace(req.ChannelKeyGroup)
	if channelKeyGroup == "" {
		channelKeyGroup = "Default"
	}

	for _, channelKeyValue := range req.ChannelKeyValues {
		channelKey := &models.ChannelKey{
			ChannelID:       req.ChannelID,
			ChannelKeyValue: channelKeyValue,
			Status:          "active",
			ChannelKeyGroup: channelKeyGroup,
			Remark:          req.Remark,
		}
		if err := h.channelKeyRepo.Create(c.Request.Context(), channelKey); err != nil {
			h.logger.Warn("创建渠道密钥异常", slog.Uint64("channel_id", uint64(req.ChannelID)), slog.String("error", err.Error()))
			resp.FailedCount++
			resp.FailedChannelKeys = append(resp.FailedChannelKeys, channelKeyValue)
		} else {
			resp.SuccessCount++
			resp.ChannelKeys = append(resp.ChannelKeys, channelKeyValue)
		}
	}

	h.logger.Info("批量创建渠道密钥完成",
		slog.Uint64("channel_id", uint64(req.ChannelID)),
		slog.Int("total", len(req.ChannelKeyValues)),
		slog.Int("success", resp.SuccessCount),
		slog.Int("failed", resp.FailedCount),
	)
	c.JSON(http.StatusOK, resp)
}

// DeleteChannelKey 删除渠道密钥
func (h *ChannelKeyHandler) DeleteChannelKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的渠道密钥ID"})
		return
	}

	channelKey, err := h.channelKeyRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("查询渠道密钥异常", slog.Uint64("channel_key_id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询渠道密钥异常"})
		return
	}
	if channelKey == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道密钥不存在"})
		return
	}

	if err := h.channelKeyRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		h.logger.Error("渠道密钥删除异常", slog.Uint64("channel_key_id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "渠道密钥删除异常"})
		return
	}

	h.circuitManager.RemoveKeyBreaker(uint(id))

	h.logger.Info("渠道密钥已删除",
		slog.Uint64("channel_key_id", id),
		slog.Uint64("channel_id", uint64(channelKey.ChannelID)),
	)
	c.JSON(http.StatusOK, gin.H{"message": "渠道密钥删除成功"})
}

// ResetChannelKeyRequest 重置渠道密钥请求
type ResetChannelKeyRequest struct {
	Status string `json:"status" binding:"omitempty,oneof=active inactive"`
}

// ResetChannelKeyStatus 重置渠道密钥状态
func (h *ChannelKeyHandler) ResetChannelKeyStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的渠道密钥ID"})
		return
	}

	var req ResetChannelKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("请求报文格式不正确", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求报文格式不正确"})
		return
	}

	channelKey, err := h.channelKeyRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("查询渠道密钥异常", slog.Uint64("channel_key_id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询渠道密钥异常"})
		return
	}
	if channelKey == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道密钥不存在"})
		return
	}

	targetStatus := req.Status
	if targetStatus == "" {
		targetStatus = "active"
	}

	channelKey.Status = targetStatus

	if err := h.channelKeyRepo.Update(c.Request.Context(), channelKey); err != nil {
		h.logger.Error("更新渠道密钥状态异常", slog.Uint64("channel_key_id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新渠道密钥状态异常"})
		return
	}

	h.circuitManager.RemoveKeyBreaker(uint(id))
	h.logger.Info("重置渠道密钥状态", slog.Uint64("channel_key_id", id), slog.String("status", targetStatus))
	c.JSON(http.StatusOK, channelKey)
}

// TestChannelKeys 测试渠道所有渠道密钥的健康状态
func (h *ChannelKeyHandler) TestChannelKeys(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的渠道ID"})
		return
	}

	result, err := h.healthCheckSvc.CheckChannelHealth(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("检查渠道密钥状态异常", slog.Uint64("channel_id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查渠道密钥状态异常"})
		return
	}
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道不存在"})
		return
	}

	h.logger.Info("渠道密钥检查完成",
		slog.Uint64("channel_id", id),
		slog.Int("total_channel_keys", result.TotalChannelKeys),
		slog.Int("healthy_channel_keys", result.HealthyChannelKeys),
	)
	c.JSON(http.StatusOK, result)
}

// TestSingleChannelKey 测试单个渠道密钥健康状态
func (h *ChannelKeyHandler) TestSingleChannelKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的渠道密钥ID"})
		return
	}

	channelKey, err := h.channelKeyRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("查询渠道密钥异常", slog.Uint64("channel_key_id", id), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询渠道密钥异常"})
		return
	}
	if channelKey == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道密钥不存在"})
		return
	}

	channel, err := h.channelRepo.FindByID(c.Request.Context(), channelKey.ChannelID)
	if err != nil {
		h.logger.Error("查询渠道异常", slog.Uint64("channel_id", uint64(channelKey.ChannelID)), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询渠道异常"})
		return
	}
	if channel == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道不存在"})
		return
	}

	result := h.healthCheckSvc.CheckSingleChannelKey(c.Request.Context(), channelKey, channel)
	h.logger.Info("渠道密钥检查完成", slog.Uint64("channel_key_id", id), slog.String("status", result.Status))
	c.JSON(http.StatusOK, result)
}

// RegisterRoutes 注册渠道密钥管理路由
func (h *ChannelKeyHandler) RegisterRoutes(r *gin.RouterGroup) {
	channelKeys := r.Group("/channel-keys")
	{
		channelKeys.POST("", h.CreateChannelKey)
		channelKeys.POST("/batch", h.BatchCreateChannelKeys)
		channelKeys.DELETE("/:id", h.DeleteChannelKey)
		channelKeys.PATCH("/:id", h.ResetChannelKeyStatus)
		channelKeys.POST("/:id/test", h.TestSingleChannelKey)
	}

	r.POST("/channels/:id/test-channel-keys", h.TestChannelKeys)
}
