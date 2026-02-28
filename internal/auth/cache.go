package auth

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type cacheEntry struct {
	permissions []string
	expiresAt   time.Time
}

type PermissionCache struct {
	mu      sync.RWMutex
	entries map[uuid.UUID]cacheEntry
	ttl     time.Duration
}

func NewPermissionCache(ttl time.Duration) *PermissionCache {
	return &PermissionCache{
		entries: make(map[uuid.UUID]cacheEntry),
		ttl:     ttl,
	}
}

func (c *PermissionCache) Get(userID uuid.UUID) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[userID]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.permissions, true
}

func (c *PermissionCache) Set(userID uuid.UUID, permissions []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[userID] = cacheEntry{
		permissions: permissions,
		expiresAt:   time.Now().Add(c.ttl),
	}
}

func (c *PermissionCache) Invalidate(userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, userID)
}
