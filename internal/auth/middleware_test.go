package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestRequireAuth_ValidToken(t *testing.T) {
	mgr := auth.NewJWTManager("testsecret-at-least-32-chars-long!", 15*time.Minute)
	token, _ := mgr.Issue(uuid.New(), uuid.New())

	middleware := auth.RequireAuthMiddleware(mgr)
	called := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAuth_MissingToken(t *testing.T) {
	mgr := auth.NewJWTManager("testsecret-at-least-32-chars-long!", 15*time.Minute)
	middleware := auth.RequireAuthMiddleware(mgr)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	mgr := auth.NewJWTManager("testsecret-at-least-32-chars-long!", 15*time.Minute)
	middleware := auth.RequireAuthMiddleware(mgr)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
