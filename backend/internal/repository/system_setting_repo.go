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

// Set 设置或更新配置值（包含 category）
// 如果记录已存在，保留原有的 category；如果不存在，使用传入的 category
func (r *SystemSettingRepository) Set(ctx context.Context, key, value, category string) error {
	// 先查询是否存在
	var existing models.SystemSetting
	err := r.db.WithContext(ctx).
		Where("key = ?", key).
		First(&existing).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 确定要使用的 category
	actualCategory := category
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 记录不存在，使用传入的 category（默认值）
		actualCategory = category
	} else if existing.Category != "" {
		// 记录存在且有 category，保留原有的
		actualCategory = existing.Category
	}

	// 执行更新或创建
	result := r.db.WithContext(ctx).
		Model(&models.SystemSetting{}).
		Where("key = ?", key).
		Updates(map[string]interface{}{
			"value":    value,
			"category": actualCategory,
		})

	if result.Error != nil {
		return result.Error
	}

	// 如果没有更新任何行,则创建新记录
	if result.RowsAffected == 0 {
		setting := &models.SystemSetting{
			Key:      key,
			Value:    value,
			Category: actualCategory,
		}
		return r.db.WithContext(ctx).Create(setting).Error
	}

	return nil
}

// BatchSet 批量设置配置（包含 category）
// 如果记录已存在，保留原有的 category；如果不存在，使用传入的 category
func (r *SystemSettingRepository) BatchSet(ctx context.Context, settings map[string]string, categories map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range settings {
			defaultCategory := categories[key]

			// 先查询是否存在
			var existing models.SystemSetting
			err := tx.Where("key = ?", key).First(&existing).Error

			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			// 确定要使用的 category
			actualCategory := defaultCategory
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 记录不存在，使用传入的 category（默认值）
				actualCategory = defaultCategory
			} else if existing.Category != "" {
				// 记录存在且有 category，保留原有的
				actualCategory = existing.Category
			}

			// 尝试更新
			result := tx.Model(&models.SystemSetting{}).
				Where("key = ?", key).
				Updates(map[string]interface{}{
					"value":    value,
					"category": actualCategory,
				})

			if result.Error != nil {
				return result.Error
			}

			// 如果没有更新任何行,则创建新记录
			if result.RowsAffected == 0 {
				setting := &models.SystemSetting{
					Key:      key,
					Value:    value,
					Category: actualCategory,
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
