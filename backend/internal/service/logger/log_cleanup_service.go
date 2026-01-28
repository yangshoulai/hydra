package logger

import (
	"context"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
	configService "github.com/yangshoulai/hydra/internal/service/config"
)

// LogCleanupService 日志清理服务（审计日志清理）
type LogCleanupService struct {
	logger         *slog.Logger
	settingService *configService.SettingService
	requestLogRepo *repository.RequestLogRepository
}

// NewLogCleanupService 创建日志清理服务
func NewLogCleanupService(
	logger *slog.Logger,
	settingService *configService.SettingService,
	requestLogRepo *repository.RequestLogRepository,
) *LogCleanupService {
	return &LogCleanupService{
		logger:         logger,
		settingService: settingService,
		requestLogRepo: requestLogRepo,
	}
}

// Run 执行日志清理任务
func (s *LogCleanupService) Run(ctx context.Context) error {
	retentionDays, _ := s.settingService.GetLogConfig(ctx)
	if retentionDays <= 0 {
		s.logger.Info("日志清理任务跳过：retention_days <= 0",
			slog.Int("retention_days", retentionDays),
		)
		return nil
	}

	var firstErr error

	mainDeleted, detailDeleted, err := s.cleanupRequestLogs(ctx, retentionDays)
	if err != nil {
		firstErr = err
		s.logger.Error("审计日志清理失败",
			slog.String("error", err.Error()),
		)
	} else {
		s.logger.Info("审计日志清理完成",
			slog.Int("retention_days", retentionDays),
			slog.Int64("deleted_main", mainDeleted),
			slog.Int64("deleted_detail", detailDeleted),
		)
	}

	return firstErr
}

func (s *LogCleanupService) cleanupRequestLogs(ctx context.Context, retentionDays int) (int64, int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	detailDeleted, err := s.requestLogRepo.DeleteDetailsByMainBefore(ctx, cutoff)
	if err != nil {
		return 0, 0, err
	}

	mainDeleted, err := s.requestLogRepo.DeleteMainBefore(ctx, cutoff)
	if err != nil {
		return mainDeleted, detailDeleted, err
	}

	return mainDeleted, detailDeleted, nil
}
