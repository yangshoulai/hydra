package admin

import (
	"net/http"
	"strconv"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/repository"
)

// LogHandler 日志处理器
type LogHandler struct {
	logger         *slog.Logger
	requestLogRepo *repository.RequestLogRepository
}

// NewLogHandler 创建日志处理器
func NewLogHandler(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
) *LogHandler {
	return &LogHandler{
		logger:         logger,
		requestLogRepo: requestLogRepo,
	}
}

// ListLogs 查询日志列表
// GET /admin/api/logs
func (h *LogHandler) ListLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := &repository.ListMainFilter{
		TraceID:     c.Query("trace_id"),
		AccessToken: c.Query("access_token"),
		Offset:      (page - 1) * pageSize,
		Limit:       pageSize,
	}

	// 解析时间范围
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filter.StartTime = &startTime
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filter.EndTime = &endTime
		}
	}

	// 查询日志
	logs, total, err := h.requestLogRepo.ListMain(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("查询日志失败", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"items":     logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetLogDetail 获取日志详情
// GET /admin/api/logs/:trace_id
func (h *LogHandler) GetLogDetail(c *gin.Context) {
	traceID := c.Param("trace_id")
	if traceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "trace_id is required",
		})
		return
	}

	log, err := h.requestLogRepo.FindMainByTraceID(c.Request.Context(), traceID)
	if err != nil {
		h.logger.Error("查询日志详情失败", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query log detail",
		})
		return
	}

	if log == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Log not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": log,
	})
}

// RegisterRoutes 注册路由
func (h *LogHandler) RegisterRoutes(router *gin.RouterGroup) {
	logs := router.Group("/logs")
	{
		logs.GET("", h.ListLogs)
		logs.GET("/:trace_id", h.GetLogDetail)
	}
}
