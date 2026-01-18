package migration

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// RunMigrations 执行所有数据库迁移
func RunMigrations(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		// v1.0.0 初始化 Schema（合并所有迁移）
		v1_0_0_Init(),
		// v1.2.0 添加端点类型
		{
			ID: "v1.2.0_add_endpoint_types",
			Migrate: func(tx *gorm.DB) error {
				return V1_2_0_AddEndpointTypes(tx)
			},
		},
		// v1.3.0 移除软删除功能
		{
			ID: "v1.3.0_remove_soft_delete",
			Migrate: func(tx *gorm.DB) error {
				return V1_3_0_RemoveSoftDelete(tx)
			},
		},
	})

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
