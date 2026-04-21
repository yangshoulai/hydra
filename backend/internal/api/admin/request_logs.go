package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/repository"
	"gorm.io/gorm"
)

// RequestLogHandler 请求日志处理器
type RequestLogHandler struct {
	repo   *repository.RequestLogRepository
	logger *slog.Logger
}

// NewRequestLogHandler 创建请求日志处理器
func NewRequestLogHandler(repo *repository.RequestLogRepository, logger *slog.Logger) *RequestLogHandler {
	return &RequestLogHandler{repo: repo, logger: logger}
}

// RequestLogListQuery 请求日志列表查询参数
type RequestLogListQuery struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=500"`

	StartAtMS int64 `form:"start_at_ms" binding:"omitempty,min=0"` // Unix milli
	EndAtMS   int64 `form:"end_at_ms" binding:"omitempty,min=0"`

	Model         string `form:"model" binding:"omitempty,max=100"`
	ChannelID     uint   `form:"channel_id" binding:"omitempty"`
	AccessTokenID uint   `form:"access_token_id" binding:"omitempty"`
	EndpointType  string `form:"endpoint_type" binding:"omitempty,max=50"`
	Status        string `form:"status" binding:"omitempty,oneof=success failed"`
	HasRetry      string `form:"has_retry" binding:"omitempty,oneof=true false"`
	TraceID       string `form:"trace_id" binding:"omitempty,max=64"`
	ClientIP      string `form:"client_ip" binding:"omitempty,max=64"`

	SortBy    string `form:"sort_by" binding:"omitempty,oneof=created_at duration_ms"`
	SortOrder string `form:"sort_order" binding:"omitempty,oneof=asc desc"`
}

type DeleteRequestLogsRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0"`
}

// ListRequestLogs 获取请求日志列表（分页 + 筛选）
func (h *RequestLogHandler) ListRequestLogs(c *gin.Context) {
	var query RequestLogListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filter := &repository.RequestLogFilter{
		Page:          query.Page,
		PageSize:      query.PageSize,
		Model:         query.Model,
		ChannelID:     query.ChannelID,
		AccessTokenID: query.AccessTokenID,
		EndpointType:  query.EndpointType,
		Status:        query.Status,
		TraceID:       query.TraceID,
		ClientIP:      query.ClientIP,
		SortBy:        query.SortBy,
		SortOrder:     query.SortOrder,
	}

	if query.StartAtMS > 0 {
		t := time.UnixMilli(query.StartAtMS)
		filter.StartedAt = &t
	}
	if query.EndAtMS > 0 {
		t := time.UnixMilli(query.EndAtMS)
		filter.EndAt = &t
	}
	switch query.HasRetry {
	case "true":
		b := true
		filter.HasRetry = &b
	case "false":
		b := false
		filter.HasRetry = &b
	}

	items, total, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		h.logger.Warn("查询请求日志失败", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询请求日志失败"})
		return
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     items,
	})
}

// GetRequestLog 按 trace_id 获取请求日志完整数据（主 + detail + attempts）
func (h *RequestLogHandler) GetRequestLog(c *gin.Context) {
	traceID := c.Param("trace_id")
	if traceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trace_id 不能为空"})
		return
	}

	full, err := h.repo.GetFull(c.Request.Context(), traceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "请求日志不存在"})
			return
		}
		h.logger.Warn("查询请求日志详情失败",
			slog.String("trace_id", traceID),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询请求日志详情失败"})
		return
	}

	c.JSON(http.StatusOK, full)
}

// DeleteRequestLogs 批量删除请求日志，同时删除关联 detail / attempts。
func (h *RequestLogHandler) DeleteRequestLogs(c *gin.Context) {
	var req DeleteRequestLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deleted, err := h.repo.DeleteByIDs(c.Request.Context(), req.IDs)
	if err != nil {
		h.logger.Warn("删除请求日志失败",
			slog.Any("ids", req.IDs),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除请求日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted_count": deleted,
	})
}

// RegisterRoutes 注册请求日志路由
func (h *RequestLogHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/request-logs")
	{
		g.GET("", h.ListRequestLogs)
		g.DELETE("", h.DeleteRequestLogs)
		g.GET("/:trace_id", h.GetRequestLog)
	}
}
