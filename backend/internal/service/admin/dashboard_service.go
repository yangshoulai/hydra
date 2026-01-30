package admin

import (
	"context"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

// DashboardService 仪表盘服务
type DashboardService struct {
	logger                *slog.Logger
	qpsAggregator         *QPSAggregator
	successRateCalculator *SuccessRateCalculator
	modelStatsAggregator  *ModelStatsAggregator
	requestLogRepo        *repository.RequestLogRepository
	channelRepo           *repository.ChannelRepository
	keyRepo               *repository.KeyRepository
	circuitManager        *circuit.Manager
}

// NewDashboardService 创建仪表盘服务
func NewDashboardService(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
	channelRepo *repository.ChannelRepository,
	keyRepo *repository.KeyRepository,
	circuitManager *circuit.Manager,
) *DashboardService {
	return &DashboardService{
		logger:                logger,
		qpsAggregator:         NewQPSAggregator(logger, requestLogRepo),
		successRateCalculator: NewSuccessRateCalculator(logger, requestLogRepo),
		modelStatsAggregator:  NewModelStatsAggregator(logger, requestLogRepo),
		requestLogRepo:        requestLogRepo,
		channelRepo:           channelRepo,
		keyRepo:               keyRepo,
		circuitManager:        circuitManager,
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
	qps, err := s.qpsAggregator.GetCurrentQPS(ctx)
	if err != nil {
		s.logger.Error("获取当前 QPS 异常", slog.String("error", err.Error()))
		return nil, err
	}

	startTime := s.getLast24HoursStartTime()
	endTime := s.getCurrentTime()

	successRateStats, err := s.successRateCalculator.CalculateSuccessRateByTimeRange(ctx, startTime, endTime)
	if err != nil {
		s.logger.Error("计算成功率异常", slog.String("error", err.Error()))
		return nil, err
	}

	modelStats, err := s.modelStatsAggregator.GetModelStatsByTimeRange(ctx, startTime, endTime)
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

	qpsTrend, err := s.qpsAggregator.AggregateLastHour(ctx)
	if err != nil {
		s.logger.Error("获取 QPS", slog.String("error", err.Error()))
		return nil, err
	}

	channelStats, err := s.GetChannelHealthMetrics(ctx)
	if err != nil {
		s.logger.Error("failed to get channel stats", slog.String("error", err.Error()))
		return nil, err
	}

	tokenUsage, err := s.requestLogRepo.SumTokenUsageByTimeRange(ctx, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to get token usage", slog.String("error", err.Error()))
		return nil, err
	}

	return &DashboardMetrics{
		CurrentQPS:            qps,
		SuccessRate:           successRateStats.SuccessRate,
		TotalRequests:         successRateStats.TotalRequests,
		ActiveModels:          modelStats.ActiveModels,
		ActiveChannels:        len(activeChannels),
		TotalChannels:         len(allChannels),
		TotalPromptTokens:     tokenUsage.PromptTokens,
		TotalCompletionTokens: tokenUsage.CompletionTokens,
		ModelStats:            modelStats,
		QPSTrend:              qpsTrend,
		ChannelHealthList:     channelStats,
	}, nil
}

// GetQPSMetrics 获取 QPS 指标
func (s *DashboardService) GetQPSMetrics(ctx context.Context) ([]QPSDataPoint, error) {
	return s.qpsAggregator.AggregateLastHour(ctx)
}

// GetSuccessRateMetrics 获取成功率指标
func (s *DashboardService) GetSuccessRateMetrics(ctx context.Context) (*SuccessRateStats, error) {
	return s.successRateCalculator.CalculateSuccessRateByTimeRange(ctx, s.getLast24HoursStartTime(), s.getCurrentTime())
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
	Priority         int     `json:"priority"`
	Weight           int     `json:"weight"`
}

// GetChannelHealthMetrics 获取渠道健康指标
func (s *DashboardService) GetChannelHealthMetrics(ctx context.Context) ([]ChannelHealthMetrics, error) {
	channels, err := s.channelRepo.FindActive(ctx)
	if err != nil {
		s.logger.Error("failed to get channels", slog.String("error", err.Error()))
		return nil, err
	}

	startTime := s.getLast24HoursStartTime()
	endTime := s.getCurrentTime()

	channelStatsMap, err := s.successRateCalculator.CalculateSuccessRateByChannel(ctx,
		startTime, endTime)
	if err != nil {
		s.logger.Error("failed to calculate channel success rate", slog.String("error", err.Error()))
		return nil, err
	}

	tokenUsageMap, err := s.requestLogRepo.SumTokenUsageByChannel(ctx, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to get channel token usage", slog.String("error", err.Error()))
		return nil, err
	}

	metrics := make([]ChannelHealthMetrics, 0, len(channels))
	for _, channel := range channels {
		stats := channelStatsMap[channel.ID]
		if stats == nil {
			stats = &SuccessRateStats{}
		}

		// 获取渠道的所有密钥
		keys, err := s.keyRepo.FindByChannelID(ctx, channel.ID)
		if err != nil {
			s.logger.Error("failed to get keys for channel",
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("error", err.Error()))
			continue
		}

		// 统计健康密钥数（active 状态）
		healthyKeys := 0
		for _, key := range keys {
			if key.Status == "active" {
				healthyKeys++
			}
		}

		// 计算健康度百分比
		healthPercentage := 0.0
		if len(keys) > 0 {
			healthPercentage = float64(healthyKeys) / float64(len(keys)) * 100
		}

		tokenUsage := tokenUsageMap[channel.ID]
		if tokenUsage == nil {
			tokenUsage = &repository.TokenUsageSummary{}
		}

		metrics = append(metrics, ChannelHealthMetrics{
			ChannelID:        channel.ID,
			ChannelName:      channel.Name,
			Status:           channel.Status,
			TotalRequests:    stats.TotalRequests,
			SuccessRequests:  stats.SuccessRequests,
			FailedRequests:   stats.FailedRequests,
			SuccessRate:      stats.SuccessRate,
			PromptTokens:     tokenUsage.PromptTokens,
			CompletionTokens: tokenUsage.CompletionTokens,
			HealthyKeys:      healthyKeys,
			TotalKeys:        len(keys),
			HealthPercentage: healthPercentage,
			Priority:         channel.Priority,
			Weight:           channel.Weight,
		})
	}

	return metrics, nil
}

func (s *DashboardService) getLast24HoursStartTime() time.Time {
	now := time.Now().UTC()
	return now.Add(-24 * time.Hour)
}

func (s *DashboardService) getCurrentTime() time.Time {
	return time.Now().UTC()
}
