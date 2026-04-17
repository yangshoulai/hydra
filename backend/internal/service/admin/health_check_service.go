package admin

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

// HealthCheckService 渠道密钥健康检查服务
type HealthCheckService struct {
	logger       *slog.Logger
	channelRepo  *repository.ChannelRepository
	probeHandler *circuit.ProbeHandler
}

// NewHealthCheckService 创建健康检查服务
func NewHealthCheckService(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	probeHandler *circuit.ProbeHandler,
) *HealthCheckService {
	return &HealthCheckService{
		logger:       logger,
		channelRepo:  channelRepo,
		probeHandler: probeHandler,
	}
}

// ChannelKeyHealthResult 渠道密钥健康检查结果
type ChannelKeyHealthResult struct {
	ChannelKeyID     uint   `json:"channel_key_id"`
	ChannelKeyRemark string `json:"channel_key_remark"`
	Status           string `json:"status"` // healthy, unhealthy, error
	Message          string `json:"message"`
	Latency          string `json:"latency"`
}

// ChannelHealthCheckResult 渠道健康检查结果
type ChannelHealthCheckResult struct {
	ChannelID          uint                     `json:"channel_id"`
	ChannelName        string                   `json:"channel_name"`
	TotalChannelKeys   int                      `json:"total_channel_keys"`
	HealthyChannelKeys int                      `json:"healthy_channel_keys"`
	ChannelKeyResults  []ChannelKeyHealthResult `json:"channel_key_results"`
}

// CheckChannelHealth 检查指定渠道的所有渠道密钥健康状态
func (s *HealthCheckService) CheckChannelHealth(ctx context.Context, channelID uint) (*ChannelHealthCheckResult, error) {
	channel, err := s.channelRepo.FindByID(ctx, channelID)
	if err != nil {
		s.logger.Error("查询渠道异常", slog.Uint64("channel_id", uint64(channelID)), slog.String("error", err.Error()))
		return nil, err
	}
	if channel == nil {
		return nil, nil
	}

	channelKeys := channel.ChannelKeys
	if len(channelKeys) == 0 {
		s.logger.Warn("渠道尚未设置渠道密钥", slog.Uint64("channel_id", uint64(channelID)))
		return &ChannelHealthCheckResult{
			ChannelID:          channel.ID,
			ChannelName:        channel.Name,
			TotalChannelKeys:   0,
			HealthyChannelKeys: 0,
			ChannelKeyResults:  []ChannelKeyHealthResult{},
		}, nil
	}

	s.logger.Info("开始检查渠道密钥状态",
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channel.Name),
		slog.Int("total_channel_keys", len(channelKeys)),
	)

	results := s.checkChannelKeysParallel(ctx, channelKeys, channel)

	healthyCount := 0
	for _, result := range results {
		if result.Status == "healthy" {
			healthyCount++
		}
	}

	return &ChannelHealthCheckResult{
		ChannelID:          channel.ID,
		ChannelName:        channel.Name,
		TotalChannelKeys:   len(channelKeys),
		HealthyChannelKeys: healthyCount,
		ChannelKeyResults:  results,
	}, nil
}

// checkChannelKeysParallel 并发检查多个渠道密钥
func (s *HealthCheckService) checkChannelKeysParallel(ctx context.Context, channelKeys []models.ChannelKey, channel *models.Channel) []ChannelKeyHealthResult {
	results := make([]ChannelKeyHealthResult, len(channelKeys))
	var wg sync.WaitGroup
	var mu sync.Mutex

	semaphore := make(chan struct{}, 10)

	for i, channelKey := range channelKeys {
		wg.Add(1)
		go func(index int, k models.ChannelKey) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := s.CheckSingleChannelKey(ctx, &k, channel)
			mu.Lock()
			results[index] = result
			mu.Unlock()
		}(i, channelKey)
	}

	wg.Wait()
	return results
}

// CheckSingleChannelKey 检查单个渠道密钥的健康状态（公共方法）
func (s *HealthCheckService) CheckSingleChannelKey(ctx context.Context, channelKey *models.ChannelKey, channel *models.Channel) ChannelKeyHealthResult {
	s.logger.Debug("检查渠道密钥状态",
		slog.Uint64("channel_key_id", uint64(channelKey.ID)),
		slog.Uint64("channel_id", uint64(channel.ID)),
	)

	start := time.Now()
	success, isHardFailure, err := s.probeHandler.ProbeChannelKey(ctx, channelKey, channel)
	latency := time.Since(start)

	result := ChannelKeyHealthResult{
		ChannelKeyID:     channelKey.ID,
		ChannelKeyRemark: channelKey.Remark,
		Latency:          latency.String(),
	}

	if success {
		result.Status = "healthy"
		result.Message = "渠道密钥正常"
		s.logger.Debug("渠道密钥状态正常", slog.Uint64("channel_key_id", uint64(channelKey.ID)), slog.Duration("latency", latency))
		return result
	}

	if isHardFailure {
		result.Status = "unhealthy"
		result.Message = "渠道密钥异常"
	} else {
		result.Status = "error"
		if err != nil {
			result.Message = err.Error()
		} else {
			result.Message = "渠道密钥异常"
		}
	}

	s.logger.Warn("渠道密钥检查异常",
		slog.Uint64("channel_key_id", uint64(channelKey.ID)),
		slog.String("status", result.Status),
		slog.String("message", result.Message),
	)
	return result
}
