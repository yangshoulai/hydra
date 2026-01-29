package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 系统配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	MaxHeaderBytes int           `mapstructure:"max_header_bytes"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type            string        `mapstructure:"type"`
	SQLitePath      string        `mapstructure:"sqlite_path"`
	PostgresDSN     string        `mapstructure:"postgres_dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level         string     `mapstructure:"level"`
	RetentionDays int        `mapstructure:"retention_days"`
	DebugEnabled  bool       `mapstructure:"debug_enabled"`
	AddSource     bool       `mapstructure:"add_source"`
	File          FileConfig `mapstructure:"file"`
}

// FileConfig 文件日志配置
type FileConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Path       string `mapstructure:"path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 默认配置文件搜索路径
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// 环境变量覆盖(支持 HYDRA_ 前缀)
	v.SetEnvPrefix("HYDRA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 设置默认值
	setDefaults(v)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// 配置文件不存在时使用默认值
	}

	// 解析配置
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 验证配置
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认配置值
func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.max_header_bytes", 1048576)

	// Database
	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.sqlite_path", "./hydra.db")
	v.SetDefault("database.max_open_conns", 10)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "300s")

	// Log
	v.SetDefault("log.level", "info")
	v.SetDefault("log.retention_days", 30)
	v.SetDefault("log.debug_enabled", false)
	v.SetDefault("log.add_source", false)
	v.SetDefault("log.file.enabled", true)
	v.SetDefault("log.file.path", "./logs/hydra.log")
	v.SetDefault("log.file.max_size", 100)   // 100MB，按大小轮转
	v.SetDefault("log.file.max_backups", 10) // 保留10个备份
	v.SetDefault("log.file.max_age", 30)     // 保留30天
	v.SetDefault("log.file.compress", true)
}

// validate 验证配置
func validate(cfg *Config) error {
	// 验证服务器端口
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	// 验证数据库类型
	if cfg.Database.Type != "sqlite" && cfg.Database.Type != "postgres" {
		return fmt.Errorf("invalid database type: %s (must be 'sqlite' or 'postgres')", cfg.Database.Type)
	}

	// 验证 SQLite 路径
	if cfg.Database.Type == "sqlite" && cfg.Database.SQLitePath == "" {
		return fmt.Errorf("sqlite_path is required when database type is 'sqlite'")
	}

	// 验证 PostgreSQL DSN
	if cfg.Database.Type == "postgres" && cfg.Database.PostgresDSN == "" {
		return fmt.Errorf("postgres_dsn is required when database type is 'postgres'")
	}

	// 验证日志级别
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.Log.Level] {
		return fmt.Errorf("invalid log level: %s (must be 'debug', 'info', 'warn', or 'error')", cfg.Log.Level)
	}

	return nil
}
