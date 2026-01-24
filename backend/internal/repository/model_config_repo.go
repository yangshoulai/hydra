package repository

import (
	"context"
	"errors"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// ChannelModelConfigRepository 渠道模型配置仓储
type ChannelModelConfigRepository struct {
	db *gorm.DB
}

// NewChannelModelConfigRepository 创建渠道模型配置仓储
func NewChannelModelConfigRepository(db *gorm.DB) *ChannelModelConfigRepository {
	return &ChannelModelConfigRepository{db: db}
}

// Create 创建模型配置
func (r *ChannelModelConfigRepository) Create(ctx context.Context, config *models.ChannelModelConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// BatchCreate 批量创建模型配置
func (r *ChannelModelConfigRepository) BatchCreate(ctx context.Context, configs []*models.ChannelModelConfig) error {
	if len(configs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&configs).Error
}

// FindByID 根据ID查询模型配置
func (r *ChannelModelConfigRepository) FindByID(ctx context.Context, id uint) (*models.ChannelModelConfig, error) {
	var config models.ChannelModelConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// FindByChannelID 根据渠道ID查询所有模型配置
func (r *ChannelModelConfigRepository) FindByChannelID(ctx context.Context, channelID uint) ([]*models.ChannelModelConfig, error) {
	var configs []*models.ChannelModelConfig
	err := r.db.WithContext(ctx).
		Where("channel_id = ?", channelID).
		Order("unified_model ASC").
		Find(&configs).Error
	return configs, err
}

// CountByChannelID 统计渠道的模型配置数量
func (r *ChannelModelConfigRepository) CountByChannelID(ctx context.Context, channelID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ChannelModelConfig{}).
		Where("channel_id = ?", channelID).
		Count(&count).Error
	return int(count), err
}

// ModelConfigStatusCount 模型配置状态统计
type ModelConfigStatusCount struct {
	Active   int64 `json:"active"`
	Disabled int64 `json:"disabled"`
	NonExist int64 `json:"non_exist"`
}

// CountByChannelIDAndStatus 统计渠道下各状态模型配置数量
func (r *ChannelModelConfigRepository) CountByChannelIDAndStatus(ctx context.Context, channelID uint) (*ModelConfigStatusCount, error) {
	var counts []struct {
		Status string
		Count  int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.ChannelModelConfig{}).
		Select("status, COUNT(*) as count").
		Where("channel_id = ?", channelID).
		Group("status").
		Scan(&counts).Error

	if err != nil {
		return nil, err
	}

	result := &ModelConfigStatusCount{}
	for _, c := range counts {
		switch c.Status {
		case "active":
			result.Active = c.Count
		case "disabled":
			result.Disabled = c.Count
		case "non_exist":
			result.NonExist = c.Count
		}
	}

	return result, nil
}

// FindByUnifiedModel 根据统一模型名查询所有支持的渠道配置
func (r *ChannelModelConfigRepository) FindByUnifiedModel(ctx context.Context, unifiedModel string) ([]*models.ChannelModelConfig, error) {
	var configs []*models.ChannelModelConfig
	err := r.db.WithContext(ctx).
		Where("unified_model = ? AND status = ?", unifiedModel, "active").
		Preload("Channel", "status = ?", "active").
		Order("id ASC").
		Find(&configs).Error
	return configs, err
}

// Update 更新模型配置
func (r *ChannelModelConfigRepository) Update(ctx context.Context, config *models.ChannelModelConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// IncrementTokenUsage 累加模型配置的 token 使用量
func (r *ChannelModelConfigRepository) IncrementTokenUsage(ctx context.Context, id uint, promptTokens, completionTokens int64) error {
	return r.db.WithContext(ctx).
		Model(&models.ChannelModelConfig{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"prompt_tokens":     gorm.Expr("prompt_tokens + ?", promptTokens),
			"completion_tokens": gorm.Expr("completion_tokens + ?", completionTokens),
		}).Error
}

// Delete 删除模型配置
func (r *ChannelModelConfigRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ChannelModelConfig{}, id).Error
}

// FindByModelNameWithChannel 根据统一模型名查询渠道模型配置（包含渠道信息）
func (r *ChannelModelConfigRepository) FindByModelNameWithChannel(ctx context.Context, modelName string) ([]*models.ChannelModelConfig, error) {
	var configs []*models.ChannelModelConfig
	err := r.db.WithContext(ctx).
		Preload("Channel").
		Where("unified_model = ?", modelName).
		Order("channel_id").
		Find(&configs).Error
	return configs, err
}

// DeleteByChannelID 删除渠道下的所有模型配置
func (r *ChannelModelConfigRepository) DeleteByChannelID(ctx context.Context, channelID uint) error {
	return r.db.WithContext(ctx).
		Where("channel_id = ?", channelID).
		Delete(&models.ChannelModelConfig{}).Error
}

// ListUnifiedModels 获取所有启用的统一模型名列表(去重)
func (r *ChannelModelConfigRepository) ListUnifiedModels(ctx context.Context) ([]string, error) {
	var modelNames []string

	// 子查询：只选择激活渠道的模型配置
	err := r.db.WithContext(ctx).
		Model(&models.ChannelModelConfig{}).
		Joins("INNER JOIN channels ON channels.id = channel_model_configs.channel_id").
		Where("channel_model_configs.status = ?", "active").
		Where("channels.status = ?", "active").
		Distinct("channel_model_configs.unified_model").
		Pluck("channel_model_configs.unified_model", &modelNames).Error

	return modelNames, err
}
