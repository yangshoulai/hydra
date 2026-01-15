package modelsync

import (
	"log/slog"
	"sort"

	"github.com/yangshoulai/hydra/internal/models"
)

// ModelDiffType 模型差异类型
type ModelDiffType string

const (
	DiffTypeAdded    ModelDiffType = "added"    // 新增模型（上游有，本地没有）
	DiffTypeRemoved  ModelDiffType = "removed"  // 移除模型（本地有，上游没有）
	DiffTypeExisting ModelDiffType = "existing" // 存量模型（两边都有）
)

// ModelDiff 模型差异项
type ModelDiff struct {
	Type           ModelDiffType `json:"type"`
	UnifiedModel   string        `json:"unified_model"`
	UpstreamModel  string        `json:"upstream_model"`
	ExistingConfig *models.ChannelModelConfig `json:"existing_config,omitempty"` // 对于存量模型，包含现有配置
}

// SyncDiff 同步差异汇总
type SyncDiff struct {
	TotalUpstreamModels int          `json:"total_upstream_models"` // 上游模型总数
	TotalLocalModels    int          `json:"total_local_models"`    // 本地配置模型总数
	AddedCount          int          `json:"added_count"`           // 新增模型数
	RemovedCount        int          `json:"removed_count"`         // 移除模型数
	ExistingCount       int          `json:"existing_count"`        // 存量模型数
	Diffs               []ModelDiff  `json:"diffs"`                 // 差异详情
}

// DiffCalculator 模型差异计算器
type DiffCalculator struct {
	logger *slog.Logger
}

// NewDiffCalculator 创建差异计算器
func NewDiffCalculator(logger *slog.Logger) *DiffCalculator {
	return &DiffCalculator{
		logger: logger,
	}
}

// Calculate 计算上游模型和本地配置的差异
// upstreamModels: 上游返回的模型列表
// localConfigs: 本地的模型配置列表
func (dc *DiffCalculator) Calculate(
	upstreamModels []string,
	localConfigs []*models.ChannelModelConfig,
) *SyncDiff {
	// 构建上游模型集合
	upstreamSet := make(map[string]bool)
	for _, model := range upstreamModels {
		upstreamSet[model] = true
	}

	// 构建本地模型映射（unified_model -> config）
	localMap := make(map[string]*models.ChannelModelConfig)
	for _, config := range localConfigs {
		localMap[config.UpstreamModel] = config
	}

	diff := &SyncDiff{
		TotalUpstreamModels: len(upstreamModels),
		TotalLocalModels:    len(localConfigs),
		Diffs:               make([]ModelDiff, 0),
	}

	// 查找新增和存量模型
	for _, upstreamModel := range upstreamModels {
		if localConfig, exists := localMap[upstreamModel]; exists {
			// 存量模型
			diff.Diffs = append(diff.Diffs, ModelDiff{
				Type:           DiffTypeExisting,
				UnifiedModel:   localConfig.UnifiedModel,
				UpstreamModel:  upstreamModel,
				ExistingConfig: localConfig,
			})
			diff.ExistingCount++
		} else {
			// 新增模型
			diff.Diffs = append(diff.Diffs, ModelDiff{
				Type:          DiffTypeAdded,
				UnifiedModel:  "", // 需要用户手动指定
				UpstreamModel: upstreamModel,
			})
			diff.AddedCount++
		}
	}

	// 查找移除的模型
	for upstreamModel, localConfig := range localMap {
		if !upstreamSet[upstreamModel] {
			// 本地有但上游没有
			diff.Diffs = append(diff.Diffs, ModelDiff{
				Type:           DiffTypeRemoved,
				UnifiedModel:   localConfig.UnifiedModel,
				UpstreamModel:  upstreamModel,
				ExistingConfig: localConfig,
			})
			diff.RemovedCount++
		}
	}

	// 排序：按类型和模型名称排序
	sortDiffs(diff.Diffs)

	dc.logger.Info("model diff calculated",
		slog.Int("total_upstream", diff.TotalUpstreamModels),
		slog.Int("total_local", diff.TotalLocalModels),
		slog.Int("added", diff.AddedCount),
		slog.Int("removed", diff.RemovedCount),
		slog.Int("existing", diff.ExistingCount),
	)

	return diff
}

// sortDiffs 排序差异列表
// 顺序: added -> existing -> removed，同类型按模型名排序
func sortDiffs(diffs []ModelDiff) {
	sort.Slice(diffs, func(i, j int) bool {
		// 类型优先级
		typeOrder := map[ModelDiffType]int{
			DiffTypeAdded:    0,
			DiffTypeExisting: 1,
			DiffTypeRemoved:  2,
		}

		orderI := typeOrder[diffs[i].Type]
		orderJ := typeOrder[diffs[j].Type]

		if orderI != orderJ {
			return orderI < orderJ
		}

		// 同类型按上游模型名排序
		return diffs[i].UpstreamModel < diffs[j].UpstreamModel
	})
}

// GetAddedModels 获取新增的模型列表
func (sd *SyncDiff) GetAddedModels() []ModelDiff {
	result := make([]ModelDiff, 0)
	for _, diff := range sd.Diffs {
		if diff.Type == DiffTypeAdded {
			result = append(result, diff)
		}
	}
	return result
}

// GetRemovedModels 获取移除的模型列表
func (sd *SyncDiff) GetRemovedModels() []ModelDiff {
	result := make([]ModelDiff, 0)
	for _, diff := range sd.Diffs {
		if diff.Type == DiffTypeRemoved {
			result = append(result, diff)
		}
	}
	return result
}

// GetExistingModels 获取存量模型列表
func (sd *SyncDiff) GetExistingModels() []ModelDiff {
	result := make([]ModelDiff, 0)
	for _, diff := range sd.Diffs {
		if diff.Type == DiffTypeExisting {
			result = append(result, diff)
		}
	}
	return result
}
