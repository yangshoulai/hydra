package admin

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	metricsService "github.com/yangshoulai/hydra/internal/service/metrics"
)

// DashboardService 仪表盘服务
type DashboardService struct {
	logger         *slog.Logger
	channelRepo    *repository.ChannelRepository
	channelKeyRepo *repository.ChannelKeyRepository
	modelRepo      *repository.ModelRepository
	circuitManager *circuit.CircuitManager
	runtimeMetrics *metricsService.RuntimeMetrics
}

// NewDashboardService 创建仪表盘服务
func NewDashboardService(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	channelKeyRepo *repository.ChannelKeyRepository,
	modelRepo *repository.ModelRepository,
	circuitManager *circuit.CircuitManager,
	runtimeMetrics *metricsService.RuntimeMetrics,
) *DashboardService {
	return &DashboardService{
		logger:         logger,
		channelRepo:    channelRepo,
		channelKeyRepo: channelKeyRepo,
		modelRepo:      modelRepo,
		circuitManager: circuitManager,
		runtimeMetrics: runtimeMetrics,
	}
}

// DashboardMetrics 仪表盘指标
type DashboardMetrics struct {
	CurrentQPS            float64                `json:"current_qps"`
	SuccessRate           float64                `json:"success_rate"`
	TotalRequests         int                    `json:"total_requests"`
	ActiveModels          int                    `json:"active_models"`
	ActiveChannels        int                    `json:"active_channels"`
	TotalChannels         int                    `json:"total_channels"`
	TotalPromptTokens     int64                  `json:"total_prompt_tokens"`
	TotalCompletionTokens int64                  `json:"total_completion_tokens"`
	ModelStats            *ModelStats            `json:"model_stats"`
	QPSTrend              []QPSDataPoint         `json:"qps_trend"`
	ChannelHealthList     []ChannelHealthMetrics `json:"channel_health_list"`
}

// GetMetrics 获取仪表盘指标
func (s *DashboardService) GetMetrics(ctx context.Context) (*DashboardMetrics, error) {
	snapshot := s.snapshotNow()
	modelStats, err := s.buildModelStats(ctx, snapshot)
	if err != nil {
		s.logger.Error("获取模型统计数据异常", slog.String("error", err.Error()))
		return nil, err
	}

	activeChannels, err := s.channelRepo.FindActive(ctx)
	if err != nil {
		s.logger.Error("获取激活渠道列表异常", slog.String("error", err.Error()))
		return nil, err
	}

	allChannels, err := s.channelRepo.FindAll(ctx)
	if err != nil {
		s.logger.Error("获取所有渠道异常", slog.String("error", err.Error()))
		return nil, err
	}

	channelStats, err := s.buildChannelHealthMetrics(ctx, snapshot)
	if err != nil {
		s.logger.Error("获取渠道健康指标异常", slog.String("error", err.Error()))
		return nil, err
	}

	return &DashboardMetrics{
		CurrentQPS:            snapshot.CurrentQPS,
		SuccessRate:           snapshot.SuccessRate,
		TotalRequests:         snapshot.TotalRequests,
		ActiveModels:          modelStats.ActiveModels,
		ActiveChannels:        len(activeChannels),
		TotalChannels:         len(allChannels),
		TotalPromptTokens:     snapshot.TotalPromptTokens,
		TotalCompletionTokens: snapshot.TotalCompletionTokens,
		ModelStats:            modelStats,
		QPSTrend:              toQPSDataPoints(snapshot.QPSTrend),
		ChannelHealthList:     channelStats,
	}, nil
}

// GetQPSMetrics 获取 QPS 指标
func (s *DashboardService) GetQPSMetrics(_ context.Context) ([]QPSDataPoint, error) {
	snapshot := s.snapshotNow()
	return toQPSDataPoints(snapshot.QPSTrend), nil
}

// GetSuccessRateMetrics 获取成功率指标
func (s *DashboardService) GetSuccessRateMetrics(_ context.Context) (*SuccessRateStats, error) {
	snapshot := s.snapshotNow()
	return &SuccessRateStats{
		TotalRequests:   snapshot.TotalRequests,
		SuccessRequests: snapshot.SuccessRequests,
		FailedRequests:  snapshot.FailedRequests,
		SuccessRate:     snapshot.SuccessRate,
	}, nil
}

