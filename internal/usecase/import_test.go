package usecase_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/m-mizutani/fireconf/internal/interfaces"
	"github.com/m-mizutani/fireconf/internal/interfaces/mock"
	"github.com/m-mizutani/fireconf/internal/usecase"
	"github.com/m-mizutani/gt"
)

func TestImport_Execute(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	t.Run("Normal: import single collection", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				gt.Equal(t, collectionID, "users")
				return []interfaces.FirestoreIndex{
					{
						Name: "projects/test/databases/default/collectionGroups/users/indexes/idx1",
						Fields: []interfaces.FirestoreIndexField{
							{FieldPath: "email", Order: "ASCENDING"},
							{FieldPath: "createdAt", Order: "DESCENDING"},
							{FieldPath: "__name__", Order: "ASCENDING"},
						},
						QueryScope: "COLLECTION",
						State:      "READY",
					},
				}, nil
			},
			FindTTLFieldFunc: func(ctx context.Context, collectionID string) (string, error) {
				gt.Equal(t, collectionID, "users")
				return "expireAt", nil
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				gt.Equal(t, fieldName, "expireAt")
				return &interfaces.FirestoreTTL{
					FieldPath: "expireAt",
					State:     "ACTIVE",
				}, nil
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		config, err := imp.Execute(ctx, []string{"users"})

		gt.NoError(t, err)
		gt.Equal(t, len(config.Collections), 1)
		gt.Equal(t, config.Collections[0].Name, "users")
		gt.Equal(t, len(config.Collections[0].Indexes), 1)
		gt.NotEqual(t, config.Collections[0].TTL, nil)
		gt.Equal(t, config.Collections[0].TTL.Field, "expireAt")
	})

	t.Run("Normal: import multiple collections", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				switch collectionID {
				case "users":
					return []interfaces.FirestoreIndex{
						{
							Fields: []interfaces.FirestoreIndexField{
								{FieldPath: "email", Order: "ASCENDING"},
								{FieldPath: "__name__", Order: "ASCENDING"},
							},
							QueryScope: "COLLECTION",
							State:      "READY",
						},
					}, nil
				case "posts":
					return []interfaces.FirestoreIndex{
						{
							Fields: []interfaces.FirestoreIndexField{
								{FieldPath: "authorId", Order: "ASCENDING"},
								{FieldPath: "publishedAt", Order: "DESCENDING"},
								{FieldPath: "__name__", Order: "ASCENDING"},
							},
							QueryScope: "COLLECTION",
							State:      "READY",
						},
					}, nil
				default:
					return nil, fmt.Errorf("unexpected collection: %s", collectionID)
				}
			},
			FindTTLFieldFunc: func(ctx context.Context, collectionID string) (string, error) {
				return "", nil // No TTL field
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil // No TTL policies
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		config, err := imp.Execute(ctx, []string{"users", "posts"})

		gt.NoError(t, err)
		gt.Equal(t, len(config.Collections), 2)
		// Order is not guaranteed, so check both collections exist
		var hasUsers, hasPosts bool
		for _, col := range config.Collections {
			if col.Name == "users" {
				hasUsers = true
			}
			if col.Name == "posts" {
				hasPosts = true
			}
		}
		gt.Equal(t, hasUsers, true)
		gt.Equal(t, hasPosts, true)
	})

	t.Run("Normal: skip creating indexes", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{
					{
						Name:       "projects/test/databases/default/collectionGroups/users/indexes/idx1",
						Fields:     []interfaces.FirestoreIndexField{},
						QueryScope: "COLLECTION",
						State:      "CREATING", // Should be skipped
					},
				}, nil
			},
			FindTTLFieldFunc: func(ctx context.Context, collectionID string) (string, error) {
				return "", nil // No TTL field
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		config, err := imp.Execute(ctx, []string{"users"})

		gt.NoError(t, err)
		gt.Equal(t, len(config.Collections[0].Indexes), 0) // Creating index should be skipped
	})

	t.Run("Normal: handle vector config", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{
					{
						Fields: []interfaces.FirestoreIndexField{
							{FieldPath: "title", Order: "ASCENDING"},
							{FieldPath: "__name__", Order: "ASCENDING"},
							{
								FieldPath: "embedding",
								VectorConfig: &interfaces.FirestoreVectorConfig{
									Dimension: 768,
								},
							},
						},
						QueryScope: "COLLECTION",
						State:      "READY",
					},
				}, nil
			},
			FindTTLFieldFunc: func(ctx context.Context, collectionID string) (string, error) {
				return "", nil // No TTL field
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		config, err := imp.Execute(ctx, []string{"documents"})

		gt.NoError(t, err)
		gt.Equal(t, len(config.Collections[0].Indexes), 1)

		// Check that vector field is properly positioned at the end (without __name__)
		fields := config.Collections[0].Indexes[0].Fields
		gt.Equal(t, len(fields), 2) // title and embedding (without __name__)
		gt.Equal(t, fields[0].Name, "title")
		gt.Equal(t, fields[1].Name, "embedding")
		gt.NotEqual(t, fields[1].VectorConfig, nil)
		gt.Equal(t, fields[1].VectorConfig.Dimension, 768)
	})

	t.Run("Normal: handle array config", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{
					{
						Fields: []interfaces.FirestoreIndexField{
							{FieldPath: "tags", ArrayConfig: "CONTAINS"},
							{FieldPath: "score", Order: "DESCENDING"},
							{FieldPath: "__name__", Order: "ASCENDING"},
						},
						QueryScope: "COLLECTION",
						State:      "READY",
					},
				}, nil
			},
			FindTTLFieldFunc: func(ctx context.Context, collectionID string) (string, error) {
				return "", nil // No TTL field
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		config, err := imp.Execute(ctx, []string{"posts"})

		gt.NoError(t, err)
		fields := config.Collections[0].Indexes[0].Fields
		gt.Equal(t, fields[0].ArrayConfig, "CONTAINS")
		gt.Equal(t, fields[1].Order, "DESCENDING")
	})

	t.Run("Error: list indexes fails", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return nil, fmt.Errorf("permission denied")
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		_, err := imp.Execute(ctx, []string{"users"})

		gt.Error(t, err).Contains("permission denied")
	})

	t.Run("Error: get TTL policy fails", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{}, nil
			},
			FindTTLFieldFunc: func(ctx context.Context, collectionID string) (string, error) {
				return "", fmt.Errorf("failed to find TTL field")
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, fmt.Errorf("failed to get TTL policy")
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		_, err := imp.Execute(ctx, []string{"users"})

		// Import should succeed even if TTL policy fails
		gt.NoError(t, err)
	})

	t.Run("Normal: empty collection list", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListCollectionsFunc: func(ctx context.Context) ([]string, error) {
				return []string{"users", "posts"}, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{}, nil
			},
			FindTTLFieldFunc: func(ctx context.Context, collectionID string) (string, error) {
				return "", nil
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		config, err := imp.Execute(ctx, []string{})

		gt.NoError(t, err)
		gt.Equal(t, len(config.Collections), 2) // Should discover users and posts
	})

	t.Run("Normal: deduplicate indexes with same key", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{
					{
						Name: "idx1",
						Fields: []interfaces.FirestoreIndexField{
							{FieldPath: "email", Order: "ASCENDING"},
							{FieldPath: "__name__", Order: "ASCENDING"},
						},
						QueryScope: "COLLECTION",
						State:      "READY",
					},
					{
						Name: "idx2", // Different name but same fields
						Fields: []interfaces.FirestoreIndexField{
							{FieldPath: "email", Order: "ASCENDING"},
							{FieldPath: "__name__", Order: "ASCENDING"},
						},
						QueryScope: "COLLECTION",
						State:      "READY",
					},
				}, nil
			},
			FindTTLFieldFunc: func(ctx context.Context, collectionID string) (string, error) {
				return "", nil // No TTL field
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
		}

		imp := usecase.NewImport(mockClient, logger)
		config, err := imp.Execute(ctx, []string{"users"})

		gt.NoError(t, err)
		gt.Equal(t, len(config.Collections), 1)
		gt.Equal(t, len(config.Collections[0].Indexes), 1) // Should be deduplicated
	})
}
