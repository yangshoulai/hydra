package circuit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/yangshoulai/hydra/internal/models"
	configservice "github.com/yangshoulai/hydra/internal/service/config"
	"github.com/yangshoulai/hydra/internal/service/upstreamhttp"
)

// ProbeHandler 探测请求处理器
type ProbeHandler struct {
	logger         *slog.Logger
	settingService *configservice.SettingService
	httpClient     *upstreamhttp.HTTPClient
}

// NewProbeHandler 创建探测处理器
func NewProbeHandler(logger *slog.Logger, settingService *configservice.SettingService, httpClient *upstreamhttp.HTTPClient) *ProbeHandler {
	if httpClient == nil {
		httpClient = upstreamhttp.NewHTTPClient(upstreamhttp.DefaultHTTPClientConfig(), logger)
	}
	return &ProbeHandler{
		logger:         logger,
		settingService: settingService,
		httpClient:     httpClient,
	}
}

// ProbeChannelKey 探测指定的渠道密钥
// 返回值: (成功, 是否为硬故障, 错误信息)
// 通过调用渠道的 /models 接口来判断密钥是否可用
func (ph *ProbeHandler) ProbeChannelKey(ctx context.Context, channelKey *models.ChannelKey, channel *models.Channel) (bool, bool, error) {
	ph.logger.Debug("开始嗅探密钥",
		slog.Uint64("channel_key_id", uint64(channelKey.ID)),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
	)

	// 构造 /models 端点的请求 URL
	probeURL := fmt.Sprintf("%s/v1/models", channel.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
	if err != nil {
		ph.logger.Error("创建嗅探请求异常",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	// 设置请求头（使用 OpenAI 格式的认证头）
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", channelKey.ChannelKeyValue))
	upstreamhttp.ApplyJSONHeaders(req, ph.getModelTestUserAgent(ctx))

	// 发送探测请求
	resp, err := ph.httpClient.Do(req, "")
	if err != nil {
		ph.logger.Warn("嗅探请求失败（网络错误）",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
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
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
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
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
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
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
		)
		return false, true, fmt.Errorf("authentication failed: %d", resp.StatusCode)

	case resp.StatusCode == 429:
		// 限流,视为软故障
		ph.logger.Warn("嗅探失败 (rate limited)",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, false, fmt.Errorf("rate limited")

	case resp.StatusCode >= 500:
		// 服务器错误,软故障
		ph.logger.Warn("嗅探失败 (server error)",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, false, fmt.Errorf("server error: %d", resp.StatusCode)

	default:
		// 其他错误,视为软故障
		ph.logger.Warn("嗅探失败 (unexpected status)",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
		)
		return false, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

func (ph *ProbeHandler) getModelTestUserAgent(ctx context.Context) string {
	if ph.settingService == nil {
		return models.DefaultModelTestUserAgent
	}
	return ph.settingService.GetModelTestUserAgent(ctx)
}
