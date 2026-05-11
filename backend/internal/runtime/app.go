package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/api/admin"
	"github.com/yangshoulai/hydra/internal/api/proxy"
	"github.com/yangshoulai/hydra/internal/app"
	"github.com/yangshoulai/hydra/internal/middleware"
	"github.com/yangshoulai/hydra/internal/migration"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	adminService "github.com/yangshoulai/hydra/internal/service/admin"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	loggerService "github.com/yangshoulai/hydra/internal/service/logger"
	metricsService "github.com/yangshoulai/hydra/internal/service/metrics"
	modelsyncService "github.com/yangshoulai/hydra/internal/service/modelsync"
	notificationService "github.com/yangshoulai/hydra/internal/service/notification"
	proxyService "github.com/yangshoulai/hydra/internal/service/proxy"
	schedulerService "github.com/yangshoulai/hydra/internal/service/scheduler"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

const (
	dbFileName  = "hydra.db"
	logsSubDir  = "logs"
	logFileName = "hydra.log"
)

var buildInfo = struct {
	Version   string
	BuildTime string
}{
	Version:   "dev",
	BuildTime: "unknown",
}

func SetBuildInfo(version, buildTime string) {
	if strings.TrimSpace(version) != "" {
		buildInfo.Version = version
	}
	if strings.TrimSpace(buildTime) != "" {
		buildInfo.BuildTime = buildTime
	}
}

// App 代表一个可运行的应用实例
// 一个 App 包含完整的一套依赖（DB、服务、HTTP Server）。
type App struct {
	ID      int64
	DataDir string

	DB         *gorm.DB
	Logger     *slog.Logger
	Components *app.Components
	Server     *http.Server

	startOnce sync.Once
	stopOnce  sync.Once
}

