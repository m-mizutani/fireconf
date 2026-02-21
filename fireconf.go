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
	config    *Config
	options   *options
	logger    *slog.Logger
}

// New creates a new fireconf client with the desired configuration
func New(ctx context.Context, projectID, databaseID string, config *Config, opts ...Option) (*Client, error) {
	options := applyOptions(opts)

	// Validate required parameters
	if projectID == "" {
		return nil, goerr.New("project ID is required")
	}
	if databaseID == "" {
		return nil, goerr.New("database ID is required")
	}

	// Create Firestore client
	authConfig := firestore.AuthConfig{
		ProjectID:   projectID,
		DatabaseID:  databaseID,
		Credentials: options.CredentialsFile,
	}

	firestoreClient, err := firestore.NewClient(ctx, authConfig)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create Firestore client")
	}

	return &Client{
		projectID: projectID,
		client:    firestoreClient,
		config:    config,
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

// Migrate applies the configuration to Firestore
func (c *Client) Migrate(ctx context.Context) error {
	if c.config == nil {
		return goerr.New("config is required for Migrate; pass it to New()")
	}

	// Validate configuration
	if err := c.config.Validate(); err != nil {
		return goerr.Wrap(err, "invalid configuration")
	}

	// Convert to internal model
	internalConfig := convertToInternalConfig(c.config)

	// Create sync use case
	syncOpts := []usecase.SyncOption{}
	if c.options.DryRun {
		syncOpts = append(syncOpts, usecase.SyncWithDryRun())
	}
	sync := usecase.NewSync(c.client, c.logger, syncOpts...)

	// Execute sync
	if err := sync.Execute(ctx, internalConfig); err != nil {
		return &MigrationError{Operation: "migrate", Cause: err}
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

// DiffConfigs compares the current configuration against the desired configuration set in New
func (c *Client) DiffConfigs(current *Config) (*DiffResult, error) {
	if current == nil {
		return nil, &DiffError{Details: []string{"current config is nil"}}
	}
	currentInternal := convertToInternalConfig(current)
	desired := c.config
	if desired == nil {
		desired = &Config{}
	}
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

			// Compare indexes - use simple comparison for now
			// This is a simplified diff that compares index counts
			indexChanges := len(desiredCol.Indexes) != len(currentCol.Indexes)
			if indexChanges {
				diff.IndexesToAdd = convertIndexesToPublic(desiredCol.Indexes)
				diff.IndexesToDelete = convertIndexesToPublic(currentCol.Indexes)
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
			if indexChanges || diff.TTLAction != "" {
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

	return result, nil
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
