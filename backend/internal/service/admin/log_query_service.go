package admin

import (
	"context"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// LogQueryService 日志查询服务
type LogQueryService struct {
	logger        *slog.Logger
	requestLogRepo *repository.RequestLogRepository
}

// NewLogQueryService 创建日志查询服务
func NewLogQueryService(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
) *LogQueryService {
	return &LogQueryService{
		logger:         logger,
		requestLogRepo: requestLogRepo,
	}
}

// LogQueryRequest 日志查询请求
type LogQueryRequest struct {
	// 分页
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`

	// 筛选条件
	TraceID        string `form:"trace_id" binding:"omitempty"`
	AccessToken    string `form:"access_token" binding:"omitempty"`
	RequestedModel string `form:"requested_model" binding:"omitempty"`
	ChannelID      *uint  `form:"channel_id" binding:"omitempty"`
	StatusCode     *int   `form:"status_code" binding:"omitempty"`
	IsSuccess      *bool  `form:"is_success" binding:"omitempty"`

	// 时间范围
	StartTime *time.Time `form:"start_time" binding:"omitempty"`
	EndTime   *time.Time `form:"end_time" binding:"omitempty"`
}

// LogQueryResponse 日志查询响应
type LogQueryResponse struct {
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Logs     []models.RequestLog `json:"logs"`
}

// Query 查询日志列表
func (s *LogQueryService) Query(ctx context.Context, req *LogQueryRequest) (*LogQueryResponse, error) {
	// 设置默认分页
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	s.logger.Debug("querying logs",
		slog.Int("page", req.Page),
		slog.Int("page_size", req.PageSize),
		slog.String("trace_id", req.TraceID),
		slog.String("access_token", req.AccessToken),
		slog.String("requested_model", req.RequestedModel),
	)

	// 构建筛选条件
	filter := &repository.RequestLogFilter{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Offset:    (req.Page - 1) * req.PageSize,
		Limit:     req.PageSize,
	}

	// 映射筛选条件
	if req.StatusCode != nil {
		filter.StatusCode = req.StatusCode
	}
	if req.ChannelID != nil {
		filter.ChannelID = req.ChannelID
	}
	if req.RequestedModel != "" {
		filter.RequestedModel = req.RequestedModel
	}
	if req.IsSuccess != nil {
		// 将is_success转换为状态码筛选
		if *req.IsSuccess {
			statusCode := 200
			filter.StatusCode = &statusCode
		} else {
			// 失败的情况，查询非200状态码（这里需要特殊处理，暂时跳过）
			// 可以扩展repository支持更复杂的查询
		}
	}

	// 查询数据
	logs, total, err := s.requestLogRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed to query logs", slog.String("error", err.Error()))
		return nil, err
	}

	// 转换为值类型
	resultLogs := make([]models.RequestLog, len(logs))
	for i, log := range logs {
		resultLogs[i] = *log
	}

	s.logger.Debug("logs queried successfully",
		slog.Int64("total", total),
		slog.Int("count", len(resultLogs)),
	)

	return &LogQueryResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Logs:     resultLogs,
	}, nil
}

// GetByTraceID 根据TraceID获取日志详情
func (s *LogQueryService) GetByTraceID(ctx context.Context, traceID string) (*models.RequestLog, error) {
	s.logger.Debug("getting log by trace_id", slog.String("trace_id", traceID))

	log, err := s.requestLogRepo.FindByTraceID(ctx, traceID)
	if err != nil {
		s.logger.Error("failed to get log by trace_id",
			slog.String("trace_id", traceID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	return log, nil
}

// LogStatistics 日志统计信息
type LogStatistics struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	SuccessRate     float64 `json:"success_rate"`
	AvgResponseTime float64 `json:"avg_response_time"`
}

// GetStatistics 获取统计信息
func (s *LogQueryService) GetStatistics(ctx context.Context, startTime, endTime *time.Time) (*LogStatistics, error) {
	// 设置默认时间范围（最近24小时）
	if startTime == nil && endTime == nil {
		now := time.Now()
		dayAgo := now.Add(-24 * time.Hour)
		startTime = &dayAgo
		endTime = &now
	}

	// 使用repository的统计方法
	stats, err := s.requestLogRepo.GetStatistics(ctx, *startTime, *endTime)
	if err != nil {
		s.logger.Error("failed to get statistics",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	return &LogStatistics{
		TotalRequests:   stats.TotalRequests,
		SuccessRequests: stats.SuccessRequests,
		FailedRequests:  stats.FailedRequests,
		SuccessRate:     stats.SuccessRate,
		AvgResponseTime: stats.AvgDuration,
	}, nil
}
