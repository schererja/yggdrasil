package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	authdb "github.com/schererja/yggdrasil/internal/auth/db"
)

type Repository struct {
	db      *sql.DB
	queries *authdb.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, queries: authdb.New(db)}
}

func (r *Repository) GetUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	row, err := r.queries.GetUserByEmail(ctx, authdb.GetUserByEmailParams{
		TenantID: tenantID,
		Email:    email,
	})
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return userFromDB(row), nil
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return userFromDB(row), nil
}

func (r *Repository) CreateUser(ctx context.Context, tenantID uuid.UUID, email string, firstName, lastName *string) (*User, error) {
	var fn, ln sql.NullString
	if firstName != nil {
		fn = sql.NullString{String: *firstName, Valid: true}
	}
	if lastName != nil {
		ln = sql.NullString{String: *lastName, Valid: true}
	}
	row, err := r.queries.CreateUser(ctx, authdb.CreateUserParams{
		TenantID:  tenantID,
		Email:     email,
		FirstName: fn,
		LastName:  ln,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return userFromDB(row), nil
}

func (r *Repository) GetIdentityByProvider(ctx context.Context, provider, providerID string, tenantID uuid.UUID) (*Identity, error) {
	row, err := r.queries.GetIdentityByProvider(ctx, authdb.GetIdentityByProviderParams{
		Provider:   provider,
		ProviderID: providerID,
		TenantID:   tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("get identity: %w", err)
	}
	return identityFromDB(row), nil
}

func (r *Repository) CreateIdentity(ctx context.Context, userID uuid.UUID, provider, providerID string, credentialHash *string) (*Identity, error) {
	var hash sql.NullString
	if credentialHash != nil {
		hash = sql.NullString{String: *credentialHash, Valid: true}
	}
	row, err := r.queries.CreateIdentity(ctx, authdb.CreateIdentityParams{
		UserID:         userID,
		Provider:       provider,
		ProviderID:     providerID,
		CredentialHash: hash,
	})
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	return identityFromDB(row), nil
}

func (r *Repository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.queries.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user permissions: %w", err)
	}
	return rows, nil
}

func (r *Repository) CreateRole(ctx context.Context, tenantID uuid.UUID, name, slug string) (*Role, error) {
	row, err := r.queries.CreateRole(ctx, authdb.CreateRoleParams{
		TenantID: tenantID,
		Name:     name,
		Slug:     slug,
	})
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	return roleFromDB(row), nil
}

func (r *Repository) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]Role, error) {
	rows, err := r.queries.ListRoles(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	out := make([]Role, len(rows))
	for i, row := range rows {
		out[i] = *roleFromDB(row)
	}
	return out, nil
}

func (r *Repository) AssignPermissionsToRole(ctx context.Context, roleID uuid.UUID, permIDs []uuid.UUID) error {
	return r.queries.AssignPermissionsToRole(ctx, authdb.AssignPermissionsToRoleParams{
		RoleID:  roleID,
		Column2: permIDs,
	})
}

func (r *Repository) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return r.queries.AssignRoleToUser(ctx, authdb.AssignRoleToUserParams{
		UserID: userID,
		RoleID: roleID,
	})
}

func (r *Repository) UpsertPermission(ctx context.Context, key, name, description string) (*Permission, error) {
	row, err := r.queries.UpsertPermission(ctx, authdb.UpsertPermissionParams{
		Key:         key,
		Name:        name,
		Description: sql.NullString{String: description, Valid: description != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("upsert permission: %w", err)
	}
	desc := ""
	if row.Description.Valid {
		desc = row.Description.String
	}
	return &Permission{Key: row.Key, Name: row.Name, Description: desc}, nil
}

func (r *Repository) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*Session, error) {
	row, err := r.queries.CreateSession(ctx, authdb.CreateSessionParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sessionFromDB(row), nil
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, hash string) (*Session, uuid.UUID, error) {
	row, err := r.queries.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("get session: %w", err)
	}
	session := &Session{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
	}
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		session.RevokedAt = &t
	}
	return session, row.TenantID, nil
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	return r.queries.RevokeSession(ctx, sessionID)
}

func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	return r.queries.RevokeAllUserSessions(ctx, userID)
}

// --- mapping helpers ---

func userFromDB(row authdb.User) *User {
	u := &User{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Email:     row.Email,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if row.FirstName.Valid {
		u.FirstName = &row.FirstName.String
	}
	if row.LastName.Valid {
		u.LastName = &row.LastName.String
	}
	return u
}

func identityFromDB(row authdb.Identity) *Identity {
	i := &Identity{
		ID:         row.ID,
		UserID:     row.UserID,
		Provider:   row.Provider,
		ProviderID: row.ProviderID,
		CreatedAt:  row.CreatedAt,
	}
	if row.CredentialHash.Valid {
		i.CredentialHash = &row.CredentialHash.String
	}
	return i
}

func roleFromDB(row authdb.Role) *Role {
	return &Role{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Name:      row.Name,
		Slug:      row.Slug,
		CreatedAt: row.CreatedAt,
	}
}

func sessionFromDB(row authdb.UserSession) *Session {
	s := &Session{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
	}
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		s.RevokedAt = &t
	}
	return s
}
