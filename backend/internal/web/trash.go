package web

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Trash: a normal (non-share) delete moves items to <source>/.trash/<userID>/
// instead of removing them. Items can be restored from the Trash page and are
// permanently purged after trashRetention by a background scheduler.

const (
	trashRetention         = 7 * 24 * time.Hour
	trashSchedulerInterval = 24 * time.Hour
)

// trashPathFor builds a unique destination inside the user's trash folder.
// The user id (not the username) names the folder so usernames never become path segments.
func trashPathFor(sourceRoot string, user *users.User, realPath string) string {
	name := uuid.New().String() + "_" + filepath.Base(realPath)
	return filepath.Join(files.TrashRootForSource(sourceRoot), strconv.FormatUint(user.ID, 10), name)
}

// moveToTrashInsteadOfDelete replaces files.DeleteFiles for regular (non-share)
// deletes. fullIndexPath includes the user scope, like the delete handlers use.
func moveToTrashInsteadOfDelete(user *users.User, source, userScope, fullIndexPath string, isDir bool) error {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v", source)
	}
	realPath, err := idx.GetRealPathForRemoval(userScope, fullIndexPath)
	if err != nil {
		return err
	}
	trashPath := trashPathFor(idx.Path, user, realPath)
	if _, err := files.MoveToTrash(source, userScope, fullIndexPath, isDir, trashPath); err != nil {
		return err
	}
	item := state.TrashItem{
		ID:           uuid.New().String(),
		Username:     user.Username,
		SourceName:   source,
		SourcePath:   idx.Path,
		OriginalPath: fullIndexPath,
		TrashPath:    trashPath,
		IsDir:        isDir,
		DeletedAt:    time.Now().Unix(),
	}
	if err := state.SaveTrashItem(item); err != nil {
		// Without a record the item would be unrecoverable: put it back.
		if restoreErr := files.RestoreFromTrash(source, trashPath, fullIndexPath, isDir); restoreErr != nil {
			logger.Errorf("trash: failed to save record and to roll back %q (now at %q): %v", fullIndexPath, trashPath, restoreErr)
		}
		return fmt.Errorf("failed to record trash item: %w", err)
	}
	return nil
}

// TrashItemResponse is the JSON representation of a trash item.
type TrashItemResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Username     string `json:"username,omitempty"` // only shown to admins
	SourceName   string `json:"sourceName"`
	OriginalPath string `json:"originalPath"` // relative to the viewer's scope when possible
	IsDir        bool   `json:"isDir"`
	Size         int64  `json:"size"`
	DeletedAt    string `json:"deletedAt"` // RFC3339
	ExpiresAt    string `json:"expiresAt"` // RFC3339
}

func toTrashItemResponse(item state.TrashItem, viewer *users.User) TrashItemResponse {
	resp := TrashItemResponse{
		ID:           item.ID,
		Name:         filepath.Base(item.OriginalPath),
		SourceName:   item.SourceName,
		OriginalPath: item.OriginalPath,
		IsDir:        item.IsDir,
		Size:         trashItemSize(item.TrashPath, item.IsDir),
		DeletedAt:    time.Unix(item.DeletedAt, 0).UTC().Format(time.RFC3339),
		ExpiresAt:    time.Unix(item.DeletedAt, 0).Add(trashRetention).UTC().Format(time.RFC3339),
	}
	if scope, err := viewer.GetScopeForSourceName(item.SourceName); err == nil {
		if rel, ok := stripScope(scope, item.OriginalPath); ok {
			resp.OriginalPath = rel
		}
	}
	if viewer.Permissions.Admin {
		resp.Username = item.Username
	}
	return resp
}

