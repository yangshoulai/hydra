package admin

import (
	"context"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
)

// DashboardService 仪表盘指标计算服务
type DashboardService struct {
	logger                  *slog.Logger
	requestLogRepo          *repository.RequestLogRepository
	channelRepo             *repository.ChannelRepository
	keyRepo                 *repository.KeyRepository
	qpsAggregator           *QPSAggregator
	successRateCalculator   *SuccessRateCalculator
	channelHealthAggregator *ChannelHealthAggregator
	modelStatsAggregator    *ModelStatsAggregator
}

// NewDashboardService 创建仪表盘服务
func NewDashboardService(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
	channelRepo *repository.ChannelRepository,
	keyRepo *repository.KeyRepository,
) *DashboardService {
	qpsAggregator := NewQPSAggregator(logger, requestLogRepo)
	successRateCalculator := NewSuccessRateCalculator(logger, requestLogRepo)
	channelHealthAggregator := NewChannelHealthAggregator(logger, channelRepo, keyRepo, requestLogRepo)
	modelStatsAggregator := NewModelStatsAggregator(logger, requestLogRepo)

	return &DashboardService{
		logger:                  logger,
		requestLogRepo:          requestLogRepo,
		channelRepo:             channelRepo,
		keyRepo:                 keyRepo,
		qpsAggregator:           qpsAggregator,
		successRateCalculator:   successRateCalculator,
		channelHealthAggregator: channelHealthAggregator,
		modelStatsAggregator:    modelStatsAggregator,
	}
}

// DashboardMetrics 仪表盘指标
type DashboardMetrics struct {
	// QPS 相关
	CurrentQPS    float64        `json:"current_qps"`
	QPSTimeSeries []QPSDataPoint `json:"qps_time_series"`

	// 成功率相关
	TodaySuccessRate *SuccessRateStats `json:"today_success_rate"`

	// 渠道健康相关
	OverallHealth     *OverallHealthStatus `json:"overall_health"`
	ChannelHealthList []ChannelHealthInfo  `json:"channel_health_list"`

	// 模型统计
	ModelStats *ModelStats `json:"model_stats"`

	// 系统概览
	TotalRequestsToday int       `json:"total_requests_today"`
	TotalChannels      int       `json:"total_channels"`
	TotalKeys          int       `json:"total_keys"`
	ActiveChannels     int       `json:"active_channels"`
	GeneratedAt        time.Time `json:"generated_at"`
}

// GetMetrics 获取仪表盘指标
func (ds *DashboardService) GetMetrics(ctx context.Context) (*DashboardMetrics, error) {
	ds.logger.Debug("生成仪表板指标")

	startTime := time.Now()
	metrics := &DashboardMetrics{
		GeneratedAt: startTime,
	}

	// 1. 获取当前 QPS
	currentQPS, err := ds.qpsAggregator.GetCurrentQPS(ctx)
	if err != nil {
		ds.logger.Warn("failed to get current QPS", slog.String("error", err.Error()))
	} else {
		metrics.CurrentQPS = currentQPS
	}

	// 2. 获取过去 1 小时的 QPS 时间序列
	qpsTimeSeries, err := ds.qpsAggregator.AggregateLastHour(ctx)
	if err != nil {
		ds.logger.Warn("failed to get QPS time series", slog.String("error", err.Error()))
	} else {
		metrics.QPSTimeSeries = qpsTimeSeries
	}

	// 3. 获取今日成功率
	todaySuccessRate, err := ds.successRateCalculator.CalculateTodaySuccessRate(ctx)
	if err != nil {
		ds.logger.Warn("failed to get today success rate", slog.String("error", err.Error()))
	} else {
		metrics.TodaySuccessRate = todaySuccessRate
		metrics.TotalRequestsToday = todaySuccessRate.TotalRequests
	}

	// 4. 获取渠道健康状态
	overallHealth, err := ds.channelHealthAggregator.GetOverallHealthStatus(ctx)
	if err != nil {
		ds.logger.Warn("failed to get overall health status", slog.String("error", err.Error()))
	} else {
		metrics.OverallHealth = overallHealth
		metrics.TotalChannels = overallHealth.TotalChannels
		metrics.TotalKeys = overallHealth.TotalKeys
		metrics.ActiveChannels = overallHealth.TotalChannels
	}

	// 5. 获取所有渠道的详细健康信息
	channelHealthList, err := ds.channelHealthAggregator.AggregateAllChannels(ctx)
	if err != nil {
		ds.logger.Warn("failed to get channel health list", slog.String("error", err.Error()))
	} else {
		metrics.ChannelHealthList = channelHealthList
	}

	// 6. 获取模型统计
	modelStats, err := ds.modelStatsAggregator.GetTodayModelStats(ctx)
	if err != nil {
		ds.logger.Warn("failed to get model stats", slog.String("error", err.Error()))
	} else {
		metrics.ModelStats = modelStats
	}

	duration := time.Since(startTime)
	ds.logger.Debug("仪表板指标生成完成",
		slog.Duration("generation_time", duration),
	)

	return metrics, nil
}

