package usecase_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/m-mizutani/fireconf/internal/adapter/firestore"
	"github.com/m-mizutani/fireconf/internal/interfaces"
	"github.com/m-mizutani/fireconf/internal/model"
	"github.com/m-mizutani/fireconf/internal/usecase"
	"github.com/m-mizutani/gt"
)

// cleanupCollection deletes all indexes and disables TTL for a collection.
// Intended for use in t.Cleanup() to guarantee cleanup even on test failure.
func cleanupCollection(ctx context.Context, t *testing.T, client *firestore.Client, collectionName string) {
	t.Helper()
	t.Logf("Cleanup: removing indexes for collection %s", collectionName)

	indexes, err := client.ListIndexes(ctx, collectionName)
	if err != nil {
		t.Logf("Cleanup: failed to list indexes for %s: %v", collectionName, err)
	} else {
		for _, idx := range indexes {
			if idx.Name != "" {
				if _, err := client.DeleteIndex(ctx, idx.Name); err != nil {
					t.Logf("Cleanup: failed to delete index %s: %v", idx.Name, err)
				}
			}
		}
	}

	if _, err := client.DisableTTLPolicy(ctx, collectionName); err != nil {
		t.Logf("Cleanup: failed to disable TTL for %s: %v", collectionName, err)
	}
}

func TestE2E_FullCycle(t *testing.T) {
	// Skip if environment variables are not set
	projectID := os.Getenv("TEST_FIRECONF_PROJECT")
	databaseID := os.Getenv("TEST_FIRECONF_DATABASE")
	if projectID == "" || databaseID == "" {
		t.Skip("TEST_FIRECONF_PROJECT and TEST_FIRECONF_DATABASE must be set for E2E tests")
	}

	t.Parallel()

	t.Run("Full E2E cycle", func(t *testing.T) {
		ctx := context.Background()
		logger := slog.Default()

		// Create Firestore client
		authConfig := firestore.AuthConfig{
			ProjectID:  projectID,
			DatabaseID: databaseID,
		}

		client, err := firestore.NewClient(ctx, authConfig)
		gt.NoError(t, err)
		defer func() { _ = client.Close() }()

		// Test collection name with timestamp to avoid conflicts
		testCollectionName := fmt.Sprintf("fireconf_e2e_test_%d", time.Now().UnixNano())

		// Register cleanup early so it runs even if the test fails or panics
		t.Cleanup(func() {
			cleanupCollection(context.Background(), t, client, testCollectionName)
		})

		// Step 1: Load test configuration from embedded file
		// Replace placeholder with actual test collection name
		testYAML := strings.ReplaceAll(usecase.TestDataE2ESimple, "__TEST_COLLECTION__", testCollectionName)

		var originalConfig model.Config
		err = yaml.Unmarshal([]byte(testYAML), &originalConfig)
		gt.NoError(t, err)

		// Step 2: Sync configuration to Firestore
		t.Log("Syncing configuration to Firestore...")
		syncUseCase := usecase.NewSync(client, logger)
		err = syncUseCase.Execute(ctx, &originalConfig)
		gt.NoError(t, err)

		// Step 3: Import configuration back from Firestore
		t.Log("Importing configuration from Firestore...")
		importUseCase := usecase.NewImport(client, logger)
		importedConfig, err := importUseCase.Execute(ctx, []string{testCollectionName})
		gt.NoError(t, err)

		// Step 4: Validate imported configuration matches original
		t.Log("Validating imported configuration...")
		gt.Equal(t, len(importedConfig.Collections), 1)
		gt.Equal(t, importedConfig.Collections[0].Name, testCollectionName)

		// Check indexes count (may have additional system indexes)
		if len(importedConfig.Collections[0].Indexes) < len(originalConfig.Collections[0].Indexes) {
			t.Errorf("Expected at least %d indexes, got %d",
				len(originalConfig.Collections[0].Indexes),
				len(importedConfig.Collections[0].Indexes))
		}

		// Verify each original index exists in imported config
		for _, origIndex := range originalConfig.Collections[0].Indexes {
			found := false
			for _, impIndex := range importedConfig.Collections[0].Indexes {
				if indexesMatch(origIndex, impIndex) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Index not found in imported config: %+v", origIndex)
			}
		}

		// Check TTL
		if originalConfig.Collections[0].TTL != nil {
			gt.NotEqual(t, importedConfig.Collections[0].TTL, nil)
			gt.Equal(t, importedConfig.Collections[0].TTL.Field, originalConfig.Collections[0].TTL.Field)
		}

		// Step 5: Re-sync to verify idempotency
		t.Log("Re-syncing to verify idempotency...")
		syncUseCase2 := usecase.NewSync(client, logger)
		err = syncUseCase2.Execute(ctx, importedConfig)
		gt.NoError(t, err)
	})
}

