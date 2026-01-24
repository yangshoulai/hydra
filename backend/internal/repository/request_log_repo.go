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

// CreateMain 创建主日志记录
func (r *RequestLogRepository) CreateMain(ctx context.Context, log *models.RequestLogMain) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// CreateDetail 创建明细日志记录
func (r *RequestLogRepository) CreateDetail(ctx context.Context, detail *models.RequestLogDetail) error {
	return r.db.WithContext(ctx).Create(detail).Error
}

// UpdateMainTokenUsageByTraceID 更新主日志的 token 使用量
func (r *RequestLogRepository) UpdateMainTokenUsageByTraceID(ctx context.Context, traceID string, promptTokens, completionTokens int64) error {
	return r.db.WithContext(ctx).
		Model(&models.RequestLogMain{}).
		Where("trace_id = ?", traceID).
		Updates(map[string]interface{}{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
		}).Error
}

// UpdateDetailTokenUsageByTraceIDAndRetryIndex 更新明细日志的 token 使用量
func (r *RequestLogRepository) UpdateDetailTokenUsageByTraceIDAndRetryIndex(
	ctx context.Context,
	traceID string,
	retryIndex int,
	promptTokens, completionTokens int64,
) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE request_logs_detail
		SET prompt_tokens = ?, completion_tokens = ?
		WHERE main_log_id = (
			SELECT id FROM request_logs_main WHERE trace_id = ? LIMIT 1
		) AND retry_index = ?
	`, promptTokens, completionTokens, traceID, retryIndex).Error
}

// FindMainByTraceID 根据 TraceID 查询主日志记录
func (r *RequestLogRepository) FindMainByTraceID(ctx context.Context, traceID string) (*models.RequestLogMain, error) {
	var log models.RequestLogMain
	err := r.db.WithContext(ctx).
		Preload("Details").
		Where("trace_id = ?", traceID).
		First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// FindDetailsByMainLogID 根据主日志 ID 查询明细记录
func (r *RequestLogRepository) FindDetailsByMainLogID(ctx context.Context, mainLogID uint) ([]models.RequestLogDetail, error) {
	var details []models.RequestLogDetail
	err := r.db.WithContext(ctx).
		Where("main_log_id = ?", mainLogID).
		Order("retry_index ASC").
		Find(&details).Error
	if err != nil {
		return nil, err
	}
	return details, nil
}

// ListMainFilter 主日志分页查询过滤器
type ListMainFilter struct {
	StartTime      *time.Time
	EndTime        *time.Time
	TraceID        string
	AccessToken    string
	StatusCode     *int
	IsSuccess      *bool
	EndpointType   string
	RequestedModel string
	UnifiedModel   string
	ChannelID      *int64
	Offset         int
	Limit          int
}

// ListMain 分页查询主日志记录
func (r *RequestLogRepository) ListMain(ctx context.Context, filter *ListMainFilter) ([]*models.RequestLogMain, int64, error) {
	var logs []*models.RequestLogMain
	var total int64

	query := r.db.WithContext(ctx).Model(&models.RequestLogMain{})

	// 应用筛选条件
	if filter.StartTime != nil {
		query = query.Where("start_time >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("end_time <= ?", filter.EndTime)
	}
	if filter.TraceID != "" {
		query = query.Where("trace_id = ?", filter.TraceID)
	}
	if filter.AccessToken != "" {
		query = query.Where("access_token = ?", filter.AccessToken)
	}
	if filter.StatusCode != nil {
		query = query.Where("status_code = ?", *filter.StatusCode)
	}
	if filter.IsSuccess != nil {
		query = query.Where("is_success = ?", *filter.IsSuccess)
	}
	if filter.EndpointType != "" {
		query = query.Where("endpoint_type = ?", filter.EndpointType)
	}
	if filter.RequestedModel != "" {
		query = query.Where("requested_model = ?", filter.RequestedModel)
	}
	if filter.UnifiedModel != "" {
		query = query.Where("unified_model = ?", filter.UnifiedModel)
	}
	if filter.ChannelID != nil {
		// 从从表 request_log_detail 的渠道 channel_id 过滤
		query = query.Where("id IN (SELECT main_log_id FROM request_logs_detail WHERE channel_id = ?)", *filter.ChannelID)
	}

	// 查询总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	db := query.Order("start_time DESC")
	if filter.Limit > 0 {
		db = db.Offset(filter.Offset).Limit(filter.Limit)
	}
	err := db.Find(&logs).Error

	return logs, total, err
}

// GetByTimeRange 根据时间范围获取主日志记录
func (r *RequestLogRepository) GetByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*models.RequestLogMain, error) {
	var logs []*models.RequestLogMain
	err := r.db.WithContext(ctx).
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Order("start_time ASC").
		Find(&logs).Error
	return logs, err
}

// GetByChannelIDAndTimeRange 根据渠道 ID 和时间范围获取主日志记录
func (r *RequestLogRepository) GetByChannelIDAndTimeRange(ctx context.Context, channelID uint, startTime, endTime time.Time) ([]*models.RequestLogMain, error) {
	var logs []*models.RequestLogMain
	err := r.db.WithContext(ctx).
		Where("last_channel_id = ? AND start_time >= ? AND start_time <= ?", channelID, startTime, endTime).
		Order("start_time ASC").
		Find(&logs).Error
	return logs, err
}

// DeleteMainBefore 删除指定时间之前的主日志记录
func (r *RequestLogRepository) DeleteMainBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&models.RequestLogMain{})
	return result.RowsAffected, result.Error
}

// DeleteDetailsBefore 删除指定时间之前的明细日志记录
func (r *RequestLogRepository) DeleteDetailsBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&models.RequestLogDetail{})
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
	if err := r.db.WithContext(ctx).Model(&models.RequestLogMain{}).
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Count(&stats.TotalRequests).Error; err != nil {
		return nil, err
	}

	// 成功请求数
	if err := r.db.WithContext(ctx).Model(&models.RequestLogMain{}).
		Where("start_time >= ? AND start_time <= ? AND is_success = ?", startTime, endTime, true).
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
	if err := r.db.WithContext(ctx).Model(&models.RequestLogMain{}).
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Select("AVG(duration)").
		Scan(&avgDuration).Error; err != nil {
		return nil, err
	}
	stats.AvgDuration = avgDuration

	return &stats, nil
}

// TokenUsageSummary token 使用量统计
type TokenUsageSummary struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
}

// SumTokenUsageByTimeRange 统计时间范围内的 token 使用量
func (r *RequestLogRepository) SumTokenUsageByTimeRange(ctx context.Context, startTime, endTime time.Time) (*TokenUsageSummary, error) {
	var result TokenUsageSummary
	err := r.db.WithContext(ctx).
		Model(&models.RequestLogMain{}).
		Select("COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens").
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SumTokenUsageByChannel 统计时间范围内各渠道的 token 使用量
func (r *RequestLogRepository) SumTokenUsageByChannel(ctx context.Context, startTime, endTime time.Time) (map[uint]*TokenUsageSummary, error) {
	type channelTokenUsage struct {
		ChannelID        uint  `gorm:"column:channel_id"`
		PromptTokens     int64 `gorm:"column:prompt_tokens"`
		CompletionTokens int64 `gorm:"column:completion_tokens"`
	}

	var rows []channelTokenUsage
	err := r.db.WithContext(ctx).
		Model(&models.RequestLogMain{}).
		Select("last_channel_id AS channel_id, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens").
		Where("last_channel_id IS NOT NULL").
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Group("last_channel_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]*TokenUsageSummary, len(rows))
	for _, row := range rows {
		result[row.ChannelID] = &TokenUsageSummary{
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
		}
	}
	return result, nil
}
