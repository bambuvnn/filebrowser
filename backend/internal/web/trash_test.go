package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/preview"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

func setupTrashTest(t *testing.T, scope string) (string, users.User) {
	t.Helper()
	sourcePath := setupDeleteQuotaTest(t)
	if preview.GetService() == nil {
		if err := preview.StartPreviewGenerator(1, t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	user, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	user.BackendScopes[0].Scope = scope
	for path, content := range map[string]string{"docs/a.txt": "hello", "docs/sub/b.txt": "world!"} {
		full := filepath.Join(sourcePath, scope, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := indexing.GetIndex("Downloads").RefreshDirectory("/", true); err != nil {
		t.Fatal(err)
	}
	return sourcePath, user
}

func trashIDsBody(t *testing.T, ids ...string) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(TrashIDsRequest{IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

func listTrash(t *testing.T, user *users.User) []TrashItemResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	status, err := trashListHandler(rec, httptest.NewRequest(http.MethodGet, "/api/trash", nil), &requestContext{User: user})
	if err != nil || (status != 0 && status != http.StatusOK) {
		t.Fatalf("list trash: status=%d err=%v", status, err)
	}
	var items []TrashItemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode list: %v (%s)", err, rec.Body.String())
	}
	return items
}

func TestTrash_DeleteMovesToTrashAndRestores(t *testing.T) {
	for _, scope := range []string{"/", "/scoped"} {
		t.Run(scope, func(t *testing.T) {
			sourcePath, user := setupTrashTest(t, scope)
			original := filepath.Join(sourcePath, scope, "docs", "a.txt")

			req := httptest.NewRequest(http.MethodDelete, "/api/resources?source=Downloads&path=/docs/a.txt", nil)
			if status, err := resourceDeleteHandler(httptest.NewRecorder(), req, &requestContext{User: &user}); err != nil || status != http.StatusOK {
				t.Fatalf("delete: status=%d err=%v", status, err)
			}
			if _, err := os.Stat(original); !os.IsNotExist(err) {
				t.Fatalf("original should be gone after delete: %v", err)
			}

			items := listTrash(t, &user)
			if len(items) != 1 {
				t.Fatalf("want 1 trash item, got %d", len(items))
			}
			it := items[0]
			if it.OriginalPath != "/docs/a.txt" || it.Name != "a.txt" || it.Size != 5 || it.IsDir {
				t.Fatalf("unexpected trash item (path must be scope-relative): %+v", it)
			}
			rec, err := state.GetTrashItem(it.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !files.IsWithinTrash(sourcePath, rec.TrashPath) {
				t.Fatalf("trash path %q not inside %s/.trash", rec.TrashPath, sourcePath)
			}
			if data, err := os.ReadFile(rec.TrashPath); err != nil || string(data) != "hello" {
				t.Fatalf("trashed content missing: %v %q", err, data)
			}

			restoreRec := httptest.NewRecorder()
			restoreReq := httptest.NewRequest(http.MethodPost, "/api/trash/restore", trashIDsBody(t, it.ID))
			if status, err := trashRestoreHandler(restoreRec, restoreReq, &requestContext{User: &user}); err != nil || (status != 0 && status != http.StatusOK) {
				t.Fatalf("restore: status=%d err=%v body=%s", status, err, restoreRec.Body.String())
			}
			if data, err := os.ReadFile(original); err != nil || string(data) != "hello" {
				t.Fatalf("restored content mismatch: %v %q", err, data)
			}
			if left := listTrash(t, &user); len(left) != 0 {
				t.Fatalf("trash should be empty after restore, got %d", len(left))
			}
		})
	}
}

func TestTrash_BulkDeleteDirectoryAndPurge(t *testing.T) {
	sourcePath, user := setupTrashTest(t, "/")
	body, _ := json.Marshal([]BulkDeleteItem{{Source: "Downloads", Path: "/docs/sub"}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/resources/bulk", bytes.NewReader(body))
	if status, err := ResourceBulkDeleteHandler(rec, req, &requestContext{User: &user}); err != nil || (status != 0 && status != http.StatusOK) {
		t.Fatalf("bulk delete: status=%d err=%v body=%s", status, err, rec.Body.String())
	}
	items := listTrash(t, &user)
	if len(items) != 1 || !items[0].IsDir || items[0].Size != 6 {
		t.Fatalf("unexpected trash items: %+v", items)
	}
	stored, _ := state.GetTrashItem(items[0].ID)

	delRec := httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, "/api/trash", trashIDsBody(t, items[0].ID))
	if status, err := trashDeleteHandler(delRec, delReq, &requestContext{User: &user}); err != nil || (status != 0 && status != http.StatusOK) {
		t.Fatalf("purge: status=%d err=%v", status, err)
	}
	if _, err := os.Stat(stored.TrashPath); !os.IsNotExist(err) {
		t.Fatalf("purged item still on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sourcePath, "docs", "sub")); !os.IsNotExist(err) {
		t.Fatalf("directory should not have been restored: %v", err)
	}
}

func TestTrash_RestoreRefusesToOverwrite(t *testing.T) {
	sourcePath, user := setupTrashTest(t, "/")
	req := httptest.NewRequest(http.MethodDelete, "/api/resources?source=Downloads&path=/docs/a.txt", nil)
	if status, err := resourceDeleteHandler(httptest.NewRecorder(), req, &requestContext{User: &user}); err != nil || status != http.StatusOK {
		t.Fatalf("delete: status=%d err=%v", status, err)
	}
	// A new file now occupies the original location.
	original := filepath.Join(sourcePath, "docs", "a.txt")
	if err := os.WriteFile(original, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	items := listTrash(t, &user)
	rec := httptest.NewRecorder()
	status, _ := trashRestoreHandler(rec, httptest.NewRequest(http.MethodPost, "/api/trash/restore", trashIDsBody(t, items[0].ID)), &requestContext{User: &user})
	if status != http.StatusMultiStatus {
		t.Fatalf("restore over existing file: got %d, want 207 (body=%s)", status, rec.Body.String())
	}
	if data, _ := os.ReadFile(original); string(data) != "new" {
		t.Fatalf("existing file was overwritten: %q", data)
	}
	if len(listTrash(t, &user)) != 1 {
		t.Fatal("trash item should remain after a refused restore")
	}
}

func TestTrash_OtherUsersCannotTouchItems(t *testing.T) {
	_, owner := setupTrashTest(t, "/")
	req := httptest.NewRequest(http.MethodDelete, "/api/resources?source=Downloads&path=/docs/a.txt", nil)
	if status, err := resourceDeleteHandler(httptest.NewRecorder(), req, &requestContext{User: &owner}); err != nil || status != http.StatusOK {
		t.Fatalf("delete: status=%d err=%v", status, err)
	}
	id := listTrash(t, &owner)[0].ID

	other := users.User{ID: owner.ID + 1000, FrontendUser: users.FrontendUser{Username: "intruder"}}
	if got := listTrash(t, &other); len(got) != 0 {
		t.Fatalf("other user must not see owner's trash, got %d", len(got))
	}
	for name, h := range map[string]HandleFunc{"restore": trashRestoreHandler, "delete": trashDeleteHandler} {
		rec := httptest.NewRecorder()
		status, _ := h(rec, httptest.NewRequest(http.MethodPost, "/", trashIDsBody(t, id)), &requestContext{User: &other})
		if status != http.StatusMultiStatus || !strings.Contains(rec.Body.String(), "not found") {
			t.Fatalf("%s by non-owner: status=%d body=%s", name, status, rec.Body.String())
		}
	}
	if _, err := state.GetTrashItem(id); err != nil {
		t.Fatalf("owner's item must survive: %v", err)
	}
}

func TestTrash_FolderIsHiddenFromListings(t *testing.T) {
	sourcePath, user := setupTrashTest(t, "/")
	req := httptest.NewRequest(http.MethodDelete, "/api/resources?source=Downloads&path=/docs/a.txt", nil)
	if status, err := resourceDeleteHandler(httptest.NewRecorder(), req, &requestContext{User: &user}); err != nil || status != http.StatusOK {
		t.Fatalf("delete: status=%d err=%v", status, err)
	}
	if _, err := os.Stat(filepath.Join(sourcePath, ".trash")); err != nil {
		t.Fatalf(".trash folder should exist on disk: %v", err)
	}
	user.ShowHidden = true
	root, err := files.FileInfoFaster(utils.FileOptions{Path: "/", Source: "Downloads", Expand: true, ShowHidden: true}, &user)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range root.Folders {
		if f.Name == ".trash" {
			t.Fatal(".trash must not appear in directory listings")
		}
	}
	if _, err := files.FileInfoFaster(utils.FileOptions{Path: "/.trash", Source: "Downloads", Expand: true, ShowHidden: true}, &user); err == nil {
		t.Fatal("direct access to /.trash must be denied")
	}
}

func TestTrash_SchedulerPurgesExpired(t *testing.T) {
	_, user := setupTrashTest(t, "/")
	req := httptest.NewRequest(http.MethodDelete, "/api/resources?source=Downloads&path=/docs/a.txt", nil)
	if status, err := resourceDeleteHandler(httptest.NewRecorder(), req, &requestContext{User: &user}); err != nil || status != http.StatusOK {
		t.Fatalf("delete: status=%d err=%v", status, err)
	}
	item, _ := state.GetTrashItem(listTrash(t, &user)[0].ID)
	item.DeletedAt = time.Now().Add(-trashRetention - time.Hour).Unix()
	if err := state.SaveTrashItem(item); err != nil {
		t.Fatal(err)
	}
	runTrashMaintenance()
	if _, err := state.GetTrashItem(item.ID); err == nil {
		t.Fatal("expired item should be purged")
	}
	if _, err := os.Stat(item.TrashPath); !os.IsNotExist(err) {
		t.Fatalf("expired item still on disk: %v", err)
	}
}
