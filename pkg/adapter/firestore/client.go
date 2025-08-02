package firestore

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"
	apiv1 "cloud.google.com/go/firestore/apiv1/admin"
	adminpb "cloud.google.com/go/firestore/apiv1/admin/adminpb"
	"github.com/m-mizutani/fireconf/pkg/domain/interfaces"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Client is the Firestore Admin API client wrapper
type Client struct {
	admin      *apiv1.FirestoreAdminClient
	client     *firestore.Client
	projectID  string
	databaseID string
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	ProjectID   string
	DatabaseID  string
	Credentials string // Service account key file path (optional)
}

// NewClient creates a new Firestore Admin API client
func NewClient(ctx context.Context, config AuthConfig) (interfaces.FirestoreClient, error) {
	// Use ADC or explicit credentials
	var opts []option.ClientOption
	if config.Credentials != "" {
		opts = append(opts, option.WithCredentialsFile(config.Credentials))
	}

	// Create Admin API client
	adminClient, err := apiv1.NewFirestoreAdminClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore Admin client: %w", err)
	}

	// Create regular Firestore client for collection listing (only for default database)
	var firestoreClient *firestore.Client
	if config.DatabaseID == "" || config.DatabaseID == "(default)" {
		firestoreClient, err = firestore.NewClient(ctx, config.ProjectID, opts...)
		if err != nil {
			// Don't fail if we can't create the regular client, just log and continue
			// Collection listing won't be available but other operations will work
			firestoreClient = nil
		}
	}

	// Set default database ID
	if config.DatabaseID == "" {
		config.DatabaseID = "(default)"
	}

	return &Client{
		admin:      adminClient,
		client:     firestoreClient,
		projectID:  config.ProjectID,
		databaseID: config.DatabaseID,
	}, nil
}

// Close closes the client
func (c *Client) Close() error {
	if c.client != nil {
		if err := c.client.Close(); err != nil {
			return err
		}
	}
	return c.admin.Close()
}

// ListCollections lists all collection IDs in the database by discovering them through indexes
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	collectionMap := make(map[string]bool)

	// Use Admin API to list all indexes and extract collection names
	req := &adminpb.ListIndexesRequest{
		Parent: fmt.Sprintf("projects/%s/databases/%s/collectionGroups/-", c.projectID, c.databaseID),
	}

	it := c.admin.ListIndexes(ctx, req)
	for {
		index, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list indexes: %w", err)
		}

		// Extract collection name from index name
		// Format: projects/{project}/databases/{database}/collectionGroups/{collection}/indexes/{index}
		if collectionID := extractCollectionFromIndexName(index.GetName()); collectionID != "" && collectionID != "-" {
			collectionMap[collectionID] = true
		}
	}

	// If we found collections through indexes, return them
	if len(collectionMap) > 0 {
		var collections []string
		for col := range collectionMap {
			collections = append(collections, col)
		}
		return collections, nil
	}

	// Fallback: try the regular client for default database only
	if c.client != nil && (c.databaseID == "(default)" || c.databaseID == "") {
		iter := c.client.Collections(ctx)
		for {
			col, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to list collections: %w", err)
			}
			collectionMap[col.ID] = true
		}
	}

	// Convert map to slice
	var collections []string
	for col := range collectionMap {
		collections = append(collections, col)
	}

	return collections, nil
}

// extractCollectionFromIndexName extracts collection ID from index name
func extractCollectionFromIndexName(indexName string) string {
	// Example: projects/PROJECT/databases/DATABASE/collectionGroups/COLLECTION/indexes/INDEX
	parts := strings.Split(indexName, "/")
	for i, part := range parts {
		if part == "collectionGroups" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// getParent returns the parent path for collection groups
func (c *Client) getParent(collectionID string) string {
	return fmt.Sprintf("projects/%s/databases/%s/collectionGroups/%s",
		c.projectID, c.databaseID, collectionID)
}

// getFieldPath returns the field path
func (c *Client) getFieldPath(collectionID, fieldName string) string {
	return fmt.Sprintf("projects/%s/databases/%s/collectionGroups/%s/fields/%s",
		c.projectID, c.databaseID, collectionID, fieldName)
}

// WaitForOperation waits for a long-running operation to complete
func (c *Client) WaitForOperation(ctx context.Context, operation interface{}) error {
	switch op := operation.(type) {
	case *apiv1.CreateIndexOperation:
		_, err := op.Wait(ctx)
		return err
	// DeleteIndex operation doesn't have a separate type, returns error directly
	case *apiv1.UpdateFieldOperation:
		_, err := op.Wait(ctx)
		return err
	default:
		return fmt.Errorf("unknown operation type: %T", operation)
	}
}

// convertQueryScope converts internal query scope to API format
func convertQueryScope(scope string) adminpb.Index_QueryScope {
	switch scope {
	case "COLLECTION_GROUP":
		return adminpb.Index_COLLECTION_GROUP
	default:
		return adminpb.Index_COLLECTION
	}
}
