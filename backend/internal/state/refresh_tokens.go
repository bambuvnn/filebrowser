package state

import (
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// ErrRefreshTokenInvalid is returned when a refresh token is unknown, already used, or expired.
var ErrRefreshTokenInvalid = sqldb.ErrRefreshTokenNotFound

// StoreRefreshToken persists the SHA-256 hash of a "Remember Me" refresh token.
// The raw token is never stored.
func StoreRefreshToken(tokenString string, userID uint64, expiresAt time.Time, userAgent string) error {
	if sqlDb == nil {
		return fmt.Errorf("database not available")
	}
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	return sqlDb.SaveRefreshToken(sqldb.RefreshTokenRecord{
		TokenHash: utils.HashSHA256(tokenString),
		UserID:    userID,
		ExpiresAt: expiresAt.Unix(),
		CreatedAt: time.Now().Unix(),
		UserAgent: userAgent,
	})
}

// ConsumeRefreshToken validates a refresh token and removes it in one step
// (rotation): each token can be redeemed exactly once. Returns the owner id.
func ConsumeRefreshToken(tokenString string) (uint64, error) {
	if sqlDb == nil {
		return 0, fmt.Errorf("database not available")
	}
	rec, err := sqlDb.ConsumeRefreshToken(utils.HashSHA256(tokenString), time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return rec.UserID, nil
}

// RevokeRefreshToken removes a single refresh token.
func RevokeRefreshToken(tokenString string) error {
	if sqlDb == nil {
		return nil
	}
	return sqlDb.DeleteRefreshToken(utils.HashSHA256(tokenString))
}

// RevokeAllRefreshTokensForUser removes every refresh token for a user (all devices).
func RevokeAllRefreshTokensForUser(userID uint64) error {
	if sqlDb == nil || userID == 0 {
		return nil
	}
	return sqlDb.DeleteRefreshTokensByUserID(userID)
}

// revokeRefreshTokensBestEffort is used from user lifecycle paths (delete,
// password change) where failure must not abort the main operation.
func revokeRefreshTokensBestEffort(userID uint64, reason string) {
	if err := RevokeAllRefreshTokensForUser(userID); err != nil {
		logger.Errorf("failed to revoke refresh tokens on %s for user %d: %v", reason, userID, err)
	}
}

// PruneExpiredRefreshTokens deletes refresh tokens past their expiry.
func PruneExpiredRefreshTokens() (int64, error) {
	if sqlDb == nil {
		return 0, nil
	}
	return sqlDb.DeleteExpiredRefreshTokens(time.Now().Unix())
}
