package migration

import (
	"fmt"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RunMigrations 执行数据库初始化（全新库场景）
//
// 当前项目按“从头开始”策略运行，不再兼容历史字段/表结构迁移逻辑。
func RunMigrations(db *gorm.DB) error {
	if err := renameLegacyWeightColumns(db); err != nil {
		return fmt.Errorf("重命名历史权重字段失败: %w", err)
	}

	if err := db.AutoMigrate(
		&models.Provider{},
		&models.AdminUser{},
		&models.AccessToken{},
		&models.SystemSetting{},
		&models.Channel{},
		&models.ChannelKey{},
		&models.ChannelModelConfig{},
		&models.ChannelModelConfigEndpointType{},
		&models.Model{},
		&models.RequestLog{},
		&models.RequestLogDetail{},
		&models.RequestLogAttempt{},
	); err != nil {
		return fmt.Errorf("初始化数据库表结构失败: %w", err)
	}

	if err := backfillChannelModelConfigEndpointTypes(db); err != nil {
		return fmt.Errorf("迁移渠道模型端点类型失败: %w", err)
	}

	if err := seedDefaultAdmin(db); err != nil {
		return fmt.Errorf("初始化默认管理员失败: %w", err)
	}

	if err := seedDefaultProviders(db); err != nil {
		return fmt.Errorf("初始化默认厂商失败: %w", err)
	}

	return nil
}

func backfillChannelModelConfigEndpointTypes(db *gorm.DB) error {
	var configs []models.ChannelModelConfig
	if err := db.Find(&configs).Error; err != nil {
		return err
	}
	if len(configs) == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, config := range configs {
			endpointTypes := models.NormalizeEndpointTypes(config.EndpointTypes)
			rows := make([]models.ChannelModelConfigEndpointType, 0, len(endpointTypes))
			for _, endpointType := range endpointTypes {
				rows = append(rows, models.ChannelModelConfigEndpointType{
					ChannelModelConfigID: config.ID,
					EndpointType:         endpointType,
				})
			}
			if len(rows) == 0 {
				continue
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
				return fmt.Errorf("channel_model_config_id=%d: %w", config.ID, err)
			}
		}
		return nil
	})
}

func renameLegacyWeightColumns(db *gorm.DB) error {
	type columnRename struct {
		table     string
		oldColumn string
		newColumn string
	}

	renames := []columnRename{
		{table: "channels", oldColumn: "priority", newColumn: "weight"},
		{table: "channel_model_configs", oldColumn: "priority", newColumn: "weight"},
	}

	for _, item := range renames {
		if db.Migrator().HasColumn(item.table, item.newColumn) {
			continue
		}
		if !db.Migrator().HasColumn(item.table, item.oldColumn) {
			continue
		}
		if err := db.Migrator().RenameColumn(item.table, item.oldColumn, item.newColumn); err != nil {
			return fmt.Errorf("%s.%s -> %s: %w", item.table, item.oldColumn, item.newColumn, err)
		}
	}

	return nil
}

func seedDefaultAdmin(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.AdminUser{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now()
	return db.Create(&models.AdminUser{
		Username:     "hydra",
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error
}

func seedDefaultProviders(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Provider{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	providers := []models.Provider{
		{ID: "openai", Name: "OpenAI", Icon: "https://models.dev/logos/openai.svg", Remark: "GPT 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "anthropic", Name: "Anthropic", Icon: "https://models.dev/logos/anthropic.svg", Remark: "Claude 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "google", Name: "Google", Icon: "https://models.dev/logos/google.svg", Remark: "Gemini 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "xai", Name: "xAI", Icon: "https://models.dev/logos/xai.svg", Remark: "Grok 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "mistral", Name: "Mistral AI", Icon: "https://models.dev/logos/mistral.svg", Remark: "Mistral 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "deepseek", Name: "DeepSeek", Icon: "https://models.dev/logos/deepseek.svg", Remark: "DeepSeek 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "alibaba", Name: "Alibaba Cloud", Icon: "https://models.dev/logos/alibaba.svg", Remark: "通义千问系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "zai", Name: "Zhipu AI", Icon: "https://models.dev/logos/zai.svg", Remark: "GLM 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "moonshot", Name: "Moonshot AI", Icon: "https://models.dev/logos/moonshotai.svg", Remark: "Kimi 系列模型", CreatedAt: now, UpdatedAt: now},
	}

	return db.Create(&providers).Error
}