// stripScope returns fullIndexPath relative to scope if it lies within it.
func stripScope(scope, fullIndexPath string) (string, bool) {
	scope = "/" + strings.Trim(scope, "/")
	if scope == "/" {
		return fullIndexPath, true
	}
	if fullIndexPath == scope {
		return "/", true
	}
	if strings.HasPrefix(fullIndexPath, scope+"/") {
		return strings.TrimPrefix(fullIndexPath, scope), true
	}
	return "", false
}

func trashItemSize(trashPath string, isDir bool) int64 {
	if !isDir {
		info, err := os.Stat(trashPath)
		if err != nil {
			return 0
		}
		return info.Size()
	}
	var total int64
	_ = filepath.WalkDir(trashPath, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// canManageTrashItem: owners manage their own items; admins manage everyone's.
func canManageTrashItem(user *users.User, item state.TrashItem) bool {
	return user.Permissions.Admin || item.Username == user.Username
}

// trashListHandler lists the current user's trash items (admins may pass all=true).
// @Summary List trash items
// @Tags Trash
// @Produce json
// @Param all query bool false "Admin only: list all users' trash items"
// @Success 200 {array} TrashItemResponse
// @Router /api/trash [get]
func trashListHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	var items []state.TrashItem
	var err error
	if r.URL.Query().Get("all") == "true" && d.User.Permissions.Admin {
		items, err = state.ListAllTrashItems()
	} else {
		items, err = state.ListTrashItemsByUser(d.User.Username)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	result := make([]TrashItemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toTrashItemResponse(item, d.User))
	}
	return RenderJSON(w, r, result)
}

// TrashIDsRequest is the request body for restore and permanent delete.
type TrashIDsRequest struct {
	IDs []string `json:"ids"`
}

// TrashResultItem reports the outcome for one trash item id.
type TrashResultItem struct {
	ID      string `json:"id"`
	Message string `json:"message,omitempty"`
}

// TrashBulkResponse lists which ids succeeded and which failed.
type TrashBulkResponse struct {
	Succeeded []TrashResultItem `json:"succeeded"`
	Failed    []TrashResultItem `json:"failed"`
}

func decodeTrashIDs(r *http.Request) ([]string, error) {
	var req TrashIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %v", err)
	}
	if len(req.IDs) == 0 {
		return nil, fmt.Errorf("ids array cannot be empty")
	}
	return req.IDs, nil
}

func renderTrashBulk(w http.ResponseWriter, r *http.Request, resp TrashBulkResponse) (int, error) {
	status := http.StatusOK
	if len(resp.Failed) > 0 {
		status = http.StatusMultiStatus
	}
	return RenderJSON(w, r, resp, status)
}

// restoreTrashItem checks the caller may write the original location, then moves it back.
func restoreTrashItem(d *Context, item state.TrashItem) error {
	if !d.User.Permissions.Admin {
		// The owner must still have the original path inside their current scope.
		scope, err := d.User.GetScopeForSourceName(item.SourceName)
		if err != nil {
			return fmt.Errorf("you no longer have access to source %s", item.SourceName)
		}
		if _, ok := stripScope(scope, item.OriginalPath); !ok {
			return fmt.Errorf("original location is outside your scope")
		}
		perms, err := effectiveFilePerms(d, item.SourceName)
		if err != nil || !perms.Create {
			return fmt.Errorf("you are not allowed to create files in %s", item.SourceName)
		}
	}
	idx := indexing.GetIndex(item.SourceName)
	if idx == nil {
		return fmt.Errorf("source %s not found", item.SourceName)
	}
	if idx.Config.ReadOnly {
		return fmt.Errorf("source is read-only")
	}
	if err := files.RestoreFromTrash(item.SourceName, item.TrashPath, item.OriginalPath, item.IsDir); err != nil {
		return err
	}
	if err := state.DeleteTrashItem(item.ID); err != nil {
		logger.Errorf("trash restore: failed to delete record %q: %v", item.ID, err)
	}
	return nil
}

