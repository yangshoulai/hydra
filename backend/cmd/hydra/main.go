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
	"github.com/yangshoulai/hydra/internal/app"
	"github.com/yangshoulai/hydra/internal/config"
	_ "github.com/yangshoulai/hydra/internal/endpoint" // 导入端点包以触发 init 函数
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/migration"
	"github.com/yangshoulai/hydra/internal/repository"
	adminService "github.com/yangshoulai/hydra/internal/service/admin"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	loggerService "github.com/yangshoulai/hydra/internal/service/logger"
	modelsyncService "github.com/yangshoulai/hydra/internal/service/modelsync"
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
	defer func(db *gorm.DB) {
		_ = config.CloseDatabase(db)
	}(db)

	// 创建仓储（全局唯一实例）
	repos := &app.Repositories{
		AdminUser:     repository.NewAdminUserRepository(db),
		AccessToken:   repository.NewAccessTokenRepository(db),
		Channel:       repository.NewChannelRepository(db),
		Key:           repository.NewKeyRepository(db),
		ModelConfig:   repository.NewChannelModelConfigRepository(db),
		RequestLog:    repository.NewRequestLogRepository(db),
		SystemSetting: repository.NewSystemSettingRepository(db),
		Model:         repository.NewModelRepository(db),
		Provider:      repository.NewProviderRepository(db),
	}

	// 初始化系统设置
	if err := initSystemSettings(repos.SystemSetting, mainLogger); err != nil {
		mainLogger.Error("初始化系统设置失败", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// 创建系统设置服务
	settingService := configService.NewSettingService(mainLogger, repos.SystemSetting)

	// 从系统设置加载熔断器配置
	ctx := context.Background()

	// 初始化熔断器管理器（传入 settingService 以支持配置热更新）
	circuitManager := initCircuitManager(ctx, db, mainLogger, settingService, repos)

	// 初始化调试模式管理器
	debugModeManager := initDebugModeManager(ctx, mainLogger, settingService)

	// 创建日志清理服务
	logCleanupService := loggerService.NewLogCleanupService(mainLogger, settingService, repos.RequestLog)

	// 初始化定时任务调度器
	cronScheduler := initCronScheduler(mainLogger, circuitManager, logCleanupService)

	// 创建模型同步服务
	syncService := modelsyncService.NewSyncService(mainLogger, repos.Channel, repos.ModelConfig, repos.Key, circuitManager)

	// 初始化渠道模型同步调度器
	channelSyncScheduler := initChannelModelSyncScheduler(ctx, cronScheduler, syncService, settingService, repos, mainLogger)

	// 创建审计日志记录器
	auditLogger := loggerService.NewAuditLogger(mainLogger, repos.RequestLog, debugModeManager)

	// 创建代理服务
	proxySvc := initProxyService(ctx, settingService, repos, mainLogger, circuitManager, auditLogger)

	// 创建管理后台相关服务
	jwtService := adminService.NewJWTService()
	authService := adminService.NewAuthService(db, mainLogger, repos.AdminUser, repos.AccessToken, jwtService)
	probeHandler := circuit.NewProbeHandler(circuitManager, mainLogger)
	healthCheckService := adminService.NewHealthCheckService(mainLogger, repos.Key, repos.Channel, probeHandler)
	dashboardService := adminService.NewDashboardService(mainLogger, repos.RequestLog, repos.Channel, repos.Key, circuitManager)
	modelService := adminService.NewModelService(repos.Model, repos.ModelConfig, mainLogger)
	providerService := adminService.NewProviderService(repos.Provider, mainLogger)

	components := &app.Components{
		DB:     db,
		Logger: mainLogger,
		Repos:  repos,
		Services: &app.Services{
			Setting:               settingService,
			CircuitManager:        circuitManager,
			DebugModeManager:      debugModeManager,
			CronScheduler:         cronScheduler,
			ChannelModelScheduler: channelSyncScheduler,
			AuditLogger:           auditLogger,
			LogCleanupService:     logCleanupService,
			ProxyService:          proxySvc,
			AuthService:           authService,
			JWTService:            jwtService,
			HealthCheckService:    healthCheckService,
			SyncService:           syncService,
			DashboardService:      dashboardService,
			ModelService:          modelService,
			ProviderService:       providerService,
			CircuitProbeHandler:   probeHandler,
		},
	}

	// 注册配置监听器以支持热更新
	configService.RegisterConfigListeners(settingService, []configService.ConfigListener{
		circuitManager,
		debugModeManager,
		channelSyncScheduler,
		proxySvc,
	})

	// 启动定时任务调度器
	go func() {
		mainLogger.Info("启动定时任务调度器")
		cronScheduler.Start()
	}()

	// 初始化 HTTP 服务器
	router := setupRouter(cfg, components)

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
func initCircuitManager(ctx context.Context,
	db *gorm.DB,
	logger *slog.Logger,
	settingService *configService.SettingService,
	repos *app.Repositories,
) *circuit.Manager {
	failureThreshold, coolingDuration := settingService.GetCircuitBreakerConfig(ctx)

	return circuit.NewManager(
		db,
		logger,
		repos.Key,
		repos.Channel,
		repos.ModelConfig,
		settingService,
		failureThreshold,
		coolingDuration,
	)
}

// setupRouter 设置路由
func setupRouter(
	cfg *config.Config,
	components *app.Components,
) *gin.Engine {
	logger := components.Logger

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

	// 注册代理路由
	proxy.RegisterRoutes(router, components)

	// 注册 Admin API 路由
	admin.RegisterRoutes(router, components)

	// 注册静态文件服务 (SPA)
	registerStaticRoutes(router, logger)

	logger.Info("路由注册成功")

	return router
}

// initCronScheduler 初始化定时任务调度器
func initCronScheduler(logger *slog.Logger, circuitManager *circuit.Manager, logCleanupService *loggerService.LogCleanupService) *schedulerService.CronScheduler {
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

	// 添加日志清理任务 - 每天凌晨1点执行
	err = cronScheduler.AddJob(
		"log-cleanup",
		schedulerService.EveryDayAt1AM,
		logCleanupService.Run,
	)
	if err != nil {
		logger.Error("添加日志清理任务失败", slog.String("error", err.Error()))
	} else {
		logger.Info("日志清理任务调度成功", slog.String("schedule", "every day at 1am"))
	}

	return cronScheduler
}

func initDebugModeManager(ctx context.Context, mainLogger *slog.Logger, settingService *configService.SettingService) *loggerService.DebugModeManager {
	debugModeManager := loggerService.NewDebugModeManager(mainLogger, settingService)
	if err := debugModeManager.Initialize(ctx); err != nil {
		mainLogger.Error("初始化调试模式管理器失败", slog.String("error", err.Error()))
	}
	return debugModeManager
}

func initProxyService(ctx context.Context, settingService *configService.SettingService, repos *app.Repositories,
	mainLogger *slog.Logger,
	circuitManager *circuit.Manager,
	auditLogger *loggerService.AuditLogger,
) *proxyService.ProxyService {
	requestTimeout, _, maxRetry := settingService.GetProxyConfig(ctx)
	proxyServiceConfig := &proxyService.ProxyServiceConfig{
		MaxRetries:     maxRetry,
		RetryDelay:     500 * time.Millisecond,
		RequestTimeout: requestTimeout,
	}
	return proxyService.NewProxyService(
		mainLogger,
		repos.Channel,
		repos.RequestLog,
		repos.Key,
		repos.ModelConfig,
		repos.AccessToken,
		circuitManager,
		auditLogger,
		proxyServiceConfig,
		settingService,
	)
}

func initChannelModelSyncScheduler(ctx context.Context, cronScheduler *schedulerService.CronScheduler, syncService *modelsyncService.SyncService, settingService *configService.SettingService, repos *app.Repositories, mainLogger *slog.Logger) *modelsyncService.ChannelModelSyncScheduler {
	channelSyncScheduler := modelsyncService.NewChannelModelSyncScheduler(
		mainLogger,
		cronScheduler,
		settingService,
		repos.Channel,
		syncService,
	)
	channelSyncScheduler.Initialize(ctx)

	return channelSyncScheduler
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
