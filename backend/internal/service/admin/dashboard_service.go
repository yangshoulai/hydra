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
	CurrentQPS         float64                 `json:"current_qps"`
	SuccessRate        float64                 `json:"success_rate"`
	TotalRequests      int                     `json:"total_requests"`
	ActiveModels       int                     `json:"active_models"`
	ActiveChannels     int                     `json:"active_channels"`
	TotalChannels      int                     `json:"total_channels"`
	ModelStats         *ModelStats             `json:"model_stats"`
	QPSTrend           []QPSDataPoint          `json:"qps_trend"`
	ChannelHealthList  []ChannelHealthMetrics  `json:"channel_health_list"`
}

// GetMetrics 获取仪表盘指标
func (s *DashboardService) GetMetrics(ctx context.Context) (*DashboardMetrics, error) {
	qps, err := s.qpsAggregator.GetCurrentQPS(ctx)
	if err != nil {
		s.logger.Error("failed to get current QPS", slog.String("error", err.Error()))
		return nil, err
	}

	successRateStats, err := s.successRateCalculator.CalculateTodaySuccessRate(ctx)
	if err != nil {
		s.logger.Error("failed to calculate success rate", slog.String("error", err.Error()))
		return nil, err
	}

	modelStats, err := s.modelStatsAggregator.GetTodayModelStats(ctx)
	if err != nil {
		s.logger.Error("failed to get model stats", slog.String("error", err.Error()))
		return nil, err
	}

	activeChannels, err := s.channelRepo.FindActive(ctx)
	if err != nil {
		s.logger.Error("failed to get active channels", slog.String("error", err.Error()))
		return nil, err
	}

	allChannels, err := s.channelRepo.FindAll(ctx)
	if err != nil {
		s.logger.Error("failed to get all channels", slog.String("error", err.Error()))
		return nil, err
	}

	qpsTrend, err := s.qpsAggregator.AggregateLastHour(ctx)
	if err != nil {
		s.logger.Error("failed to get QPS trend", slog.String("error", err.Error()))
		return nil, err
	}

	channelStats, err := s.GetChannelHealthMetrics(ctx)
	if err != nil {
		s.logger.Error("failed to get channel stats", slog.String("error", err.Error()))
		return nil, err
	}

	return &DashboardMetrics{
		CurrentQPS:        qps,
		SuccessRate:       successRateStats.SuccessRate,
		TotalRequests:     successRateStats.TotalRequests,
		ActiveModels:      modelStats.ActiveModels,
		ActiveChannels:    len(activeChannels),
		TotalChannels:     len(allChannels),
		ModelStats:        modelStats,
		QPSTrend:          qpsTrend,
		ChannelHealthList: channelStats,
	}, nil
}

// GetQPSMetrics 获取 QPS 指标
func (s *DashboardService) GetQPSMetrics(ctx context.Context) ([]QPSDataPoint, error) {
	return s.qpsAggregator.AggregateLastHour(ctx)
}

// GetSuccessRateMetrics 获取成功率指标
func (s *DashboardService) GetSuccessRateMetrics(ctx context.Context) (*SuccessRateStats, error) {
	return s.successRateCalculator.CalculateTodaySuccessRate(ctx)
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

	channelStatsMap, err := s.successRateCalculator.CalculateSuccessRateByChannel(ctx,
		s.getTodayStartTime(), s.getCurrentTime())
	if err != nil {
		s.logger.Error("failed to calculate channel success rate", slog.String("error", err.Error()))
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

		metrics = append(metrics, ChannelHealthMetrics{
			ChannelID:        channel.ID,
			ChannelName:      channel.Name,
			Status:           channel.Status,
			TotalRequests:    stats.TotalRequests,
			SuccessRequests:  stats.SuccessRequests,
			FailedRequests:   stats.FailedRequests,
			SuccessRate:      stats.SuccessRate,
			HealthyKeys:      healthyKeys,
			TotalKeys:        len(keys),
			HealthPercentage: healthPercentage,
			Priority:         channel.Priority,
			Weight:           channel.Weight,
		})
	}

	return metrics, nil
}

func (s *DashboardService) getTodayStartTime() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *DashboardService) getCurrentTime() time.Time {
	return time.Now().UTC()
}
