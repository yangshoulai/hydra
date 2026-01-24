package logger

import (
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

// NewDetailLogBuilder 创建明细日志构建器
func NewDetailLogBuilder() *DetailLogBuilder {
	return &DetailLogBuilder{
		detail: &models.RequestLogDetail{},
	}
}

// DetailLogBuilder 明细日志构建器
type DetailLogBuilder struct {
	detail *models.RequestLogDetail
}

func (b *DetailLogBuilder) ChannelID(id uint) *DetailLogBuilder {
	b.detail.ChannelID = &id
	return b
}

func (b *DetailLogBuilder) ChannelName(name string) *DetailLogBuilder {
	b.detail.ChannelName = name
	return b
}

func (b *DetailLogBuilder) Model(model string) *DetailLogBuilder {
	b.detail.Model = model
	return b
}

func (b *DetailLogBuilder) KeyID(id uint) *DetailLogBuilder {
	b.detail.KeyID = &id
	return b
}

func (b *DetailLogBuilder) StartTime(startTime time.Time) *DetailLogBuilder {
	b.detail.StartTime = startTime
	return b
}

func (b *DetailLogBuilder) EndTime(endTime time.Time) *DetailLogBuilder {
	b.detail.EndTime = endTime
	return b
}

func (b *DetailLogBuilder) Duration(ms int) *DetailLogBuilder {
	b.detail.Duration = ms
	return b
}

func (b *DetailLogBuilder) StatusCode(code int) *DetailLogBuilder {
	b.detail.StatusCode = code
	return b
}

func (b *DetailLogBuilder) IsSuccess(success bool) *DetailLogBuilder {
	b.detail.IsSuccess = success
	return b
}

func (b *DetailLogBuilder) Status(status string) *DetailLogBuilder {
	b.detail.Status = status
	return b
}

func (b *DetailLogBuilder) RetryIndex(index int) *DetailLogBuilder {
	b.detail.RetryIndex = index
	return b
}

func (b *DetailLogBuilder) PromptTokens(tokens int64) *DetailLogBuilder {
	b.detail.PromptTokens = tokens
	return b
}

func (b *DetailLogBuilder) CompletionTokens(tokens int64) *DetailLogBuilder {
	b.detail.CompletionTokens = tokens
	return b
}

func (b *DetailLogBuilder) IsStream(stream bool) *DetailLogBuilder {
	b.detail.IsStream = stream
	return b
}

func (b *DetailLogBuilder) StreamChunks(chunks int) *DetailLogBuilder {
	b.detail.StreamChunks = chunks
	return b
}

func (b *DetailLogBuilder) StreamFirstChunkTime(ms int) *DetailLogBuilder {
	b.detail.StreamFirstChunkTime = &ms
	return b
}

func (b *DetailLogBuilder) RequestBodySize(size int) *DetailLogBuilder {
	b.detail.RequestBodySize = size
	return b
}

func (b *DetailLogBuilder) ResponseBodySize(size int) *DetailLogBuilder {
	b.detail.ResponseBodySize = size
	return b
}

func (b *DetailLogBuilder) RequestHeaders(headers string) *DetailLogBuilder {
	b.detail.RequestHeaders = headers
	return b
}

func (b *DetailLogBuilder) RequestBody(body string) *DetailLogBuilder {
	b.detail.RequestBody = body
	return b
}

func (b *DetailLogBuilder) ResponseHeaders(headers string) *DetailLogBuilder {
	b.detail.ResponseHeaders = headers
	return b
}

func (b *DetailLogBuilder) ResponseBody(body string) *DetailLogBuilder {
	b.detail.ResponseBody = body
	return b
}

func (b *DetailLogBuilder) ErrorMessage(msg string) *DetailLogBuilder {
	b.detail.ErrorMessage = msg
	return b
}

func (b *DetailLogBuilder) Build() models.RequestLogDetail {
	return *b.detail
}
