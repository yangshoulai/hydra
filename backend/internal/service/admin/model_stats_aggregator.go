package admin

import (
	"context"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// ModelStatsAggregator 模型统计聚合器
type ModelStatsAggregator struct {
	logger         *slog.Logger
	requestLogRepo *repository.RequestLogRepository
}

// NewModelStatsAggregator 创建模型统计聚合器
func NewModelStatsAggregator(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
) *ModelStatsAggregator {
	return &ModelStatsAggregator{
		logger:         logger,
		requestLogRepo: requestLogRepo,
	}
}

// ModelStats 模型统计信息
type ModelStats struct {
	ActiveModels    int               `json:"active_models"`
	TotalRequests   int               `json:"total_requests"`
	SuccessRequests int               `json:"success_requests"`
	FailedRequests  int               `json:"failed_requests"`
	ModelList       []ModelDetailInfo `json:"model_list"`
}

// ModelDetailInfo 模型详细信息
type ModelDetailInfo struct {
	ModelName       string  `json:"model_name"`
	TotalRequests   int     `json:"total_requests"`
	SuccessRequests int     `json:"success_requests"`
	FailedRequests  int     `json:"failed_requests"`
	SuccessRate     float64 `json:"success_rate"`
}

// GetTodayModelStats 获取今日模型统计
func (m *ModelStatsAggregator) GetTodayModelStats(ctx context.Context) (*ModelStats, error) {
	now := time.Now()
	// 使用 UTC 时间计算今天的开始时间（数据库存储的是 UTC）
	nowUTC := now.UTC()
	startOfDayUTC := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)

	return m.GetModelStatsByTimeRange(ctx, startOfDayUTC, nowUTC)
}

// GetModelStatsByTimeRange 获取指定时间范围的模型统计
func (m *ModelStatsAggregator) GetModelStatsByTimeRange(ctx context.Context, startTime, endTime time.Time) (*ModelStats, error) {
	db := m.requestLogRepo.GetDB().WithContext(ctx)

	stats := &ModelStats{}

	// 统计活跃模型数（时间范围内有请求的不同模型数）
	var activeModels int64
	if err := db.Model(&models.RequestLogMain{}).
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Where("requested_model != ''").
		Distinct("requested_model").
		Count(&activeModels).Error; err != nil {
		m.logger.Error("统计激活的模型数异常", slog.String("error", err.Error()))
		return nil, err
	}
	stats.ActiveModels = int(activeModels)

	// 统计时间范围内模型请求总数
	var totalRequests int64
	if err := db.Model(&models.RequestLogMain{}).
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Where("requested_model != ''").
		Count(&totalRequests).Error; err != nil {
		m.logger.Error("统计所有请求数异常", slog.String("error", err.Error()))
		return nil, err
	}
	stats.TotalRequests = int(totalRequests)

	// 统计时间范围内成功请求数
	var successRequests int64
	if err := db.Model(&models.RequestLogMain{}).
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Where("requested_model != ''").
		Where("is_success = ?", true).
		Count(&successRequests).Error; err != nil {
		m.logger.Error("统计成功请求数异常", slog.String("error", err.Error()))
		return nil, err
	}
	stats.SuccessRequests = int(successRequests)

	// 计算失败请求数
	stats.FailedRequests = stats.TotalRequests - stats.SuccessRequests

	// 获取每个模型的详细统计
	type modelStat struct {
		ModelName       string
		TotalRequests   int64
		SuccessRequests int64
	}
	var modelStats []modelStat
	if err := db.Model(&models.RequestLogMain{}).
		Select("requested_model as model_name, COUNT(*) as total_requests, SUM(CASE WHEN is_success = true THEN 1 ELSE 0 END) as success_requests").
		Where("start_time >= ? AND start_time <= ?", startTime, endTime).
		Where("requested_model != ''").
		Group("requested_model").
		Order("total_requests DESC").
		Scan(&modelStats).Error; err != nil {
		m.logger.Error("获取模型详情异常", slog.String("error", err.Error()))
		return nil, err
	}

	stats.ModelList = make([]ModelDetailInfo, len(modelStats))
	for i, ms := range modelStats {
		failedRequests := int(ms.TotalRequests - ms.SuccessRequests)
		successRate := 0.0
		if ms.TotalRequests > 0 {
			successRate = float64(ms.SuccessRequests) / float64(ms.TotalRequests) * 100
		}
		stats.ModelList[i] = ModelDetailInfo{
			ModelName:       ms.ModelName,
			TotalRequests:   int(ms.TotalRequests),
			SuccessRequests: int(ms.SuccessRequests),
			FailedRequests:  failedRequests,
			SuccessRate:     successRate,
		}
	}

	return stats, nil
}