func TestE2E_WithTestData(t *testing.T) {
	// Skip if environment variables are not set
	projectID := os.Getenv("TEST_FIRECONF_PROJECT")
	databaseID := os.Getenv("TEST_FIRECONF_DATABASE")
	if projectID == "" || databaseID == "" {
		t.Skip("TEST_FIRECONF_PROJECT and TEST_FIRECONF_DATABASE must be set for E2E tests")
	}

	t.Parallel()

	testCases := []struct {
		name     string
		testData string
	}{
		{
			name:     "Basic configuration",
			testData: usecase.TestDataBasic,
		},
		{
			name:     "Vector configuration",
			testData: usecase.TestDataVector,
		},
		{
			name:     "Array configuration",
			testData: usecase.TestDataArray,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			logger := slog.Default()

			// Create Firestore client for this test
			authConfig := firestore.AuthConfig{
				ProjectID:  projectID,
				DatabaseID: databaseID,
			}

			client, err := firestore.NewClient(ctx, authConfig)
			gt.NoError(t, err)
			defer func() { _ = client.Close() }()

			// Parse test data
			var config model.Config
			err = yaml.Unmarshal([]byte(tc.testData), &config)
			gt.NoError(t, err)

			// Add timestamp to collection names to avoid conflicts
			timestamp := time.Now().UnixNano()
			for i := range config.Collections {
				config.Collections[i].Name = fmt.Sprintf("%s_e2e_%d", config.Collections[i].Name, timestamp)
			}

			// Register cleanup early so it runs even if the test fails
			for _, collection := range config.Collections {
				collectionName := collection.Name
				t.Cleanup(func() {
					cleanupCollection(context.Background(), t, client, collectionName)
				})
			}

			// Sync configuration
			syncUseCase := usecase.NewSync(client, logger)
			err = syncUseCase.Execute(ctx, &config)
			gt.NoError(t, err)

			// Import back and verify
			for _, collection := range config.Collections {
				importUseCase := usecase.NewImport(client, logger)
				importedConfig, err := importUseCase.Execute(ctx, []string{collection.Name})
				gt.NoError(t, err)
				gt.Equal(t, len(importedConfig.Collections), 1)
				gt.Equal(t, importedConfig.Collections[0].Name, collection.Name)
			}
		})
	}
}

