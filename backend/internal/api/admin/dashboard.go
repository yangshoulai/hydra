package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/service/admin"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

// DashboardHandler 仪表盘处理器
type DashboardHandler struct {
	dashboardService *admin.DashboardService
}

type dashboardStreamFrame struct {
	Metrics  *admin.DashboardMetrics   `json:"metrics"`
	Circuits []circuit.BreakerSnapshot `json:"circuits"`
}

type dashboardMetricsQuery struct {
	QPSRange string `form:"qps_range" binding:"omitempty,oneof=1h 6h 24h"`
}

type clearCircuitRequest struct {
	Kind string `json:"kind" binding:"required,oneof=key model"`
	ID   uint   `json:"id" binding:"required,min=1"`
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
	var query dashboardMetricsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metrics, err := h.dashboardService.GetMetricsWithQPSRange(c.Request.Context(), admin.QPSRange(query.QPSRange))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get dashboard metrics",
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
	var query dashboardMetricsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metrics, err := h.dashboardService.GetQPSMetricsWithRange(c.Request.Context(), admin.QPSRange(query.QPSRange))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get QPS metrics",
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
			"error":   "Failed to get success rate metrics",
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
			"error":   "Failed to get channel health metrics",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetCircuitStatus 获取熔断状态
// GET /admin/api/dashboard/circuits
func (h *DashboardHandler) GetCircuitStatus(c *gin.Context) {
	circuits, err := h.dashboardService.GetCircuitStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get circuit status",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": circuits,
	})
}

// ClearCircuit 手动清除熔断状态
// POST /admin/api/dashboard/circuits/clear
func (h *DashboardHandler) ClearCircuit(c *gin.Context) {
	var req clearCircuitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	result, err := h.dashboardService.ClearCircuit(c.Request.Context(), req.Kind, req.ID)
	if err != nil {
		switch {
		case errors.Is(err, admin.ErrInvalidCircuitKind):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid circuit kind",
				"message": err.Error(),
			})
		case errors.Is(err, admin.ErrCircuitTargetNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Circuit target not found",
				"message": err.Error(),
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to clear circuit status",
				"message": err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "circuit status cleared",
		"data":    result,
	})
}

// StreamMetrics SSE 推送仪表盘指标
// GET /admin/api/dashboard/metrics/stream
func (h *DashboardHandler) StreamMetrics(c *gin.Context) {
	var query dashboardMetricsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "streaming unsupported",
		})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	push := func() bool {
		metrics, err := h.dashboardService.GetMetricsWithQPSRange(c.Request.Context(), admin.QPSRange(query.QPSRange))
		if err != nil {
			return writeSSEEvent(c, flusher, "error", map[string]string{"message": err.Error()})
		}

		circuits, err := h.dashboardService.GetCircuitStatus(c.Request.Context())
		if err != nil {
			return writeSSEEvent(c, flusher, "error", map[string]string{"message": err.Error()})
		}

		frame := dashboardStreamFrame{
			Metrics:  metrics,
			Circuits: circuits,
		}
		return writeSSEEvent(c, flusher, "metrics", frame)
	}

	if !push() {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if !push() {
				return
			}
		}
	}
}

func writeSSEEvent(c *gin.Context, flusher http.Flusher, event string, data any) bool {
	payload, err := json.Marshal(data)
	if err != nil {
		return false
	}

	if _, err := fmt.Fprintf(c.Writer, "event: %s\n", event); err != nil {
		return false
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", payload); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

// RegisterRoutes 注册路由
func (h *DashboardHandler) RegisterRoutes(router *gin.RouterGroup) {
	dashboard := router.Group("/dashboard")
	{
		dashboard.GET("/circuits", h.GetCircuitStatus)
		dashboard.POST("/circuits/clear", h.ClearCircuit)
		metrics := dashboard.Group("/metrics")
		{
			metrics.GET("", h.GetMetrics)
			metrics.GET("/stream", h.StreamMetrics)
			metrics.GET("/qps", h.GetQPSMetrics)
			metrics.GET("/success-rate", h.GetSuccessRateMetrics)
			metrics.GET("/channel-health", h.GetChannelHealthMetrics)
		}
	}
}
