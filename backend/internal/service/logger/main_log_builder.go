package logger

import (
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

// MainLogBuilder 主日志构建器
type MainLogBuilder struct {
	main *models.RequestLogMain
}

// NewMainLogBuilder 创建主日志构建器
func NewMainLogBuilder() *MainLogBuilder {
	return &MainLogBuilder{
		main: &models.RequestLogMain{},
	}
}

func (b *MainLogBuilder) TraceID(traceID string) *MainLogBuilder {
	b.main.TraceID = traceID
	return b
}

func (b *MainLogBuilder) EndpointType(endpointType string) *MainLogBuilder {
	b.main.EndpointType = endpointType
	return b
}

func (b *MainLogBuilder) RequestPath(path string) *MainLogBuilder {
	b.main.RequestPath = path
	return b
}

func (b *MainLogBuilder) RequestMethod(method string) *MainLogBuilder {
	b.main.RequestMethod = method
	return b
}

func (b *MainLogBuilder) RequestedModel(model string) *MainLogBuilder {
	b.main.RequestedModel = model
	return b
}

func (b *MainLogBuilder) AccessToken(token string) *MainLogBuilder {
	b.main.AccessToken = token
	return b
}

func (b *MainLogBuilder) ClientIP(ip string) *MainLogBuilder {
	b.main.ClientIP = ip
	return b
}

func (b *MainLogBuilder) UserAgent(ua string) *MainLogBuilder {
	b.main.UserAgent = ua
	return b
}

func (b *MainLogBuilder) StartTime(t time.Time) *MainLogBuilder {
	b.main.StartTime = t
	return b
}

func (b *MainLogBuilder) EndTime(t time.Time) *MainLogBuilder {
	b.main.EndTime = t
	return b
}

func (b *MainLogBuilder) Duration(ms int) *MainLogBuilder {
	b.main.Duration = ms
	return b
}

func (b *MainLogBuilder) IsSuccess(success bool) *MainLogBuilder {
	b.main.IsSuccess = success
	return b
}

func (b *MainLogBuilder) StatusCode(code int) *MainLogBuilder {
	b.main.StatusCode = code
	return b
}

func (b *MainLogBuilder) RetryCount(count int) *MainLogBuilder {
	b.main.RetryCount = count
	return b
}

func (b *MainLogBuilder) IsStream(stream bool) *MainLogBuilder {
	b.main.IsStream = stream
	return b
}

func (b *MainLogBuilder) PromptTokens(tokens int64) *MainLogBuilder {
	b.main.PromptTokens = tokens
	return b
}

func (b *MainLogBuilder) CompletionTokens(tokens int64) *MainLogBuilder {
	b.main.CompletionTokens = tokens
	return b
}

func (b *MainLogBuilder) LastChannelID(id uint) *MainLogBuilder {
	b.main.LastChannelID = &id
	return b
}

func (b *MainLogBuilder) LastChannelName(name string) *MainLogBuilder {
	b.main.LastChannelName = name
	return b
}

func (b *MainLogBuilder) LastModel(model string) *MainLogBuilder {
	b.main.LastModel = model
	return b
}

func (b *MainLogBuilder) ErrorMessage(msg string) *MainLogBuilder {
	b.main.ErrorMessage = msg
	return b
}

func (b *MainLogBuilder) AddDetail(detail *DetailLogBuilder) *models.RequestLogMain {
	b.main.Details = append(b.main.Details, detail.Build())
	return b.main
}

func (b *MainLogBuilder) Build() *models.RequestLogMain {
	return b.main
}
