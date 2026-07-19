package proxy

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
)

// tokenUsageEvent 异步 token 统计事件
type tokenUsageEvent struct {
	modelConfigID    uint
	keyID            uint
	accessTokenID    uint
	promptTokens     int64
	completionTokens int64
	traceID          string
}

// TokenUsageRecorder 基于 worker pool 的 token 使用量异步写入器。
//
// 设计目标：
// - 控制 goroutine 数量，避免高 QPS 下无界增长
// - 合并同时写三张表的数据库抖动到固定 worker
// - 队列满时丢弃统计并告警，优先保障代理请求时延
type TokenUsageRecorder struct {
	logger          *slog.Logger
	modelConfigRepo *repository.ChannelModelConfigRepository
	channelKeyRepo  *repository.ChannelKeyRepository
	accessTokenRepo *repository.AccessTokenRepository

	ch            chan tokenUsageEvent
	wg            sync.WaitGroup
	mu            sync.RWMutex
	closed        bool
	stopOnce      sync.Once
	dropped       atomic.Uint64
	workerCtx     context.Context
	cancelWorkers context.CancelFunc
}

// NewTokenUsageRecorder 创建并启动 worker
func NewTokenUsageRecorder(
	logger *slog.Logger,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	channelKeyRepo *repository.ChannelKeyRepository,
	accessTokenRepo *repository.AccessTokenRepository,
	queueSize, workers int,
) *TokenUsageRecorder {
	if queueSize <= 0 {
		queueSize = 1024
	}
	if workers <= 0 {
		workers = 4
	}
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	r := &TokenUsageRecorder{
		logger:          logger,
		modelConfigRepo: modelConfigRepo,
		channelKeyRepo:  channelKeyRepo,
		accessTokenRepo: accessTokenRepo,
		ch:              make(chan tokenUsageEvent, queueSize),
		workerCtx:       workerCtx,
		cancelWorkers:   cancelWorkers,
	}
	for i := 0; i < workers; i++ {
		r.wg.Add(1)
		go r.worker()
	}
	return r
}

// Record 非阻塞投递事件。promptTokens 与 completionTokens 都为 0 时直接忽略。
// 队列满或关闭时丢弃统计，避免统计写入反压代理主链路。
func (r *TokenUsageRecorder) Record(event tokenUsageEvent) {
	if event.promptTokens == 0 && event.completionTokens == 0 {
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
			r.logger.Warn("Token 统计队列已满，已丢弃统计事件",
				slog.Uint64("dropped_total", dropped),
				slog.Int("queue_capacity", cap(r.ch)),
			)
		}
	}
}

func (r *TokenUsageRecorder) worker() {
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

func (r *TokenUsageRecorder) process(ev tokenUsageEvent) {
	ctx, cancel := context.WithTimeout(r.workerCtx, 5*time.Second)
	defer cancel()

	if ev.modelConfigID != 0 {
		if err := r.modelConfigRepo.IncrementTokenUsage(ctx, ev.modelConfigID, ev.promptTokens, ev.completionTokens); err != nil {
			r.logger.Warn("更新模型配置 token 统计失败",
				slog.String("trace_id", ev.traceID),
				slog.String("error", err.Error()),
			)
		}
	}
	if ev.keyID != 0 {
		if err := r.channelKeyRepo.IncrementTokenUsage(ctx, ev.keyID, ev.promptTokens, ev.completionTokens); err != nil {
			r.logger.Warn("更新密钥 token 统计失败",
				slog.String("trace_id", ev.traceID),
				slog.String("error", err.Error()),
			)
		}
	}
	if ev.accessTokenID != 0 {
		if err := r.accessTokenRepo.IncrementTokenUsage(ctx, ev.accessTokenID, ev.promptTokens, ev.completionTokens); err != nil {
			r.logger.Warn("更新访问令牌 token 统计失败",
				slog.String("trace_id", ev.traceID),
				slog.String("error", err.Error()),
			)
		}
	}
}

// Close 停止接收新事件并取消 worker 正在执行的写库操作。
// 停机优先于统计完整性：遗留队列被丢弃，避免 SQLite 卡顿令重启无限等待。
// 幂等且与并发 Record 安全。
func (r *TokenUsageRecorder) Close() {
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
				r.logger.Warn("服务停止，已丢弃未写入的 Token 统计事件",
					slog.Int("dropped_on_shutdown", pending),
				)
			}
		}
		r.wg.Wait()
	})
}
