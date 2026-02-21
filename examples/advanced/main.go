package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/m-mizutani/fireconf"
)

func main() {
	ctx := context.Background()

	// Create a custom logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Define advanced configuration with vector search
	config := &fireconf.Config{
		Collections: []fireconf.Collection{
			{
				Name: "documents",
				Indexes: []fireconf.Index{
					// Text search index
					{
						Fields: []fireconf.IndexField{
							{Path: "category", Order: fireconf.OrderAscending},
							{Path: "tags", Array: fireconf.ArrayConfigContains},
							{Path: "createdAt", Order: fireconf.OrderDescending},
						},
						QueryScope: fireconf.QueryScopeCollectionGroup,
					},
					// Vector search index
					{
						Fields: []fireconf.IndexField{
							{Path: "embedding", Vector: &fireconf.VectorConfig{
								Dimension: 768,
							}},
						},
					},
				},
			},
			{
				Name: "sessions",
				TTL: &fireconf.TTL{
					Field: "expiresAt",
				},
			},
		},
	}

	// Create client with options and configuration
	client, err := fireconf.New(ctx, "my-project", "custom-db", config,
		fireconf.WithLogger(logger),
		fireconf.WithCredentialsFile("service-account.json"),
		fireconf.WithDryRun(false),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Apply migration
	logger.Info("Applying migration...")
	if err := client.Migrate(ctx); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Import current configuration for comparison
	logger.Info("Importing current configuration...")
	current, err := client.Import(ctx, "documents", "sessions")
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	// Save current configuration to file
	if err := current.SaveToYAML("current.yaml"); err != nil {
		log.Fatalf("Failed to save configuration: %v", err)
	}

	logger.Info("Advanced migration completed successfully!")
}
