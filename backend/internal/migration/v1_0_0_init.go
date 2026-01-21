package migration

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/yangshoulai/hydra/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// v1_0_0_Init 初始化数据库 Schema（合并所有迁移）
func v1_0_0_Init() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.0_init",
		Migrate: func(tx *gorm.DB) error {
			// 使用 GORM AutoMigrate 自动创建所有表
			// GORM 会根据数据库类型生成对应的 SQL
			if err := tx.AutoMigrate(
				&models.Provider{},
				&models.AdminUser{},
				&models.AccessToken{},
				&models.SystemSetting{},
				&models.Channel{},
				&models.Key{},
				&models.ChannelModelConfig{},
				&models.Model{},
			); err != nil {
				return err
			}

			// 创建额外的索引（GORM 可能不会创建所有优化的复合索引）
			if err := createAdditionalIndexes(tx); err != nil {
				return err
			}

			// 插入默认数据
			if err := seedData(tx); err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 删除所有表
			return tx.Migrator().DropTable(
				&models.Model{},
				&models.Provider{},
				&models.ChannelModelConfig{},
				&models.Key{},
				&models.Channel{},
				&models.SystemSetting{},
				&models.AccessToken{},
				&models.AdminUser{},
			)
		},
	}
}

// createAdditionalIndexes 创建额外的优化索引
func createAdditionalIndexes(db *gorm.DB) error {
	// GORM 的 AutoMigrate 会创建基本的索引，但我们需要额外的复合索引来优化查询

	// 其他表的复合索引可以在这里添加

	return nil
}

// seedData 插入默认数据
func seedData(db *gorm.DB) error {
	// 只插入 admin_users 表的默认数据
	// 生成密码哈希 (admin123)
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now()
	adminUser := &models.AdminUser{
		Username:     "hydra",
		PasswordHash: string(hash),
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 检查是否已存在管理员用户
	var count int64
	if err := db.Model(&models.AdminUser{}).Count(&count).Error; err != nil {
		return err
	}

	// 只有在没有管理员用户时才插入默认数据
	if count == 0 {
		if err := db.Create(adminUser).Error; err != nil {
			return err
		}
	}

	return nil
}
