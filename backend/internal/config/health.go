package config

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status   string `json:"status"`   // healthy, unhealthy
	Database string `json:"database"` // 数据库类型
	Latency  string `json:"latency"`  // 响应延迟
	Error    string `json:"error,omitempty"`
}

// CheckDatabaseHealth 检查数据库健康状态
func CheckDatabaseHealth(db *gorm.DB, timeout time.Duration) *HealthCheckResult {
	result := &HealthCheckResult{
		Status: "healthy",
	}

	// 获取数据库类型
	dbName := db.Name()
	result.Database = dbName

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 测量查询延迟
	start := time.Now()

	// 执行简单的 ping 查询
	sqlDB, err := db.DB()
	if err != nil {
		result.Status = "unhealthy"
		result.Error = fmt.Sprintf("failed to get database instance: %v", err)
		return result
	}

	// 使用 PingContext 检查连接
	if err := sqlDB.PingContext(ctx); err != nil {
		result.Status = "unhealthy"
		result.Error = fmt.Sprintf("database ping failed: %v", err)
		return result
	}

	// 记录延迟
	latency := time.Since(start)
	result.Latency = latency.String()

	// 检查连接池状态
	stats := sqlDB.Stats()
	if stats.OpenConnections == 0 {
		result.Status = "unhealthy"
		result.Error = "no open database connections"
		return result
	}

	return result
}

// CheckDatabaseHealthSimple 简化的健康检查(用于启动时检查)
func CheckDatabaseHealthSimple(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}
