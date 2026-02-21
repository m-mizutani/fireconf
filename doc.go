// Package fireconf provides programmatic Firestore index and TTL management for Go applications.
//
// The fireconf package allows you to define and manage Firestore composite indexes and TTL policies
// as code, enabling version control, automated deployments, and consistent configuration across environments.
//
// # Basic Usage
//
// Create a client with configuration and migrate:
//
//	ctx := context.Background()
//
//	// Define configuration
//	config := &fireconf.Config{
//	    Collections: []fireconf.Collection{
//	        {
//	            Name: "users",
//	            Indexes: []fireconf.Index{
//	                {
//	                    Fields: []fireconf.IndexField{
//	                        {Path: "email", Order: fireconf.OrderAscending},
//	                        {Path: "createdAt", Order: fireconf.OrderDescending},
//	                    },
//	                },
//	            },
//	            TTL: &fireconf.TTL{
//	                Field: "expireAt",
//	            },
//	        },
//	    },
//	}
//
//	// Create a new fireconf client with configuration
//	client, err := fireconf.New(ctx, "my-project", "(default)", config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Apply configuration to Firestore (waits for indexes to be READY)
//	if err := client.Migrate(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// # Loading from YAML
//
// Load configuration from a YAML file:
//
//	config, err := fireconf.LoadConfigFromYAML("fireconf.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	client, err := fireconf.New(ctx, "my-project", "(default)", config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if err := client.Migrate(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// # Importing Existing Configuration
//
// Import configuration from an existing Firestore database:
//
//	// config can be nil for import-only use
//	client, err := fireconf.New(ctx, "my-project", "(default)", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	config, err := client.Import(ctx, "users", "posts")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Save to YAML
//	if err := config.SaveToYAML("fireconf.yaml"); err != nil {
//	    log.Fatal(err)
//	}
//
// # Comparing Configurations
//
// Compare the current Firestore state against the desired configuration:
//
//	current, err := client.Import(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	diff, err := client.DiffConfigs(current)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, colDiff := range diff.Collections {
//	    fmt.Printf("Collection: %s (Action: %s)\n", colDiff.Name, colDiff.Action)
//	}
//
// # Error Handling
//
// The package defines structured error types that can be inspected using errors.As:
//
//   - [MigrationError]: returned by Migrate on sync failure
//
//   - [DiffError]: returned by DiffConfigs on invalid input
//
//   - [ValidationError]: returned by Config.Validate on configuration errors
//
//     var migErr *fireconf.MigrationError
//     if errors.As(err, &migErr) {
//     fmt.Printf("Migration operation %q failed: %v\n", migErr.Operation, migErr.Cause)
//     }
package fireconf
