package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/service/admin"
)

// DashboardHandler 仪表盘处理器
type DashboardHandler struct {
	dashboardService *admin.DashboardService
}

// NewDashboardHandler 创建仪表盘处理器
func NewDashboardHandler(
	dashboardService *admin.DashboardService,
) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
	}
}

// GetMetrics 获取仪表盘指标
// GET /admin/api/dashboard/metrics
func (h *DashboardHandler) GetMetrics(c *gin.Context) {
	metrics, err := h.dashboardService.GetMetrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get dashboard metrics",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetQPSMetrics 获取 QPS 指标
// GET /admin/api/dashboard/metrics/qps
func (h *DashboardHandler) GetQPSMetrics(c *gin.Context) {
	metrics, err := h.dashboardService.GetQPSMetrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get QPS metrics",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetSuccessRateMetrics 获取成功率指标
// GET /admin/api/dashboard/metrics/success-rate
func (h *DashboardHandler) GetSuccessRateMetrics(c *gin.Context) {
	metrics, err := h.dashboardService.GetSuccessRateMetrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get success rate metrics",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetChannelHealthMetrics 获取渠道健康指标
// GET /admin/api/dashboard/metrics/channel-health
func (h *DashboardHandler) GetChannelHealthMetrics(c *gin.Context) {
	metrics, err := h.dashboardService.GetChannelHealthMetrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get channel health metrics",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// RegisterRoutes 注册路由
func (h *DashboardHandler) RegisterRoutes(router *gin.RouterGroup) {
	dashboard := router.Group("/dashboard")
	{
		metrics := dashboard.Group("/metrics")
		{
			metrics.GET("", h.GetMetrics)
			metrics.GET("/qps", h.GetQPSMetrics)
			metrics.GET("/success-rate", h.GetSuccessRateMetrics)
			metrics.GET("/channel-health", h.GetChannelHealthMetrics)
		}
	}
}
