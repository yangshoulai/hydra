package repository

import (
	"context"
	"log/slog"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// ProviderRepository 厂商仓储
type ProviderRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewProviderRepository 创建厂商仓储
func NewProviderRepository(db *gorm.DB, logger *slog.Logger) *ProviderRepository {
	return &ProviderRepository{
		db:     db,
		logger: logger,
	}
}

// Create 创建厂商
func (r *ProviderRepository) Create(ctx context.Context, provider *models.Provider) error {
	err := r.db.WithContext(ctx).Create(provider).Error
	if err != nil {
		r.logger.Error("failed to create provider",
			slog.String("error", err.Error()),
			slog.String("id", provider.ID),
			slog.String("name", provider.Name),
		)
		return err
	}
	return nil
}

// FindByID 根据 ID 查询厂商
func (r *ProviderRepository) FindByID(ctx context.Context, id string) (*models.Provider, error) {
	var provider models.Provider
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&provider).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		r.logger.Error("failed to find provider by id",
			slog.String("id", id),
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	return &provider, nil
}

// FindByName 根据名称查询厂商
func (r *ProviderRepository) FindByName(ctx context.Context, name string) (*models.Provider, error) {
	var provider models.Provider
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&provider).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		r.logger.Error("failed to find provider by name",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	return &provider, nil
}

// List 查询厂商列表
func (r *ProviderRepository) List(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&providers).Error
	if err != nil {
		r.logger.Error("failed to list providers",
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	return providers, nil
}

// Update 更新厂商
func (r *ProviderRepository) Update(ctx context.Context, provider *models.Provider) error {
	err := r.db.WithContext(ctx).Save(provider).Error
	if err != nil {
		r.logger.Error("failed to update provider",
			slog.String("id", provider.ID),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

// Delete 删除厂商
func (r *ProviderRepository) Delete(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Delete(&models.Provider{}, "id = ?", id).Error
	if err != nil {
		r.logger.Error("failed to delete provider",
			slog.String("id", id),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

// ExistsByID 检查 ID 是否存在
func (r *ProviderRepository) ExistsByID(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Provider{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		r.logger.Error("failed to check provider id existence",
			slog.String("id", id),
			slog.String("error", err.Error()),
		)
		return false, err
	}
	return count > 0, nil
}

// IsUsedByModels 检查厂商是否被模型使用
func (r *ProviderRepository) IsUsedByModels(ctx context.Context, providerID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Model{}).Where("provider_id = ?", providerID).Count(&count).Error
	if err != nil {
		r.logger.Error("failed to check if provider is used by models",
			slog.String("provider_id", providerID),
			slog.String("error", err.Error()),
		)
		return false, err
	}
	return count > 0, nil
}
