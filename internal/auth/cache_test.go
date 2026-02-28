package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestCache_SetAndGet(t *testing.T) {
	c := auth.NewPermissionCache(5 * time.Minute)
	uid := uuid.New()
	c.Set(uid, []string{"ticket:create", "ticket:read"})
	perms, ok := c.Get(uid)
	assert.True(t, ok)
	assert.ElementsMatch(t, []string{"ticket:create", "ticket:read"}, perms)
}

func TestCache_Miss(t *testing.T) {
	c := auth.NewPermissionCache(5 * time.Minute)
	_, ok := c.Get(uuid.New())
	assert.False(t, ok)
}

func TestCache_Expiry(t *testing.T) {
	c := auth.NewPermissionCache(10 * time.Millisecond)
	uid := uuid.New()
	c.Set(uid, []string{"ticket:create"})
	time.Sleep(20 * time.Millisecond)
	_, ok := c.Get(uid)
	assert.False(t, ok, "should be expired")
}

func TestCache_Invalidate(t *testing.T) {
	c := auth.NewPermissionCache(5 * time.Minute)
	uid := uuid.New()
	c.Set(uid, []string{"ticket:create"})
	c.Invalidate(uid)
	_, ok := c.Get(uid)
	assert.False(t, ok)
}
