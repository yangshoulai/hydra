package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yangshoulai/hydra/internal/runtime"
	loggerService "github.com/yangshoulai/hydra/internal/service/logger"
)

var (
	version   = "dev"
	buildTime = "unknown"
	dataDir   = flag.String("data-dir", "", "data directory for sqlite db & logs (default: <exe_dir>/data)")
)

func main() {
	flag.Parse()
	runtime.SetBuildInfo(version, buildTime)
	fmt.Printf("Hydra API Gateway v%s (build: %s)\n", version, buildTime)

	resolvedDataDir, err := resolveDataDir(*dataDir)
	if err != nil {
		log.Fatalf("解析数据目录失败: %v", err)
	}

	bootstrapLogger, err := loggerService.InitLogger(&loggerService.LoggerConfig{
		Level:      "info",
		AddSource:  false,
		EnableFile: false,
	})
	if err != nil {
		log.Fatalf("初始化 bootstrap 日志失败: %v", err)
	}

	manager := runtime.NewAppManager(bootstrapLogger, resolvedDataDir)
	if err := manager.Start(); err != nil {
		bootstrapLogger.Error("启动 AppManager 失败", slog.String("error", err.Error()))
		os.Exit(1)
	}

	bootstrapLogger.Info("Hydra 已启动", slog.String("data_dir", resolvedDataDir))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	bootstrapLogger.Info("收到退出信号，开始优雅停机")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := manager.Shutdown(shutdownCtx); err != nil {
		bootstrapLogger.Error("停机异常", slog.String("error", err.Error()))
	}

	bootstrapLogger.Info("Hydra 已退出")
}

// resolveDataDir 确定最终的数据目录绝对路径：
//  1. 用户传入 --data-dir → 使用该路径
//  2. 未传入 → <exe_dir>/data
func resolveDataDir(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "data"), nil
}
