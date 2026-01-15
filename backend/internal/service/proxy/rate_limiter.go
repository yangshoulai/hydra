package proxy

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrTooManyRequests 并发超限错误
	ErrTooManyRequests = errors.New("concurrent requests limit exceeded")
)

// RateLimiter 限流器,控制并发请求数量
type RateLimiter struct {
	maxConcurrent int
	semaphore     chan struct{}
	activeCount   int
	mu            sync.Mutex
}

// NewRateLimiter 创建限流器实例
func NewRateLimiter(maxConcurrent int) *RateLimiter {
	if maxConcurrent <= 0 {
		maxConcurrent = 1000 // 默认值
	}

	return &RateLimiter{
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// Acquire 尝试获取请求许可
// 如果并发超限,立即返回 ErrTooManyRequests
func (r *RateLimiter) Acquire(ctx context.Context) error {
	select {
	case r.semaphore <- struct{}{}:
		r.mu.Lock()
		r.activeCount++
		r.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 非阻塞模式,超限立即返回错误
		return ErrTooManyRequests
	}
}

// Release 释放请求许可
func (r *RateLimiter) Release() {
	select {
	case <-r.semaphore:
		r.mu.Lock()
		r.activeCount--
		r.mu.Unlock()
	default:
		// 防止重复释放
	}
}

// GetActiveCount 获取当前活跃请求数
func (r *RateLimiter) GetActiveCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeCount
}

// GetMaxConcurrent 获取最大并发数配置
func (r *RateLimiter) GetMaxConcurrent() int {
	return r.maxConcurrent
}

// UpdateMaxConcurrent 动态更新最大并发数(需要重新创建限流器)
// 注意: 此方法仅用于运行时配置更新,需要业务层协调
func (r *RateLimiter) UpdateMaxConcurrent(newMax int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if newMax <= 0 {
		return
	}

	r.maxConcurrent = newMax
	// 创建新的信号量通道
	newSemaphore := make(chan struct{}, newMax)
	
	// 迁移现有许可到新通道
	close(r.semaphore)
	for range r.semaphore {
		select {
		case newSemaphore <- struct{}{}:
		default:
			break
		}
	}
	
	r.semaphore = newSemaphore
}
