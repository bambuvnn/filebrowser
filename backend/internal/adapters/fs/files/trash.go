package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

// TrashDirName is the per-source folder holding soft-deleted items. It is
// hidden from listings, search and indexing (see indexing.IsTrashIndexPath).
const TrashDirName = indexing.TrashDirName

// TrashRootForSource returns <sourceRoot>/.trash.
func TrashRootForSource(sourceRoot string) string {
	return filepath.Join(sourceRoot, TrashDirName)
}

// IsWithinTrash reports whether absPath is strictly inside <sourceRoot>/.trash/.
func IsWithinTrash(sourceRoot, absPath string) bool {
	root := filepath.Clean(TrashRootForSource(sourceRoot)) + string(os.PathSeparator)
	return strings.HasPrefix(filepath.Clean(absPath), root)
}

// MoveToTrash moves indexPath (bounded by boundIndexPath, like DeleteFiles)
// into trashPath, which must be inside the source's .trash folder, then
// updates the index exactly as a delete would. It returns the resolved real
// path that was moved.
func MoveToTrash(source, boundIndexPath, indexPath string, isDir bool, trashPath string) (string, error) {
	index := indexing.GetIndex(source)
	if index == nil {
		return "", fmt.Errorf("could not get index: %v ", source)
	}
	if utils.CleanIndexPathSegment(indexPath) == "" {
		return "", fmt.Errorf("refusing to trash source root directory: %s", indexPath)
	}
	if indexing.IsTrashIndexPath(indexPath) {
		return "", fmt.Errorf("item is already in the trash")
	}
	if !IsWithinTrash(index.Path, trashPath) {
		return "", fmt.Errorf("trash destination escapes trash folder")
	}
	realPath, err := index.GetRealPathForRemoval(boundIndexPath, indexPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(trashPath), fileutils.PermDir); err != nil {
		return "", fmt.Errorf("cannot create trash folder: %w", err)
	}
	if err := os.Rename(realPath, trashPath); err != nil {
		return "", fmt.Errorf("cannot move item to trash: %w", err)
	}
	if index.Config.ResolvedRules.IndexingDisabled {
		return realPath, nil
	}
	removeFromIndexAfterMove(index, boundIndexPath, indexPath, isDir)
	return realPath, nil
}

// removeFromIndexAfterMove mirrors the index bookkeeping in DeleteFiles.
func removeFromIndexAfterMove(index *indexing.Index, boundIndexPath, indexPath string, isDir bool) {
	deleteIndexPath := indexPath
	indexPath = index.MakeIndexPath(indexPath, isDir).String()
	if isDir {
		indexing.ClearRealPathCaches()
	}
	index.InvalidateRealPathCache(deleteIndexPath, boundIndexPath)
	if parent := utils.GetParentDirectoryPath(deleteIndexPath); parent != "" && parent != "/" {
		index.InvalidateRealPathCache(parent, boundIndexPath)
	}
	if !index.DeleteMetadata(indexPath, isDir, isDir) {
		logger.Errorf("Failed to delete metadata from index for %s, but move to trash succeeded", indexPath)
	}
	refreshConfig := utils.FileOptions{
		Path:  index.MakeIndexPath(filepath.Dir(strings.TrimSuffix(indexPath, "/")), true).String(),
		IsDir: true,
	}
	if err := index.RefreshFileInfo(refreshConfig); err != nil {
		logger.Debugf("refresh after trash move for %s: %v", indexPath, err)
	}
}

// RestoreFromTrash moves trashPath back to the real path of indexPath in
// source. It refuses to overwrite an existing item and refreshes the index.
func RestoreFromTrash(source, trashPath, indexPath string, isDir bool) error {
	index := indexing.GetIndex(source)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}
	if !IsWithinTrash(index.Path, trashPath) {
		return fmt.Errorf("trash item path is outside the trash folder")
	}
	if utils.CleanIndexPathSegment(indexPath) == "" || indexing.IsTrashIndexPath(indexPath) {
		return fmt.Errorf("invalid restore destination")
	}
	dest, err := index.GetRealPathForRemoval("/", indexPath)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("an item already exists at %s", indexPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), fileutils.PermDir); err != nil {
		return fmt.Errorf("cannot create parent folder: %w", err)
	}
	if err := os.Rename(trashPath, dest); err != nil {
		return fmt.Errorf("cannot restore item: %w", err)
	}
	if index.Config.ResolvedRules.IndexingDisabled {
		return nil
	}
	if isDir {
		indexing.ClearRealPathCaches()
	}
	parent := utils.GetParentDirectoryPath(indexPath)
	if parent == "" {
		parent = "/"
	}
	if err := RefreshIndex(source, parent, true, isDir); err != nil {
		logger.Debugf("refresh after restore for %s: %v", indexPath, err)
	}
	return nil
}

// PurgeTrashPath permanently removes trashPath, refusing anything outside the
// source's .trash folder. Missing files are not an error.
func PurgeTrashPath(sourceRoot, trashPath string) error {
	if !IsWithinTrash(sourceRoot, trashPath) {
		return fmt.Errorf("refusing to purge path outside trash: %s", trashPath)
	}
	if err := os.RemoveAll(trashPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
