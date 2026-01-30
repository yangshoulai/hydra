package admin

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// Provider 相关错误定义
var (
	ErrProviderIdExists   = errors.New("provider id already exists")
	ErrProviderNameExists = errors.New("provider name already exists")
	ErrProviderNotFound   = errors.New("provider not found")
	ErrProviderInUse      = errors.New("provider is in use by models")
)

// ProviderService 厂商服务
type ProviderService struct {
	providerRepo *repository.ProviderRepository
	logger       *slog.Logger
}

// NewProviderService 创建厂商服务
func NewProviderService(
	providerRepo *repository.ProviderRepository,
	logger *slog.Logger,
) *ProviderService {
	return &ProviderService{
		providerRepo: providerRepo,
		logger:       logger,
	}
}

// CreateProviderRequest 创建厂商请求
type CreateProviderRequest struct {
	ID       string `json:"id" binding:"required,min=1,max=50"`
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Icon     string `json:"icon" binding:"omitempty,max=500"`
	LobeIcon string `json:"lobeIcon" binding:"omitempty,max=100"`
	Remark   string `json:"remark" binding:"omitempty,max=500"`
}

// UpdateProviderRequest 更新厂商请求
type UpdateProviderRequest struct {
	Name     string `json:"name" binding:"omitempty,min=1,max=100"`
	Icon     string `json:"icon" binding:"omitempty,max=500"`
	LobeIcon string `json:"lobeIcon" binding:"omitempty,max=100"`
	Remark   string `json:"remark" binding:"omitempty,max=500"`
}

// Create 创建厂商
func (s *ProviderService) Create(ctx context.Context, req CreateProviderRequest) (*models.Provider, error) {
	// 标准化厂商 ID（去除前后空格，转为小写）
	id := strings.TrimSpace(strings.ToLower(req.ID))
	if id == "" {
		return nil, errors.New("invalid input")
	}

	// 标准化厂商名称
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("invalid input")
	}

	// 检查 ID 是否已存在
	exists, err := s.providerRepo.ExistsByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrProviderIdExists
	}

	provider := &models.Provider{
		ID:       id,
		Name:     name,
		Icon:     req.Icon,
		LobeIcon: req.LobeIcon,
		Remark:   req.Remark,
	}

	if err := s.providerRepo.Create(ctx, provider); err != nil {
		return nil, err
	}

	s.logger.Info("供应商已创建",
		slog.String("id", provider.ID),
		slog.String("name", provider.Name),
	)

	return provider, nil
}

// Update 更新厂商
func (s *ProviderService) Update(ctx context.Context, id string, req UpdateProviderRequest) (*models.Provider, error) {
	// 查询厂商
	provider, err := s.providerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, ErrProviderNotFound
	}

	// 更新字段
	if req.Name != "" {
		provider.Name = strings.TrimSpace(req.Name)
	}
	if req.Icon != "" {
		provider.Icon = req.Icon
	}
	if req.LobeIcon != "" {
		provider.LobeIcon = req.LobeIcon
	}
	if req.Remark != "" {
		provider.Remark = req.Remark
	}

	if err := s.providerRepo.Update(ctx, provider); err != nil {
		return nil, err
	}

	s.logger.Info("供应商已更新",
		slog.String("id", provider.ID),
		slog.String("name", provider.Name),
	)

	return provider, nil
}

// Delete 删除厂商
func (s *ProviderService) Delete(ctx context.Context, id string) error {
	// 查询厂商
	provider, err := s.providerRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if provider == nil {
		return ErrProviderNotFound
	}

	// 检查是否被模型使用
	inUse, err := s.providerRepo.IsUsedByModels(ctx, id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrProviderInUse
	}

	if err := s.providerRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.logger.Info("供应商已删除",
		slog.String("id", id),
		slog.String("name", provider.Name),
	)

	return nil
}

// List 查询厂商列表
func (s *ProviderService) List(ctx context.Context) ([]models.Provider, error) {
	return s.providerRepo.List(ctx)
}

// FindByID 根据 ID 查询厂商
func (s *ProviderService) FindByID(ctx context.Context, id string) (*models.Provider, error) {
	return s.providerRepo.FindByID(ctx, id)
}

// FindByName 根据名称查询厂商
func (s *ProviderService) FindByName(ctx context.Context, name string) (*models.Provider, error) {
	return s.providerRepo.FindByName(ctx, name)
}
