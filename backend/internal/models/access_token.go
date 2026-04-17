package models

import (
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// AllowedModels 令牌允许访问的模型列表
// 约定：
// - 空数组或 nil 表示“不限制模型”（允许访问所有模型）
// - 非空数组表示白名单
//
// 为便于跨数据库兼容，使用 JSON 文本存储。
type AllowedModels []string

// Scan 实现 sql.Scanner
func (a *AllowedModels) Scan(value any) error {
	if value == nil {
		*a = nil
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		*a = nil
		return nil
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		*a = nil
		return nil
	}

	var models []string
	if err := json.Unmarshal(raw, &models); err != nil {
		return err
	}

	*a = normalizeAllowedModels(models)
	return nil
}

// Value 实现 driver.Valuer
func (a AllowedModels) Value() (driver.Value, error) {
	normalized := normalizeAllowedModels(a)
	if len(normalized) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// IsRestricted 是否启用模型白名单
func (a AllowedModels) IsRestricted() bool {
	return len(a) > 0
}

// Contains 是否包含模型
func (a AllowedModels) Contains(model string) bool {
	if len(a) == 0 {
		return true
	}
	for _, item := range a {
		if item == model {
			return true
		}
	}
	return false
}

func normalizeAllowedModels(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	normalized := make([]string, 0, len(raw))
	for _, item := range raw {
		model := strings.TrimSpace(item)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		normalized = append(normalized, model)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// AccessToken 访问令牌模型
type AccessToken struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Token        string     `gorm:"type:varchar(255)" json:"token,omitempty"`                 // 明文令牌（仅创建时返回）
	TokenHash    string     `gorm:"type:varchar(64);not null;uniqueIndex" json:"-"`           // SHA256 哈希
	TokenPreview string     `gorm:"type:varchar(20)" json:"token_preview"`                    // 脱敏令牌(前8位+后4位)
	Status       string     `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, disabled
	Name         string     `gorm:"type:varchar(20);not null;uniqueIndex" json:"name"`        // 令牌名称
	LastUsedAt   *time.Time `json:"last_used_at"`
	ExpiresAt    *time.Time `json:"expires_at"` // 过期时间，nil 表示永不过期

	// AllowedModels 模型白名单（空表示不限制）
	AllowedModels AllowedModels `gorm:"type:text;default:'[]'" json:"allowed_models"`

	// Token 统计
	PromptTokens     int64 `gorm:"not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64 `gorm:"not null;default:0" json:"completion_tokens"`
}

// TableName 指定表名
func (a *AccessToken) TableName() string {
	return "access_tokens"
}

// IsActive 检查令牌是否激活且未过期
func (a *AccessToken) IsActive() bool {
	if a.Status != "active" {
		return false
	}

	return !a.IsExpired()
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
