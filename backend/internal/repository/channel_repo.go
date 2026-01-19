package repository

import (
	"context"
	"errors"

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
		Preload("Keys").
		Preload("ModelConfigs").
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
		Preload("Keys").
		Preload("ModelConfigs").
		Order("priority ASC, id ASC").
		Find(&channels).Error
	return channels, err
}

// FindActive 查询所有激活的渠道
func (r *ChannelRepository) FindActive(ctx context.Context) ([]*models.Channel, error) {
	var channels []*models.Channel
	err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Preload("Keys", "status = ?", "active").
		Preload("ModelConfigs", "enabled = ?", true).
		Order("priority ASC, weight DESC").
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
	Field     string // 排序字段：id, name, priority, weight, status
	Direction string // 排序方向：asc, desc
}

// List 分页查询渠道列表
func (r *ChannelRepository) List(ctx context.Context, offset, limit int) ([]*models.Channel, int64, error) {
	return r.ListWithFilter(ctx, offset, limit, nil, nil)
}

// ListWithFilter 分页查询渠道列表（带过滤）
func (r *ChannelRepository) ListWithFilter(ctx context.Context, offset, limit int, filter *ChannelFilter, sortOpts *ChannelSortOptions) ([]*models.Channel, int64, error) {
	var channels []*models.Channel
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Channel{})

	// 应用过滤条件
	if filter != nil {
		if filter.Name != "" {
			query = query.Where("name LIKE ?", "%"+filter.Name+"%")
		}
		if filter.BaseURL != "" {
			query = query.Where("base_url LIKE ?", "%"+filter.BaseURL+"%")
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
	orderBy := "priority ASC, id DESC" // 默认排序
	if sortOpts != nil && sortOpts.Field != "" {
		direction := "ASC"
		if sortOpts.Direction == "desc" {
			direction = "DESC"
		}

		// 验证排序字段，防止 SQL 注入
		allowedFields := map[string]bool{
			"id":       true,
			"name":     true,
			"priority": true,
			"weight":   true,
			"status":   true,
		}

		if allowedFields[sortOpts.Field] {
			orderBy = sortOpts.Field + " " + direction
		}
	}

	// 分页查询
	err := query.
		Preload("Keys").
		Preload("ModelConfigs").
		Offset(offset).
		Limit(limit).
		Order(orderBy).
		Find(&channels).Error

	return channels, total, err
}

// List 分页查询渠道列表（已废弃，使用 ListWithFilter）
// func (r *ChannelRepository) List(ctx context.Context, offset, limit int) ([]*models.Channel, int64, error) {

// FindByModel 根据统一模型名查询所有支持该模型的渠道
func (r *ChannelRepository) FindByModel(ctx context.Context, unifiedModel string) ([]models.Channel, error) {
	var channels []models.Channel

	// 子查询:找到所有支持该模型的 channel_id
	err := r.db.WithContext(ctx).
		Joins("INNER JOIN channel_model_configs ON channel_model_configs.channel_id = channels.id").
		Where("channel_model_configs.unified_model = ?", unifiedModel).
		Where("channel_model_configs.status = ?", "active").
		Where("channels.status = ?", "active").
		Preload("Keys").
		Preload("ModelConfigs", "unified_model = ? AND status = ?", unifiedModel, "active").
		Order("channels.priority DESC, channels.weight DESC").
		Find(&channels).Error

	return channels, err
}

// FindByModelAndEndpointType 根据模型名和端点类型查找渠道
func (r *ChannelRepository) FindByModelAndEndpointType(ctx context.Context, unifiedModel string, endpointType string) ([]models.Channel, error) {
	// 先查询所有支持该模型的渠道
	channels, err := r.FindByModel(ctx, unifiedModel)
	if err != nil {
		return nil, err
	}

	// 在内存中过滤出支持目标端点类型的渠道
	var filteredChannels []models.Channel
	for _, channel := range channels {
		// 检查渠道的模型配置是否支持目标端点类型
		for _, config := range channel.ModelConfigs {
			if config.UnifiedModel == unifiedModel {
				// 检查 endpoint_types 数组是否包含目标端点类型
				for _, et := range config.EndpointTypes {
					if et == endpointType {
						filteredChannels = append(filteredChannels, channel)
						break
					}
				}
				break
			}
		}
	}

	return filteredChannels, nil
}
