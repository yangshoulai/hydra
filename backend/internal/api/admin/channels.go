package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	"gorm.io/gorm"
)

// ChannelHandler 渠道管理处理器
type ChannelHandler struct {
	channelRepo     *repository.ChannelRepository
	modelConfigRepo *repository.ChannelModelConfigRepository
	keyRepo         *repository.KeyRepository
	db              *gorm.DB
	logger          *slog.Logger
	circuitManager  *circuit.Manager
}

// NewChannelHandler 创建渠道处理器
func NewChannelHandler(
	channelRepo *repository.ChannelRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	keyRepo *repository.KeyRepository,
	db *gorm.DB,
	logger *slog.Logger,
	circuitManager *circuit.Manager,
) *ChannelHandler {
	return &ChannelHandler{
		channelRepo:     channelRepo,
		modelConfigRepo: modelConfigRepo,
		keyRepo:         keyRepo,
		db:              db,
		logger:          logger,
		circuitManager:  circuitManager,
	}
}

// ChannelListRequest 渠道列表请求
type ChannelListRequest struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=1000"`
	Name      string `form:"name" binding:"omitempty,max=100"`                                 // 名称过滤
	BaseURL   string `form:"base_url" binding:"omitempty,max=500"`                             // Base URL 过滤
	Status    string `form:"status" binding:"omitempty,oneof=active disabled"`                 // 状态过滤
	SortBy    string `form:"sort_by" binding:"omitempty,oneof=id name priority weight status"` // 排序字段
	SortOrder string `form:"sort_order" binding:"omitempty,oneof=asc desc"`                    // 排序方向
}

// ChannelListResponse 渠道列表响应
type ChannelListResponse struct {
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
	Items    []ChannelWithModelCount `json:"items"`
}

// ChannelWithModelCount 带模型数量的渠道信息
type ChannelWithModelCount struct {
	*models.Channel
	ModelCount      int                        `json:"model_count"`
	ModelStats      *repository.ModelConfigStatusCount `json:"model_stats"`
	KeyStats        *repository.KeyStatusCount `json:"key_stats"`
}

// CreateChannelRequest 创建渠道请求
type CreateChannelRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	BaseURL     string `json:"base_url" binding:"required,url,max=500"`
	Priority    int    `json:"priority" binding:"omitempty,min=1,max=1000"`
	Weight      int    `json:"weight" binding:"omitempty,min=1,max=1000"`
	Status      string `json:"status" binding:"omitempty,oneof=active disabled"`
	Description string `json:"description" binding:"omitempty,max=500"`
	SyncEnabled *bool  `json:"sync_enabled" binding:"omitempty"`
}

// UpdateChannelRequest 更新渠道请求
type UpdateChannelRequest struct {
	Name        string `json:"name" binding:"omitempty,max=100"`
	BaseURL     string `json:"base_url" binding:"omitempty,url,max=500"`
	Priority    int    `json:"priority" binding:"omitempty,min=1,max=1000"`
	Weight      int    `json:"weight" binding:"omitempty,min=1,max=1000"`
	Status      string `json:"status" binding:"omitempty,oneof=active disabled"`
	Description string `json:"description" binding:"omitempty,max=500"`
	SyncEnabled *bool  `json:"sync_enabled" binding:"omitempty"`
}

