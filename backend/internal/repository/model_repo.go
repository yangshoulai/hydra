package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// ModelRepository 统一模型仓储
type ModelRepository struct {
	db *gorm.DB
}

// NewModelRepository 创建统一模型仓储
func NewModelRepository(db *gorm.DB) *ModelRepository {
	return &ModelRepository{
		db: db,
	}
}

// Create 创建统一模型
func (r *ModelRepository) Create(ctx context.Context, model *models.Model) error {
	return r.db.WithContext(ctx).Create(model).Error
}

// FindByID 根据 ID 查询统一模型
func (r *ModelRepository) FindByID(ctx context.Context, id uint) (*models.Model, error) {
	var model models.Model
	err := r.db.WithContext(ctx).
		Preload("Provider").
		First(&model, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

// List 查询统一模型列表
func (r *ModelRepository) List(ctx context.Context) ([]models.Model, error) {
	list, _, err := r.ListWithFilter(ctx, 0, 0, nil, nil)
	return list, err
}

// ModelFilter 模型过滤选项
type ModelFilter struct {
	Name       string // 模型名称模糊查询
	ProviderID string // 厂商ID精确查询
}

// ModelSortOptions 模型排序选项
type ModelSortOptions struct {
	Field     string // 排序字段：id, name
	Direction string // 排序方向：asc, desc
}

// ListWithFilter 分页查询模型列表（带过滤和排序）
func (r *ModelRepository) ListWithFilter(
	ctx context.Context,
	offset, limit int,
	filter *ModelFilter,
	sortOpts *ModelSortOptions,
) ([]models.Model, int64, error) {
	query := r.db.WithContext(ctx).
		Table("models").
		Select("models.*").
		Joins("LEFT JOIN providers ON models.provider_id = providers.id")

	// 应用过滤条件
	if filter != nil {
		if filter.Name != "" {
			name := strings.ToLower(filter.Name)
			query = query.Where("LOWER(models.name) LIKE ?", "%"+name+"%")
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
			name := strings.ToLower(filter.Name)
			countQuery = countQuery.Where("LOWER(name) LIKE ?", "%"+name+"%")
		}
		if filter.ProviderID != "" {
			countQuery = countQuery.Where("provider_id = ?", filter.ProviderID)
		}
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
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
			"id":   true,
			"name": true,
		}

		if allowedFields[sortOpts.Field] {
			orderBy = "models." + sortOpts.Field + " " + direction
		}
	}

	// 应用分页
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}

	// 执行查询
	query = query.Order(orderBy)

	var modelList []models.Model
	if err := query.Scan(&modelList).Error; err != nil {
		return nil, 0, err
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

	return modelList, total, nil
}

// ListWithActiveChannelConfigs 查询有激活渠道配置的统一模型列表
// 返回在 models 表中存在，且在 channel_model_configs 中有配置，
// 且配置的渠道是 active 状态，且 channel_model_config 本身也是 active 状态的模型
func (r *ModelRepository) ListWithActiveChannelConfigs(ctx context.Context) ([]models.Model, error) {
	var modelList []models.Model
	err := r.db.WithContext(ctx).
		Distinct("models.*").
		Table("models").
		Joins("INNER JOIN channel_model_configs ON channel_model_configs.model = models.name").
		Joins("INNER JOIN channels ON channel_model_configs.channel_id = channels.id").
		Where("channel_model_configs.status = ?", "active").
		Where("channels.status = ?", "active").
		Order("models.created_at DESC").
		Find(&modelList).Error
	if err != nil {
		return nil, err
	}
	return modelList, nil
}

// ListWithActiveChannelConfigsByEndpointType 查询指定端点类型的统一模型列表
func (r *ModelRepository) ListWithActiveChannelConfigsByEndpointType(ctx context.Context, endpointType string) ([]models.Model, error) {
	var modelList []models.Model
	likePattern := "%\"" + endpointType + "\"%"
	err := r.db.WithContext(ctx).
		Distinct("models.*").
		Table("models").
		Joins("INNER JOIN channel_model_configs ON channel_model_configs.model = models.name").
		Joins("INNER JOIN channels ON channel_model_configs.channel_id = channels.id").
		Where("channel_model_configs.status = ?", "active").
		Where("channels.status = ?", "active").
		Where("channel_model_configs.endpoint_types LIKE ?", likePattern).
		Order("models.created_at DESC").
		Find(&modelList).Error
	if err != nil {
		return nil, err
	}
	return modelList, nil
}

// Update 更新统一模型
func (r *ModelRepository) Update(ctx context.Context, model *models.Model) error {
	return r.db.WithContext(ctx).Save(model).Error
}

// Delete 删除统一模型
func (r *ModelRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Model{}, id).Error
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
		return false, err
	}
	return count > 0, nil
}
