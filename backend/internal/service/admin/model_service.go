package admin

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// 错误定义
var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrModelNameExists   = errors.New("model name already exists")
	ErrModelNotFound     = errors.New("model not found")
	ErrModelInUse        = errors.New("model is in use by channel configurations")
)

// ModelService 统一模型服务
type ModelService struct {
	modelRepo       *repository.ModelRepository
	modelConfigRepo *repository.ChannelModelConfigRepository
	logger          *slog.Logger
}

// NewModelService 创建统一模型服务
func NewModelService(
	modelRepo *repository.ModelRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	logger *slog.Logger,
) *ModelService {
	return &ModelService{
		modelRepo:       modelRepo,
		modelConfigRepo: modelConfigRepo,
		logger:          logger,
	}
}

// CreateModelRequest 创建统一模型请求
type CreateModelRequest struct {
	Name       string  `json:"name" binding:"required,min=1,max=100"`
	ProviderID *string `json:"provider_id"`
	Remark     string  `json:"remark" binding:"omitempty,max=500"`
}

// UpdateModelRequest 更新统一模型请求
type UpdateModelRequest struct {
	Name       string  `json:"name" binding:"omitempty,min=1,max=100"`
	ProviderID *string `json:"provider_id,omitempty"`
	Remark     string  `json:"remark" binding:"omitempty,max=500"`
}

// Create 创建统一模型
func (s *ModelService) Create(ctx context.Context, req CreateModelRequest) (*models.Model, error) {
	// 标准化模型名称（去除前后空格，转为小写）
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrInvalidInput
	}

	// 检查名称是否已存在
	exists, err := s.modelRepo.ExistsByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrModelNameExists
	}

	model := &models.Model{
		Name:       name,
		ProviderID: req.ProviderID,
		Remark:     req.Remark,
	}

	if err := s.modelRepo.Create(ctx, model); err != nil {
		return nil, err
	}

	s.logger.Info("model created",
		slog.Uint64("id", uint64(model.ID)),
		slog.String("name", model.Name),
	)

	return model, nil
}

// Update 更新统一模型
func (s *ModelService) Update(ctx context.Context, id uint, req UpdateModelRequest) (*models.Model, error) {
	// 查询模型
	model, err := s.modelRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, ErrModelNotFound
	}

	// 如果更新名称，检查新名称是否已被其他模型使用
	if req.Name != "" && req.Name != model.Name {
		name := strings.TrimSpace(req.Name)
		exists, err := s.modelRepo.ExistsByName(ctx, name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrModelNameExists
		}
		model.Name = name
	}

	// 更新其他字段
	if req.ProviderID != nil {
		model.ProviderID = req.ProviderID
	}
	if req.Remark != "" {
		model.Remark = req.Remark
	}

	if err := s.modelRepo.Update(ctx, model); err != nil {
		return nil, err
	}

	s.logger.Info("model updated",
		slog.Uint64("id", uint64(model.ID)),
		slog.String("name", model.Name),
	)

	return model, nil
}

// Delete 删除统一模型
func (s *ModelService) Delete(ctx context.Context, id uint) error {
	// 查询模型
	model, err := s.modelRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if model == nil {
		return ErrModelNotFound
	}

	// 检查是否有渠道模型配置正在使用此模型
	configs, err := s.modelConfigRepo.FindByModelNameWithChannel(ctx, model.Name)
	if err != nil {
		return err
	}
	if len(configs) > 0 {
		return ErrModelInUse
	}

	if err := s.modelRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.logger.Info("model deleted",
		slog.Uint64("id", uint64(id)),
		slog.String("name", model.Name),
	)

	return nil
}

// List 查询统一模型列表（管理后台用，返回所有模型）
func (s *ModelService) List(ctx context.Context) ([]models.Model, error) {
	return s.modelRepo.List(ctx)
}

// ListWithActiveChannelConfigs 查询有激活渠道配置的模型列表（对外API用）
func (s *ModelService) ListWithActiveChannelConfigs(ctx context.Context) ([]models.Model, error) {
	return s.modelRepo.ListWithActiveChannelConfigs(ctx)
}

// FindByID 根据 ID 查询统一模型
func (s *ModelService) FindByID(ctx context.Context, id uint) (*models.Model, error) {
	return s.modelRepo.FindByID(ctx, id)
}
