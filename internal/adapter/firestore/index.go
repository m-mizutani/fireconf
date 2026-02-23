package firestore

import (
	"context"
	"fmt"

	adminpb "cloud.google.com/go/firestore/apiv1/admin/adminpb"
	"github.com/m-mizutani/fireconf/internal/interfaces"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListIndexes lists all composite indexes for a collection
func (c *Client) ListIndexes(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
	req := &adminpb.ListIndexesRequest{
		Parent: c.getParent(collectionID),
	}

	var indexes []interfaces.FirestoreIndex
	it := c.admin.ListIndexes(ctx, req)

	for {
		index, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list indexes: %w", err)
		}

		// Extract collection ID from index name
		// Index name format: projects/{project}/databases/{database}/collectionGroups/{collection}/indexes/{index}
		indexCollectionID := extractCollectionFromIndexName(index.GetName())

		// Only include indexes that belong to the requested collection
		if indexCollectionID != collectionID {
			continue
		}

		// Convert to domain model
		firestoreIndex := convertIndexFromAPI(index)
		indexes = append(indexes, firestoreIndex)
	}

	return indexes, nil
}

// CreateIndex creates a new composite index and returns the index resource name.
// Returns an empty string if the index already exists.
func (c *Client) CreateIndex(ctx context.Context, collectionID string, index interfaces.FirestoreIndex) (string, error) {
	// Convert domain model to API format
	apiIndex := convertIndexToAPI(index)

	req := &adminpb.CreateIndexRequest{
		Parent: c.getParent(collectionID),
		Index:  apiIndex,
	}

	op, err := c.admin.CreateIndex(ctx, req)
	if err != nil {
		// Handle already exists error gracefully
		if s, ok := status.FromError(err); ok && s.Code() == codes.AlreadyExists {
			return "", nil // Index already exists, no need to wait
		}
		return "", fmt.Errorf("failed to create index: %w", err)
	}

	// Extract the index resource name from the operation metadata
	meta, err := op.Metadata()
	if err != nil || meta == nil {
		return "", fmt.Errorf("failed to get index operation metadata: %w", err)
	}

	return meta.Index, nil
}

// GetIndex retrieves a single index by its full resource name.
func (c *Client) GetIndex(ctx context.Context, indexName string) (*interfaces.FirestoreIndex, error) {
	req := &adminpb.GetIndexRequest{
		Name: indexName,
	}

	index, err := c.admin.GetIndex(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get index %s: %w", indexName, err)
	}

	result := convertIndexFromAPI(index)
	return &result, nil
}

// DeleteIndex deletes an index by its name
func (c *Client) DeleteIndex(ctx context.Context, indexName string) (interface{}, error) {
	req := &adminpb.DeleteIndexRequest{
		Name: indexName,
	}

	err := c.admin.DeleteIndex(ctx, req)
	if err != nil {
		// Handle not found error gracefully
		if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
			return nil, nil // Index already deleted, consider it success
		}
		return nil, fmt.Errorf("failed to delete index: %w", err)
	}

	// DeleteIndex doesn't return an operation object, just nil for success
	return nil, nil
}

// convertIndexFromAPI converts API index to domain model
func convertIndexFromAPI(index *adminpb.Index) interfaces.FirestoreIndex {
	var fields []interfaces.FirestoreIndexField

	for _, field := range index.GetFields() {
		indexField := interfaces.FirestoreIndexField{
			FieldPath: field.GetFieldPath(),
		}

		// Handle different field types
		switch v := field.GetValueMode().(type) {
		case *adminpb.Index_IndexField_Order_:
			indexField.Order = v.Order.String()
		case *adminpb.Index_IndexField_ArrayConfig_:
			indexField.ArrayConfig = v.ArrayConfig.String()
		case *adminpb.Index_IndexField_VectorConfig_:
			indexField.VectorConfig = &interfaces.FirestoreVectorConfig{
				Dimension: int(v.VectorConfig.GetDimension()),
			}
		}

		fields = append(fields, indexField)
	}

	return interfaces.FirestoreIndex{
		Name:       index.GetName(),
		Fields:     fields,
		QueryScope: index.GetQueryScope().String(),
		State:      index.GetState().String(),
	}
}

// convertIndexToAPI converts domain model to API format
func convertIndexToAPI(index interfaces.FirestoreIndex) *adminpb.Index {
	var fields []*adminpb.Index_IndexField

	for _, field := range index.Fields {
		apiField := &adminpb.Index_IndexField{
			FieldPath: field.FieldPath,
		}

		// Set field type based on what's specified
		// Vector config takes priority over order/array config
		if field.VectorConfig != nil {
			// Validate dimension to prevent integer overflow
			// Firestore vector dimensions are typically small (e.g., 128, 256, 768, 1536)
			if field.VectorConfig.Dimension < 0 || field.VectorConfig.Dimension > 2147483647 {
				// This should never happen with valid vector configs, but check anyway
				continue
			}
			apiField.ValueMode = &adminpb.Index_IndexField_VectorConfig_{
				VectorConfig: &adminpb.Index_IndexField_VectorConfig{
					Dimension: int32(field.VectorConfig.Dimension), // #nosec G115 - validated above
					Type: &adminpb.Index_IndexField_VectorConfig_Flat{
						Flat: &adminpb.Index_IndexField_VectorConfig_FlatIndex{},
					},
				},
			}
		} else if field.Order != "" {
			order := adminpb.Index_IndexField_ASCENDING
			if field.Order == "DESCENDING" {
				order = adminpb.Index_IndexField_DESCENDING
			}
			apiField.ValueMode = &adminpb.Index_IndexField_Order_{
				Order: order,
			}
		} else if field.ArrayConfig != "" {
			apiField.ValueMode = &adminpb.Index_IndexField_ArrayConfig_{
				ArrayConfig: adminpb.Index_IndexField_CONTAINS,
			}
		}

		fields = append(fields, apiField)
	}

	return &adminpb.Index{
		QueryScope: convertQueryScope(index.QueryScope),
		Fields:     fields,
	}
}
