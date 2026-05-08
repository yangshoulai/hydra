package proxy

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// RequestLogEvent 请求日志异步写入事件
type RequestLogEvent struct {
	Log      *models.RequestLog
	Detail   *models.RequestLogDetail    // nil 表示调试关，不写客户端请求/响应详情
	Attempts []*models.RequestLogAttempt // 始终可写基础上游尝试信息；调试关时不含报文/头
}

// RequestLogRecorder 基于 worker pool 的请求日志异步写入器
//
// 和 TokenUsageRecorder 同样的思路：阻塞投递保证不丢；workers 按事务批写。
// 调用方负责判断是否进入调试模式并组装 Detail/Attempts 的敏感字段；本组件不读设置。
type RequestLogRecorder struct {
	logger *slog.Logger
	repo   *repository.RequestLogRepository

	ch       chan RequestLogEvent
	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewRequestLogRecorder 创建并启动 workers
func NewRequestLogRecorder(
	logger *slog.Logger,
	repo *repository.RequestLogRepository,
	queueSize, workers int,
) *RequestLogRecorder {
	if queueSize <= 0 {
		queueSize = 1024
	}
	if workers <= 0 {
		workers = 2
	}
	r := &RequestLogRecorder{
		logger: logger,
		repo:   repo,
		ch:     make(chan RequestLogEvent, queueSize),
		stopCh: make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		r.wg.Add(1)
		go r.worker()
	}
	return r
}

// Record 阻塞投递事件。进程关闭后投递会被丢弃。
func (r *RequestLogRecorder) Record(event RequestLogEvent) {
	if event.Log == nil {
		return
	}
	select {
	case r.ch <- event:
	case <-r.stopCh:
	}
}

func (r *RequestLogRecorder) worker() {
	defer r.wg.Done()
	for ev := range r.ch {
		r.process(ev)
	}
}

func (r *RequestLogRecorder) process(ev RequestLogEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := r.repo.CreateWithTx(ctx, ev.Log, ev.Detail, ev.Attempts); err != nil {
		r.logger.Warn("写入请求日志失败",
			slog.String("trace_id", ev.Log.TraceID),
			slog.String("error", err.Error()),
		)
	}
}

// Close 关闭队列并等待所有在飞事件处理完毕。幂等。
func (r *RequestLogRecorder) Close() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
		close(r.ch)
		r.wg.Wait()
	})
}
