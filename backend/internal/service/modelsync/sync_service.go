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
func (s *SyncService) SyncChannelModels(ctx context.Context, channel *models.Channel) (*SyncResult, error) {
	// 调用上游 /v1/models 接口
	upstreamModels, err := s.fetchUpstreamModels(ctx, channel)
	if err != nil {
		s.logger.Error("查询渠道模型列表异常",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_id", channel.Name),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to fetch upstream models: %w", err)
	}
	// 获取本地模型配置
	localConfigs, err := s.modelConfigRepo.FindByChannelID(ctx, channel.ID)
	if err != nil {
		s.logger.Error("查询本地渠道模型配置异常",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_id", channel.Name),
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

	s.logger.Debug("",
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_id", channel.Name),
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) CherryStudio/1.7.13 Chrome/140.0.7339.249 Electron/38.7.0 Safari/537.36")

	// 查询渠道的活跃密钥
	keys, err := s.keyRepo.FindActiveByChannelID(ctx, channel.ID)
	if err != nil {
		s.logger.Warn("查询渠道可用密钥异常",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_ nane", channel.Name),
			slog.String("error", err.Error()),
		)
	}

	// 如果有可用密钥，选择第一个并添加到认证头
	if len(keys) > 0 {
		selectedKey := keys[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", selectedKey.KeyValue))

	} else {
		s.logger.Debug("渠道没有可用密钥，查询将以无认证的方式进行",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_ nane", channel.Name),
		)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

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
	upstreamModels := make([]string, len(upstreamResp.Data))
	for i, model := range upstreamResp.Data {
		upstreamModels[i] = model.ID
	}

	return upstreamModels, nil
}

// GetChannel 获取渠道信息
func (s *SyncService) GetChannel(ctx context.Context, channelID uint) (*models.Channel, error) {
	return s.channelRepo.FindByID(ctx, channelID)
}
