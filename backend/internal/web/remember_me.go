package web

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// "Remember Me": an opaque, long-lived refresh token in an HttpOnly cookie that
// can mint a fresh session token after the short session JWT expires. Only its
// SHA-256 hash is stored server-side, and each token is single-use (rotated).

const (
	refreshCookieName = "filebrowser_quantum_refresh"
	refreshTokenTTL   = 90 * 24 * time.Hour
)

// rememberMeRequested reports whether the login request opted into Remember Me.
func rememberMeRequested(r *http.Request) bool {
	return r.URL.Query().Get("rememberMe") == "true"
}

// refreshCookiePath scopes the refresh cookie to the auth API so it is not sent
// with every request (only refresh and logout need it).
func refreshCookiePath() string {
	return settings.Config.Http.BaseURL + "api/auth/"
}

func refreshCookie(r *http.Request, value string, expires time.Time, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Domain:   sessionCookieDomain(r),
		Path:     refreshCookiePath(),
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   requestScheme(r) == "https",
		Expires:  expires,
		MaxAge:   maxAge,
	}
}

func clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, refreshCookie(r, "", time.Unix(0, 0), -1))
}

// issueRefreshToken creates, stores and sets a new refresh token cookie.
// It must run before anything writes the response body. Returns the raw token
// so callers can revoke it if the rest of the login fails.
func issueRefreshToken(w http.ResponseWriter, r *http.Request, user *users.User) (string, error) {
	token, err := utils.RandomHex(32)
	if err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	expires := time.Now().Add(refreshTokenTTL)
	if err := state.StoreRefreshToken(token, user.ID, expires, r.Header.Get("User-Agent")); err != nil {
		return "", err
	}
	http.SetCookie(w, refreshCookie(r, token, expires, 0))
	return token, nil
}

// issueRefreshTokenBestEffort issues a refresh token but never fails the
// surrounding login: without it the user simply is not remembered.
func issueRefreshTokenBestEffort(w http.ResponseWriter, r *http.Request, user *users.User) string {
	token, err := issueRefreshToken(w, r, user)
	if err != nil {
		logger.Errorf("Failed to issue refresh token for %s: %v", user.Username, err)
		return ""
	}
	return token
}

// revokeRefreshTokenBestEffort undoes an issued token when the login that
// created it did not complete.
func revokeRefreshTokenBestEffort(token string) {
	if token == "" {
		return
	}
	if err := state.RevokeRefreshToken(token); err != nil {
		logger.Errorf("Failed to revoke refresh token: %v", err)
	}
}

// refreshHandler exchanges a valid refresh token cookie for a new session.
// @Summary Refresh session using refresh token
// @Description Uses the HttpOnly "Remember Me" refresh token cookie to issue a new session token without re-authentication. The refresh token is rotated on every use.
// @Tags Auth
// @Produce plain
// @Success 200 {string} string "New JWT token for authentication"
// @Failure 401 {object} map[string]string "Unauthorized - missing, invalid or expired refresh token"
// @Failure 403 {object} map[string]string "Forbidden - password change required"
// @Router /api/auth/refresh [post]
func refreshHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	c, err := r.Cookie(refreshCookieName)
	if err != nil || c.Value == "" {
		return http.StatusUnauthorized, errors.ErrUnauthorized
	}
	userID, err := state.ConsumeRefreshToken(c.Value)
	if err != nil {
		clearRefreshCookie(w, r)
		if !stderrors.Is(err, state.ErrRefreshTokenInvalid) {
			logger.Errorf("refresh token lookup failed: %v", err)
		}
		return http.StatusUnauthorized, errors.ErrUnauthorized
	}
	user, err := state.GetUserByID(userID)
	if err != nil || user.Username == "" {
		clearRefreshCookie(w, r)
		return http.StatusUnauthorized, errors.ErrUnauthorized
	}
	// Same gate as loginHandler: a pending forced password change must be done interactively.
	if user.LoginMethod == users.LoginMethodPassword && user.RequirePasswordChange {
		clearRefreshCookie(w, r)
		return http.StatusForbidden, errors.ErrPasswordChangeRequired
	}

	newRefresh := issueRefreshTokenBestEffort(w, r, &user)
	status, err := printToken(w, r, &user, "")
	if err != nil || status != 0 {
		revokeRefreshTokenBestEffort(newRefresh)
		return status, err
	}
	return 0, nil
}
