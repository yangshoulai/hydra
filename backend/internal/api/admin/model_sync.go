package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	configservice "github.com/yangshoulai/hydra/internal/service/config"
	loggerutil "github.com/yangshoulai/hydra/internal/service/logger"
	"github.com/yangshoulai/hydra/internal/service/modelsync"
	"github.com/yangshoulai/hydra/internal/service/upstreamhttp"
	"gorm.io/gorm"
)

// ModelSyncHandler 模型同步处理器
type ModelSyncHandler struct {
	syncService     *modelsync.SyncService
	modelConfigRepo *repository.ChannelModelConfigRepository
	settingService  *configservice.SettingService
	httpClient      *upstreamhttp.HTTPClient
	db              *gorm.DB
	logger          *slog.Logger
}

// NewModelSyncHandler 创建模型同步处理器
func NewModelSyncHandler(
	syncService *modelsync.SyncService,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	settingService *configservice.SettingService,
	httpClient *upstreamhttp.HTTPClient,
	db *gorm.DB,
	logger *slog.Logger,
) *ModelSyncHandler {
	if httpClient == nil {
		httpClient = upstreamhttp.NewHTTPClient(upstreamhttp.DefaultHTTPClientConfig(), logger)
	}
	return &ModelSyncHandler{
		syncService:     syncService,
		modelConfigRepo: modelConfigRepo,
		settingService:  settingService,
		httpClient:      httpClient,
		db:              db,
		logger:          logger,
	}
}

// SyncChannelModels 同步渠道模型
// @Summary 同步渠道模型
// @Description 调用上游 /v1/models 接口，计算模型差异
// @Tags 模型管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "渠道ID"
// @Success 200 {object} modelsync.SyncResult
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /admin/api/channels/{id}/sync-models [post]
func (h *ModelSyncHandler) SyncChannelModels(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Warn("非法的渠道 ID",
			slog.String("id", idStr),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid channel id",
		})
		return
	}

	channel, err := h.syncService.GetChannel(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("查询渠道异常",
			slog.Uint64("channel_id", uint64(id)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get channel",
		})
		return
	}

	if channel == nil {
		h.logger.Error("渠道不存在",
			slog.Uint64("channel_id", id),
		)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "channel not found",
		})
		return
	}

	// 执行同步
	result, err := h.syncService.SyncChannelModels(c.Request.Context(), channel)
	if err != nil {
		h.logger.Error("同步渠道模型异常",
			slog.Uint64("channel_id", id),
			slog.String("channel_name", channel.Name),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to sync models: " + err.Error(),
		})
		return
	}

	h.logger.Info("渠道模型同步查询完成",
		slog.Uint64("channel_id", id),
		slog.String("channel_name", channel.Name),
		slog.Int("upstream_count", result.Diff.TotalUpstreamModels),
		slog.Int("added", result.Diff.AddedCount),
		slog.Int("removed", result.Diff.RemovedCount),
	)
	c.JSON(http.StatusOK, result)
}

// RegisterRoutes 注册模型同步路由
func (h *ModelSyncHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/channels/:id/sync-models", h.SyncChannelModels)
	r.POST("/channels/:id/apply-sync", h.ApplyChannelSync)
	r.POST("/channels/:id/test-model", h.TestModel)
}

// TestModelRequest 测试模型请求
type TestModelRequest struct {
	ChannelModel          string   `json:"channel_model" binding:"required"`
	Model                 string   `json:"model"`
	EndpointType          string   `json:"endpoint_type" binding:"required"`
	KeyGroups             []string `json:"key_groups"`
	TestPrompt            string   `json:"test_prompt" binding:"omitempty,max=4000"`
	ImageData             string   `json:"image_data"`
	ImageSize             string   `json:"image_size" binding:"omitempty,max=50"`
	ImageQuality          string   `json:"image_quality" binding:"omitempty,max=50"`
	ClientHeaderProfileID string   `json:"client_header_profile_id" binding:"omitempty,max=64"`
}

// TestModelResponse 测试模型响应
type TestModelResponse struct {
	Success      bool           `json:"success"`
	Message      string         `json:"message"`
	ChannelModel string         `json:"channel_model"`
	Model        string         `json:"model"`
	Latency      string         `json:"latency,omitempty"` // 等于非流式延迟
	NonStream    TestModeResult `json:"non_stream"`
	Stream       TestModeResult `json:"stream"`
}

// TestModeResult 单种调用模式的测试结果
type TestModeResult struct {
	Tested  bool                 `json:"tested"`
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Latency string               `json:"latency,omitempty"`
	Content *TestResponseContent `json:"content,omitempty"`
}

