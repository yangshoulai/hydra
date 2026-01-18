package migration

import (
	"gorm.io/gorm"
)

// V1_3_0_RemoveSoftDelete 移除所有表的软删除功能（删除 deleted_at 列）
func V1_3_0_RemoveSoftDelete(db *gorm.DB) error {
	// 定义需要移除 deleted_at 列的表
	tables := []string{
		"request_logs",
		"system_settings",
		"admin_users",
		"keys",
		"channels",
	}

	// 对每个表执行删除列操作
	for _, table := range tables {
		// 检查列是否存在，如果存在则删除
		// 使用原生 SQL 删除列
		if err := db.Exec("ALTER TABLE " + table + " DROP COLUMN IF EXISTS deleted_at").Error; err != nil {
			return err
		}

		// 同时删除与 deleted_at 关联的索引（GORM 会自动创建 idx_<table>_deleted_at 索引）
		// 注意：有些数据库（如 SQLite）在删除列时会自动删除关联索引，
		// 但为了兼容性，我们显式删除索引（如果存在的话）
		indexName := "idx_" + table + "_deleted_at"
		// 使用 IF EXISTS 避免索引不存在时报错（仅支持 PostgreSQL 9.5+）
		// 对于 MySQL，DROP INDEX IF EXISTS 语法也支持
		db.Exec("DROP INDEX IF EXISTS " + indexName)
		// 如果是 SQLite，索引在列删除时会自动删除，所以不需要额外处理
	}

	return nil
}
