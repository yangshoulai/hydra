package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/sniffer"
)

// Initializer 系统设置初始化器
type Initializer struct {
	logger            *slog.Logger
	systemSettingRepo *repository.SystemSettingRepository
}

// NewInitializer 创建系统设置初始化器
func NewInitializer(logger *slog.Logger, systemSettingRepo *repository.SystemSettingRepository) *Initializer {
	return &Initializer{
		logger:            logger,
		systemSettingRepo: systemSettingRepo,
	}
}

// DefaultSettings 默认系统设置
var DefaultSettings = []models.SystemSetting{
	// 熔断器设置
	{
		Key:       models.SettingCircuitBreakerFailureThreshold,
		Value:     "3",
		ValueType: "int",
		Category:  "circuit_breaker",
		Remark:    "熔断器触发失败阈值",
	},
	{
		Key:       models.SettingCircuitBreakerCoolingDuration,
		Value:     "300",
		ValueType: "int",
		Category:  "circuit_breaker",
		Remark:    "熔断器冷却时长(秒)",
	},
	// 日志设置
	{
		Key:       models.SettingLogRetentionDays,
		Value:     "30",
		ValueType: "int",
		Category:  "logging",
		Remark:    "日志保留天数",
	},
	{
		Key:       models.SettingLogDebugEnabled,
		Value:     "false",
		ValueType: "bool",
		Category:  "logging",
		Remark:    "是否启用调试日志",
	},
	// 代理设置
	{
		Key:       models.SettingProxyRequestTimeout,
		Value:     "120",
		ValueType: "int",
		Category:  "proxy",
		Remark:    "代理请求超时时间(秒)",
	},
	{
		Key:       models.SettingProxyMaxConcurrent,
		Value:     "1000",
		ValueType: "int",
		Category:  "proxy",
		Remark:    "最大并发请求数",
	},
	{
		Key:       models.SettingProxyMaxRetry,
		Value:     "3",
		ValueType: "int",
		Category:  "proxy",
		Remark:    "最大重试次数",
	},
	// 渠道同步设置
	{
		Key:       models.SettingChannelGlobalSyncEnabled,
		Value:     "false",
		ValueType: "bool",
		Category:  "channel_sync",
		Remark:    "渠道模型定时同步开关",
	},
}

// GetDefaultPlainTextErrorRulesSetting 获取默认的明文错误规则设置
func GetDefaultPlainTextErrorRulesSetting() models.SystemSetting {
	// 将默认关键词转换为 JSON
	keywords := sniffer.GetDefaultPlainTextErrorKeywords()
	jsonBytes, err := json.Marshal(keywords)
	if err != nil {
		// 如果序列化失败，使用空数组
		jsonBytes = []byte("[]")
	}

	return models.SystemSetting{
		Key:       models.SettingSnifferPlainTextErrorRules,
		Value:     string(jsonBytes),
		ValueType: "json",
		Category:  "sniffer",
		Remark:    "明文错误关键词规则(每行一个关键词)",
	}
}

// Initialize 初始化系统设置
// 如果设置不存在则创建,存在则跳过
func (i *Initializer) Initialize(ctx context.Context) error {
	i.logger.Info("初始化系统设置")

	// 合并默认设置和明文错误规则设置
	allSettings := append(DefaultSettings, GetDefaultPlainTextErrorRulesSetting())

	for _, setting := range allSettings {
		// 检查设置是否已存在
		existing, err := i.systemSettingRepo.GetByKey(ctx, setting.Key)
		if err != nil {
			i.logger.Error("检查现有设置失败",
				slog.String("key", setting.Key),
				slog.String("error", err.Error()),
			)
			return err
		}

		// 如果不存在,创建新设置
		if existing == nil {
			newSetting := &models.SystemSetting{
				Key:       setting.Key,
				Value:     setting.Value,
				ValueType: setting.ValueType,
				Category:  setting.Category,
				Remark:    setting.Remark,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := i.systemSettingRepo.Set(ctx, newSetting.Key, newSetting.Value, newSetting.Category); err != nil {
				i.logger.Error("创建系统设置失败",
					slog.String("key", setting.Key),
					slog.String("error", err.Error()),
				)
				return err
			}

			i.logger.Debug("系统设置已创建",
				slog.String("key", setting.Key),
				slog.String("value", setting.Value),
			)
		} else {
			i.logger.Debug("系统设置已存在",
				slog.String("key", setting.Key),
				slog.String("value", existing.Value),
			)
		}
	}

	i.logger.Info("系统设置初始化成功",
		slog.Int("total_settings", len(allSettings)),
	)

	return nil
}

// EnsureDefaults 确保所有默认设置都存在
func (i *Initializer) EnsureDefaults(ctx context.Context) error {
	return i.Initialize(ctx)
}

// ResetToDefaults 重置所有设置为默认值(慎用)
func (i *Initializer) ResetToDefaults(ctx context.Context) error {
	i.logger.Warn("重置所有设置为默认值")

	for _, setting := range DefaultSettings {
		if err := i.systemSettingRepo.Set(ctx, setting.Key, setting.Value, setting.Category); err != nil {
			i.logger.Error("重置设置失败",
				slog.String("key", setting.Key),
				slog.String("error", err.Error()),
			)
			return err
		}

		i.logger.Debug("设置已重置为默认值",
			slog.String("key", setting.Key),
			slog.String("value", setting.Value),
		)
	}

	i.logger.Info("所有设置已重置为默认值",
		slog.Int("total_settings", len(DefaultSettings)),
	)

	return nil
}
