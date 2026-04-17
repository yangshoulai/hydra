package proxy

import (
	"context"
	"log/slog"
	"sync"
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
// - 阻塞投递保证统计不丢
type TokenUsageRecorder struct {
	logger          *slog.Logger
	modelConfigRepo *repository.ChannelModelConfigRepository
	channelKeyRepo  *repository.ChannelKeyRepository
	accessTokenRepo *repository.AccessTokenRepository

	ch       chan tokenUsageEvent
	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
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
	r := &TokenUsageRecorder{
		logger:          logger,
		modelConfigRepo: modelConfigRepo,
		channelKeyRepo:  channelKeyRepo,
		accessTokenRepo: accessTokenRepo,
		ch:              make(chan tokenUsageEvent, queueSize),
		stopCh:          make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		r.wg.Add(1)
		go r.worker()
	}
	return r
}

// Record 阻塞投递事件。promptTokens 与 completionTokens 都为 0 时直接忽略。
// 调用方应承受 channel 满载时的短暂阻塞；进程关闭后投递会被丢弃。
func (r *TokenUsageRecorder) Record(event tokenUsageEvent) {
	if event.promptTokens == 0 && event.completionTokens == 0 {
		return
	}
	select {
	case r.ch <- event:
	case <-r.stopCh:
	}
}

func (r *TokenUsageRecorder) worker() {
	defer r.wg.Done()
	for ev := range r.ch {
		r.process(ev)
	}
}

func (r *TokenUsageRecorder) process(ev tokenUsageEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

// Close 关闭队列并等待所有在飞事件处理完毕。幂等。
func (r *TokenUsageRecorder) Close() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
		close(r.ch)
		r.wg.Wait()
	})
}
