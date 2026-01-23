package modelsync

import (
	"context"
	"log/slog"
	"sync"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	"github.com/yangshoulai/hydra/internal/service/scheduler"
)

const channelModelSyncJobName = "channel-model-status-sync"

// ChannelModelSyncScheduler 渠道模型定时同步调度器
type ChannelModelSyncScheduler struct {
	logger         *slog.Logger
	cronScheduler  *scheduler.CronScheduler
	settingService *configService.SettingService
	syncService    *SyncService
	channelRepo    *repository.ChannelRepository
	mu             sync.Mutex
	enabled        bool
}

// NewChannelModelSyncScheduler 创建渠道模型定时同步调度器
func NewChannelModelSyncScheduler(
	logger *slog.Logger,
	cronScheduler *scheduler.CronScheduler,
	settingService *configService.SettingService,
	channelRepo *repository.ChannelRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	keyRepo *repository.KeyRepository,
) *ChannelModelSyncScheduler {
	syncService := NewSyncService(logger, channelRepo, modelConfigRepo, keyRepo)

	return &ChannelModelSyncScheduler{
		logger:         logger,
		cronScheduler:  cronScheduler,
		settingService: settingService,
		syncService:    syncService,
		channelRepo:    channelRepo,
	}
}

// Initialize 初始化调度器状态（根据配置决定是否启动任务）
func (s *ChannelModelSyncScheduler) Initialize(ctx context.Context) {
	s.refresh(ctx)
}

// OnConfigChanged 响应配置变更
func (s *ChannelModelSyncScheduler) OnConfigChanged(ctx context.Context, category string) {
	if category != "channel_sync" {
		return
	}
	s.refresh(ctx)
}

func (s *ChannelModelSyncScheduler) refresh(ctx context.Context) {
	enabled := s.settingService.GetBool(ctx, models.SettingChannelGlobalSyncEnabled, false)

	s.mu.Lock()
	defer s.mu.Unlock()

	if enabled == s.enabled {
		return
	}

	if enabled {
		if err := s.cronScheduler.AddJob(channelModelSyncJobName, scheduler.Every30Minutes, s.run); err != nil {
			s.logger.Error("渠道模型定时同步任务启动失败",
				slog.String("error", err.Error()),
			)
			return
		}
		s.enabled = true
		s.logger.Info("渠道模型定时同步任务已启动")
		return
	}

	s.cronScheduler.RemoveJob(channelModelSyncJobName)
	s.enabled = false
	s.logger.Info("渠道模型定时同步任务已停止")
}

func (s *ChannelModelSyncScheduler) run(ctx context.Context) error {
	channels, err := s.channelRepo.FindSyncEnabledActive(ctx)
	if err != nil {
		return err
	}

	if len(channels) == 0 {
		s.logger.Debug("没有可同步的渠道")
		return nil
	}

	var failed int
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if _, err := s.syncService.AutoSyncChannelModelStatuses(ctx, channel); err != nil {
			failed++
			s.logger.Warn("渠道模型自动同步失败",
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("channel_name", channel.Name),
				slog.String("error", err.Error()),
			)
		}
	}

	if failed > 0 {
		s.logger.Warn("渠道模型定时同步存在失败",
			slog.Int("failed_count", failed),
			slog.Int("total", len(channels)),
		)
	}

	return nil
}
