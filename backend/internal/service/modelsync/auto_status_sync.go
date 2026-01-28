package modelsync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

// AutoStatusSyncResult 自动同步结果（仅更新状态）
type AutoStatusSyncResult struct {
	ChannelID           uint      `json:"channel_id"`
	ChannelName         string    `json:"channel_name"`
	FetchedAt           time.Time `json:"fetched_at"`
	UpdatedToNonExist   int       `json:"updated_to_non_exist"`
	UpdatedToActive     int       `json:"updated_to_active"`
	SkippedDisabled     int       `json:"skipped_disabled"`
	TotalLocalProcessed int       `json:"total_local_processed"`
}

// AutoSyncChannelModelStatuses 对比上游 /v1/models 与本地配置（按 upstream_model 对比），仅更新本地配置状态：
// - 本地存在但上游不存在：active -> non_exist
// - 两边都存在且本地是 non_exist：non_exist -> active
// - disabled 状态不参与自动更新（保持人工禁用）
func (s *SyncService) AutoSyncChannelModelStatuses(ctx context.Context, channel *models.Channel) (*AutoStatusSyncResult, error) {
	if channel == nil {
		return nil, fmt.Errorf("channel is nil")
	}

	upstreamModels, _, err := s.fetchUpstreamModels(ctx, channel)
	if err != nil {
		return nil, err
	}

	upstreamSet := make(map[string]struct{}, len(upstreamModels))
	for _, m := range upstreamModels {
		upstreamSet[m] = struct{}{}
	}

	localConfigs, err := s.modelConfigRepo.FindByChannelID(ctx, channel.ID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	result := &AutoStatusSyncResult{
		ChannelID:   channel.ID,
		ChannelName: channel.Name,
		FetchedAt:   now,
	}

	for _, cfg := range localConfigs {
		if cfg == nil {
			continue
		}
		if cfg.Status == "disabled" {
			result.SkippedDisabled++
			continue
		}

		result.TotalLocalProcessed++
		_, exists := upstreamSet[cfg.UpstreamModel]

		switch {
		case !exists && cfg.Status != "non_exist":
			cfg.Status = "non_exist"
			cfg.CoolingAt = nil
			if err := s.modelConfigRepo.Update(ctx, cfg); err != nil {
				return nil, err
			}
			result.UpdatedToNonExist++
			s.circuitManager.RemoveModelConfigBreaker(cfg.ID)

		case exists && cfg.Status == "non_exist":
			cfg.Status = "active"
			cfg.CoolingAt = nil
			if err := s.modelConfigRepo.Update(ctx, cfg); err != nil {
				return nil, err
			}
			result.UpdatedToActive++
			s.circuitManager.RemoveModelConfigBreaker(cfg.ID)
		}
	}

	channel.LastSyncTime = &now
	if err := s.channelRepo.Update(ctx, channel); err != nil {
		return nil, err
	}

	s.logger.Info("渠道模型状态自动同步完成",
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
		slog.Int("updated_to_non_exist", result.UpdatedToNonExist),
		slog.Int("updated_to_active", result.UpdatedToActive),
		slog.Int("skipped_disabled", result.SkippedDisabled),
	)

	return result, nil
}
