package store

import (
	"database/sql"
	"errors"
	"fmt"

	"volt/internal/model"
)

// maxFolderDepth is the maximum allowed Folder nesting depth (Req 5.3). A Folder
// stored directly under a Collection is at depth 1, a Folder nested one level
// inside it is at depth 2, and so on. A save that would place any Folder deeper
// than this is rejected.
const maxFolderDepth = 10

// ErrMaxDepthExceeded is returned when a SaveFolder/SaveCollection would place a
// Folder deeper than maxFolderDepth levels (Req 5.3). The enclosing transaction
// rolls back, so the attempted write leaves prior data unchanged.
var ErrMaxDepthExceeded = errors.New("store: folder nesting depth exceeds maximum")

// subtreeHeight returns the depth of the supplied Folder subtree counting the
// folder itself as level 1, its child folders as level 2, and so on. A folder
// with no nested folders has height 1; the height of a folder is one more than
// the greatest height among its children.
func subtreeHeight(f *model.Folder) int {
	max := 0
	for i := range f.Folders {
		if h := subtreeHeight(&f.Folders[i]); h > max {
			max = h
		}
	}
	return max + 1
}

// folderDepth returns the depth of an existing Folder identified by folderID,
// where a Folder directly under a Collection has depth 1 and each level of
// nesting adds one. It walks the parent_id chain up to the Collection. It
// returns ErrUnknownParent when folderID does not exist. A guard bounds the
// traversal so an unexpectedly long or cyclic chain reports an error instead of
// looping forever.
func folderDepth(tx *sql.Tx, folderID string) (int, error) {
	depth := 0
	current := sql.NullString{String: folderID, Valid: true}
	for current.Valid {
		depth++
		if depth > maxFolderDepth+1 {
			return 0, fmt.Errorf("store: folder chain exceeds maximum traversal depth")
		}
		var parent sql.NullString
		err := tx.QueryRow(`SELECT parent_id FROM folders WHERE id = ?`, current.String).Scan(&parent)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, ErrUnknownParent
			}
			return 0, fmt.Errorf("store: read folder parent: %w", err)
		}
		current = parent
	}
	return depth, nil
}

// enforceFolderDepth verifies that placing the folder f (with its supplied
// nested subtree) under the given parent does not exceed maxFolderDepth levels
// (Req 5.3). parentFolder is the resolved parent Folder reference: when it is
// not valid the parent is a Collection and f sits at depth 1; otherwise the
// parent Folder's own depth is added. It returns ErrMaxDepthExceeded when the
// deepest folder in the resulting subtree would be deeper than maxFolderDepth.
func enforceFolderDepth(tx *sql.Tx, parentFolder sql.NullString, f *model.Folder) error {
	parentDepth := 0
	if parentFolder.Valid {
		d, err := folderDepth(tx, parentFolder.String)
		if err != nil {
			return err
		}
		parentDepth = d
	}
	if parentDepth+subtreeHeight(f) > maxFolderDepth {
		return ErrMaxDepthExceeded
	}
	return nil
}
