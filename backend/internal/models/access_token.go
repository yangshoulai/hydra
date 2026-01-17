package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// AccessToken 访问令牌模型
type AccessToken struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Token        string     `gorm:"type:varchar(255)" json:"token,omitempty"`                                   // 明文令牌（仅创建时返回）
	TokenHash    string     `gorm:"type:varchar(64);not null;uniqueIndex" json:"-"`                            // SHA256 哈希
	TokenPreview string     `gorm:"type:varchar(20)" json:"token_preview"`                                     // 脱敏令牌(前8位+后4位)
	Status       string     `gorm:"type:varchar(20);not null;default:'active'" json:"status"`                  // active, disabled
	Name         string     `gorm:"type:varchar(20);not null;uniqueIndex" json:"name"`                         // 令牌名称
	LastUsedAt   *time.Time `json:"last_used_at"`
	ExpiresAt    *time.Time `json:"expires_at"` // 过期时间，nil 表示永不过期
}

// TableName 指定表名
func (AccessToken) TableName() string {
	return "access_tokens"
}

// IsActive 检查令牌是否激活且未过期
func (a *AccessToken) IsActive() bool {
	if a.Status != "active" {
		return false
	}

	// 如果设置了过期时间，检查是否过期
	if a.ExpiresAt != nil && time.Now().After(*a.ExpiresAt) {
		return false
	}

	return true
}

// IsExpired 检查令牌是否已过期
func (a *AccessToken) IsExpired() bool {
	if a.ExpiresAt == nil {
		return false // 没有过期时间，永不过期
	}
	return time.Now().After(*a.ExpiresAt)
}

// HashToken 对令牌进行 SHA256 哈希
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// MaskToken 生成脱敏令牌预览（前6位 + ********** + 后4位）
func MaskToken(token string) string {
	if len(token) < 10 {
		return token // 如果token太短，不做脱敏
	}
	return token[:6] + "**********" + token[len(token)-4:]
}
