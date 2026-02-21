package commands

import (
	"context"

	"github.com/m-mizutani/fireconf"
	"github.com/m-mizutani/goerr/v2"
	"github.com/urfave/cli/v3"
)

// NewSyncCommand creates the sync command
func NewSyncCommand() *cli.Command {
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
		},
		Action: runSync,
	}
}

func runSync(ctx context.Context, c *cli.Command) error {
	logger := getLogger(ctx)

	// Check required project flag
	projectID := c.String("project")
	if projectID == "" {
		return goerr.New("project flag is required for sync command")
	}

	// Read configuration file
	configPath := c.String("config")
	logger.Info("Reading configuration file", "path", configPath)

	// Load configuration from YAML
	config, err := fireconf.LoadConfigFromYAML(configPath)
	if err != nil {
		return goerr.Wrap(err, "failed to load configuration")
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return goerr.Wrap(err, "invalid configuration")
	}

	// Get database ID
	databaseID := c.String("database")
	if databaseID == "" {
		return goerr.New("database flag is required for sync command")
	}

	// Create fireconf client
	opts := []fireconf.Option{
		fireconf.WithLogger(logger),
		fireconf.WithDryRun(c.Bool("dry-run")),
	}

	if credentials := c.String("credentials"); credentials != "" {
		opts = append(opts, fireconf.WithCredentialsFile(credentials))
	}

	client, err := fireconf.New(ctx, projectID, databaseID, config, opts...)
	if err != nil {
		return goerr.Wrap(err, "failed to create client")
	}
	defer func() { _ = client.Close() }()

	// Execute migration (dry-run logging is handled by WithDryRun option)
	if c.Bool("dry-run") {
		logger.Info("Running in dry-run mode")
	}
	if err := client.Migrate(ctx); err != nil {
		return goerr.Wrap(err, "migration failed")
	}
	if !c.Bool("dry-run") {
		logger.Info("Configuration applied successfully")
	}

	return nil
}
