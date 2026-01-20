package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
)

// EndpointHandler 端点管理处理器
type EndpointHandler struct{}

// NewEndpointHandler 创建端点处理器
func NewEndpointHandler() *EndpointHandler {
	return &EndpointHandler{}
}

// GetEndpoints 获取所有支持的端点类型
// @Summary 获取端点列表
// @Description 获取系统支持的所有端点类型
// @Tags 端点
// @Produce json
// @Success 200 {array} endpoint.EndpointInfo
// @Router /admin/api/endpoints [get]
func (h *EndpointHandler) GetEndpoints(c *gin.Context) {
	endpoints := endpoint.GetAllInfo()
	c.JSON(http.StatusOK, endpoints)
}
