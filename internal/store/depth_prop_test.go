package store

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 14: Folder nesting depth is bounded at 10
//
// Validates: Requirements 5.3
//
// TestFolderNestingDepthIsBounded is the property-based test for Property 14:
// for any attempted folder creation, the save succeeds if and only if the
// resulting nesting depth is at most maxFolderDepth (10) levels. A folder stored
// directly under a Collection is at depth 1, a folder nested one level inside it
// is at depth 2, and so on.
//
// Each iteration builds a single linear chain of D folders (1..15) nested inside
// one Collection and persists it via SaveCollection against a fresh in-memory
// store. When D <= 10 the save must succeed and the stored tree must contain the
// full chain at depth D. When D > 10 the save must be rejected with
// ErrMaxDepthExceeded and, because the rejection rolls the transaction back, the
// store must be left unchanged — no over-deep folder is persisted (here, the
// store started empty, so nothing is persisted at all).
func TestFolderNestingDepthIsBounded(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		// Total nesting depth of the chain to attempt: 1..15, straddling the
		// maxFolderDepth (10) boundary in both directions.
		depth := rng.Intn(15) + 1

		counter := 0
		col := model.Collection{
			ID:       fmt.Sprintf("depth-col-%d", seed),
			Name:     "chain",
			Order:    0,
			Requests: []model.SavedRequest{},
			Folders:  buildLinearFolderChain(depth, &counter),
		}

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		_, saveErr := s.SaveCollection(col)

		tree, err := s.ListTree()
		if err != nil {
			failMsg = fmt.Sprintf("ListTree: %v", err)
			return false
		}
		storedDepth := treeMaxFolderDepth(tree)

		if depth <= maxFolderDepth {
			// Within the limit: the save must succeed and persist the full chain.
			if saveErr != nil {
				failMsg = fmt.Sprintf(
					"chain depth %d (<= %d) was rejected: %v",
					depth, maxFolderDepth, saveErr)
				return false
			}
			if storedDepth != depth {
				failMsg = fmt.Sprintf(
					"chain depth %d (<= %d) stored at depth %d, want %d",
					depth, maxFolderDepth, storedDepth, depth)
				return false
			}
			return true
		}

		// Beyond the limit: the save must be rejected with ErrMaxDepthExceeded
		// and leave the store unchanged (no over-deep folder persisted).
		if !errors.Is(saveErr, ErrMaxDepthExceeded) {
			failMsg = fmt.Sprintf(
				"chain depth %d (> %d) returned err %v, want ErrMaxDepthExceeded",
				depth, maxFolderDepth, saveErr)
			return false
		}
		if storedDepth != 0 {
			failMsg = fmt.Sprintf(
				"chain depth %d (> %d) was rejected but the store retained folders at depth %d, want an unchanged (empty) store",
				depth, maxFolderDepth, storedDepth)
			return false
		}
		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 14 failed: %v\n%s", err, failMsg)
	}
}

// buildLinearFolderChain returns a single-folder slice forming a linear chain of
// total nesting depth d: the returned folder is depth 1, its sole child folder
// is depth 2, and so on down to depth d. A non-positive d yields an empty
// (non-nil) slice. Each folder gets a unique ID via counter and a valid
// (1..255-char) name so only the depth rule is under test.
func buildLinearFolderChain(d int, counter *int) []model.Folder {
	if d <= 0 {
		return []model.Folder{}
	}
	*counter++
	return []model.Folder{{
		ID:       fmt.Sprintf("depth-folder-%d", *counter),
		Name:     "f",
		Requests: []model.SavedRequest{},
		Folders:  buildLinearFolderChain(d-1, counter),
	}}
}

// treeMaxFolderDepth returns the greatest folder nesting depth present anywhere
// in the reconstructed collection tree, counting a folder directly under a
// collection as depth 1. It returns 0 when no folder is present.
func treeMaxFolderDepth(cols []model.Collection) int {
	max := 0
	for _, c := range cols {
		for _, f := range c.Folders {
			if d := folderChainDepth(f); d > max {
				max = d
			}
		}
	}
	return max
}

// folderChainDepth returns the height of the folder subtree rooted at f counting
// f itself as level 1: a folder with no nested folders has depth 1, and the
// depth of a folder is one more than the greatest depth among its children.
func folderChainDepth(f model.Folder) int {
	max := 0
	for _, child := range f.Folders {
		if d := folderChainDepth(child); d > max {
			max = d
		}
	}
	return max + 1
}
