package proxy

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// ModelsHandler Models 列表处理器
type ModelsHandler struct {
	logger    *slog.Logger
	modelRepo *repository.ModelRepository
}

// NewModelsHandler 创建 Models 处理器
func NewModelsHandler(logger *slog.Logger, modelRepo *repository.ModelRepository) *ModelsHandler {
	return &ModelsHandler{
		logger:    logger,
		modelRepo: modelRepo,
	}
}

// ModelObject OpenAI Models API 返回的模型对象
type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse OpenAI Models API 响应结构
type ModelsResponse struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

// GeminiModelsResponse Gemini Models API 响应结构
type GeminiModelsResponse struct {
	Models        []GeminiModel `json:"models"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
}

// GeminiModel Gemini 模型对象
type GeminiModel struct {
	Name                       string   `json:"name"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	Version                    string   `json:"version,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
}

// Handle 处理 GET /v1/models 请求
// 返回系统中所有可用的统一模型名列表
func (h *ModelsHandler) Handle(c *gin.Context) {
	h.handleModelsByEndpointType(c, "")
}

// HandleV1Beta 处理 GET /v1beta/models 请求
// 返回系统中所有支持 Gemini 端点的统一模型名列表
func (h *ModelsHandler) HandleV1Beta(c *gin.Context) {
	h.handleModelsByEndpointType(c, "gemini")
}

func (h *ModelsHandler) handleModelsByEndpointType(c *gin.Context, endpointType string) {
	traceID := middleware.GetTraceID(c)
	ctx := c.Request.Context()

	h.logger.Debug("收到模型列表请求", slog.String("trace_id", traceID))

	// 查询所有有激活渠道配置的统一模型
	var modelList []models.Model
	var err error
	if endpointType == "" {
		modelList, err = h.modelRepo.ListWithActiveChannelConfigs(ctx)
	} else {
		modelList, err = h.modelRepo.ListWithActiveChannelConfigsByEndpointType(ctx, endpointType)
	}
	if err != nil {
		h.logger.Error("查询启用令牌列表异常",
			slog.String("trace_id", traceID),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message":  "Failed to retrieve models",
				"type":     "internal_error",
				"trace_id": traceID,
			},
		})
		return
	}

	if endpointType == "gemini" {
		c.JSON(http.StatusOK, buildGeminiModelsResponse(modelList))
		return
	}

	// 构建响应
	modelObjects := make([]ModelObject, 0, len(modelList))
	for _, model := range modelList {
		// 获取厂商名称
		providerName := "unknown"
		if model.Provider != nil {
			providerName = model.Provider.Name
		}

		modelObjects = append(modelObjects, ModelObject{
			ID:      model.Name,
			Object:  "model",
			Created: 1677610602, // 固定时间戳
			OwnedBy: providerName,
		})
	}

	response := ModelsResponse{
		Object: "list",
		Data:   modelObjects,
	}

	h.logger.Info("模型列表请求完成",
		slog.String("trace_id", traceID),
		slog.Int("model_count", len(modelObjects)),
	)

	c.JSON(http.StatusOK, response)
}

func buildGeminiModelsResponse(modelList []models.Model) GeminiModelsResponse {
	modelsData := make([]GeminiModel, 0, len(modelList))
	for _, model := range modelList {
		modelsData = append(modelsData, GeminiModel{
			Name:                       "models/" + model.Name,
			DisplayName:                model.Name,
			Description:                model.Name,
			Version:                    "",
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		})
	}

	return GeminiModelsResponse{
		Models: modelsData,
	}
}