// ChannelHealthMetrics 渠道健康指标
type ChannelHealthMetrics struct {
	ChannelID        uint    `json:"channel_id"`
	ChannelName      string  `json:"channel_name"`
	Status           string  `json:"status"`
	TotalRequests    int     `json:"total_requests"`
	SuccessRequests  int     `json:"success_requests"`
	FailedRequests   int     `json:"failed_requests"`
	SuccessRate      float64 `json:"success_rate"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	HealthyKeys      int     `json:"healthy_keys"`
	TotalKeys        int     `json:"total_keys"`
	HealthPercentage float64 `json:"health_percentage"`
	Weight           int     `json:"weight"`
}

// GetChannelHealthMetrics 获取渠道健康指标
func (s *DashboardService) GetChannelHealthMetrics(ctx context.Context) ([]ChannelHealthMetrics, error) {
	return s.buildChannelHealthMetrics(ctx, s.snapshotNow())
}

// GetCircuitStatus 获取熔断状态快照
func (s *DashboardService) GetCircuitStatus(_ context.Context) ([]circuit.BreakerSnapshot, error) {
	return s.circuitManager.SnapshotBreakers(), nil
}

func (s *DashboardService) buildChannelHealthMetrics(ctx context.Context, snapshot metricsService.Snapshot) ([]ChannelHealthMetrics, error) {
	channels, err := s.channelRepo.FindActive(ctx)
	if err != nil {
		return nil, err
	}

	metrics := make([]ChannelHealthMetrics, 0, len(channels))
	for _, channel := range channels {
		keys, err := s.channelKeyRepo.FindByChannelID(ctx, channel.ID)
		if err != nil {
			s.logger.Error("failed to get keys for channel",
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("error", err.Error()))
			continue
		}

		healthyKeys := 0
		for _, key := range keys {
			if key.Status == "active" {
				healthyKeys++
			}
		}

		healthPercentage := 0.0
		if len(keys) > 0 {
			healthPercentage = float64(healthyKeys) / float64(len(keys)) * 100
		}

		channelStat := snapshot.ChannelStats[channel.ID]
		successRate := 0.0
		if channelStat.TotalRequests > 0 {
			successRate = float64(channelStat.SuccessRequests) / float64(channelStat.TotalRequests) * 100
		}

		channelName := channel.Name
		if channelStat.ChannelName != "" {
			channelName = channelStat.ChannelName
		}

		metrics = append(metrics, ChannelHealthMetrics{
			ChannelID:        channel.ID,
			ChannelName:      channelName,
			Status:           channel.Status,
			TotalRequests:    channelStat.TotalRequests,
			SuccessRequests:  channelStat.SuccessRequests,
			FailedRequests:   channelStat.FailedRequests,
			SuccessRate:      successRate,
			PromptTokens:     channelStat.PromptTokens,
			CompletionTokens: channelStat.CompletionTokens,
			HealthyKeys:      healthyKeys,
			TotalKeys:        len(keys),
			HealthPercentage: healthPercentage,
			Weight:           channel.Weight,
		})
	}

	return metrics, nil
}

func (s *DashboardService) buildModelStats(ctx context.Context, snapshot metricsService.Snapshot) (*ModelStats, error) {
	stats := &ModelStats{
		ActiveModels:    0,
		TotalRequests:   snapshot.TotalRequests,
		SuccessRequests: snapshot.SuccessRequests,
		FailedRequests:  snapshot.FailedRequests,
		ModelList:       make([]ModelDetailInfo, 0),
	}

	activeModels, err := s.modelRepo.ListWithActiveChannelConfigs(ctx)
	if err != nil {
		return nil, err
	}
	stats.ActiveModels = len(activeModels)

	agg := make(map[string]ModelDetailInfo)
	for _, modelStat := range snapshot.ModelStats {
		agg[modelStat.ModelName] = ModelDetailInfo{
			ModelName:       modelStat.ModelName,
			TotalRequests:   modelStat.TotalRequests,
			SuccessRequests: modelStat.SuccessRequests,
			FailedRequests:  modelStat.FailedRequests,
		}
	}

	for _, model := range activeModels {
		if _, exists := agg[model.Name]; !exists {
			agg[model.Name] = ModelDetailInfo{ModelName: model.Name}
		}
	}

	for _, item := range agg {
		if item.TotalRequests > 0 {
			item.SuccessRate = float64(item.SuccessRequests) / float64(item.TotalRequests) * 100
		}
		stats.ModelList = append(stats.ModelList, item)
	}

	sort.Slice(stats.ModelList, func(i, j int) bool {
		if stats.ModelList[i].TotalRequests == stats.ModelList[j].TotalRequests {
			return stats.ModelList[i].ModelName < stats.ModelList[j].ModelName
		}
		return stats.ModelList[i].TotalRequests > stats.ModelList[j].TotalRequests
	})

	return stats, nil
}

func (s *DashboardService) snapshotNow() metricsService.Snapshot {
	return s.runtimeMetrics.Snapshot(time.Now())
}

func toQPSDataPoints(points []metricsService.QPSPoint) []QPSDataPoint {
	result := make([]QPSDataPoint, 0, len(points))
	for _, point := range points {
		result = append(result, QPSDataPoint{Timestamp: point.Timestamp, QPS: point.QPS})
	}
	return result
}
