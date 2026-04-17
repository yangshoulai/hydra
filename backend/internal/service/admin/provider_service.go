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
	ErrProviderIdExists = errors.New("provider id already exists")
	ErrProviderNotFound = errors.New("provider not found")
	ErrProviderInUse    = errors.New("provider is in use by models")
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
	ID     string `json:"id" binding:"required,min=1,max=50"`
	Name   string `json:"name" binding:"required,min=1,max=100"`
	Icon   string `json:"icon" binding:"omitempty,max=500"`
	Remark string `json:"remark" binding:"omitempty,max=500"`
}

// UpdateProviderRequest 更新厂商请求
type UpdateProviderRequest struct {
	Name   string `json:"name" binding:"omitempty,min=1,max=100"`
	Icon   string `json:"icon" binding:"omitempty,max=500"`
	Remark string `json:"remark" binding:"omitempty,max=500"`
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
		ID:     id,
		Name:   name,
		Icon:   req.Icon,
		Remark: req.Remark,
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

// ProviderListItem 厂商列表项（嵌入原厂商 + 扩展统计）
type ProviderListItem struct {
	models.Provider
	ModelCount int64 `json:"model_count"`
}

// List 查询厂商列表（带每个厂商的模型数量）
func (s *ProviderService) List(ctx context.Context) ([]ProviderListItem, error) {
	providers, err := s.providerRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(providers))
	for _, p := range providers {
		ids = append(ids, p.ID)
	}
	counts, err := s.providerRepo.CountModelsByProviders(ctx, ids)
	if err != nil {
		s.logger.Warn("聚合厂商模型数失败",
			slog.String("error", err.Error()),
		)
		counts = map[string]int64{}
	}
	items := make([]ProviderListItem, 0, len(providers))
	for _, p := range providers {
		items = append(items, ProviderListItem{
			Provider:   p,
			ModelCount: counts[p.ID],
		})
	}
	return items, nil
}

// FindByID 根据 ID 查询厂商
func (s *ProviderService) FindByID(ctx context.Context, id string) (*models.Provider, error) {
	return s.providerRepo.FindByID(ctx, id)
}
