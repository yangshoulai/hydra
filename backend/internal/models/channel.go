package models

import (
	"time"
)

// Channel 渠道模型
type Channel struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Name         string     `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	BaseURL      string     `gorm:"type:varchar(500);not null" json:"base_url"`
	Priority     int        `gorm:"not null;default:100;index" json:"priority"`
	Weight       int        `gorm:"not null;default:100" json:"weight"`
	Status       string     `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, disabled
	Description  string     `gorm:"type:text" json:"description"`
	LastSyncTime *time.Time `json:"last_sync_time,omitempty"`
	SyncEnabled  bool       `gorm:"not null;default:true" json:"sync_enabled"`

	// 关联
	Keys         []Key                `gorm:"foreignKey:ChannelID;constraint:OnDelete:CASCADE" json:"keys,omitempty"`
	ModelConfigs []ChannelModelConfig `gorm:"foreignKey:ChannelID;constraint:OnDelete:CASCADE" json:"model_configs,omitempty"`
}

// TableName 指定表名
func (Channel) TableName() string {
	return "channels"
}

// IsActive 检查渠道是否激活
func (c *Channel) IsActive() bool {
	return c.Status == "active"
}
