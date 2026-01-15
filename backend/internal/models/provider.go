package models

import (
	"time"
)

// Provider 模型厂商
type Provider struct {
	ID        string    `json:"id" gorm:"type:varchar(50);primaryKey;comment:厂商ID"`
	Name      string    `json:"name" gorm:"type:varchar(100);not null;comment:厂商名称"`
	Icon      string    `json:"icon" gorm:"type:varchar(500);comment:厂商图标URL"`
	LobeIcon  string    `json:"lobeIcon" gorm:"column:lobeIcon;type:varchar(100);comment:Lobe图标组件名"`
	Remark    string    `json:"remark" gorm:"type:varchar(500);comment:备注"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Provider) TableName() string {
	return "providers"
}
