package state

import (
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
)

// TrashItem is a soft-deleted file or directory (see sqldb.TrashItemRecord).
type TrashItem = sqldb.TrashItemRecord

// ErrTrashItemNotFound is returned when a trash item id is unknown.
var ErrTrashItemNotFound = sqldb.ErrTrashItemNotFound

func trashStore() (*sqldb.SQLStore, error) {
	if sqlDb == nil {
		return nil, fmt.Errorf("database not available")
	}
	return sqlDb, nil
}

// SaveTrashItem persists a trash item record.
func SaveTrashItem(item TrashItem) error {
	s, err := trashStore()
	if err != nil {
		return err
	}
	return s.SaveTrashItem(item)
}

// GetTrashItem returns a trash item by id.
func GetTrashItem(id string) (TrashItem, error) {
	s, err := trashStore()
	if err != nil {
		return TrashItem{}, err
	}
	return s.GetTrashItem(id)
}

// ListTrashItemsByUser returns a user's trash items, newest first.
func ListTrashItemsByUser(username string) ([]TrashItem, error) {
	s, err := trashStore()
	if err != nil {
		return nil, err
	}
	return s.ListTrashItemsByUser(username)
}

// ListAllTrashItems returns all users' trash items, newest first.
func ListAllTrashItems() ([]TrashItem, error) {
	s, err := trashStore()
	if err != nil {
		return nil, err
	}
	return s.ListAllTrashItems()
}

// ListExpiredTrashItems returns items deleted longer ago than retention.
func ListExpiredTrashItems(retention time.Duration) ([]TrashItem, error) {
	s, err := trashStore()
	if err != nil {
		return nil, err
	}
	return s.ListTrashItemsDeletedBefore(time.Now().Add(-retention).Unix())
}

// DeleteTrashItem removes a trash item record (filesystem untouched).
func DeleteTrashItem(id string) error {
	s, err := trashStore()
	if err != nil {
		return err
	}
	return s.DeleteTrashItem(id)
}