// ListChannels 获取渠道列表(分页)
// @Summary 获取渠道列表
// @Description 分页获取所有渠道
// @Tags 渠道管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} ChannelListResponse
// @Failure 400 {object} map[string]interface{}
// @Router /admin/api/channels [get]
func (h *ChannelHandler) ListChannels(c *gin.Context) {
	var req ChannelListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("无效的渠道列表请求",
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
	filter := &repository.ChannelFilter{
		Name:    req.Name,
		BaseURL: req.BaseURL,
		Status:  req.Status,
	}

	// 构建排序选项
	var sortOpts *repository.ChannelSortOptions
	if req.SortBy != "" {
		sortOpts = &repository.ChannelSortOptions{
			Field:     req.SortBy,
			Direction: req.SortOrder,
		}
		if sortOpts.Direction == "" {
			sortOpts.Direction = "asc" // 默认升序
		}
	}

	// 查询渠道列表
	channels, total, err := h.channelRepo.ListWithFilter(c.Request.Context(), offset, req.PageSize, filter, sortOpts)
	if err != nil {
		h.logger.Error("查询渠道列表失败",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list channels",
		})
		return
	}

	// 查询每个渠道的模型配置数量和密钥统计
	result := make([]ChannelWithModelCount, 0, len(channels))
	for _, channel := range channels {
		// 查询该渠道的模型配置统计
		modelStats, err := h.modelConfigRepo.CountByChannelIDAndStatus(c.Request.Context(), channel.ID)
		if err != nil {
			h.logger.Warn("查询渠道模型统计失败",
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("error", err.Error()),
			)
			modelStats = &repository.ModelConfigStatusCount{}
		}
		modelCount := int(modelStats.Active + modelStats.Disabled + modelStats.NonExist)

		// 查询该渠道的密钥统计
		keyStats, err := h.keyRepo.CountByChannelIDAndStatus(c.Request.Context(), channel.ID)
		if err != nil {
			h.logger.Warn("查询渠道密钥统计失败",
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("error", err.Error()),
			)
			keyStats = &repository.KeyStatusCount{}
		}

		result = append(result, ChannelWithModelCount{
			Channel:    channel,
			ModelCount: modelCount,
			ModelStats: modelStats,
			KeyStats:   keyStats,
		})
	}

	c.JSON(http.StatusOK, ChannelListResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    result,
	})
}

// CreateChannel 创建渠道
// @Summary 创建渠道
// @Description 创建新的渠道
// @Tags 渠道管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateChannelRequest true "创建渠道请求"
// @Success 201 {object} models.Channel
// @Failure 400 {object} map[string]interface{}
// @Router /admin/api/channels [post]
func (h *ChannelHandler) CreateChannel(c *gin.Context) {
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("无效的创建渠道请求",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// 创建渠道对象
	channel := &models.Channel{
		Name:        req.Name,
		BaseURL:     req.BaseURL,
		Priority:    req.Priority,
		Weight:      req.Weight,
		Status:      req.Status,
		Description: req.Description,
	}

	// 设置默认值
	if channel.Priority == 0 {
		channel.Priority = 100
	}
	if channel.Weight == 0 {
		channel.Weight = 100
	}
	if channel.Status == "" {
		channel.Status = "active"
	}
	// 默认开启自动同步（实际执行仍受全局开关控制）
	if req.SyncEnabled == nil {
		channel.SyncEnabled = true
	} else {
		channel.SyncEnabled = *req.SyncEnabled
	}

	// 保存到数据库
	if err := h.channelRepo.Create(c.Request.Context(), channel); err != nil {
		h.logger.Error("创建渠道失败",
			slog.String("name", req.Name),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create channel",
		})
		return
	}

	h.logger.Info("渠道已创建",
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("name", channel.Name),
	)

	c.JSON(http.StatusCreated, channel)
}

// GetChannel 获取渠道详情
// @Summary 获取渠道详情
// @Description 根据ID获取渠道详细信息
// @Tags 渠道管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "渠道ID"
// @Success 200 {object} models.Channel
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/channels/{id} [get]
func (h *ChannelHandler) GetChannel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid channel id",
		})
		return
	}

	channel, err := h.channelRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("获取渠道失败",
			slog.Uint64("channel_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get channel",
		})
		return
	}

	if channel == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "channel not found",
		})
		return
	}

	c.JSON(http.StatusOK, channel)
}

