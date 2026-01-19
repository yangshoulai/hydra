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

// HealthCheckService Key 健康检查服务
type HealthCheckService struct {
	logger       *slog.Logger
	keyRepo      *repository.KeyRepository
	channelRepo  *repository.ChannelRepository
	probeHandler *circuit.ProbeHandler
}

// NewHealthCheckService 创建健康检查服务
func NewHealthCheckService(
	logger *slog.Logger,
	keyRepo *repository.KeyRepository,
	channelRepo *repository.ChannelRepository,
	probeHandler *circuit.ProbeHandler,
) *HealthCheckService {
	return &HealthCheckService{
		logger:       logger,
		keyRepo:      keyRepo,
		channelRepo:  channelRepo,
		probeHandler: probeHandler,
	}
}

// KeyHealthResult Key 健康检查结果
type KeyHealthResult struct {
	KeyID     uint   `json:"key_id"`
	KeyRemark string `json:"key_remark"`
	Status    string `json:"status"` // healthy, unhealthy, error
	Message   string `json:"message"`
	Latency   string `json:"latency"`
}

// ChannelHealthCheckResult 渠道健康检查结果
type ChannelHealthCheckResult struct {
	ChannelID   uint              `json:"channel_id"`
	ChannelName string            `json:"channel_name"`
	TotalKeys   int               `json:"total_keys"`
	HealthyKeys int               `json:"healthy_keys"`
	KeyResults  []KeyHealthResult `json:"key_results"`
}

// CheckChannelHealth 检查指定渠道的所有 Key 健康状态
func (s *HealthCheckService) CheckChannelHealth(ctx context.Context, channelID uint) (*ChannelHealthCheckResult, error) {
	// 查询渠道
	channel, err := s.channelRepo.FindByID(ctx, channelID)
	if err != nil {
		s.logger.Error("渠道不存在", slog.Uint64("channel_id", uint64(channelID)), slog.String("error", err.Error()))
		return nil, err
	}

	if channel == nil {
		return nil, nil
	}

	// 获取渠道的所有 Key
	keys := channel.Keys
	if len(keys) == 0 {
		s.logger.Warn("渠道尚未设置密钥", slog.Uint64("channel_id", uint64(channelID)))
		return &ChannelHealthCheckResult{
			ChannelID:   channel.ID,
			ChannelName: channel.Name,
			TotalKeys:   0,
			HealthyKeys: 0,
			KeyResults:  []KeyHealthResult{},
		}, nil
	}

	s.logger.Info("开始检查渠道密钥状态", slog.Uint64("channel_id", uint64(channelID)), slog.String("channel_name", channel.Name), slog.Int("total_keys", len(keys)))

	// 并发检查所有 Key
	results := s.checkKeysParallel(ctx, keys, channel)

	// 统计健康的 Key 数量
	healthyCount := 0
	for _, result := range results {
		if result.Status == "healthy" {
			healthyCount++
		}
	}

	return &ChannelHealthCheckResult{
		ChannelID:   channel.ID,
		ChannelName: channel.Name,
		TotalKeys:   len(keys),
		HealthyKeys: healthyCount,
		KeyResults:  results,
	}, nil
}

// checkKeysParallel 并发检查多个 Key
func (s *HealthCheckService) checkKeysParallel(ctx context.Context, keys []models.Key, channel *models.Channel) []KeyHealthResult {
	results := make([]KeyHealthResult, len(keys))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 使用并发控制，避免过多并发请求
	semaphore := make(chan struct{}, 10) // 最多 10 个并发

	for i, key := range keys {
		wg.Add(1)
		go func(index int, k models.Key) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := s.checkSingleKey(ctx, &k, channel)

			mu.Lock()
			results[index] = result
			mu.Unlock()
		}(i, key)
	}

	wg.Wait()
	return results
}

// CheckSingleKey 检查单个 Key 的健康状态（公共方法）
func (s *HealthCheckService) CheckSingleKey(ctx context.Context, key *models.Key, channel *models.Channel) KeyHealthResult {
	return s.checkSingleKey(ctx, key, channel)
}

// checkSingleKey 检查单个 Key
func (s *HealthCheckService) checkSingleKey(ctx context.Context, key *models.Key, channel *models.Channel) KeyHealthResult {
	s.logger.Debug("检查密钥状态", slog.Uint64("key_id", uint64(key.ID)), slog.Uint64("channel_id", uint64(channel.ID)))

	start := time.Now()

	// 使用探测处理器进行健康检查
	success, isHardFailure, err := s.probeHandler.ProbeKey(ctx, key, channel)

	latency := time.Since(start)

	result := KeyHealthResult{
		KeyID:     key.ID,
		KeyRemark: key.Remark,
		Latency:   latency.String(),
	}

	if success {
		result.Status = "healthy"
		result.Message = "密钥正常"
		s.logger.Debug("密钥状态正常", slog.Uint64("key_id", uint64(key.ID)), slog.Duration("latency", latency))
	} else {
		if isHardFailure {
			result.Status = "unhealthy"
			result.Message = "密钥异常"
		} else {
			result.Status = "error"
			if err != nil {
				result.Message = err.Error()
			} else {
				result.Message = "密钥异常"
			}
		}
		s.logger.Warn("密钥检查异常", slog.Uint64("key_id", uint64(key.ID)), slog.String("status", result.Status), slog.String("message", result.Message))
	}

	return result
}

// CheckAllChannels 检查所有渠道的健康状态
func (s *HealthCheckService) CheckAllChannels(ctx context.Context) ([]ChannelHealthCheckResult, error) {
	// 查询所有激活的渠道
	channels, err := s.channelRepo.FindActive(ctx)
	if err != nil {
		s.logger.Error("failed to find channels",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	results := make([]ChannelHealthCheckResult, 0, len(channels))

	for _, channel := range channels {
		result, err := s.CheckChannelHealth(ctx, channel.ID)
		if err != nil {
			s.logger.Error("检查渠道密钥状态异常", slog.Uint64("channel_id", uint64(channel.ID)), slog.String("error", err.Error()))
			continue
		}

		if result != nil {
			results = append(results, *result)
		}
	}

	return results, nil
}
