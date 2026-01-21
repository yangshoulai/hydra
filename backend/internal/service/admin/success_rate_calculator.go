package admin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
)

// SuccessRateCalculator 成功率计算器
type SuccessRateCalculator struct {
	logger         *slog.Logger
	requestLogRepo *repository.RequestLogRepository
}

// NewSuccessRateCalculator 创建成功率计算器
func NewSuccessRateCalculator(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
) *SuccessRateCalculator {
	return &SuccessRateCalculator{
		logger:         logger,
		requestLogRepo: requestLogRepo,
	}
}

// SuccessRateStats 成功率统计
type SuccessRateStats struct {
	TotalRequests   int     `json:"total_requests"`
	SuccessRequests int     `json:"success_requests"`
	FailedRequests  int     `json:"failed_requests"`
	SuccessRate     float64 `json:"success_rate"`
}

// CalculateTodaySuccessRate 计算今日成功率
func (src *SuccessRateCalculator) CalculateTodaySuccessRate(ctx context.Context) (*SuccessRateStats, error) {
	now := time.Now()
	// 使用 UTC 时间计算今天的开始时间（数据库存储的是 UTC）
	nowUTC := now.UTC()
	startOfDayUTC := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)

	src.logger.Debug("计算今天的成功率",
		slog.Time("start_of_day", startOfDayUTC),
		slog.Time("now", nowUTC),
	)

	return src.calculateSuccessRate(ctx, startOfDayUTC, nowUTC)
}

// CalculateSuccessRateByTimeRange 计算指定时间范围的成功率
func (src *SuccessRateCalculator) CalculateSuccessRateByTimeRange(
	ctx context.Context,
	startTime, endTime time.Time,
) (*SuccessRateStats, error) {
	src.logger.Info("calculating success rate by time range",
		slog.Time("start_time", startTime),
		slog.Time("end_time", endTime),
	)

	return src.calculateSuccessRate(ctx, startTime, endTime)
}

// calculateSuccessRate 计算成功率的核心逻辑
func (src *SuccessRateCalculator) calculateSuccessRate(
	ctx context.Context,
	startTime, endTime time.Time,
) (*SuccessRateStats, error) {
	// 查询时间范围内的所有请求日志
	logs, err := src.requestLogRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get request logs: %w", err)
	}

	stats := &SuccessRateStats{
		TotalRequests: len(logs),
	}

	// 统计成功和失败的请求数
	for _, log := range logs {
		// 使用 IsSuccess 字段判断请求是否成功
		if log.IsSuccess {
			stats.SuccessRequests++
		} else {
			stats.FailedRequests++
		}
	}

	// 计算成功率
	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
	}

	src.logger.Debug("success rate calculation completed",
		slog.Int("total_requests", stats.TotalRequests),
		slog.Int("success_requests", stats.SuccessRequests),
		slog.Int("failed_requests", stats.FailedRequests),
		slog.Float64("success_rate", stats.SuccessRate),
	)

	return stats, nil
}

// CalculateSuccessRateByChannel 计算各渠道的成功率
func (src *SuccessRateCalculator) CalculateSuccessRateByChannel(
	ctx context.Context,
	startTime, endTime time.Time,
) (map[uint]*SuccessRateStats, error) {
	src.logger.Info("calculating success rate by channel",
		slog.Time("start_time", startTime),
		slog.Time("end_time", endTime),
	)

	// 查询时间范围内的所有请求日志
	logs, err := src.requestLogRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get request logs: %w", err)
	}

	// 按渠道分组统计
	channelStats := make(map[uint]*SuccessRateStats)

	for _, log := range logs {
		// 使用 LastChannelID 替代原来的 ChannelID
		if log.LastChannelID == nil {
			continue
		}

		if _, exists := channelStats[*log.LastChannelID]; !exists {
			channelStats[*log.LastChannelID] = &SuccessRateStats{}
		}

		stats := channelStats[*log.LastChannelID]
		stats.TotalRequests++

		if log.IsSuccess {
			stats.SuccessRequests++
		} else {
			stats.FailedRequests++
		}
	}

	// 计算各渠道的成功率
	for channelID, stats := range channelStats {
		if stats.TotalRequests > 0 {
			stats.SuccessRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
		}

		src.logger.Debug("channel success rate calculated",
			slog.Int64("channel_id", int64(channelID)),
			slog.Int("total_requests", stats.TotalRequests),
			slog.Float64("success_rate", stats.SuccessRate),
		)
	}

	return channelStats, nil
}

// CalculateSuccessRateByModel 计算各模型的成功率
func (src *SuccessRateCalculator) CalculateSuccessRateByModel(
	ctx context.Context,
	startTime, endTime time.Time,
) (map[string]*SuccessRateStats, error) {
	src.logger.Info("calculating success rate by model",
		slog.Time("start_time", startTime),
		slog.Time("end_time", endTime),
	)

	// 查询时间范围内的所有请求日志
	logs, err := src.requestLogRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get request logs: %w", err)
	}

	// 按模型分组统计
	modelStats := make(map[string]*SuccessRateStats)

	for _, log := range logs {
		// 使用 UnifiedModel 替代原来的 RequestedModel
		modelName := log.UnifiedModel
		if modelName == "" {
			modelName = "unknown"
		}

		if _, exists := modelStats[modelName]; !exists {
			modelStats[modelName] = &SuccessRateStats{}
		}

		stats := modelStats[modelName]
		stats.TotalRequests++

		if log.IsSuccess {
			stats.SuccessRequests++
		} else {
			stats.FailedRequests++
		}
	}

	// 计算各模型的成功率
	for modelName, stats := range modelStats {
		if stats.TotalRequests > 0 {
			stats.SuccessRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
		}

		src.logger.Debug("model success rate calculated",
			slog.String("model", modelName),
			slog.Int("total_requests", stats.TotalRequests),
			slog.Float64("success_rate", stats.SuccessRate),
		)
	}

	return modelStats, nil
}
