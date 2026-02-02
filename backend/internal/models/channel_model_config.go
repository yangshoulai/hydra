package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// EndpointTypes 端点类型列表
type EndpointTypes []string

// KeyGroups 密钥分组列表
type KeyGroups []string

// Scan 实现 sql.Scanner 接口
func (e *EndpointTypes) Scan(value interface{}) error {
	if value == nil {
		*e = []string{"openai-chat"}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*e = []string{"openai-chat"}
		return nil
	}

	return json.Unmarshal(bytes, e)
}

// Value 实现 driver.Valuer 接口
func (e EndpointTypes) Value() (driver.Value, error) {
	if len(e) == 0 {
		e = []string{"openai-chat"}
	}
	return json.Marshal(e)
}

// Scan 实现 sql.Scanner 接口
func (k *KeyGroups) Scan(value interface{}) error {
	if value == nil {
		*k = []string{"Default"}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*k = []string{"Default"}
		return nil
	}

	return json.Unmarshal(bytes, k)
}

// Value 实现 driver.Valuer 接口
func (k KeyGroups) Value() (driver.Value, error) {
	if len(k) == 0 {
		k = []string{"Default"}
	}
	return json.Marshal(k)
}

// ChannelModelConfig 渠道模型配置
type ChannelModelConfig struct {
	ID            uint          `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	ChannelID     uint          `gorm:"not null;uniqueIndex:idx_channel_upstream" json:"channel_id"`
	UnifiedModel  string        `gorm:"type:varchar(100);not null;index:idx_unified_model" json:"unified_model"`
	UpstreamModel string        `gorm:"type:varchar(100);not null;uniqueIndex:idx_channel_upstream" json:"upstream_model"`
	Status        string        `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, cooling, disabled, non_exist
	CoolingAt     *time.Time    `gorm:"index" json:"cooling_at"`
	EndpointTypes EndpointTypes `gorm:"type:text;default:'[\"openai\"]'" json:"endpoint_types"`
	KeyGroups     KeyGroups     `gorm:"type:text;default:'[\"Default\"]'" json:"key_groups"`
	Remark        string        `gorm:"type:varchar(200)" json:"remark"`

	// Token 统计
	PromptTokens     int64 `gorm:"not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64 `gorm:"not null;default:0" json:"completion_tokens"`

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