// GetQPSMetrics 获取 QPS 相关指标
func (ds *DashboardService) GetQPSMetrics(ctx context.Context) (*QPSMetrics, error) {
	currentQPS, err := ds.qpsAggregator.GetCurrentQPS(ctx)
	if err != nil {
		return nil, err
	}

	qpsTimeSeries, err := ds.qpsAggregator.AggregateLastHour(ctx)
	if err != nil {
		return nil, err
	}

	return &QPSMetrics{
		CurrentQPS:    currentQPS,
		QPSTimeSeries: qpsTimeSeries,
	}, nil
}

// QPSMetrics QPS 指标
type QPSMetrics struct {
	CurrentQPS    float64        `json:"current_qps"`
	QPSTimeSeries []QPSDataPoint `json:"qps_time_series"`
}

// GetSuccessRateMetrics 获取成功率相关指标
func (ds *DashboardService) GetSuccessRateMetrics(ctx context.Context) (*SuccessRateMetrics, error) {
	todaySuccessRate, err := ds.successRateCalculator.CalculateTodaySuccessRate(ctx)
	if err != nil {
		return nil, err
	}

	// 按渠道统计成功率
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	channelSuccessRates, err := ds.successRateCalculator.CalculateSuccessRateByChannel(ctx, startOfDay, now)
	if err != nil {
		ds.logger.Warn("failed to calculate success rate by channel", slog.String("error", err.Error()))
	}

	return &SuccessRateMetrics{
		TodaySuccessRate:    todaySuccessRate,
		ChannelSuccessRates: channelSuccessRates,
	}, nil
}

// SuccessRateMetrics 成功率指标
type SuccessRateMetrics struct {
	TodaySuccessRate    *SuccessRateStats          `json:"today_success_rate"`
	ChannelSuccessRates map[uint]*SuccessRateStats `json:"channel_success_rates,omitempty"`
}

// GetChannelHealthMetrics 获取渠道健康相关指标
func (ds *DashboardService) GetChannelHealthMetrics(ctx context.Context) (*ChannelHealthMetrics, error) {
	overallHealth, err := ds.channelHealthAggregator.GetOverallHealthStatus(ctx)
	if err != nil {
		return nil, err
	}

	channelHealthList, err := ds.channelHealthAggregator.AggregateAllChannels(ctx)
	if err != nil {
		return nil, err
	}

	return &ChannelHealthMetrics{
		OverallHealth:     overallHealth,
		ChannelHealthList: channelHealthList,
	}, nil
}

// ChannelHealthMetrics 渠道健康指标
type ChannelHealthMetrics struct {
	OverallHealth     *OverallHealthStatus `json:"overall_health"`
	ChannelHealthList []ChannelHealthInfo  `json:"channel_health_list"`
}
