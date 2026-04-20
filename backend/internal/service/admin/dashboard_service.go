package admin

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	metricsService "github.com/yangshoulai/hydra/internal/service/metrics"
)

// DashboardService 仪表盘服务
type DashboardService struct {
	logger          *slog.Logger
	channelRepo     *repository.ChannelRepository
	channelKeyRepo  *repository.ChannelKeyRepository
	modelConfigRepo *repository.ChannelModelConfigRepository
	modelRepo       *repository.ModelRepository
	requestLogRepo  *repository.RequestLogRepository
	circuitManager  *circuit.CircuitManager
	runtimeMetrics  *metricsService.RuntimeMetrics
}

// NewDashboardService 创建仪表盘服务
func NewDashboardService(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	channelKeyRepo *repository.ChannelKeyRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	modelRepo *repository.ModelRepository,
	requestLogRepo *repository.RequestLogRepository,
	circuitManager *circuit.CircuitManager,
	runtimeMetrics *metricsService.RuntimeMetrics,
) *DashboardService {
	return &DashboardService{
		logger:          logger,
		channelRepo:     channelRepo,
		channelKeyRepo:  channelKeyRepo,
		modelConfigRepo: modelConfigRepo,
		modelRepo:       modelRepo,
		requestLogRepo:  requestLogRepo,
		circuitManager:  circuitManager,
		runtimeMetrics:  runtimeMetrics,
	}
}

const (
	dashboardStatsWindow = 24 * time.Hour
)

var (
	ErrInvalidCircuitKind    = errors.New("invalid circuit kind")
	ErrCircuitTargetNotFound = errors.New("circuit target not found")
)

type QPSRange string

const (
	QPSRange1H  QPSRange = "1h"
	QPSRange6H  QPSRange = "6h"
	QPSRange24H QPSRange = "24h"
)

type OverallHealthStatus struct {
	TotalChannels int     `json:"total_channels"`
	OverallHealth float64 `json:"overall_health"`
	TotalKeys     int     `json:"total_keys"`
	HealthyKeys   int     `json:"healthy_keys"`
	UnhealthyKeys int     `json:"unhealthy_keys"`
}

type DashboardQPSMetrics struct {
	CurrentQPS    float64        `json:"current_qps"`
	QPSTimeSeries []QPSDataPoint `json:"qps_time_series"`
}

type DashboardSuccessRateMetrics struct {
	TodaySuccessRate SuccessRateStats `json:"today_success_rate"`
}

type DashboardChannelHealthMetrics struct {
	OverallHealth     OverallHealthStatus    `json:"overall_health"`
	ChannelHealthList []ChannelHealthMetrics `json:"channel_health_list"`
}

type ClearCircuitResult struct {
	Kind           string `json:"kind"`
	ID             uint   `json:"id"`
	RestoredStatus bool   `json:"restored_status"`
}

type dashboardLogSnapshot struct {
	Summary           *repository.RequestLogSummary
	QPSTrend          []QPSDataPoint
	ChannelAggregates map[uint]repository.RequestLogChannelAggregate
	ModelAggregates   map[string]repository.RequestLogModelAggregate
}

// DashboardMetrics 仪表盘指标
type DashboardMetrics struct {
	QPS                   float64                `json:"qps"`
	CurrentQPS            float64                `json:"current_qps"`
	SuccessRate           float64                `json:"success_rate"`
	TodaySuccessRate      SuccessRateStats       `json:"today_success_rate"`
	OverallHealth         OverallHealthStatus    `json:"overall_health"`
	ChannelStats          []ChannelHealthMetrics `json:"channel_stats"`
	TotalRequests         int                    `json:"total_requests"`
	ActiveModels          int                    `json:"active_models"`
	ActiveChannels        int                    `json:"active_channels"`
	TotalChannels         int                    `json:"total_channels"`
	TotalKeys             int                    `json:"total_keys"`
	TotalPromptTokens     int64                  `json:"total_prompt_tokens"`
	TotalCompletionTokens int64                  `json:"total_completion_tokens"`
	ModelStats            *ModelStats            `json:"model_stats"`
	QPSTrend              []QPSDataPoint         `json:"qps_trend"`
	ChannelHealthList     []ChannelHealthMetrics `json:"channel_health_list"`
	GeneratedAt           string                 `json:"generated_at"`
}

