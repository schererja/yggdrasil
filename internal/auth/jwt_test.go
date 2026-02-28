package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWT_IssueAndVerify(t *testing.T) {
	mgr := auth.NewJWTManager("supersecretkey-at-least-32-chars!!", 15*time.Minute)
	userID := uuid.New()
	tenantID := uuid.New()
	token, err := mgr.Issue(userID, tenantID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	claims, err := mgr.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, tenantID, claims.TenantID)
}

func TestJWT_Expired(t *testing.T) {
	mgr := auth.NewJWTManager("supersecretkey-at-least-32-chars!!", -1*time.Second)
	token, err := mgr.Issue(uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = mgr.Verify(token)
	assert.Error(t, err)
}

func TestJWT_InvalidSignature(t *testing.T) {
	mgr1 := auth.NewJWTManager("supersecretkey-at-least-32-chars!!", 15*time.Minute)
	mgr2 := auth.NewJWTManager("a-completely-different-secret-key!", 15*time.Minute)
	token, err := mgr1.Issue(uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = mgr2.Verify(token)
	assert.Error(t, err)
}
