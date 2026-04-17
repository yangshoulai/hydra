package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/yangshoulai/hydra/internal/endpoint"
)

// EndpointTypes 端点类型列表
type EndpointTypes []string

// KeyGroups 密钥分组列表
type KeyGroups []string

// Scan 实现 sql.Scanner 接口
func (e *EndpointTypes) Scan(value any) error {
	if value == nil {
		*e = []string{endpoint.TypeOpenAIChatCompletions}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*e = []string{endpoint.TypeOpenAIChatCompletions}
		return nil
	}

	return json.Unmarshal(bytes, e)
}

// Value 实现 driver.Valuer 接口
func (e EndpointTypes) Value() (driver.Value, error) {
	if len(e) == 0 {
		e = []string{endpoint.TypeOpenAIChatCompletions}
	}
	return json.Marshal(e)
}

// Scan 实现 sql.Scanner 接口
func (k *KeyGroups) Scan(value any) error {
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
	ChannelID     uint          `gorm:"not null;uniqueIndex:idx_channel_model_configs_channel_model" json:"channel_id"`
	Model         string        `gorm:"type:varchar(100);not null;index:idx_channel_model_configs_model" json:"model"`
	ChannelModel  string        `gorm:"type:varchar(100);not null;uniqueIndex:idx_channel_model_configs_channel_model" json:"channel_model"`
	Weight        int           `gorm:"not null;default:100;index:idx_channel_model_configs_weight" json:"weight"` // 越大权重越高
	Status        string        `gorm:"type:varchar(20);not null;default:'active'" json:"status"`                  // active, inactive
	EndpointTypes EndpointTypes `gorm:"type:text;default:'[\"OpenAIChatCompletions\"]'" json:"endpoint_types"`
	KeyGroups     KeyGroups     `gorm:"type:text;default:'[\"Default\"]'" json:"key_groups"`
	TestPrompt    string        `gorm:"type:text" json:"test_prompt"`
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
