package sqldb

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestRefreshToken_ConsumeIsSingleUse(t *testing.T) {
	store := newGroupTestStore(t)
	rec := RefreshTokenRecord{TokenHash: "h1", UserID: 7, ExpiresAt: 2000, CreatedAt: 1000, UserAgent: "ua"}
	if err := store.SaveRefreshToken(rec); err != nil {
		t.Fatal(err)
	}
	got, err := store.ConsumeRefreshToken("h1", 1500)
	if err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if got.UserID != 7 || got.UserAgent != "ua" {
		t.Fatalf("unexpected record: %+v", got)
	}
	if _, err := store.ConsumeRefreshToken("h1", 1500); !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("second consume should fail with not found, got %v", err)
	}
}

func TestRefreshToken_ExpiredIsRejectedAndRemoved(t *testing.T) {
	store := newGroupTestStore(t)
	if err := store.SaveRefreshToken(RefreshTokenRecord{TokenHash: "old", UserID: 1, ExpiresAt: 100, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeRefreshToken("old", 100); !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("expected expired token rejected, got %v", err)
	}
	var n int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM refresh_tokens`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expired token should be deleted on consume, %d rows left", n)
	}
}

func TestRefreshToken_DeleteByUserAndExpired(t *testing.T) {
	store := newGroupTestStore(t)
	for _, r := range []RefreshTokenRecord{
		{TokenHash: "a", UserID: 1, ExpiresAt: 5000, CreatedAt: 1},
		{TokenHash: "b", UserID: 1, ExpiresAt: 5000, CreatedAt: 1},
		{TokenHash: "c", UserID: 2, ExpiresAt: 5000, CreatedAt: 1},
		{TokenHash: "d", UserID: 2, ExpiresAt: 10, CreatedAt: 1},
	} {
		if err := store.SaveRefreshToken(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DeleteRefreshTokensByUserID(1); err != nil {
		t.Fatal(err)
	}
	n, err := store.DeleteExpiredRefreshTokens(100)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 expired token deleted, got %d", n)
	}
	if _, err := store.ConsumeRefreshToken("c", 100); err != nil {
		t.Fatalf("user 2 live token should survive: %v", err)
	}
	if _, err := store.ConsumeRefreshToken("a", 100); !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("user 1 token should be gone, got %v", err)
	}
}

func TestTrashItems_CRUD(t *testing.T) {
	store := newGroupTestStore(t)
	items := []TrashItemRecord{
		{ID: "1", Username: "alice", SourceName: "src", SourcePath: "/srv", OriginalPath: "/a.txt", TrashPath: "/srv/.trash/alice/x_a.txt", DeletedAt: 100},
		{ID: "2", Username: "alice", SourceName: "src", SourcePath: "/srv", OriginalPath: "/dir", TrashPath: "/srv/.trash/alice/y_dir", IsDir: true, DeletedAt: 300},
		{ID: "3", Username: "bob", SourceName: "src", SourcePath: "/srv", OriginalPath: "/b.txt", TrashPath: "/srv/.trash/bob/z_b.txt", DeletedAt: 200},
	}
	for _, it := range items {
		if err := store.SaveTrashItem(it); err != nil {
			t.Fatal(err)
		}
	}
	alice, err := store.ListTrashItemsByUser("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(alice) != 2 || alice[0].ID != "2" || !alice[0].IsDir {
		t.Fatalf("unexpected alice items (want newest first): %+v", alice)
	}
	all, err := store.ListAllTrashItems()
	if err != nil || len(all) != 3 {
		t.Fatalf("list all: %v %d", err, len(all))
	}
	old, err := store.ListTrashItemsDeletedBefore(200)
	if err != nil || len(old) != 2 {
		t.Fatalf("deleted-before: %v %d", err, len(old))
	}
	if err := store.DeleteTrashItem("1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetTrashItem("1"); !errors.Is(err, ErrTrashItemNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	got, err := store.GetTrashItem("3")
	if err != nil || got.Username != "bob" {
		t.Fatalf("get: %v %+v", err, got)
	}
}

func TestForkSchema_SurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	store, _, err := NewSQLStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTrashItem(TrashItemRecord{ID: "k", Username: "u", DeletedAt: 1}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, _, err = NewSQLStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.GetTrashItem("k"); err != nil {
		t.Fatalf("trash item lost after reopen: %v", err)
	}
	v, err := getSchemaVersion(store.DB())
	if err != nil {
		t.Fatal(err)
	}
	if v != currentSchemaVersion {
		t.Fatalf("fork tables must not change schema version: got %d want %d", v, currentSchemaVersion)
	}
}
