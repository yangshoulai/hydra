package circuit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/yangshoulai/hydra/internal/endpoint"
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
	ph.logger.Debug("开始嗅探密钥",
		slog.Uint64("key_id", uint64(key.ID)),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
	)

	// 确定要使用的端点类型和模型名称
	endpointType, modelName, err := ph.getTestEndpointAndModel(channel)
	if err != nil {
		ph.logger.Warn("渠道未配置模型，跳过密钥探测",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
		)
		// 返回软故障，不影响其他密钥
		return false, false, err
	}

	// 从注册中心获取端点
	ep, err := endpoint.Get(endpointType)
	if err != nil {
		ph.logger.Error("无法获取端点类型",
			slog.String("endpoint_type", endpointType),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	// 构造探测请求
	probeURL := fmt.Sprintf("%s%s", channel.BaseURL, ep.GetPath())
	payload := ep.GetTestPayload(modelName)

	// 序列化请求体
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		ph.logger.Error("序列化测试报文异常",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", probeURL, bytes.NewReader(payloadBytes))
	if err != nil {
		ph.logger.Error("创建嗅探请求异常",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	// 使用端点的配置方法设置请求头
	ep.ConfigureRequest(req, key.KeyValue)
	req.Header.Set("User-Agent", "Hydra-Probe/1.0")

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

	// 读取响应体
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

	// 使用端点的验证方法验证响应
	valid, errMsg := ep.ValidateResponse(resp.StatusCode, body)
	if valid {
		ph.logger.Info("嗅探成功",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.String("status_code", strconv.Itoa(resp.StatusCode)),
		)
		return true, false, nil
	}

	// 验证失败，根据状态码判断故障类型
	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		// 认证失败,硬故障
		ph.logger.Warn("嗅探失败 (authentication error)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, true, fmt.Errorf("authentication failed: %d", resp.StatusCode)

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
		ph.logger.Warn("嗅探失败 (validation error)",
			slog.Uint64("key_id", uint64(key.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
			slog.String("error", errMsg),
		)
		return false, false, fmt.Errorf("嗅探失败: %s", errMsg)
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
