package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/modelsync"
	"gorm.io/gorm"
)

// ModelSyncHandler 模型同步处理器
type ModelSyncHandler struct {
	syncService     *modelsync.SyncService
	modelConfigRepo *repository.ChannelModelConfigRepository
	db              *gorm.DB
	logger          *slog.Logger
}

// NewModelSyncHandler 创建模型同步处理器
func NewModelSyncHandler(
	syncService *modelsync.SyncService,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	db *gorm.DB,
	logger *slog.Logger,
) *ModelSyncHandler {
	return &ModelSyncHandler{
		syncService:     syncService,
		modelConfigRepo: modelConfigRepo,
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
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /admin/api/channels/{id}/sync-models [post]
func (h *ModelSyncHandler) SyncChannelModels(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Warn("invalid channel id",
			slog.String("id", idStr),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid channel id",
		})
		return
	}

	// 执行同步
	result, err := h.syncService.SyncChannelModels(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.Error("failed to sync channel models",
			slog.Uint64("channel_id", id),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to sync models: " + err.Error(),
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "channel not found",
		})
		return
	}

	h.logger.Info("channel models synced successfully",
		slog.Uint64("channel_id", id),
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
	UpstreamModel string `json:"upstream_model" binding:"required"`
	UnifiedModel  string `json:"unified_model"`
	EndpointType  string `json:"endpoint_type" binding:"required"`
}

// TestModelResponse 测试模型响应
type TestModelResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	UpstreamModel string `json:"upstream_model"`
	UnifiedModel  string `json:"unified_model"`
	Latency       string `json:"latency,omitempty"`
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
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
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
		h.logger.Error("failed to get channel",
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

	// 获取渠道的一个可用key
	keyRepo := repository.NewKeyRepository(h.db)
	keys, err := keyRepo.FindActiveByChannelID(c.Request.Context(), channelID)
	if err != nil {
		h.logger.Error("failed to get keys",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get keys",
		})
		return
	}

	if len(keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "no available keys for this channel",
		})
		return
	}

	// 使用第一个可用的key
	testKey := keys[0]

	// 调用上游API测试模型
	success, message, latency, err := h.testModelViaUpstream(channel, testKey.KeyValue, req.UpstreamModel, req.EndpointType)
	if err != nil {
		h.logger.Error("failed to test model",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("upstream_model", req.UpstreamModel),
			slog.String("endpoint_type", req.EndpointType),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to test model: %v", err),
		})
		return
	}

	if success {
		h.logger.Info("model test succeeded",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("upstream_model", req.UpstreamModel),
			slog.String("unified_model", req.UnifiedModel),
			slog.String("latency", latency),
		)
	} else {
		h.logger.Warn("model test failed",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("upstream_model", req.UpstreamModel),
			slog.String("message", message),
		)
	}

	c.JSON(http.StatusOK, TestModelResponse{
		Success:       success,
		Message:       message,
		UpstreamModel: req.UpstreamModel,
		UnifiedModel:  req.UnifiedModel,
		Latency:       latency,
	})
}

// testModelViaUpstream 通过上游API测试模型
func (h *ModelSyncHandler) testModelViaUpstream(channel *models.Channel, apiKey, upstreamModel, endpointType string) (bool, string, string, error) {
	// 根据端点类型确定测试端点
	var endpoint string
	var requestBody map[string]interface{}

	switch endpointType {
	case "openai":
		endpoint = "/v1/chat/completions"
		requestBody = map[string]interface{}{
			"model": upstreamModel,
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Hi",
				},
			},
			"max_tokens": 5,
			"stream":     false,
		}
	case "openai-response":
		endpoint = "/v1/responses"
		requestBody = map[string]interface{}{
			"model": upstreamModel,
			"input": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]string{
						{"type": "input_text", "text": "Hi"},
					},
				},
			},
		}
	case "anthropic":
		endpoint = "/v1/messages"
		requestBody = map[string]interface{}{
			"model": upstreamModel,
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hi",
				},
			},
			"max_tokens": 5,
		}
	default:
		endpoint = "/v1/chat/completions"
		requestBody = map[string]interface{}{
			"model": upstreamModel,
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hi",
				},
			},
			"stream": false,
		}
	}

	url := fmt.Sprintf("%s%s", channel.BaseURL, endpoint)

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return false, "failed to marshal request", "", err
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, "failed to create request", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// 为 Anthropic 添加特定的头部
	if endpointType == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("x-api-key", apiKey)
	}

	// 记录开始时间
	startTime := time.Now()

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("request failed: %v", err), "", err
	}
	defer resp.Body.Close()

	// 计算延迟
	latency := fmt.Sprintf("%dms", time.Now().Sub(startTime).Milliseconds())

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Sprintf("upstream returned status %d: %s", resp.StatusCode, string(body)), latency, nil
	}

	// 检查 Content-Type，警告非 JSON 响应（可能是流式）
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" && !bytes.Contains([]byte(contentType), []byte("application/json")) {
		h.logger.Warn("unexpected content type from upstream",
			slog.String("content_type", contentType),
			slog.String("model", upstreamModel),
		)
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Sprintf("failed to decode response: %v", err), latency, err
	}

	// 根据端点类型验证响应格式
	switch endpointType {
	case "openai":
		// OpenAI Chat Completions: 检查 choices 字段
		choices, ok := result["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			if errMsg, ok := result["error"]; ok {
				errBytes, _ := json.Marshal(errMsg)
				return false, fmt.Sprintf("upstream error: %s", string(errBytes)), latency, nil
			}
			responseBody, _ := json.Marshal(result)
			return false, fmt.Sprintf("invalid response: no choices (response: %s)", string(responseBody)), latency, nil
		}

	case "openai-response":
		// OpenAI Response: 检查 output 字段
		if output, ok := result["output"].([]interface{}); !ok || len(output) <= 0 {
			// 有 output 字段
			if errMsg, ok := result["error"]; ok {
				errBytes, _ := json.Marshal(errMsg)
				return false, fmt.Sprintf("upstream error: %s", string(errBytes)), latency, nil
			}
			responseBody, _ := json.Marshal(result)
			return false, fmt.Sprintf("invalid response: no choices or output (response: %s)", string(responseBody)), latency, nil
		}

	case "anthropic":
		// Anthropic Messages: 检查 content 字段
		content, ok := result["content"].([]interface{})
		if !ok || len(content) == 0 {
			if errMsg, ok := result["error"]; ok {
				errBytes, _ := json.Marshal(errMsg)
				return false, fmt.Sprintf("upstream error: %s", string(errBytes)), latency, nil
			}
			responseBody, _ := json.Marshal(result)
			return false, fmt.Sprintf("invalid response: no content (response: %s)", string(responseBody)), latency, nil
		}

	default:
		// 默认检查 choices
		choices, ok := result["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			if errMsg, ok := result["error"]; ok {
				errBytes, _ := json.Marshal(errMsg)
				return false, fmt.Sprintf("upstream error: %s", string(errBytes)), latency, nil
			}
			responseBody, _ := json.Marshal(result)
			return false, fmt.Sprintf("invalid response: no choices (response: %s)", string(responseBody)), latency, nil
		}
	}

	return true, "模型测试成功", latency, nil
}
