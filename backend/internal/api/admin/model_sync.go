package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	configservice "github.com/yangshoulai/hydra/internal/service/config"
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
	ChannelModel string   `json:"channel_model" binding:"required"`
	Model        string   `json:"model"`
	EndpointType string   `json:"endpoint_type" binding:"required"`
	KeyGroups    []string `json:"key_groups"`
	TestPrompt   string   `json:"test_prompt" binding:"omitempty,max=4000"`
	ImageData    string   `json:"image_data"`
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
	Tested  bool   `json:"tested"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency string `json:"latency,omitempty"`
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
	if req.EndpointType == endpoint.TypeOpenAIImagesEdits && imageData == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "image_data is required for OpenAIImagesEdits test",
		})
		return
	}

	testInput := modelTestInput{
		Prompt:    testPrompt,
		ImageData: imageData,
	}

	// 使用第一个可用的key
	testKey := keys[0]
	traceID := middleware.GetTraceID(c)

	nonStreamSuccess, nonStreamMessage, nonStreamLatency, err := h.testModelViaUpstream(
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
			streamSuccess, streamMessage, streamLatency, streamErr := h.testModelViaUpstream(
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
		},
		Stream: streamResult,
	})
}

type modelTestInput struct {
	Prompt    string
	ImageData string
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
) (bool, string, string, error) {
	// 从端点注册中心获取端点
	ep, err := endpoint.Get(endpointType)
	if err != nil {
		return false, fmt.Sprintf("不支持的端点类型: %s", endpointType), "", err
	}

	// 构造请求URL
	url := fmt.Sprintf("%s%s", channel.BaseURL, ep.GetPath())

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return false, "无法创建测试请求", "", err
	}

	if endpointType == endpoint.TypeOpenAIImagesEdits {
		if err := applyImageEditTestPayload(req, apiKey, upstreamModel, input.Prompt, input.ImageData); err != nil {
			return false, fmt.Sprintf("配置图像编辑测试请求异常: %v", err), "", err
		}
	} else {
		// 使用端点的测试请求配置方法
		if err := ep.ConfigureTestRequest(req, apiKey, upstreamModel); err != nil {
			return false, "配置测试请求异常", "", err
		}

		if err := applyPromptToTestRequest(req, endpointType, input.Prompt); err != nil {
			return false, fmt.Sprintf("设置测试提示词异常: %v", err), "", err
		}
	}

	if stream {
		if err := applyStreamMode(req, endpointType); err != nil {
			return false, fmt.Sprintf("配置流式测试请求异常: %v", err), "", err
		}
	}

	upstreamhttp.ApplyUserAgent(req, h.getModelTestUserAgent(ctx))

	// 记录开始时间
	startTime := time.Now()

	// 发送请求
	resp, err := h.httpClient.Do(req, traceID)
	if err != nil {
		return false, fmt.Sprintf("请求失败: %v", err), "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// 计算延迟
	latency := fmt.Sprintf("%dms", time.Since(startTime).Milliseconds())

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Sprintf("无法读取响应内容: %v", err), latency, err
	}
	h.logger.Info("模型测试完成", slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
		slog.String("channel_model", upstreamModel),
		slog.String("endpoint_type", endpointType),
		slog.Bool("stream", stream),
		slog.String("url", req.URL.String()),
		slog.Uint64("status_code", uint64(resp.StatusCode)),
		slog.String("response_body", string(body)),
	)

	if stream {
		valid, errMsg := validateStreamTestResponse(resp.StatusCode, body)
		if valid {
			return true, "流式模型测试成功", latency, nil
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Sprintf("流式渠道 http status %d: %s", resp.StatusCode, string(body)), latency, nil
		}
		return false, fmt.Sprintf("流式模型测试失败: %s (response: %s)", errMsg, string(body)), latency, nil
	}

	// 非流式使用端点的验证方法验证响应
	valid, errMsg := ep.ValidateResponse(resp.StatusCode, body)
	if valid {
		return true, "模型测试成功", latency, nil
	}

	// 验证失败，返回详细错误信息
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("渠道 http status %d: %s", resp.StatusCode, string(body)), latency, nil
	}

	return false, fmt.Sprintf("模型测试失败: %s (response: %s)", errMsg, string(body)), latency, nil
}

func (h *ModelSyncHandler) getModelTestUserAgent(ctx context.Context) string {
	if h.settingService == nil {
		return models.DefaultModelTestUserAgent
	}
	return h.settingService.GetModelTestUserAgent(ctx)
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

func applyImageEditTestPayload(req *http.Request, apiKey, modelName, prompt, imageData string) error {
	imageBytes, fileName, err := decodeImageData(imageData)
	if err != nil {
		return err
	}

	if strings.TrimSpace(prompt) == "" {
		prompt = "请将图片中的背景替换为星空"
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("model", modelName); err != nil {
		return err
	}
	if err := writer.WriteField("prompt", prompt); err != nil {
		return err
	}

	part, err := writer.CreateFormFile("image", fileName)
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

func decodeImageData(imageData string) ([]byte, string, error) {
	raw := strings.TrimSpace(imageData)
	if raw == "" {
		return nil, "", fmt.Errorf("image_data is empty")
	}

	fileExt := "png"
	if strings.HasPrefix(raw, "data:") {
		parts := strings.SplitN(raw, ",", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid data URL")
		}
		meta := parts[0]
		if !strings.Contains(meta, ";base64") {
			return nil, "", fmt.Errorf("only base64 data URL is supported")
		}
		raw = parts[1]

		if strings.HasPrefix(meta, "data:image/") {
			mimePart := strings.TrimPrefix(meta, "data:image/")
			mimePart = strings.SplitN(mimePart, ";", 2)[0]
			switch strings.ToLower(strings.TrimSpace(mimePart)) {
			case "jpeg", "jpg":
				fileExt = "jpg"
			case "webp":
				fileExt = "webp"
			case "gif":
				fileExt = "gif"
			default:
				fileExt = "png"
			}
		}
	}

	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil, "", fmt.Errorf("invalid base64 image_data")
		}
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("decoded image is empty")
	}

	return data, "test-image." + fileExt, nil
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
