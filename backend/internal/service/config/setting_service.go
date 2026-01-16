package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
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

// GetNotifier 获取配置变更通知器
func (s *SettingService) GetNotifier() *ConfigNotifier {
	return s.notifier
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
		s.logger.Warn("failed to parse int setting",
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
		s.logger.Warn("failed to parse bool setting",
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
		s.logger.Error("failed to get setting from database",
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
	err := s.systemSettingRepo.Set(ctx, key, value)
	if err != nil {
		s.logger.Error("failed to set setting",
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

	s.logger.Info("setting updated",
		slog.String("key", key),
		slog.String("value", value),
	)

	// 触发配置变更通知
	category := s.getCategoryByKey(key)
	s.notifier.Notify(ctx, category)

	return nil
}

// BatchSet 批量设置配置
func (s *SettingService) BatchSet(ctx context.Context, settings map[string]string) error {
	err := s.systemSettingRepo.BatchSet(ctx, settings)
	if err != nil {
		s.logger.Error("failed to batch set settings",
			slog.String("error", err.Error()),
		)
		return err
	}

	// 收集所有变更的分类
	categories := make(map[string]bool)

	// 更新缓存
	for key, value := range settings {
		s.cache.Store(key, cachedSetting{
			value:     value,
			expiresAt: time.Now().Add(s.cacheTTL),
		})

		category := s.getCategoryByKey(key)
		categories[category] = true
	}

	s.logger.Info("settings batch updated",
		slog.Int("count", len(settings)),
	)

	// 触发配置变更通知（通知所有涉及的分类）
	for category := range categories {
		s.notifier.Notify(ctx, category)
	}

	return nil
}

// getCategoryByKey 根据配置键获取分类
func (s *SettingService) getCategoryByKey(key string) string {
	switch {
	case key == models.SettingCircuitBreakerFailureThreshold ||
		key == models.SettingCircuitBreakerCoolingDuration ||
		key == models.SettingCircuitBreakerMaxRetry:
		return "circuit_breaker"
	case key == models.SettingLogRetentionDays ||
		key == models.SettingLogDebugEnabled:
		return "logging"
	case key == models.SettingProxyRequestTimeout ||
		key == models.SettingProxyMaxResponseSize ||
		key == models.SettingProxyMaxConcurrent:
		return "proxy"
	case key == models.SettingSnifferPlainTextErrorRules:
		return "sniffer"
	default:
		return "unknown"
	}
}

// GetAll 获取所有设置
func (s *SettingService) GetAll(ctx context.Context) ([]*models.SystemSetting, error) {
	return s.systemSettingRepo.GetAll(ctx)
}

// GetByCategory 根据分类获取设置
func (s *SettingService) GetByCategory(ctx context.Context, category string) ([]*models.SystemSetting, error) {
	return s.systemSettingRepo.GetByCategory(ctx, category)
}

// InvalidateCache 使缓存失效
func (s *SettingService) InvalidateCache(key string) {
	s.cache.Delete(key)
}

// InvalidateAllCache 使所有缓存失效
func (s *SettingService) InvalidateAllCache() {
	s.cache = sync.Map{}
	s.logger.Debug("all settings cache invalidated")
}

// GetCircuitBreakerConfig 获取熔断器配置
func (s *SettingService) GetCircuitBreakerConfig(ctx context.Context) (failureThreshold int, coolingDuration time.Duration, maxRetry int) {
	failureThreshold = s.GetInt(ctx, models.SettingCircuitBreakerFailureThreshold, 3)
	coolingDuration = s.GetDuration(ctx, models.SettingCircuitBreakerCoolingDuration, 5*time.Minute)
	maxRetry = s.GetInt(ctx, models.SettingCircuitBreakerMaxRetry, 3)
	return
}

// GetLogConfig 获取日志配置
func (s *SettingService) GetLogConfig(ctx context.Context) (retentionDays int, debugEnabled bool) {
	retentionDays = s.GetInt(ctx, models.SettingLogRetentionDays, 30)
	debugEnabled = s.GetBool(ctx, models.SettingLogDebugEnabled, false)
	return
}

// GetProxyConfig 获取代理配置
func (s *SettingService) GetProxyConfig(ctx context.Context) (requestTimeout time.Duration, maxResponseSize int, maxConcurrent int) {
	requestTimeout = s.GetDuration(ctx, models.SettingProxyRequestTimeout, 120*time.Second)
	maxResponseSize = s.GetInt(ctx, models.SettingProxyMaxResponseSize, 10*1024*1024)
	maxConcurrent = s.GetInt(ctx, models.SettingProxyMaxConcurrent, 1000)
	return
}

// GetPlainTextErrorRules 获取明文错误规则
func (s *SettingService) GetPlainTextErrorRules(ctx context.Context) []string {
	value, err := s.get(ctx, models.SettingSnifferPlainTextErrorRules)
	if err != nil || value == "" {
		// 返回默认规则
		return []string{}
	}

	// 解析 JSON
	var keywords []string
	if err := json.Unmarshal([]byte(value), &keywords); err != nil {
		s.logger.Warn("failed to parse plain text error rules",
			slog.String("value", value),
			slog.String("error", err.Error()),
		)
		return []string{}
	}

	return keywords
}

// SetPlainTextErrorRules 设置明文错误规则
func (s *SettingService) SetPlainTextErrorRules(ctx context.Context, keywords []string) error {
	// 转换为 JSON
	jsonBytes, err := json.Marshal(keywords)
	if err != nil {
		s.logger.Error("failed to marshal plain text error rules",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to marshal keywords: %w", err)
	}

	// 保存到数据库
	if err := s.Set(ctx, models.SettingSnifferPlainTextErrorRules, string(jsonBytes)); err != nil {
		return err
	}

	// 通知 SnifferManager 更新规则
	GetSnifferManager().UpdateKeywords(ctx, keywords)

	return nil
}
