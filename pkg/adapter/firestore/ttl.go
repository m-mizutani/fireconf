package firestore

import (
	"context"
	"fmt"

	adminpb "cloud.google.com/go/firestore/apiv1/admin/adminpb"
	"github.com/m-mizutani/fireconf/pkg/domain/interfaces"
	"google.golang.org/api/iterator"
	fieldmaskpb "google.golang.org/genproto/protobuf/field_mask"
)

// GetTTLPolicy gets the TTL policy for a specific field
func (c *Client) GetTTLPolicy(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
	// List fields with TTL configuration
	req := &adminpb.ListFieldsRequest{
		Parent: c.getParent(collectionID),
		Filter: "ttlConfig:*",
	}

	it := c.admin.ListFields(ctx, req)
	for {
		field, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list TTL policies: %w", err)
		}

		// Check if this is the field we're looking for
		if getFieldNameFromPath(field.GetName()) == fieldName {
			ttlConfig := field.GetTtlConfig()
			if ttlConfig != nil {
				return &interfaces.FirestoreTTL{
					FieldPath: fieldName,
					State:     ttlConfig.GetState().String(),
				}, nil
			}
		}
	}

	// No TTL policy found for this field
	return nil, nil
}

// EnableTTLPolicy enables TTL policy on a field
func (c *Client) EnableTTLPolicy(ctx context.Context, collectionID string, fieldName string) (interface{}, error) {
	fieldPath := c.getFieldPath(collectionID, fieldName)

	// First, disable indexing on the TTL field to avoid hotspots
	if err := c.disableIndexOnTTLField(ctx, collectionID, fieldName); err != nil {
		return nil, fmt.Errorf("failed to disable index on TTL field: %w", err)
	}

	// Enable TTL policy
	req := &adminpb.UpdateFieldRequest{
		Field: &adminpb.Field{
			Name: fieldPath,
			TtlConfig: &adminpb.Field_TtlConfig{
				State: adminpb.Field_TtlConfig_CREATING,
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"ttl_config"},
		},
	}

	op, err := c.admin.UpdateField(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to enable TTL policy: %w", err)
	}

	return op, nil
}

// DisableTTLPolicy disables TTL policy for a collection
func (c *Client) DisableTTLPolicy(ctx context.Context, collectionID string) (interface{}, error) {
	// First, find which field has TTL enabled
	ttlField, err := c.findTTLField(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	if ttlField == "" {
		// No TTL policy to disable
		return nil, nil
	}

	fieldPath := c.getFieldPath(collectionID, ttlField)

	// To disable TTL, we set the ttl_config to nil
	req := &adminpb.UpdateFieldRequest{
		Field: &adminpb.Field{
			Name:      fieldPath,
			TtlConfig: nil, // Setting to nil disables TTL
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"ttl_config"},
		},
	}

	op, err := c.admin.UpdateField(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to disable TTL policy: %w", err)
	}

	return op, nil
}

// disableIndexOnTTLField disables single-field index on TTL field to avoid hotspots
func (c *Client) disableIndexOnTTLField(ctx context.Context, collectionID string, fieldName string) error {
	fieldPath := c.getFieldPath(collectionID, fieldName)

	req := &adminpb.UpdateFieldRequest{
		Field: &adminpb.Field{
			Name: fieldPath,
			IndexConfig: &adminpb.Field_IndexConfig{
				Indexes: []*adminpb.Index{}, // Empty means no single-field indexes
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"index_config"},
		},
	}

	op, err := c.admin.UpdateField(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update field index config: %w", err)
	}

	// Wait for the operation to complete
	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("field index update failed: %w", err)
	}

	return nil
}

// findTTLField finds which field has TTL enabled in a collection
func (c *Client) findTTLField(ctx context.Context, collectionID string) (string, error) {
	req := &adminpb.ListFieldsRequest{
		Parent: c.getParent(collectionID),
		Filter: "ttlConfig:*",
	}

	it := c.admin.ListFields(ctx, req)
	for {
		field, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to list TTL policies: %w", err)
		}

		ttlConfig := field.GetTtlConfig()
		if ttlConfig != nil && (ttlConfig.GetState() == adminpb.Field_TtlConfig_ACTIVE || ttlConfig.GetState() == adminpb.Field_TtlConfig_CREATING) {
			return getFieldNameFromPath(field.GetName()), nil
		}
	}

	return "", nil
}

// getFieldNameFromPath extracts field name from full resource path
func getFieldNameFromPath(path string) string {
	// Path format: projects/{project}/databases/{database}/collectionGroups/{collection}/fields/{field}
	parts := split(path, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
}

// split is a simple string split function
func split(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
