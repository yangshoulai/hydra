package logger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
	"gorm.io/gorm"
)

// CleanupService 日志清理服务
type CleanupService struct {
	logger          *slog.Logger
	requestLogRepo  *repository.RequestLogRepository
	db              *gorm.DB
	retentionDays   int // 日志保留天数
}

// NewCleanupService 创建日志清理服务
func NewCleanupService(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
	db *gorm.DB,
	retentionDays int,
) *CleanupService {
	if retentionDays <= 0 {
		retentionDays = 30 // 默认保留30天
	}

	return &CleanupService{
		logger:         logger,
		requestLogRepo: requestLogRepo,
		db:             db,
		retentionDays:  retentionDays,
	}
}

// CleanupResult 清理结果
type CleanupResult struct {
	DeletedCount    int64     `json:"deleted_count"`
	VacuumSucceeded bool      `json:"vacuum_succeeded"`
	VacuumError     string    `json:"vacuum_error,omitempty"`
	Duration        time.Duration `json:"duration"`
	ExecutedAt      time.Time `json:"executed_at"`
}

// CleanupOldLogs 清理过期日志
func (s *CleanupService) CleanupOldLogs(ctx context.Context) (*CleanupResult, error) {
	startTime := time.Now()

	s.logger.Info("starting log cleanup",
		slog.Int("retention_days", s.retentionDays),
	)

	// 计算删除的截止时间
	cutoffTime := time.Now().AddDate(0, 0, -s.retentionDays)

	s.logger.Info("deleting logs before cutoff time",
		slog.Time("cutoff_time", cutoffTime),
		slog.String("cutoff_time_iso", cutoffTime.Format(time.RFC3339)),
	)

	// 删除过期日志
	deletedCount, err := s.requestLogRepo.DeleteBefore(ctx, cutoffTime)
	if err != nil {
		s.logger.Error("failed to delete old logs",
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to delete old logs: %w", err)
	}

	s.logger.Info("deleted old logs successfully",
		slog.Int64("deleted_count", deletedCount),
	)

	// 执行VACUUM以回收空间
	vacuumSucceeded, vacuumErr := s.vacuumDatabase(ctx)

	result := &CleanupResult{
		DeletedCount:    deletedCount,
		VacuumSucceeded: vacuumSucceeded,
		VacuumError:     vacuumErr,
		Duration:        time.Since(startTime),
		ExecutedAt:      startTime,
	}

	// 记录清理结果
	if vacuumSucceeded {
		s.logger.Info("log cleanup completed successfully",
			slog.Int64("deleted_count", deletedCount),
			slog.Duration("duration", result.Duration),
		)
	} else {
		s.logger.Warn("log cleanup completed with vacuum error",
			slog.Int64("deleted_count", deletedCount),
			slog.String("vacuum_error", vacuumErr),
			slog.Duration("duration", result.Duration),
		)
	}

	return result, nil
}

// vacuumDatabase 执行数据库VACUUM操作
func (s *CleanupService) vacuumDatabase(ctx context.Context) (bool, string) {
	s.logger.Info("starting database vacuum")

	// VACUUM不能在事务中执行
	err := s.db.WithContext(ctx).Exec("VACUUM").Error

	if err != nil {
		s.logger.Error("database vacuum failed",
			slog.String("error", err.Error()),
		)
		return false, err.Error()
	}

	s.logger.Info("database vacuum completed successfully")
	return true, ""
}

// SetRetentionDays 设置日志保留天数
func (s *CleanupService) SetRetentionDays(days int) {
	if days > 0 {
		s.retentionDays = days
		s.logger.Info("updated log retention days",
			slog.Int("retention_days", days),
		)
	}
}

// GetRetentionDays 获取日志保留天数
func (s *CleanupService) GetRetentionDays() int {
	return s.retentionDays
}
