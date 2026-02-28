package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const (
	contextKeyUserID   contextKey = "userID"
	contextKeyTenantID contextKey = "tenantID"
)

// RequireAuthMiddleware validates the Bearer JWT and injects userID + tenantID into context.
func RequireAuthMiddleware(jwtManager *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeUnauthorized(w, "missing token")
				return
			}
			claims, err := jwtManager.Verify(token)
			if err != nil {
				writeUnauthorized(w, "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), contextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, contextKeyTenantID, claims.TenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission returns middleware that checks a specific permission key.
// Must be chained after RequireAuthMiddleware.
func RequirePermission(svc *Service, permKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				writeUnauthorized(w, "not authenticated")
				return
			}
			if !svc.HasPermission(r.Context(), userID, permKey) {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserIDFromContext retrieves the authenticated user ID from context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(contextKeyUserID).(uuid.UUID)
	return v, ok
}

// TenantIDFromContext retrieves the tenant ID from context.
func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(contextKeyTenantID).(uuid.UUID)
	return v, ok
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
}
