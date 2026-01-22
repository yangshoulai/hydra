package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
)

// ApplySyncRequest 应用同步请求
type ApplySyncRequest struct {
	// 添加的模型配置（unified_model为空则使用upstream_model）
	AddModels []ModelConfigItem `json:"add_models" binding:"required"`
	// 删除的模型配置ID列表
	DeleteModelIDs []uint `json:"delete_model_ids"`
	// 更新的模型配置
	UpdateModels []ModelConfigUpdateItem `json:"update_models"`
}

// ModelConfigItem 模型配置项
type ModelConfigItem struct {
	UnifiedModel  string   `json:"unified_model"`
	UpstreamModel string   `json:"upstream_model" binding:"required"`
	EndpointTypes []string `json:"endpoint_types"`
	Remark        string   `json:"remark"`
}

// ModelConfigUpdateItem 模型配置更新项
type ModelConfigUpdateItem struct {
	ID            uint     `json:"id" binding:"required"`
	UnifiedModel  string   `json:"unified_model"`
	UpstreamModel string   `json:"upstream_model" binding:"required"`
	EndpointTypes []string `json:"endpoint_types"`
	Remark        string   `json:"remark"`
}

// ApplySyncResponse 应用同步响应
type ApplySyncResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	AddedCount   int    `json:"added_count"`
	UpdatedCount int    `json:"updated_count"`
	DeletedCount int    `json:"deleted_count"`
}

// ApplyChannelSync 应用渠道模型同步
// @Summary 应用渠道模型同步
// @Description 批量添加、更新、删除模型配置
// @Tags 模型管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "渠道ID"
// @Param request body ApplySyncRequest true "应用同步请求"
// @Success 200 {object} ApplySyncResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /admin/api/channels/{id}/apply-sync [post]
func (h *ModelSyncHandler) ApplyChannelSync(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid channel id",
		})
		return
	}

	var req ApplySyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("非法的渠道模型配置更新请求",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	channelID := uint(id)
	addedCount := 0
	updatedCount := 0
	deletedCount := 0

	// 1. 添加新模型
	for _, item := range req.AddModels {
		// 如果 unified_model 为空，使用上游模型名称
		unifiedModel := item.UnifiedModel
		if unifiedModel == "" {
			unifiedModel = item.UpstreamModel
		}

		// 处理端点类型，如果为空则使用默认值
		endpointTypes := item.EndpointTypes
		if len(endpointTypes) == 0 {
			endpointTypes = []string{"openai"}
		}

		config := &models.ChannelModelConfig{
			ChannelID:     channelID,
			UnifiedModel:  unifiedModel,
			UpstreamModel: item.UpstreamModel,
			EndpointTypes: models.EndpointTypes(endpointTypes),
			Status:        "active",
			Remark:        item.Remark,
		}

		if err := h.modelConfigRepo.Create(c.Request.Context(), config); err != nil {
			h.logger.Error("新建渠道模型配置异常",
				slog.Uint64("channel_id", uint64(channelID)),
				slog.String("upstream_model", item.UpstreamModel),
				slog.String("error", err.Error()),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to create model config",
			})
			return
		}
		addedCount++
	}

	// 2. 更新现有模型
	for _, item := range req.UpdateModels {
		// 如果 unified_model 为空，使用上游模型名称
		unifiedModel := item.UnifiedModel
		if unifiedModel == "" {
			unifiedModel = item.UpstreamModel
		}

		config, err := h.modelConfigRepo.FindByID(c.Request.Context(), item.ID)
		if err != nil {
			h.logger.Error("获取渠道模型配置异常",
				slog.Uint64("config_id", uint64(item.ID)),
				slog.String("error", err.Error()),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to find model config",
			})
			return
		}

		if config == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "model config not found",
			})
			return
		}

		// 验证配置属于该渠道
		if config.ChannelID != channelID {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "model config does not belong to this channel",
			})
			return
		}

		config.UnifiedModel = unifiedModel
		config.UpstreamModel = item.UpstreamModel
		config.Remark = item.Remark

		// 更新端点类型，如果提供了的话
		if len(item.EndpointTypes) > 0 {
			config.EndpointTypes = models.EndpointTypes(item.EndpointTypes)
		}

		if err := h.modelConfigRepo.Update(c.Request.Context(), config); err != nil {
			h.logger.Error("更新渠道模型异常",
				slog.Uint64("config_id", uint64(item.ID)),
				slog.String("error", err.Error()),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to update model config",
			})
			return
		}
		updatedCount++
	}

	// 3. 删除模型配置
	for _, modelID := range req.DeleteModelIDs {
		if err := h.modelConfigRepo.Delete(c.Request.Context(), modelID); err != nil {
			h.logger.Error("删除渠道模型配置异常",
				slog.Uint64("model_id", uint64(modelID)),
				slog.String("error", err.Error()),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to delete model config",
			})
			return
		}
		deletedCount++
	}

	c.JSON(http.StatusOK, ApplySyncResponse{
		Success:      true,
		Message:      "Sync applied successfully",
		AddedCount:   addedCount,
		UpdatedCount: updatedCount,
		DeletedCount: deletedCount,
	})
}
