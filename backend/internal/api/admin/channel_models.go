package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// ChannelModelHandler 渠道模型配置处理器
type ChannelModelHandler struct {
	modelConfigRepo *repository.ChannelModelConfigRepository
	channelRepo     *repository.ChannelRepository
	logger          *slog.Logger
}

// NewChannelModelHandler 创建渠道模型处理器
func NewChannelModelHandler(
	modelConfigRepo *repository.ChannelModelConfigRepository,
	channelRepo *repository.ChannelRepository,
	logger *slog.Logger,
) *ChannelModelHandler {
	return &ChannelModelHandler{
		modelConfigRepo: modelConfigRepo,
		channelRepo:     channelRepo,
		logger:          logger,
	}
}

// CreateModelConfigRequest 创建模型配置请求
type CreateModelConfigRequest struct {
	ChannelID     uint     `json:"channel_id" binding:"required"`
	UnifiedModel  string   `json:"unified_model" binding:"required,max=100"`
	UpstreamModel string   `json:"upstream_model" binding:"required,max=100"`
	EndpointTypes []string `json:"endpoint_types" binding:"omitempty"`
	Remark        string   `json:"remark" binding:"omitempty,max=200"`
}

// UpdateModelConfigRequest 更新模型配置请求
type UpdateModelConfigRequest struct {
	UnifiedModel  string   `json:"unified_model" binding:"omitempty,max=100"`
	UpstreamModel string   `json:"upstream_model" binding:"omitempty,max=100"`
	EndpointTypes []string `json:"endpoint_types" binding:"omitempty"`
	Status        string   `json:"status" binding:"omitempty,oneof=active disabled"`
	Remark        string   `json:"remark" binding:"omitempty,max=200"`
}

// CreateChannelModel 创建渠道模型配置
// @Summary 创建模型配置
// @Description 为渠道创建新的模型映射配置
// @Tags 模型配置
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateModelConfigRequest true "创建模型配置请求"
// @Success 201 {object} models.ChannelModelConfig
// @Failure 400 {object} map[string]interface{}
// @Router /admin/api/channel-models [post]
func (h *ChannelModelHandler) CreateChannelModel(c *gin.Context) {
	var req CreateModelConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("非法的模型配置保存请求",
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
		h.logger.Error("查询渠道异常",
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

	// 创建模型配置对象
	endpointTypes := req.EndpointTypes
	if len(endpointTypes) == 0 {
		endpointTypes = []string{"openai"}
	}

	modelConfig := &models.ChannelModelConfig{
		ChannelID:     req.ChannelID,
		UnifiedModel:  req.UnifiedModel,
		UpstreamModel: req.UpstreamModel,
		EndpointTypes: endpointTypes,
		Status:        "active",
		Remark:        req.Remark,
	}

	// 保存到数据库
	if err := h.modelConfigRepo.Create(c.Request.Context(), modelConfig); err != nil {
		h.logger.Error("保存模型配置异常",
			slog.Uint64("channel_id", uint64(req.ChannelID)),
			slog.String("upstream_model", req.UpstreamModel),
			slog.String("unified_model", req.UnifiedModel),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create model config",
		})
		return
	}

	h.logger.Info("模型配置已保存",
		slog.Uint64("config_id", uint64(modelConfig.ID)),
		slog.Uint64("channel_id", uint64(req.ChannelID)),
		slog.String("unified_model", req.UnifiedModel),
		slog.String("upstream_model", req.UpstreamModel),
	)

	c.JSON(http.StatusCreated, modelConfig)
}

// UpdateChannelModel 更新渠道模型配置
// @Summary 更新模型配置
// @Description 更新指定的模型映射配置
// @Tags 模型配置
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "配置ID"
// @Param request body UpdateModelConfigRequest true "更新模型配置请求"
// @Success 200 {object} models.ChannelModelConfig
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/channel-models/{id} [put]
func (h *ChannelModelHandler) UpdateChannelModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid config id",
		})
		return
	}

	var req UpdateModelConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("非法的模型配置更新请求",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// 查询现有配置
	modelConfig, err := h.modelConfigRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("查询模型配置异常",
			slog.Uint64("config_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find model config",
		})
		return
	}

	if modelConfig == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model config not found",
		})
		return
	}

	// 更新字段
	if req.UnifiedModel != "" {
		modelConfig.UnifiedModel = req.UnifiedModel
	}
	if req.UpstreamModel != "" {
		modelConfig.UpstreamModel = req.UpstreamModel
	}
	if len(req.EndpointTypes) > 0 {
		modelConfig.EndpointTypes = req.EndpointTypes
	}
	if req.Status != "" {
		modelConfig.Status = req.Status
	}
	if req.Remark != "" {
		modelConfig.Remark = req.Remark
	}

	// 保存更新
	if err := h.modelConfigRepo.Update(c.Request.Context(), modelConfig); err != nil {
		h.logger.Error("模型配置更新异常",
			slog.Uint64("config_id", id),
			slog.Uint64("channel_id", uint64(modelConfig.ChannelID)),
			slog.String("upstream_model", modelConfig.UpstreamModel),
			slog.String("unified_model", modelConfig.UnifiedModel),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update model config",
		})
		return
	}

	h.logger.Info("模型配置已更新",
		slog.Uint64("config_id", uint64(modelConfig.ID)),
		slog.Uint64("channel_id", uint64(modelConfig.ChannelID)),
		slog.String("upstream_model", modelConfig.UpstreamModel),
		slog.String("unified_model", modelConfig.UnifiedModel),
	)

	c.JSON(http.StatusOK, modelConfig)
}

