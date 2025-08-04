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
	"github.com/m-mizutani/fireconf/pkg/adapter/firestore"
	"github.com/m-mizutani/fireconf/pkg/domain/model"
	"github.com/m-mizutani/fireconf/pkg/usecase"
	"github.com/m-mizutani/gt"
)

func TestE2E_FullCycle(t *testing.T) {
	// Skip if environment variables are not set
	projectID := os.Getenv("TEST_FIRECONF_PROJECT")
	databaseID := os.Getenv("TEST_FIRECONF_DATABASE")
	if projectID == "" || databaseID == "" {
		t.Skip("TEST_FIRECONF_PROJECT and TEST_FIRECONF_DATABASE must be set for E2E tests")
	}

	ctx := context.Background()
	logger := slog.Default()

	// Create Firestore client
	authConfig := firestore.AuthConfig{
		ProjectID:  projectID,
		DatabaseID: databaseID,
	}

	client, err := firestore.NewClient(ctx, authConfig)
	gt.NoError(t, err)

	// Type assert to get Close method
	firestoreClient, ok := client.(*firestore.Client)
	gt.Equal(t, ok, true)
	defer firestoreClient.Close()

	// Test collection name with timestamp to avoid conflicts
	testCollectionName := fmt.Sprintf("fireconf_e2e_test_%d", time.Now().Unix())

	t.Run("Full E2E cycle", func(t *testing.T) {
		// Step 1: Load test configuration from embedded file
		// Replace placeholder with actual test collection name
		testYAML := strings.ReplaceAll(usecase.TestDataE2ESimple, "__TEST_COLLECTION__", testCollectionName)

		var originalConfig model.Config
		err := yaml.Unmarshal([]byte(testYAML), &originalConfig)
		gt.NoError(t, err)

		// Step 2: Delete all existing indexes for the test collection
		t.Log("Deleting existing indexes...")
		existingIndexes, err := client.ListIndexes(ctx, testCollectionName)
		gt.NoError(t, err)

		for _, idx := range existingIndexes {
			if idx.Name != "" { // Skip default indexes
				t.Logf("Deleting index: %s", idx.Name)
				op, err := client.DeleteIndex(ctx, idx.Name)
				gt.NoError(t, err)
				if op != nil {
					err = client.WaitForOperation(ctx, op)
					gt.NoError(t, err)
				}
			}
		}

		// Disable any existing TTL
		op, err := client.DisableTTLPolicy(ctx, testCollectionName)
		if err == nil && op != nil {
			_ = client.WaitForOperation(ctx, op)
		}

		// Step 3: Sync configuration to Firestore
		t.Log("Syncing configuration to Firestore...")
		syncUseCase := usecase.NewSyncWithOptions(client, logger, false, true) // skipWait=true for faster tests
		err = syncUseCase.Execute(ctx, &originalConfig)
		gt.NoError(t, err)

		// Wait a bit for Firestore to process
		time.Sleep(2 * time.Second)

		// Step 4: Import configuration back from Firestore
		t.Log("Importing configuration from Firestore...")
		importUseCase := usecase.NewImport(client, logger)
		importedConfig, err := importUseCase.Execute(ctx, []string{testCollectionName})
		gt.NoError(t, err)

		// Step 5: Validate imported configuration matches original
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

		// Step 6: Re-sync to verify idempotency
		t.Log("Re-syncing to verify idempotency...")
		syncUseCase2 := usecase.NewSyncWithOptions(client, logger, false, true) // skipWait=true
		err = syncUseCase2.Execute(ctx, importedConfig)
		gt.NoError(t, err)

		// Step 7: Clean up - delete test indexes
		t.Log("Cleaning up test indexes...")
		finalIndexes, err := client.ListIndexes(ctx, testCollectionName)
		gt.NoError(t, err)

		for _, idx := range finalIndexes {
			if idx.Name != "" {
				op, err := client.DeleteIndex(ctx, idx.Name)
				if err == nil && op != nil {
					_ = client.WaitForOperation(ctx, op)
				}
			}
		}

		// Disable TTL
		op, err = client.DisableTTLPolicy(ctx, testCollectionName)
		if err == nil && op != nil {
			_ = client.WaitForOperation(ctx, op)
		}
	})
}

func TestE2E_WithTestData(t *testing.T) {
	// Skip if environment variables are not set
	projectID := os.Getenv("TEST_FIRECONF_PROJECT")
	databaseID := os.Getenv("TEST_FIRECONF_DATABASE")
	if projectID == "" || databaseID == "" {
		t.Skip("TEST_FIRECONF_PROJECT and TEST_FIRECONF_DATABASE must be set for E2E tests")
	}

	ctx := context.Background()
	logger := slog.Default()

	// Create Firestore client
	authConfig := firestore.AuthConfig{
		ProjectID:  projectID,
		DatabaseID: databaseID,
	}

	client, err := firestore.NewClient(ctx, authConfig)
	gt.NoError(t, err)

	// Type assert to get Close method
	firestoreClient, ok := client.(*firestore.Client)
	gt.Equal(t, ok, true)
	defer firestoreClient.Close()

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
		t.Run(tc.name, func(t *testing.T) {
			// Parse test data
			var config model.Config
			err := yaml.Unmarshal([]byte(tc.testData), &config)
			gt.NoError(t, err)

			// Add timestamp to collection names to avoid conflicts
			timestamp := time.Now().Unix()
			for i := range config.Collections {
				config.Collections[i].Name = fmt.Sprintf("%s_e2e_%d", config.Collections[i].Name, timestamp)
			}

			// Sync configuration
			syncUseCase := usecase.NewSyncWithOptions(client, logger, false, true) // skipWait=true
			err = syncUseCase.Execute(ctx, &config)
			gt.NoError(t, err)

			// Wait for Firestore to process
			time.Sleep(2 * time.Second)

			// Import back and verify
			for _, collection := range config.Collections {
				importUseCase := usecase.NewImport(client, logger)
				importedConfig, err := importUseCase.Execute(ctx, []string{collection.Name})
				gt.NoError(t, err)
				gt.Equal(t, len(importedConfig.Collections), 1)
				gt.Equal(t, importedConfig.Collections[0].Name, collection.Name)
			}

			// Clean up
			for _, collection := range config.Collections {
				indexes, err := client.ListIndexes(ctx, collection.Name)
				gt.NoError(t, err)
				for _, idx := range indexes {
					if idx.Name != "" {
						op, err := client.DeleteIndex(ctx, idx.Name)
						if err == nil && op != nil {
							_ = client.WaitForOperation(ctx, op)
						}
					}
				}
			}
		})
	}
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
