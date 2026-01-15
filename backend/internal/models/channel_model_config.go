package models

import (
	"time"

	"gorm.io/gorm"
)

// ChannelModelConfig 渠道模型配置
type ChannelModelConfig struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	ChannelID      uint           `gorm:"not null;index:idx_channel_models" json:"channel_id"`
	UnifiedModel   string         `gorm:"type:varchar(100);not null;index:idx_unified_model" json:"unified_model"`
	UpstreamModel  string         `gorm:"type:varchar(100);not null;index:idx_channel_models" json:"upstream_model"`
	Status         string         `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, disabled
	Remark         string         `gorm:"type:varchar(200)" json:"remark"`

	// 关联
	Channel *Channel `gorm:"foreignKey:ChannelID" json:"channel,omitempty"`
}

// TableName 指定表名
func (ChannelModelConfig) TableName() string {
	return "channel_model_configs"
}

// IsActive 检查配置是否激活
func (c *ChannelModelConfig) IsActive() bool {
	return c.Status == "active"
}
