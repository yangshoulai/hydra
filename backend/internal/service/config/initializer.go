package config

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

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
		Key:       models.SettingLogFormat,
		Value:     models.LogFormatText,
		ValueType: "string",
		Remark:    "日志输出格式(text/json)",
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
		Value:     "0",
		ValueType: "int",
		Remark:    "单次上游调用超时时间(秒，0 表示不限制；流式请求建议保持 0)",
	},
	{
		Key:       models.SettingProxyTotalTimeout,
		Value:     strconv.Itoa(models.DefaultProxyTotalTimeoutSeconds),
		ValueType: "int",
		Remark:    "代理请求总预算(秒，包含所有重试；0 表示不限制，流式请求建议保持 0)",
	},
	{
		Key:       models.SettingProxyUpstreamHeaderTimeout,
		Value:     strconv.Itoa(models.DefaultProxyUpstreamHeaderTimeoutSeconds),
		ValueType: "int",
		Remark:    "上游响应头超时时间(秒，0 表示不限制；不限制正常流式持续时间)",
	},
	{
		Key:       models.SettingProxyStreamIdleTimeout,
		Value:     strconv.Itoa(models.DefaultProxyStreamIdleTimeoutSeconds),
		ValueType: "int",
		Remark:    "流式上游空闲超时时间(秒，0 表示不限制；仅在持续无上游数据时断开)",
	},
	{
		Key:       models.SettingProxyKeepaliveInterval,
		Value:     "0",
		ValueType: "int",
		Remark:    "流式响应保活间隔(秒，0 表示禁用，仅对流式响应生效)",
	},
	{
		Key:       models.SettingProxyNonStreamKeepaliveEnabled,
		Value:     "false",
		ValueType: "bool",
		Remark:    "是否启用非流式响应保活",
	},
	{
		Key:       models.SettingProxyNonStreamKeepaliveDelay,
		Value:     strconv.Itoa(models.DefaultProxyNonStreamKeepaliveDelaySeconds),
		ValueType: "int",
		Remark:    "非流式响应首个保活延迟(秒)",
	},
	{
		Key:       models.SettingProxyNonStreamKeepaliveInterval,
		Value:     strconv.Itoa(models.DefaultProxyNonStreamKeepaliveIntervalSeconds),
		ValueType: "int",
		Remark:    "非流式响应保活间隔(秒)",
	},
	{
		Key:       models.SettingProxyNetworkURL,
		Value:     "",
		ValueType: "string",
		Remark:    "系统级上游网络代理地址(支持 http/https/socks5，仅对启用系统代理的渠道生效)",
	},
	{
		Key:       models.SettingProxyMaxRetry,
		Value:     "3",
		ValueType: "int",
		Remark:    "单次请求最多尝试的上游路由数；0 表示失败后不再重试",
	},
	{
		Key:       models.SettingProxyLoadBalanceStrategy,
		Value:     models.ProxyLoadBalanceStrategyWeightedRandom,
		ValueType: "string",
		Remark:    "历史渠道负载策略(兼容保留；当前路由按渠道模型权重统一加权随机)",
	},
	{
		Key:       models.SettingProxyMaxBodyBytes,
		Value:     strconv.Itoa(models.DefaultProxyMaxBodyBytes),
		ValueType: "int",
		Remark:    "代理请求体最大大小(字节，0 表示不限制)",
	},
	{
		Key:       models.SettingProxyMaxResponseBytes,
		Value:     strconv.Itoa(models.DefaultProxyMaxResponseBytes),
		ValueType: "int",
		Remark:    "代理非流式响应体最大大小(字节，0 表示不限制)",
	},
	{
		Key:       models.SettingProxyRateLimitEnabled,
		Value:     "true",
		ValueType: "bool",
		Remark:    "是否启用代理请求限流",
	},
	{
		Key:       models.SettingProxyRateLimitGlobalRPS,
		Value:     strconv.Itoa(models.DefaultProxyRateLimitGlobalRPS),
		ValueType: "int",
		Remark:    "全局代理请求每秒限制(0 表示不限制)",
	},
	{
		Key:       models.SettingProxyRateLimitGlobalBurst,
		Value:     strconv.Itoa(models.DefaultProxyRateLimitGlobalBurst),
		ValueType: "int",
		Remark:    "全局代理请求突发容量",
	},
	{
		Key:       models.SettingProxyRateLimitTokenRPS,
		Value:     strconv.Itoa(models.DefaultProxyRateLimitTokenRPS),
		ValueType: "int",
		Remark:    "单访问令牌每秒限制(0 表示不限制)",
	},
	{
		Key:       models.SettingProxyRateLimitTokenBurst,
		Value:     strconv.Itoa(models.DefaultProxyRateLimitTokenBurst),
		ValueType: "int",
		Remark:    "单访问令牌突发容量",
	},
	{
		Key:       models.SettingModelTestPrompt,
		Value:     "Hi",
		ValueType: "string",
		Remark:    "渠道模型测试默认提示词",
	},
	{
		Key:       models.SettingModelTestUserAgent,
		Value:     models.DefaultModelTestUserAgent,
		ValueType: "string",
		Remark:    "渠道模型测试/同步/健康检查统一使用的 User-Agent",
	},
	{
		Key:       models.SettingModelTestClientHeaderProfiles,
		Value:     defaultModelTestClientHeaderProfilesJSON,
		ValueType: "json",
		Remark:    "模型测试客户端请求头配置档案(JSON)",
	},
	// 通知设置
	{
		Key:       models.SettingNotificationEnabled,
		Value:     "false",
		ValueType: "bool",
		Remark:    "是否启用系统通知",
	},
	{
		Key:       models.SettingNotificationChannel,
		Value:     models.NotificationChannelTelegram,
		ValueType: "string",
		Remark:    "系统通知渠道",
	},
	{
		Key:       models.SettingNotificationEvents,
		Value:     "[]",
		ValueType: "json",
		Remark:    "已启用的通知事件列表",
	},
	{
		Key:       models.SettingNotificationTelegramBotToken,
		Value:     "",
		ValueType: "string",
		Remark:    "Telegram Bot Token",
	},
	{
		Key:       models.SettingNotificationTelegramChatID,
		Value:     "",
		ValueType: "string",
		Remark:    "Telegram Chat ID",
	},
	// 响应嗅探设置
	{
		Key:       models.SettingSnifferNonStreamEnabled,
		Value:     "true",
		ValueType: "bool",
		Remark:    "是否启用非流式响应嗅探",
	},
	{
		Key:       models.SettingSnifferStreamEnabled,
		Value:     "true",
		ValueType: "bool",
		Remark:    "是否启用流式响应嗅探",
	},
	{
		Key:       models.SettingSnifferStreamPacketCount,
		Value:     "1",
		ValueType: "int",
		Remark:    "流式响应嗅探前缓存的数据包数量",
	},
}

