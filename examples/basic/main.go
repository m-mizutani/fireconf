package main

import (
	"context"
	"log"

	"github.com/m-mizutani/fireconf"
)

func main() {
	ctx := context.Background()

	// Create a new fireconf client
	client, err := fireconf.NewClient(ctx, "my-project")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Define configuration programmatically
	config := &fireconf.Config{
		Collections: []fireconf.Collection{
			{
				Name: "users",
				Indexes: []fireconf.Index{
					{
						Fields: []fireconf.IndexField{
							{Path: "email", Order: fireconf.OrderAscending},
							{Path: "createdAt", Order: fireconf.OrderDescending},
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
							{Path: "status", Order: fireconf.OrderAscending},
							{Path: "publishedAt", Order: fireconf.OrderDescending},
						},
						QueryScope: fireconf.QueryScopeCollection,
					},
				},
			},
		},
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Apply configuration to Firestore
	log.Println("Applying configuration to Firestore...")
	if err := client.Migrate(ctx, config); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Configuration applied successfully!")
}