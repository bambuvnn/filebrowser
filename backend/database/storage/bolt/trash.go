package bolt

import (
	"time"

	storm "github.com/asdine/storm/v3"

	"github.com/gtsteffaniak/filebrowser/backend/database/trash"
)

type trashBackend struct {
	db *storm.DB
}

func (t *trashBackend) Save(item *trash.TrashItem) error {
	return t.db.Save(item)
}

func (t *trashBackend) GetByUser(username string) ([]*trash.TrashItem, error) {
	var items []*trash.TrashItem
	if err := t.db.Find("Username", username, &items); err != nil {
		if err == storm.ErrNotFound {
			return []*trash.TrashItem{}, nil
		}
		return nil, err
	}
	return items, nil
}

func (t *trashBackend) GetAll() ([]*trash.TrashItem, error) {
	var items []*trash.TrashItem
	if err := t.db.All(&items); err != nil {
		if err == storm.ErrNotFound {
			return []*trash.TrashItem{}, nil
		}
		return nil, err
	}
	return items, nil
}

func (t *trashBackend) GetByID(id string) (*trash.TrashItem, error) {
	var item trash.TrashItem
	if err := t.db.One("ID", id, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (t *trashBackend) Delete(id string) error {
	var item trash.TrashItem
	if err := t.db.One("ID", id, &item); err != nil {
		return err
	}
	return t.db.DeleteStruct(&item)
}

func (t *trashBackend) GetExpired(olderThan time.Duration) ([]*trash.TrashItem, error) {
	var all []*trash.TrashItem
	if err := t.db.All(&all); err != nil {
		if err == storm.ErrNotFound {
			return []*trash.TrashItem{}, nil
		}
		return nil, err
	}

	cutoff := time.Now().Add(-olderThan).Unix()
	var expired []*trash.TrashItem
	for _, item := range all {
		if item.DeletedAt <= cutoff {
			expired = append(expired, item)
		}
	}
	return expired, nil
}
