package admin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/service/admin"
)

// LogHandler 日志处理器
type LogHandler struct {
	logQueryService *admin.LogQueryService
	logger          *slog.Logger
}

// NewLogHandler 创建日志处理器
func NewLogHandler(
	logQueryService *admin.LogQueryService,
	logger *slog.Logger,
) *LogHandler {
	return &LogHandler{
		logQueryService: logQueryService,
		logger:          logger,
	}
}

// QueryLogs 查询日志列表
// @Summary 查询日志列表
// @Description 支持多条件筛选和分页查询
// @Tags 日志管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param trace_id query string false "Trace ID"
// @Param access_token query string false "访问令牌"
// @Param requested_model query string false "请求的模型"
// @Param channel_id query int false "渠道ID"
// @Param status_code query int false "状态码"
// @Param is_success query bool false "是否成功"
// @Param start_time query string false "开始时间" format(datetime)
// @Param end_time query string false "结束时间" format(datetime)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(20)
// @Param order_by query string false "排序字段" default(created_at)
// @Param order query string false "排序方向" default(desc)
// @Success 200 {object} admin.LogQueryResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /admin/api/logs [get]
func (h *LogHandler) QueryLogs(c *gin.Context) {
	var req admin.LogQueryRequest

	// 绑定查询参数
	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("invalid log query request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// 解析时间参数
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err == nil {
			req.StartTime = &startTime
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err == nil {
			req.EndTime = &endTime
		}
	}

	// 执行查询
	result, err := h.logQueryService.Query(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to query logs",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to query logs",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetLogByTraceID 根据TraceID获取日志详情
// @Summary 获取日志详情
// @Description 根据TraceID获取单条日志的详细信息
// @Tags 日志管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param traceId path string true "Trace ID"
// @Success 200 {object} models.RequestLog
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /admin/api/logs/{traceId} [get]
func (h *LogHandler) GetLogByTraceID(c *gin.Context) {
	traceID := c.Param("traceId")
	if traceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "trace_id is required",
		})
		return
	}

	log, err := h.logQueryService.GetByTraceID(c.Request.Context(), traceID)
	if err != nil {
		h.logger.Error("failed to get log by trace_id",
			slog.String("trace_id", traceID),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get log",
		})
		return
	}

	if log == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "log not found",
		})
		return
	}

	c.JSON(http.StatusOK, log)
}

// GetStatistics 获取日志统计信息
// @Summary 获取统计信息
// @Description 获取指定时间范围内的日志统计信息
// @Tags 日志管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param start_time query string false "开始时间" format(datetime)
// @Param end_time query string false "结束时间" format(datetime)
// @Success 200 {object} admin.LogStatistics
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /admin/api/logs/statistics [get]
func (h *LogHandler) GetStatistics(c *gin.Context) {
	var startTime, endTime *time.Time

	// 解析时间参数
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err == nil {
			startTime = &parsedTime
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err == nil {
			endTime = &parsedTime
		}
	}

	// 默认查询最近24小时
	if startTime == nil && endTime == nil {
		now := time.Now()
		dayAgo := now.Add(-24 * time.Hour)
		startTime = &dayAgo
		endTime = &now
	}

	stats, err := h.logQueryService.GetStatistics(c.Request.Context(), startTime, endTime)
	if err != nil {
		h.logger.Error("failed to get statistics",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// RegisterRoutes 注册日志路由
func (h *LogHandler) RegisterRoutes(r *gin.RouterGroup) {
	logs := r.Group("/logs")
	{
		logs.GET("", h.QueryLogs)
		logs.GET("/statistics", h.GetStatistics)
		logs.GET("/:traceId", h.GetLogByTraceID)
	}
}
