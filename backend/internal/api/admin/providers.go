package admin

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/service/admin"
)

// ProviderHandler 厂商处理器
type ProviderHandler struct {
	providerService *admin.ProviderService
	logger          *slog.Logger
}

// NewProviderHandler 创建厂商处理器
func NewProviderHandler(
	providerService *admin.ProviderService,
	logger *slog.Logger,
) *ProviderHandler {
	return &ProviderHandler{
		providerService: providerService,
		logger:          logger,
	}
}

// CreateProvider 创建厂商
// @Summary 创建厂商
// @Description 创建新的厂商
// @Tags 厂商管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body admin.CreateProviderRequest true "创建请求"
// @Success 201 {object} models.Provider
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /admin/api/providers [post]
func (h *ProviderHandler) CreateProvider(c *gin.Context) {
	var req admin.CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid create provider request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	provider, err := h.providerService.Create(c.Request.Context(), req)
	if err != nil {
		if err == admin.ErrProviderIdExists {
			c.JSON(http.StatusConflict, gin.H{
				"error": "provider id already exists",
			})
			return
		}
		if err == admin.ErrInvalidInput {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid input",
			})
			return
		}
		h.logger.Error("failed to create provider",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create provider",
		})
		return
	}

	c.JSON(http.StatusCreated, provider)
}

// UpdateProvider 更新厂商
// @Summary 更新厂商
// @Description 更新厂商信息
// @Tags 厂商管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "厂商ID"
// @Param request body admin.UpdateProviderRequest true "更新请求"
// @Success 200 {object} models.Provider
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /admin/api/providers/{id} [put]
func (h *ProviderHandler) UpdateProvider(c *gin.Context) {
	id := c.Param("id")

	var req admin.UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid update provider request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	provider, err := h.providerService.Update(c.Request.Context(), id, req)
	if err != nil {
		if err == admin.ErrProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "provider not found",
			})
			return
		}
		h.logger.Error("failed to update provider",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update provider",
		})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// DeleteProvider 删除厂商
// @Summary 删除厂商
// @Description 删除指定的厂商
// @Tags 厂商管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "厂商ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /admin/api/providers/{id} [delete]
func (h *ProviderHandler) DeleteProvider(c *gin.Context) {
	id := c.Param("id")

	err := h.providerService.Delete(c.Request.Context(), id)
	if err != nil {
		if err == admin.ErrProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "provider not found",
			})
			return
		}
		if err == admin.ErrProviderInUse {
			c.JSON(http.StatusConflict, gin.H{
				"error": "provider is in use by models",
			})
			return
		}
		h.logger.Error("failed to delete provider",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete provider",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "provider deleted successfully",
	})
}

// ListProviders 查询厂商列表
// @Summary 查询厂商列表
// @Description 获取所有厂商
// @Tags 厂商管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.Provider
// @Router /admin/api/providers [get]
func (h *ProviderHandler) ListProviders(c *gin.Context) {
	providers, err := h.providerService.List(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list providers",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list providers",
		})
		return
	}

	c.JSON(http.StatusOK, providers)
}

// GetProvider 获取单个厂商
// @Summary 获取厂商
// @Description 根据ID获取厂商详情
// @Tags 厂商管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "厂商ID"
// @Success 200 {object} models.Provider
// @Failure 404 {object} map[string]interface{}
// @Router /admin/api/providers/{id} [get]
func (h *ProviderHandler) GetProvider(c *gin.Context) {
	id := c.Param("id")

	provider, err := h.providerService.FindByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("failed to get provider",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get provider",
		})
		return
	}

	if provider == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "provider not found",
		})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// SyncRemoteProviders 同步远程厂商
// @Summary 同步远程厂商
// @Description 从远程服务器获取厂商列表
// @Tags 厂商管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} admin.RemoteProvider
// @Failure 500 {object} map[string]interface{}
// @Router /admin/api/providers/sync [get]
func (h *ProviderHandler) SyncRemoteProviders(c *gin.Context) {
	providers, err := h.providerService.SyncProviders(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to sync remote providers",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to sync remote providers",
		})
		return
	}

	c.JSON(http.StatusOK, providers)
}

// BatchCreateProviders 批量创建厂商
// @Summary 批量创建厂商
// @Description 批量创建多个厂商
// @Tags 厂商管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body []admin.CreateProviderRequest true "厂商列表"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /admin/api/providers/batch [post]
func (h *ProviderHandler) BatchCreateProviders(c *gin.Context) {
	var req []admin.CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid batch create providers request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	createdProviders, errors := h.providerService.BatchCreateProviders(c.Request.Context(), req)

	c.JSON(http.StatusOK, gin.H{
		"created": len(createdProviders),
		"failed":  len(errors),
		"data":    createdProviders,
	})
}

// RegisterRoutes 注册路由
func (h *ProviderHandler) RegisterRoutes(r *gin.RouterGroup) {
	providerGroup := r.Group("/providers")
	{
		providerGroup.GET("", h.ListProviders)
		providerGroup.GET("/:id", h.GetProvider)
		providerGroup.POST("", h.CreateProvider)
		providerGroup.PUT("/:id", h.UpdateProvider)
		providerGroup.DELETE("/:id", h.DeleteProvider)
		providerGroup.GET("/sync", h.SyncRemoteProviders)
		providerGroup.POST("/batch", h.BatchCreateProviders)
	}
}
