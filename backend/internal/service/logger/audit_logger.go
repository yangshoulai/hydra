package logger

import (
	"context"
	"log/slog"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// AuditLogger 审计日志记录器(写入数据库)
type AuditLogger struct {
	logger           *slog.Logger
	requestLogRepo   *repository.RequestLogRepository
	debugModeManager *DebugModeManager
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(
	logger *slog.Logger,
	requestLogRepo *repository.RequestLogRepository,
	debugModeManager *DebugModeManager,
) *AuditLogger {
	return &AuditLogger{
		logger:           logger,
		requestLogRepo:   requestLogRepo,
		debugModeManager: debugModeManager,
	}
}

// LogRequest 记录请求审计日志
func (a *AuditLogger) LogRequest(ctx context.Context, log *models.RequestLog) error {
	if log == nil {
		return nil
	}

	// 设置创建时间
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	if err := a.requestLogRepo.Create(ctx, log); err != nil {
		a.logger.Error("failed to write audit log to database",
			slog.String("trace_id", log.TraceID),
			slog.String("error", err.Error()),
		)
		return err
	}

	a.logger.Debug("audit log written to database",
		slog.String("trace_id", log.TraceID),
		slog.Int("status_code", log.StatusCode),
		slog.Int("response_time", log.ResponseTime),
	)

	return nil
}

// LogRequestAsync 异步记录请求审计日志(不阻塞主流程)
func (a *AuditLogger) LogRequestAsync(log *models.RequestLog) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := a.LogRequest(ctx, log); err != nil {
			a.logger.Warn("async audit log write failed",
				slog.String("trace_id", log.TraceID),
				slog.String("error", err.Error()),
			)
		}
	}()
}

// IsDebugModeEnabled 检查调试模式是否启用
func (a *AuditLogger) IsDebugModeEnabled() bool {
	if a.debugModeManager == nil {
		return false
	}
	return a.debugModeManager.IsEnabled()
}

// NewRequestLogBuilder 创建请求日志构建器
func NewRequestLogBuilder() *RequestLogBuilder {
	return &RequestLogBuilder{
		log: &models.RequestLog{},
	}
}

// RequestLogBuilder 请求日志构建器
type RequestLogBuilder struct {
	log *models.RequestLog
}

func (b *RequestLogBuilder) TraceID(traceID string) *RequestLogBuilder {
	b.log.TraceID = traceID
	return b
}

func (b *RequestLogBuilder) AccessToken(token string) *RequestLogBuilder {
	b.log.AccessToken = token
	return b
}

func (b *RequestLogBuilder) RequestPath(path string) *RequestLogBuilder {
	b.log.RequestPath = path
	return b
}

func (b *RequestLogBuilder) RequestMethod(method string) *RequestLogBuilder {
	b.log.RequestMethod = method
	return b
}

func (b *RequestLogBuilder) RequestedModel(model string) *RequestLogBuilder {
	b.log.RequestedModel = model
	return b
}

func (b *RequestLogBuilder) UnifiedModel(model string) *RequestLogBuilder {
	b.log.UnifiedModel = model
	return b
}

func (b *RequestLogBuilder) UpstreamModel(model string) *RequestLogBuilder {
	b.log.UpstreamModel = model
	return b
}

func (b *RequestLogBuilder) ChannelID(id uint) *RequestLogBuilder {
	b.log.ChannelID = &id
	return b
}

func (b *RequestLogBuilder) ChannelName(name string) *RequestLogBuilder {
	b.log.ChannelName = name
	return b
}

func (b *RequestLogBuilder) KeyID(id uint) *RequestLogBuilder {
	b.log.KeyID = &id
	return b
}

func (b *RequestLogBuilder) StatusCode(code int) *RequestLogBuilder {
	b.log.StatusCode = code
	return b
}

func (b *RequestLogBuilder) ResponseTime(ms int) *RequestLogBuilder {
	b.log.ResponseTime = ms
	return b
}

func (b *RequestLogBuilder) IsSuccess(success bool) *RequestLogBuilder {
	b.log.IsSuccess = success
	return b
}

func (b *RequestLogBuilder) ErrorMessage(msg string) *RequestLogBuilder {
	b.log.ErrorMessage = msg
	return b
}

func (b *RequestLogBuilder) RetryCount(count int) *RequestLogBuilder {
	b.log.RetryCount = count
	return b
}

func (b *RequestLogBuilder) IsStream(stream bool) *RequestLogBuilder {
	b.log.IsStream = stream
	return b
}

func (b *RequestLogBuilder) StreamChunks(chunks int) *RequestLogBuilder {
	b.log.StreamChunks = chunks
	return b
}

func (b *RequestLogBuilder) ClientIP(ip string) *RequestLogBuilder {
	b.log.ClientIP = ip
	return b
}

func (b *RequestLogBuilder) UserAgent(ua string) *RequestLogBuilder {
	b.log.UserAgent = ua
	return b
}

func (b *RequestLogBuilder) RequestBody(body string) *RequestLogBuilder {
	b.log.RequestBody = body
	return b
}

func (b *RequestLogBuilder) ResponseBody(body string) *RequestLogBuilder {
	b.log.ResponseBody = body
	return b
}

func (b *RequestLogBuilder) RequestHeaders(headers string) *RequestLogBuilder {
	b.log.RequestHeaders = headers
	return b
}

func (b *RequestLogBuilder) ResponseHeaders(headers string) *RequestLogBuilder {
	b.log.ResponseHeaders = headers
	return b
}

func (b *RequestLogBuilder) Build() *models.RequestLog {
	return b.log
}
