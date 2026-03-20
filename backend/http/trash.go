package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bambuvnn/filebrowser/backend/adapters/fs/fileutils"
	"github.com/bambuvnn/filebrowser/backend/database/trash"
	"github.com/bambuvnn/filebrowser/backend/indexing"
	"github.com/google/uuid"
	"github.com/gtsteffaniak/go-logger/logger"
)

const trashFolderName = ".trash"

// trashDirForUser returns the absolute path of the user's trash directory within a source.
// Path: <sourceRoot>/.trash/<username>/
func trashDirForUser(sourceRoot, username string) string {
	return filepath.Join(sourceRoot, trashFolderName, username)
}

// TrashItemResponse is the JSON representation of a trash item for the API.
type TrashItemResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Username     string `json:"username,omitempty"` // only shown to admin
	SourceName   string `json:"sourceName"`
	OriginalPath string `json:"originalPath"`
	IsDir        bool   `json:"isDir"`
	DeletedAt    string `json:"deletedAt"` // RFC3339
	ExpiresAt    string `json:"expiresAt"` // RFC3339
}

func toTrashItemResponse(item *trash.TrashItem, isAdmin bool) TrashItemResponse {
	resp := TrashItemResponse{
		ID:           item.ID,
		Name:         filepath.Base(item.OriginalPath),
		SourceName:   item.SourceName,
		OriginalPath: item.OriginalPath,
		IsDir:        item.IsDir,
		DeletedAt:    time.Unix(item.DeletedAt, 0).Format(time.RFC3339),
		ExpiresAt:    item.ExpiresAt().Format(time.RFC3339),
	}
	if isAdmin {
		resp.Username = item.Username
	}
	return resp
}

// trashListHandler lists trash items for the current user (or all users if admin + ?all=true).
// @Summary List trash items
// @Tags Trash
// @Produce json
// @Param all query bool false "Admin only: list all users' trash items"
// @Success 200 {array} TrashItemResponse
// @Router /api/trash [get]
func trashListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	isAdmin := d.user.Permissions.Admin
	all := r.URL.Query().Get("all") == "true"

	var items []*trash.TrashItem
	var err error

	if all && isAdmin {
		items, err = store.Trash.GetAll()
	} else {
		items, err = store.Trash.GetByUser(d.user.Username)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	result := make([]TrashItemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toTrashItemResponse(item, isAdmin))
	}
	return renderJSON(w, r, result)
}

// TrashMoveRequest is the request body for moving items to trash.
type TrashMoveRequest struct {
	Items []TrashMoveItem `json:"items"`
}

// TrashMoveItem is a single item in a trash move request.
type TrashMoveItem struct {
	Source string `json:"source"`
	Path   string `json:"path"` // full index path (including user scope)
}

// moveItemsToTrash moves the given items into the user's .trash directory within each source.
// items[].Path should be the full index path (i.e. already includes user scope).
func moveItemsToTrash(items []TrashMoveItem, username string) error {
	for _, item := range items {
		if item.Path == "" || item.Path == "/" {
			continue
		}

		idx := indexing.GetIndex(item.Source)
		if idx == nil {
			logger.Errorf("trash move: source %q not found", item.Source)
			continue
		}

		realPath, isDir, err := idx.GetRealPath(item.Path)
		if err != nil {
			logger.Errorf("trash move: cannot resolve real path for %q: %v", item.Path, err)
			continue
		}

		sourceRoot := idx.Path
		trashDir := trashDirForUser(sourceRoot, username)
		if err := os.MkdirAll(trashDir, fileutils.PermDir); err != nil {
			logger.Errorf("trash move: cannot create trash dir %q: %v", trashDir, err)
			continue
		}

		// Build unique trash destination: uuid_originalname
		originalName := filepath.Base(realPath)
		uniqueName := uuid.New().String() + "_" + originalName
		trashPath := filepath.Join(trashDir, uniqueName)

		// Move file into trash
		if err := os.Rename(realPath, trashPath); err != nil {
			logger.Errorf("trash move: cannot move %q to %q: %v", realPath, trashPath, err)
			continue
		}

		// Persist metadata
		trashItem := &trash.TrashItem{
			ID:           uuid.New().String(),
			Username:     username,
			SourceName:   item.Source,
			SourcePath:   sourceRoot,
			OriginalPath: item.Path,
			TrashPath:    trashPath,
			IsDir:        isDir,
			DeletedAt:    time.Now().Unix(),
		}
		if err := store.Trash.Save(trashItem); err != nil {
			logger.Errorf("trash move: failed to save trash metadata for %q: %v", item.Path, err)
		}
	}
	return nil
}

