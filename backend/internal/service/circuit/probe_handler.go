package circuit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/models"
	configservice "github.com/yangshoulai/hydra/internal/service/config"
	loggerutil "github.com/yangshoulai/hydra/internal/service/logger"
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

type probeRequestPlan struct {
	req          *http.Request
	ep           endpoint.Endpoint
	modelConfig  *models.ChannelModelConfig
	endpointType string
	modelName    string
	fallback     bool
}

// ProbeChannelKey 探测指定的渠道密钥。
// 返回值: (成功, 是否为硬故障, 错误信息)
//
// 优先按渠道已有的 active 模型配置构造真实端点测试请求；如果渠道还没有模型配置，
// 再回退到 OpenAI 兼容的 /v1/models 探测，保留旧版使用体验。
func (ph *ProbeHandler) ProbeChannelKey(ctx context.Context, channelKey *models.ChannelKey, channel *models.Channel) (bool, bool, error) {
	ph.logger.Debug("开始嗅探密钥",
		slog.Uint64("channel_key_id", uint64(channelKey.ID)),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
	)

	plan, err := ph.buildProbeRequest(ctx, channelKey, channel)
	if err != nil {
		ph.logger.Error("创建嗅探请求异常",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	// 发送探测请求
	resp, err := ph.httpClient.DoWithProxy(plan.req, "", channel.UseProxy)
	if err != nil {
		ph.logger.Warn("嗅探请求失败（网络错误）",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", plan.req.URL.String()),
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
			slog.String("url", plan.req.URL.String()),
			slog.String("status_code", strconv.Itoa(resp.StatusCode)),
			slog.String("error", err.Error()),
		)
		return false, false, err
	}

	// 判断响应状态码。真实模型探测还会用端点校验响应结构，避免假 200 被当作健康。
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		if plan.ep != nil {
			if valid, validateMsg := plan.ep.ValidateResponse(resp.StatusCode, body); !valid {
				logAttrs := ph.baseProbeLogAttrs(channelKey, channel, plan, resp.StatusCode)
				logAttrs = append(logAttrs, loggerutil.ResponseBodyLogAttrs(body)...)
				ph.logger.Warn("嗅探失败 (invalid protocol response)", append(logAttrs, slog.String("validation", validateMsg))...)
				return false, false, fmt.Errorf("invalid protocol response: %s", validateMsg)
			}
		}
		ph.logger.Info("嗅探成功",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", plan.req.URL.String()),
			slog.String("status_code", strconv.Itoa(resp.StatusCode)),
			slog.String("endpoint_type", plan.endpointType),
			slog.String("model", plan.modelName),
			slog.Bool("fallback", plan.fallback),
		)
		return true, false, nil
	}

	// 根据状态码判断故障类型
	switch {
	case resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusPaymentRequired ||
		resp.StatusCode == http.StatusForbidden:
		hardFailure, reason := classifyProbeAuthFailure(resp.StatusCode, body)
		logAttrs := ph.baseProbeLogAttrs(channelKey, channel, plan, resp.StatusCode)
		logAttrs = append(logAttrs, loggerutil.ResponseBodyLogAttrs(body)...)
		ph.logger.Warn("嗅探失败 ("+reason+")", logAttrs...)
		return false, hardFailure, fmt.Errorf("%s: %d", reason, resp.StatusCode)

	case resp.StatusCode == 429:
		// 限流,视为软故障
		ph.logger.Warn("嗅探失败 (rate limited)",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", plan.req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, false, fmt.Errorf("rate limited")

	case resp.StatusCode >= 500:
		// 服务器错误,软故障
		ph.logger.Warn("嗅探失败 (server error)",
			slog.Uint64("channel_key_id", uint64(channelKey.ID)),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("url", plan.req.URL.String()),
			slog.Int("status_code", resp.StatusCode),
		)
		return false, false, fmt.Errorf("server error: %d", resp.StatusCode)

	default:
		// 其他错误,视为软故障
		logAttrs := ph.baseProbeLogAttrs(channelKey, channel, plan, resp.StatusCode)
		logAttrs = append(logAttrs, loggerutil.ResponseBodyLogAttrs(body)...)
		ph.logger.Warn("嗅探失败 (unexpected status)", logAttrs...)
		return false, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

func (ph *ProbeHandler) buildProbeRequest(ctx context.Context, channelKey *models.ChannelKey, channel *models.Channel) (*probeRequestPlan, error) {
	if plan, err := ph.buildProtocolProbeRequest(ctx, channelKey, channel); err == nil && plan != nil {
		return plan, nil
	}

	probeURL := strings.TrimRight(channel.BaseURL, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", channelKey.ChannelKeyValue))
	upstreamhttp.ApplyJSONHeaders(req, ph.getModelTestUserAgent(ctx))
	return &probeRequestPlan{
		req:          req,
		endpointType: "OpenAIModels",
		fallback:     true,
	}, nil
}

func (ph *ProbeHandler) buildProtocolProbeRequest(ctx context.Context, channelKey *models.ChannelKey, channel *models.Channel) (*probeRequestPlan, error) {
	for _, config := range channel.ModelConfigs {
		if config.Status != "active" {
			continue
		}
		endpointTypes := orderedProbeEndpointTypes(models.NormalizeEndpointTypes(config.EndpointTypes))
		for _, endpointType := range endpointTypes {
			ep, err := endpoint.Get(endpointType)
			if err != nil {
				continue
			}
			reqURL := strings.TrimRight(channel.BaseURL, "/") + ep.GetPath()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
			if err != nil {
				return nil, err
			}
			if err := ep.ConfigureTestRequest(req, channelKey.ChannelKeyValue, config.ChannelModel); err != nil {
				continue
			}
			upstreamhttp.ApplyUserAgent(req, ph.getModelTestUserAgent(ctx))
			cfg := config
			return &probeRequestPlan{
				req:          req,
				ep:           ep,
				modelConfig:  &cfg,
				endpointType: endpointType,
				modelName:    config.ChannelModel,
				fallback:     false,
			}, nil
		}
	}
	return nil, fmt.Errorf("no active model config for protocol probe")
}

func orderedProbeEndpointTypes(endpointTypes []string) []string {
	priority := map[string]int{
		endpoint.TypeOpenAIChatCompletions:   0,
		endpoint.TypeOpenAIResponses:         1,
		endpoint.TypeAnthropicMessages:       2,
		endpoint.TypeGemini:                  3,
		endpoint.TypeOpenAIImagesGenerations: 4,
		endpoint.TypeOpenAIImagesEdits:       5,
	}
	ordered := append([]string(nil), endpointTypes...)
	for i := 0; i < len(ordered); i++ {
		for j := i + 1; j < len(ordered); j++ {
			if priorityValue(ordered[j], priority) < priorityValue(ordered[i], priority) {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}
	return ordered
}

func priorityValue(endpointType string, priority map[string]int) int {
	if value, ok := priority[endpointType]; ok {
		return value
	}
	return 100
}

func (ph *ProbeHandler) baseProbeLogAttrs(channelKey *models.ChannelKey, channel *models.Channel, plan *probeRequestPlan, statusCode int) []any {
	attrs := []any{
		slog.Uint64("channel_key_id", uint64(channelKey.ID)),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
		slog.Int("status_code", statusCode),
	}
	if plan != nil && plan.req != nil {
		attrs = append(attrs, slog.String("url", plan.req.URL.String()))
	}
	if plan != nil {
		attrs = append(attrs,
			slog.String("endpoint_type", plan.endpointType),
			slog.String("model", plan.modelName),
			slog.Bool("fallback", plan.fallback),
		)
		if plan.modelConfig != nil {
			attrs = append(attrs, slog.Uint64("model_config_id", uint64(plan.modelConfig.ID)))
		}
	}
	return attrs
}

func classifyProbeAuthFailure(statusCode int, body []byte) (hard bool, reason string) {
	bodyText := strings.ToLower(string(body))
	if containsProbeKeyword(bodyText, probeSoftKeyFailureKeywords) {
		return false, "quota or billing limited"
	}
	if containsProbeKeyword(bodyText, probeHardKeyFailureKeywords) {
		return true, "authentication error"
	}
	if statusCode == http.StatusPaymentRequired {
		return false, "quota or billing limited"
	}
	return false, "authorization error"
}

var probeHardKeyFailureKeywords = []string{
	"invalid api key",
	"incorrect api key",
	"api key not valid",
	"invalid key",
	"invalid token",
	"invalid bearer token",
	"authentication failed",
	"invalid authentication",
	"unauthenticated",
	"permission denied",
	"revoked api key",
	"expired api key",
}

var probeSoftKeyFailureKeywords = []string{
	"quota",
	"rate limit",
	"rate_limit",
	"too many requests",
	"billing",
	"insufficient funds",
	"insufficient credits",
	"insufficient balance",
	"balance not enough",
	"payment required",
}

func containsProbeKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func (ph *ProbeHandler) getModelTestUserAgent(ctx context.Context) string {
	if ph.settingService == nil {
		return models.DefaultModelTestUserAgent
	}
	return ph.settingService.GetModelTestUserAgent(ctx)
}
