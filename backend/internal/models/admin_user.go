package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminUser 管理员用户模型
type AdminUser struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	Username     string         `gorm:"type:varchar(50);not null;uniqueIndex" json:"username"`
	PasswordHash string         `gorm:"type:varchar(100);not null" json:"-"`
	Status       string         `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, disabled
	LastLoginAt  *time.Time     `json:"last_login_at"`
}

// TableName 指定表名
func (AdminUser) TableName() string {
	return "admin_users"
}

// IsActive 检查用户是否激活
func (a *AdminUser) IsActive() bool {
	return a.Status == "active"
}

// SetPassword 设置密码(自动哈希)
func (a *AdminUser) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	a.PasswordHash = string(hash)
	return nil
}

// CheckPassword 验证密码
func (a *AdminUser) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password))
	return err == nil
}
