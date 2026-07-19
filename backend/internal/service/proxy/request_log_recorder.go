package proxy

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
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
// 队列满时主动降级丢弃日志，不能反向阻塞代理请求；workers 按事务写入。
// 调用方负责判断是否进入调试模式并组装 Detail/Attempts 的敏感字段；本组件不读设置。
type RequestLogRecorder struct {
	logger *slog.Logger
	repo   *repository.RequestLogRepository

	ch            chan RequestLogEvent
	wg            sync.WaitGroup
	mu            sync.RWMutex
	closed        bool
	stopOnce      sync.Once
	dropped       atomic.Uint64
	workerCtx     context.Context
	cancelWorkers context.CancelFunc
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
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	r := &RequestLogRecorder{
		logger:        logger,
		repo:          repo,
		ch:            make(chan RequestLogEvent, queueSize),
		workerCtx:     workerCtx,
		cancelWorkers: cancelWorkers,
	}
	for i := 0; i < workers; i++ {
		r.wg.Add(1)
		go r.worker()
	}
	return r
}

// Record 非阻塞投递事件。队列满或关闭时丢弃日志，保证日志系统不会拖慢代理。
func (r *RequestLogRecorder) Record(event RequestLogEvent) {
	if event.Log == nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return
	}
	select {
	case r.ch <- event:
	default:
		dropped := r.dropped.Add(1)
		if r.logger != nil && (dropped == 1 || dropped%100 == 0) {
			r.logger.Warn("请求日志队列已满，已丢弃日志事件",
				slog.Uint64("dropped_total", dropped),
				slog.Int("queue_capacity", cap(r.ch)),
			)
		}
	}
}

func (r *RequestLogRecorder) worker() {
	defer r.wg.Done()
	for {
		// Close 时优先退出，避免停机时在满队列上串行等待大量数据库超时。
		if r.workerCtx.Err() != nil {
			return
		}
		select {
		case <-r.workerCtx.Done():
			return
		case ev, ok := <-r.ch:
			if !ok {
				return
			}
			r.process(ev)
		}
	}
}

func (r *RequestLogRecorder) process(ev RequestLogEvent) {
	ctx, cancel := context.WithTimeout(r.workerCtx, 10*time.Second)
	defer cancel()

	if err := r.repo.CreateWithTx(ctx, ev.Log, ev.Detail, ev.Attempts); err != nil {
		r.logger.Warn("写入请求日志失败",
			slog.String("trace_id", ev.Log.TraceID),
			slog.String("error", err.Error()),
		)
	}
}

// Close 停止接收新事件并取消 worker 正在执行的写库操作。
// 停机优先于日志完整性：遗留队列被丢弃，避免 SQLite 卡顿令重启无限等待。
// 幂等且与并发 Record 安全。
func (r *RequestLogRecorder) Close() {
	r.stopOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		pending := len(r.ch)
		close(r.ch)
		if r.cancelWorkers != nil {
			r.cancelWorkers()
		}
		r.mu.Unlock()
		if pending > 0 {
			r.dropped.Add(uint64(pending))
			if r.logger != nil {
				r.logger.Warn("服务停止，已丢弃未写入的请求日志事件",
					slog.Int("dropped_on_shutdown", pending),
				)
			}
		}
		r.wg.Wait()
	})
}
