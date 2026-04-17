package models

import (
	"time"
)

// ChannelKey 渠道密钥模型
type ChannelKey struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	ChannelID       uint      `gorm:"not null;index" json:"channel_id"`
	ChannelKeyValue string    `gorm:"column:channel_key_value;type:varchar(500);not null" json:"channel_key_value"`
	Status          string    `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, inactive
	ChannelKeyGroup string    `gorm:"column:channel_key_group;type:varchar(100);not null;default:'Default'" json:"channel_key_group"`
	Remark          string    `gorm:"type:varchar(200)" json:"remark"`

	// Token 统计
	PromptTokens     int64 `gorm:"not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64 `gorm:"not null;default:0" json:"completion_tokens"`

	// 关联
	Channel *Channel `gorm:"foreignKey:ChannelID" json:"channel,omitempty"`
}

// TableName 指定表名
func (ChannelKey) TableName() string {
	return "channel_keys"
}

// IsActive 检查渠道密钥是否可用
func (k *ChannelKey) IsActive() bool {
	return k.Status == "active"
}