// DeleteChannelModel 删除渠道模型配置
// @Summary 删除模型配置
// @Description 删除指定的模型映射配置
// @Tags 模型配置
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "配置ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/channel-models/{id} [delete]
func (h *ChannelModelHandler) DeleteChannelModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid config id",
		})
		return
	}

	// 先检查配置是否存在
	modelConfig, err := h.modelConfigRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("查询模型配置异常",
			slog.Uint64("config_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find model config",
		})
		return
	}

	if modelConfig == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model config not found",
		})
		return
	}

	// 执行删除
	if err := h.modelConfigRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		h.logger.Error("删除模型配置异常",
			slog.Uint64("config_id", id),
			slog.Uint64("channel_id", uint64(modelConfig.ChannelID)),
			slog.String("upstream_model", modelConfig.UpstreamModel),
			slog.String("unified_model", modelConfig.UnifiedModel),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete model config",
		})
		return
	}

	h.logger.Info("模型配置已删除",
		slog.Uint64("config_id", id),
		slog.Uint64("channel_id", uint64(modelConfig.ChannelID)),
		slog.String("upstream_model", modelConfig.UpstreamModel),
		slog.String("unified_model", modelConfig.UnifiedModel),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "model config deleted successfully",
	})
}

// ToggleChannelModelStatus 切换渠道模型配置状态
// @Summary 切换模型配置状态
// @Description 切换指定模型配置的启用/禁用状态
// @Tags 模型配置
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "配置ID"
// @Success 200 {object} models.ChannelModelConfig
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/channel-models/{id}/toggle-status [patch]
func (h *ChannelModelHandler) ToggleChannelModelStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid config id",
		})
		return
	}

	// 查询现有配置
	modelConfig, err := h.modelConfigRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("查询模型配置异常",
			slog.Uint64("config_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find model config",
		})
		return
	}

	if modelConfig == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model config not found",
		})
		return
	}

	// 切换状态
	if modelConfig.Status == "active" {
		modelConfig.Status = "disabled"
	} else {
		modelConfig.Status = "active"
	}

	// 保存更新
	if err := h.modelConfigRepo.Update(c.Request.Context(), modelConfig); err != nil {
		h.logger.Error("更新模型状态异常",
			slog.Uint64("config_id", id),
			slog.Uint64("channel_id", uint64(modelConfig.ChannelID)),
			slog.String("upstream_model", modelConfig.UpstreamModel),
			slog.String("unified_model", modelConfig.UnifiedModel),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update model config status",
		})
		return
	}

	h.logger.Info("模型配置状态已更新",
		slog.Uint64("config_id", uint64(modelConfig.ID)),
		slog.Uint64("channel_id", uint64(modelConfig.ChannelID)),
		slog.String("upstream_model", modelConfig.UpstreamModel),
		slog.String("unified_model", modelConfig.UnifiedModel),
		slog.String("new_status", modelConfig.Status),
	)

	c.JSON(http.StatusOK, modelConfig)
}

// RegisterRoutes 注册模型配置路由
func (h *ChannelModelHandler) RegisterRoutes(r *gin.RouterGroup) {
	models := r.Group("/channel-models")
	{
		models.POST("", h.CreateChannelModel)
		models.PUT("/:id", h.UpdateChannelModel)
		models.DELETE("/:id", h.DeleteChannelModel)
		models.PATCH("/:id/toggle-status", h.ToggleChannelModelStatus)
	}
}
