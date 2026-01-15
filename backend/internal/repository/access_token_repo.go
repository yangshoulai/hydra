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

// FindByTokenHash 根据令牌哈希查询
func (r *AccessTokenRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*models.AccessToken, error) {
	var token models.AccessToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// List 查询所有令牌
func (r *AccessTokenRepository) List(ctx context.Context) ([]*models.AccessToken, error) {
	var tokens []*models.AccessToken
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&tokens).Error
	return tokens, err
}

// Update 更新令牌
func (r *AccessTokenRepository) Update(ctx context.Context, token *models.AccessToken) error {
	return r.db.WithContext(ctx).Save(token).Error
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
