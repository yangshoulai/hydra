package config

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// DBHealthChecker 数据库健康检测器
type DBHealthChecker struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewDBHealthChecker 创建数据库健康检测器
func NewDBHealthChecker(db *gorm.DB, logger *slog.Logger) *DBHealthChecker {
	return &DBHealthChecker{
		db:     db,
		logger: logger,
	}
}

// PerformStartupCheck 执行启动时的数据库健康检查
// 此检查会验证数据库连接、权限、表结构等
func (h *DBHealthChecker) PerformStartupCheck() error {
	h.logger.Info("performing database startup health check")

	// 1. 检查数据库连接
	if err := h.checkConnection(); err != nil {
		h.logger.Error("database connection check failed",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("connection check failed: %w", err)
	}
	h.logger.Info("✓ database connection check passed")

	// 2. 检查连接池状态
	if err := h.checkConnectionPool(); err != nil {
		h.logger.Error("database connection pool check failed",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("connection pool check failed: %w", err)
	}
	h.logger.Info("✓ database connection pool check passed")

	// 3. 检查表是否存在
	if err := h.checkTablesExist(); err != nil {
		h.logger.Error("database tables check failed",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("tables check failed: %w", err)
	}
	h.logger.Info("✓ database tables check passed")

	// 4. 检查读写权限
	if err := h.checkReadWritePermissions(); err != nil {
		h.logger.Error("database read/write permissions check failed",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("read/write permissions check failed: %w", err)
	}
	h.logger.Info("✓ database read/write permissions check passed")

	h.logger.Info("database startup health check completed successfully")
	return nil
}

// checkConnection 检查数据库连接
func (h *DBHealthChecker) checkConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// checkConnectionPool 检查连接池状态
func (h *DBHealthChecker) checkConnectionPool() error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	stats := sqlDB.Stats()

	h.logger.Info("database connection pool stats",
		slog.Int("open_connections", stats.OpenConnections),
		slog.Int("in_use", stats.InUse),
		slog.Int("idle", stats.Idle),
		slog.Int64("wait_count", stats.WaitCount),
		slog.Duration("wait_duration", stats.WaitDuration),
		slog.Int64("max_idle_closed", stats.MaxIdleClosed),
		slog.Int64("max_lifetime_closed", stats.MaxLifetimeClosed),
	)

	// 验证至少有一个连接
	if stats.OpenConnections == 0 {
		return fmt.Errorf("no open database connections")
	}

	// 检查是否有过多的等待
	if stats.WaitCount > 100 {
		h.logger.Warn("high connection wait count detected",
			slog.Int64("wait_count", stats.WaitCount),
		)
	}

	return nil
}

// checkTablesExist 检查必需的表是否存在
func (h *DBHealthChecker) checkTablesExist() error {
	// 必需的表列表
	requiredTables := []string{
		"channels",
		"keys",
		"channel_model_configs",
		"request_logs",
		"system_settings",
		"access_tokens",
		"admin_users",
	}

	for _, tableName := range requiredTables {
		exists := h.db.Migrator().HasTable(tableName)
		if !exists {
			return fmt.Errorf("required table '%s' does not exist", tableName)
		}

		h.logger.Debug("table exists",
			slog.String("table", tableName),
		)
	}

	return nil
}

// checkReadWritePermissions 检查读写权限
func (h *DBHealthChecker) checkReadWritePermissions() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 使用 system_settings 表进行读写测试
	testKey := "_health_check_test"
	testValue := fmt.Sprintf("test_%d", time.Now().Unix())

	// 测试写入
	result := h.db.WithContext(ctx).Exec(
		"INSERT OR REPLACE INTO system_settings (setting_key, setting_value) VALUES (?, ?)",
		testKey, testValue,
	)
	if result.Error != nil {
		// 尝试 PostgreSQL 语法
		result = h.db.WithContext(ctx).Exec(
			"INSERT INTO system_settings (setting_key, setting_value) VALUES ($1, $2) ON CONFLICT (setting_key) DO UPDATE SET setting_value = $2",
			testKey, testValue,
		)
		if result.Error != nil {
			return fmt.Errorf("write permission check failed: %w", result.Error)
		}
	}

	h.logger.Debug("write permission check passed")

	// 测试读取
	var readValue string
	err := h.db.WithContext(ctx).
		Table("system_settings").
		Where("setting_key = ?", testKey).
		Select("setting_value").
		Scan(&readValue).Error

	if err != nil {
		return fmt.Errorf("read permission check failed: %w", err)
	}

	if readValue != testValue {
		return fmt.Errorf("read value mismatch: expected '%s', got '%s'", testValue, readValue)
	}

	h.logger.Debug("read permission check passed")

	// 清理测试数据
	h.db.WithContext(ctx).Exec("DELETE FROM system_settings WHERE setting_key = ?", testKey)

	return nil
}

// GetDatabaseInfo 获取数据库信息
func (h *DBHealthChecker) GetDatabaseInfo() map[string]interface{} {
	sqlDB, err := h.db.DB()
	if err != nil {
		h.logger.Error("failed to get database instance",
			slog.String("error", err.Error()),
		)
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	stats := sqlDB.Stats()
	dbName := h.db.Name()

	info := map[string]interface{}{
		"database_type":        dbName,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration_ms":     stats.WaitDuration.Milliseconds(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
	}

	return info
}

// MonitorConnectionPool 持续监控连接池状态
// 此方法可以在后台定期运行,记录连接池指标
func (h *DBHealthChecker) MonitorConnectionPool(interval time.Duration, stopChan <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	h.logger.Info("database connection pool monitoring started",
		slog.Duration("interval", interval),
	)

	for {
		select {
		case <-ticker.C:
			info := h.GetDatabaseInfo()
			h.logger.Debug("connection pool stats",
				slog.Any("stats", info),
			)

			// 检查潜在问题
			if openConns, ok := info["open_connections"].(int); ok && openConns == 0 {
				h.logger.Error("no open database connections detected")
			}

			if waitCount, ok := info["wait_count"].(int64); ok && waitCount > 1000 {
				h.logger.Warn("high connection wait count",
					slog.Int64("wait_count", waitCount),
				)
			}

		case <-stopChan:
			h.logger.Info("database connection pool monitoring stopped")
			return
		}
	}
}

// QuickHealthCheck 快速健康检查(用于健康检查端点)
func (h *DBHealthChecker) QuickHealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}
