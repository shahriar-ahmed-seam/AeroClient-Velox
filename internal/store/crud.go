package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"volt/internal/model"
)

// ErrNotFound is returned when a rename or delete targets an item that does not
// exist. Callers (the bindings/frontend) surface this as an error while leaving
// existing stored data untouched (Req 5.9).
var ErrNotFound = errors.New("store: item not found")

// ErrUnknownParent is returned when a save targets a parent ID that is neither
// an existing Collection nor an existing Folder.
var ErrUnknownParent = errors.New("store: unknown parent")

// newID returns a fresh random identifier for a Collection, Folder, or Request
// that arrives without one.
func newID() string { return uuid.NewString() }

// ---------------------------------------------------------------------------
// Collections
// ---------------------------------------------------------------------------

// SaveCollection persists a Collection and its entire nested tree (folders and
// requests) in a single transaction, preserving names, nesting, and order
// (Req 5.1, 5.6). Slice position determines the stored order within a parent,
// and the Collection's Order field is its own position among collections.
//
// Save uses upsert semantics: saving a Collection whose ID already exists
// replaces it. The collection's existing subtree is cleared and rebuilt from
// the supplied model so removed folders/requests do not linger. Any entity that
// arrives without an ID is assigned a fresh one, and the fully-populated
// Collection (with assigned IDs) is returned.
func (s *Store) SaveCollection(c model.Collection) (model.Collection, error) {
	// Defense-in-depth name validation (Req 5.8): reject the whole save before
	// any write if the collection, any nested folder, or any request name is
	// out of bounds, so prior data is left unchanged.
	if err := validateCollectionTree(&c); err != nil {
		return model.Collection{}, err
	}
	if c.ID == "" {
		c.ID = newID()
	}
	err := s.withTx(func(tx *sql.Tx) error {
		// Reject the save if any supplied folder subtree would nest deeper than
		// the limit (Req 5.3). Top-level folders sit directly under the
		// collection, so their subtree height alone must stay within bounds. The
		// withTx rollback leaves prior data unchanged on rejection.
		for i := range c.Folders {
			if err := enforceFolderDepth(tx, sql.NullString{}, &c.Folders[i]); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO collections(id, name, ord) VALUES (?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, ord = excluded.ord`,
			c.ID, c.Name, c.Order,
		); err != nil {
			return fmt.Errorf("store: save collection: %w", err)
		}

		// Rebuild the subtree from scratch so the stored tree mirrors the model
		// exactly (including removals). Deleting the folders cascades to their
		// descendant folders and requests; top-level requests (folder_id NULL)
		// are removed explicitly.
		if _, err := tx.Exec(`DELETE FROM requests WHERE collection_id = ?`, c.ID); err != nil {
			return fmt.Errorf("store: clear collection requests: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM folders WHERE collection_id = ?`, c.ID); err != nil {
			return fmt.Errorf("store: clear collection folders: %w", err)
		}

		for i := range c.Requests {
			if err := insertRequestRow(tx, &c.Requests[i], c.ID, sql.NullString{}, i); err != nil {
				return err
			}
		}
		for i := range c.Folders {
			if err := insertFolderTree(tx, &c.Folders[i], c.ID, sql.NullString{}, i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return model.Collection{}, err
	}
	return c, nil
}

// RenameCollection changes a Collection's name (Req 5.2). It returns ErrNotFound
// when no Collection has the given ID. Name-length validation is applied by a
// later defense-in-depth backstop; this method performs the rename itself.
func (s *Store) RenameCollection(id, name string) error {
	// Defense-in-depth name validation (Req 5.8): reject an out-of-bounds name
	// before any write so the stored name is left unchanged.
	if err := validateCollectionName(name); err != nil {
		return err
	}
	return s.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE collections SET name = ? WHERE id = ?`, name, id)
		if err != nil {
			return fmt.Errorf("store: rename collection: %w", err)
		}
		return requireAffected(res)
	})
}

// DeleteCollection removes a Collection and, by ON DELETE CASCADE, all of its
// folders and requests (Req 5.7). It returns ErrNotFound when no Collection has
// the given ID, leaving existing data untouched.
func (s *Store) DeleteCollection(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM collections WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("store: delete collection: %w", err)
		}
		return requireAffected(res)
	})
}

// ---------------------------------------------------------------------------
// Folders
// ---------------------------------------------------------------------------

// SaveFolder persists a Folder (and any nested folders/requests it carries)
// under the given parent, which may be a Collection ID or another Folder ID.
// Save uses upsert semantics: an existing folder keeps its stored position,
// while a new folder is appended after its siblings. The folder's existing
// subtree is rebuilt from the supplied model so removals are honored. The
// fully-populated Folder (with assigned IDs) is returned.
func (s *Store) SaveFolder(f model.Folder, parentID string) (model.Folder, error) {
	// Defense-in-depth name validation (Req 5.8): reject the save before any
	// write if the folder, any nested folder, or any request name is out of
	// bounds, so prior data is left unchanged.
	if err := validateFolderTree(&f); err != nil {
		return model.Folder{}, err
	}
	if f.ID == "" {
		f.ID = newID()
	}
	err := s.withTx(func(tx *sql.Tx) error {
		collectionID, folderParent, err := resolveParent(tx, parentID)
		if err != nil {
			return err
		}
		// Reject a save that would nest a folder deeper than the limit (Req 5.3);
		// the withTx rollback leaves prior data unchanged.
		if err := enforceFolderDepth(tx, folderParent, &f); err != nil {
			return err
		}
		ord, err := positionFor(tx,
			`SELECT ord FROM folders WHERE id = ?`, f.ID,
			"folders", "collection_id", collectionID, "parent_id", folderParent)
		if err != nil {
			return err
		}
		return upsertFolderTree(tx, &f, collectionID, folderParent, ord)
	})
	if err != nil {
		return model.Folder{}, err
	}
	return f, nil
}

// DeleteFolder removes a Folder and, by ON DELETE CASCADE, all of its nested
// folders and requests. It returns ErrNotFound when no Folder has the given ID.
func (s *Store) DeleteFolder(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM folders WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("store: delete folder: %w", err)
		}
		return requireAffected(res)
	})
}

// ---------------------------------------------------------------------------
// Requests
// ---------------------------------------------------------------------------

// SaveRequest persists a SavedRequest under the given parent, which may be a
// Collection ID or a Folder ID (Req 5.1). The full request configuration
// (method, URL, params, headers, body, auth) is stored, with the structured
// configuration serialized as JSON in the requests.data column.
//
// Save uses upsert semantics: an existing request keeps its stored position
// while a new request is appended after its siblings. A request without an ID
// is assigned one, and the saved request (with assigned ID) is returned.
//
// Relocating a request to a different parent is handled by MoveRequest (a
// separate operation); SaveRequest writes the request into the resolved parent.
func (s *Store) SaveRequest(r model.SavedRequest, parentID string) (model.SavedRequest, error) {
	// Defense-in-depth name validation (Req 5.8): reject an out-of-bounds
	// request name before any write so prior data is left unchanged.
	if err := validateRequestName(r.Name); err != nil {
		return model.SavedRequest{}, err
	}
	if r.ID == "" {
		r.ID = newID()
	}
	err := s.withTx(func(tx *sql.Tx) error {
		collectionID, folderID, err := resolveParent(tx, parentID)
		if err != nil {
			return err
		}
		ord, err := positionFor(tx,
			`SELECT ord FROM requests WHERE id = ?`, r.ID,
			"requests", "collection_id", collectionID, "folder_id", folderID)
		if err != nil {
			return err
		}
		return insertRequestRow(tx, &r, collectionID, folderID, ord)
	})
	if err != nil {
		return model.SavedRequest{}, err
	}
	return r, nil
}

// DeleteRequest removes a single saved Request. It returns ErrNotFound when no
// Request has the given ID.
func (s *Store) DeleteRequest(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM requests WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("store: delete request: %w", err)
		}
		return requireAffected(res)
	})
}

// ---------------------------------------------------------------------------
// ListTree
// ---------------------------------------------------------------------------

// ListTree reconstructs the full Collection tree in saved order (Req 5.6). Each
// Collection's folders and requests, and every folder's nested folders and
// requests, are returned in their stored "ord" order, with each request's
// configuration rehydrated from the JSON stored in requests.data.
func (s *Store) ListTree() ([]model.Collection, error) {
	collections, collOrder, err := s.loadCollections()
	if err != nil {
		return nil, err
	}

	// Folder adjacency: ordered child-folder IDs per parent folder, and ordered
	// top-level folder IDs per collection. Names are kept in a side table so the
	// tree can be materialized recursively at the end.
	folderName := map[string]string{}
	childFolders := map[string][]string{}
	collFolders := map[string][]string{}
	if err := s.loadFolders(folderName, childFolders, collFolders); err != nil {
		return nil, err
	}

	// Request adjacency: ordered requests per folder, and per collection (for
	// requests stored directly under a collection).
	folderReqs := map[string][]model.SavedRequest{}
	collReqs := map[string][]model.SavedRequest{}
	if err := s.loadRequests(folderReqs, collReqs); err != nil {
		return nil, err
	}

	// Materialize the nested folder tree bottom-up via recursion.
	var build func(folderID string) model.Folder
	build = func(folderID string) model.Folder {
		f := model.Folder{
			ID:       folderID,
			Name:     folderName[folderID],
			Folders:  []model.Folder{},
			Requests: []model.SavedRequest{},
		}
		for _, childID := range childFolders[folderID] {
			f.Folders = append(f.Folders, build(childID))
		}
		if reqs := folderReqs[folderID]; reqs != nil {
			f.Requests = reqs
		}
		return f
	}

	out := make([]model.Collection, 0, len(collOrder))
	for _, id := range collOrder {
		c := collections[id]
		c.Folders = []model.Folder{}
		c.Requests = []model.SavedRequest{}
		for _, topFolderID := range collFolders[id] {
			c.Folders = append(c.Folders, build(topFolderID))
		}
		if reqs := collReqs[id]; reqs != nil {
			c.Requests = reqs
		}
		out = append(out, c)
	}
	return out, nil
}

// loadCollections returns the collections keyed by ID plus the ID list in stored
// order.
func (s *Store) loadCollections() (map[string]model.Collection, []string, error) {
	rows, err := s.db.Query(`SELECT id, name, ord FROM collections ORDER BY ord, id`)
	if err != nil {
		return nil, nil, fmt.Errorf("store: list collections: %w", err)
	}
	defer rows.Close()

	byID := map[string]model.Collection{}
	order := []string{}
	for rows.Next() {
		var c model.Collection
		if err := rows.Scan(&c.ID, &c.Name, &c.Order); err != nil {
			return nil, nil, fmt.Errorf("store: scan collection: %w", err)
		}
		byID[c.ID] = c
		order = append(order, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("store: iterate collections: %w", err)
	}
	return byID, order, nil
}

// loadFolders fills the supplied maps with folder names and the ordered child
// adjacency (per parent folder and per collection for top-level folders).
func (s *Store) loadFolders(folderName map[string]string, childFolders, collFolders map[string][]string) error {
	rows, err := s.db.Query(
		`SELECT id, name, collection_id, parent_id, ord FROM folders ORDER BY ord, id`)
	if err != nil {
		return fmt.Errorf("store: list folders: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, name, collectionID string
			parentID               sql.NullString
			ord                    int
		)
		if err := rows.Scan(&id, &name, &collectionID, &parentID, &ord); err != nil {
			return fmt.Errorf("store: scan folder: %w", err)
		}
		folderName[id] = name
		if parentID.Valid {
			childFolders[parentID.String] = append(childFolders[parentID.String], id)
		} else {
			collFolders[collectionID] = append(collFolders[collectionID], id)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate folders: %w", err)
	}
	return nil
}

// loadRequests fills the supplied maps with ordered requests per folder and per
// collection (for requests stored directly under a collection), rehydrating the
// configuration JSON from requests.data.
func (s *Store) loadRequests(folderReqs, collReqs map[string][]model.SavedRequest) error {
	rows, err := s.db.Query(
		`SELECT id, name, collection_id, folder_id, data FROM requests ORDER BY ord, id`)
	if err != nil {
		return fmt.Errorf("store: list requests: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, name, collectionID, data string
			folderID                     sql.NullString
		)
		if err := rows.Scan(&id, &name, &collectionID, &folderID, &data); err != nil {
			return fmt.Errorf("store: scan request: %w", err)
		}
		var raw model.RawRequest
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			return fmt.Errorf("store: decode request %q: %w", id, err)
		}
		sr := model.SavedRequest{RawRequest: raw, ID: id, Name: name}
		if folderID.Valid {
			folderReqs[folderID.String] = append(folderReqs[folderID.String], sr)
		} else {
			collReqs[collectionID] = append(collReqs[collectionID], sr)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate requests: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Shared write helpers
// ---------------------------------------------------------------------------

// resolveParent maps a parent ID (a Collection ID or a Folder ID) to the
// (collection_id, folder_id) pair used to store a child folder or request. When
// the parent is a Collection the folder reference is NULL; when it is a Folder
// the request/child folder belongs to that folder's collection. It returns
// ErrUnknownParent when the ID matches neither table.
func resolveParent(tx *sql.Tx, parentID string) (collectionID string, folderRef sql.NullString, err error) {
	// Is the parent a collection?
	err = tx.QueryRow(`SELECT id FROM collections WHERE id = ?`, parentID).Scan(&collectionID)
	if err == nil {
		return collectionID, sql.NullString{}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", sql.NullString{}, fmt.Errorf("store: resolve parent collection: %w", err)
	}

	// Otherwise it must be a folder; inherit that folder's collection.
	err = tx.QueryRow(`SELECT collection_id FROM folders WHERE id = ?`, parentID).Scan(&collectionID)
	if err == nil {
		return collectionID, sql.NullString{String: parentID, Valid: true}, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", sql.NullString{}, ErrUnknownParent
	}
	return "", sql.NullString{}, fmt.Errorf("store: resolve parent folder: %w", err)
}

// positionFor returns the "ord" to store a row at: the existing row's ord when
// the ID already exists (so an update keeps its place), otherwise the next
// position after the current siblings sharing the same (scopeCol, parentCol)
// scope. parentRef NULL is matched with IS NULL so collection-level siblings are
// counted correctly.
func positionFor(
	tx *sql.Tx,
	existingQuery, id string,
	table, scopeCol, scopeVal, parentCol string, parentRef sql.NullString,
) (int, error) {
	var ord int
	err := tx.QueryRow(existingQuery, id).Scan(&ord)
	if err == nil {
		return ord, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("store: read existing position: %w", err)
	}

	var q string
	args := []any{scopeVal}
	if parentRef.Valid {
		q = fmt.Sprintf(
			`SELECT COALESCE(MAX(ord)+1, 0) FROM %s WHERE %s = ? AND %s = ?`,
			table, scopeCol, parentCol)
		args = append(args, parentRef.String)
	} else {
		q = fmt.Sprintf(
			`SELECT COALESCE(MAX(ord)+1, 0) FROM %s WHERE %s = ? AND %s IS NULL`,
			table, scopeCol, parentCol)
	}
	if err := tx.QueryRow(q, args...).Scan(&ord); err != nil {
		return 0, fmt.Errorf("store: compute next position: %w", err)
	}
	return ord, nil
}

// insertRequestRow upserts a single request row, serializing its full
// configuration to JSON in the data column and duplicating method/url into their
// own columns for cheap listing. A blank ID is assigned before insertion.
func insertRequestRow(tx *sql.Tx, r *model.SavedRequest, collectionID string, folderID sql.NullString, ord int) error {
	if r.ID == "" {
		r.ID = newID()
	}
	data, err := json.Marshal(r.RawRequest)
	if err != nil {
		return fmt.Errorf("store: encode request %q: %w", r.ID, err)
	}
	if _, err := tx.Exec(
		`INSERT INTO requests(id, name, collection_id, folder_id, ord, method, url, data)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		    name = excluded.name,
		    collection_id = excluded.collection_id,
		    folder_id = excluded.folder_id,
		    ord = excluded.ord,
		    method = excluded.method,
		    url = excluded.url,
		    data = excluded.data`,
		r.ID, r.Name, collectionID, folderID, ord, r.Method, r.URL, string(data),
	); err != nil {
		return fmt.Errorf("store: save request %q: %w", r.ID, err)
	}
	return nil
}

// insertFolderRow upserts a single folder row (without touching its children).
// A blank ID is assigned before insertion.
func insertFolderRow(tx *sql.Tx, f *model.Folder, collectionID string, parentID sql.NullString, ord int) error {
	if f.ID == "" {
		f.ID = newID()
	}
	if _, err := tx.Exec(
		`INSERT INTO folders(id, name, collection_id, parent_id, ord)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		    name = excluded.name,
		    collection_id = excluded.collection_id,
		    parent_id = excluded.parent_id,
		    ord = excluded.ord`,
		f.ID, f.Name, collectionID, parentID, ord,
	); err != nil {
		return fmt.Errorf("store: save folder %q: %w", f.ID, err)
	}
	return nil
}

// insertFolderTree inserts a folder and its full nested subtree (child folders
// and requests) in order. It is used by SaveCollection when rebuilding a
// collection's subtree from scratch.
func insertFolderTree(tx *sql.Tx, f *model.Folder, collectionID string, parentID sql.NullString, ord int) error {
	if err := insertFolderRow(tx, f, collectionID, parentID, ord); err != nil {
		return err
	}
	thisFolder := sql.NullString{String: f.ID, Valid: true}
	for i := range f.Requests {
		if err := insertRequestRow(tx, &f.Requests[i], collectionID, thisFolder, i); err != nil {
			return err
		}
	}
	for i := range f.Folders {
		if err := insertFolderTree(tx, &f.Folders[i], collectionID, thisFolder, i); err != nil {
			return err
		}
	}
	return nil
}

// upsertFolderTree upserts a folder at the given position and rebuilds its
// nested subtree from the supplied model so that removed children do not linger.
func upsertFolderTree(tx *sql.Tx, f *model.Folder, collectionID string, parentID sql.NullString, ord int) error {
	if f.ID == "" {
		f.ID = newID()
	}
	// Clear the existing subtree of this folder before rebuilding. Deleting child
	// folders cascades to their descendants; direct requests are removed here.
	if _, err := tx.Exec(`DELETE FROM requests WHERE folder_id = ?`, f.ID); err != nil {
		return fmt.Errorf("store: clear folder requests: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM folders WHERE parent_id = ?`, f.ID); err != nil {
		return fmt.Errorf("store: clear child folders: %w", err)
	}
	return insertFolderTree(tx, f, collectionID, parentID, ord)
}

// requireAffected converts a zero-rows-affected result into ErrNotFound so a
// rename/delete of a missing item is reported rather than silently ignored.
func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
