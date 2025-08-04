package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/m-mizutani/fireconf/pkg/domain/model"
	"github.com/m-mizutani/goerr/v2"
)

// Validator handles validation of Firestore configuration
type Validator struct {
	logger *slog.Logger
}

// NewValidator creates a new Validator use case
func NewValidator(logger *slog.Logger) *Validator {
	return &Validator{
		logger: logger,
	}
}

// Execute validates the configuration against Firestore constraints
func (v *Validator) Execute(ctx context.Context, config *model.Config) error {
	v.logger.Info("Starting validation")

	// Validate each collection
	for _, collection := range config.Collections {
		v.logger.Info("Validating collection", slog.String("name", collection.Name))

		// Basic validation
		if err := collection.Validate(); err != nil {
			return goerr.Wrap(err, "invalid collection configuration", goerr.V("collection", collection.Name))
		}

		// Firestore-specific constraint validation
		if err := v.validateFirestoreConstraints(collection); err != nil {
			return goerr.Wrap(err, "Firestore constraint violation", goerr.V("collection", collection.Name))
		}
	}

	v.logger.Info("Validation completed successfully")
	return nil
}

// validateFirestoreConstraints validates Firestore-specific constraints
func (v *Validator) validateFirestoreConstraints(collection model.Collection) error {
	for i, index := range collection.Indexes {
		if err := v.validateIndexConstraints(index, i); err != nil {
			return err
		}
	}

	if collection.TTL != nil {
		if err := v.validateTTLConstraints(*collection.TTL); err != nil {
			return err
		}
	}

	return nil
}

// validateIndexConstraints validates index-specific constraints
func (v *Validator) validateIndexConstraints(index model.Index, indexNum int) error {
	fields := index.Fields

	// Check field order constraints
	var nameFieldIndex = -1
	var vectorFieldIndices []int

	for i, field := range fields {
		if field.Name == "__name__" {
			nameFieldIndex = i
		}
		if field.VectorConfig != nil {
			vectorFieldIndices = append(vectorFieldIndices, i)
		}
	}

	// Constraint 1: __name__ field positioning
	// - For non-vector indexes: __name__ must be last
	// - For vector indexes: __name__ must be before vector fields
	if nameFieldIndex >= 0 {
		if len(vectorFieldIndices) == 0 {
			// Non-vector index: __name__ must be last
			if nameFieldIndex != len(fields)-1 {
				return fmt.Errorf("index[%d]: __name__ field must be last in non-vector index", indexNum)
			}
		} else {
			// Vector index: __name__ must not be after vector fields
			for _, vectorIndex := range vectorFieldIndices {
				if nameFieldIndex > vectorIndex {
					return fmt.Errorf("index[%d]: __name__ field must be before vector config fields", indexNum)
				}
			}
		}
	}

	// Constraint 2: Vector fields must be at the very end of the index
	if len(vectorFieldIndices) > 0 {
		// Vector fields must be after __name__ field
		if nameFieldIndex >= 0 {
			for _, vectorIndex := range vectorFieldIndices {
				if vectorIndex < nameFieldIndex {
					return fmt.Errorf("index[%d]: vector config field '%s' must be after __name__ field",
						indexNum, fields[vectorIndex].Name)
				}
			}
		}

		// All vector fields must be at the end
		expectedStart := len(fields) - len(vectorFieldIndices)
		for j, vectorIndex := range vectorFieldIndices {
			expectedIndex := expectedStart + j
			if vectorIndex != expectedIndex {
				return fmt.Errorf("index[%d]: vector config field '%s' must be at the end of the index (expected position %d, got %d)",
					indexNum, fields[vectorIndex].Name, expectedIndex, vectorIndex)
			}
		}
	}

	// Constraint 3: Index must have at least one field
	if len(fields) == 0 {
		return fmt.Errorf("index[%d]: index must have at least one field", indexNum)
	}

	// Constraint 4: Single-field indexes with just __name__ are not allowed for composite indexes
	if len(fields) == 1 && fields[0].Name == "__name__" {
		return fmt.Errorf("index[%d]: single-field index on __name__ is not necessary", indexNum)
	}

	// Constraint 5: Check for duplicate field names
	fieldNames := make(map[string]bool)
	for _, field := range fields {
		if fieldNames[field.Name] {
			return fmt.Errorf("index[%d]: duplicate field name '%s'", indexNum, field.Name)
		}
		fieldNames[field.Name] = true
	}

	// Constraint 6: Vector config dimension must be positive
	for _, field := range fields {
		if field.VectorConfig != nil && field.VectorConfig.Dimension <= 0 {
			return fmt.Errorf("index[%d]: vector dimension must be positive for field '%s'", indexNum, field.Name)
		}
	}

	return nil
}

// validateTTLConstraints validates TTL-specific constraints
func (v *Validator) validateTTLConstraints(ttl model.TTL) error {
	// Constraint 1: TTL field name must not be empty
	if ttl.Field == "" {
		return fmt.Errorf("TTL field name cannot be empty")
	}

	// Constraint 2: TTL field should not be a reserved field
	reservedFields := []string{"__name__"}
	for _, reserved := range reservedFields {
		if ttl.Field == reserved {
			return fmt.Errorf("TTL field '%s' cannot be a reserved field", ttl.Field)
		}
	}

	return nil
}

