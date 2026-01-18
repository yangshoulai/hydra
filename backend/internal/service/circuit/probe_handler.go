package circuit

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

// ProbeHandler 探测请求处理器
type ProbeHandler struct {
	manager    *Manager
	logger     *slog.Logger
	httpClient *http.Client
}

// NewProbeHandler 创建探测处理器
func NewProbeHandler(manager *Manager, logger *slog.Logger) *ProbeHandler {
	return &ProbeHandler{
		manager: manager,
		logger:  logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

// ProbeKey 探测指定的 Key
// 返回值: (成功, 是否为硬故障, 错误信息)
func (ph *ProbeHandler) ProbeKey(ctx context.Context, key *models.Key, channel *models.Channel) (bool, bool, error) {
	ph.logger.Debug("probing key",
		slog.Uint64("key_id", uint64(key.ID)),
		slog.Uint64("channel_id", uint64(channel.ID)),
	)

	// 构造探测请求(调用 /v1/models 接口)
	probeURL := fmt.Sprintf("%s/v1/models", channel.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
	if err != nil {
		ph.logger.Error("failed to create probe request",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	// 设置认证头
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key.KeyValue))
	req.Header.Set("User-Agent", "Hydra-Probe/1.0")

	// 发送探测请求
	resp, err := ph.httpClient.Do(req)
	if err != nil {
		ph.logger.Warn("探测请求失败（网络错误）",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.String("error", err.Error()),
		)
		// 网络错误视为软故障
		return false, false, err
	}
	defer resp.Body.Close()

	// 分析响应状态码
	switch {
	case resp.StatusCode == 200:
		// 探测成功
		ph.logger.Info("probe succeeded",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
		)
		return true, false, nil

	case resp.StatusCode == 401 || resp.StatusCode == 403:
		// 认证失败,硬故障
		ph.logger.Warn("probe failed (authentication error)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, true, fmt.Errorf("authentication failed: %d", resp.StatusCode)

	case resp.StatusCode == 429:
		// 限流,视为软故障
		ph.logger.Warn("probe failed (rate limited)",
			slog.Uint64("key_id", uint64(key.ID)),
		)
		return false, false, fmt.Errorf("rate limited")

	case resp.StatusCode >= 500:
		// 服务器错误,软故障
		ph.logger.Warn("probe failed (server error)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, false, fmt.Errorf("server error: %d", resp.StatusCode)

	default:
		// 其他错误,视为软故障
		ph.logger.Warn("probe failed (unknown error)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// HandleProbeResult 处理探测结果
func (ph *ProbeHandler) HandleProbeResult(keyID uint, channelID uint, success bool, isHardFailure bool) {
	keyBreaker := ph.manager.GetKeyBreaker(keyID)
	currentState := keyBreaker.GetState()

	if success {
		// 探测成功,记录成功并恢复为正常状态
		ph.manager.RecordKeySuccess(keyID, channelID)

		ph.logger.Info("key recovered after probe",
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("previous_state", string(currentState)),
		)
	} else {
		if isHardFailure {
			// 硬故障,标记为永久禁用
			ph.manager.RecordKeyHardFailure(keyID, channelID)

			ph.logger.Error("key marked as dead after probe",
				slog.Uint64("key_id", uint64(keyID)),
			)
		} else {
			// 软故障,重新进入冷却状态
			ph.manager.RecordKeySoftFailure(keyID, channelID)

			newState := keyBreaker.GetState()
			if newState == KeyStateCooling {
				ph.logger.Warn("探测失败后密钥重新进入冷却状态",
					slog.Uint64("key_id", uint64(keyID)),
					slog.String("previous_state", string(currentState)),
				)
			}
		}
	}
}

// ProbeKeyWithRetry 带重试的探测
func (ph *ProbeHandler) ProbeKeyWithRetry(ctx context.Context, key *models.Key, channel *models.Channel, maxRetries int) (bool, bool, error) {
	var lastErr error
	var isHardFailure bool

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return false, false, ctx.Err()
			}

			ph.logger.Debug("retrying probe",
				slog.Uint64("key_id", uint64(key.ID)),
				slog.Int("attempt", attempt+1),
			)
		}

		success, hardFailure, err := ph.ProbeKey(ctx, key, channel)
		if success {
			return true, false, nil
		}

		lastErr = err
		isHardFailure = hardFailure

		// 如果是硬故障,不再重试
		if hardFailure {
			ph.logger.Warn("stopping probe retries due to hard failure",
				slog.Uint64("key_id", uint64(key.ID)),
			)
			break
		}
	}

	return false, isHardFailure, lastErr
}
