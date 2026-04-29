package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	configService "github.com/yangshoulai/hydra/internal/service/config"
)

const rateLimitBucketTTL = 10 * time.Minute

type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

func (b *tokenBucket) allow(now time.Time, rps int, burst int) bool {
	if rps <= 0 {
		b.lastSeen = now
		return true
	}
	if burst <= 0 {
		burst = rps
	}
	if b.lastSeen.IsZero() {
		b.tokens = float64(burst)
		b.lastSeen = now
	}
	elapsed := now.Sub(b.lastSeen).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * float64(rps)
		if maxTokens := float64(burst); b.tokens > maxTokens {
			b.tokens = maxTokens
		}
		b.lastSeen = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}

// ProxyRateLimiter 是代理层内存令牌桶限流器。
// 配置每次请求从 SettingService 读取，后台保存配置后即可立即影响后续请求。
type ProxyRateLimiter struct {
	settingService *configService.SettingService
	logger         *slog.Logger

	mu          sync.Mutex
	global      tokenBucket
	tokenBucket map[string]*tokenBucket
	lastSweep   time.Time
}

func NewProxyRateLimiter(settingService *configService.SettingService, logger *slog.Logger) *ProxyRateLimiter {
	return &ProxyRateLimiter{
		settingService: settingService,
		logger:         logger,
		tokenBucket:    make(map[string]*tokenBucket),
	}
}

func (l *ProxyRateLimiter) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil || l.settingService == nil {
			c.Next()
			return
		}

		cfg := l.settingService.GetProxyRateLimitConfig(c.Request.Context())
		if !cfg.Enabled {
			c.Next()
			return
		}

		now := time.Now()
		tokenKey := l.resolveTokenKey(c)

		l.mu.Lock()
		allowed := l.global.allow(now, cfg.GlobalRPS, cfg.GlobalBurst)
		if allowed && cfg.TokenRPS > 0 {
			bucket := l.tokenBucket[tokenKey]
			if bucket == nil {
				bucket = &tokenBucket{}
				l.tokenBucket[tokenKey] = bucket
			}
			allowed = bucket.allow(now, cfg.TokenRPS, cfg.TokenBurst)
		}
		l.sweepLocked(now)
		l.mu.Unlock()

		if !allowed {
			traceID := GetTraceID(c)
			if l.logger != nil {
				l.logger.Warn("代理请求触发限流",
					slog.String("trace_id", traceID),
					slog.String("token_key", tokenKey),
					slog.Int("global_rps", cfg.GlobalRPS),
					slog.Int("token_rps", cfg.TokenRPS),
				)
			}
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (l *ProxyRateLimiter) resolveTokenKey(c *gin.Context) string {
	if tokenID, ok := c.Get("access_token_id"); ok {
		return fmt.Sprintf("token:%v", tokenID)
	}
	return "ip:" + c.ClientIP()
}

func (l *ProxyRateLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for key, bucket := range l.tokenBucket {
		if bucket == nil || now.Sub(bucket.lastSeen) > rateLimitBucketTTL {
			delete(l.tokenBucket, key)
		}
	}
}
