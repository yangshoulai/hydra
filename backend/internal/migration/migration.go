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
		// v1.4.0 添加请求头和响应头字段
		{
			ID: "v1.4.0_add_request_response_headers",
			Migrate: func(tx *gorm.DB) error {
				return V1_4_0_AddRequestResponseHeaders(tx)
			},
		},
		// v1.5.0 添加总响应时间字段
		{
			ID: "v1.5.0_add_total_response_time",
			Migrate: func(tx *gorm.DB) error {
				return V1_5_0_AddTotalResponseTime(tx)
			},
		},
		// v1.6.0 添加请求日志主表和明细表
		{
			ID: "v1.6.0_add_request_log_main_detail",
			Migrate: func(tx *gorm.DB) error {
				return V1_6_0_AddRequestLogMainDetail(tx)
			},
		},
		// v1.7.0 删除旧的 RequestLog 表
		{
			ID: "v1.7.0_drop_request_log",
			Migrate: func(tx *gorm.DB) error {
				return V1_7_0_DropRequestLog(tx)
			},
		},
		// v1.8.0 添加渠道同步字段
		{
			ID: "v1.8.0_add_channel_sync_fields",
			Migrate: func(tx *gorm.DB) error {
				return V1_8_0_AddChannelSyncFields(tx)
			},
		},
	})

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
