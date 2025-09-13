package fireconf

import (
	"context"
	"log/slog"

	"github.com/m-mizutani/fireconf/internal/adapter/firestore"
	"github.com/m-mizutani/fireconf/internal/interfaces"
	"github.com/m-mizutani/fireconf/internal/model"
	"github.com/m-mizutani/fireconf/internal/usecase"
	"github.com/m-mizutani/goerr/v2"
)

// Client is the main client for fireconf operations
type Client struct {
	projectID string
	client    interfaces.FirestoreClient
	options   *Options
	logger    *slog.Logger
}

// NewClient creates a new fireconf client
func NewClient(ctx context.Context, projectID string, opts ...Option) (*Client, error) {
	options := applyOptions(opts)

	// Create Firestore client
	config := firestore.AuthConfig{
		ProjectID:   projectID,
		DatabaseID:  options.DatabaseID,
		Credentials: options.CredentialsFile,
	}

	firestoreClient, err := firestore.NewClient(ctx, config)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create Firestore client")
	}

	return &Client{
		projectID: projectID,
		client:    firestoreClient,
		options:   options,
		logger:    options.Logger,
	}, nil
}

// Close closes the client
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Migrate applies configuration to Firestore
func (c *Client) Migrate(ctx context.Context, config *Config) error {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return goerr.Wrap(err, "invalid configuration")
	}

	// Convert to internal model
	internalConfig := convertToInternalConfig(config)

	// Create sync use case
	sync := usecase.NewSync(c.client, c.logger, c.options.DryRun)

	// Execute sync
	if err := sync.Execute(ctx, internalConfig); err != nil {
		return goerr.Wrap(err, "migration failed")
	}

	return nil
}

// Import retrieves current configuration from Firestore
func (c *Client) Import(ctx context.Context, collections ...string) (*Config, error) {
	// Create import use case
	imp := usecase.NewImport(c.client, c.logger)

	// Execute import
	internalConfig, err := imp.Execute(ctx, collections)
	if err != nil {
		return nil, goerr.Wrap(err, "import failed")
	}

	// Convert to public API
	config := convertFromInternalConfig(internalConfig)
	return config, nil
}

// Diff compares configurations
func Diff(current, desired *Config) *DiffResult {
	currentInternal := convertToInternalConfig(current)
	desiredInternal := convertToInternalConfig(desired)

	result := &DiffResult{
		Collections: make([]CollectionDiff, 0),
	}

	// Create maps for easier comparison
	currentMap := make(map[string]model.Collection)
	desiredMap := make(map[string]model.Collection)

	for _, col := range currentInternal.Collections {
		currentMap[col.Name] = col
	}

	for _, col := range desiredInternal.Collections {
		desiredMap[col.Name] = col
	}

	// Check for collections to add or modify
	for name, desiredCol := range desiredMap {
		currentCol, exists := currentMap[name]

		if !exists {
			// Collection to add
			result.Collections = append(result.Collections, CollectionDiff{
				Name:    name,
				Action:  ActionAdd,
				Indexes: convertIndexesToPublic(desiredCol.Indexes),
				TTL:     convertTTLToPublic(desiredCol.TTL),
			})
		} else {
			// Compare indexes and TTL
			diff := CollectionDiff{
				Name:   name,
				Action: ActionModify,
			}

			// Compare indexes
			toCreate, toDelete := usecase.DiffIndexes(desiredCol.Indexes, convertIndexesToInternal(currentCol.Indexes))
			if len(toCreate) > 0 || len(toDelete) > 0 {
				diff.IndexesToAdd = convertInternalIndexesToPublic(toCreate)
				diff.IndexesToDelete = convertInternalIndexesToPublic(toDelete)
			}

			// Compare TTL
			if (desiredCol.TTL == nil) != (currentCol.TTL == nil) ||
				(desiredCol.TTL != nil && currentCol.TTL != nil && desiredCol.TTL.Field != currentCol.TTL.Field) {
				diff.TTL = convertTTLToPublic(desiredCol.TTL)
				diff.TTLAction = ActionModify
				if desiredCol.TTL == nil {
					diff.TTLAction = ActionDelete
				} else if currentCol.TTL == nil {
					diff.TTLAction = ActionAdd
				}
			}

			// Only add to result if there are changes
			if len(diff.IndexesToAdd) > 0 || len(diff.IndexesToDelete) > 0 || diff.TTLAction != "" {
				result.Collections = append(result.Collections, diff)
			}
		}
	}

	// Check for collections to delete
	for name := range currentMap {
		if _, exists := desiredMap[name]; !exists {
			result.Collections = append(result.Collections, CollectionDiff{
				Name:   name,
				Action: ActionDelete,
			})
		}
	}

	return result
}

