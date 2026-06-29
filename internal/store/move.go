package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// MoveRequest relocates a saved Request to a new parent, which may be a
// Collection ID or a Folder ID (Req 5.4). The request is placed at the end of
// the target parent's existing requests and removed from its prior location in
// a single transaction, so it ends up in exactly one place and a failure leaves
// the prior arrangement intact (Req 5.9).
//
// It returns ErrUnknownParent when targetParentID matches neither an existing
// Collection nor an existing Folder, and ErrNotFound when no Request has the
// given requestID. In both error cases the stored data is left unchanged.
func (s *Store) MoveRequest(requestID, targetParentID string) error {
	return s.withTx(func(tx *sql.Tx) error {
		collectionID, folderID, err := resolveParent(tx, targetParentID)
		if err != nil {
			return err
		}

		// Append after the target's current requests, excluding the moved
		// request itself so a move within the same parent does not count its own
		// row (which would leave a gap). parentRef NULL is matched with IS NULL so
		// collection-level siblings are counted correctly.
		ord, err := nextRequestPosition(tx, requestID, collectionID, folderID)
		if err != nil {
			return err
		}

		res, err := tx.Exec(
			`UPDATE requests SET collection_id = ?, folder_id = ?, ord = ? WHERE id = ?`,
			collectionID, folderID, ord, requestID,
		)
		if err != nil {
			return fmt.Errorf("store: move request %q: %w", requestID, err)
		}
		return requireAffected(res)
	})
}

// nextRequestPosition returns the ord to append a request at within the target
// parent scope (collection_id plus folder_id, where a NULL folder_id is the
// collection level). The moved request is excluded from the computation so
// relocating within the same parent does not count its own existing row.
func nextRequestPosition(tx *sql.Tx, requestID, collectionID string, folderID sql.NullString) (int, error) {
	var (
		ord int
		err error
	)
	if folderID.Valid {
		err = tx.QueryRow(
			`SELECT COALESCE(MAX(ord)+1, 0) FROM requests
			 WHERE collection_id = ? AND folder_id = ? AND id != ?`,
			collectionID, folderID.String, requestID,
		).Scan(&ord)
	} else {
		err = tx.QueryRow(
			`SELECT COALESCE(MAX(ord)+1, 0) FROM requests
			 WHERE collection_id = ? AND folder_id IS NULL AND id != ?`,
			collectionID, requestID,
		).Scan(&ord)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("store: compute move position: %w", err)
	}
	return ord, nil
}
