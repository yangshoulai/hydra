package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/service/admin"
)

// ModelHandler 统一模型处理器
type ModelHandler struct {
	modelService *admin.ModelService
	logger        *slog.Logger
}

// NewModelHandler 创建统一模型处理器
func NewModelHandler(
	modelService *admin.ModelService,
	logger *slog.Logger,
) *ModelHandler {
	return &ModelHandler{
		modelService: modelService,
		logger:        logger,
	}
}

// CreateModel 创建统一模型
// @Summary 创建统一模型
// @Description 创建新的统一模型
// @Tags 模型管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body admin.CreateModelRequest true "创建请求"
// @Success 201 {object} models.Model
// @Failure 400 {object} map[string]interface{}
// @Router /admin/api/models [post]
func (h *ModelHandler) CreateModel(c *gin.Context) {
	var req admin.CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid create model request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	model, err := h.modelService.Create(c.Request.Context(), req)
	if err != nil {
		if err == admin.ErrModelNameExists {
			c.JSON(http.StatusConflict, gin.H{
				"error": "model name already exists",
			})
			return
		}
		if err == admin.ErrInvalidInput {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid input",
			})
			return
		}
		h.logger.Error("failed to create model",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create model",
		})
		return
	}

	// 显式类型断言以使用 models 包
	var _ models.Model = *model

	c.JSON(http.StatusCreated, model)
}

// UpdateModel 更新统一模型
// @Summary 更新统一模型
// @Description 更新统一模型信息
// @Tags 模型管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模型ID"
// @Param request body admin.UpdateModelRequest true "更新请求"
// @Success 200 {object} models.Model
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/models/{id} [put]
func (h *ModelHandler) UpdateModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid model id",
		})
		return
	}

	var req admin.UpdateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid update model request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	model, err := h.modelService.Update(c.Request.Context(), uint(id), req)
	if err != nil {
		if err == admin.ErrModelNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "model not found",
			})
			return
		}
		if err == admin.ErrModelNameExists {
			c.JSON(http.StatusConflict, gin.H{
				"error": "model name already exists",
			})
			return
		}
		h.logger.Error("failed to update model",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update model",
		})
		return
	}

	c.JSON(http.StatusOK, model)
}

// DeleteModel 删除统一模型
// @Summary 删除统一模型
// @Description 删除指定的统一模型
// @Tags 模型管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模型ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/models/{id} [delete]
func (h *ModelHandler) DeleteModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid model id",
		})
		return
	}

	err = h.modelService.Delete(c.Request.Context(), uint(id))
	if err != nil {
		if err == admin.ErrModelNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "model not found",
			})
			return
		}
		if err == admin.ErrModelInUse {
			c.JSON(http.StatusConflict, gin.H{
				"error": "model is in use by channel configurations",
			})
			return
		}
		h.logger.Error("failed to delete model",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete model",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "model deleted successfully",
	})
}

// ListModels 查询统一模型列表
// @Summary 查询统一模型列表
// @Description 获取所有统一模型
// @Tags 模型管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.Model
// @Router /admin/api/models [get]
func (h *ModelHandler) ListModels(c *gin.Context) {
	modelList, err := h.modelService.List(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list models",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list models",
		})
		return
	}

	c.JSON(http.StatusOK, modelList)
}

// GetModel 获取单个统一模型
// @Summary 获取统一模型
// @Description 根据ID获取统一模型详情
// @Tags 模型管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模型ID"
// @Success 200 {object} models.Model
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/models/{id} [get]
func (h *ModelHandler) GetModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid model id",
		})
		return
	}

	model, err := h.modelService.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("failed to get model",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get model",
		})
		return
	}

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model not found",
		})
		return
	}

	c.JSON(http.StatusOK, model)
}

// RegisterRoutes 注册路由
func (h *ModelHandler) RegisterRoutes(r *gin.RouterGroup) {
	modelGroup := r.Group("/models")
	{
		modelGroup.GET("", h.ListModels)
		modelGroup.GET("/:id", h.GetModel)
		modelGroup.POST("", h.CreateModel)
		modelGroup.PUT("/:id", h.UpdateModel)
		modelGroup.DELETE("/:id", h.DeleteModel)
	}
}
