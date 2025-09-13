package main

import (
	"context"
	"fmt"
	"log"

	"github.com/m-mizutani/fireconf"
)

func main() {
	ctx := context.Background()

	// Example: Migrating from YAML-based configuration to programmatic configuration
	
	// Step 1: Load existing YAML configuration
	log.Println("Loading existing YAML configuration...")
	yamlConfig, err := fireconf.LoadConfigFromYAML("legacy-fireconf.yaml")
	if err != nil {
		log.Printf("Could not load YAML config (this is expected for this example): %v", err)
		// Create a sample configuration instead
		yamlConfig = createSampleConfig()
	}

	// Step 2: Create client
	client, err := fireconf.NewClient(ctx, "my-project")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Step 3: Import current Firestore configuration
	log.Println("Importing current Firestore configuration...")
	currentConfig, err := client.Import(ctx)
	if err != nil {
		log.Fatalf("Failed to import current configuration: %v", err)
	}

	// Step 4: Compare configurations
	log.Println("Comparing configurations...")
	diff := client.DiffConfigs(currentConfig, yamlConfig)

	// Display differences
	if len(diff.Collections) == 0 {
		log.Println("No differences found between current and desired configuration")
		return
	}

	log.Printf("Found %d collection differences:", len(diff.Collections))
	for _, colDiff := range diff.Collections {
		fmt.Printf("Collection: %s (Action: %s)\n", colDiff.Name, colDiff.Action)
		
		if len(colDiff.IndexesToAdd) > 0 {
			fmt.Printf("  Indexes to add: %d\n", len(colDiff.IndexesToAdd))
		}
		if len(colDiff.IndexesToDelete) > 0 {
			fmt.Printf("  Indexes to delete: %d\n", len(colDiff.IndexesToDelete))
		}
		if colDiff.TTLAction != "" {
			fmt.Printf("  TTL action: %s\n", colDiff.TTLAction)
		}
	}

	// Step 5: Apply migration
	log.Println("Applying migration...")
	if err := client.Migrate(ctx, yamlConfig); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Step 6: Verify migration
	log.Println("Verifying migration...")
	updatedConfig, err := client.Import(ctx)
	if err != nil {
		log.Fatalf("Failed to verify migration: %v", err)
	}

	// Check if configurations match
	finalDiff := client.DiffConfigs(updatedConfig, yamlConfig)
	if len(finalDiff.Collections) == 0 {
		log.Println("Migration completed successfully! Configurations match.")
	} else {
		log.Printf("Warning: %d differences still exist after migration", len(finalDiff.Collections))
	}

	// Step 7: Save the final configuration for future reference
	if err := updatedConfig.SaveToYAML("migrated-fireconf.yaml"); err != nil {
		log.Printf("Warning: Could not save migrated configuration: %v", err)
	} else {
		log.Println("Saved migrated configuration to migrated-fireconf.yaml")
	}
}

func createSampleConfig() *fireconf.Config {
	return &fireconf.Config{
		Collections: []fireconf.Collection{
			{
				Name: "users",
				Indexes: []fireconf.Index{
					{
						Fields: []fireconf.IndexField{
							{Path: "email", Order: fireconf.OrderAscending},
							{Path: "status", Order: fireconf.OrderAscending},
						},
					},
				},
				TTL: &fireconf.TTL{
					Field: "expireAt",
				},
			},
			{
				Name: "posts",
				Indexes: []fireconf.Index{
					{
						Fields: []fireconf.IndexField{
							{Path: "authorId", Order: fireconf.OrderAscending},
							{Path: "createdAt", Order: fireconf.OrderDescending},
						},
					},
				},
			},
		},
	}
}