// NewApp 创建应用实例（不会阻塞启动监听）
func NewApp(id int64, dataDir string, bootstrapLogger *slog.Logger, restartListener configService.ConfigListener) (*App, error) {
	if dataDir == "" {
		return nil, errors.New("data dir is required")
	}
	if err := ensureDataDir(dataDir); err != nil {
		return nil, err
	}

	sqlitePath := filepath.Join(dataDir, dbFileName)
	db, err := initSQLite(sqlitePath)
	if err != nil {
		return nil, err
	}

	if err := migration.RunMigrations(db); err != nil {
		return nil, fmt.Errorf("执行数据库迁移失败: %w", err)
	}

	repos := &app.Repositories{
		AdminUserRepo:     repository.NewAdminUserRepository(db),
		AccessTokenRepo:   repository.NewAccessTokenRepository(db),
		ChannelRepo:       repository.NewChannelRepository(db),
		ChannelKeyRepo:    repository.NewChannelKeyRepository(db),
		ModelConfigRepo:   repository.NewChannelModelConfigRepository(db),
		SystemSettingRepo: repository.NewSystemSettingRepository(db),
		ModelRepo:         repository.NewModelRepository(db),
		ProviderRepo:      repository.NewProviderRepository(db),
		RequestLogRepo:    repository.NewRequestLogRepository(db),
	}

	// 首次运行写入默认系统设置
	if err := initSystemSettings(repos.SystemSettingRepo, bootstrapLogger); err != nil {
		_ = closeDatabase(db)
		return nil, fmt.Errorf("初始化系统设置失败: %w", err)
	}

	// 先用 bootstrap logger 构建配置服务，读取 DB 中日志配置
	settingService := configService.NewSettingService(bootstrapLogger, repos.SystemSettingRepo)
	runtimeLogger, err := initRuntimeLogger(settingService, dataDir)
	if err != nil {
		_ = closeDatabase(db)
		return nil, fmt.Errorf("初始化日志系统失败: %w", err)
	}

	// 使用运行时 logger 重新创建配置服务（日志输出更加一致）
	settingService = configService.NewSettingService(runtimeLogger, repos.SystemSettingRepo)
	settingService.RegisterListener(restartListener)

	ctx := context.Background()
	runtimeMetrics := metricsService.NewRuntimeMetrics()
	notificationSvc := notificationService.NewService(runtimeLogger, settingService)

	failureThreshold, coolingDuration := settingService.GetCircuitBreakerConfig(ctx)
	circuitManager := circuit.NewCircuitManager(
		db,
		runtimeLogger,
		repos.ChannelKeyRepo,
		repos.ModelConfigRepo,
		settingService,
		notificationSvc,
		failureThreshold,
		coolingDuration,
	)

	cronScheduler := schedulerService.NewCronScheduler(runtimeLogger)
	if err := cronScheduler.AddJob("circuit-breaker-cleanup", schedulerService.EveryHour, func(ctx context.Context) error {
		circuitManager.CleanupOrphanBreakers(ctx)
		return nil
	}); err != nil {
		runtimeLogger.Warn("添加熔断器清理任务失败", slog.String("error", err.Error()))
	}
	// 每天 03:17 清理过期请求日志
	if err := cronScheduler.AddJob("request-log-cleanup", "0 17 3 * * *", func(ctx context.Context) error {
		days := settingService.GetInt(ctx, models.SettingLogRetentionDays, 30)
		if days < 1 {
			days = 1
		}
		before := time.Now().AddDate(0, 0, -days)
		deleted, err := repos.RequestLogRepo.DeleteOlderThan(ctx, before)
		if err != nil {
			return err
		}
		if deleted > 0 {
			runtimeLogger.Info("已清理过期请求日志",
				slog.Int64("deleted", deleted),
				slog.Int("retention_days", days),
			)
		}
		return nil
	}); err != nil {
		runtimeLogger.Warn("添加请求日志清理任务失败", slog.String("error", err.Error()))
	}

	requestTimeout, keepaliveInterval, networkProxyURL, maxRetry, loadBalanceStrategy := settingService.GetProxyConfig(ctx)
	nonStreamKeepalive := settingService.GetNonStreamKeepaliveConfig(ctx)
	requestLogRecorder := proxyService.NewRequestLogRecorder(runtimeLogger, repos.RequestLogRepo, 1024, 2)
	proxySvc := proxyService.NewProxyService(
		runtimeLogger,
		repos.ChannelRepo,
		repos.ChannelKeyRepo,
		repos.ModelConfigRepo,
		repos.AccessTokenRepo,
		circuitManager,
		&proxyService.ProxyServiceConfig{
			MaxRetries:                   maxRetry,
			RetryDelay:                   500 * time.Millisecond,
			RequestTimeout:               requestTimeout,
			StreamKeepaliveInterval:      keepaliveInterval,
			NonStreamKeepaliveEnabled:    nonStreamKeepalive.Enabled,
			NonStreamKeepaliveFirstDelay: nonStreamKeepalive.FirstDelay,
			NonStreamKeepaliveInterval:   nonStreamKeepalive.Interval,
			NetworkProxy:                 networkProxyURL,
			LoadBalanceStrategy:          loadBalanceStrategy,
		},
		settingService,
		runtimeMetrics,
		requestLogRecorder,
	)

	syncService := modelsyncService.NewSyncService(runtimeLogger, repos.ChannelRepo, repos.ModelConfigRepo, repos.ChannelKeyRepo, settingService, proxySvc.GetHTTPClient())
	jwtService, err := adminService.NewJWTService(settingService.GetJWTSecret(ctx))
	if err != nil {
		_ = closeDatabase(db)
		return nil, fmt.Errorf("初始化 JWT 服务失败: %w", err)
	}
	authService := adminService.NewAuthService(runtimeLogger, repos.AdminUserRepo, jwtService)
	probeHandler := circuit.NewProbeHandler(runtimeLogger, settingService, proxySvc.GetHTTPClient())
	healthCheckService := adminService.NewHealthCheckService(runtimeLogger, repos.ChannelRepo, probeHandler)
	dashboardService := adminService.NewDashboardService(
		runtimeLogger,
		repos.ChannelRepo,
		repos.ChannelKeyRepo,
		repos.ModelConfigRepo,
		repos.ModelRepo,
		repos.RequestLogRepo,
		circuitManager,
		runtimeMetrics,
	)
	modelService := adminService.NewModelService(repos.ModelRepo, repos.ModelConfigRepo, runtimeLogger)
	providerService := adminService.NewProviderService(repos.ProviderRepo, runtimeLogger)

	components := &app.Components{
		DB:     db,
		Logger: runtimeLogger,
		Repos:  repos,
		Services: &app.Services{
			SettingService:      settingService,
			NotificationService: notificationSvc,
			CircuitManager:      circuitManager,
			CronScheduler:       cronScheduler,
			ProxyService:        proxySvc,
			AuthService:         authService,
			JWTService:          jwtService,
			HealthCheckService:  healthCheckService,
			SyncService:         syncService,
			DashboardService:    dashboardService,
			ModelService:        modelService,
			ProviderService:     providerService,
			CircuitProbeHandler: probeHandler,
		},
	}

	configService.RegisterConfigListeners(settingService, []configService.ConfigListener{
		circuitManager,
		proxySvc,
	})

	router := setupRouter(runtimeLogger, components)
	port, readTimeout, writeTimeout, maxHeaderBytes := settingService.GetServerConfig(ctx)

	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        router,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxHeaderBytes: maxHeaderBytes,
	}

	return &App{
		ID:         id,
		DataDir:    dataDir,
		DB:         db,
		Logger:     runtimeLogger,
		Components: components,
		Server:     srv,
	}, nil
}

// Start 启动应用实例
func (a *App) Start() error {
	a.startOnce.Do(func() {
		go a.Components.Services.CronScheduler.Start()

		go func() {
			a.Logger.Info("App 启动 HTTP 服务", slog.Int64("app_id", a.ID), slog.String("addr", a.Server.Addr))
			if err := a.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				a.Logger.Error("HTTP 服务异常退出", slog.Int64("app_id", a.ID), slog.String("error", err.Error()))
			}
		}()
	})

	return nil
}

