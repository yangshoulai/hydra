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

type SettingValidationError struct {
	message string
}

func (e *SettingValidationError) Error() string {
	return e.message
}

type SettingConflictError struct {
	message string
}

func (e *SettingConflictError) Error() string {
	return e.message
}

type SnifferConfig struct {
	NonStreamEnabled  bool
	StreamEnabled     bool
	StreamPacketCount int
	Keywords          []string
}

type ProxyRateLimitConfig struct {
	Enabled     bool
	GlobalRPS   int
	GlobalBurst int
	TokenRPS    int
	TokenBurst  int
}

type NonStreamKeepaliveConfig struct {
	Enabled    bool
	FirstDelay time.Duration
	Interval   time.Duration
}

// NewSettingService 创建系统设置服务
func NewSettingService(logger *slog.Logger, systemSettingRepo *repository.SystemSettingRepository) *SettingService {
	return &SettingService{
		logger:            logger,
		systemSettingRepo: systemSettingRepo,
		cacheTTL:          0, // 0 表示无 TTL；配置变更由 Set 主动更新缓存并通知监听器
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

func (s *SettingService) cacheExpiresAt() time.Time {
	if s.cacheTTL <= 0 {
		return time.Time{}
	}
	return time.Now().Add(s.cacheTTL)
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
		if cs.expiresAt.IsZero() || time.Now().Before(cs.expiresAt) {
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
		expiresAt: s.cacheExpiresAt(),
	})

	return setting.Value, nil
}

// Set 设置配置值
func (s *SettingService) Set(ctx context.Context, key string, value string) error {
	if err := s.validateValue(key, value); err != nil {
		return err
	}

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
			expiresAt: s.cacheExpiresAt(),
		})
		return nil
	}

	// 计算配置通知分类（仅用于运行时监听分发，不落库存储）
	category := s.getDefaultCategory(key)

	err = s.systemSettingRepo.Set(ctx, key, value)
	if err != nil {
		s.logger.Error("设置配置失败",
			slog.String("key", key),
			slog.String("value", safeSettingValueForLog(key, value)),
			slog.String("error", err.Error()),
		)
		return err
	}

	// 更新缓存
	s.cache.Store(key, cachedSetting{
		value:     value,
		expiresAt: s.cacheExpiresAt(),
	})

	s.logger.Info("设置已更新",
		slog.String("key", key),
		slog.String("value", safeSettingValueForLog(key, value)),
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
	case key == models.SettingSecurityJWTSecret:
		return "security"
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
		key == models.SettingProxyKeepaliveInterval ||
		key == models.SettingProxyNonStreamKeepaliveEnabled ||
		key == models.SettingProxyNonStreamKeepaliveDelay ||
		key == models.SettingProxyNonStreamKeepaliveInterval ||
		key == models.SettingProxyNetworkURL ||
		key == models.SettingProxyMaxRetry ||
		key == models.SettingProxyLoadBalanceStrategy ||
		key == models.SettingProxyMaxBodyBytes ||
		key == models.SettingProxyRateLimitEnabled ||
		key == models.SettingProxyRateLimitGlobalRPS ||
		key == models.SettingProxyRateLimitGlobalBurst ||
		key == models.SettingProxyRateLimitTokenRPS ||
		key == models.SettingProxyRateLimitTokenBurst:
		return "proxy"
	case key == models.SettingModelTestPrompt ||
		key == models.SettingModelTestUserAgent:
		return "model_test"
	case key == models.SettingSnifferPlainTextErrorRules:
		return "sniffer"
	case key == models.SettingSnifferEnabled ||
		key == models.SettingSnifferNonStreamEnabled ||
		key == models.SettingSnifferStreamEnabled ||
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

// GetProxyConfig 获取代理配置（超时/网络代理/重试/负载策略）
func (s *SettingService) GetProxyConfig(ctx context.Context) (requestTimeout time.Duration, keepaliveInterval time.Duration, networkProxyURL string, maxRetry int, loadBalanceStrategy string) {
	requestTimeoutSeconds := s.GetInt(ctx, models.SettingProxyRequestTimeout, 120)
	if requestTimeoutSeconds < 0 {
		requestTimeoutSeconds = 0
	}
	requestTimeout = time.Duration(requestTimeoutSeconds) * time.Second

	keepaliveSeconds := s.GetInt(ctx, models.SettingProxyKeepaliveInterval, 0)
	if keepaliveSeconds < 0 {
		keepaliveSeconds = 0
	}
	if keepaliveSeconds > 120 {
		keepaliveSeconds = 120
	}
	keepaliveInterval = time.Duration(keepaliveSeconds) * time.Second

	networkProxyURL = s.GetString(ctx, models.SettingProxyNetworkURL, "")
	maxRetry = s.GetInt(ctx, models.SettingProxyMaxRetry, 3)
	loadBalanceStrategy = normalizeProxyLoadBalanceStrategy(
		s.GetString(ctx, models.SettingProxyLoadBalanceStrategy, models.ProxyLoadBalanceStrategyWeightedRandom),
	)
	return
}

// GetNonStreamKeepaliveConfig 获取非流式响应保活配置。
// 非流式保活通过写出 JSON whitespace 维持下游连接，默认关闭。
func (s *SettingService) GetNonStreamKeepaliveConfig(ctx context.Context) NonStreamKeepaliveConfig {
	firstDelaySeconds := s.GetInt(ctx, models.SettingProxyNonStreamKeepaliveDelay, models.DefaultProxyNonStreamKeepaliveDelaySeconds)
	if firstDelaySeconds < 0 {
		firstDelaySeconds = 0
	}
	if firstDelaySeconds > 120 {
		firstDelaySeconds = 120
	}

	intervalSeconds := s.GetInt(ctx, models.SettingProxyNonStreamKeepaliveInterval, models.DefaultProxyNonStreamKeepaliveIntervalSeconds)
	if intervalSeconds < 0 {
		intervalSeconds = 0
	}
	if intervalSeconds > 120 {
		intervalSeconds = 120
	}

	return NonStreamKeepaliveConfig{
		Enabled:    s.GetBool(ctx, models.SettingProxyNonStreamKeepaliveEnabled, false),
		FirstDelay: time.Duration(firstDelaySeconds) * time.Second,
		Interval:   time.Duration(intervalSeconds) * time.Second,
	}
}

func (s *SettingService) GetJWTSecret(ctx context.Context) string {
	return strings.TrimSpace(s.GetString(ctx, models.SettingSecurityJWTSecret, ""))
}

func (s *SettingService) GetProxyMaxBodyBytes(ctx context.Context) int64 {
	value := s.GetInt(ctx, models.SettingProxyMaxBodyBytes, models.DefaultProxyMaxBodyBytes)
	if value < 0 {
		return int64(models.DefaultProxyMaxBodyBytes)
	}
	return int64(value)
}

func (s *SettingService) GetProxyRateLimitConfig(ctx context.Context) ProxyRateLimitConfig {
	cfg := ProxyRateLimitConfig{
		Enabled:     s.GetBool(ctx, models.SettingProxyRateLimitEnabled, true),
		GlobalRPS:   s.GetInt(ctx, models.SettingProxyRateLimitGlobalRPS, models.DefaultProxyRateLimitGlobalRPS),
		GlobalBurst: s.GetInt(ctx, models.SettingProxyRateLimitGlobalBurst, models.DefaultProxyRateLimitGlobalBurst),
		TokenRPS:    s.GetInt(ctx, models.SettingProxyRateLimitTokenRPS, models.DefaultProxyRateLimitTokenRPS),
		TokenBurst:  s.GetInt(ctx, models.SettingProxyRateLimitTokenBurst, models.DefaultProxyRateLimitTokenBurst),
	}
	if cfg.GlobalRPS < 0 {
		cfg.GlobalRPS = 0
	}
	if cfg.GlobalBurst < 0 {
		cfg.GlobalBurst = 0
	}
	if cfg.TokenRPS < 0 {
		cfg.TokenRPS = 0
	}
	if cfg.TokenBurst < 0 {
		cfg.TokenBurst = 0
	}
	if cfg.GlobalBurst == 0 && cfg.GlobalRPS > 0 {
		cfg.GlobalBurst = cfg.GlobalRPS
	}
	if cfg.TokenBurst == 0 && cfg.TokenRPS > 0 {
		cfg.TokenBurst = cfg.TokenRPS
	}
	return cfg
}

func (s *SettingService) validateValue(key string, value string) error {
	trimmed := strings.TrimSpace(value)

	switch key {
	case models.SettingSecurityJWTSecret:
		if len(trimmed) < 32 {
			return &SettingValidationError{message: "JWT 签名密钥至少需要 32 个字符"}
		}
	case models.SettingProxyRequestTimeout:
		seconds, err := strconv.Atoi(trimmed)
		if err != nil {
			return &SettingValidationError{message: "代理请求超时必须是 0-300 的整数秒数"}
		}
		if seconds < 0 || seconds > 300 {
			return &SettingValidationError{message: "代理请求超时必须在 0-300 秒之间"}
		}
	case models.SettingProxyKeepaliveInterval:
		seconds, err := strconv.Atoi(trimmed)
		if err != nil {
			return &SettingValidationError{message: "保活间隔必须是 0-120 的整数秒数"}
		}
		if seconds < 0 || seconds > 120 {
			return &SettingValidationError{message: "保活间隔必须在 0-120 秒之间"}
		}
	case models.SettingProxyNonStreamKeepaliveEnabled:
		if _, err := strconv.ParseBool(trimmed); err != nil {
			return &SettingValidationError{message: "非流式保活开关必须是 true 或 false"}
		}
	case models.SettingProxyNonStreamKeepaliveDelay:
		seconds, err := strconv.Atoi(trimmed)
		if err != nil {
			return &SettingValidationError{message: "非流式首个保活延迟必须是 0-120 的整数秒数"}
		}
		if seconds < 0 || seconds > 120 {
			return &SettingValidationError{message: "非流式首个保活延迟必须在 0-120 秒之间"}
		}
	case models.SettingProxyNonStreamKeepaliveInterval:
		seconds, err := strconv.Atoi(trimmed)
		if err != nil {
			return &SettingValidationError{message: "非流式保活间隔必须是 0-120 的整数秒数"}
		}
		if seconds < 0 || seconds > 120 {
			return &SettingValidationError{message: "非流式保活间隔必须在 0-120 秒之间"}
		}
	case models.SettingSnifferEnabled,
		models.SettingSnifferNonStreamEnabled,
		models.SettingSnifferStreamEnabled:
		if _, err := strconv.ParseBool(trimmed); err != nil {
			return &SettingValidationError{message: "嗅探开关必须是 true 或 false"}
		}
	case models.SettingProxyLoadBalanceStrategy:
		switch trimmed {
		case models.ProxyLoadBalanceStrategyWeightedRandom, models.ProxyLoadBalanceStrategyRoundRobin:
		default:
			return &SettingValidationError{message: "负载策略必须是 weighted_random 或 round_robin"}
		}
	case models.SettingProxyMaxBodyBytes:
		bytes, err := strconv.Atoi(trimmed)
		if err != nil {
			return &SettingValidationError{message: "代理请求体大小限制必须是 0-1073741824 的整数"}
		}
		if bytes < 0 || bytes > 1<<30 {
			return &SettingValidationError{message: "代理请求体大小限制必须在 0-1GB 之间，0 表示不限制"}
		}
	case models.SettingProxyRateLimitEnabled:
		if _, err := strconv.ParseBool(trimmed); err != nil {
			return &SettingValidationError{message: "限流开关必须是 true 或 false"}
		}
	case models.SettingProxyRateLimitGlobalRPS,
		models.SettingProxyRateLimitGlobalBurst,
		models.SettingProxyRateLimitTokenRPS,
		models.SettingProxyRateLimitTokenBurst:
		value, err := strconv.Atoi(trimmed)
		if err != nil {
			return &SettingValidationError{message: "限流参数必须是 0-100000 的整数"}
		}
		if value < 0 || value > 100000 {
			return &SettingValidationError{message: "限流参数必须在 0-100000 之间，0 表示不限制"}
		}
	}

	return nil
}

func safeSettingValueForLog(key, value string) string {
	switch key {
	case models.SettingSecurityJWTSecret:
		if strings.TrimSpace(value) == "" {
			return ""
		}
		return "***"
	default:
		return value
	}
}

func normalizeProxyLoadBalanceStrategy(value string) string {
	switch strings.TrimSpace(value) {
	case models.ProxyLoadBalanceStrategyRoundRobin:
		return models.ProxyLoadBalanceStrategyRoundRobin
	default:
		return models.ProxyLoadBalanceStrategyWeightedRandom
	}
}

func (s *SettingService) ValidateSettingsPatch(ctx context.Context, patch map[string]string) error {
	if len(patch) == 0 {
		return nil
	}

	for key, value := range patch {
		if err := s.validateValue(key, value); err != nil {
			return err
		}
	}

	cfg := s.GetSnifferConfig(ctx)
	keepaliveSeconds := s.GetInt(ctx, models.SettingProxyKeepaliveInterval, 0)
	if keepaliveSeconds < 0 {
		keepaliveSeconds = 0
	}

	parseBoolValue := func(key string, current bool) (bool, error) {
		value, ok := patch[key]
		if !ok {
			return current, nil
		}
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return false, &SettingValidationError{message: "嗅探开关必须是 true 或 false"}
		}
		return parsed, nil
	}

	var err error
	cfg.NonStreamEnabled, err = parseBoolValue(models.SettingSnifferNonStreamEnabled, cfg.NonStreamEnabled)
	if err != nil {
		return err
	}
	cfg.StreamEnabled, err = parseBoolValue(models.SettingSnifferStreamEnabled, cfg.StreamEnabled)
	if err != nil {
		return err
	}

	if value, ok := patch[models.SettingSnifferEnabled]; ok {
		legacyEnabled, parseErr := strconv.ParseBool(strings.TrimSpace(value))
		if parseErr != nil {
			return &SettingValidationError{message: "嗅探开关必须是 true 或 false"}
		}
		cfg.NonStreamEnabled = legacyEnabled
		cfg.StreamEnabled = legacyEnabled
	}

	if value, ok := patch[models.SettingProxyKeepaliveInterval]; ok {
		seconds, parseErr := strconv.Atoi(strings.TrimSpace(value))
		if parseErr != nil {
			return &SettingValidationError{message: "保活间隔必须是 0-120 的整数秒数"}
		}
		keepaliveSeconds = seconds
	}

	if cfg.StreamEnabled && keepaliveSeconds > 0 {
		return &SettingConflictError{message: "流式响应嗅探与流式保活不能同时启用，请关闭流式响应嗅探或将流式保活间隔设置为 0"}
	}

	nonStreamKeepalive := s.GetNonStreamKeepaliveConfig(ctx)
	if value, ok := patch[models.SettingProxyNonStreamKeepaliveEnabled]; ok {
		enabled, parseErr := strconv.ParseBool(strings.TrimSpace(value))
		if parseErr != nil {
			return &SettingValidationError{message: "非流式保活开关必须是 true 或 false"}
		}
		nonStreamKeepalive.Enabled = enabled
	}
	if value, ok := patch[models.SettingProxyNonStreamKeepaliveDelay]; ok {
		seconds, parseErr := strconv.Atoi(strings.TrimSpace(value))
		if parseErr != nil {
			return &SettingValidationError{message: "非流式首个保活延迟必须是 0-120 的整数秒数"}
		}
		nonStreamKeepalive.FirstDelay = time.Duration(seconds) * time.Second
	}
	if value, ok := patch[models.SettingProxyNonStreamKeepaliveInterval]; ok {
		seconds, parseErr := strconv.Atoi(strings.TrimSpace(value))
		if parseErr != nil {
			return &SettingValidationError{message: "非流式保活间隔必须是 0-120 的整数秒数"}
		}
		nonStreamKeepalive.Interval = time.Duration(seconds) * time.Second
	}
	if nonStreamKeepalive.Enabled && nonStreamKeepalive.FirstDelay <= 0 {
		return &SettingConflictError{message: "启用非流式保活时，首个保活延迟必须大于 0 秒"}
	}
	if nonStreamKeepalive.Enabled && nonStreamKeepalive.Interval <= 0 {
		return &SettingConflictError{message: "启用非流式保活时，保活间隔必须大于 0 秒"}
	}

	return nil
}

// GetEffectiveLogLevel 获取实际生效的日志级别
// 规则：log_debug_enabled 是唯一的日志级别开关，开启时输出 debug，否则 info。
func (s *SettingService) GetEffectiveLogLevel(ctx context.Context) string {
	if s.GetBool(ctx, models.SettingLogDebugEnabled, false) {
		return "debug"
	}
	return "info"
}

// GetSnifferConfig 获取响应嗅探配置（按流式/非流式拆分）。
func (s *SettingService) GetSnifferConfig(ctx context.Context) SnifferConfig {
	legacyEnabled := s.GetBool(ctx, models.SettingSnifferEnabled, true)
	streamPacketCount := s.GetInt(ctx, models.SettingSnifferStreamPacketCount, 1)
	if streamPacketCount <= 0 {
		streamPacketCount = 1
	}

	return SnifferConfig{
		NonStreamEnabled:  s.GetBool(ctx, models.SettingSnifferNonStreamEnabled, legacyEnabled),
		StreamEnabled:     s.GetBool(ctx, models.SettingSnifferStreamEnabled, legacyEnabled),
		StreamPacketCount: streamPacketCount,
		Keywords:          s.GetPlainTextErrorRules(ctx),
	}
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
