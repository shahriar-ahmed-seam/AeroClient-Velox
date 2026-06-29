package store

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"volt/internal/model"
)

// Name and value length bounds enforced as a defense-in-depth backstop to the
// frontend's own validation (Req 5.8, 6.1, 6.2, 6.9). Lengths are measured in
// Unicode code points (runes) so multi-byte names are counted the same way the
// user perceives them, matching the frontend character counts.
const (
	// maxCollectionNameLen bounds a Collection name (Req 5.8): 1..255.
	maxCollectionNameLen = 255
	// maxRequestNameLen bounds a saved Request name (Req 5.8): 1..255.
	maxRequestNameLen = 255
	// maxFolderNameLen bounds a Folder name (Req 5.8): 1..255.
	maxFolderNameLen = 255
	// maxEnvironmentNameLen bounds an Environment name (Req 6.1): 1..64.
	maxEnvironmentNameLen = 64
	// maxVariableNameLen bounds a Variable name (Req 6.2): 1..128.
	maxVariableNameLen = 128
	// maxVariableValueLen bounds a Variable value (Req 6.2): 0..4096.
	maxVariableValueLen = 4096
)

// ErrInvalidName is returned when a Collection, Folder, Request, Environment, or
// Variable name is empty or longer than its allowed maximum (Req 5.8, 6.1, 6.2,
// 6.9). It is returned before any write so the attempted save leaves prior data
// unchanged.
var ErrInvalidName = errors.New("store: invalid name")

// ErrInvalidValue is returned when a Variable value exceeds its allowed maximum
// length (Req 6.2). It is returned before any write so prior data is unchanged.
var ErrInvalidValue = errors.New("store: invalid value")

// ErrDuplicateName is returned when a name must be unique within its scope but
// collides with an existing one: an Environment name among Environments, or a
// Variable name within its Environment (Req 6.2, 6.9). It is returned before any
// write so prior data is left unchanged.
var ErrDuplicateName = errors.New("store: duplicate name")

// validateNameLen reports ErrInvalidName unless name's rune length is within
// 1..max inclusive. It is the shared core of the name validators below.
func validateNameLen(name string, max int) error {
	n := utf8.RuneCountInString(name)
	if n < 1 || n > max {
		return fmt.Errorf("%w: length %d not in 1..%d", ErrInvalidName, n, max)
	}
	return nil
}

// validateCollectionName enforces the 1..255 Collection name bound (Req 5.8).
func validateCollectionName(name string) error {
	return validateNameLen(name, maxCollectionNameLen)
}

// validateRequestName enforces the 1..255 saved-Request name bound (Req 5.8).
func validateRequestName(name string) error {
	return validateNameLen(name, maxRequestNameLen)
}

// validateFolderName enforces the 1..255 Folder name bound (Req 5.8).
func validateFolderName(name string) error {
	return validateNameLen(name, maxFolderNameLen)
}

// validateEnvironmentName enforces the 1..64 Environment name bound (Req 6.1).
// Cross-Environment uniqueness is enforced separately by the environment CRUD
// layer, which has the transaction needed to consult existing rows.
func validateEnvironmentName(name string) error {
	return validateNameLen(name, maxEnvironmentNameLen)
}

// validateVariableName enforces the 1..128 Variable name bound (Req 6.2).
func validateVariableName(name string) error {
	return validateNameLen(name, maxVariableNameLen)
}

// validateVariableValue enforces the 0..4096 Variable value bound (Req 6.2). An
// empty value is allowed; only an over-long value is rejected.
func validateVariableValue(value string) error {
	if n := utf8.RuneCountInString(value); n > maxVariableValueLen {
		return fmt.Errorf("%w: value length %d exceeds %d", ErrInvalidValue, n, maxVariableValueLen)
	}
	return nil
}

// validateVariable validates a single Variable's name and value (Req 6.2). It is
// a reusable helper for the environment CRUD layer (task 9.1).
func validateVariable(v model.Variable) error {
	if err := validateVariableName(v.Name); err != nil {
		return err
	}
	return validateVariableValue(v.Value)
}

// validateEnvironment validates an Environment's name plus all of its Variables,
// including the in-memory uniqueness of Variable names within the Environment
// (Req 6.1, 6.2, 6.9). Uniqueness of the Environment name among other
// Environments depends on stored state and is enforced by the environment CRUD
// layer (task 9.1); this helper covers everything checkable from the value
// alone. It is provided here so task 9.1 can call it.
func validateEnvironment(e model.Environment) error {
	if err := validateEnvironmentName(e.Name); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(e.Variables))
	for i := range e.Variables {
		v := e.Variables[i]
		if err := validateVariable(v); err != nil {
			return err
		}
		if _, dup := seen[v.Name]; dup {
			return fmt.Errorf("%w: variable %q", ErrDuplicateName, v.Name)
		}
		seen[v.Name] = struct{}{}
	}
	return nil
}

// validateFolderTree validates a Folder's name and, recursively, the names of
// every saved Request it carries and every nested Folder subtree (Req 5.8). It
// returns the first violation found so a save can be rejected before any write.
func validateFolderTree(f *model.Folder) error {
	if err := validateFolderName(f.Name); err != nil {
		return err
	}
	for i := range f.Requests {
		if err := validateRequestName(f.Requests[i].Name); err != nil {
			return err
		}
	}
	for i := range f.Folders {
		if err := validateFolderTree(&f.Folders[i]); err != nil {
			return err
		}
	}
	return nil
}

// validateCollectionTree validates a Collection's name and, recursively, the
// names of every saved Request and Folder subtree it carries (Req 5.8). It
// returns the first violation found so SaveCollection can reject the save before
// touching the database, leaving prior data unchanged.
func validateCollectionTree(c *model.Collection) error {
	if err := validateCollectionName(c.Name); err != nil {
		return err
	}
	for i := range c.Requests {
		if err := validateRequestName(c.Requests[i].Name); err != nil {
			return err
		}
	}
	for i := range c.Folders {
		if err := validateFolderTree(&c.Folders[i]); err != nil {
			return err
		}
	}
	return nil
}
