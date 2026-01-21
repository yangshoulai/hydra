package logger

import (
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

// RequestLogContext 请求日志上下文
type RequestLogContext struct {
	main    *models.RequestLogMain
	details []models.RequestLogDetail
}

// NewRequestLogContext 创建请求日志上下文
func NewRequestLogContext() *RequestLogContext {
	return &RequestLogContext{
		main:    &models.RequestLogMain{},
		details: make([]models.RequestLogDetail, 0),
	}
}

// SetMainLog 设置主日志信息
func (ctx *RequestLogContext) SetMainLog(main *models.RequestLogMain) {
	ctx.main = main
}

// AddDetail 添加明细日志
func (ctx *RequestLogContext) AddDetail(detail models.RequestLogDetail) {
	ctx.details = append(ctx.details, detail)
}

// GetMain 获取主日志
func (ctx *RequestLogContext) GetMain() *models.RequestLogMain {
	return ctx.main
}

// GetDetails 获取所有明细日志
func (ctx *RequestLogContext) GetDetails() []models.RequestLogDetail {
	return ctx.details
}

// UpdateMainLogOnSuccess 更新主日志为成功状态
func (ctx *RequestLogContext) UpdateMainLogOnSuccess(
	endTime time.Time,
	statusCode int,
	channelID uint,
	channelName string,
	upstreamModel string,
) {
	ctx.main.EndTime = endTime
	ctx.main.Duration = int(endTime.Sub(ctx.main.StartTime).Milliseconds())
	ctx.main.IsSuccess = true
	ctx.main.StatusCode = statusCode
	ctx.main.LastChannelID = &channelID
	ctx.main.LastChannelName = channelName
	ctx.main.LastModel = upstreamModel
	ctx.main.RetryCount = len(ctx.details) - 1
}

// UpdateMainLogOnFailure 更新主日志为失败状态
func (ctx *RequestLogContext) UpdateMainLogOnFailure(
	endTime time.Time,
	statusCode int,
	errorMessage string,
	channelID *uint,
	channelName string,
	upstreamModel string,
) {
	ctx.main.EndTime = endTime
	ctx.main.Duration = int(endTime.Sub(ctx.main.StartTime).Milliseconds())
	ctx.main.IsSuccess = false
	ctx.main.StatusCode = statusCode
	ctx.main.ErrorMessage = errorMessage
	ctx.main.LastChannelID = channelID
	ctx.main.LastChannelName = channelName
	ctx.main.LastModel = upstreamModel
	ctx.main.RetryCount = len(ctx.details) - 1
}
