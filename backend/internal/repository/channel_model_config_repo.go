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

// CountDistinctChannelsByModels 批量统计一组模型各自关联的渠道数量
// 返回 map[model]count；只统计 active 的配置。未命中模型默认为 0。
func (r *ChannelModelConfigRepository) CountDistinctChannelsByModels(ctx context.Context, models []string) (map[string]int64, error) {
	result := make(map[string]int64, len(models))
	if len(models) == 0 {
		return result, nil
	}
	type row struct {
		Model string
		Count int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("channel_model_configs").
		Select("model, COUNT(DISTINCT channel_id) AS count").
		Where("model IN ? AND status = ?", models, "active").
		Group("model").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Model] = r.Count
	}
	return result, nil
}

// Create 创建模型配置
func (r *ChannelModelConfigRepository) Create(ctx context.Context, config *models.ChannelModelConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
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

// FindByChannelAndChannelModel 根据渠道 ID 和渠道模型名查询模型配置
func (r *ChannelModelConfigRepository) FindByChannelAndChannelModel(ctx context.Context, channelID uint, channelModel string) (*models.ChannelModelConfig, error) {
	var config models.ChannelModelConfig
	err := r.db.WithContext(ctx).
		Where("channel_id = ? AND channel_model = ?", channelID, channelModel).
		First(&config).Error
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
		Order("model ASC").
		Find(&configs).Error
	return configs, err
}

// ModelConfigStatusCount 模型配置状态统计
type ModelConfigStatusCount struct {
	Active   int64 `json:"active"`
	Inactive int64 `json:"inactive"`
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
		case "inactive":
			result.Inactive = c.Count
		}
	}

	return result, nil
}

// Update 更新模型配置
func (r *ChannelModelConfigRepository) Update(ctx context.Context, config *models.ChannelModelConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// BulkUpdateWeightByChannelAndCurrentWeight 批量更新渠道模型权重
// 仅更新“当前权重等于 expectedCurrentWeight”的记录，用于同步继承渠道权重的模型配置。
func (r *ChannelModelConfigRepository) BulkUpdateWeightByChannelAndCurrentWeight(
	ctx context.Context,
	channelID uint,
	expectedCurrentWeight int,
	targetWeight int,
) (int64, error) {
	if targetWeight <= 0 {
		return 0, nil
	}

	result := r.db.WithContext(ctx).
		Model(&models.ChannelModelConfig{}).
		Where("channel_id = ?", channelID).
		Where("weight = ?", expectedCurrentWeight).
		Updates(map[string]any{
			"weight": targetWeight,
		})

	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// IncrementTokenUsage 累加模型配置的 token 使用量
func (r *ChannelModelConfigRepository) IncrementTokenUsage(ctx context.Context, id uint, promptTokens, completionTokens int64) error {
	return r.db.WithContext(ctx).
		Model(&models.ChannelModelConfig{}).
		Where("id = ?", id).
		Updates(map[string]any{
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
		Where("model = ?", modelName).
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

// ExistsActiveModel 检查是否存在可用的统一模型配置
func (r *ChannelModelConfigRepository) ExistsActiveModel(ctx context.Context, model string, endpointType string) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).
		Model(&models.ChannelModelConfig{}).
		Joins("INNER JOIN channels ON channels.id = channel_model_configs.channel_id").
		Where("channel_model_configs.model = ?", model).
		Where("channel_model_configs.status = ?", "active").
		Where("channels.status = ?", "active").
		Where("channel_model_configs.endpoint_types like '%\"" + endpointType + "\"%'")
	err := query.Limit(1).Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
