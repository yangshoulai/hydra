package repository

import (
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

const defaultAccessTokenCacheTTL = 30 * time.Second

type accessTokenCacheEntry struct {
	token     *models.AccessToken
	expiresAt time.Time
}

type accessTokenCache struct {
	mu       sync.RWMutex
	ttl      time.Duration
	byHash   map[string]accessTokenCacheEntry
	idToHash map[uint]string
}

func newAccessTokenCache(ttl time.Duration) *accessTokenCache {
	if ttl <= 0 {
		ttl = defaultAccessTokenCacheTTL
	}
	return &accessTokenCache{
		ttl:      ttl,
		byHash:   make(map[string]accessTokenCacheEntry),
		idToHash: make(map[uint]string),
	}
}

func (c *accessTokenCache) get(tokenHash string) (*models.AccessToken, bool) {
	if c == nil || tokenHash == "" {
		return nil, false
	}
	now := time.Now()
	c.mu.RLock()
	entry, ok := c.byHash[tokenHash]
	c.mu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		if ok {
			c.invalidateHash(tokenHash)
		}
		return nil, false
	}
	return cloneAccessToken(entry.token), true
}

func (c *accessTokenCache) set(tokenHash string, token *models.AccessToken) {
	if c == nil || tokenHash == "" || token == nil {
		return
	}
	expiresAt := time.Now().Add(c.ttl)
	if token.ExpiresAt != nil && token.ExpiresAt.Before(expiresAt) {
		expiresAt = *token.ExpiresAt
	}
	if time.Now().After(expiresAt) {
		return
	}

	c.mu.Lock()
	c.byHash[tokenHash] = accessTokenCacheEntry{
		token:     cloneAccessToken(token),
		expiresAt: expiresAt,
	}
	c.idToHash[token.ID] = tokenHash
	c.mu.Unlock()
}

func (c *accessTokenCache) invalidateByID(id uint) {
	if c == nil || id == 0 {
		return
	}
	c.mu.Lock()
	if tokenHash, ok := c.idToHash[id]; ok {
		delete(c.byHash, tokenHash)
		delete(c.idToHash, id)
	}
	c.mu.Unlock()
}

func (c *accessTokenCache) invalidateHash(tokenHash string) {
	if c == nil || tokenHash == "" {
		return
	}
	c.mu.Lock()
	if entry, ok := c.byHash[tokenHash]; ok && entry.token != nil {
		delete(c.idToHash, entry.token.ID)
	}
	delete(c.byHash, tokenHash)
	c.mu.Unlock()
}

func cloneAccessToken(token *models.AccessToken) *models.AccessToken {
	if token == nil {
		return nil
	}
	cloned := *token
	if token.ExpiresAt != nil {
		expiresAt := *token.ExpiresAt
		cloned.ExpiresAt = &expiresAt
	}
	if token.LastUsedAt != nil {
		lastUsedAt := *token.LastUsedAt
		cloned.LastUsedAt = &lastUsedAt
	}
	if token.AllowedModels != nil {
		cloned.AllowedModels = append(models.AllowedModels(nil), token.AllowedModels...)
	}
	return &cloned
}