// GetMetrics 获取仪表盘指标
func (s *DashboardService) GetMetrics(ctx context.Context) (*DashboardMetrics, error) {
	return s.GetMetricsWithQPSRange(ctx, QPSRange1H)
}

func (s *DashboardService) GetMetricsWithQPSRange(ctx context.Context, qpsRange QPSRange) (*DashboardMetrics, error) {
	now := time.Now()
	runtimeSnapshot := s.snapshotNow(now)
	logSnapshot, err := s.loadLogSnapshot(ctx, now, normalizeQPSRange(qpsRange))
	if err != nil {
		return nil, err
	}

	modelStats, err := s.buildModelStats(ctx, logSnapshot)
	if err != nil {
		return nil, err
	}

	allChannels, err := s.channelRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	channelStats, overallHealth, err := s.buildChannelHealthMetrics(ctx, logSnapshot.ChannelAggregates)
	if err != nil {
		return nil, err
	}

	successRateStats := toSuccessRateStats(logSnapshot.Summary)

	return &DashboardMetrics{
		QPS:                   runtimeSnapshot.CurrentQPS,
		CurrentQPS:            runtimeSnapshot.CurrentQPS,
		SuccessRate:           successRateStats.SuccessRate,
		TodaySuccessRate:      successRateStats,
		OverallHealth:         overallHealth,
		ChannelStats:          channelStats,
		TotalRequests:         logSnapshot.Summary.TotalRequests,
		ActiveModels:          modelStats.ActiveModels,
		ActiveChannels:        len(channelStats),
		TotalChannels:         len(allChannels),
		TotalKeys:             overallHealth.TotalKeys,
		TotalPromptTokens:     logSnapshot.Summary.TotalPromptTokens,
		TotalCompletionTokens: logSnapshot.Summary.TotalCompletionTokens,
		ModelStats:            modelStats,
		QPSTrend:              logSnapshot.QPSTrend,
		ChannelHealthList:     channelStats,
		GeneratedAt:           now.Format(time.RFC3339),
	}, nil
}

// GetQPSMetrics 获取 QPS 指标
func (s *DashboardService) GetQPSMetrics(ctx context.Context) (*DashboardQPSMetrics, error) {
	return s.GetQPSMetricsWithRange(ctx, QPSRange1H)
}

func (s *DashboardService) GetQPSMetricsWithRange(ctx context.Context, qpsRange QPSRange) (*DashboardQPSMetrics, error) {
	now := time.Now()
	runtimeSnapshot := s.snapshotNow(now)
	logSnapshot, err := s.loadLogSnapshot(ctx, now, normalizeQPSRange(qpsRange))
	if err != nil {
		return nil, err
	}
	return &DashboardQPSMetrics{
		CurrentQPS:    runtimeSnapshot.CurrentQPS,
		QPSTimeSeries: logSnapshot.QPSTrend,
	}, nil
}

