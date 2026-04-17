package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// AccessTokenRepository 访问令牌仓储
type AccessTokenRepository struct {
	db *gorm.DB
}

// NewAccessTokenRepository 创建访问令牌仓储
func NewAccessTokenRepository(db *gorm.DB) *AccessTokenRepository {
	return &AccessTokenRepository{db: db}
}

// hashToken 对令牌值进行 SHA256 哈希
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Create 创建访问令牌 (TokenHash should be set before calling)
func (r *AccessTokenRepository) Create(ctx context.Context, token *models.AccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// FindByID 根据ID查询令牌
func (r *AccessTokenRepository) FindByID(ctx context.Context, id uint) (*models.AccessToken, error) {
	var token models.AccessToken
	err := r.db.WithContext(ctx).First(&token, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

// FindByToken 根据令牌值查询(用于认证)
func (r *AccessTokenRepository) FindByToken(ctx context.Context, tokenValue string) (*models.AccessToken, error) {
	var token models.AccessToken
	hashedToken := hashToken(tokenValue)

	err := r.db.WithContext(ctx).
		Where("token_hash = ?", hashedToken).
		First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid token")
		}
		return nil, err
	}
	return &token, nil
}

// List 查询所有令牌
func (r *AccessTokenRepository) List(ctx context.Context) ([]*models.AccessToken, error) {
	tokens, _, err := r.ListWithFilter(ctx, 0, 0, nil, nil)
	return tokens, err
}

// AccessTokenFilter 令牌过滤选项
type AccessTokenFilter struct {
	Name   string // 名称模糊查询
	Status string // 状态精确查询: active, disabled
	Token  string // 令牌模糊查询
}

// AccessTokenSortOptions 令牌排序选项
type AccessTokenSortOptions struct {
	Field     string // 排序字段：id, status, created_at, last_used_at
	Direction string // 排序方向：asc, desc
}

// ListWithFilter 分页查询令牌列表（带过滤和排序）
func (r *AccessTokenRepository) ListWithFilter(
	ctx context.Context,
	offset, limit int,
	filter *AccessTokenFilter,
	sortOpts *AccessTokenSortOptions,
) ([]*models.AccessToken, int64, error) {
	var tokens []*models.AccessToken
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AccessToken{})

	// 应用过滤条件
	if filter != nil {
		if filter.Name != "" {
			query = query.Where("name LIKE ?", "%"+filter.Name+"%")
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.Token != "" {
			query = query.Where("token LIKE ?", "%"+filter.Token+"%")
		}
	}

	// 查询总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 构建排序
	orderBy := "created_at DESC" // 默认排序：创建时间倒序
	if sortOpts != nil && sortOpts.Field != "" {
		direction := "ASC"
		if sortOpts.Direction == "desc" {
			direction = "DESC"
		}

		// 验证排序字段，防止 SQL 注入
		allowedFields := map[string]bool{
			"id":           true,
			"status":       true,
			"created_at":   true,
			"last_used_at": true,
		}

		if allowedFields[sortOpts.Field] {
			orderBy = sortOpts.Field + " " + direction
		}
	}

	// 分页查询
	err := query.
		Offset(offset).
		Limit(limit).
		Order(orderBy).
		Find(&tokens).Error

	return tokens, total, err
}

// Update 更新令牌
func (r *AccessTokenRepository) Update(ctx context.Context, token *models.AccessToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}

// IncrementTokenUsage 累加访问令牌的 token 使用量
func (r *AccessTokenRepository) IncrementTokenUsage(ctx context.Context, id uint, promptTokens, completionTokens int64) error {
	return r.db.WithContext(ctx).
		Model(&models.AccessToken{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"prompt_tokens":     gorm.Expr("prompt_tokens + ?", promptTokens),
			"completion_tokens": gorm.Expr("completion_tokens + ?", completionTokens),
		}).Error
}

// UpdateLastUsed 更新最后使用时间
func (r *AccessTokenRepository) UpdateLastUsed(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.AccessToken{}).
		Where("id = ?", id).
		Update("last_used_at", &now).Error
}

// Delete 删除令牌
func (r *AccessTokenRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.AccessToken{}, id).Error
}

// ToggleStatus 切换令牌状态
func (r *AccessTokenRepository) ToggleStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).
		Model(&models.AccessToken{}).
		Where("id = ?", id).
		Update("status", status).Error
}
