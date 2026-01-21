package logger

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// AuditLogger 审计日志记录器（支持主表+明细表）
type AuditLogger struct {
	logger           *slog.Logger
	requestLogRepo   *repository.RequestLogRepository
	debugModeManager *DebugModeManager
	logChan          chan *models.RequestLogMain
	stopChan         chan struct{}
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
	debugModeManager *DebugModeManager,
) *AuditLogger {
	al := &AuditLogger{
		logger:           logger,
		requestLogRepo:   requestLogRepo,
		debugModeManager: debugModeManager,
		logChan:          make(chan *models.RequestLogMain, 1000),
		stopChan:         make(chan struct{}),
	}

	// 启动异步写入协程
	go al.processLogs()

	return al
}

// IsDebugModeEnabled 检查调试模式是否启用
func (al *AuditLogger) IsDebugModeEnabled() bool {
	return al.debugModeManager.IsEnabled()
}

// LogAsync 异步记录请求日志
func (al *AuditLogger) LogAsync(mainLog *models.RequestLogMain) {
	if mainLog == nil {
		return
	}
	mainLog.Duration = int(mainLog.EndTime.Sub(mainLog.StartTime).Milliseconds())
	mainLog.RetryCount = len(mainLog.Details) - 1

	for i := range mainLog.Details {
		mainLog.Details[i].Duration = int(mainLog.Details[i].EndTime.Sub(mainLog.Details[i].StartTime).Milliseconds())
		mainLog.Details[i].RequestBodySize = len(mainLog.Details[i].RequestBody)
		mainLog.Details[i].ResponseBodySize = len(mainLog.Details[i].ResponseBody)
		if !al.IsDebugModeEnabled() {
			mainLog.Details[i].RequestBody = ""
			mainLog.Details[i].ResponseBody = ""
		} else {
			mainLog.Details[i].ResponseBody = sanitizeUTF8(mainLog.Details[i].ResponseBody)
		}
	}

	select {
	case al.logChan <- mainLog:
	default:
		al.logger.Warn("日志队列已满，丢弃日志")
	}
}

// processLogs 处理日志写入
func (al *AuditLogger) processLogs() {
	for {
		select {
		case logReq := <-al.logChan:
			al.writeLog(logReq)
		case <-al.stopChan:
			// 处理剩余日志
			for len(al.logChan) > 0 {
				logReq := <-al.logChan
				al.writeLog(logReq)
			}
			return
		}
	}
}

// writeLog 写入日志到数据库
func (al *AuditLogger) writeLog(logReq *models.RequestLogMain) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 写入主日志
	if logReq != nil {
		if err := al.requestLogRepo.CreateMain(ctx, logReq); err != nil {
			al.logger.Error("写入日志失败", slog.String("error", err.Error()))
			return
		}
	}
}

// Stop 停止日志记录器
func (al *AuditLogger) Stop() {
	close(al.stopChan)
}

// sanitizeUTF8 清洗字符串中的非 UTF-8 字符
// 将非法的 UTF-8 字节序列替换为替换字符，避免数据库写入失败
func sanitizeUTF8(data string) string {
	if utf8.ValidString(data) {
		return data // 数据是有效的 UTF-8，直接返回
	}

	// 包含无效的 UTF-8 序列，需要清洗
	var buf strings.Builder
	buf.Grow(len(data))

	for i, r := range data {
		if r == utf8.RuneError {
			// 检查是否真的是非法字符
			_, size := utf8.DecodeRuneInString(data[i:])
			if size == 1 {
				// 非法的 UTF-8 字节，替换为 �
				buf.WriteRune(utf8.RuneError)
			} else {
				buf.WriteRune(r)
			}
		} else {
			buf.WriteRune(r)
		}
	}

	return buf.String()
}