const defaultModelTestClientHeaderProfilesJSON = `[
  {
    "id": "codex_cli",
    "name": "Codex CLI",
    "headers": {}
  },
  {
    "id": "claude_code",
    "name": "Claude Code",
    "headers": {}
  }
]`

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

func generateJWTSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成 JWT 随机密钥失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (i *Initializer) ensureJWTSecret(ctx context.Context) error {
	existing, err := i.systemSettingRepo.GetByKey(ctx, models.SettingSecurityJWTSecret)
	if err != nil {
		i.logger.Error("读取 JWT 密钥设置失败",
			slog.String("key", models.SettingSecurityJWTSecret),
			slog.String("error", err.Error()),
		)
		return err
	}
	if existing != nil {
		value := strings.TrimSpace(existing.Value)
		if value != "" {
			return nil
		}
	}

	secret, err := generateJWTSecret()
	if err != nil {
		return err
	}
	if err := i.systemSettingRepo.Set(ctx, models.SettingSecurityJWTSecret, secret); err != nil {
		i.logger.Error("写入 JWT 自动生成密钥失败",
			slog.String("key", models.SettingSecurityJWTSecret),
			slog.String("error", err.Error()),
		)
		return err
	}
	i.logger.Info("已自动生成 JWT 签名密钥",
		slog.String("key", models.SettingSecurityJWTSecret),
	)
	return nil
}

