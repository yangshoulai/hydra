package repository

import (
	"context"
	"errors"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// AdminUserRepository 管理员用户仓储
type AdminUserRepository struct {
	db *gorm.DB
}

// NewAdminUserRepository 创建管理员用户仓储
func NewAdminUserRepository(db *gorm.DB) *AdminUserRepository {
	return &AdminUserRepository{db: db}
}

// Create 创建管理员用户
func (r *AdminUserRepository) Create(ctx context.Context, user *models.AdminUser) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindByID 根据ID查询用户
func (r *AdminUserRepository) FindByID(ctx context.Context, id uint) (*models.AdminUser, error) {
	var user models.AdminUser
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByUsername 根据用户名查询用户
func (r *AdminUserRepository) FindByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	var user models.AdminUser
	err := r.db.WithContext(ctx).
		Where("username = ?", username).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// List 查询所有用户
func (r *AdminUserRepository) List(ctx context.Context) ([]*models.AdminUser, error) {
	var users []*models.AdminUser
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&users).Error
	return users, err
}

// Update 更新用户
func (r *AdminUserRepository) Update(ctx context.Context, user *models.AdminUser) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// UpdatePassword 更新用户密码
func (r *AdminUserRepository) UpdatePassword(ctx context.Context, id uint, passwordHash string) error {
	return r.db.WithContext(ctx).
		Model(&models.AdminUser{}).
		Where("id = ?", id).
		Update("password_hash", passwordHash).Error
}

// UpdateLastLogin 更新最后登录时间
func (r *AdminUserRepository) UpdateLastLogin(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.AdminUser{}).
		Where("id = ?", id).
		Update("last_login_at", &now).Error
}

// Delete 删除用户
func (r *AdminUserRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.AdminUser{}, id).Error
}

// Count 统计用户数量
func (r *AdminUserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.AdminUser{}).Count(&count).Error
	return count, err
}
