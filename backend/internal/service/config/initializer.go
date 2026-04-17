package config

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
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
	// 服务启动设置
	{
		Key:       models.SettingServerPort,
		Value:     "8080",
		ValueType: "int",
		Remark:    "HTTP 服务端口",
	},
	{
		Key:       models.SettingServerReadTimeout,
		Value:     "120",
		ValueType: "int",
		Remark:    "HTTP 读超时(秒)",
	},
	{
		Key:       models.SettingServerWriteTimeout,
		Value:     "0",
		ValueType: "int",
		Remark:    "HTTP 写超时(秒)",
	},
	{
		Key:       models.SettingServerMaxHeaderBytes,
		Value:     "1048576",
		ValueType: "int",
		Remark:    "HTTP 最大请求头大小(字节)",
	},
	// 熔断器设置
	{
		Key:       models.SettingCircuitBreakerFailureThreshold,
		Value:     "3",
		ValueType: "int",
		Remark:    "熔断器触发失败阈值",
	},
	{
		Key:       models.SettingCircuitBreakerCoolingDuration,
		Value:     "300",
		ValueType: "int",
		Remark:    "熔断器冷却时长(秒)",
	},
	// 日志设置
	{
		Key:       models.SettingLogRetentionDays,
		Value:     "30",
		ValueType: "int",
		Remark:    "日志保留天数",
	},
	{
		Key:       models.SettingLogDebugEnabled,
		Value:     "false",
		ValueType: "bool",
		Remark:    "是否启用调试日志",
	},
	{
		Key:       models.SettingLogAddSource,
		Value:     "false",
		ValueType: "bool",
		Remark:    "是否输出源码位置（随调试模式联动）",
	},
	{
		Key:       models.SettingLogFileEnabled,
		Value:     "true",
		ValueType: "bool",
		Remark:    "是否写入日志文件",
	},
	{
		Key:       models.SettingLogFileMaxSize,
		Value:     "100",
		ValueType: "int",
		Remark:    "日志文件最大大小(MB)",
	},
	{
		Key:       models.SettingLogFileMaxBackups,
		Value:     "10",
		ValueType: "int",
		Remark:    "日志文件最大备份数",
	},
	{
		Key:       models.SettingLogFileMaxAge,
		Value:     "30",
		ValueType: "int",
		Remark:    "日志文件最大保留天数",
	},
	{
		Key:       models.SettingLogFileCompress,
		Value:     "true",
		ValueType: "bool",
		Remark:    "是否压缩历史日志",
	},
	// 代理设置
	{
		Key:       models.SettingProxyRequestTimeout,
		Value:     "120",
		ValueType: "int",
		Remark:    "代理请求超时时间(秒)",
	},
	{
		Key:       models.SettingProxyNetworkURL,
		Value:     "",
		ValueType: "string",
		Remark:    "上游网络代理地址(支持 http/https/socks5，留空为禁用)",
	},
	{
		Key:       models.SettingProxyMaxRetry,
		Value:     "3",
		ValueType: "int",
		Remark:    "最大重试次数",
	},
	{
		Key:       models.SettingModelTestPrompt,
		Value:     "Hi",
		ValueType: "string",
		Remark:    "渠道模型测试默认提示词",
	},
	// 响应嗅探设置
	{
		Key:       models.SettingSnifferEnabled,
		Value:     "true",
		ValueType: "bool",
		Remark:    "是否启用响应嗅探",
	},
	{
		Key:       models.SettingSnifferStreamPacketCount,
		Value:     "1",
		ValueType: "int",
		Remark:    "流式响应嗅探前缓存的数据包数量",
	},
}

// GetDefaultPlainTextErrorRulesSetting 获取默认的明文错误规则设置
func GetDefaultPlainTextErrorRulesSetting() models.SystemSetting {
	// 将默认关键词转换为 JSON
	keywords := getDefaultPlainTextErrorKeywords()
	jsonBytes, err := json.Marshal(keywords)
	if err != nil {
		// 如果序列化失败，使用空数组
		jsonBytes = []byte("[]")
	}

	return models.SystemSetting{
		Key:       models.SettingSnifferPlainTextErrorRules,
		Value:     string(jsonBytes),
		ValueType: "json",
		Remark:    "明文错误关键词规则(每行一个关键词)",
	}
}

func getDefaultPlainTextErrorKeywords() []string {
	return []string{
		"无可用后端",
		"额度不足",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
		"quota exceeded",
		"rate limit",
		"unauthorized",
		"forbidden",
		"not found",
		"invalid api key",
		"invalid key",
		"authentication failed",
		"insufficient funds",
		"insufficient quota",
		"billing issue",
	}
}

// obsoleteSettingKeys 早期版本存在、现已废弃的系统设置 key。
// 启动时会从 system_settings 表中清理这些历史残留。
var obsoleteSettingKeys = []string{
	"database_sqlite_path", // 由 CLI --data-dir 派生
	"log_level",            // 由 log_debug_enabled 单开关替代
	"log_file_path",        // 由 CLI --data-dir 派生
	"proxy_max_concurrent", // 未实际接线的入站并发控制，已移除
}

// Initialize 初始化系统设置
// 如果设置不存在则创建,存在则跳过；同时清理已废弃的历史 key。
func (i *Initializer) Initialize(ctx context.Context) error {
	i.logger.Info("初始化系统设置")

	// 清理历史残留设置（失败不阻塞初始化）
	for _, key := range obsoleteSettingKeys {
		if err := i.systemSettingRepo.Delete(ctx, key); err != nil {
			i.logger.Warn("清理历史设置失败（可忽略）",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
	}

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
			if err := i.systemSettingRepo.Set(ctx, setting.Key, setting.Value); err != nil {
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
