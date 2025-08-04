package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/m-mizutani/fireconf/pkg/adapter/firestore"
	"github.com/m-mizutani/fireconf/pkg/domain/model"
	"github.com/m-mizutani/fireconf/pkg/usecase"
	"github.com/m-mizutani/goerr/v2"
	"github.com/urfave/cli/v3"
)

func newSyncCommand() *cli.Command {
	return &cli.Command{
		Name:  "sync",
		Usage: "Sync Firestore configuration from YAML file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Configuration file path",
				Value:   "fireconf.yaml",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Show what would be changed without making actual changes",
			},
			&cli.BoolFlag{
				Name:  "skip-wait",
				Usage: "Skip waiting for operations to complete",
			},
		},
		Action: runSync,
	}
}

func runSync(ctx context.Context, c *cli.Command) error {
	logger := getLogger(ctx)

	// Check required project flag
	if c.String("project") == "" {
		return goerr.New("project flag is required for sync command")
	}

	// Read configuration file
	configPath := c.String("config")
	logger.Info("Reading configuration file", "path", configPath)

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return goerr.Wrap(err, "failed to read configuration file", goerr.V("path", configPath))
	}

	// Parse YAML
	var config model.Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return goerr.Wrap(err, "failed to parse YAML", goerr.V("path", configPath))
	}

	// Validate configuration
	for _, collection := range config.Collections {
		if err := collection.Validate(); err != nil {
			return goerr.Wrap(err, "invalid configuration", goerr.V("collection", collection.Name))
		}
	}

	// Create Firestore client
	authConfig := firestore.AuthConfig{
		ProjectID:   c.String("project"),
		DatabaseID:  c.String("database"),
		Credentials: c.String("credentials"),
	}

	client, err := firestore.NewClient(ctx, authConfig)
	if err != nil {
		return goerr.Wrap(err, "failed to create Firestore client")
	}

	defer client.Close()

	// Create sync use case
	sync := usecase.NewSyncWithOptions(client, logger, c.Bool("dry-run"), c.Bool("skip-wait"))

	// Execute sync
	if err := sync.Execute(ctx, &config); err != nil {
		return goerr.Wrap(err, "sync failed")
	}

	if c.Bool("dry-run") {
		fmt.Println("\n✓ Dry run completed. No changes were made.")
	} else {
		fmt.Println("\n✓ Sync completed successfully.")
	}

	return nil
}
