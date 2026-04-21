package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// SettingService 系统设置服务
type SettingService struct {
	logger            *slog.Logger
	systemSettingRepo *repository.SystemSettingRepository
	cache             sync.Map // 设置缓存
	cacheTTL          time.Duration
	notifier          *ConfigNotifier // 配置变更通知器
}

// NewSettingService 创建系统设置服务
func NewSettingService(logger *slog.Logger, systemSettingRepo *repository.SystemSettingRepository) *SettingService {
	return &SettingService{
		logger:            logger,
		systemSettingRepo: systemSettingRepo,
		cacheTTL:          5 * time.Minute, // 缓存 5 分钟
		notifier:          NewConfigNotifier(),
	}
}

// RegisterListener 注册配置变更监听器
func (s *SettingService) RegisterListener(listener ConfigListener) {
	s.notifier.Register(listener)
}

// cachedSetting 缓存的设置项
type cachedSetting struct {
	value     string
	expiresAt time.Time
}

// GetString 获取字符串类型的设置
func (s *SettingService) GetString(ctx context.Context, key string, defaultValue string) string {
	value, err := s.get(ctx, key)
	if err != nil || value == "" {
		return defaultValue
	}
	return value
}

// GetInt 获取整数类型的设置
func (s *SettingService) GetInt(ctx context.Context, key string, defaultValue int) int {
	value, err := s.get(ctx, key)
	if err != nil || value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		s.logger.Warn("解析整数设置失败",
			slog.String("key", key),
			slog.String("value", value),
			slog.String("error", err.Error()),
		)
		return defaultValue
	}

	return intValue
}

// GetBool 获取布尔类型的设置
func (s *SettingService) GetBool(ctx context.Context, key string, defaultValue bool) bool {
	value, err := s.get(ctx, key)
	if err != nil || value == "" {
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		s.logger.Warn("解析布尔设置失败",
			slog.String("key", key),
			slog.String("value", value),
			slog.String("error", err.Error()),
		)
		return defaultValue
	}

	return boolValue
}

// GetDuration 获取时间间隔类型的设置(以秒为单位)
func (s *SettingService) GetDuration(ctx context.Context, key string, defaultValue time.Duration) time.Duration {
	seconds := s.GetInt(ctx, key, int(defaultValue.Seconds()))
	return time.Duration(seconds) * time.Second
}

