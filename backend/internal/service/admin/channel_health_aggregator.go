package admin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// ChannelHealthAggregator 渠道健康状态汇总器
type ChannelHealthAggregator struct {
	logger       *slog.Logger
	channelRepo  *repository.ChannelRepository
	keyRepo      *repository.KeyRepository
	requestLogRepo *repository.RequestLogRepository
}

// NewChannelHealthAggregator 创建渠道健康状态汇总器
func NewChannelHealthAggregator(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	keyRepo *repository.KeyRepository,
	requestLogRepo *repository.RequestLogRepository,
) *ChannelHealthAggregator {
	return &ChannelHealthAggregator{
		logger:       logger,
		channelRepo:  channelRepo,
		keyRepo:      keyRepo,
		requestLogRepo: requestLogRepo,
	}
}

// ChannelHealthInfo 渠道健康信息
type ChannelHealthInfo struct {
	ChannelID        uint                   `json:"channel_id"`
	ChannelName      string                 `json:"channel_name"`
	Status           string                 `json:"status"`
	Priority         int                    `json:"priority"`
	Weight           int                    `json:"weight"`
	TotalKeys        int                    `json:"total_keys"`
	HealthyKeys      int                    `json:"healthy_keys"`
	UnhealthyKeys    int                    `json:"unhealthy_keys"`
	HealthPercentage float64                `json:"health_percentage"`
	LastRequestTime  *time.Time             `json:"last_request_time,omitempty"`
	SuccessRate      float64                `json:"success_rate"`
	TotalRequests    int                    `json:"total_requests"`
	ErrorDistribution map[string]int        `json:"error_distribution,omitempty"`
}

// AggregateAllChannels 汇总所有渠道的健康状态
func (cha *ChannelHealthAggregator) AggregateAllChannels(ctx context.Context) ([]ChannelHealthInfo, error) {
	cha.logger.Info("aggregating channel health status")

	// 获取所有渠道
	channels, err := cha.channelRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get channels: %w", err)
	}

	healthInfos := make([]ChannelHealthInfo, 0, len(channels))

	for _, channel := range channels {
		healthInfo, err := cha.aggregateChannelHealth(ctx, channel)
		if err != nil {
			cha.logger.Warn("failed to aggregate channel health",
				slog.Int64("channel_id", int64(channel.ID)),
				slog.String("error", err.Error()),
			)
			continue
		}

		healthInfos = append(healthInfos, *healthInfo)
	}

	cha.logger.Debug("channel health aggregation completed",
		slog.Int("total_channels", len(healthInfos)),
	)

	return healthInfos, nil
}

// AggregateChannelByID 汇总指定渠道的健康状态
func (cha *ChannelHealthAggregator) AggregateChannelByID(
	ctx context.Context,
	channelID uint,
) (*ChannelHealthInfo, error) {
	channel, err := cha.channelRepo.FindByID(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	return cha.aggregateChannelHealth(ctx, channel)
}

// aggregateChannelHealth 汇总单个渠道的健康状态
func (cha *ChannelHealthAggregator) aggregateChannelHealth(
	ctx context.Context,
	channel *models.Channel,
) (*ChannelHealthInfo, error) {
	healthInfo := &ChannelHealthInfo{
		ChannelID:        channel.ID,
		ChannelName:      channel.Name,
		Status:           channel.Status,
		Priority:         channel.Priority,
		Weight:           channel.Weight,
		ErrorDistribution: make(map[string]int),
	}

	// 获取渠道的所有 Key
	keys, err := cha.keyRepo.FindByChannelID(ctx, channel.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	healthInfo.TotalKeys = len(keys)

	// 统计健康和不健康的 Key 数量
	for _, key := range keys {
		if key.Status == "active" {
			healthInfo.HealthyKeys++
		} else {
			healthInfo.UnhealthyKeys++
		}
	}

	// 计算健康百分比
	if healthInfo.TotalKeys > 0 {
		healthInfo.HealthPercentage = float64(healthInfo.HealthyKeys) / float64(healthInfo.TotalKeys) * 100
	}

	// 获取该渠道今日请求统计
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	logs, err := cha.requestLogRepo.GetByChannelIDAndTimeRange(ctx, channel.ID, startOfDay, now)
	if err != nil {
		cha.logger.Warn("failed to get request logs for channel",
			slog.Int64("channel_id", int64(channel.ID)),
			slog.String("error", err.Error()),
		)
	} else {
		healthInfo.TotalRequests = len(logs)

		// 计算成功率
		successCount := 0
		for _, log := range logs {
			if log.StatusCode >= 200 && log.StatusCode < 300 {
				successCount++
			} else {
				// 统计错误分布
				errorCode := fmt.Sprintf("%d", log.StatusCode)
				healthInfo.ErrorDistribution[errorCode]++
			}

			// 记录最后请求时间
			if healthInfo.LastRequestTime == nil || log.CreatedAt.After(*healthInfo.LastRequestTime) {
				healthInfo.LastRequestTime = &log.CreatedAt
			}
		}

		if healthInfo.TotalRequests > 0 {
			healthInfo.SuccessRate = float64(successCount) / float64(healthInfo.TotalRequests) * 100
		}
	}

	return healthInfo, nil
}

// GetOverallHealthStatus 获取整体健康状态摘要
type OverallHealthStatus struct {
	TotalChannels      int     `json:"total_channels"`
	HealthyChannels    int     `json:"healthy_channels"`
	DegradedChannels   int     `json:"degraded_channels"`
	UnhealthyChannels  int     `json:"unhealthy_channels"`
	OverallHealth      float64 `json:"overall_health"`
	TotalKeys          int     `json:"total_keys"`
	HealthyKeys        int     `json:"healthy_keys"`
	UnhealthyKeys      int     `json:"unhealthy_keys"`
}

func (cha *ChannelHealthAggregator) GetOverallHealthStatus(ctx context.Context) (*OverallHealthStatus, error) {
	cha.logger.Info("calculating overall health status")

	healthInfos, err := cha.AggregateAllChannels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate channel health: %w", err)
	}

	status := &OverallHealthStatus{
		TotalChannels: len(healthInfos),
	}

	for _, info := range healthInfos {
		status.TotalKeys += info.TotalKeys
		status.HealthyKeys += info.HealthyKeys
		status.UnhealthyKeys += info.UnhealthyKeys

		// 根据健康百分比分类渠道状态
		if info.HealthPercentage >= 80 {
			status.HealthyChannels++
		} else if info.HealthPercentage >= 50 {
			status.DegradedChannels++
		} else {
			status.UnhealthyChannels++
		}
	}

	// 计算整体健康度
	if status.TotalKeys > 0 {
		status.OverallHealth = float64(status.HealthyKeys) / float64(status.TotalKeys) * 100
	}

	cha.logger.Debug("overall health status calculated",
		slog.Int("total_channels", status.TotalChannels),
		slog.Int("healthy_channels", status.HealthyChannels),
		slog.Int("degraded_channels", status.DegradedChannels),
		slog.Int("unhealthy_channels", status.UnhealthyChannels),
		slog.Float64("overall_health", status.OverallHealth),
	)

	return status, nil
}
