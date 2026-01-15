package repository

import (
	"context"
	"errors"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// KeyRepository Key 仓储
type KeyRepository struct {
	db *gorm.DB
}

// NewKeyRepository 创建 Key 仓储
func NewKeyRepository(db *gorm.DB) *KeyRepository {
	return &KeyRepository{db: db}
}

// Create 创建 Key
func (r *KeyRepository) Create(ctx context.Context, key *models.Key) error {
	return r.db.WithContext(ctx).Create(key).Error
}

// FindByID 根据ID查询 Key
func (r *KeyRepository) FindByID(ctx context.Context, id uint) (*models.Key, error) {
	var key models.Key
	err := r.db.WithContext(ctx).First(&key, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &key, nil
}

// FindByChannelID 根据渠道ID查询所有 Key
func (r *KeyRepository) FindByChannelID(ctx context.Context, channelID uint) ([]*models.Key, error) {
	var keys []*models.Key
	err := r.db.WithContext(ctx).
		Where("channel_id = ?", channelID).
		Order("id ASC").
		Find(&keys).Error
	return keys, err
}

// FindActiveByChannelID 根据渠道ID查询所有激活的 Key
func (r *KeyRepository) FindActiveByChannelID(ctx context.Context, channelID uint) ([]*models.Key, error) {
	var keys []*models.Key
	err := r.db.WithContext(ctx).
		Where("channel_id = ? AND status = ?", channelID, "active").
		Order("id ASC").
		Find(&keys).Error
	return keys, err
}

// Update 更新 Key
func (r *KeyRepository) Update(ctx context.Context, key *models.Key) error {
	return r.db.WithContext(ctx).Save(key).Error
}

// UpdateStatus 更新 Key 状态
func (r *KeyRepository) UpdateStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).
		Model(&models.Key{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Delete 删除 Key(软删除)
func (r *KeyRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Key{}, id).Error
}

// DeleteByChannelID 删除渠道下的所有 Key
func (r *KeyRepository) DeleteByChannelID(ctx context.Context, channelID uint) error {
	return r.db.WithContext(ctx).
		Where("channel_id = ?", channelID).
		Delete(&models.Key{}).Error
}
