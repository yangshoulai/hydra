package repository

import (
	"context"
	"errors"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// ProviderRepository 厂商仓储
type ProviderRepository struct {
	db *gorm.DB
}

// NewProviderRepository 创建厂商仓储
func NewProviderRepository(db *gorm.DB) *ProviderRepository {
	return &ProviderRepository{
		db: db,
	}
}

// Create 创建厂商
func (r *ProviderRepository) Create(ctx context.Context, provider *models.Provider) error {
	err := r.db.WithContext(ctx).Create(provider).Error
	return err
}

// FindByID 根据 ID 查询厂商
func (r *ProviderRepository) FindByID(ctx context.Context, id string) (*models.Provider, error) {
	var provider models.Provider
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&provider).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// FindByName 根据名称查询厂商
func (r *ProviderRepository) FindByName(ctx context.Context, name string) (*models.Provider, error) {
	var provider models.Provider
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&provider).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// List 查询厂商列表
func (r *ProviderRepository) List(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&providers).Error
	if err != nil {
		return nil, err
	}
	return providers, nil
}

// Update 更新厂商
func (r *ProviderRepository) Update(ctx context.Context, provider *models.Provider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// Delete 删除厂商
func (r *ProviderRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Provider{}, "id = ?", id).Error
}

// ExistsByID 检查 ID 是否存在
func (r *ProviderRepository) ExistsByID(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Provider{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsUsedByModels 检查厂商是否被模型使用
func (r *ProviderRepository) IsUsedByModels(ctx context.Context, providerID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Model{}).Where("provider_id = ?", providerID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
