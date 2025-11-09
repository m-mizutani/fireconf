package usecase

import (
	"fmt"
	"sort"
	"strings"

	"github.com/m-mizutani/fireconf/internal/interfaces"
	"github.com/m-mizutani/fireconf/internal/model"
)

// DiffIndexes compares desired and existing indexes and returns what needs to be created/deleted
func DiffIndexes(desired []model.Index, existing []interfaces.FirestoreIndex) (toCreate, toDelete []interfaces.FirestoreIndex) {
	// Create maps for easier comparison
	desiredMap := make(map[string]model.Index)
	existingMap := make(map[string]interfaces.FirestoreIndex)

	// Build desired map
	for _, idx := range desired {
		key := getIndexKey(convertModelToFirestoreIndex(idx))
		desiredMap[key] = idx
	}

	// Build existing map and find indexes to delete
	for _, idx := range existing {
		key := getIndexKey(idx)
		existingMap[key] = idx

		if _, found := desiredMap[key]; !found {
			toDelete = append(toDelete, idx)
		}
	}

	// Find indexes to create
	for key, idx := range desiredMap {
		if _, found := existingMap[key]; !found {
			toCreate = append(toCreate, convertModelToFirestoreIndex(idx))
		}
	}

	return toCreate, toDelete
}

// DiffTTL compares desired and existing TTL configuration
func DiffTTL(desired *model.TTL, existing *interfaces.FirestoreTTL) (needsUpdate bool, action string) {
	// No TTL desired
	if desired == nil {
		if existing != nil && (existing.State == "ACTIVE" || existing.State == "CREATING") {
			return true, "disable"
		}
		return false, ""
	}

	// TTL desired
	if existing == nil || (existing.State != "ACTIVE" && existing.State != "CREATING") {
		return true, "enable"
	}

	// Check if field changed
	if existing.FieldPath != desired.Field {
		return true, "change"
	}

	return false, ""
}

// getIndexKey generates a unique key for an index based on its fields and scope
func getIndexKey(idx interfaces.FirestoreIndex) string {
	var parts []string

	// Add query scope
	parts = append(parts, idx.QueryScope)

	// Sort fields to ensure consistent key generation
	fieldKeys := make([]string, 0, len(idx.Fields))
	for _, field := range idx.Fields {
		var fieldKey string
		if field.Order != "" {
			fieldKey = fmt.Sprintf("%s:%s", field.FieldPath, field.Order)
		} else if field.ArrayConfig != "" {
			fieldKey = fmt.Sprintf("%s:ARRAY_%s", field.FieldPath, field.ArrayConfig)
		} else if field.VectorConfig != nil {
			fieldKey = fmt.Sprintf("%s:VECTOR_%d", field.FieldPath, field.VectorConfig.Dimension)
		}
		fieldKeys = append(fieldKeys, fieldKey)
	}

	// Sort field keys for consistent comparison
	sort.Strings(fieldKeys)
	parts = append(parts, fieldKeys...)

	return strings.Join(parts, "|")
}

// convertModelToFirestoreIndex converts domain model to Firestore interface
func convertModelToFirestoreIndex(idx model.Index) interfaces.FirestoreIndex {
	firestoreIndex := interfaces.FirestoreIndex{
		QueryScope: idx.GetQueryScope(),
		Fields:     make([]interfaces.FirestoreIndexField, 0, len(idx.Fields)),
	}

	for _, field := range idx.Fields {
		firestoreField := interfaces.FirestoreIndexField{
			FieldPath: field.Name,
		}

		// Handle VectorConfig first (takes priority)
		if field.VectorConfig != nil {
			firestoreField.VectorConfig = &interfaces.FirestoreVectorConfig{
				Dimension: field.VectorConfig.Dimension,
			}
		} else if field.Order != "" {
			firestoreField.Order = field.Order
		} else if field.ArrayConfig != "" {
			firestoreField.ArrayConfig = field.ArrayConfig
		}

		firestoreIndex.Fields = append(firestoreIndex.Fields, firestoreField)
	}

	return firestoreIndex
}

// convertFirestoreToModelIndex converts Firestore index to domain model
func convertFirestoreToModelIndex(idx interfaces.FirestoreIndex) model.Index {
	modelIndex := model.Index{
		QueryScope: idx.QueryScope,
		Fields:     make([]model.IndexField, 0, len(idx.Fields)),
	}

	for _, field := range idx.Fields {
		modelField := model.IndexField{
			Name: field.FieldPath,
		}

		// Handle VectorConfig first (takes priority)
		if field.VectorConfig != nil {
			modelField.VectorConfig = &model.VectorConfig{
				Dimension: field.VectorConfig.Dimension,
			}
			// For convenience, also set order for vector fields to help with YAML generation
			modelField.Order = "ASCENDING"
		} else if field.Order != "" {
			modelField.Order = field.Order
		} else if field.ArrayConfig != "" {
			modelField.ArrayConfig = field.ArrayConfig
		}

		modelIndex.Fields = append(modelIndex.Fields, modelField)
	}

	return modelIndex
}
