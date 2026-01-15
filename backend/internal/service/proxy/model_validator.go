package proxy

import (
	"context"
	"errors"
	"log/slog"

	"github.com/yangshoulai/hydra/internal/repository"
)

var (
	// ErrModelNotSupported 模型不支持
	ErrModelNotSupported = errors.New("model not supported")
)

// ModelValidator 模型验证器
type ModelValidator struct {
	logger      *slog.Logger
	channelRepo *repository.ChannelRepository
}

// NewModelValidator 创建模型验证器
func NewModelValidator(logger *slog.Logger, channelRepo *repository.ChannelRepository) *ModelValidator {
	return &ModelValidator{
		logger:      logger,
		channelRepo: channelRepo,
	}
}

// ValidateModel 验证模型是否被系统支持
// 检查是否至少有一个激活的渠道支持该模型
func (mv *ModelValidator) ValidateModel(ctx context.Context, modelName string) error {
	if modelName == "" {
		mv.logger.Warn("empty model name provided")
		return ErrModelNotSupported
	}

	// 查询支持该模型的渠道
	channels, err := mv.channelRepo.FindByModel(ctx, modelName)
	if err != nil {
		mv.logger.Error("failed to validate model",
			slog.String("model", modelName),
			slog.String("error", err.Error()),
		)
		return err
	}

	// 检查是否有激活的渠道支持该模型
	hasActiveChannel := false
	for _, channel := range channels {
		if channel.IsActive() {
			hasActiveChannel = true
			break
		}
	}

	if !hasActiveChannel {
		mv.logger.Warn("model not supported by any active channel",
			slog.String("model", modelName),
			slog.Int("total_channels", len(channels)),
		)
		return ErrModelNotSupported
	}

	mv.logger.Debug("model validated successfully",
		slog.String("model", modelName),
		slog.Int("supporting_channels", len(channels)),
	)

	return nil
}

// GetSupportedModels 获取所有支持的模型列表
func (mv *ModelValidator) GetSupportedModels(ctx context.Context) ([]string, error) {
	// 获取所有激活的渠道
	channels, err := mv.channelRepo.FindAll(ctx)
	if err != nil {
		mv.logger.Error("failed to get all channels",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// 收集所有支持的模型
	modelsMap := make(map[string]bool)
	for _, channel := range channels {
		if !channel.IsActive() {
			continue
		}

		// 遍历该渠道的模型配置
		for _, config := range channel.ModelConfigs {
			if config.IsActive() {
				modelsMap[config.UnifiedModel] = true
			}
		}
	}

	// 转换为切片
	models := make([]string, 0, len(modelsMap))
	for model := range modelsMap {
		models = append(models, model)
	}

	mv.logger.Debug("retrieved supported models",
		slog.Int("total_models", len(models)),
	)

	return models, nil
}

// ModelInfo 模型信息
type ModelInfo struct {
	Name             string   `json:"name"`              // 统一模型名
	SupportedBy      []string `json:"supported_by"`      // 支持的渠道名称列表
	ChannelCount     int      `json:"channel_count"`     // 支持的渠道数量
	IsHighlyAvailable bool     `json:"is_highly_available"` // 是否高可用(多渠道支持)
}

// GetModelInfo 获取模型详细信息
func (mv *ModelValidator) GetModelInfo(ctx context.Context, modelName string) (*ModelInfo, error) {
	channels, err := mv.channelRepo.FindByModel(ctx, modelName)
	if err != nil {
		mv.logger.Error("failed to get model info",
			slog.String("model", modelName),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if len(channels) == 0 {
		mv.logger.Warn("model not found",
			slog.String("model", modelName),
		)
		return nil, ErrModelNotSupported
	}

	// 收集激活渠道的名称
	channelNames := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel.IsActive() {
			channelNames = append(channelNames, channel.Name)
		}
	}

	info := &ModelInfo{
		Name:             modelName,
		SupportedBy:      channelNames,
		ChannelCount:     len(channelNames),
		IsHighlyAvailable: len(channelNames) > 1,
	}

	mv.logger.Debug("model info retrieved",
		slog.String("model", modelName),
		slog.Int("channel_count", info.ChannelCount),
		slog.Bool("highly_available", info.IsHighlyAvailable),
	)

	return info, nil
}

// ValidateModelList 批量验证模型
// 返回: (有效的模型列表, 无效的模型列表)
func (mv *ModelValidator) ValidateModelList(ctx context.Context, models []string) ([]string, []string) {
	valid := make([]string, 0, len(models))
	invalid := make([]string, 0)

	for _, model := range models {
		err := mv.ValidateModel(ctx, model)
		if err == nil {
			valid = append(valid, model)
		} else {
			invalid = append(invalid, model)
		}
	}

	if len(invalid) > 0 {
		mv.logger.Warn("some models are not supported",
			slog.Int("valid_count", len(valid)),
			slog.Int("invalid_count", len(invalid)),
		)
	}

	return valid, invalid
}
