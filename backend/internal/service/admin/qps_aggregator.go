package admin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
)

// QPSAggregator QPS 时间序列聚合器
type QPSAggregator struct {
	logger         *slog.Logger
	requestLogRepo *repository.RequestLogRepository
}

// NewQPSAggregator 创建 QPS 聚合器
func NewQPSAggregator(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
) *QPSAggregator {
	return &QPSAggregator{
		logger:         logger,
		requestLogRepo: requestLogRepo,
	}
}

// QPSDataPoint QPS 数据点
type QPSDataPoint struct {
	Timestamp string  `json:"timestamp"`
	QPS       float64 `json:"qps"`
}

// AggregateLastHour 聚合过去 1 小时的 QPS 数据（按分钟）
func (qa *QPSAggregator) AggregateLastHour(ctx context.Context) ([]QPSDataPoint, error) {
	now := time.Now()
	// 使用 UTC 时间进行查询（数据库存储的是 UTC）
	nowUTC := now.UTC()
	startTimeUTC := nowUTC.Add(-1 * time.Hour)

	qa.logger.Debug("聚合最近一小时的QPS数据",
		slog.Time("start_time", startTimeUTC),
		slog.Time("end_time", nowUTC),
	)

	// 查询过去 1 小时的所有请求日志（使用 UTC 时间）
	logs, err := qa.requestLogRepo.GetByTimeRange(ctx, startTimeUTC, nowUTC)
	if err != nil {
		return nil, fmt.Errorf("failed to get request logs: %w", err)
	}

	// 按分钟分组统计（使用本地时间）
	minuteBuckets := make(map[string]int)

	for _, log := range logs {
		// 将 UTC 时间转换为本地时间再分组
		localTime := log.CreatedAt.Local()
		// 按分钟分组 (格式: 2024-01-12 15:30)
		minuteKey := localTime.Format("2006-01-02 15:04")
		minuteBuckets[minuteKey]++
	}

	// 生成 60 个数据点（每分钟一个点）- 使用本地时间
	dataPoints := make([]QPSDataPoint, 0, 60)
	for i := 59; i >= 0; i-- {
		// 使用本地时间生成时间点
		timestamp := now.Add(-time.Duration(i) * time.Minute)
		localTime := timestamp.Local()
		minuteKey := localTime.Format("2006-01-02 15:04")

		count := minuteBuckets[minuteKey]
		qps := float64(count) / 60.0 // 每分钟的请求数除以 60 秒

		dataPoints = append(dataPoints, QPSDataPoint{
			Timestamp: localTime.Format("15:04"),
			QPS:       qps,
		})
	}

	qa.logger.Debug("QPS aggregation completed",
		slog.Int("data_points", len(dataPoints)),
		slog.Int("total_requests", len(logs)),
	)

	return dataPoints, nil
}

// AggregateByInterval 按指定时间间隔聚合 QPS 数据
func (qa *QPSAggregator) AggregateByInterval(
	ctx context.Context,
	startTime, endTime time.Time,
	interval time.Duration,
) ([]QPSDataPoint, error) {
	qa.logger.Info("aggregating QPS data by interval",
		slog.Time("start_time", startTime),
		slog.Time("end_time", endTime),
		slog.Duration("interval", interval),
	)

	// 查询时间范围内的所有请求日志
	logs, err := qa.requestLogRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get request logs: %w", err)
	}

	// 按时间间隔分组统计
	intervalBuckets := make(map[string]int)

	for _, log := range logs {
		// 按间隔分组（使用 Unix 时间戳整除间隔秒数）
		intervalKey := log.CreatedAt.Truncate(interval).Format(time.RFC3339)
		intervalBuckets[intervalKey]++
	}

	// 生成数据点
	dataPoints := make([]QPSDataPoint, 0)
	for timestamp := startTime.Truncate(interval); timestamp.Before(endTime); timestamp = timestamp.Add(interval) {
		intervalKey := timestamp.Format(time.RFC3339)
		count := intervalBuckets[intervalKey]
		qps := float64(count) / interval.Seconds()

		dataPoints = append(dataPoints, QPSDataPoint{
			Timestamp: timestamp.Format("2006-01-02 15:04:05"),
			QPS:       qps,
		})
	}

	return dataPoints, nil
}

// GetCurrentQPS 获取当前 QPS（最近 1 分钟）
func (qa *QPSAggregator) GetCurrentQPS(ctx context.Context) (float64, error) {
	now := time.Now()
	// 使用 UTC 时间进行查询（数据库存储的是 UTC）
	nowUTC := now.UTC()
	startTimeUTC := nowUTC.Add(-1 * time.Minute)

	logs, err := qa.requestLogRepo.GetByTimeRange(ctx, startTimeUTC, nowUTC)
	if err != nil {
		return 0, fmt.Errorf("failed to get request logs: %w", err)
	}

	qps := float64(len(logs)) / 60.0
	return qps, nil
}
