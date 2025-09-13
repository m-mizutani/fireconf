package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/m-mizutani/fireconf/internal/interfaces"
	"github.com/m-mizutani/fireconf/internal/model"
	"github.com/m-mizutani/goerr/v2"
)

// Import handles importing existing Firestore configuration
type Import struct {
	client interfaces.FirestoreClient
	logger *slog.Logger
}

// NewImport creates a new Import use case
func NewImport(client interfaces.FirestoreClient, logger *slog.Logger) *Import {
	return &Import{
		client: client,
		logger: logger,
	}
}

// Execute imports configuration from Firestore
func (i *Import) Execute(ctx context.Context, collections []string) (*model.Config, error) {
	i.logger.Info("Starting import operation", slog.Int("collections", len(collections)))

	config := &model.Config{
		Collections: make([]model.Collection, 0, len(collections)),
	}

	// If no collections specified, discover all collections
	if len(collections) == 0 {
		i.logger.Info("No collections specified, discovering all collections...")
		discovered, err := i.client.ListCollections(ctx)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to discover collections. Please specify collection names explicitly if discovery fails.")
		}
		collections = discovered
		i.logger.Info("Discovered collections", "count", len(collections))
	}

	// Process each collection
	for _, collectionName := range collections {
		i.logger.Info("Importing collection", slog.String("name", collectionName))

		collection := model.Collection{
			Name: collectionName,
		}

		// Import indexes
		indexes, err := i.importIndexes(ctx, collectionName)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to import indexes", goerr.V("collection", collectionName))
		}
		collection.Indexes = indexes

		// Import TTL
		ttl, err := i.importTTL(ctx, collectionName)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to import TTL", goerr.V("collection", collectionName))
		}
		collection.TTL = ttl

		config.Collections = append(config.Collections, collection)
	}

	// Sort collections by name for consistent output
	sort.Slice(config.Collections, func(i, j int) bool {
		return config.Collections[i].Name < config.Collections[j].Name
	})

	i.logger.Info("Import operation completed successfully",
		slog.Int("collections", len(config.Collections)))

	return config, nil
}

// importIndexes imports indexes for a collection
func (i *Import) importIndexes(ctx context.Context, collectionName string) ([]model.Index, error) {
	// List existing indexes
	existing, err := i.client.ListIndexes(ctx, collectionName)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list indexes")
	}

	i.logger.Debug("Found indexes",
		slog.String("collection", collectionName),
		slog.Int("count", len(existing)))

	// Convert to model format and deduplicate
	indexMap := make(map[string]model.Index) // Use string key for deduplication
	for _, idx := range existing {
		// Skip indexes that are not READY
		if idx.State != "READY" {
			i.logger.Warn("Skipping non-ready index",
				slog.String("collection", collectionName),
				slog.String("state", idx.State))
			continue
		}

		modelIndex := convertFirestoreToModelIndex(idx)

		// Adjust field order to comply with Firestore constraints
		modelIndex = adjustFieldOrder(modelIndex)

		// Create a unique key for the index based on its structure
		key := createIndexKey(modelIndex)

		// Only add if we haven't seen this index configuration before
		if _, exists := indexMap[key]; !exists {
			indexMap[key] = modelIndex
		} else {
			i.logger.Debug("Skipping duplicate index",
				slog.String("collection", collectionName),
				slog.String("key", key))
		}
	}

	// Convert map back to slice
	indexes := make([]model.Index, 0, len(indexMap))
	for _, idx := range indexMap {
		indexes = append(indexes, idx)
	}

	// Sort indexes for consistent output
	sort.Slice(indexes, func(i, j int) bool {
		// Sort by number of fields first, then by field names
		if len(indexes[i].Fields) != len(indexes[j].Fields) {
			return len(indexes[i].Fields) < len(indexes[j].Fields)
		}

		// Compare field names
		for k := 0; k < len(indexes[i].Fields); k++ {
			if indexes[i].Fields[k].Name != indexes[j].Fields[k].Name {
				return indexes[i].Fields[k].Name < indexes[j].Fields[k].Name
			}
		}

		return false
	})

	return indexes, nil
}

// importTTL imports TTL configuration for a collection
func (i *Import) importTTL(ctx context.Context, collectionName string) (*model.TTL, error) {
	// Find which field has TTL enabled in this collection
	ttlField, err := i.client.FindTTLField(ctx, collectionName)
	if err != nil {
		// Log the error but continue - TTL is optional
		i.logger.Debug("Failed to find TTL field",
			slog.String("collection", collectionName),
			slog.String("error", err.Error()))
		return nil, nil
	}

	// No TTL field found
	if ttlField == "" {
		i.logger.Debug("No TTL policy found",
			slog.String("collection", collectionName))
		return nil, nil
	}

	// Verify the TTL policy is active
	ttl, err := i.client.GetTTLPolicy(ctx, collectionName, ttlField)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get TTL policy",
			goerr.V("collection", collectionName),
			goerr.V("field", ttlField))
	}

	if ttl != nil && (ttl.State == "ACTIVE" || ttl.State == "CREATING") {
		i.logger.Debug("Found TTL policy",
			slog.String("collection", collectionName),
			slog.String("field", ttlField))

		return &model.TTL{
			Field: ttlField,
		}, nil
	}

	// TTL field exists but not active
	i.logger.Debug("TTL field found but not active",
		slog.String("collection", collectionName),
		slog.String("field", ttlField),
		slog.String("state", ttl.State))

	return nil, nil
}

// createIndexKey creates a unique key for an index based on its structure
func createIndexKey(index model.Index) string {
	key := fmt.Sprintf("scope:%s", index.QueryScope)

	for _, field := range index.Fields {
		fieldKey := fmt.Sprintf("field:%s", field.Name)

		if field.Order != "" {
			fieldKey += fmt.Sprintf(":order:%s", field.Order)
		}

		if field.ArrayConfig != "" {
			fieldKey += fmt.Sprintf(":array:%s", field.ArrayConfig)
		}

		if field.VectorConfig != nil {
			fieldKey += fmt.Sprintf(":vector:%d", field.VectorConfig.Dimension)
		}

		key += ";" + fieldKey
	}

	return key
}

// adjustFieldOrder adjusts field order to comply with Firestore constraints:
// 1. Regular fields (with Order) first
// 2. __name__ field (if not vector index)
// 3. Vector config fields must be last
func adjustFieldOrder(index model.Index) model.Index {
	var regularFields []model.IndexField
	var vectorFields []model.IndexField
	var nameField *model.IndexField

	for _, field := range index.Fields {
		if field.Name == "__name__" {
			nameField = &field
		} else if field.VectorConfig != nil {
			vectorFields = append(vectorFields, field)
		} else {
			regularFields = append(regularFields, field)
		}
	}

	// Reconstruct fields in correct order:
	// For vector indexes: regular fields, __name__, vector fields (last)
	// For non-vector indexes: regular fields, __name__ (last)
	var newFields []model.IndexField
	newFields = append(newFields, regularFields...)

	if len(vectorFields) > 0 {
		// Vector index: __name__ before vector fields
		if nameField != nil {
			newFields = append(newFields, *nameField)
		}
		newFields = append(newFields, vectorFields...)
	} else {
		// Non-vector index: __name__ at the end
		if nameField != nil {
			newFields = append(newFields, *nameField)
		}
	}

	return model.Index{
		Fields:     newFields,
		QueryScope: index.QueryScope,
	}
}
