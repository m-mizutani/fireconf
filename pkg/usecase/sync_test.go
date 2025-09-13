package usecase_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/m-mizutani/fireconf/pkg/domain/interfaces"
	"github.com/m-mizutani/fireconf/pkg/domain/interfaces/mock"
	"github.com/m-mizutani/fireconf/pkg/domain/model"
	"github.com/m-mizutani/fireconf/pkg/usecase"
	"github.com/m-mizutani/gt"
)

func TestSync_Execute(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	t.Run("Normal: sync indexes successfully", func(t *testing.T) {
		// Setup mock
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return true, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				// Return empty list - no existing indexes
				return []interfaces.FirestoreIndex{}, nil
			},
			CreateIndexFunc: func(ctx context.Context, collectionID string, index interfaces.FirestoreIndex) (interface{}, error) {
				return nil, nil // No operation object in dry run
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil // No existing TTL
			},
			DisableTTLPolicyFunc: func(ctx context.Context, collectionID string) (interface{}, error) {
				return nil, nil
			},
		}

		// Create sync use case
		sync := usecase.NewSync(mockClient, logger, false)

		// Create test config
		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "email", Order: "ASCENDING"},
								{Name: "createdAt", Order: "DESCENDING"},
							},
							QueryScope: "COLLECTION",
						},
					},
				},
			},
		}

		// Execute
		err := sync.Execute(ctx, config)
		gt.NoError(t, err)

		// Verify calls
		gt.Equal(t, len(mockClient.CollectionExistsCalls()), 1)
		gt.Equal(t, len(mockClient.ListIndexesCalls()), 1)
		gt.Equal(t, len(mockClient.CreateIndexCalls()), 1)
		gt.Equal(t, mockClient.CreateIndexCalls()[0].CollectionID, "users")
	})

	t.Run("Normal: create collection if not exists", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return false, nil // Collection does not exist
			},
			CreateCollectionFunc: func(ctx context.Context, collectionID string) error {
				return nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{}, nil
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
			DisableTTLPolicyFunc: func(ctx context.Context, collectionID string) (interface{}, error) {
				return nil, nil
			},
		}

		sync := usecase.NewSync(mockClient, logger, false)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name:    "newcollection",
					Indexes: []model.Index{},
				},
			},
		}

		err := sync.Execute(ctx, config)
		gt.NoError(t, err)

		gt.Equal(t, len(mockClient.CreateCollectionCalls()), 1)
		gt.Equal(t, mockClient.CreateCollectionCalls()[0].CollectionID, "newcollection")
	})

	t.Run("Normal: delete obsolete indexes", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return true, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				// Return existing index that is not in desired state
				return []interfaces.FirestoreIndex{
					{
						Name: "projects/test/databases/default/collectionGroups/users/indexes/idx1",
						Fields: []interfaces.FirestoreIndexField{
							{FieldPath: "oldField", Order: "ASCENDING"},
						},
						QueryScope: "COLLECTION",
					},
				}, nil
			},
			DeleteIndexFunc: func(ctx context.Context, indexName string) (interface{}, error) {
				return nil, nil
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
			DisableTTLPolicyFunc: func(ctx context.Context, collectionID string) (interface{}, error) {
				return nil, nil
			},
		}

		sync := usecase.NewSync(mockClient, logger, false)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name:    "users",
					Indexes: []model.Index{}, // No indexes desired
				},
			},
		}

		err := sync.Execute(ctx, config)
		gt.NoError(t, err)

		gt.Equal(t, len(mockClient.DeleteIndexCalls()), 1)
		gt.Equal(t, mockClient.DeleteIndexCalls()[0].IndexName, "projects/test/databases/default/collectionGroups/users/indexes/idx1")
	})

	t.Run("Normal: enable TTL policy", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return true, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{}, nil
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil // No existing TTL
			},
			EnableTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (interface{}, error) {
				return nil, nil
			},
		}

		sync := usecase.NewSync(mockClient, logger, false)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name:    "users",
					Indexes: []model.Index{},
					TTL: &model.TTL{
						Field: "expireAt",
					},
				},
			},
		}

		err := sync.Execute(ctx, config)
		gt.NoError(t, err)

		gt.Equal(t, len(mockClient.EnableTTLPolicyCalls()), 1)
		gt.Equal(t, mockClient.EnableTTLPolicyCalls()[0].FieldName, "expireAt")
	})

	t.Run("Normal: dry run mode", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return false, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{}, nil
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
		}

		sync := usecase.NewSync(mockClient, logger, true) // dry run = true

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "email", Order: "ASCENDING"},
							},
						},
					},
				},
			},
		}

		err := sync.Execute(ctx, config)
		gt.NoError(t, err)

		// In dry run mode, no actual operations should be performed
		gt.Equal(t, len(mockClient.CreateCollectionCalls()), 0)
		gt.Equal(t, len(mockClient.CreateIndexCalls()), 0)
	})

	t.Run("Error: collection validation fails", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{}

		sync := usecase.NewSync(mockClient, logger, false)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "", // Invalid: empty name
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "email", Order: "ASCENDING"},
							},
						},
					},
				},
			},
		}

		err := sync.Execute(ctx, config)
		gt.Error(t, err).Contains("collection name is required")
	})

	t.Run("Error: list indexes fails", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return true, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return nil, fmt.Errorf("firestore error: permission denied")
			},
		}

		sync := usecase.NewSync(mockClient, logger, false)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name:    "users",
					Indexes: []model.Index{},
				},
			},
		}

		err := sync.Execute(ctx, config)
		gt.Error(t, err).Contains("permission denied")
	})

	t.Run("Error: create index fails", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return true, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{}, nil
			},
			CreateIndexFunc: func(ctx context.Context, collectionID string, index interfaces.FirestoreIndex) (interface{}, error) {
				return nil, fmt.Errorf("invalid index configuration")
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
			DisableTTLPolicyFunc: func(ctx context.Context, collectionID string) (interface{}, error) {
				return nil, nil
			},
		}

		sync := usecase.NewSync(mockClient, logger, false)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "email", Order: "ASCENDING"},
							},
						},
					},
				},
			},
		}

		err := sync.Execute(ctx, config)
		gt.Error(t, err).Contains("invalid index configuration")
	})

	t.Run("Error: TTL enable fails", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return true, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{}, nil
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
			EnableTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (interface{}, error) {
				return nil, fmt.Errorf("TTL field must be a timestamp")
			},
		}

		sync := usecase.NewSync(mockClient, logger, false)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name:    "users",
					Indexes: []model.Index{},
					TTL: &model.TTL{
						Field: "nonTimestampField",
					},
				},
			},
		}

		err := sync.Execute(ctx, config)
		gt.Error(t, err).Contains("TTL field must be a timestamp")
	})

	t.Run("Normal: sync with test data", func(t *testing.T) {
		mockClient := &mock.FirestoreClientMock{
			CollectionExistsFunc: func(ctx context.Context, collectionID string) (bool, error) {
				return true, nil
			},
			ListIndexesFunc: func(ctx context.Context, collectionID string) ([]interfaces.FirestoreIndex, error) {
				return []interfaces.FirestoreIndex{}, nil
			},
			CreateIndexFunc: func(ctx context.Context, collectionID string, index interfaces.FirestoreIndex) (interface{}, error) {
				return nil, nil
			},
			GetTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (*interfaces.FirestoreTTL, error) {
				return nil, nil
			},
			EnableTTLPolicyFunc: func(ctx context.Context, collectionID string, fieldName string) (interface{}, error) {
				return nil, nil
			},
			DisableTTLPolicyFunc: func(ctx context.Context, collectionID string) (interface{}, error) {
				return nil, nil
			},
		}

		sync := usecase.NewSync(mockClient, logger, false)
		config := LoadBasicTestConfig(t)

		err := sync.Execute(ctx, config)
		gt.NoError(t, err)

		// Verify the correct number of operations
		gt.Equal(t, len(mockClient.ListIndexesCalls()), 2)     // 2 collections
		gt.Equal(t, len(mockClient.CreateIndexCalls()), 3)     // 2 indexes for users + 1 for posts
		gt.Equal(t, len(mockClient.EnableTTLPolicyCalls()), 1) // TTL for users only
	})
}
