package migration

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Model 统一模型定义（迁移时的旧版本）
type ModelV1_0_3 struct {
	ID        uint      `gorm:"primarykey"`
	Name      string    `gorm:"type:varchar(100);not null;uniqueIndex;comment:统一模型名称"`
	ProviderID *uint     `gorm:"type:uint;comment:厂商ID"`
	Remark    string    `gorm:"type:varchar(500);comment:备注"`
	CreatedAt time.Time `gorm:"comment:创建时间"`
	UpdatedAt time.Time `gorm:"comment:更新时间"`
}

// TableName 指定表名
func (ModelV1_0_3) TableName() string {
	return "models"
}

// v1_0_3_create_models_table 创建统一模型表
func v1_0_3_create_models_table() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.3-create-models-table",
		Migrate: func(tx *gorm.DB) error {
			// 创建 models 表（使用新结构）
			if err := tx.AutoMigrate(&ModelV1_0_3{}); err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 删除 models 表
			return tx.Migrator().DropTable(&ModelV1_0_3{})
		},
	}
}
