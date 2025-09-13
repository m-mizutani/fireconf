package fireconf

import "fmt"

// MigrationError represents an error that occurred during migration
type MigrationError struct {
	Collection string
	Operation  string
	Cause      error
}

func (e *MigrationError) Error() string {
	return fmt.Sprintf("migration error in collection %s during %s: %v", e.Collection, e.Operation, e.Cause)
}

func (e *MigrationError) Unwrap() error {
	return e.Cause
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error in field %s: %s", e.Field, e.Message)
}

// DiffError represents an error during diff calculation
type DiffError struct {
	Details []string
}

func (e *DiffError) Error() string {
	return fmt.Sprintf("diff error: %v", e.Details)
}