// Stop 优雅停止应用实例
func (a *App) Stop(ctx context.Context) error {
	var stopErr error
	a.stopOnce.Do(func() {
		a.Logger.Info("开始停止 App", slog.Int64("app_id", a.ID))

		a.Components.Services.CronScheduler.Stop()

		if err := a.Server.Shutdown(ctx); err != nil {
			stopErr = err
		}

		if a.Components.Services.ProxyService != nil {
			a.Components.Services.ProxyService.Close()
		}
		a.Components.Services.CircuitManager.Stop()

		if err := closeDatabase(a.DB); err != nil && stopErr == nil {
			stopErr = err
		}

		a.Logger.Info("App 已停止", slog.Int64("app_id", a.ID))
	})

	return stopErr
}

func setupRouter(logger *slog.Logger, components *app.Components) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())

	startedAt := time.Now()
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"uptime_sec": int64(time.Since(startedAt).Seconds()),
		})
	}
	router.GET("/health", healthHandler)
	router.GET("/healthz", healthHandler)
	router.GET("/ready", func(c *gin.Context) {
		statusCode := http.StatusOK
		dbStatus := "ok"
		openBreakers := 0
		if components == nil || components.DB == nil {
			statusCode = http.StatusServiceUnavailable
			dbStatus = "unavailable"
		} else if sqlDB, err := components.DB.DB(); err != nil {
			statusCode = http.StatusServiceUnavailable
			dbStatus = err.Error()
		} else {
			pingCtx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
			defer cancel()
			if err := sqlDB.PingContext(pingCtx); err != nil {
				statusCode = http.StatusServiceUnavailable
				dbStatus = err.Error()
			}
		}
		if components != nil && components.Services != nil && components.Services.CircuitManager != nil {
			openBreakers = len(components.Services.CircuitManager.SnapshotBreakers())
		}

		c.JSON(statusCode, gin.H{
			"status":     statusFromCode(statusCode),
			"uptime_sec": int64(time.Since(startedAt).Seconds()),
			"version":    buildInfo.Version,
			"build_time": buildInfo.BuildTime,
			"database":   dbStatus,
			"circuit_breakers": gin.H{
				"open_count": openBreakers,
			},
		})
	})

	proxy.RegisterRoutes(router, components)
	admin.RegisterRoutes(router, components)
	registerStaticRoutes(router, logger)

	logger.Info("路由注册成功")
	return router
}

func statusFromCode(statusCode int) string {
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return "ok"
	}
	return "unavailable"
}

func registerStaticRoutes(router *gin.Engine, logger *slog.Logger) {
	staticDir := "./static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		logger.Warn("静态文件目录不存在，仅提供 API 服务", slog.String("static_dir", staticDir))
		return
	}

	router.Static("/static", staticDir)
	router.Static("/assets", staticDir+"/assets")
	router.StaticFile("/", staticDir+"/index.html")
	router.StaticFile("/favicon.svg", staticDir+"/favicon.svg")

	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/admin/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/static/") {
			c.Status(http.StatusNotFound)
			return
		}

		c.File(staticDir + "/index.html")
	})
}

// ensureDataDir 创建数据目录与日志子目录
func ensureDataDir(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, logsSubDir), 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	return nil
}

func initSQLite(sqlitePath string) (*gorm.DB, error) {
	dsn := sqlitePath + "?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("连接 SQLite 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库实例失败: %w", err)
	}

	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		return nil, fmt.Errorf("启用 SQLite WAL 模式失败: %w", err)
	}
	if err := db.Exec("PRAGMA busy_timeout=5000").Error; err != nil {
		return nil, fmt.Errorf("设置 SQLite busy_timeout 失败: %w", err)
	}

	// WAL 模式允许读写并发。连接数保持保守，避免 SQLite 写锁竞争过度放大。
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(2 * time.Minute)

	return db, nil
}

func closeDatabase(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func initSystemSettings(systemSettingRepo *repository.SystemSettingRepository, logger *slog.Logger) error {
	initializer := configService.NewInitializer(logger, systemSettingRepo)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return initializer.Initialize(ctx)
}

func initRuntimeLogger(settingService *configService.SettingService, dataDir string) (*slog.Logger, error) {
	ctx := context.Background()
	cfg := &loggerService.LoggerConfig{
		Level:      settingService.GetEffectiveLogLevel(ctx),
		AddSource:  settingService.GetBool(ctx, models.SettingLogAddSource, false),
		EnableFile: settingService.GetBool(ctx, models.SettingLogFileEnabled, true),
		FilePath:   filepath.Join(dataDir, logsSubDir, logFileName),
		MaxSize:    settingService.GetInt(ctx, models.SettingLogFileMaxSize, 100),
		MaxBackups: settingService.GetInt(ctx, models.SettingLogFileMaxBackups, 10),
		MaxAge:     settingService.GetInt(ctx, models.SettingLogFileMaxAge, 30),
		Compress:   settingService.GetBool(ctx, models.SettingLogFileCompress, true),
	}
	return loggerService.InitLogger(cfg)
}
