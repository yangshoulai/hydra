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
	list, _, _, err := r.ListWithFilter(ctx, 0, 0, nil, nil, nil)
	return list, err
}

// ModelFilter 模型过滤选项
type ModelFilter struct {
	Name       string // 模型名称模糊查询
	ProviderID string // 厂商ID精确查询
}

// ModelSortOptions 模型排序选项
type ModelSortOptions struct {
	Field     string // 排序字段：id, name, channel_count
	Direction string // 排序方向：asc, desc
}

// ListWithFilter 分页查询模型列表（带过滤和排序）
func (r *ModelRepository) ListWithFilter(
	ctx context.Context,
	offset, limit int,
	filter *ModelFilter,
	sortOpts *ModelSortOptions,
	channelCount *bool, // 是否返回渠道数量
) ([]models.Model, int64, int, error) {
	type ModelWithCount struct {
		models.Model
		ChannelCount int
	}

	query := r.db.WithContext(ctx).
		Table("models").
		Select("models.*, COUNT(DISTINCT channel_model_configs.channel_id) as channel_count").
		Joins("LEFT JOIN providers ON models.provider_id = providers.id").
		Joins("LEFT JOIN channel_model_configs ON channel_model_configs.unified_model = models.name")

	// 应用过滤条件
	if filter != nil {
		if filter.Name != "" {
			query = query.Where("models.name LIKE ?", "%"+filter.Name+"%")
		}
		if filter.ProviderID != "" {
			query = query.Where("models.provider_id = ?", filter.ProviderID)
		}
	}

	// 查询总数
	var total int64
	countQuery := r.db.WithContext(ctx).Table("models")
	if filter != nil {
		if filter.Name != "" {
			countQuery = countQuery.Where("name LIKE ?", "%"+filter.Name+"%")
		}
		if filter.ProviderID != "" {
			countQuery = countQuery.Where("provider_id = ?", filter.ProviderID)
		}
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	// 计算总数时过滤后的渠道配置总数
	var totalChannelConfigs int
	if channelCount != nil && *channelCount {
		var results []ModelWithCount
		tempQuery := query.Group("models.id, providers.name")
		if err := tempQuery.Scan(&results).Error; err != nil {
			return nil, 0, 0, err
		}
		totalChannelConfigs = len(results)
	}

	// 构建排序
	orderBy := "providers.name ASC, models.name ASC" // 默认排序
	if sortOpts != nil && sortOpts.Field != "" {
		direction := "ASC"
		if sortOpts.Direction == "desc" {
			direction = "DESC"
		}

		// 验证排序字段，防止 SQL 注入
		allowedFields := map[string]bool{
			"id":             true,
			"name":           true,
			"channel_count":  true,
		}

		if allowedFields[sortOpts.Field] {
			if sortOpts.Field == "channel_count" {
				orderBy = "channel_count " + direction + ", providers.name ASC, models.name ASC"
			} else {
				orderBy = "models." + sortOpts.Field + " " + direction
			}
		}
	}

	// 应用分页
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}

	// 执行查询
	query = query.Group("models.id, providers.name").Order(orderBy)

	var results []ModelWithCount
	if err := query.Scan(&results).Error; err != nil {
		r.logger.Error("failed to list models",
			slog.String("error", err.Error()),
		)
		return nil, 0, 0, err
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

	return modelList, total, totalChannelConfigs, nil
}

// List 查询统一模型列表（已废弃，使用 ListWithFilter）
// func (r *ModelRepository) List(ctx context.Context) ([]models.Model, error) {

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
