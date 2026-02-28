package auth_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestUserFullName(t *testing.T) {
	fn := "Jane"
	ln := "Doe"
	u := auth.User{FirstName: &fn, LastName: &ln}
	assert.Equal(t, "Jane Doe", u.FullName())
}

func TestUserFullName_NilFields(t *testing.T) {
	u := auth.User{}
	assert.Equal(t, "", u.FullName())
}

func TestPermissionKey_Valid(t *testing.T) {
	assert.True(t, auth.IsValidPermissionKey("ticket:create"))
	assert.True(t, auth.IsValidPermissionKey("admin:*"))
	assert.False(t, auth.IsValidPermissionKey("invalid"))
	assert.False(t, auth.IsValidPermissionKey(""))
}

var _ = auth.User{ID: uuid.New(), TenantID: uuid.New(), Email: "a@b.com"}
var _ = auth.Role{ID: uuid.New(), TenantID: uuid.New(), Name: "admin", Slug: "admin"}
var _ = auth.Permission{Key: "ticket:create", Name: "Create Tickets"}
