package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/yangshoulai/hydra/internal/models"
)

// BodyLogger 完整请求/响应Body日志记录器
type BodyLogger struct {
	logger        *slog.Logger
	debugMode     *DebugModeManager
	logDir        string
	maxBodySize   int64 // 最大Body大小（字节）
}

// NewBodyLogger 创建Body日志记录器
func NewBodyLogger(
	logger *slog.Logger,
	debugMode *DebugModeManager,
	logDir string,
	maxBodySize int64,
) *BodyLogger {
	if maxBodySize <= 0 {
		maxBodySize = 10 * 1024 * 1024 // 默认10MB
	}

	return &BodyLogger{
		logger:      logger,
		debugMode:   debugMode,
		logDir:      logDir,
		maxBodySize: maxBodySize,
	}
}

// LogRequest 记录请求Body
func (bl *BodyLogger) LogRequest(ctx context.Context, traceID string, body []byte) error {
	// 如果未启用调试模式，跳过记录
	if !bl.debugMode.IsEnabled() {
		return nil
	}

	// 检查Body大小
	if int64(len(body)) > bl.maxBodySize {
		bl.logger.Warn("request body too large, truncating",
			slog.String("trace_id", traceID),
			slog.Int64("body_size", int64(len(body))),
			slog.Int64("max_size", bl.maxBodySize),
		)
		body = body[:bl.maxBodySize]
	}

	return bl.writeBodyFile(ctx, traceID, "request", body)
}

// LogResponse 记录响应Body
func (bl *BodyLogger) LogResponse(ctx context.Context, traceID string, body []byte) error {
	// 如果未启用调试模式，跳过记录
	if !bl.debugMode.IsEnabled() {
		return nil
	}

	// 检查Body大小
	if int64(len(body)) > bl.maxBodySize {
		bl.logger.Warn("response body too large, truncating",
			slog.String("trace_id", traceID),
			slog.Int64("body_size", int64(len(body))),
			slog.Int64("max_size", bl.maxBodySize),
		)
		body = body[:bl.maxBodySize]
	}

	return bl.writeBodyFile(ctx, traceID, "response", body)
}

// LogRequestLog 记录完整的请求日志（包含Body）
func (bl *BodyLogger) LogRequestLog(ctx context.Context, log *models.RequestLog, reqBody, respBody []byte) error {
	// 如果未启用调试模式，跳过记录
	if !bl.debugMode.IsEnabled() {
		return nil
	}

	debugLog := &DebugLog{
		TraceID:       log.TraceID,
		CreatedAt:     log.CreatedAt,
		RequestLog:    log,
		RequestBody:   reqBody,
		ResponseBody:  respBody,
	}

	return bl.writeDebugLogFile(ctx, debugLog)
}

// DebugLog 调试日志结构
type DebugLog struct {
	TraceID      string              `json:"trace_id"`
	CreatedAt    time.Time           `json:"created_at"`
	RequestLog   *models.RequestLog  `json:"request_log"`
	RequestBody  []byte              `json:"request_body,omitempty"`
	ResponseBody []byte              `json:"response_body,omitempty"`
}

// writeBodyFile 写入Body文件
func (bl *BodyLogger) writeBodyFile(ctx context.Context, traceID, bodyType string, body []byte) error {
	// 确保日志目录存在
	if err := os.MkdirAll(bl.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// 创建文件名：traceid_type.json
	filename := fmt.Sprintf("%s_%s.json", traceID, bodyType)
	filepath := filepath.Join(bl.logDir, filename)

	// 尝试解析JSON格式化输出
	var formattedBody []byte
	if json.Valid(body) {
		var obj interface{}
		if err := json.Unmarshal(body, &obj); err == nil {
			formattedBody, _ = json.MarshalIndent(obj, "", "  ")
		} else {
			formattedBody = body
		}
	} else {
		formattedBody = body
	}

	// 写入文件
	if err := os.WriteFile(filepath, formattedBody, 0644); err != nil {
		return fmt.Errorf("failed to write body file: %w", err)
	}

	bl.logger.Debug("body file written",
		slog.String("trace_id", traceID),
		slog.String("type", bodyType),
		slog.String("file", filepath),
		slog.Int("size", len(formattedBody)),
	)

	return nil
}

// writeDebugLogFile 写入完整调试日志文件
func (bl *BodyLogger) writeDebugLogFile(ctx context.Context, debugLog *DebugLog) error {
	// 创建按日期分组的子目录
	dateStr := debugLog.CreatedAt.Format("2006-01-02")
	dateDir := filepath.Join(bl.logDir, dateStr)

	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return fmt.Errorf("failed to create date directory: %w", err)
	}

	// 创建文件名：traceid_debug.json
	filename := fmt.Sprintf("%s_debug.json", debugLog.TraceID)
	filepath := filepath.Join(dateDir, filename)

	// 序列化为JSON
	data, err := json.MarshalIndent(debugLog, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal debug log: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write debug log file: %w", err)
	}

	bl.logger.Debug("debug log file written",
		slog.String("trace_id", debugLog.TraceID),
		slog.String("file", filepath),
	)

	return nil
}

// CleanupOldDebugLogs 清理旧的调试日志文件
func (bl *BodyLogger) CleanupOldDebugLogs(ctx context.Context, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = 7 // 默认保留7天
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	bl.logger.Info("cleaning up old debug logs",
		slog.Int("retention_days", retentionDays),
		slog.Time("cutoff_time", cutoffTime),
	)

	// 遍历日志目录中的日期子目录
	entries, err := os.ReadDir(bl.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 目录不存在，无需清理
		}
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	cleanupCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 解析日期目录名
		dateDir := filepath.Join(bl.logDir, entry.Name())
		fileInfo, err := entry.Info()
		if err != nil {
			bl.logger.Warn("failed to get file info",
				slog.String("dir", entry.Name()),
				slog.String("error", err.Error()),
			)
			continue
		}

		// 如果目录修改时间早于截止时间，删除整个目录
		if fileInfo.ModTime().Before(cutoffTime) {
			if err := os.RemoveAll(dateDir); err != nil {
				bl.logger.Warn("failed to remove old debug log directory",
					slog.String("dir", entry.Name()),
					slog.String("error", err.Error()),
				)
			} else {
				cleanupCount++
				bl.logger.Debug("removed old debug log directory",
					slog.String("dir", entry.Name()),
				)
			}
		}
	}

	bl.logger.Info("debug log cleanup completed",
		slog.Int("directories_removed", cleanupCount),
	)

	return nil
}

// GenerateTempTraceID 生成临时TraceID（用于早期日志记录）
func GenerateTempTraceID() string {
	return uuid.New().String()
}
