package web

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestNonExpiringApiTokenAccepted(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "never-expire-user", users.Permissions{Admin: true, Api: true})
	tokenString, meta, err := auth.MakeSignedNonExpiringTokenAPI(user, "forever", users.Permissions{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if meta.RegisteredClaims.ExpiresAt != nil {
		t.Fatalf("non-expiring token must not carry exp")
	}
	meta.Name = "forever"
	meta.Token = tokenString
	if err := state.AddUserToken(user.Username, meta); err != nil {
		t.Fatal(err)
	}
	if err := state.AddApiToken(tokenString, user.ID); err != nil {
		t.Fatal(err)
	}

	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusOK {
		t.Fatalf("registered non-expiring API token: got %d, want 200", rec.Code)
	}
}

// A session token (registered as session) without exp must still be rejected,
// so the never-expire path cannot be used to mint immortal sessions.
func TestSessionTokenWithoutExpRejected(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "no-exp-session-user", users.Permissions{Admin: true})
	claims := jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now()), Issuer: auth.FB_ISSUER, ID: "abc"}
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(settings.Config.Auth.Key))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.RegisterSessionToken(tokenString, user.ID); err != nil {
		t.Fatal(err)
	}
	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("session token without exp: got %d, want 401", rec.Code)
	}
}