// get 从缓存或数据库获取设置值
func (s *SettingService) get(ctx context.Context, key string) (string, error) {
	// 尝试从缓存获取
	if cached, ok := s.cache.Load(key); ok {
		cs := cached.(cachedSetting)
		if time.Now().Before(cs.expiresAt) {
			return cs.value, nil
		}
		// 缓存过期,删除
		s.cache.Delete(key)
	}

	// 从数据库获取
	setting, err := s.systemSettingRepo.GetByKey(ctx, key)
	if err != nil {
		s.logger.Error("从数据库获取设置失败",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return "", err
	}

	if setting == nil {
		return "", fmt.Errorf("setting not found: %s", key)
	}

	// 更新缓存
	s.cache.Store(key, cachedSetting{
		value:     setting.Value,
		expiresAt: time.Now().Add(s.cacheTTL),
	})

	return setting.Value, nil
}

// Set 设置配置值
func (s *SettingService) Set(ctx context.Context, key string, value string) error {
	existing, err := s.systemSettingRepo.GetByKey(ctx, key)
	if err != nil {
		s.logger.Error("获取配置失败",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return err
	}
	if existing != nil && existing.Value == value {
		s.cache.Store(key, cachedSetting{
			value:     value,
			expiresAt: time.Now().Add(s.cacheTTL),
		})
		return nil
	}

	// 计算配置通知分类（仅用于运行时监听分发，不落库存储）
	category := s.getDefaultCategory(key)

	err = s.systemSettingRepo.Set(ctx, key, value)
	if err != nil {
		s.logger.Error("设置配置失败",
			slog.String("key", key),
			slog.String("value", value),
			slog.String("error", err.Error()),
		)
		return err
	}

	// 更新缓存
	s.cache.Store(key, cachedSetting{
		value:     value,
		expiresAt: time.Now().Add(s.cacheTTL),
	})

	s.logger.Info("设置已更新",
		slog.String("key", key),
		slog.String("value", value),
		slog.String("category", category),
	)

	s.notifier.Notify(ctx, category)

	// 联动：调试模式驱动 log_add_source；关闭调试时一并关源码定位以减少开销
	if key == models.SettingLogDebugEnabled {
		if err := s.cascadeDebugModeToAddSource(ctx, value); err != nil {
			s.logger.Warn("联动更新 log_add_source 失败",
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// cascadeDebugModeToAddSource 在 log_debug_enabled 变化时同步 log_add_source
func (s *SettingService) cascadeDebugModeToAddSource(ctx context.Context, debugValue string) error {
	target := "false"
	if debugValue == "true" {
		target = "true"
	}
	current := s.GetString(ctx, models.SettingLogAddSource, "false")
	if current == target {
		return nil
	}
	return s.Set(ctx, models.SettingLogAddSource, target)
}

// getDefaultCategory 根据配置键推导通知分类
func (s *SettingService) getDefaultCategory(key string) string {
	switch {
	case key == models.SettingServerPort ||
		key == models.SettingServerReadTimeout ||
		key == models.SettingServerWriteTimeout ||
		key == models.SettingServerMaxHeaderBytes:
		return "server"
	case key == models.SettingCircuitBreakerFailureThreshold ||
		key == models.SettingCircuitBreakerCoolingDuration:
		return "circuit_breaker"
	case key == models.SettingLogRetentionDays ||
		key == models.SettingLogDebugEnabled ||
		key == models.SettingLogAddSource ||
		key == models.SettingLogFileEnabled ||
		key == models.SettingLogFileMaxSize ||
		key == models.SettingLogFileMaxBackups ||
		key == models.SettingLogFileMaxAge ||
		key == models.SettingLogFileCompress:
		return "logging"
	case key == models.SettingProxyRequestTimeout ||
		key == models.SettingProxyNetworkURL ||
		key == models.SettingProxyMaxRetry:
		return "proxy"
	case key == models.SettingModelTestPrompt ||
		key == models.SettingModelTestUserAgent:
		return "model_test"
	case key == models.SettingSnifferPlainTextErrorRules:
		return "sniffer"
	case key == models.SettingSnifferEnabled ||
		key == models.SettingSnifferStreamPacketCount:
		return "sniffer"
	default:
		return "unknown"
	}
}

// GetCircuitBreakerConfig 获取熔断器配置
func (s *SettingService) GetCircuitBreakerConfig(ctx context.Context) (failureThreshold int, coolingDuration time.Duration) {
	failureThreshold = s.GetInt(ctx, models.SettingCircuitBreakerFailureThreshold, 3)
	coolingDuration = s.GetDuration(ctx, models.SettingCircuitBreakerCoolingDuration, 5*time.Minute)
	return
}

// GetProxyConfig 获取代理配置（超时/网络代理/重试）
func (s *SettingService) GetProxyConfig(ctx context.Context) (requestTimeout time.Duration, networkProxyURL string, maxRetry int) {
	requestTimeout = s.GetDuration(ctx, models.SettingProxyRequestTimeout, 120*time.Second)
	networkProxyURL = s.GetString(ctx, models.SettingProxyNetworkURL, "")
	maxRetry = s.GetInt(ctx, models.SettingProxyMaxRetry, 3)
	return
}

// GetEffectiveLogLevel 获取实际生效的日志级别
// 规则：log_debug_enabled 是唯一的日志级别开关，开启时输出 debug，否则 info。
func (s *SettingService) GetEffectiveLogLevel(ctx context.Context) string {
	if s.GetBool(ctx, models.SettingLogDebugEnabled, false) {
		return "debug"
	}
	return "info"
}

// GetSnifferConfig 获取响应嗅探配置（启用开关、流式探测包数量、错误关键词）
func (s *SettingService) GetSnifferConfig(ctx context.Context) (enabled bool, streamPacketCount int, keywords []string) {
	enabled = s.GetBool(ctx, models.SettingSnifferEnabled, true)
	streamPacketCount = s.GetInt(ctx, models.SettingSnifferStreamPacketCount, 1)
	if streamPacketCount <= 0 {
		streamPacketCount = 1
	}
	keywords = s.GetPlainTextErrorRules(ctx)
	return
}

// GetModelTestPrompt 获取模型测试默认提示词
func (s *SettingService) GetModelTestPrompt(ctx context.Context) string {
	value := strings.TrimSpace(s.GetString(ctx, models.SettingModelTestPrompt, ""))
	if value != "" {
		return value
	}
	return "Hi"
}

// GetModelTestUserAgent 获取模型测试相关请求统一使用的 User-Agent。
func (s *SettingService) GetModelTestUserAgent(ctx context.Context) string {
	value := strings.TrimSpace(s.GetString(ctx, models.SettingModelTestUserAgent, ""))
	if value != "" {
		return value
	}
	return models.DefaultModelTestUserAgent
}

// GetServerConfig 获取服务启动配置
func (s *SettingService) GetServerConfig(ctx context.Context) (port int, readTimeout time.Duration, writeTimeout time.Duration, maxHeaderBytes int) {
	port = s.GetInt(ctx, models.SettingServerPort, 8080)
	readTimeout = s.GetDuration(ctx, models.SettingServerReadTimeout, 120*time.Second)
	writeTimeout = s.GetDuration(ctx, models.SettingServerWriteTimeout, 0)
	maxHeaderBytes = s.GetInt(ctx, models.SettingServerMaxHeaderBytes, 1<<20)
	return
}

// GetPlainTextErrorRules 获取明文错误规则
func (s *SettingService) GetPlainTextErrorRules(ctx context.Context) []string {
	value, err := s.get(ctx, models.SettingSnifferPlainTextErrorRules)
	if err != nil || value == "" {
		return []string{}
	}

	// 解析 JSON
	var keywords []string
	if err := json.Unmarshal([]byte(value), &keywords); err != nil {
		s.logger.Warn("解析明文错误规则失败",
			slog.String("value", value),
			slog.String("error", err.Error()),
		)
		return []string{}
	}

	return keywords
}