// trashRestoreHandler restores trash items to their original location.
// @Summary Restore trash items
// @Tags Trash
// @Accept json
// @Produce json
// @Param ids body TrashIDsRequest true "IDs of trash items to restore"
// @Success 200 {object} TrashBulkResponse
// @Success 207 {object} TrashBulkResponse "Partial success"
// @Router /api/trash/restore [post]
func trashRestoreHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	ids, err := decodeTrashIDs(r)
	if err != nil {
		return http.StatusBadRequest, err
	}
	resp := TrashBulkResponse{Succeeded: []TrashResultItem{}, Failed: []TrashResultItem{}}
	for _, id := range ids {
		item, err := state.GetTrashItem(id)
		if err != nil || !canManageTrashItem(d.User, item) {
			resp.Failed = append(resp.Failed, TrashResultItem{ID: id, Message: "not found"})
			continue
		}
		if err := restoreTrashItem(d, item); err != nil {
			resp.Failed = append(resp.Failed, TrashResultItem{ID: id, Message: err.Error()})
			continue
		}
		resp.Succeeded = append(resp.Succeeded, TrashResultItem{ID: id})
	}
	return renderTrashBulk(w, r, resp)
}

// purgeTrashItem permanently removes an item from disk and its record.
func purgeTrashItem(item state.TrashItem) error {
	sourceRoot := item.SourcePath
	if idx := indexing.GetIndex(item.SourceName); idx != nil {
		sourceRoot = idx.Path
	}
	if err := files.PurgeTrashPath(sourceRoot, item.TrashPath); err != nil {
		return err
	}
	return state.DeleteTrashItem(item.ID)
}

// trashDeleteHandler permanently deletes trash items.
// @Summary Permanently delete trash items
// @Tags Trash
// @Accept json
// @Produce json
// @Param ids body TrashIDsRequest true "IDs of trash items to permanently delete"
// @Success 200 {object} TrashBulkResponse
// @Success 207 {object} TrashBulkResponse "Partial success"
// @Router /api/trash [delete]
func trashDeleteHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	ids, err := decodeTrashIDs(r)
	if err != nil {
		return http.StatusBadRequest, err
	}
	resp := TrashBulkResponse{Succeeded: []TrashResultItem{}, Failed: []TrashResultItem{}}
	for _, id := range ids {
		item, err := state.GetTrashItem(id)
		if err != nil || !canManageTrashItem(d.User, item) {
			resp.Failed = append(resp.Failed, TrashResultItem{ID: id, Message: "not found"})
			continue
		}
		if err := purgeTrashItem(item); err != nil {
			logger.Errorf("trash delete %q: %v", id, err)
			resp.Failed = append(resp.Failed, TrashResultItem{ID: id, Message: "failed to delete, admin must check the logs"})
			continue
		}
		resp.Succeeded = append(resp.Succeeded, TrashResultItem{ID: id})
	}
	return renderTrashBulk(w, r, resp)
}

// startTrashScheduler purges expired trash items and refresh tokens daily.
func startTrashScheduler(ctx context.Context) {
	go func() {
		runTrashMaintenance()
		ticker := time.NewTicker(trashSchedulerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runTrashMaintenance()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func runTrashMaintenance() {
	expired, err := state.ListExpiredTrashItems(trashRetention)
	if err != nil {
		logger.Errorf("trash scheduler: failed to list expired items: %v", err)
	} else if len(expired) > 0 {
		logger.Infof("trash scheduler: purging %d expired item(s)", len(expired))
		for _, item := range expired {
			if err := purgeTrashItem(item); err != nil && !stderrors.Is(err, os.ErrNotExist) {
				logger.Errorf("trash scheduler: purge %q: %v", item.ID, err)
			}
		}
	}
	if n, err := state.PruneExpiredRefreshTokens(); err != nil {
		logger.Errorf("refresh token prune failed: %v", err)
	} else if n > 0 {
		logger.Debugf("pruned %d expired refresh token(s)", n)
	}
}
