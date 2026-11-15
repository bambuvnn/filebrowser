package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func refreshCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == refreshCookieName {
			return c
		}
	}
	return nil
}

func doRefresh(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", http.NoBody)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: token})
	}
	status, err := refreshHandler(rec, req, &Context{})
	if status != 0 {
		rec.Code = status
	}
	_ = err
	return rec
}

func setupRememberMe(t *testing.T, username string) *users.User {
	t.Helper()
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })
	return createTokenAuthUser(t, username, users.Permissions{})
}

func TestRememberMe_RefreshRotatesToken(t *testing.T) {
	user := setupRememberMe(t, "remember-rotate")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", http.NoBody)
	token, err := issueRefreshToken(rec, req, user)
	if err != nil {
		t.Fatal(err)
	}
	c := refreshCookieFrom(t, rec)
	if c == nil || c.Value != token || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("refresh cookie not set correctly: %+v", c)
	}

	first := doRefresh(t, token)
	if first.Code != http.StatusOK {
		t.Fatalf("refresh with valid token: got %d body=%q", first.Code, first.Body.String())
	}
	if first.Body.Len() == 0 {
		t.Fatal("refresh should return a new session token")
	}
	rotated := refreshCookieFrom(t, first)
	if rotated == nil || rotated.Value == "" || rotated.Value == token {
		t.Fatalf("refresh must rotate the refresh token, got %+v", rotated)
	}

	// The old token is single-use.
	if again := doRefresh(t, token); again.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh token: got %d, want 401", again.Code)
	}
	// The rotated token works.
	if next := doRefresh(t, rotated.Value); next.Code != http.StatusOK {
		t.Fatalf("rotated refresh token: got %d, want 200", next.Code)
	}
}

func TestRememberMe_MissingOrBogusTokenRejected(t *testing.T) {
	setupRememberMe(t, "remember-bogus")
	if rec := doRefresh(t, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: got %d, want 401", rec.Code)
	}
	if rec := doRefresh(t, "not-a-real-token"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bogus cookie: got %d, want 401", rec.Code)
	}
}

func TestRememberMe_PasswordChangeRevokes(t *testing.T) {
	user := setupRememberMe(t, "remember-pwchange")
	token, err := issueRefreshToken(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", http.NoBody), user)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := state.GetUserByUsername(user.Username)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.UpdateUser(&stored, "A-new-Passw0rd-123!", "password"); err != nil {
		t.Fatal(err)
	}
	if rec := doRefresh(t, token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after password change: got %d, want 401", rec.Code)
	}
}

func TestRememberMe_UserDeletionRevokes(t *testing.T) {
	user := setupRememberMe(t, "remember-deleted")
	token, err := issueRefreshToken(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", http.NoBody), user)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.DeleteUser(user.ID); err != nil {
		t.Fatal(err)
	}
	if rec := doRefresh(t, token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after user deletion: got %d, want 401", rec.Code)
	}
}

func TestRememberMe_LogoutRevokesAllDevices(t *testing.T) {
	user := setupRememberMe(t, "remember-logout")
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	a, err := issueRefreshToken(httptest.NewRecorder(), req, user)
	if err != nil {
		t.Fatal(err)
	}
	b, err := issueRefreshToken(httptest.NewRecorder(), req, user)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", http.NoBody)
	logoutReq.AddCookie(&http.Cookie{Name: refreshCookieName, Value: a})
	if _, err := logoutHandler(rec, logoutReq, &Context{User: user}); err != nil {
		t.Fatal(err)
	}
	if c := refreshCookieFrom(t, rec); c == nil || c.MaxAge >= 0 {
		t.Fatalf("logout must clear the refresh cookie, got %+v", c)
	}
	for _, tok := range []string{a, b} {
		if r := doRefresh(t, tok); r.Code != http.StatusUnauthorized {
			t.Fatalf("refresh after logout: got %d, want 401", r.Code)
		}
	}
}
