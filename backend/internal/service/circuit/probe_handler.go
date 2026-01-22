package circuit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
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
// 通过调用渠道的 /models 接口来判断密钥是否可用
func (ph *ProbeHandler) ProbeKey(ctx context.Context, key *models.Key, channel *models.Channel) (bool, bool, error) {
	ph.logger.Debug("开始嗅探密钥",
		slog.Uint64("key_id", uint64(key.ID)),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
	)

	// 构造 /models 端点的请求 URL
	probeURL := fmt.Sprintf("%s/v1/models", channel.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
	if err != nil {
		ph.logger.Error("创建嗅探请求异常",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	// 设置请求头（使用 OpenAI 格式的认证头）
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key.KeyValue))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) CherryStudio/1.7.13 Chrome/140.0.7339.249 Electron/38.7.0 Safari/537.36")

	// 发送探测请求
	resp, err := ph.httpClient.Do(req)
	if err != nil {
		ph.logger.Warn("嗅探请求失败（网络错误）",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.String("error", err.Error()),
		)
		// 网络错误视为软故障
		return false, false, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// 读取响应体（用于日志记录）
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ph.logger.Warn("无法读取嗅探响应报文",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.String("status_code", strconv.Itoa(resp.StatusCode)),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	// 判断响应状态码
	// 200 状态码表示密钥可用（包括模型列表为空的情况）
	if resp.StatusCode == http.StatusOK {
		ph.logger.Info("嗅探成功",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.String("status_code", strconv.Itoa(resp.StatusCode)),
		)
		return true, false, nil
	}

	// 根据状态码判断故障类型
	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		// 认证失败,硬故障
		ph.logger.Warn("嗅探失败 (authentication error)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
		)
		return false, false, fmt.Errorf("authentication failed: %d", resp.StatusCode)

	case resp.StatusCode == 429:
		// 限流,视为软故障
		ph.logger.Warn("嗅探失败 (rate limited)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, false, fmt.Errorf("rate limited")

	case resp.StatusCode >= 500:
		// 服务器错误,软故障
		ph.logger.Warn("嗅探失败 (server error)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, false, fmt.Errorf("server error: %d", resp.StatusCode)

	default:
		// 其他错误,视为软故障
		ph.logger.Warn("嗅探失败 (unexpected status)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
		)
		return false, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// getTestEndpointAndModel 获取测试用的端点类型和模型名称
// 从渠道的激活模型配置中获取，如果没有则返回错误
func (ph *ProbeHandler) getTestEndpointAndModel(channel *models.Channel) (string, string, error) {
	// 查找第一个激活的模型配置
	for _, config := range channel.ModelConfigs {
		if config.IsActive() && len(config.EndpointTypes) > 0 {
			return config.EndpointTypes[0], config.UpstreamModel, nil
		}
	}

	// 渠道没有激活的模型配置，返回错误
	return "", "", fmt.Errorf("渠道尚未配置模型")
}

// HandleProbeResult 处理探测结果
func (ph *ProbeHandler) HandleProbeResult(keyID uint, channel *models.Channel, success bool, isHardFailure bool, errMsg string) {
	keyBreaker := ph.manager.GetKeyBreaker(keyID)
	currentState := keyBreaker.GetState()

	if success {
		// 探测成功,记录成功并恢复为正常状态
		ph.manager.RecordKeySuccess(keyID, channel.ID)

		ph.logger.Info("密钥已恢复",
			slog.Uint64("key_id", uint64(keyID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("previous_state", string(currentState)),
		)
	} else {
		if isHardFailure {
			// 硬故障,标记为永久禁用
			ph.manager.RecordKeyHardFailure(keyID, channel.ID, channel.Name, errMsg)
		} else {
			// 软故障,重新进入冷却状态
			ph.manager.RecordKeySoftFailure(keyID, channel.ID, channel.Name, errMsg)

			newState := keyBreaker.GetState()
			if newState == KeyStateCooling {
				ph.logger.Warn("密钥未恢复",
					slog.Uint64("key_id", uint64(keyID)),
					slog.Uint64("channel_id", uint64(channel.ID)),
					slog.String("channel_name", channel.Name),
					slog.String("error", errMsg),
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

			ph.logger.Debug("重新嗅探",
				slog.Uint64("key_id", uint64(key.ID)),
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("channel_name", channel.Name),
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
			ph.logger.Warn("密钥硬故障，停止嗅探",
				slog.Uint64("key_id", uint64(key.ID)),
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("channel_name", channel.Name),
			)
			break
		}
	}

	return false, isHardFailure, lastErr
}
