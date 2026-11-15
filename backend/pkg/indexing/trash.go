package indexing

import "strings"

// TrashDirName is the per-source folder (at the source root) that holds
// soft-deleted items. It is never indexed, listed, searched or viewable.
const TrashDirName = ".trash"

// IsTrashIndexPath reports whether an index path (relative to the source root,
// e.g. "/.trash" or "/.trash/1/x.txt") is the trash folder or inside it.
func IsTrashIndexPath(indexPath string) bool {
	p := "/" + strings.TrimLeft(indexPath, "/")
	return p == "/"+TrashDirName || strings.HasPrefix(p, "/"+TrashDirName+"/")
}
