package sqldb

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// ErrRefreshTokenNotFound is returned when a refresh token hash is unknown or expired.
var ErrRefreshTokenNotFound = errors.New("refresh token not found")

// RefreshTokenRecord is a persisted "Remember Me" refresh token.
type RefreshTokenRecord struct {
	TokenHash string
	UserID    uint64
	ExpiresAt int64 // unix seconds
	CreatedAt int64 // unix seconds
	UserAgent string
}

// SaveRefreshToken stores a refresh token hash.
func (s *SQLStore) SaveRefreshToken(rec RefreshTokenRecord) error {
	query := `INSERT OR REPLACE INTO refresh_tokens (token_hash, user_id, expires_at, created_at, user_agent) VALUES (?, ?, ?, ?, ?)`
	if _, err := s.db.Exec(query, rec.TokenHash, strconv.FormatUint(rec.UserID, 10), rec.ExpiresAt, rec.CreatedAt, rec.UserAgent); err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}
	return nil
}

// ConsumeRefreshToken atomically deletes a refresh token and returns it if it
// existed and had not expired at `now`. Deleting before returning guarantees a
// token can be redeemed at most once, even under concurrent refresh requests.
func (s *SQLStore) ConsumeRefreshToken(tokenHash string, now int64) (RefreshTokenRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return RefreshTokenRecord{}, fmt.Errorf("begin refresh token transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var rec RefreshTokenRecord
	var idStr string
	err = tx.QueryRow(`SELECT token_hash, user_id, expires_at, created_at, user_agent FROM refresh_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&rec.TokenHash, &idStr, &rec.ExpiresAt, &rec.CreatedAt, &rec.UserAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return RefreshTokenRecord{}, ErrRefreshTokenNotFound
	}
	if err != nil {
		return RefreshTokenRecord{}, fmt.Errorf("failed to load refresh token: %w", err)
	}
	res, err := tx.Exec(`DELETE FROM refresh_tokens WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return RefreshTokenRecord{}, fmt.Errorf("failed to delete refresh token: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Another request redeemed it between SELECT and DELETE.
		return RefreshTokenRecord{}, ErrRefreshTokenNotFound
	}
	if err := tx.Commit(); err != nil {
		return RefreshTokenRecord{}, fmt.Errorf("commit refresh token transaction: %w", err)
	}
	if rec.ExpiresAt <= now {
		return RefreshTokenRecord{}, ErrRefreshTokenNotFound
	}
	rec.UserID, err = strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return RefreshTokenRecord{}, fmt.Errorf("invalid user_id in refresh_tokens: %w", err)
	}
	return rec, nil
}

// DeleteRefreshToken removes one refresh token hash.
func (s *SQLStore) DeleteRefreshToken(tokenHash string) error {
	if _, err := s.db.Exec(`DELETE FROM refresh_tokens WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}
	return nil
}

// DeleteRefreshTokensByUserID removes every refresh token owned by a user (all devices).
func (s *SQLStore) DeleteRefreshTokensByUserID(userID uint64) error {
	if _, err := s.db.Exec(`DELETE FROM refresh_tokens WHERE user_id = ?`, strconv.FormatUint(userID, 10)); err != nil {
		return fmt.Errorf("failed to delete refresh tokens by user: %w", err)
	}
	return nil
}

// DeleteExpiredRefreshTokens removes refresh tokens that expired at or before `now`.
func (s *SQLStore) DeleteExpiredRefreshTokens(now int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM refresh_tokens WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired refresh tokens: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
