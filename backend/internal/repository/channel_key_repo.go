package repository

import (
	"context"
	"errors"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// ChannelKeyRepository 渠道密钥仓储
type ChannelKeyRepository struct {
	db *gorm.DB
}

// NewChannelKeyRepository 创建渠道密钥仓储
func NewChannelKeyRepository(db *gorm.DB) *ChannelKeyRepository {
	return &ChannelKeyRepository{db: db}
}

// Create 创建渠道密钥
func (r *ChannelKeyRepository) Create(ctx context.Context, channelKey *models.ChannelKey) error {
	return r.db.WithContext(ctx).Create(channelKey).Error
}

// FindByID 根据ID查询渠道密钥
func (r *ChannelKeyRepository) FindByID(ctx context.Context, id uint) (*models.ChannelKey, error) {
	var channelKey models.ChannelKey
	err := r.db.WithContext(ctx).First(&channelKey, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &channelKey, nil
}

// FindByChannelID 根据渠道ID查询所有渠道密钥
func (r *ChannelKeyRepository) FindByChannelID(ctx context.Context, channelID uint) ([]*models.ChannelKey, error) {
	var channelKeys []*models.ChannelKey
	err := r.db.WithContext(ctx).
		Where("channel_id = ?", channelID).
		Order("id ASC").
		Find(&channelKeys).Error
	return channelKeys, err
}

// FindActiveByChannelID 根据渠道ID查询所有启用的渠道密钥
func (r *ChannelKeyRepository) FindActiveByChannelID(ctx context.Context, channelID uint) ([]*models.ChannelKey, error) {
	var channelKeys []*models.ChannelKey
	err := r.db.WithContext(ctx).
		Where("channel_id = ? AND status = ?", channelID, "active").
		Order("id ASC").
		Find(&channelKeys).Error
	return channelKeys, err
}

// FindNonDeadByChannelID 兼容旧方法名，等价于 FindActiveByChannelID
func (r *ChannelKeyRepository) FindNonDeadByChannelID(ctx context.Context, channelID uint) ([]*models.ChannelKey, error) {
	return r.FindActiveByChannelID(ctx, channelID)
}

// Update 更新渠道密钥
func (r *ChannelKeyRepository) Update(ctx context.Context, channelKey *models.ChannelKey) error {
	return r.db.WithContext(ctx).Save(channelKey).Error
}

// IncrementTokenUsage 累加渠道密钥的 token 使用量
func (r *ChannelKeyRepository) IncrementTokenUsage(ctx context.Context, id uint, promptTokens, completionTokens int64) error {
	return r.db.WithContext(ctx).
		Model(&models.ChannelKey{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"prompt_tokens":     gorm.Expr("prompt_tokens + ?", promptTokens),
			"completion_tokens": gorm.Expr("completion_tokens + ?", completionTokens),
		}).Error
}

// UpdateStatus 更新渠道密钥状态
func (r *ChannelKeyRepository) UpdateStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).
		Model(&models.ChannelKey{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Delete 删除渠道密钥
func (r *ChannelKeyRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ChannelKey{}, id).Error
}

// DeleteByChannelID 删除渠道下的所有渠道密钥
func (r *ChannelKeyRepository) DeleteByChannelID(ctx context.Context, channelID uint) error {
	return r.db.WithContext(ctx).
		Where("channel_id = ?", channelID).
		Delete(&models.ChannelKey{}).Error
}

// FindAll 查询所有渠道密钥（不限制状态）
func (r *ChannelKeyRepository) FindAll(ctx context.Context) ([]*models.ChannelKey, error) {
	var channelKeys []*models.ChannelKey
	err := r.db.WithContext(ctx).
		Order("id ASC").
		Find(&channelKeys).Error
	return channelKeys, err
}

// ChannelKeyStatusCount 渠道密钥状态统计
type ChannelKeyStatusCount struct {
	Active   int64 `json:"active"`
	Inactive int64 `json:"inactive"`
}

// CountByChannelIDAndStatus 根据渠道ID统计不同状态的密钥数量
func (r *ChannelKeyRepository) CountByChannelIDAndStatus(ctx context.Context, channelID uint) (*ChannelKeyStatusCount, error) {
	var counts []struct {
		Status string
		Count  int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.ChannelKey{}).
		Select("status, COUNT(*) as count").
		Where("channel_id = ?", channelID).
		Group("status").
		Scan(&counts).Error

	if err != nil {
		return nil, err
	}

	result := &ChannelKeyStatusCount{}
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