// obsoleteSettingKeys 早期版本存在、现已废弃的系统设置 key。
// 启动时会从 system_settings 表中清理这些历史残留。
var obsoleteSettingKeys = []string{
	"database_sqlite_path", // 由 CLI --data-dir 派生
	"log_level",            // 由 log_debug_enabled 单开关替代
	"log_file_path",        // 由 CLI --data-dir 派生
	"proxy_max_concurrent", // 未实际接线的入站并发控制，已移除
}

func resolveSplitSnifferDefaults(legacyEnabled *bool, keepaliveSeconds int) (nonStreamEnabled string, streamEnabled string) {
	if legacyEnabled == nil {
		return "true", "true"
	}

	if !*legacyEnabled {
		return "false", "false"
	}

	if keepaliveSeconds > 0 {
		return "true", "false"
	}

	return "true", "true"
}

// Initialize 初始化系统设置
// 如果设置不存在则创建,存在则跳过；同时清理已废弃的历史 key。
func (i *Initializer) Initialize(ctx context.Context) error {
	i.logger.Info("初始化系统设置")

	legacySnifferSetting, err := i.systemSettingRepo.GetByKey(ctx, models.SettingSnifferEnabled)
	if err != nil {
		i.logger.Error("读取旧版嗅探设置失败",
			slog.String("key", models.SettingSnifferEnabled),
			slog.String("error", err.Error()),
		)
		return err
	}

	var legacySnifferEnabled *bool
	if legacySnifferSetting != nil {
		enabled := strings.EqualFold(strings.TrimSpace(legacySnifferSetting.Value), "true")
		legacySnifferEnabled = &enabled
	}

	keepaliveSetting, err := i.systemSettingRepo.GetByKey(ctx, models.SettingProxyKeepaliveInterval)
	if err != nil {
		i.logger.Error("读取流式保活设置失败",
			slog.String("key", models.SettingProxyKeepaliveInterval),
			slog.String("error", err.Error()),
		)
		return err
	}

	keepaliveSeconds := 0
	if keepaliveSetting != nil {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(keepaliveSetting.Value)); parseErr == nil && parsed > 0 {
			keepaliveSeconds = parsed
		}
	}

	defaultNonStreamEnabled, defaultStreamEnabled := resolveSplitSnifferDefaults(legacySnifferEnabled, keepaliveSeconds)

	allSettings := make([]models.SystemSetting, 0, len(DefaultSettings)+1)
	for _, setting := range DefaultSettings {
		switch setting.Key {
		case models.SettingSnifferNonStreamEnabled:
			setting.Value = defaultNonStreamEnabled
		case models.SettingSnifferStreamEnabled:
			setting.Value = defaultStreamEnabled
		}
		allSettings = append(allSettings, setting)
	}
	allSettings = append(allSettings, GetDefaultPlainTextErrorRulesSetting())

	// 清理历史残留设置（失败不阻塞初始化）
	for _, key := range obsoleteSettingKeys {
		if err := i.systemSettingRepo.Delete(ctx, key); err != nil {
			i.logger.Warn("清理历史设置失败（可忽略）",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
	}

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

	if err := i.ensureJWTSecret(ctx); err != nil {
		return err
	}

	if legacySnifferSetting != nil {
		nonStreamSetting, err := i.systemSettingRepo.GetByKey(ctx, models.SettingSnifferNonStreamEnabled)
		if err != nil {
			return err
		}
		streamSetting, err := i.systemSettingRepo.GetByKey(ctx, models.SettingSnifferStreamEnabled)
		if err != nil {
			return err
		}

		if nonStreamSetting != nil && streamSetting != nil {
			if err := i.systemSettingRepo.Delete(ctx, models.SettingSnifferEnabled); err != nil {
				i.logger.Warn("清理旧版全局嗅探开关失败（可忽略）",
					slog.String("key", models.SettingSnifferEnabled),
					slog.String("error", err.Error()),
				)
			} else {
				i.logger.Info("已迁移旧版全局嗅探开关到流式/非流式独立配置",
					slog.String("legacy_key", models.SettingSnifferEnabled),
					slog.String("non_stream_enabled", nonStreamSetting.Value),
					slog.String("stream_enabled", streamSetting.Value),
				)
			}
		}
	}

	i.logger.Info("系统设置初始化成功",
		slog.Int("total_settings", len(allSettings)+1),
	)

	return nil
}