// GetSuccessRateMetrics 获取成功率指标
func (s *DashboardService) GetSuccessRateMetrics(ctx context.Context) (*DashboardSuccessRateMetrics, error) {
	logSnapshot, err := s.loadLogSnapshot(ctx, time.Now(), QPSRange1H)
	if err != nil {
		return nil, err
	}
	return &DashboardSuccessRateMetrics{
		TodaySuccessRate: toSuccessRateStats(logSnapshot.Summary),
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
func (s *DashboardService) GetChannelHealthMetrics(ctx context.Context) (*DashboardChannelHealthMetrics, error) {
	logSnapshot, err := s.loadLogSnapshot(ctx, time.Now(), QPSRange1H)
	if err != nil {
		return nil, err
	}
	channelStats, overallHealth, err := s.buildChannelHealthMetrics(ctx, logSnapshot.ChannelAggregates)
	if err != nil {
		return nil, err
	}
	return &DashboardChannelHealthMetrics{
		OverallHealth:     overallHealth,
		ChannelHealthList: channelStats,
	}, nil
}

// GetCircuitStatus 获取熔断状态快照
func (s *DashboardService) GetCircuitStatus(_ context.Context) ([]circuit.BreakerSnapshot, error) {
	return s.circuitManager.SnapshotBreakers(), nil
}

// ClearCircuit 手动清除熔断状态
func (s *DashboardService) ClearCircuit(ctx context.Context, kind string, id uint) (*ClearCircuitResult, error) {
	switch kind {
	case "key":
		channelKey, err := s.channelKeyRepo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if channelKey == nil {
			return nil, ErrCircuitTargetNotFound
		}

		restoredStatus := false
		if channelKey.Status == "inactive" {
			channelKey.Status = "active"
			if err := s.channelKeyRepo.Update(ctx, channelKey); err != nil {
				return nil, err
			}
			restoredStatus = true
		}

		s.circuitManager.RemoveKeyBreaker(id)
		s.logger.Info("手动清除密钥熔断状态",
			slog.Uint64("key_id", uint64(id)),
			slog.Bool("restored_status", restoredStatus),
		)

		return &ClearCircuitResult{
			Kind:           kind,
			ID:             id,
			RestoredStatus: restoredStatus,
		}, nil
	case "model":
		modelConfig, err := s.modelConfigRepo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if modelConfig == nil {
			return nil, ErrCircuitTargetNotFound
		}

		restoredStatus := false
		if modelConfig.Status == "inactive" {
			modelConfig.Status = "active"
			if err := s.modelConfigRepo.Update(ctx, modelConfig); err != nil {
				return nil, err
			}
			restoredStatus = true
		}

		s.circuitManager.RemoveModelConfigBreaker(id)
		s.logger.Info("手动清除模型配置熔断状态",
			slog.Uint64("model_config_id", uint64(id)),
			slog.Bool("restored_status", restoredStatus),
		)

		return &ClearCircuitResult{
			Kind:           kind,
			ID:             id,
			RestoredStatus: restoredStatus,
		}, nil
	default:
		return nil, ErrInvalidCircuitKind
	}
}

func (s *DashboardService) buildChannelHealthMetrics(
	ctx context.Context,
	aggregates map[uint]repository.RequestLogChannelAggregate,
) ([]ChannelHealthMetrics, OverallHealthStatus, error) {
	channels, err := s.channelRepo.FindActive(ctx)
	if err != nil {
		return nil, OverallHealthStatus{}, err
	}

	metrics := make([]ChannelHealthMetrics, 0, len(channels))
	overall := OverallHealthStatus{
		TotalChannels: len(channels),
	}

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

		channelStat := aggregates[channel.ID]
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

		overall.TotalKeys += len(keys)
		overall.HealthyKeys += healthyKeys
	}

	overall.UnhealthyKeys = overall.TotalKeys - overall.HealthyKeys
	if overall.TotalKeys > 0 {
		overall.OverallHealth = float64(overall.HealthyKeys) / float64(overall.TotalKeys) * 100
	}

	return metrics, overall, nil
}

func (s *DashboardService) buildModelStats(ctx context.Context, logSnapshot *dashboardLogSnapshot) (*ModelStats, error) {
	stats := &ModelStats{
		ActiveModels:    0,
		TotalRequests:   logSnapshot.Summary.TotalRequests,
		SuccessRequests: logSnapshot.Summary.SuccessRequests,
		FailedRequests:  logSnapshot.Summary.FailedRequests,
		ModelList:       make([]ModelDetailInfo, 0),
	}

	activeModels, err := s.modelRepo.ListWithActiveChannelConfigs(ctx)
	if err != nil {
		return nil, err
	}
	stats.ActiveModels = len(activeModels)

	agg := make(map[string]ModelDetailInfo)
	for _, modelStat := range logSnapshot.ModelAggregates {
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

func (s *DashboardService) loadLogSnapshot(ctx context.Context, now time.Time, qpsRange QPSRange) (*dashboardLogSnapshot, error) {
	statsSince := now.UTC().Add(-dashboardStatsWindow)
	rangeConfig := getQPSRangeConfig(normalizeQPSRange(qpsRange))
	qpsSince := now.UTC().Truncate(time.Minute).Add(-rangeConfig.window + time.Duration(rangeConfig.bucketMinutes)*time.Minute)

	summary, err := s.requestLogRepo.AggregateSummary(ctx, statsSince)
	if err != nil {
		s.logger.Error("聚合请求日志总览失败", slog.String("error", err.Error()))
		return nil, err
	}

	qpsRows, err := s.requestLogRepo.AggregateQPSByMinute(ctx, qpsSince)
	if err != nil {
		s.logger.Error("聚合 QPS 趋势失败", slog.String("error", err.Error()))
		return nil, err
	}

	channelRows, err := s.requestLogRepo.AggregateByChannel(ctx, statsSince)
	if err != nil {
		s.logger.Error("聚合渠道统计失败", slog.String("error", err.Error()))
		return nil, err
	}

	modelRows, err := s.requestLogRepo.AggregateByModel(ctx, statsSince)
	if err != nil {
		s.logger.Error("聚合模型统计失败", slog.String("error", err.Error()))
		return nil, err
	}

	channelMap := make(map[uint]repository.RequestLogChannelAggregate, len(channelRows))
	for _, item := range channelRows {
		channelMap[item.ChannelID] = item
	}

	modelMap := make(map[string]repository.RequestLogModelAggregate, len(modelRows))
	for _, item := range modelRows {
		modelMap[item.ModelName] = item
	}

	return &dashboardLogSnapshot{
		Summary:           summary,
		QPSTrend:          buildQPSDataPoints(now, qpsRows, rangeConfig),
		ChannelAggregates: channelMap,
		ModelAggregates:   modelMap,
	}, nil
}

func (s *DashboardService) snapshotNow(now time.Time) metricsService.Snapshot {
	return s.runtimeMetrics.Snapshot(now)
}

type qpsRangeConfig struct {
	window        time.Duration
	bucketMinutes int
}

func getQPSRangeConfig(qpsRange QPSRange) qpsRangeConfig {
	switch normalizeQPSRange(qpsRange) {
	case QPSRange6H:
		return qpsRangeConfig{
			window:        6 * time.Hour,
			bucketMinutes: 5,
		}
	case QPSRange24H:
		return qpsRangeConfig{
			window:        24 * time.Hour,
			bucketMinutes: 15,
		}
	default:
		return qpsRangeConfig{
			window:        1 * time.Hour,
			bucketMinutes: 1,
		}
	}
}

func normalizeQPSRange(qpsRange QPSRange) QPSRange {
	switch qpsRange {
	case QPSRange6H, QPSRange24H:
		return qpsRange
	default:
		return QPSRange1H
	}
}

func buildQPSDataPoints(now time.Time, rows []repository.RequestLogMinuteAggregate, config qpsRangeConfig) []QPSDataPoint {
	countMap := make(map[int64]int, len(rows))
	for _, row := range rows {
		bucketMinute := row.MinuteUnix - (row.MinuteUnix % int64(config.bucketMinutes))
		countMap[bucketMinute] += row.TotalRequests
	}

	bucketSpan := int64(config.bucketMinutes)
	result := make([]QPSDataPoint, 0, int(config.window.Minutes())/config.bucketMinutes)
	currentMinute := now.UTC().Truncate(time.Minute).Unix() / 60
	currentBucket := currentMinute - (currentMinute % bucketSpan)
	totalBuckets := int(config.window.Minutes()) / config.bucketMinutes
	startBucket := currentBucket - int64(totalBuckets-1)*bucketSpan

	for minute := startBucket; minute <= currentBucket; minute += bucketSpan {
		count := countMap[minute]
		result = append(result, QPSDataPoint{
			Timestamp: time.Unix(minute*60, 0).Local().Format("15:04"),
			QPS:       float64(count) / float64(config.bucketMinutes*60),
		})
	}

	return result
}

func toSuccessRateStats(summary *repository.RequestLogSummary) SuccessRateStats {
	if summary == nil {
		return SuccessRateStats{}
	}

	successRate := 0.0
	if summary.TotalRequests > 0 {
		successRate = float64(summary.SuccessRequests) / float64(summary.TotalRequests) * 100
	}

	return SuccessRateStats{
		TotalRequests:   summary.TotalRequests,
		SuccessRequests: summary.SuccessRequests,
		FailedRequests:  summary.FailedRequests,
		SuccessRate:     successRate,
	}
}
