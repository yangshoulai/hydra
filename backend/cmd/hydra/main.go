package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/api/admin"
	"github.com/yangshoulai/hydra/internal/api/proxy"
	"github.com/yangshoulai/hydra/internal/config"
	_ "github.com/yangshoulai/hydra/internal/endpoint" // 导入端点包以触发 init 函数
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/migration"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	loggerService "github.com/yangshoulai/hydra/internal/service/logger"
	proxyService "github.com/yangshoulai/hydra/internal/service/proxy"
	schedulerService "github.com/yangshoulai/hydra/internal/service/scheduler"
	"gorm.io/gorm"
)

var (
	configPath = flag.String("config", "", "path to config file")
	version    = "1.0.0"
)

func main() {
	flag.Parse()

	// 打印版本信息
	fmt.Printf("Hydra API Gateway v%s\n", version)

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志系统
	mainLogger, err := initLogger(cfg)
	if err != nil {
		log.Fatalf("初始化日志系统失败: %v", err)
	}

	mainLogger.Info("启动 Hydra API 网关", slog.String("version", version), slog.Int("port", cfg.Server.Port))

	// 初始化数据库
	db, err := initDatabase(cfg, mainLogger)
	if err != nil {
		mainLogger.Error("初始化数据库失败", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer config.CloseDatabase(db)

	// 创建系统设置仓储（全局唯一实例）
	systemSettingRepo := repository.NewSystemSettingRepository(db)

	// 初始化系统设置
	if err := initSystemSettings(systemSettingRepo, mainLogger); err != nil {
		mainLogger.Error("初始化系统设置失败", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// 创建系统设置服务（全局唯一实例）
	settingService := configService.NewSettingService(mainLogger, systemSettingRepo)

	// 从系统设置加载熔断器配置
	ctx := context.Background()
	failureThreshold, coolingDuration := settingService.GetCircuitBreakerConfig(ctx)

	mainLogger.Info("熔断器配置已加载", slog.Int("failure_threshold", failureThreshold), slog.Duration("cooling_duration", coolingDuration))

	// 初始化熔断器管理器（传入 settingService 以支持配置热更新）
	circuitManager := initCircuitManager(db, mainLogger, settingService, failureThreshold, coolingDuration)

	// 初始化调试模式管理器
	debugModeManager := loggerService.NewDebugModeManager(mainLogger, settingService)
	if err := debugModeManager.Initialize(ctx); err != nil {
		mainLogger.Error("初始化调试模式管理器失败", slog.String("error", err.Error()))
	}

	// 注册配置监听器以支持热更新
	configService.RegisterConfigListeners(settingService, []configService.ConfigListener{
		circuitManager,
		debugModeManager, // 注册 DebugModeManager 作为配置监听器
	})

	// 启动熔断器探测调度器
	go func() {
		mainLogger.Info("启动熔断器探测调度器")
		circuitManager.StartProbeScheduler()
	}()

	// 初始化定时任务调度器
	cronScheduler := initCronScheduler(db, mainLogger, settingService, circuitManager)

	// 启动定时任务调度器
	go func() {
		mainLogger.Info("启动定时任务调度器")
		cronScheduler.Start()
	}()

	// 初始化 HTTP 服务器（传入 settingService 以共享同一实例）
	router := setupRouter(cfg, db, mainLogger, circuitManager, debugModeManager, settingService)

	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:        router,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// 启动 HTTP 服务器
	go func() {
		mainLogger.Info("启动 HTTP 服务器",
			slog.Int("port", cfg.Server.Port),
			slog.String("address", fmt.Sprintf("http://0.0.0.0:%d", cfg.Server.Port)),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			mainLogger.Error("HTTP 服务器启动异常", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// 等待中断信号优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	mainLogger.Info("正在关闭服务器...")

	// 停止定时任务调度器
	mainLogger.Info("停止定时任务调度器")
	cronScheduler.Stop()

	// 停止熔断器管理器
	circuitManager.Stop()

	// 优雅关闭 HTTP 服务器
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		mainLogger.Error("服务器强制关闭", slog.String("error", err.Error()))
	}

	mainLogger.Info("服务器已退出")
}

// initLogger 初始化日志系统
func initLogger(cfg *config.Config) (*slog.Logger, error) {
	loggerCfg := &loggerService.LoggerConfig{
		Level:          cfg.Log.Level,
		EnableFile:     cfg.Log.File.Enabled,
		FilePath:       cfg.Log.File.Path,
		MaxSize:        cfg.Log.File.MaxSize,
		MaxBackups:     cfg.Log.File.MaxBackups,
		MaxAge:         cfg.Log.File.MaxAge,
		Compress:       cfg.Log.File.Compress,
		AddSource:      cfg.Log.AddSource,
		EnableDatabase: false, // 审计日志单独处理
	}

	return loggerService.InitLogger(loggerCfg)
}

// initDatabase 初始化数据库
func initDatabase(cfg *config.Config, logger *slog.Logger) (*gorm.DB, error) {
	// 连接数据库
	db, err := config.InitDatabase(cfg)
	if err != nil {
		return nil, err
	}

	logger.Info("正在运行数据库迁移...")

	// 运行数据库迁移
	if err := migration.RunMigrations(db); err != nil {
		return nil, fmt.Errorf("执行数据库迁移失败: %w", err)
	}

	logger.Info("数据库初始化成功")

	return db, nil
}

// initSystemSettings 初始化系统设置
func initSystemSettings(systemSettingRepo *repository.SystemSettingRepository, logger *slog.Logger) error {
	initializer := configService.NewInitializer(logger, systemSettingRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return initializer.Initialize(ctx)
}

// initCircuitManager 初始化熔断器管理器
func initCircuitManager(
	db *gorm.DB,
	logger *slog.Logger,
	settingService *configService.SettingService,
	failureThreshold int,
	coolingDuration time.Duration,
) *circuit.Manager {
	keyRepo := repository.NewKeyRepository(db)
	channelRepo := repository.NewChannelRepository(db)

	probeInterval := 30 * time.Second // 探测间隔 30 秒

	return circuit.NewManager(
		db,
		logger,
		keyRepo,
		channelRepo,
		settingService,
		failureThreshold,
		coolingDuration,
		probeInterval,
	)
}

// setupRouter 设置路由
func setupRouter(
	cfg *config.Config,
	db *gorm.DB,
	logger *slog.Logger,
	circuitManager *circuit.Manager,
	debugModeManager *loggerService.DebugModeManager,
	settingService *configService.SettingService, // 接收共享的 settingService 实例
) *gin.Engine {
	// 设置 Gin 模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// CORS 中间件（必须在其他中间件之前）
	router.Use(middleware.CORS())

	// 使用恢复中间件
	router.Use(gin.Recovery())

	// 健康检查端点(不需要认证)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version})
	})

	// 从系统设置加载代理配置（使用传入的 settingService）
	ctx := context.Background()
	requestTimeout, _, _, maxRetry := settingService.GetProxyConfig(ctx)

	// 创建审计日志记录器（新版）
	requestLogRepo := repository.NewRequestLogRepository(db)
	auditLoggerV2 := loggerService.NewAuditLogger(logger, requestLogRepo, debugModeManager)

	// 注册代理路由
	proxyServiceConfig := &proxyService.ProxyServiceConfig{
		MaxRetries:     maxRetry,
		RetryDelay:     500 * time.Millisecond,
		RequestTimeout: requestTimeout,
	}

	proxy.RegisterRoutes(router, db, logger, circuitManager, auditLoggerV2, debugModeManager, proxyServiceConfig, settingService)

	// 注册 Admin API 路由
	admin.RegisterRoutes(router, db, logger, circuitManager, settingService)

	// 注册静态文件服务 (SPA)
	registerStaticRoutes(router, logger)

	logger.Info("路由注册成功")

	return router
}

// initCronScheduler 初始化定时任务调度器
func initCronScheduler(db *gorm.DB, logger *slog.Logger, settingService *configService.SettingService, circuitManager *circuit.Manager) *schedulerService.CronScheduler {
	// 创建定时任务调度器
	cronScheduler := schedulerService.NewCronScheduler(logger)

	// 添加熔断器清理任务 - 每小时执行一次
	err := cronScheduler.AddJob(
		"circuit-breaker-cleanup",
		schedulerService.EveryHour,
		func(ctx context.Context) error {
			circuitManager.CleanupOrphanBreakers(ctx)
			return nil
		},
	)

	if err != nil {
		logger.Error("添加熔断器清理任务失败", slog.String("error", err.Error()))
	} else {
		logger.Info("熔断器清理任务调度成功", slog.String("schedule", "every hour"))
	}

	return cronScheduler
}

// registerStaticRoutes 注册静态文件服务 (SPA)
func registerStaticRoutes(router *gin.Engine, logger *slog.Logger) {
	// 静态文件目录路径（相对于工作目录）
	staticDir := "./static"

	// 检查静态文件目录是否存在
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		logger.Warn("静态文件目录不存在，仅提供 API 服务", slog.String("static_dir", staticDir))
		// 不注册静态文件路由，只提供 API 服务
		return
	}

	// 注册静态文件路由
	router.Static("/static", staticDir)
	router.Static("/assets", staticDir+"/assets")
	router.StaticFile("/", staticDir+"/index.html")
	router.StaticFile("/favicon.svg", staticDir+"/vite.svg")

	// SPA 路由回退：所有未匹配的路由返回 index.html
	// 这样 Vue Router 可以处理前端路由
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// 如果是 API 请求，返回 404
		if len(path) >= 4 && path[:4] == "/api" || len(path) >= 11 && path[:11] == "/admin/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		// 检查是否是静态资源请求（以 /assets/ 或 /static/ 开头）
		if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/static/") {
			c.Status(http.StatusNotFound)
			return
		}

		// 对于其他所有请求（前端路由），返回 index.html
		c.File(staticDir + "/index.html")
	})

	logger.Info("静态文件路由注册成功", slog.String("static_dir", staticDir))
}
