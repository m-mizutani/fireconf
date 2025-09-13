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

	// Create client with options
	client, err := fireconf.NewClient(ctx, "my-project", "custom-db",
		fireconf.WithLogger(logger),
		fireconf.WithCredentialsFile("service-account.json"),
		fireconf.WithDryRun(false),
		fireconf.WithVerbose(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

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

	// Get migration plan first
	logger.Info("Getting migration plan...")
	plan, err := client.GetMigrationPlan(ctx, config)
	if err != nil {
		log.Fatalf("Failed to get migration plan: %v", err)
	}

	// Display plan
	logger.Info("Migration plan", "steps", len(plan.Steps))
	for i, step := range plan.Steps {
		logger.Info("Step",
			"index", i+1,
			"collection", step.Collection,
			"operation", step.Operation,
			"description", step.Description,
			"destructive", step.Destructive,
		)
	}

	// Apply migration
	logger.Info("Applying migration...")
	if err := client.Migrate(ctx, config); err != nil {
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
