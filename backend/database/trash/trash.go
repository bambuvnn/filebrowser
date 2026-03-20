package trash

import "time"

// TrashItem represents a soft-deleted file or directory in the trash.
type TrashItem struct {
	ID           string `storm:"id"`
	Username     string `storm:"index"`
	SourceName   string // name of the source (frontend source name)
	SourcePath   string // absolute root path of the source on disk
	OriginalPath string // path relative to the source root before deletion
	TrashPath    string // absolute path on disk inside .trash/ folder
	IsDir        bool
	DeletedAt    int64 // Unix timestamp (seconds)
}

// ExpiresAt returns the time when this item will be auto-purged (7 days after deletion).
func (t *TrashItem) ExpiresAt() time.Time {
	return time.Unix(t.DeletedAt, 0).Add(7 * 24 * time.Hour)
}

// Backend is the interface for trash persistence.
type Backend interface {
	Save(item *TrashItem) error
	GetByUser(username string) ([]*TrashItem, error)
	GetAll() ([]*TrashItem, error)
	GetByID(id string) (*TrashItem, error)
	Delete(id string) error
	// GetExpired returns all items whose DeletedAt is older than the given duration.
	GetExpired(olderThan time.Duration) ([]*TrashItem, error)
}

// Storage wraps the Backend to provide trash operations.
type Storage struct {
	backend Backend
}

// NewStorage creates a new trash Storage with the given backend.
func NewStorage(backend Backend) *Storage {
	return &Storage{backend: backend}
}

// Save persists a new trash item.
func (s *Storage) Save(item *TrashItem) error {
	return s.backend.Save(item)
}

// GetByUser returns all trash items belonging to the given user.
func (s *Storage) GetByUser(username string) ([]*TrashItem, error) {
	return s.backend.GetByUser(username)
}

// GetAll returns all trash items across all users (admin only).
func (s *Storage) GetAll() ([]*TrashItem, error) {
	return s.backend.GetAll()
}

// GetByID returns a single trash item by its ID.
func (s *Storage) GetByID(id string) (*TrashItem, error) {
	return s.backend.GetByID(id)
}

// Delete removes a trash item record from the database (does not touch the filesystem).
func (s *Storage) Delete(id string) error {
	return s.backend.Delete(id)
}

// GetExpired returns all trash items older than the specified duration.
func (s *Storage) GetExpired(olderThan time.Duration) ([]*TrashItem, error) {
	return s.backend.GetExpired(olderThan)
}
