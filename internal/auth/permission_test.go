package auth_test

import (
	"testing"

	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndList(t *testing.T) {
	r := auth.NewRegistry()
	r.Register(auth.Permission{Key: "ticket:create", Name: "Create Tickets"})
	r.Register(auth.Permission{Key: "ticket:read", Name: "Read Tickets"})
	assert.Len(t, r.List(), 2)
}

func TestRegistry_Register_DuplicateKey(t *testing.T) {
	r := auth.NewRegistry()
	r.Register(auth.Permission{Key: "ticket:create", Name: "Create Tickets"})
	r.Register(auth.Permission{Key: "ticket:create", Name: "Updated Name"})
	perms := r.List()
	require.Len(t, perms, 1)
	assert.Equal(t, "Updated Name", perms[0].Name)
}

func TestRegistry_Get(t *testing.T) {
	r := auth.NewRegistry()
	r.Register(auth.Permission{Key: "ticket:create", Name: "Create Tickets"})
	p, ok := r.Get("ticket:create")
	assert.True(t, ok)
	assert.Equal(t, "ticket:create", p.Key)
	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}
