package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// ChannelRepository 渠道仓储
type ChannelRepository struct {
	db *gorm.DB
}

// NewChannelRepository 创建渠道仓储
func NewChannelRepository(db *gorm.DB) *ChannelRepository {
	return &ChannelRepository{db: db}
}

// Create 创建渠道
func (r *ChannelRepository) Create(ctx context.Context, channel *models.Channel) error {
	return r.db.WithContext(ctx).Create(channel).Error
}

// FindByID 根据ID查询渠道
func (r *ChannelRepository) FindByID(ctx context.Context, id uint) (*models.Channel, error) {
	var channel models.Channel
	err := r.db.WithContext(ctx).
		Preload("ChannelKeys").
		Preload("ModelConfigs", func(db *gorm.DB) *gorm.DB {
			return db.Order("weight DESC, id DESC")
		}).
		First(&channel, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &channel, nil
}

// FindAll 查询所有渠道
func (r *ChannelRepository) FindAll(ctx context.Context) ([]*models.Channel, error) {
	var channels []*models.Channel
	err := r.db.WithContext(ctx).
		Preload("ChannelKeys").
		Preload("ModelConfigs", func(db *gorm.DB) *gorm.DB {
			return db.Order("weight DESC, id DESC")
		}).
		Order("weight ASC, id ASC").
		Find(&channels).Error
	return channels, err
}

// FindActive 查询所有激活的渠道
func (r *ChannelRepository) FindActive(ctx context.Context) ([]*models.Channel, error) {
	var channels []*models.Channel
	err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Preload("ChannelKeys", "status = ?", "active").
		Preload("ModelConfigs", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = ?", "active").Order("weight DESC, id DESC")
		}).
		Order("weight ASC, id ASC").
		Find(&channels).Error
	return channels, err
}

// Update 更新渠道
func (r *ChannelRepository) Update(ctx context.Context, channel *models.Channel) error {
	return r.db.WithContext(ctx).Save(channel).Error
}

// Delete 删除渠道
func (r *ChannelRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Channel{}, id).Error
}

// ChannelFilter 渠道过滤选项
type ChannelFilter struct {
	Name    string // 名称模糊查询
	BaseURL string // Base URL 模糊查询
	Status  string // 状态精确查询
}

// ChannelSortOptions 渠道排序选项
type ChannelSortOptions struct {
	Field     string // 排序字段：id, name, weight, status
	Direction string // 排序方向：asc, desc
}

// ListWithFilter 分页查询渠道列表（带过滤）
func (r *ChannelRepository) ListWithFilter(ctx context.Context, offset, limit int, filter *ChannelFilter, sortOpts *ChannelSortOptions) ([]*models.Channel, int64, error) {
	var channels []*models.Channel
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Channel{})

	// 应用过滤条件
	if filter != nil {
		if filter.Name != "" {
			name := strings.ToLower(filter.Name)
			query = query.Where("LOWER(name) LIKE ?", "%"+name+"%")
		}
		if filter.BaseURL != "" {
			baseURL := strings.ToLower(filter.BaseURL)
			query = query.Where("LOWER(base_url) LIKE ?", "%"+baseURL+"%")
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
	}

	// 查询总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 构建排序
	orderBy := "weight ASC, id DESC" // 默认排序
	if sortOpts != nil && sortOpts.Field != "" {
		direction := "ASC"
		if sortOpts.Direction == "desc" {
			direction = "DESC"
		}

		// 验证排序字段，防止 SQL 注入
		allowedFields := map[string]bool{
			"id":     true,
			"name":   true,
			"weight": true,
			"status": true,
		}

		if allowedFields[sortOpts.Field] {
			orderBy = sortOpts.Field + " " + direction
		}
	}

	// 分页查询
	err := query.
		Preload("ChannelKeys").
		Preload("ModelConfigs", func(db *gorm.DB) *gorm.DB {
			return db.Order("weight DESC, id DESC")
		}).
		Offset(offset).
		Limit(limit).
		Order(orderBy).
		Find(&channels).Error

	return channels, total, err
}

// FindByModel 根据统一模型名查询所有支持该模型的渠道
func (r *ChannelRepository) FindByModel(ctx context.Context, unifiedModel string, endpointType string, _ bool) ([]models.Channel, error) {
	var channels []models.Channel

	// 子查询:找到所有支持该模型的 channel_id
	query := r.db.WithContext(ctx).
		Select("DISTINCT channels.*").
		Joins("INNER JOIN channel_model_configs ON channel_model_configs.channel_id = channels.id").
		Joins("INNER JOIN channel_keys ON channel_keys.channel_id = channels.id AND channel_keys.status = ?", "active").
		Where("channel_model_configs.model = ?", unifiedModel).
		Where("channel_model_configs.endpoint_types like ?", "%"+endpointType+"%").
		Where("channel_model_configs.status = ?", "active").
		Where("channels.status = ?", "active")
	modelConfigCondition := "model = ? AND status = ? AND endpoint_types like ?"
	modelConfigArgs := []any{unifiedModel, "active", "%" + endpointType + "%"}
	err := query.
		Preload("ChannelKeys", "status = ?", "active").
		Preload("ModelConfigs", func(db *gorm.DB) *gorm.DB {
			return db.
				Where(modelConfigCondition, modelConfigArgs...).
				Order("weight DESC, id DESC")
		}).
		Order("channels.weight DESC, channels.id DESC").
		Find(&channels).Error

	return channels, err
}
