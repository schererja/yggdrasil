package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionExpired     = errors.New("session expired or revoked")
	ErrUserNotFound       = errors.New("user not found")
)

type Service struct {
	repo       *Repository
	jwtManager *JWTManager
	cache      *PermissionCache
	refreshTTL time.Duration
}

func NewService(db *sql.DB, jwtSecret string, jwtExpiry, refreshTTL time.Duration) *Service {
	return &Service{
		repo:       NewRepository(db),
		jwtManager: NewJWTManager(jwtSecret, jwtExpiry),
		cache:      NewPermissionCache(5 * time.Minute),
		refreshTTL: refreshTTL,
	}
}

// JWTManager exposes the JWT manager for middleware wiring.
func (s *Service) JWTManager() *JWTManager {
	return s.jwtManager
}

// Register creates a new user with a local identity (email + password).
func (s *Service) Register(ctx context.Context, tenantID uuid.UUID, email, password string, firstName, lastName *string) error {
	user, err := s.repo.CreateUser(ctx, tenantID, email, firstName, lastName)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	hash, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = s.repo.CreateIdentity(ctx, user.ID, "local", email, &hash)
	return err
}

// Login verifies credentials and returns an access token + raw refresh token.
func (s *Service) Login(ctx context.Context, tenantID uuid.UUID, email, password string) (*LoginResponse, string, error) {
	identity, err := s.repo.GetIdentityByProvider(ctx, "local", email, tenantID)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if identity.CredentialHash == nil || !verifyPassword(password, *identity.CredentialHash) {
		return nil, "", ErrInvalidCredentials
	}
	user, err := s.repo.GetUserByID(ctx, identity.UserID)
	if err != nil {
		return nil, "", ErrUserNotFound
	}
	accessToken, err := s.jwtManager.Issue(user.ID, tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("issue token: %w", err)
	}
	rawRefresh, err := generateRefreshToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate refresh token: %w", err)
	}
	tokenHash := hashRefreshToken(rawRefresh)
	expiresAt := time.Now().Add(s.refreshTTL)
	if _, err = s.repo.CreateSession(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return nil, "", fmt.Errorf("create session: %w", err)
	}
	return &LoginResponse{
		AccessToken: accessToken,
		ExpiresIn:   int(s.jwtManager.expiry.Seconds()),
	}, rawRefresh, nil
}

// Refresh rotates a refresh token: revokes old session, issues new access + refresh tokens.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (string, string, error) {
	hash := hashRefreshToken(rawRefreshToken)
	session, tenantID, err := s.repo.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return "", "", ErrSessionExpired
	}
	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return "", "", fmt.Errorf("revoke old session: %w", err)
	}
	accessToken, err := s.jwtManager.Issue(session.UserID, tenantID)
	if err != nil {
		return "", "", fmt.Errorf("issue token: %w", err)
	}
	newRaw, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	newHash := hashRefreshToken(newRaw)
	if _, err = s.repo.CreateSession(ctx, session.UserID, newHash, time.Now().Add(s.refreshTTL)); err != nil {
		return "", "", fmt.Errorf("create new session: %w", err)
	}
	return accessToken, newRaw, nil
}

// Logout revokes the session identified by the raw refresh token.
func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	hash := hashRefreshToken(rawRefreshToken)
	session, _, err := s.repo.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return nil // already gone
	}
	return s.repo.RevokeSession(ctx, session.ID)
}

// HasPermission checks if a user has a given permission, using the in-memory cache.
func (s *Service) HasPermission(ctx context.Context, userID uuid.UUID, permKey string) bool {
	perms, ok := s.cache.Get(userID)
	if !ok {
		var err error
		perms, err = s.repo.GetUserPermissions(ctx, userID)
		if err != nil {
			return false
		}
		s.cache.Set(userID, perms)
	}
	for _, p := range perms {
		if p == permKey {
			return true
		}
	}
	return false
}

// VerifyToken validates a JWT and returns its claims.
func (s *Service) VerifyToken(token string) (*TokenClaims, error) {
	return s.jwtManager.Verify(token)
}

// SyncPermissions writes all registry permissions to the DB at startup.
func (s *Service) SyncPermissions(ctx context.Context) error {
	for _, p := range GlobalRegistry.List() {
		if _, err := s.repo.UpsertPermission(ctx, p.Key, p.Name, p.Description); err != nil {
			return fmt.Errorf("upsert permission %q: %w", p.Key, err)
		}
	}
	return nil
}

func (s *Service) GetUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	return s.repo.GetUserByEmail(ctx, tenantID, email)
}

func (s *Service) CreateRole(ctx context.Context, tenantID uuid.UUID, name, slug string) (*Role, error) {
	return s.repo.CreateRole(ctx, tenantID, name, slug)
}

func (s *Service) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]Role, error) {
	return s.repo.ListRoles(ctx, tenantID)
}

func (s *Service) AssignPermissionsToRole(ctx context.Context, roleID uuid.UUID, permIDs []uuid.UUID) error {
	return s.repo.AssignPermissionsToRole(ctx, roleID, permIDs)
}

func (s *Service) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return s.repo.AssignRoleToUser(ctx, userID, roleID)
}

func (s *Service) UpsertPermission(ctx context.Context, key, name, description string) (*Permission, error) {
	return s.repo.UpsertPermission(ctx, key, name, description)
}

// --- crypto helpers ---

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return base64.RawStdEncoding.EncodeToString(salt) + "." + base64.RawStdEncoding.EncodeToString(hash), nil
}

func verifyPassword(password, encoded string) bool {
	parts := splitOnDot(encoded)
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return constEq(actual, expected)
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h)
}

func splitOnDot(s string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func constEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
