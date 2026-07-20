package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"

	"github.com/yangshoulai/hydra/internal/endpoint"
)

// EndpointTypes 端点类型列表
type EndpointTypes []string

// KeyGroups 密钥分组列表
type KeyGroups []string

// NormalizeEndpointTypes 规范化端点类型列表。
//
// 空列表或全部为空时回退到 OpenAI Chat Completions，避免历史空值导致路由不可达。
func NormalizeEndpointTypes(raw []string) []string {
	if len(raw) == 0 {
		return []string{endpoint.TypeOpenAIChatCompletions}
	}
	seen := make(map[string]struct{}, len(raw))
	normalized := make([]string, 0, len(raw))
	for _, item := range raw {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return []string{endpoint.TypeOpenAIChatCompletions}
	}
	return normalized
}

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

	var endpointTypes []string
	if err := json.Unmarshal(bytes, &endpointTypes); err != nil {
		*e = []string{endpoint.TypeOpenAIChatCompletions}
		return nil
	}
	*e = NormalizeEndpointTypes(endpointTypes)
	return nil
}

// Value 实现 driver.Valuer 接口
func (e EndpointTypes) Value() (driver.Value, error) {
	return json.Marshal(NormalizeEndpointTypes(e))
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
	Channel          *Channel                         `gorm:"foreignKey:ChannelID" json:"channel,omitempty"`
	EndpointTypeRows []ChannelModelConfigEndpointType `gorm:"foreignKey:ChannelModelConfigID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 指定表名
func (ChannelModelConfig) TableName() string {
	return "channel_model_configs"
}

// IsActive 检查配置是否激活
func (c *ChannelModelConfig) IsActive() bool {
	return c.Status == "active"
}

// ChannelModelConfigEndpointType 渠道模型配置支持的端点类型。
//
// 保留 channel_model_configs.endpoint_types JSON 字段用于前端展示与兼容；
// 路由与可用性查询走该结构化表，避免 JSON 文本 LIKE 误匹配。
type ChannelModelConfigEndpointType struct {
	ID                   uint      `gorm:"primarykey" json:"id"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	ChannelModelConfigID uint      `gorm:"not null;uniqueIndex:idx_cmc_endpoint_type;index" json:"channel_model_config_id"`
	EndpointType         string    `gorm:"type:varchar(80);not null;uniqueIndex:idx_cmc_endpoint_type;index" json:"endpoint_type"`
}

// TableName 指定表名
func (ChannelModelConfigEndpointType) TableName() string {
	return "channel_model_config_endpoint_types"
}
