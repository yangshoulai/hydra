package repository

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

const defaultRouteCacheTTL = 10 * time.Second

var routeDataVersion atomic.Uint64

func touchRouteDataVersion() {
	routeDataVersion.Add(1)
}

func currentRouteDataVersion() uint64 {
	return routeDataVersion.Load()
}

type routeCacheKey struct {
	model        string
	endpointType string
}

type routeCacheEntry struct {
	channels  []models.Channel
	expiresAt time.Time
	version   uint64
}

type routeCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[routeCacheKey]routeCacheEntry
}

func newRouteCache(ttl time.Duration) *routeCache {
	if ttl <= 0 {
		ttl = defaultRouteCacheTTL
	}
	return &routeCache{
		ttl:     ttl,
		entries: make(map[routeCacheKey]routeCacheEntry),
	}
}

func (c *routeCache) get(key routeCacheKey, version uint64) ([]models.Channel, bool) {
	if c == nil {
		return nil, false
	}
	now := time.Now()
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || entry.version != version || now.After(entry.expiresAt) {
		if ok {
			c.mu.Lock()
			if latest, exists := c.entries[key]; exists && (latest.version != version || now.After(latest.expiresAt)) {
				delete(c.entries, key)
			}
			c.mu.Unlock()
		}
		return nil, false
	}
	return cloneChannels(entry.channels), true
}

func (c *routeCache) set(key routeCacheKey, channels []models.Channel, version uint64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries[key] = routeCacheEntry{
		channels:  cloneChannels(channels),
		expiresAt: time.Now().Add(c.ttl),
		version:   version,
	}
	c.mu.Unlock()
}

func cloneChannels(channels []models.Channel) []models.Channel {
	if len(channels) == 0 {
		return nil
	}
	cloned := make([]models.Channel, len(channels))
	for i := range channels {
		cloned[i] = channels[i]
		if len(channels[i].ChannelKeys) > 0 {
			cloned[i].ChannelKeys = append([]models.ChannelKey(nil), channels[i].ChannelKeys...)
		}
		if len(channels[i].ModelConfigs) > 0 {
			cloned[i].ModelConfigs = append([]models.ChannelModelConfig(nil), channels[i].ModelConfigs...)
		}
	}
	return cloned
}
