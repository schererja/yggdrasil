package auth

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Email     string
	FirstName *string
	LastName  *string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (u User) FullName() string {
	parts := []string{}
	if u.FirstName != nil {
		parts = append(parts, *u.FirstName)
	}
	if u.LastName != nil {
		parts = append(parts, *u.LastName)
	}
	return strings.Join(parts, " ")
}

type Identity struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       string
	ProviderID     string
	CredentialHash *string
	CreatedAt      time.Time
}

type Role struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
}

type Permission struct {
	Key         string
	Name        string
	Description string
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type TokenClaims struct {
	UserID   uuid.UUID `json:"sub"`
	TenantID uuid.UUID `json:"tid"`
}

// IsValidPermissionKey checks that a key follows "resource:action" format.
func IsValidPermissionKey(key string) bool {
	if key == "" {
		return false
	}
	parts := strings.SplitN(key, ":", 2)
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
