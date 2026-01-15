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

// Delete 删除渠道(软删除)
func (r *ChannelRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Channel{}, id).Error
}

// List 分页查询渠道列表
func (r *ChannelRepository) List(ctx context.Context, offset, limit int) ([]*models.Channel, int64, error) {
	var channels []*models.Channel
	var total int64

	// 查询总数
	if err := r.db.WithContext(ctx).Model(&models.Channel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err := r.db.WithContext(ctx).
		Preload("Keys").
		Preload("ModelConfigs").
		Offset(offset).
		Limit(limit).
		Order("priority ASC, id ASC").
		Find(&channels).Error

	return channels, total, err
}

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
