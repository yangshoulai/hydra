package models

import (
	"time"
)

// Channel 渠道模型
type Channel struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	BaseURL     string    `gorm:"type:varchar(500);not null" json:"base_url"`
	UseProxy    bool      `gorm:"not null;default:false" json:"use_proxy"`
	Weight      int       `gorm:"not null;default:100;index" json:"weight"`                 // 仅作为渠道模型配置的初始权重来源
	Status      string    `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, inactive
	Description string    `gorm:"type:text" json:"description"`

	// 关联
	ChannelKeys  []ChannelKey         `gorm:"foreignKey:ChannelID;constraint:OnDelete:CASCADE" json:"channel_keys,omitempty"`
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