// newTrashMoveBody creates an io.ReadCloser for a TrashMoveRequest JSON body.
func newTrashMoveBody(items []TrashMoveItem) io.ReadCloser {
	body, _ := json.Marshal(TrashMoveRequest{Items: items})
	return io.NopCloser(bytes.NewReader(body))
}

// trashMoveHandler moves files/folders to the user's trash directory.
// @Summary Move items to trash
// @Tags Trash
// @Accept json
// @Produce json
// @Param items body TrashMoveRequest true "Items to move to trash"
// @Success 200
// @Router /api/trash/move [post]
func trashMoveHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Delete {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to delete")
	}

	var req TrashMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}
	if len(req.Items) == 0 {
		return http.StatusBadRequest, fmt.Errorf("items array cannot be empty")
	}

	if err := moveItemsToTrash(req.Items, d.user.Username); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// TrashRestoreRequest is the request body for restoring trash items.
type TrashRestoreRequest struct {
	IDs []string `json:"ids"`
}

// trashRestoreHandler restores trash items back to their original location.
// @Summary Restore trash items
// @Tags Trash
// @Accept json
// @Produce json
// @Param ids body TrashRestoreRequest true "IDs of trash items to restore"
// @Success 200
// @Router /api/trash/restore [post]
func trashRestoreHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var req TrashRestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}
	if len(req.IDs) == 0 {
		return http.StatusBadRequest, fmt.Errorf("ids array cannot be empty")
	}

	for _, id := range req.IDs {
		item, err := store.Trash.GetByID(id)
		if err != nil {
			logger.Errorf("trash restore: item %q not found: %v", id, err)
			continue
		}
		// Authorization: non-admin can only restore their own items
		if !d.user.Permissions.Admin && item.Username != d.user.Username {
			logger.Warningf("trash restore: user %q tried to restore item owned by %q", d.user.Username, item.Username)
			continue
		}

		originalRealPath := filepath.Join(item.SourcePath, item.OriginalPath)

		// Ensure parent dir exists
		if err := os.MkdirAll(filepath.Dir(originalRealPath), fileutils.PermDir); err != nil {
			logger.Errorf("trash restore: cannot create parent dir for %q: %v", originalRealPath, err)
			continue
		}

		// Move file back to original location
		if err := os.Rename(item.TrashPath, originalRealPath); err != nil {
			logger.Errorf("trash restore: cannot move %q to %q: %v", item.TrashPath, originalRealPath, err)
			continue
		}

		// Delete metadata
		if err := store.Trash.Delete(id); err != nil {
			logger.Errorf("trash restore: failed to delete metadata for %q: %v", id, err)
		}
	}

	return http.StatusOK, nil
}

// TrashDeleteRequest is the request body for permanently deleting trash items.
type TrashDeleteRequest struct {
	IDs []string `json:"ids"`
}

// trashDeleteHandler permanently deletes trash items from disk and database.
// @Summary Permanently delete trash items
// @Tags Trash
// @Accept json
// @Produce json
// @Param ids body TrashDeleteRequest true "IDs of trash items to permanently delete"
// @Success 200
// @Router /api/trash [delete]
func trashDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var req TrashDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}
	if len(req.IDs) == 0 {
		return http.StatusBadRequest, fmt.Errorf("ids array cannot be empty")
	}

	for _, id := range req.IDs {
		item, err := store.Trash.GetByID(id)
		if err != nil {
			logger.Errorf("trash delete: item %q not found: %v", id, err)
			continue
		}
		// Authorization: non-admin can only delete their own items
		if !d.user.Permissions.Admin && item.Username != d.user.Username {
			logger.Warningf("trash delete: user %q tried to delete item owned by %q", d.user.Username, item.Username)
			continue
		}

		permanentlyDeleteTrashItem(item)
	}

	return http.StatusOK, nil
}

// permanentlyDeleteTrashItem removes the file from disk and the metadata from DB.
func permanentlyDeleteTrashItem(item *trash.TrashItem) {
	// Remove from disk
	var removeErr error
	if item.IsDir {
		removeErr = os.RemoveAll(item.TrashPath)
	} else {
		removeErr = os.Remove(item.TrashPath)
	}
	if removeErr != nil && !os.IsNotExist(removeErr) {
		logger.Errorf("trash permanent delete: failed to remove %q from disk: %v", item.TrashPath, removeErr)
	}

	// Remove metadata from DB
	if err := store.Trash.Delete(item.ID); err != nil {
		logger.Errorf("trash permanent delete: failed to delete metadata for %q: %v", item.ID, err)
	}
}
