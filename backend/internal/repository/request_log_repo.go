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
	SortBy    string // created_at / duration_ms
	SortOrder string // asc / desc
}

// RequestLogFull 请求日志详情聚合
type RequestLogFull struct {
	Log      *models.RequestLog          `json:"log"`
	Detail   *models.RequestLogDetail    `json:"detail"`
	Attempts []*models.RequestLogAttempt `json:"attempts"`
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
