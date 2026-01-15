package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// v1_1_4_add_providers_lobe_icon 添加 providers 表的 lobeIcon 字段
func v1_1_4_add_providers_lobe_icon() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.1.4-add-providers-lobe-icon",
		Migrate: func(tx *gorm.DB) error {
			// SQLite 不支持 ALTER TABLE 多个列，使用 AutoMigrate
			if tx.Dialector.Name() == "sqlite" {
				// SQLite: 直接添加列（使用蛇形命名）
				if err := tx.Exec(`ALTER TABLE providers ADD COLUMN lobe_icon VARCHAR(100)`).Error; err != nil {
					// 如果列已存在，忽略错误
					if !isDuplicateColumnError(err) {
						return err
					}
				}
			} else {
				// MySQL/PostgreSQL: 添加列（使用蛇形命名）
				if err := tx.Exec(`ALTER TABLE providers ADD COLUMN lobe_icon VARCHAR(100) COMMENT 'Lobe图标组件名'`).Error; err != nil {
					// 如果列已存在，忽略错误
					if !isDuplicateColumnError(err) {
						return err
					}
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// SQLite 不支持 DROP COLUMN，需要重建表
			if tx.Dialector.Name() == "sqlite" {
				// 创建新表结构（不包含 lobe_icon 列）
				if err := tx.Exec(`
					CREATE TABLE providers_new (
						id VARCHAR(50) PRIMARY KEY,
						name VARCHAR(100) NOT NULL,
						icon VARCHAR(500),
						remark VARCHAR(500),
						created_at DATETIME,
						updated_at DATETIME
					)
				`).Error; err != nil {
					return err
				}

				// 复制数据
				if err := tx.Exec(`
					INSERT INTO providers_new (id, name, icon, remark, created_at, updated_at)
					SELECT id, name, icon, remark, created_at, updated_at
					FROM providers
				`).Error; err != nil {
					return err
				}

				// 删除旧表
				if err := tx.Exec(`DROP TABLE providers`).Error; err != nil {
					return err
				}

				// 重命名新表
				if err := tx.Exec(`ALTER TABLE providers_new RENAME TO providers`).Error; err != nil {
					return err
				}
			} else {
				// MySQL/PostgreSQL: 直接删除列（使用蛇形命名）
				if err := tx.Exec(`ALTER TABLE providers DROP COLUMN lobe_icon`).Error; err != nil {
					// 如果列不存在，忽略错误
					if !isUnknownColumnError(err) {
						return err
					}
				}
			}

			return nil
		},
	}
}

// isDuplicateColumnError 检查是否是重复列错误
func isDuplicateColumnError(err error) bool {
	// SQLite: "duplicate column name"
	// MySQL: "Duplicate column name"
	// PostgreSQL: "column already exists"
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return containsIgnoreCase(errMsg, "duplicate column") ||
		containsIgnoreCase(errMsg, "column already exists")
}

// isUnknownColumnError 检查是否是未知列错误
func isUnknownColumnError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return containsIgnoreCase(errMsg, "unknown column") ||
		containsIgnoreCase(errMsg, "column does not exist")
}

// containsIgnoreCase 不区分大小写的字符串包含检查
func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

// 简单的字符串处理函数
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
