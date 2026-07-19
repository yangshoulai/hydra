package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/repository"
)

const accessTokenLastUsedUpdateInterval = time.Minute

// accessTokenLastUsedUpdater 将高频请求的 last_used_at 更新合并为每个 Token 最多每分钟一次。
// 它不持有 Gin Context，避免请求结束后使用已取消的上下文，也避免每个请求都创建写库 goroutine。
type accessTokenLastUsedUpdater struct {
	repo *repository.AccessTokenRepository

	mu       sync.Mutex
	lastSeen map[uint]time.Time
	inFlight map[uint]struct{}
}

func newAccessTokenLastUsedUpdater(repo *repository.AccessTokenRepository) *accessTokenLastUsedUpdater {
	return &accessTokenLastUsedUpdater{
		repo:     repo,
		lastSeen: make(map[uint]time.Time),
		inFlight: make(map[uint]struct{}),
	}
}

func (u *accessTokenLastUsedUpdater) Schedule(tokenID uint) {
	if u == nil || u.repo == nil || tokenID == 0 {
		return
	}

	now := time.Now()
	u.mu.Lock()
	if _, busy := u.inFlight[tokenID]; busy {
		u.mu.Unlock()
		return
	}
	if last, ok := u.lastSeen[tokenID]; ok && now.Sub(last) < accessTokenLastUsedUpdateInterval {
		u.mu.Unlock()
		return
	}
	u.inFlight[tokenID] = struct{}{}
	u.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := u.repo.UpdateLastUsed(ctx, tokenID)
		cancel()

		u.mu.Lock()
		delete(u.inFlight, tokenID)
		if err == nil {
			u.lastSeen[tokenID] = now
		}
		u.mu.Unlock()
	}()
}

// Auth 访问令牌认证中间件(用于代理接口)
func Auth(tokenRepo *repository.AccessTokenRepository, logger *slog.Logger) gin.HandlerFunc {
	lastUsedUpdater := newAccessTokenLastUsedUpdater(tokenRepo)
	return func(c *gin.Context) {
		var tokenValue string

		xApiKeyValue := c.GetHeader("X-Api-Key")
		authorizationValue := c.GetHeader("Authorization")
		googleApiKey := c.GetHeader("x-goog-api-key")
		if authorizationValue != "" {
			// 解析 Bearer token
			parts := strings.SplitN(authorizationValue, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
				c.Abort()
				return
			}
			tokenValue = parts[1]
		} else if xApiKeyValue != "" {
			tokenValue = xApiKeyValue
		} else {
			tokenValue = googleApiKey
		}
		if tokenValue == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			c.Abort()
			return
		}
		// 验证 token
		token, err := tokenRepo.FindByToken(c.Request.Context(), tokenValue)
		if err != nil {
			traceID := GetTraceID(c)
			logger.Warn("令牌验证失败", slog.String("trace_id", traceID), slog.String("error", err.Error()))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 检查 token 是否启用
		if !token.IsActive() {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token is disabled"})
			c.Abort()
			return
		}

		// 按 Token 合并异步更新时间，避免高 QPS 下产生无界 goroutine 与 SQLite 写入。
		lastUsedUpdater.Schedule(token.ID)

		// 将 token 信息存储到上下文
		c.Set("access_token_id", token.ID)
		c.Set("access_token_name", token.Name)
		c.Set("access_token_allowed_models", []string(token.AllowedModels))

		c.Next()
	}
}
