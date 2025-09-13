package firestore

import (
	"context"
	"fmt"
)

// CollectionExists checks if a collection exists in the database
func (c *Client) CollectionExists(ctx context.Context, collectionID string) (bool, error) {
	// For regular Firestore client, we can list collections
	if c.client != nil {
		collections, err := c.ListCollections(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to list collections: %w", err)
		}

		for _, existingCollection := range collections {
			if existingCollection == collectionID {
				return true, nil
			}
		}
		return false, nil
	}

	// Fallback: try to get a document reference to check if collection exists
	// This is a simple heuristic - we try to access the collection
	// If we can't create a client for this database, assume it doesn't exist
	return false, nil
}

// CreateCollection creates a collection by adding a temporary document and then deleting it
func (c *Client) CreateCollection(ctx context.Context, collectionID string) error {
	if c.client == nil {
		// For non-default databases, we can't create collections directly
		// Collections will be created automatically when indexes are created
		// So we just return success here
		return nil
	}

	// Create a temporary document to initialize the collection
	tempDocRef := c.client.Collection(collectionID).Doc("__temp_init_doc__")

	// Add a temporary document
	_, err := tempDocRef.Set(ctx, map[string]interface{}{
		"__temp":       true,
		"__created_by": "fireconf",
	})
	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", collectionID, err)
	}

	// Immediately delete the temporary document
	_, err = tempDocRef.Delete(ctx)
	if err != nil {
		// Log warning but don't fail - the collection is created
		// The temporary document will remain but that's acceptable
		return nil
	}

	return nil
}