// DiffResult represents the difference between configurations
type DiffResult struct {
	Collections []CollectionDiff
}

// CollectionDiff represents differences in a collection
type CollectionDiff struct {
	Name            string
	Action          DiffAction
	Indexes         []Index
	IndexesToAdd    []Index
	IndexesToDelete []Index
	TTL             *TTL
	TTLAction       DiffAction
}

// DiffAction represents the type of change
type DiffAction string

const (
	ActionAdd    DiffAction = "ADD"
	ActionModify DiffAction = "MODIFY"
	ActionDelete DiffAction = "DELETE"
)

// Helper functions for conversion

func convertIndexesToPublic(indexes []model.Index) []Index {
	result := make([]Index, len(indexes))
	for i, idx := range indexes {
		result[i] = Index{
			Fields:     convertFieldsToPublic(idx.Fields),
			QueryScope: QueryScope(idx.QueryScope),
		}
	}
	return result
}

func convertFieldsToPublic(fields []model.IndexField) []IndexField {
	result := make([]IndexField, len(fields))
	for i, field := range fields {
		result[i] = IndexField{
			Path:  field.Name,
			Order: Order(field.Order),
			Array: ArrayConfig(field.ArrayConfig),
		}
		if field.VectorConfig != nil {
			result[i].Vector = &VectorConfig{
				Dimension: field.VectorConfig.Dimension,
			}
		}
	}
	return result
}

func convertTTLToPublic(ttl *model.TTL) *TTL {
	if ttl == nil {
		return nil
	}
	return &TTL{
		Field: ttl.Field,
	}
}

func convertIndexesToInternal(indexes []model.Index) []interfaces.FirestoreIndex {
	result := make([]interfaces.FirestoreIndex, len(indexes))
	for i, idx := range indexes {
		result[i] = interfaces.FirestoreIndex{
			Fields:     convertFieldsToInternal(idx.Fields),
			QueryScope: idx.QueryScope,
		}
	}
	return result
}

func convertFieldsToInternal(fields []model.IndexField) []interfaces.FirestoreIndexField {
	result := make([]interfaces.FirestoreIndexField, len(fields))
	for i, field := range fields {
		result[i] = interfaces.FirestoreIndexField{
			FieldPath:   field.Name,
			Order:       field.Order,
			ArrayConfig: field.ArrayConfig,
		}
		if field.VectorConfig != nil {
			result[i].VectorConfig = &interfaces.FirestoreVectorConfig{
				Dimension: field.VectorConfig.Dimension,
			}
		}
	}
	return result
}

func convertInternalIndexesToPublic(indexes []interfaces.FirestoreIndex) []Index {
	result := make([]Index, len(indexes))
	for i, idx := range indexes {
		result[i] = Index{
			Fields:     convertInternalFieldsToPublic(idx.Fields),
			QueryScope: QueryScope(idx.QueryScope),
		}
	}
	return result
}

func convertInternalFieldsToPublic(fields []interfaces.FirestoreIndexField) []IndexField {
	result := make([]IndexField, len(fields))
	for i, field := range fields {
		result[i] = IndexField{
			Path:  field.FieldPath,
			Order: Order(field.Order),
			Array: ArrayConfig(field.ArrayConfig),
		}
		if field.VectorConfig != nil {
			result[i].Vector = &VectorConfig{
				Dimension: field.VectorConfig.Dimension,
			}
		}
	}
	return result
}
