// Package fireconf provides programmatic Firestore index and TTL management for Go applications.
//
// The fireconf package allows you to define and manage Firestore composite indexes and TTL policies
// as code, enabling version control, automated deployments, and consistent configuration across environments.
//
// # Basic Usage
//
// Create a client and migrate configuration:
//
//	ctx := context.Background()
//
//	// Create a new fireconf client
//	client, err := fireconf.NewClient(ctx, "my-project")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
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
//	// Apply configuration to Firestore
//	if err := client.Migrate(ctx, config); err != nil {
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
//	if err := client.Migrate(ctx, config); err != nil {
//	    log.Fatal(err)
//	}
//
// # Importing Existing Configuration
//
// Import configuration from an existing Firestore database:
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
package fireconf
