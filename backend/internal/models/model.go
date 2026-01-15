package models

import (
	"time"
)

// Model 统一模型定义
type Model struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	Name       string    `json:"name" gorm:"type:varchar(100);not null;uniqueIndex;comment:统一模型名称"`
	ProviderID *string   `json:"provider_id" gorm:"type:varchar(50);comment:厂商ID"`
	Provider   *Provider `json:"provider,omitempty" gorm:"foreignKey:ProviderID;references:ID"`
	Remark     string    `json:"remark" gorm:"type:varchar(500);comment:备注"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Model) TableName() string {
	return "models"
}
