package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// MakeSignedNonExpiringTokenAPI mints a named API token with no exp claim.
// It is kept separate from MakeSignedTokenAPI on purpose: session tokens must
// always expire, even if tokenExpirationHours is misconfigured to 0. The access
// layer records expiresAt=0 for such tokens, which it treats as "never expires";
// they stay valid until the user deletes them (or the user is deleted).
func MakeSignedNonExpiringTokenAPI(user *users.User, name string, perms users.Permissions, minimal bool) (string, users.AuthToken, error) {
	if _, ok := user.Tokens[name]; ok {
		return "", users.AuthToken{}, fmt.Errorf("key already exists with same name %v ", name)
	}
	id, err := utils.RandomHex(16)
	if err != nil {
		return "", users.AuthToken{}, fmt.Errorf("generate token id: %w", err)
	}
	claim := users.AuthToken{
		MinimalAuthToken: users.MinimalAuthToken{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:       id,
				IssuedAt: jwt.NewNumericDate(time.Now()),
				Issuer:   FB_ISSUER,
			},
		},
	}
	claim.Minimal = &minimal
	if !minimal {
		claim.Permissions = users.SanitizeTokenPermissions(perms)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.Claims(claim.MinimalAuthToken))
	tokenString, err := token.SignedString([]byte(settings.Config.Auth.Key))
	if err != nil {
		return "", users.AuthToken{}, err
	}
	return tokenString, claim, nil
}