// UpdateChannel 更新渠道
// @Summary 更新渠道
// @Description 更新渠道信息
// @Tags 渠道管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "渠道ID"
// @Param request body UpdateChannelRequest true "更新渠道请求"
// @Success 200 {object} models.Channel
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/channels/{id} [put]
func (h *ChannelHandler) UpdateChannel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid channel id",
		})
		return
	}

	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("无效的更新渠道请求",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// 查询现有渠道
	channel, err := h.channelRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("查找渠道失败",
			slog.Uint64("channel_id", id),
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

	// 更新字段
	oldStatus := channel.Status
	if req.Name != "" {
		channel.Name = req.Name
	}
	if req.BaseURL != "" {
		channel.BaseURL = req.BaseURL
	}
	if req.Priority > 0 {
		channel.Priority = req.Priority
	}
	if req.Weight > 0 {
		channel.Weight = req.Weight
	}
	if req.Status != "" {
		channel.Status = req.Status
	}
	if req.Description != "" {
		channel.Description = req.Description
	}
	if req.SyncEnabled != nil {
		channel.SyncEnabled = *req.SyncEnabled
	}

	// 保存更新
	if err := h.channelRepo.Update(c.Request.Context(), channel); err != nil {
		h.logger.Error("更新渠道失败",
			slog.Uint64("channel_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update channel",
		})
		return
	}

	// 如果渠道被禁用，清理所有相关熔断器
	if oldStatus == "active" && channel.Status == "disabled" {
		if h.circuitManager != nil {
			h.circuitManager.RemoveChannelBreakersAndKeys(uint(id))
			h.logger.Info("渠道已禁用，已清理熔断器",
				slog.Uint64("channel_id", id),
				slog.String("name", channel.Name),
			)
		}
	}

	h.logger.Info("渠道已更新",
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("name", channel.Name),
	)

	c.JSON(http.StatusOK, channel)
}

// DeleteChannel 删除渠道
// @Summary 删除渠道
// @Description 删除指定的渠道
// @Tags 渠道管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "渠道ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/channels/{id} [delete]
func (h *ChannelHandler) DeleteChannel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid channel id",
		})
		return
	}

	// 先检查渠道是否存在
	channel, err := h.channelRepo.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "channel not found",
			})
			return
		}
		h.logger.Error("查找渠道失败",
			slog.Uint64("channel_id", id),
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

	// 执行删除
	if err := h.channelRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		h.logger.Error("删除渠道失败",
			slog.Uint64("channel_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete channel",
		})
		return
	}

	// 清理熔断器缓存
	if h.circuitManager != nil {
		h.circuitManager.RemoveChannelBreakersAndKeys(uint(id))
	}

	h.logger.Info("渠道已删除", slog.Uint64("channel_id", id), slog.String("name", channel.Name))

	c.JSON(http.StatusOK, gin.H{"message": "channel deleted successfully"})
}

// GetChannelsByModel 获取模型关联的渠道列表
func (h *ChannelHandler) GetChannelsByModel(c *gin.Context) {
	modelID := c.Param("id")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "model id is required",
		})
		return
	}

	id, err := strconv.ParseUint(modelID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid model id",
		})
		return
	}

	// 查询模型信息获取模型名称
	var model models.Model
	if err := h.db.WithContext(c.Request.Context()).First(&model, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "model not found",
			})
			return
		}
		h.logger.Error("查找模型失败",
			slog.Uint64("model_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find model",
		})
		return
	}

	// 查询渠道模型配置
	configs, err := h.modelConfigRepo.FindByModelNameWithChannel(c.Request.Context(), model.Name)
	if err != nil {
		h.logger.Error("查找渠道模型配置失败",
			slog.String("model_name", model.Name),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to find channel model configs",
		})
		return
	}

	// 构建响应
	type ChannelModelInfo struct {
		ConfigID      uint     `json:"config_id"`
		ConfigStatus  string   `json:"config_status"`
		ChannelID     uint     `json:"channel_id"`
		ChannelName   string   `json:"channel_name"`
		ChannelStatus string   `json:"channel_status"`
		UpstreamModel string   `json:"upstream_model"`
		EndpointTypes []string `json:"endpoint_types"`
	}

	result := make([]ChannelModelInfo, 0, len(configs))
	for _, config := range configs {
		if config.Channel != nil {
			result = append(result, ChannelModelInfo{
				ConfigID:      config.ID,
				ConfigStatus:  config.Status,
				ChannelID:     config.ChannelID,
				ChannelName:   config.Channel.Name,
				ChannelStatus: config.Channel.Status,
				UpstreamModel: config.UpstreamModel,
				EndpointTypes: config.EndpointTypes,
			})
		}
	}

	c.JSON(http.StatusOK, result)
}

// RegisterRoutes 注册渠道管理路由
func (h *ChannelHandler) RegisterRoutes(r *gin.RouterGroup) {
	channels := r.Group("/channels")
	{
		channels.GET("", h.ListChannels)
		channels.POST("", h.CreateChannel)
		channels.GET("/:id", h.GetChannel)
		channels.PUT("/:id", h.UpdateChannel)
		channels.DELETE("/:id", h.DeleteChannel)
	}

	// 模型相关路由
	r.GET("/models/:id/channels", h.GetChannelsByModel)
}
