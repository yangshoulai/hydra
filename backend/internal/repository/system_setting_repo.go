package repository

import (
	"context"
	"errors"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// SystemSettingRepository 系统设置仓储
type SystemSettingRepository struct {
	db *gorm.DB
}

// NewSystemSettingRepository 创建系统设置仓储
func NewSystemSettingRepository(db *gorm.DB) *SystemSettingRepository {
	return &SystemSettingRepository{db: db}
}

// GetByKey 根据 Key 获取设置
func (r *SystemSettingRepository) GetByKey(ctx context.Context, key string) (*models.SystemSetting, error) {
	var setting models.SystemSetting
	err := r.db.WithContext(ctx).
		Where("key = ?", key).
		First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &setting, nil
}

// GetAll 获取所有设置
func (r *SystemSettingRepository) GetAll(ctx context.Context) ([]*models.SystemSetting, error) {
	var settings []*models.SystemSetting
	err := r.db.WithContext(ctx).
		Order("category ASC, key ASC").
		Find(&settings).Error
	return settings, err
}

// GetByCategory 根据分类获取设置
func (r *SystemSettingRepository) GetByCategory(ctx context.Context, category string) ([]*models.SystemSetting, error) {
	var settings []*models.SystemSetting
	err := r.db.WithContext(ctx).
		Where("category = ?", category).
		Order("key ASC").
		Find(&settings).Error
	return settings, err
}

// Set 设置或更新配置值
func (r *SystemSettingRepository) Set(ctx context.Context, key, value string) error {
	// 尝试更新
	result := r.db.WithContext(ctx).
		Model(&models.SystemSetting{}).
		Where("key = ?", key).
		Update("value", value)

	if result.Error != nil {
		return result.Error
	}

	// 如果没有更新任何行,则创建新记录
	if result.RowsAffected == 0 {
		setting := &models.SystemSetting{
			Key:   key,
			Value: value,
		}
		return r.db.WithContext(ctx).Create(setting).Error
	}

	return nil
}

// BatchSet 批量设置配置
func (r *SystemSettingRepository) BatchSet(ctx context.Context, settings map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range settings {
			// 尝试更新
			result := tx.Model(&models.SystemSetting{}).
				Where("key = ?", key).
				Update("value", value)

			if result.Error != nil {
				return result.Error
			}

			// 如果没有更新任何行,则创建新记录
			if result.RowsAffected == 0 {
				setting := &models.SystemSetting{
					Key:   key,
					Value: value,
				}
				if err := tx.Create(setting).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Delete 删除配置
func (r *SystemSettingRepository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).
		Where("key = ?", key).
		Delete(&models.SystemSetting{}).Error
}
