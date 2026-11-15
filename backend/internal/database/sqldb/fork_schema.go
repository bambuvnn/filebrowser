package sqldb

import (
	"database/sql"
	"fmt"
)

// createForkSchema creates tables used only by this fork (Remember Me refresh
// tokens and Trash). They are idempotent CREATE IF NOT EXISTS statements run on
// every open and deliberately do NOT bump currentSchemaVersion, so upstream's
// numbered migrations keep applying cleanly after future upstream syncs.
func createForkSchema(db *sql.DB) error {
	schema := `
	-- Long-lived "Remember Me" refresh tokens (only the SHA-256 hash is stored)
	CREATE TABLE IF NOT EXISTS refresh_tokens (
		token_hash TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		expires_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		user_agent TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
	CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

	-- Soft-deleted items moved into <source>/.trash/<username>/
	CREATE TABLE IF NOT EXISTS trash_items (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		source_name TEXT NOT NULL,
		source_path TEXT NOT NULL,
		original_path TEXT NOT NULL,
		trash_path TEXT NOT NULL,
		is_dir INTEGER NOT NULL DEFAULT 0,
		deleted_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_trash_items_username ON trash_items(username);
	CREATE INDEX IF NOT EXISTS idx_trash_items_deleted_at ON trash_items(deleted_at);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create fork schema: %w", err)
	}
	return nil
}
