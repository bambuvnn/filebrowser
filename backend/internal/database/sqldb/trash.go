package sqldb

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrTrashItemNotFound is returned when a trash item id is unknown.
var ErrTrashItemNotFound = errors.New("trash item not found")

// TrashItemRecord is a soft-deleted file or directory kept under <source>/.trash/<username>/.
type TrashItemRecord struct {
	ID           string
	Username     string
	SourceName   string // frontend source name
	SourcePath   string // absolute root path of the source on disk at deletion time
	OriginalPath string // index path (relative to source root) before deletion
	TrashPath    string // absolute path on disk inside the .trash folder
	IsDir        bool
	DeletedAt    int64 // unix seconds
}

const trashColumns = `id, username, source_name, source_path, original_path, trash_path, is_dir, deleted_at`

func scanTrashItem(sc interface{ Scan(dest ...any) error }) (TrashItemRecord, error) {
	var t TrashItemRecord
	err := sc.Scan(&t.ID, &t.Username, &t.SourceName, &t.SourcePath, &t.OriginalPath, &t.TrashPath, &t.IsDir, &t.DeletedAt)
	return t, err
}

func (s *SQLStore) queryTrashItems(query string, args ...any) ([]TrashItemRecord, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query trash items: %w", err)
	}
	defer rows.Close()
	items := []TrashItemRecord{}
	for rows.Next() {
		t, err := scanTrashItem(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trash item: %w", err)
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating trash items: %w", err)
	}
	return items, nil
}

// SaveTrashItem inserts or replaces a trash item record.
func (s *SQLStore) SaveTrashItem(t TrashItemRecord) error {
	query := `INSERT OR REPLACE INTO trash_items (` + trashColumns + `) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := s.db.Exec(query, t.ID, t.Username, t.SourceName, t.SourcePath, t.OriginalPath, t.TrashPath, t.IsDir, t.DeletedAt); err != nil {
		return fmt.Errorf("failed to save trash item: %w", err)
	}
	return nil
}

// GetTrashItem returns one trash item by id.
func (s *SQLStore) GetTrashItem(id string) (TrashItemRecord, error) {
	t, err := scanTrashItem(s.db.QueryRow(`SELECT `+trashColumns+` FROM trash_items WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return TrashItemRecord{}, ErrTrashItemNotFound
	}
	if err != nil {
		return TrashItemRecord{}, fmt.Errorf("failed to get trash item: %w", err)
	}
	return t, nil
}

// ListTrashItemsByUser returns a user's trash items, newest first.
func (s *SQLStore) ListTrashItemsByUser(username string) ([]TrashItemRecord, error) {
	return s.queryTrashItems(`SELECT `+trashColumns+` FROM trash_items WHERE username = ? ORDER BY deleted_at DESC`, username)
}

// ListAllTrashItems returns every trash item, newest first.
func (s *SQLStore) ListAllTrashItems() ([]TrashItemRecord, error) {
	return s.queryTrashItems(`SELECT ` + trashColumns + ` FROM trash_items ORDER BY deleted_at DESC`)
}

// ListTrashItemsDeletedBefore returns items whose deleted_at is at or before cutoff.
func (s *SQLStore) ListTrashItemsDeletedBefore(cutoff int64) ([]TrashItemRecord, error) {
	return s.queryTrashItems(`SELECT `+trashColumns+` FROM trash_items WHERE deleted_at <= ?`, cutoff)
}

// DeleteTrashItem removes a trash item record (does not touch the filesystem).
func (s *SQLStore) DeleteTrashItem(id string) error {
	if _, err := s.db.Exec(`DELETE FROM trash_items WHERE id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete trash item: %w", err)
	}
	return nil
}
