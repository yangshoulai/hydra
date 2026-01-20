package proxy

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/middleware"
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

// Handle 处理 GET /v1/models 请求
// 返回系统中所有可用的统一模型名列表
func (h *ModelsHandler) Handle(c *gin.Context) {
	traceID := middleware.GetTraceID(c)
	ctx := c.Request.Context()

	h.logger.Debug("收到模型列表请求", slog.String("trace_id", traceID))

	// 查询所有有激活渠道配置的统一模型
	models, err := h.modelRepo.ListWithActiveChannelConfigs(ctx)
	if err != nil {
		h.logger.Error("failed to list enabled models",
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

	// 构建响应
	modelObjects := make([]ModelObject, 0, len(models))
	for _, model := range models {
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
