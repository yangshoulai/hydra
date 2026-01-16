package modelsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// SyncService 模型同步服务
type SyncService struct {
	logger          *slog.Logger
	channelRepo     *repository.ChannelRepository
	modelConfigRepo *repository.ChannelModelConfigRepository
	keyRepo         *repository.KeyRepository
	diffCalculator  *DiffCalculator
	httpClient      *http.Client
}

// NewSyncService 创建模型同步服务
func NewSyncService(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	keyRepo *repository.KeyRepository,
) *SyncService {
	return &SyncService{
		logger:          logger,
		channelRepo:     channelRepo,
		modelConfigRepo: modelConfigRepo,
		keyRepo:         keyRepo,
		diffCalculator:  NewDiffCalculator(logger),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

// UpstreamModelsResponse 上游 /v1/models 接口响应
type UpstreamModelsResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// SyncResult 同步结果
type SyncResult struct {
	Success        bool      `json:"success"`
	Message        string    `json:"message"`
	ChannelID      uint      `json:"channel_id"`
	ChannelName    string    `json:"channel_name"`
	FetchedAt      time.Time `json:"fetched_at"`
	UpstreamModels []string  `json:"upstream_models"`
	Diff           *SyncDiff `json:"diff"`
}

// SyncChannelModels 同步渠道模型
func (s *SyncService) SyncChannelModels(ctx context.Context, channelID uint) (*SyncResult, error) {
	// 查询渠道
	channel, err := s.channelRepo.FindByID(ctx, channelID)
	if err != nil {
		s.logger.Error("failed to find channel",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to find channel: %w", err)
	}

	if channel == nil {
		return nil, fmt.Errorf("channel not found")
	}

	s.logger.Info("starting model sync for channel",
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channel.Name),
		slog.String("base_url", channel.BaseURL),
	)

	// 调用上游 /v1/models 接口
	upstreamModels, err := s.fetchUpstreamModels(ctx, channel)
	if err != nil {
		s.logger.Error("failed to fetch upstream models",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to fetch upstream models: %w", err)
	}

	s.logger.Info("fetched upstream models",
		slog.Uint64("channel_id", uint64(channelID)),
		slog.Int("model_count", len(upstreamModels)),
	)

	// 获取本地模型配置
	localConfigs, err := s.modelConfigRepo.FindByChannelID(ctx, channelID)
	if err != nil {
		s.logger.Error("failed to fetch local model configs",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to fetch local configs: %w", err)
	}

	// 计算差异
	diff := s.diffCalculator.Calculate(upstreamModels, localConfigs)

	result := &SyncResult{
		Success:        true,
		Message:        "Models synced successfully",
		ChannelID:      channel.ID,
		ChannelName:    channel.Name,
		FetchedAt:      time.Now(),
		UpstreamModels: upstreamModels,
		Diff:           diff,
	}

	s.logger.Info("model sync completed",
		slog.Uint64("channel_id", uint64(channelID)),
		slog.Int("upstream_count", diff.TotalUpstreamModels),
		slog.Int("local_count", diff.TotalLocalModels),
		slog.Int("added", diff.AddedCount),
		slog.Int("removed", diff.RemovedCount),
		slog.Int("existing", diff.ExistingCount),
	)

	return result, nil
}

// fetchUpstreamModels 调用上游 /v1/models 接口
func (s *SyncService) fetchUpstreamModels(ctx context.Context, channel *models.Channel) ([]string, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/v1/models", channel.BaseURL)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Hydra/1.0")

	// 查询渠道的活跃密钥
	keys, err := s.keyRepo.FindActiveByChannelID(ctx, channel.ID)
	if err != nil {
		s.logger.Warn("failed to fetch active keys for channel",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("error", err.Error()),
		)
	}

	// 如果有可用密钥，选择第一个并添加到认证头
	if len(keys) > 0 {
		selectedKey := keys[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", selectedKey.KeyValue))
		s.logger.Debug("using api key for upstream request",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.Uint64("key_id", uint64(selectedKey.ID)),
		)
	} else {
		s.logger.Debug("no active api key found for channel, requesting without authentication",
			slog.Uint64("channel_id", uint64(channel.ID)),
		)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var upstreamResp UpstreamModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&upstreamResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 提取模型ID列表
	models := make([]string, len(upstreamResp.Data))
	for i, model := range upstreamResp.Data {
		models[i] = model.ID
	}

	return models, nil
}

// GetChannel 获取渠道信息
func (s *SyncService) GetChannel(ctx context.Context, channelID uint) (*models.Channel, error) {
	return s.channelRepo.FindByID(ctx, channelID)
}
