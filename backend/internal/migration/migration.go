package migration

import (
	"fmt"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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
		&models.Model{},
		&models.RequestLog{},
		&models.RequestLogDetail{},
		&models.RequestLogAttempt{},
	); err != nil {
		return fmt.Errorf("初始化数据库表结构失败: %w", err)
	}

	if err := seedDefaultAdmin(db); err != nil {
		return fmt.Errorf("初始化默认管理员失败: %w", err)
	}

	if err := seedDefaultProviders(db); err != nil {
		return fmt.Errorf("初始化默认厂商失败: %w", err)
	}

	return nil
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
		{ID: "openai", Name: "OpenAI", Icon: "https://openai.com/favicon.ico", Remark: "GPT 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "anthropic", Name: "Anthropic", Icon: "https://www.anthropic.com/favicon.ico", Remark: "Claude 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "google", Name: "Google", Icon: "https://ai.google.dev/favicon.ico", Remark: "Gemini 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "xai", Name: "xAI", Icon: "https://x.ai/favicon.ico", Remark: "Grok 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "meta", Name: "Meta", Icon: "https://ai.meta.com/favicon.ico", Remark: "Llama 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "mistral", Name: "Mistral AI", Icon: "https://mistral.ai/favicon.ico", Remark: "Mistral 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "cohere", Name: "Cohere", Icon: "https://cohere.com/favicon.ico", Remark: "Command 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "deepseek", Name: "DeepSeek", Icon: "https://www.deepseek.com/favicon.ico", Remark: "DeepSeek 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "alibaba", Name: "Alibaba Cloud", Icon: "https://www.aliyun.com/favicon.ico", Remark: "通义千问系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "baidu", Name: "Baidu", Icon: "https://www.baidu.com/favicon.ico", Remark: "文心大模型", CreatedAt: now, UpdatedAt: now},
		{ID: "zhipu", Name: "Zhipu AI", Icon: "https://www.zhipuai.cn/favicon.ico", Remark: "GLM 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "moonshot", Name: "Moonshot AI", Icon: "https://moonshot.cn/favicon.ico", Remark: "Kimi 系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "tencent", Name: "Tencent Cloud", Icon: "https://cloud.tencent.com/favicon.ico", Remark: "混元系列模型", CreatedAt: now, UpdatedAt: now},
		{ID: "bytedance", Name: "ByteDance", Icon: "https://www.volcengine.com/favicon.ico", Remark: "豆包大模型", CreatedAt: now, UpdatedAt: now},
	}

	return db.Create(&providers).Error
}
