package models

import (
	"time"
)

// Key API Key 模型
type Key struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ChannelID uint       `gorm:"not null;index" json:"channel_id"`
	KeyValue  string     `gorm:"type:varchar(500);not null" json:"key_value"`
	Status    string     `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, cooling, disabled
	CoolingAt *time.Time `gorm:"index" json:"cooling_at"`
	KeyGroup  string     `gorm:"column:key_group;type:varchar(100);not null;default:'Default'" json:"key_group"`
	Remark    string     `gorm:"type:varchar(200)" json:"remark"`

	// Token 统计
	PromptTokens     int64 `gorm:"not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64 `gorm:"not null;default:0" json:"completion_tokens"`

	// 关联
	Channel *Channel `gorm:"foreignKey:ChannelID" json:"channel,omitempty"`
}

// TableName 指定表名
func (Key) TableName() string {
	return "keys"
}

// IsActive 检查 Key 是否可用
func (k *Key) IsActive() bool {
	return k.Status == "active"
}

// IsCooling 检查 Key 是否在冷却中
func (k *Key) IsCooling() bool {
	return k.Status == "cooling" && k.CoolingAt != nil && time.Now().Before(*k.CoolingAt)
}

// IsDisabled 检查 Key 是否已禁用
func (k *Key) IsDisabled() bool {
	return k.Status == "disabled"
}

// EnterCooling 进入冷却状态
func (k *Key) EnterCooling(duration time.Duration) {
	coolingUntil := time.Now().Add(duration)
	k.Status = "cooling"
	k.CoolingAt = &coolingUntil
}

// ExitCooling 退出冷却状态
func (k *Key) ExitCooling() {
	k.Status = "active"
	k.CoolingAt = nil
}