// TestE2E_VectorIndexNameFieldBehavior verifies the actual Firestore API behavior
// regarding __name__ in vector indexes. This confirms the assumption that our
// validation logic is based on.
func TestE2E_VectorIndexNameFieldBehavior(t *testing.T) {
	projectID := os.Getenv("TEST_FIRECONF_PROJECT")
	databaseID := os.Getenv("TEST_FIRECONF_DATABASE")
	if projectID == "" || databaseID == "" {
		t.Skip("TEST_FIRECONF_PROJECT and TEST_FIRECONF_DATABASE must be set for E2E tests")
	}

	t.Parallel()

	ctx := context.Background()
	authConfig := firestore.AuthConfig{
		ProjectID:  projectID,
		DatabaseID: databaseID,
	}

	client, err := firestore.NewClient(ctx, authConfig)
	gt.NoError(t, err)
	defer func() { _ = client.Close() }()

	timestamp := time.Now().UnixNano()
	collectionName := fmt.Sprintf("fireconf_vector_name_test_%d", timestamp)

	// Register cleanup for the entire collection used by both sub-tests
	t.Cleanup(func() {
		cleanupCollection(context.Background(), t, client, collectionName)
	})

	t.Run("Firestore API rejects vector index with __name__ field", func(t *testing.T) {
		// Attempt to create a vector index that includes __name__.
		// The Firestore Admin API should reject this with an error indicating
		// that __name__ has no valid order or array config in vector context.
		indexWithName := interfaces.FirestoreIndex{
			QueryScope: "COLLECTION",
			Fields: []interfaces.FirestoreIndexField{
				{FieldPath: "title", Order: "ASCENDING"},
				{FieldPath: "__name__", Order: "ASCENDING"},
				{FieldPath: "embedding", VectorConfig: &interfaces.FirestoreVectorConfig{Dimension: 768}},
			},
		}

		idxName, err := client.CreateIndex(ctx, collectionName, indexWithName)
		if err != nil {
			// Expected: Firestore API rejects __name__ in vector indexes synchronously
			t.Logf("Firestore API rejected __name__ in vector index (expected): %v", err)
		} else if idxName != "" {
			// Creation submitted; poll until it fails or succeeds
			rejected := false
			for i := 0; i < 30; i++ {
				idx, getErr := client.GetIndex(ctx, idxName)
				if getErr != nil {
					t.Logf("Firestore operation failed as expected: %v", getErr)
					rejected = true
					break
				}
				if idx.State == "ERROR" || idx.State == "NEEDS_REPAIR" {
					t.Logf("Firestore index entered %s state as expected", idx.State)
					rejected = true
					break
				}
				if idx.State == "READY" {
					break
				}
				time.Sleep(5 * time.Second)
			}
			if !rejected {
				t.Error("Expected Firestore API to reject vector index with __name__ field, but it succeeded")
			}
		} else {
			// AlreadyExists treated as success with empty name - unexpected for a new collection
			t.Error("Expected Firestore API to reject vector index with __name__ field, but it was accepted (AlreadyExists)")
		}
	})

	t.Run("Vector index without __name__ is accepted", func(t *testing.T) {
		logger := slog.Default()

		testYAML := `collections:
- name: ` + collectionName + `
  indexes:
  - fields:
    - name: title
      order: ASCENDING
    - name: embedding
      order: ASCENDING
      vector_config:
        dimension: 768
    query_scope: COLLECTION
`
		var config model.Config
		err := yaml.Unmarshal([]byte(testYAML), &config)
		gt.NoError(t, err)

		syncUseCase := usecase.NewSync(client, logger)
		err = syncUseCase.Execute(ctx, &config)
		gt.NoError(t, err)

		// Verify the index was created
		importUseCase := usecase.NewImport(client, logger)
		importedConfig, err := importUseCase.Execute(ctx, []string{collectionName})
		gt.NoError(t, err)
		gt.Equal(t, len(importedConfig.Collections), 1)

		expectedIndex := config.Collections[0].Indexes[0]
		found := false
		for _, importedIndex := range importedConfig.Collections[0].Indexes {
			if indexesMatch(expectedIndex, importedIndex) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find index %+v, but it was not present in imported config", expectedIndex)
		}
	})
}

// indexesMatch compares two indexes for equality
func indexesMatch(a, b model.Index) bool {
	if a.QueryScope != b.QueryScope {
		return false
	}

	if len(a.Fields) != len(b.Fields) {
		return false
	}

	for i, fieldA := range a.Fields {
		fieldB := b.Fields[i]
		if fieldA.Name != fieldB.Name ||
			fieldA.Order != fieldB.Order ||
			fieldA.ArrayConfig != fieldB.ArrayConfig {
			return false
		}

		// Compare vector config
		if (fieldA.VectorConfig == nil) != (fieldB.VectorConfig == nil) {
			return false
		}
		if fieldA.VectorConfig != nil && fieldA.VectorConfig.Dimension != fieldB.VectorConfig.Dimension {
			return false
		}
	}

	return true
}
