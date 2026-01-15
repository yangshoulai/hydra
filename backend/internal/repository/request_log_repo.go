package repository

import (
	"context"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// RequestLogRepository 请求日志仓储
type RequestLogRepository struct {
	db *gorm.DB
}

// NewRequestLogRepository 创建请求日志仓储
func NewRequestLogRepository(db *gorm.DB) *RequestLogRepository {
	return &RequestLogRepository{db: db}
}

// GetDB 获取数据库连接
func (r *RequestLogRepository) GetDB() *gorm.DB {
	return r.db
}

// Create 创建请求日志
func (r *RequestLogRepository) Create(ctx context.Context, log *models.RequestLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// FindByTraceID 根据 TraceID 查询请求日志
func (r *RequestLogRepository) FindByTraceID(ctx context.Context, traceID string) (*models.RequestLog, error) {
	var log models.RequestLog
	err := r.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// ListFilter 分页查询请求日志(支持筛选)
type RequestLogFilter struct {
	StartTime       *time.Time
	EndTime         *time.Time
	StatusCode      *int
	ChannelID       *uint
	AccessTokenID   *uint
	RequestedModel  string
	IsFakeSuccess   *bool
	Offset          int
	Limit           int
}

func (r *RequestLogRepository) List(ctx context.Context, filter *RequestLogFilter) ([]*models.RequestLog, int64, error) {
	var logs []*models.RequestLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.RequestLog{})

	// 应用筛选条件
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	if filter.StatusCode != nil {
		query = query.Where("status_code = ?", *filter.StatusCode)
	}
	if filter.ChannelID != nil {
		query = query.Where("channel_id = ?", *filter.ChannelID)
	}
	if filter.AccessTokenID != nil {
		query = query.Where("access_token_id = ?", *filter.AccessTokenID)
	}
	if filter.RequestedModel != "" {
		query = query.Where("requested_model = ?", filter.RequestedModel)
	}
	if filter.IsFakeSuccess != nil {
		query = query.Where("is_fake_success = ?", *filter.IsFakeSuccess)
	}

	// 查询总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err := query.
		Offset(filter.Offset).
		Limit(filter.Limit).
		Order("created_at DESC").
		Find(&logs).Error

	return logs, total, err
}

// DeleteBefore 删除指定时间之前的日志
func (r *RequestLogRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&models.RequestLog{})
	return result.RowsAffected, result.Error
}

// GetStatistics 获取统计数据
type RequestLogStatistics struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	SuccessRate     float64 `json:"success_rate"`
	AvgDuration     float64 `json:"avg_duration_ms"`
}

func (r *RequestLogRepository) GetStatistics(ctx context.Context, startTime, endTime time.Time) (*RequestLogStatistics, error) {
	var stats RequestLogStatistics

	// 总请求数
	if err := r.db.WithContext(ctx).Model(&models.RequestLog{}).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Count(&stats.TotalRequests).Error; err != nil {
		return nil, err
	}

	// 成功请求数(状态码 200)
	if err := r.db.WithContext(ctx).Model(&models.RequestLog{}).
		Where("created_at BETWEEN ? AND ? AND status_code = ?", startTime, endTime, 200).
		Count(&stats.SuccessRequests).Error; err != nil {
		return nil, err
	}

	stats.FailedRequests = stats.TotalRequests - stats.SuccessRequests

	// 计算成功率
	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
	}

	// 平均响应时间
	var avgDuration float64
	if err := r.db.WithContext(ctx).Model(&models.RequestLog{}).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Select("AVG(response_time)").
		Scan(&avgDuration).Error; err != nil {
		return nil, err
	}
	stats.AvgDuration = avgDuration

	return &stats, nil
}

// GetByTimeRange 根据时间范围获取请求日志
func (r *RequestLogRepository) GetByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*models.RequestLog, error) {
	var logs []*models.RequestLog
	err := r.db.WithContext(ctx).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// GetByChannelIDAndTimeRange 根据渠道ID和时间范围获取请求日志
func (r *RequestLogRepository) GetByChannelIDAndTimeRange(ctx context.Context, channelID uint, startTime, endTime time.Time) ([]*models.RequestLog, error) {
	var logs []*models.RequestLog
	err := r.db.WithContext(ctx).
		Where("channel_id = ? AND created_at BETWEEN ? AND ?", channelID, startTime, endTime).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}
