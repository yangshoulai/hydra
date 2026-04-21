package repository

import (
	"context"
	"errors"
	"strings"
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

// RequestLogFilter 请求日志查询条件
type RequestLogFilter struct {
	Page     int
	PageSize int

	StartedAt *time.Time
	EndAt     *time.Time

	Model         string
	ChannelID     uint
	AccessTokenID uint
	EndpointType  string
	// Status: "success" / "failed" / ""(全部)
	Status    string
	HasRetry  *bool
	TraceID   string // 前缀匹配
	ClientIP  string // 前缀匹配
	SortBy    string // created_at / duration_ms
	SortOrder string // asc / desc
}

// RequestLogFull 请求日志详情聚合
type RequestLogFull struct {
	Log      *models.RequestLog          `json:"log"`
	Detail   *models.RequestLogDetail    `json:"detail"`
	Attempts []*models.RequestLogAttempt `json:"attempts"`
}

// RequestLogSummary 请求日志聚合总览
type RequestLogSummary struct {
	TotalRequests         int   `json:"total_requests"`
	SuccessRequests       int   `json:"success_requests"`
	FailedRequests        int   `json:"failed_requests"`
	TotalPromptTokens     int64 `json:"total_prompt_tokens"`
	TotalCompletionTokens int64 `json:"total_completion_tokens"`
}

// RequestLogMinuteAggregate 请求日志按分钟聚合
type RequestLogMinuteAggregate struct {
	MinuteUnix      int64 `json:"minute_unix"`
	TotalRequests   int   `json:"total_requests"`
	SuccessRequests int   `json:"success_requests"`
	FailedRequests  int   `json:"failed_requests"`
}

// RequestLogChannelAggregate 请求日志按渠道聚合
type RequestLogChannelAggregate struct {
	ChannelID        uint   `json:"channel_id"`
	ChannelName      string `json:"channel_name"`
	TotalRequests    int    `json:"total_requests"`
	SuccessRequests  int    `json:"success_requests"`
	FailedRequests   int    `json:"failed_requests"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

// RequestLogModelAggregate 请求日志按模型聚合
type RequestLogModelAggregate struct {
	ModelName       string `json:"model_name"`
	TotalRequests   int    `json:"total_requests"`
	SuccessRequests int    `json:"success_requests"`
	FailedRequests  int    `json:"failed_requests"`
}

// CreateWithTx 在单一事务内写入主表 + 详情 + 尝试明细
// detail 与 attempts 为 nil 时跳过写入，支持调试模式开关语义。
func (r *RequestLogRepository) CreateWithTx(
	ctx context.Context,
	log *models.RequestLog,
	detail *models.RequestLogDetail,
	attempts []*models.RequestLogAttempt,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		if detail != nil {
			if err := tx.Create(detail).Error; err != nil {
				return err
			}
		}
		if len(attempts) > 0 {
			if err := tx.Create(&attempts).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// List 分页查询请求日志主表
func (r *RequestLogRepository) List(ctx context.Context, filter *RequestLogFilter) ([]*models.RequestLog, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}

	q := r.db.WithContext(ctx).Model(&models.RequestLog{})

	if filter.StartedAt != nil {
		q = q.Where("created_at >= ?", *filter.StartedAt)
	}
	if filter.EndAt != nil {
		q = q.Where("created_at < ?", *filter.EndAt)
	}
	if filter.Model != "" {
		q = q.Where("model = ?", filter.Model)
	}
	if filter.ChannelID != 0 {
		q = q.Where("final_channel_id = ?", filter.ChannelID)
	}
	if filter.AccessTokenID != 0 {
		q = q.Where("access_token_id = ?", filter.AccessTokenID)
	}
	if filter.EndpointType != "" {
		q = q.Where("endpoint_type = ?", filter.EndpointType)
	}
	switch strings.ToLower(filter.Status) {
	case "success":
		q = q.Where("success = ?", true)
	case "failed":
		q = q.Where("success = ?", false)
	}
	if filter.HasRetry != nil {
		if *filter.HasRetry {
			q = q.Where("retry_count > 0")
		} else {
			q = q.Where("retry_count = 0")
		}
	}
	if filter.TraceID != "" {
		q = q.Where("trace_id LIKE ?", filter.TraceID+"%")
	}
	if filter.ClientIP != "" {
		q = q.Where("client_ip LIKE ?", filter.ClientIP+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderBy := "created_at DESC"
	if filter.SortBy != "" {
		col := "created_at"
		switch strings.ToLower(filter.SortBy) {
		case "duration_ms":
			col = "duration_ms"
		case "created_at":
			col = "created_at"
		}
		direction := "DESC"
		if strings.ToLower(filter.SortOrder) == "asc" {
			direction = "ASC"
		}
		orderBy = col + " " + direction
	}

	var items []*models.RequestLog
	if err := q.Order(orderBy).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetFull 按 trace_id 获取完整日志（主 + 详情 + 尝试明细）
// 主记录不存在时返回 (nil, gorm.ErrRecordNotFound) 由调用方决定是否转 404。
func (r *RequestLogRepository) GetFull(ctx context.Context, traceID string) (*RequestLogFull, error) {
	var log models.RequestLog
	if err := r.db.WithContext(ctx).Where("trace_id = ?", traceID).First(&log).Error; err != nil {
		return nil, err
	}

	full := &RequestLogFull{Log: &log}

	var detail models.RequestLogDetail
	if err := r.db.WithContext(ctx).Where("trace_id = ?", traceID).First(&detail).Error; err == nil {
		full.Detail = &detail
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var attempts []*models.RequestLogAttempt
	if err := r.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("attempt_num ASC").
		Find(&attempts).Error; err != nil {
		return nil, err
	}
	full.Attempts = attempts

	return full, nil
}

// AggregateSummary 获取指定时间之后的请求日志总览聚合
func (r *RequestLogRepository) AggregateSummary(ctx context.Context, since time.Time) (*RequestLogSummary, error) {
	var summary RequestLogSummary
	err := r.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select(`
			COUNT(*) AS total_requests,
			COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) AS success_requests,
			COALESCE(SUM(CASE WHEN success = 1 THEN 0 ELSE 1 END), 0) AS failed_requests,
			COALESCE(SUM(prompt_tokens), 0) AS total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS total_completion_tokens
		`).
		Where("created_at >= ?", since).
		Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// AggregateQPSByMinute 获取指定时间之后按分钟聚合的请求日志
func (r *RequestLogRepository) AggregateQPSByMinute(ctx context.Context, since time.Time) ([]RequestLogMinuteAggregate, error) {
	var rows []RequestLogMinuteAggregate
	err := r.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select(`
			CAST(strftime('%s', created_at) AS INTEGER) / 60 AS minute_unix,
			COUNT(*) AS total_requests,
			COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) AS success_requests,
			COALESCE(SUM(CASE WHEN success = 1 THEN 0 ELSE 1 END), 0) AS failed_requests
		`).
		Where("created_at >= ?", since).
		Group("minute_unix").
		Order("minute_unix ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// AggregateByChannel 获取指定时间之后按渠道聚合的请求日志
func (r *RequestLogRepository) AggregateByChannel(ctx context.Context, since time.Time) ([]RequestLogChannelAggregate, error) {
	var rows []RequestLogChannelAggregate
	err := r.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select(`
			final_channel_id AS channel_id,
			MAX(final_channel_name) AS channel_name,
			COUNT(*) AS total_requests,
			COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) AS success_requests,
			COALESCE(SUM(CASE WHEN success = 1 THEN 0 ELSE 1 END), 0) AS failed_requests,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens
		`).
		Where("created_at >= ?", since).
		Where("final_channel_id > 0").
		Group("final_channel_id").
		Order("total_requests DESC, channel_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// AggregateByModel 获取指定时间之后按模型聚合的请求日志
func (r *RequestLogRepository) AggregateByModel(ctx context.Context, since time.Time) ([]RequestLogModelAggregate, error) {
	var rows []RequestLogModelAggregate
	err := r.db.WithContext(ctx).
		Model(&models.RequestLog{}).
		Select(`
			model AS model_name,
			COUNT(*) AS total_requests,
			COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) AS success_requests,
			COALESCE(SUM(CASE WHEN success = 1 THEN 0 ELSE 1 END), 0) AS failed_requests
		`).
		Where("created_at >= ?", since).
		Where("model <> ''").
		Group("model").
		Order("total_requests DESC, model_name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteByIDs 按主键批量删除请求日志，并同步删除关联的 detail / attempts。
// 返回删除的主记录条数。
func (r *RequestLogRepository) DeleteByIDs(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	var deleted int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var logs []*models.RequestLog
		if err := tx.Select("id", "trace_id").Where("id IN ?", ids).Find(&logs).Error; err != nil {
			return err
		}
		if len(logs) == 0 {
			deleted = 0
			return nil
		}

		traceIDs := make([]string, 0, len(logs))
		resolvedIDs := make([]uint, 0, len(logs))
		for _, item := range logs {
			traceIDs = append(traceIDs, item.TraceID)
			resolvedIDs = append(resolvedIDs, item.ID)
		}

		if err := tx.Where("trace_id IN ?", traceIDs).Delete(&models.RequestLogAttempt{}).Error; err != nil {
			return err
		}
		if err := tx.Where("trace_id IN ?", traceIDs).Delete(&models.RequestLogDetail{}).Error; err != nil {
			return err
		}

		res := tx.Where("id IN ?", resolvedIDs).Delete(&models.RequestLog{})
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return nil
	})

	return deleted, err
}

// DeleteOlderThan 删除 created_at 早于指定时间的所有日志（包含三张表）
// 返回删除的主记录条数。
func (r *RequestLogRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	var deleted int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("created_at < ?", before).Delete(&models.RequestLogAttempt{})
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("created_at < ?", before).Delete(&models.RequestLogDetail{})
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("created_at < ?", before).Delete(&models.RequestLog{})
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return nil
	})
	return deleted, err
}
