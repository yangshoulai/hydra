package repository

import (
	"context"
	"log/slog"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ModelRepository 统一模型仓储
type ModelRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewModelRepository 创建统一模型仓储
func NewModelRepository(db *gorm.DB, logger *slog.Logger) *ModelRepository {
	return &ModelRepository{
		db:     db,
		logger: logger,
	}
}

// Create 创建统一模型
func (r *ModelRepository) Create(ctx context.Context, model *models.Model) error {
	err := r.db.WithContext(ctx).Create(model).Error
	if err != nil {
		r.logger.Error("failed to create model",
			slog.String("error", err.Error()),
			slog.String("name", model.Name),
		)
		return err
	}
	return nil
}

// FindByID 根据 ID 查询统一模型
func (r *ModelRepository) FindByID(ctx context.Context, id uint) (*models.Model, error) {
	var model models.Model
	err := r.db.WithContext(ctx).
		Preload("Provider").
		First(&model, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		r.logger.Error("failed to find model by id",
			slog.Uint64("id", uint64(id)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	return &model, nil
}

// FindByName 根据名称查询统一模型
func (r *ModelRepository) FindByName(ctx context.Context, name string) (*models.Model, error) {
	var model models.Model
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		r.logger.Error("failed to find model by name",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	return &model, nil
}

// List 查询统一模型列表
func (r *ModelRepository) List(ctx context.Context) ([]models.Model, error) {
	type ModelWithCount struct {
		models.Model
		ChannelCount int
	}

	var results []ModelWithCount
	err := r.db.WithContext(ctx).
		Table("models").
		Select("models.*, COUNT(DISTINCT channel_model_configs.channel_id) as channel_count").
		Joins("LEFT JOIN providers ON models.provider_id = providers.id").
		Joins("LEFT JOIN channel_model_configs ON channel_model_configs.unified_model = models.name").
		Group("models.id, providers.name, models.name").
		Order("providers.name ASC, models.name ASC").
		Scan(&results).Error
	if err != nil {
		r.logger.Error("failed to list models",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// 转换为 []models.Model 并预加载 Provider
	modelList := make([]models.Model, len(results))
	for i, result := range results {
		modelList[i] = result.Model
		modelList[i].ChannelCount = result.ChannelCount
	}

	// 预加载 Provider
	if len(modelList) > 0 {
		var providerIDs []string
		for _, m := range modelList {
			if m.ProviderID != nil {
				providerIDs = append(providerIDs, *m.ProviderID)
			}
		}

		if len(providerIDs) > 0 {
			var providers []models.Provider
			if err := r.db.WithContext(ctx).Where("id IN ?", providerIDs).Find(&providers).Error; err == nil {
				providerMap := make(map[string]*models.Provider)
				for i := range providers {
					providerMap[providers[i].ID] = &providers[i]
				}
				for i := range modelList {
					if modelList[i].ProviderID != nil {
						modelList[i].Provider = providerMap[*modelList[i].ProviderID]
					}
				}
			}
		}
	}

	return modelList, nil
}

// ListWithActiveChannelConfigs 查询有激活渠道配置的统一模型列表
// 返回在 models 表中存在，且在 channel_model_configs 中有配置，
// 且配置的渠道是 active 状态，且 channel_model_config 本身也是 active 状态的模型
func (r *ModelRepository) ListWithActiveChannelConfigs(ctx context.Context) ([]models.Model, error) {
	var modelList []models.Model
	err := r.db.WithContext(ctx).
		Distinct("models.*").
		Table("models").
		Joins("INNER JOIN channel_model_configs ON channel_model_configs.unified_model = models.name").
		Joins("INNER JOIN channels ON channel_model_configs.channel_id = channels.id").
		Where("channel_model_configs.status = ?", "active").
		Where("channels.status = ?", "active").
		Order("models.created_at DESC").
		Find(&modelList).Error
	if err != nil {
		r.logger.Error("failed to list models with active channel configs",
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	return modelList, nil
}

// Update 更新统一模型
func (r *ModelRepository) Update(ctx context.Context, model *models.Model) error {
	err := r.db.WithContext(ctx).Save(model).Error
	if err != nil {
		r.logger.Error("failed to update model",
			slog.Uint64("id", uint64(model.ID)),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

// Delete 删除统一模型
func (r *ModelRepository) Delete(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&models.Model{}, id).Error
	if err != nil {
		r.logger.Error("failed to delete model",
			slog.Uint64("id", uint64(id)),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

// ExistsByName 检查名称是否存在
func (r *ModelRepository) ExistsByName(ctx context.Context, name string, excludeID ...uint) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.Model{}).Where("name = ?", name)

	if len(excludeID) > 0 && excludeID[0] > 0 {
		query = query.Where("id != ?", excludeID[0])
	}

	err := query.Count(&count).Error
	if err != nil {
		r.logger.Error("failed to check model name existence",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
		return false, err
	}
	return count > 0, nil
}

// BatchCreate 批量创建统一模型（用于 upsert）
func (r *ModelRepository) BatchCreate(ctx context.Context, models []*models.Model) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"provider_id", "remark", "updated_at"}),
	}).Create(&models).Error
	if err != nil {
		r.logger.Error("failed to batch create models",
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}