// TestResponseContent 渠道测试响应内容预览
type TestResponseContent struct {
	Type     string `json:"type,omitempty"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Raw      string `json:"raw,omitempty"`
}

// TestModel 测试单个模型
// @Summary 测试单个模型
// @Description 测试渠道的某个模型是否可用
// @Tags 模型管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "渠道ID"
// @Param request body TestModelRequest true "测试模型请求"
// @Success 200 {object} TestModelResponse
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /admin/api/channels/{id}/test-model [post]
func (h *ModelSyncHandler) TestModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid channel id",
		})
		return
	}

	var req TestModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid test model request",
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	channelID := uint(id)

	// 获取渠道信息
	channel, err := h.syncService.GetChannel(c.Request.Context(), channelID)
	if err != nil {
		h.logger.Error("无法获取渠道信息",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get channel",
		})
		return
	}

	if channel == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "channel not found",
		})
		return
	}

	// 获取模型配置（用于过滤密钥分组）
	modelConfig, err := h.modelConfigRepo.FindByChannelAndChannelModel(c.Request.Context(), channelID, req.ChannelModel)
	if err != nil {
		h.logger.Error("无法获取渠道模型配置",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_model", req.ChannelModel),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get model config",
		})
		return
	}

	// 获取渠道的一个可用key
	channelKeyRepo := repository.NewChannelKeyRepository(h.db)
	keys, err := channelKeyRepo.FindActiveByChannelID(c.Request.Context(), channelID)
	if err != nil {
		h.logger.Error("无法获取渠道密钥信息",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channel.Name),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get keys",
		})
		return
	}

	if len(req.KeyGroups) > 0 {
		keys = filterKeysByGroups(keys, req.KeyGroups)
		if len(keys) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "no available keys for this model group",
			})
			return
		}
	} else if modelConfig != nil {
		keys = filterKeysByGroups(keys, modelConfig.KeyGroups)
		if len(keys) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "no available keys for this model group",
			})
			return
		}
	}

	if len(keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "no available keys for this channel",
		})
		return
	}

	testPrompt := resolveEffectiveTestPrompt(c.Request.Context(), h.settingService, req.TestPrompt, modelConfig)
	imageData := strings.TrimSpace(req.ImageData)
	imageSize, err := resolveImageTestSize(req.ImageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	imageQuality, err := resolveImageTestQuality(req.ImageQuality)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	if req.EndpointType == endpoint.TypeOpenAIImagesEdits && imageData == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "image_data is required for OpenAIImagesEdits test",
		})
		return
	}

	headerProfile, ok := h.resolveModelTestClientHeaderProfile(c.Request.Context(), req.ClientHeaderProfileID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "client header profile not found",
		})
		return
	}

	testInput := modelTestInput{
		Prompt:                     testPrompt,
		ImageData:                  imageData,
		ImageSize:                  imageSize,
		ImageQuality:               imageQuality,
		ClientHeaderProfileID:      modelTestHeaderProfileID(headerProfile),
		ClientHeaderProfileName:    modelTestHeaderProfileName(headerProfile),
		ClientHeaderProfileHeaders: modelTestHeaderProfileHeaders(headerProfile),
	}

	// 使用第一个可用的key
	testKey := keys[0]
	traceID := middleware.GetTraceID(c)

	nonStreamSuccess, nonStreamMessage, nonStreamLatency, nonStreamContent, err := h.testModelViaUpstream(
		c.Request.Context(),
		traceID,
		channel,
		testKey.ChannelKeyValue,
		req.ChannelModel,
		req.EndpointType,
		false,
		testInput,
	)
	if err != nil {
		h.logger.Error("测试渠道模型异常",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channel.Name),
			slog.String("channel_model", req.ChannelModel),
			slog.String("endpoint_type", req.EndpointType),
			slog.Bool("stream", false),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to test model: %v", err),
		})
		return
	}

	streamResult := TestModeResult{
		Tested:  false,
		Success: false,
		Message: "非流式测试成功，已跳过流式回退测试",
	}
	if !nonStreamSuccess {
		if supportsStreamTest(req.EndpointType) {
			streamSuccess, streamMessage, streamLatency, streamContent, streamErr := h.testModelViaUpstream(
				c.Request.Context(),
				traceID,
				channel,
				testKey.ChannelKeyValue,
				req.ChannelModel,
				req.EndpointType,
				true,
				testInput,
			)
			if streamErr != nil {
				h.logger.Error("测试渠道模型异常",
					slog.Uint64("channel_id", uint64(channelID)),
					slog.String("channel_name", channel.Name),
					slog.String("channel_model", req.ChannelModel),
					slog.String("endpoint_type", req.EndpointType),
					slog.Bool("stream", true),
					slog.String("error", streamErr.Error()),
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("failed to test stream model: %v", streamErr),
				})
				return
			}
			streamResult = TestModeResult{
				Tested:  true,
				Success: streamSuccess,
				Message: streamMessage,
				Latency: streamLatency,
				Content: streamContent,
			}
		} else {
			streamResult = TestModeResult{
				Tested:  false,
				Success: false,
				Message: "该端点类型不支持流式测试，已跳过",
			}
		}
	}

	overallSuccess := nonStreamSuccess || (streamResult.Tested && streamResult.Success)
	overallMessage := buildCombinedTestMessage(nonStreamSuccess, nonStreamMessage, streamResult)

	if overallSuccess {
		h.logger.Info("模型测试成功",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channel.Name),
			slog.String("channel_model", req.ChannelModel),
			slog.String("model", req.Model),
			slog.String("non_stream_latency", nonStreamLatency),
			slog.String("stream_latency", streamResult.Latency),
			slog.Bool("non_stream_success", nonStreamSuccess),
			slog.Bool("stream_tested", streamResult.Tested),
			slog.Bool("stream_success", streamResult.Success),
			slog.String("client_header_profile_id", testInput.ClientHeaderProfileID),
			slog.String("client_header_profile_name", testInput.ClientHeaderProfileName),
		)
	} else {
		h.logger.Warn("渠道模型测试失败",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channel.Name),
			slog.String("channel_model", req.ChannelModel),
			slog.Bool("non_stream_success", nonStreamSuccess),
			slog.Bool("stream_tested", streamResult.Tested),
			slog.Bool("stream_success", streamResult.Success),
			slog.String("non_stream_message", nonStreamMessage),
			slog.String("stream_message", streamResult.Message),
			slog.String("client_header_profile_id", testInput.ClientHeaderProfileID),
			slog.String("client_header_profile_name", testInput.ClientHeaderProfileName),
		)
	}

	c.JSON(http.StatusOK, TestModelResponse{
		Success:      overallSuccess,
		Message:      overallMessage,
		ChannelModel: req.ChannelModel,
		Model:        req.Model,
		Latency:      nonStreamLatency,
		NonStream: TestModeResult{
			Tested:  true,
			Success: nonStreamSuccess,
			Message: nonStreamMessage,
			Latency: nonStreamLatency,
			Content: nonStreamContent,
		},
		Stream: streamResult,
	})
}

type modelTestInput struct {
	Prompt                     string
	ImageData                  string
	ImageSize                  string
	ImageQuality               string
	ClientHeaderProfileID      string
	ClientHeaderProfileName    string
	ClientHeaderProfileHeaders map[string]string
}

// testModelViaUpstream 通过上游API测试模型
func (h *ModelSyncHandler) testModelViaUpstream(
	ctx context.Context,
	traceID string,
	channel *models.Channel,
	apiKey,
	upstreamModel,
	endpointType string,
	stream bool,
	input modelTestInput,
) (bool, string, string, *TestResponseContent, error) {
	// 从端点注册中心获取端点
	ep, err := endpoint.Get(endpointType)
	if err != nil {
		return false, fmt.Sprintf("不支持的端点类型: %s", endpointType), "", nil, err
	}

	// 构造请求URL
	url := fmt.Sprintf("%s%s", channel.BaseURL, ep.GetPath())

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return false, "无法创建测试请求", "", nil, err
	}

	if endpointType == endpoint.TypeOpenAIImagesEdits {
		if err := applyImageEditTestPayload(req, apiKey, upstreamModel, input.Prompt, input.ImageData, input.ImageSize, input.ImageQuality); err != nil {
			return false, fmt.Sprintf("配置图像编辑测试请求异常: %v", err), "", nil, err
		}
	} else {
		// 使用端点的测试请求配置方法
		if err := ep.ConfigureTestRequest(req, apiKey, upstreamModel); err != nil {
			return false, "配置测试请求异常", "", nil, err
		}

		if err := applyPromptToTestRequest(req, endpointType, input.Prompt); err != nil {
			return false, fmt.Sprintf("设置测试提示词异常: %v", err), "", nil, err
		}

		if endpointType == endpoint.TypeOpenAIImagesGenerations {
			if err := applyImageGenerationTestOptions(req, input.ImageSize, input.ImageQuality); err != nil {
				return false, fmt.Sprintf("设置图像生成测试参数异常: %v", err), "", nil, err
			}
		}
	}

	if stream {
		if err := applyStreamMode(req, endpointType); err != nil {
			return false, fmt.Sprintf("配置流式测试请求异常: %v", err), "", nil, err
		}
	}

	upstreamhttp.ApplyUserAgent(req, h.getModelTestUserAgent(ctx))
	applyModelTestClientHeaders(req, input.ClientHeaderProfileHeaders)

	// 记录开始时间
	startTime := time.Now()

	// 发送请求
	resp, err := h.httpClient.DoWithProxy(req, traceID, channel.UseProxy)
	if err != nil {
		return false, fmt.Sprintf("请求失败: %v", err), "", nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// 计算延迟
	latency := fmt.Sprintf("%dms", time.Since(startTime).Milliseconds())

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Sprintf("无法读取响应内容: %v", err), latency, nil, err
	}
	content := buildModelTestResponseContent(endpointType, stream, body)
	logAttrs := []any{
		slog.String("component", "admin"),
		slog.String("event", "model_sync.test.completed"),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
		slog.String("channel_model", upstreamModel),
		slog.String("endpoint_type", endpointType),
		slog.Bool("stream", stream),
		slog.String("url", loggerutil.SafeURLValueForLog(req.URL)),
		slog.Int("status_code", resp.StatusCode),
		slog.String("client_header_profile_id", input.ClientHeaderProfileID),
		slog.String("client_header_profile_name", input.ClientHeaderProfileName),
		slog.Int("client_header_count", len(input.ClientHeaderProfileHeaders)),
	}
	logAttrs = append(logAttrs, loggerutil.ResponseBodyLogAttrs(body)...)
	h.logger.Info("模型测试完成", logAttrs...)

	if stream {
		valid, errMsg := validateStreamTestResponse(resp.StatusCode, body)
		if valid {
			return true, "流式模型测试成功", latency, content, nil
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Sprintf("流式渠道 http status %d: %s", resp.StatusCode, string(body)), latency, content, nil
		}
		return false, fmt.Sprintf("流式模型测试失败: %s (response: %s)", errMsg, string(body)), latency, content, nil
	}

	// 非流式使用端点的验证方法验证响应
	valid, errMsg := ep.ValidateResponse(resp.StatusCode, body)
	if valid {
		return true, "模型测试成功", latency, content, nil
	}

	// 验证失败，返回详细错误信息
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("渠道 http status %d: %s", resp.StatusCode, string(body)), latency, content, nil
	}

	return false, fmt.Sprintf("模型测试失败: %s (response: %s)", errMsg, string(body)), latency, content, nil
}

func (h *ModelSyncHandler) getModelTestUserAgent(ctx context.Context) string {
	if h.settingService == nil {
		return models.DefaultModelTestUserAgent
	}
	return h.settingService.GetModelTestUserAgent(ctx)
}

func (h *ModelSyncHandler) resolveModelTestClientHeaderProfile(ctx context.Context, id string) (*configservice.ModelTestClientHeaderProfile, bool) {
	if h.settingService == nil {
		return nil, true
	}
	return h.settingService.GetModelTestClientHeaderProfile(ctx, id)
}

func modelTestHeaderProfileID(profile *configservice.ModelTestClientHeaderProfile) string {
	if profile == nil {
		return ""
	}
	return profile.ID
}

func modelTestHeaderProfileName(profile *configservice.ModelTestClientHeaderProfile) string {
	if profile == nil {
		return ""
	}
	return profile.Name
}

func modelTestHeaderProfileHeaders(profile *configservice.ModelTestClientHeaderProfile) map[string]string {
	if profile == nil {
		return nil
	}
	return profile.Headers
}

func applyModelTestClientHeaders(req *http.Request, headers map[string]string) {
	if req == nil || len(headers) == 0 {
		return
	}
	for key, value := range headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" || isProtectedModelTestClientHeader(key) {
			continue
		}
		req.Header.Set(key, value)
	}
}

func isProtectedModelTestClientHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization",
		"x-api-key",
		"x-goog-api-key",
		"cookie",
		"set-cookie",
		"content-type",
		"content-length",
		"host":
		return true
	default:
		return false
	}
}

const (
	maxModelTestPreviewTextRunes  = 4000
	maxModelTestPreviewRawRunes   = 4000
	maxModelTestPreviewImageBytes = 6 * 1024 * 1024
)

func buildModelTestResponseContent(endpointType string, stream bool, body []byte) *TestResponseContent {
	raw := truncatePreviewText(strings.TrimSpace(string(body)), maxModelTestPreviewRawRunes)
	if len(body) == 0 {
		return nil
	}

	if stream {
		if text := extractStreamResponseText(endpointType, body); text != "" {
			return &TestResponseContent{Type: "text", Text: truncatePreviewText(text, maxModelTestPreviewTextRunes), Raw: raw}
		}
		return &TestResponseContent{Type: "raw", Raw: raw}
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return &TestResponseContent{Type: "raw", Raw: raw}
	}

	switch endpointType {
	case endpoint.TypeOpenAIImagesGenerations, endpoint.TypeOpenAIImagesEdits:
		if content := extractImageResponseContent(payload); content != nil {
			content.Raw = raw
			return content
		}
	case endpoint.TypeOpenAIChatCompletions:
		if text := extractOpenAIChatResponseText(payload); text != "" {
			return &TestResponseContent{Type: "text", Text: truncatePreviewText(text, maxModelTestPreviewTextRunes), Raw: raw}
		}
	case endpoint.TypeOpenAIResponses:
		if text := extractOpenAIResponsesText(payload); text != "" {
			return &TestResponseContent{Type: "text", Text: truncatePreviewText(text, maxModelTestPreviewTextRunes), Raw: raw}
		}
	case endpoint.TypeAnthropicMessages:
		if text := extractAnthropicMessagesText(payload); text != "" {
			return &TestResponseContent{Type: "text", Text: truncatePreviewText(text, maxModelTestPreviewTextRunes), Raw: raw}
		}
	case endpoint.TypeGemini:
		if text := extractGeminiResponseText(payload); text != "" {
			return &TestResponseContent{Type: "text", Text: truncatePreviewText(text, maxModelTestPreviewTextRunes), Raw: raw}
		}
	}

	return &TestResponseContent{Type: "json", Raw: raw}
}

func extractImageResponseContent(payload map[string]any) *TestResponseContent {
	data := asAnySlice(payload["data"])
	if len(data) == 0 {
		return nil
	}
	item := asStringAnyMap(data[0])
	if item == nil {
		return nil
	}

	text := strings.TrimSpace(asString(item["revised_prompt"]))
	if text == "" {
		text = strings.TrimSpace(asString(payload["revised_prompt"]))
	}

	if imageURL := strings.TrimSpace(asString(item["url"])); imageURL != "" {
		return &TestResponseContent{Type: "image", Text: truncatePreviewText(text, maxModelTestPreviewTextRunes), ImageURL: imageURL}
	}

	b64 := strings.TrimSpace(asString(item["b64_json"]))
	if b64 == "" {
		return nil
	}
	if strings.HasPrefix(b64, "data:image/") {
		return &TestResponseContent{Type: "image", Text: truncatePreviewText(text, maxModelTestPreviewTextRunes), ImageURL: b64}
	}
	if len(b64) > maxModelTestPreviewImageBytes {
		sizeMB := float64(len(b64)) / 1024 / 1024
		message := fmt.Sprintf("上游返回了 base64 图片（约 %.1f MB），超过预览限制，已省略图片预览", sizeMB)
		if text != "" {
			message = text + "\n" + message
		}
		return &TestResponseContent{Type: "text", Text: truncatePreviewText(message, maxModelTestPreviewTextRunes)}
	}
	return &TestResponseContent{Type: "image", Text: truncatePreviewText(text, maxModelTestPreviewTextRunes), ImageURL: "data:image/png;base64," + b64}
}

func extractOpenAIChatResponseText(payload map[string]any) string {
	var parts []string
	for _, item := range asAnySlice(payload["choices"]) {
		choice := asStringAnyMap(item)
		if choice == nil {
			continue
		}
		if message := asStringAnyMap(choice["message"]); message != nil {
			parts = appendNonEmpty(parts, extractTextValue(message["content"]))
		}
		if delta := asStringAnyMap(choice["delta"]); delta != nil {
			parts = appendNonEmpty(parts, extractTextValue(delta["content"]))
		}
		parts = appendNonEmpty(parts, extractTextValue(choice["text"]))
	}
	return joinPreviewParts(parts)
}

func extractOpenAIResponsesText(payload map[string]any) string {
	if text := strings.TrimSpace(asString(payload["output_text"])); text != "" {
		return text
	}

	var parts []string
	for _, item := range asAnySlice(payload["output"]) {
		output := asStringAnyMap(item)
		if output == nil {
			continue
		}
		parts = appendNonEmpty(parts, extractTextValue(output["content"]))
		parts = appendNonEmpty(parts, extractTextValue(output["text"]))
	}
	return joinPreviewParts(parts)
}

func extractAnthropicMessagesText(payload map[string]any) string {
	return extractTextValue(payload["content"])
}

func extractGeminiResponseText(payload map[string]any) string {
	var parts []string
	for _, item := range asAnySlice(payload["candidates"]) {
		candidate := asStringAnyMap(item)
		if candidate == nil {
			continue
		}
		content := asStringAnyMap(candidate["content"])
		if content == nil {
			continue
		}
		parts = appendNonEmpty(parts, extractTextValue(content["parts"]))
	}
	return joinPreviewParts(parts)
}

func extractStreamResponseText(endpointType string, body []byte) string {
	lines := strings.Split(string(body), "\n")
	var incrementalParts []string
	var fallbackParts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}
		switch endpointType {
		case endpoint.TypeOpenAIChatCompletions:
			incrementalParts = appendStreamPart(incrementalParts, extractOpenAIChatStreamText(payload))
			fallbackParts = appendNonEmpty(fallbackParts, extractOpenAIChatResponseText(payload))
		case endpoint.TypeOpenAIResponses:
			incrementalParts = appendStreamPart(incrementalParts, extractOpenAIResponsesStreamText(payload))
			fallbackParts = appendNonEmpty(fallbackParts, extractOpenAIResponsesText(payload))
			fallbackParts = appendNonEmpty(fallbackParts, asString(payload["text"]))
		case endpoint.TypeAnthropicMessages:
			incrementalParts = appendStreamPart(incrementalParts, extractAnthropicStreamText(payload))
			fallbackParts = appendNonEmpty(fallbackParts, extractAnthropicMessagesText(payload))
		case endpoint.TypeGemini:
			incrementalParts = appendStreamPart(incrementalParts, extractGeminiStreamText(payload))
			fallbackParts = appendNonEmpty(fallbackParts, extractGeminiResponseText(payload))
		default:
			incrementalParts = appendStreamPart(incrementalParts, extractTextValuePreserveSpace(payload))
			fallbackParts = appendNonEmpty(fallbackParts, extractTextValue(payload))
		}
	}
	if text := joinStreamParts(incrementalParts); text != "" {
		return text
	}
	return joinPreviewParts(fallbackParts)
}

func extractOpenAIChatStreamText(payload map[string]any) string {
	var parts []string
	for _, item := range asAnySlice(payload["choices"]) {
		choice := asStringAnyMap(item)
		if choice == nil {
			continue
		}
		if delta := asStringAnyMap(choice["delta"]); delta != nil {
			parts = appendStreamPart(parts, extractTextValuePreserveSpace(delta["content"]))
		}
		parts = appendStreamPart(parts, extractTextValuePreserveSpace(choice["text"]))
	}
	return concatStreamParts(parts)
}

func extractOpenAIResponsesStreamText(payload map[string]any) string {
	var parts []string
	parts = appendStreamPart(parts, asString(payload["delta"]))
	return concatStreamParts(parts)
}

func extractAnthropicStreamText(payload map[string]any) string {
	var parts []string
	if delta := asStringAnyMap(payload["delta"]); delta != nil {
		parts = appendStreamPart(parts, extractTextValuePreserveSpace(delta["text"]))
	}
	if block := asStringAnyMap(payload["content_block"]); block != nil {
		parts = appendStreamPart(parts, extractTextValuePreserveSpace(block["text"]))
	}
	return concatStreamParts(parts)
}

func extractGeminiStreamText(payload map[string]any) string {
	var parts []string
	for _, item := range asAnySlice(payload["candidates"]) {
		candidate := asStringAnyMap(item)
		if candidate == nil {
			continue
		}
		content := asStringAnyMap(candidate["content"])
		if content == nil {
			continue
		}
		parts = appendStreamPart(parts, extractTextValuePreserveSpace(content["parts"]))
	}
	return concatStreamParts(parts)
}

func extractTextValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		var parts []string
		for _, item := range v {
			parts = appendNonEmpty(parts, extractTextValue(item))
		}
		return joinPreviewParts(parts)
	case map[string]any:
		for _, key := range []string{"text", "content", "delta"} {
			if text := extractTextValue(v[key]); text != "" {
				return text
			}
		}
		return ""
	default:
		return ""
	}
}

func extractTextValuePreserveSpace(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			parts = appendStreamPart(parts, extractTextValuePreserveSpace(item))
		}
		return concatStreamParts(parts)
	case map[string]any:
		for _, key := range []string{"text", "content", "delta"} {
			if text := extractTextValuePreserveSpace(v[key]); text != "" {
				return text
			}
		}
		return ""
	default:
		return ""
	}
}

func asAnySlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	return nil
}

func asStringAnyMap(value any) map[string]any {
	if item, ok := value.(map[string]any); ok {
		return item
	}
	return nil
}

func asString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func appendNonEmpty(parts []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return parts
	}
	return append(parts, value)
}

func appendStreamPart(parts []string, value string) []string {
	if value == "" {
		return parts
	}
	return append(parts, value)
}

func joinPreviewParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func concatStreamParts(parts []string) string {
	return strings.Join(parts, "")
}

func joinStreamParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func truncatePreviewText(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}

func resolveEffectiveTestPrompt(
	ctx context.Context,
	settingService *configservice.SettingService,
	requestPrompt string,
	modelConfig *models.ChannelModelConfig,
) string {
	prompt := strings.TrimSpace(requestPrompt)
	if prompt != "" {
		return prompt
	}
	if modelConfig != nil {
		modelPrompt := strings.TrimSpace(modelConfig.TestPrompt)
		if modelPrompt != "" {
			return modelPrompt
		}
	}
	if settingService == nil {
		return "Hi"
	}
	return settingService.GetModelTestPrompt(ctx)
}

const (
	defaultImageTestSize    = "1024x1024"
	defaultImageTestQuality = "low"
)

var imageTestAllowedSizes = map[string]struct{}{
	"256x256":   {},
	"512x512":   {},
	"1024x1024": {},
	"1024x1536": {},
	"1536x1024": {},
	"1024x1792": {},
	"1792x1024": {},
	"auto":      {},
}

var imageTestAllowedQualities = map[string]struct{}{
	"low":      {},
	"medium":   {},
	"high":     {},
	"auto":     {},
	"standard": {},
	"hd":       {},
}

func resolveImageTestSize(size string) (string, error) {
	value := strings.TrimSpace(size)
	if value == "" {
		return defaultImageTestSize, nil
	}
	if _, ok := imageTestAllowedSizes[value]; !ok {
		return "", fmt.Errorf("unsupported image_size %q", value)
	}
	return value, nil
}

func resolveImageTestQuality(quality string) (string, error) {
	value := strings.TrimSpace(quality)
	if value == "" {
		return defaultImageTestQuality, nil
	}
	if _, ok := imageTestAllowedQualities[value]; !ok {
		return "", fmt.Errorf("unsupported image_quality %q", value)
	}
	return value, nil
}

func applyPromptToTestRequest(req *http.Request, endpointType string, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}

	switch endpointType {
	case endpoint.TypeOpenAIChatCompletions, endpoint.TypeAnthropicMessages:
		payload, err := readJSONRequestPayload(req)
		if err != nil {
			return err
		}
		payload["messages"] = []map[string]any{
			{
				"role":    "user",
				"content": prompt,
			},
		}
		return writeJSONRequestPayload(req, payload)
	case endpoint.TypeOpenAIResponses:
		payload, err := readJSONRequestPayload(req)
		if err != nil {
			return err
		}
		payload["input"] = []map[string]any{
			{
				"role": "user",
				"content": []map[string]string{
					{"type": "input_text", "text": prompt},
				},
			},
		}
		return writeJSONRequestPayload(req, payload)
	case endpoint.TypeGemini:
		payload, err := readJSONRequestPayload(req)
		if err != nil {
			return err
		}
		payload["contents"] = []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		}
		return writeJSONRequestPayload(req, payload)
	case endpoint.TypeOpenAIImagesGenerations:
		payload, err := readJSONRequestPayload(req)
		if err != nil {
			return err
		}
		payload["prompt"] = prompt
		return writeJSONRequestPayload(req, payload)
	default:
		return nil
	}
}

func applyImageGenerationTestOptions(req *http.Request, size, quality string) error {
	payload, err := readJSONRequestPayload(req)
	if err != nil {
		return err
	}
	if strings.TrimSpace(size) == "" {
		size = defaultImageTestSize
	}
	if strings.TrimSpace(quality) == "" {
		quality = defaultImageTestQuality
	}
	payload["size"] = size
	payload["quality"] = quality
	return writeJSONRequestPayload(req, payload)
}

func readJSONRequestPayload(req *http.Request) (map[string]any, error) {
	if req.Body == nil {
		return nil, fmt.Errorf("request body is empty")
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()

	payload := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return payload, nil
}

func writeJSONRequestPayload(req *http.Request, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(bytes.NewReader(data))
	req.ContentLength = int64(len(data))
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func applyImageEditTestPayload(req *http.Request, apiKey, modelName, prompt, imageData, size, quality string) error {
	imageBytes, fileName, imageMIME, err := decodeImageData(imageData)
	if err != nil {
		return err
	}

	if strings.TrimSpace(prompt) == "" {
		prompt = "请将图片中的背景替换为星空"
	}
	if strings.TrimSpace(size) == "" {
		size = defaultImageTestSize
	}
	if strings.TrimSpace(quality) == "" {
		quality = defaultImageTestQuality
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("model", modelName); err != nil {
		return err
	}
	if err := writer.WriteField("prompt", prompt); err != nil {
		return err
	}
	if err := writer.WriteField("size", size); err != nil {
		return err
	}
	if err := writer.WriteField("quality", quality); err != nil {
		return err
	}

	part, err := createImageFormFile(writer, "image", fileName, imageMIME)
	if err != nil {
		return err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	data := buf.Bytes()
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Body = io.NopCloser(bytes.NewBuffer(data))
	req.ContentLength = int64(len(data))
	return nil
}

func createImageFormFile(writer *multipart.Writer, fieldName, fileName, contentType string) (io.Writer, error) {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     fieldName,
		"filename": fileName,
	}))
	header.Set("Content-Type", contentType)
	return writer.CreatePart(header)
}

func decodeImageData(imageData string) ([]byte, string, string, error) {
	raw := strings.TrimSpace(imageData)
	if raw == "" {
		return nil, "", "", fmt.Errorf("image_data is empty")
	}

	declaredMIME := ""
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		parts := strings.SplitN(raw, ",", 2)
		if len(parts) != 2 {
			return nil, "", "", fmt.Errorf("invalid data URL")
		}
		meta := parts[0]
		if !strings.Contains(strings.ToLower(meta), ";base64") {
			return nil, "", "", fmt.Errorf("only base64 data URL is supported")
		}
		raw = parts[1]
		declaredMIME = extractDataURLMIME(meta)
	}

	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil, "", "", fmt.Errorf("invalid base64 image_data")
		}
	}
	if len(data) == 0 {
		return nil, "", "", fmt.Errorf("decoded image is empty")
	}

	imageMIME := detectImageMIME(data)
	if imageMIME == "" {
		imageMIME = normalizeImageMIME(declaredMIME)
	}
	if imageMIME == "" {
		imageMIME = "image/png"
	}

	return data, "test-image." + imageFileExt(imageMIME), imageMIME, nil
}

func extractDataURLMIME(meta string) string {
	meta = strings.TrimSpace(meta)
	if !strings.HasPrefix(strings.ToLower(meta), "data:") {
		return ""
	}
	mediaType := strings.TrimSpace(meta[len("data:"):])
	if idx := strings.Index(mediaType, ";"); idx >= 0 {
		mediaType = mediaType[:idx]
	}
	return normalizeImageMIME(mediaType)
}

func normalizeImageMIME(value string) string {
	mediaType := strings.ToLower(strings.TrimSpace(value))
	if idx := strings.Index(mediaType, ";"); idx >= 0 {
		mediaType = strings.TrimSpace(mediaType[:idx])
	}
	switch mediaType {
	case "image/jpg":
		return "image/jpeg"
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return mediaType
	default:
		if strings.HasPrefix(mediaType, "image/") {
			return mediaType
		}
		return ""
	}
}

func detectImageMIME(data []byte) string {
	switch {
	case len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' &&
		data[4] == '\r' && data[5] == '\n' && data[6] == 0x1a && data[7] == '\n':
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	default:
		return ""
	}
}

func imageFileExt(imageMIME string) string {
	switch normalizeImageMIME(imageMIME) {
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "image/png":
		return "png"
	default:
		subtype := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(imageMIME)), "image/")
		if subtype == "" || subtype == imageMIME {
			return "png"
		}
		if idx := strings.Index(subtype, "+"); idx >= 0 {
			subtype = subtype[:idx]
		}
		return strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(subtype)
	}
}

func supportsStreamTest(endpointType string) bool {
	switch endpointType {
	case endpoint.TypeOpenAIChatCompletions,
		endpoint.TypeOpenAIResponses,
		endpoint.TypeAnthropicMessages,
		endpoint.TypeGemini:
		return true
	default:
		return false
	}
}

func applyStreamMode(req *http.Request, endpointType string) error {
	switch endpointType {
	case endpoint.TypeOpenAIChatCompletions,
		endpoint.TypeOpenAIResponses,
		endpoint.TypeAnthropicMessages:
		return setJSONRequestStream(req, true)
	case endpoint.TypeGemini:
		path := req.URL.Path
		idx := strings.Index(path, "/models/")
		if idx >= 0 {
			modelPart := path[idx+len("/models/"):]
			parts := strings.SplitN(modelPart, ":", 2)
			modelName := parts[0]
			if modelName != "" {
				req.URL.Path = "/v1beta/models/" + modelName + ":streamGenerateContent"
			}
		}
		q := req.URL.Query()
		q.Set("alt", "sse")
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Accept", "text/event-stream")
		return nil
	default:
		return fmt.Errorf("endpoint %s 不支持流式测试", endpointType)
	}
}

func setJSONRequestStream(req *http.Request, stream bool) error {
	if req.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	originalBody := req.Body
	body, err := io.ReadAll(originalBody)
	_ = originalBody.Close()
	if err != nil {
		return err
	}

	payload := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
	}
	payload["stream"] = stream

	updatedBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req.Body = io.NopCloser(bytes.NewReader(updatedBody))
	req.ContentLength = int64(len(updatedBody))
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func validateStreamTestResponse(statusCode int, body []byte) (bool, string) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return false, "non-success status code"
	}

	content := strings.TrimSpace(string(body))
	if content == "" {
		return false, "empty stream response"
	}

	lines := strings.Split(content, "\n")
	hasDataFrame := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		hasDataFrame = true
		if strings.HasPrefix(data, "{") {
			var payload map[string]any
			if err := json.Unmarshal([]byte(data), &payload); err == nil {
				if errObj, ok := payload["error"]; ok {
					return false, fmt.Sprintf("stream error: %v", errObj)
				}
			}
		}
	}

	if !hasDataFrame {
		return false, "no stream data frame"
	}

	return true, ""
}

func buildCombinedTestMessage(nonStreamSuccess bool, nonStreamMessage string, streamResult TestModeResult) string {
	if nonStreamSuccess {
		return "非流式测试成功"
	}

	if streamResult.Tested {
		if streamResult.Success {
			return fmt.Sprintf("非流式失败，流式回退成功（非流式: %s；流式: %s）", nonStreamMessage, streamResult.Message)
		}
		return fmt.Sprintf("非流式: %s；流式: %s", nonStreamMessage, streamResult.Message)
	}

	return nonStreamMessage
}

func filterKeysByGroups(keys []*models.ChannelKey, keyGroups []string) []*models.ChannelKey {
	groupSet := make(map[string]struct{})
	for _, group := range keyGroups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		groupSet[group] = struct{}{}
	}

	if len(groupSet) == 0 {
		groupSet["Default"] = struct{}{}
	}

	filtered := make([]*models.ChannelKey, 0, len(keys))
	for _, key := range keys {
		group := strings.TrimSpace(key.ChannelKeyGroup)
		if group == "" {
			group = "Default"
		}
		if _, ok := groupSet[group]; ok {
			filtered = append(filtered, key)
		}
	}
	return filtered
}
