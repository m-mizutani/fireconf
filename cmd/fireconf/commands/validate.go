package commands

import (
	"context"
	"fmt"

	"github.com/m-mizutani/fireconf"
	"github.com/m-mizutani/goerr/v2"
	"github.com/urfave/cli/v3"
)

// NewValidateCommand creates the validate command
func NewValidateCommand() *cli.Command {
	return &cli.Command{
		Name:  "validate",
		Usage: "Validate configuration file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Configuration file path",
				Value:   "fireconf.yaml",
			},
		},
		Action: runValidate,
	}
}

func runValidate(ctx context.Context, c *cli.Command) error {
	logger := getLogger(ctx)

	// Read configuration file
	configPath := c.String("config")
	logger.Info("Validating configuration file", "path", configPath)

	// Load configuration from YAML
	config, err := fireconf.LoadConfigFromYAML(configPath)
	if err != nil {
		return goerr.Wrap(err, "failed to load configuration")
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return goerr.Wrap(err, "validation failed")
	}

	// Print summary
	fmt.Printf("✓ Configuration is valid\n")
	fmt.Printf("  Collections: %d\n", len(config.Collections))

	totalIndexes := 0
	ttlCount := 0
	for _, col := range config.Collections {
		totalIndexes += len(col.Indexes)
		if col.TTL != nil {
			ttlCount++
		}
	}

	fmt.Printf("  Total indexes: %d\n", totalIndexes)
	fmt.Printf("  TTL policies: %d\n", ttlCount)

	if c.Bool("verbose") {
		fmt.Println("\nDetails:")
		for _, col := range config.Collections {
			fmt.Printf("  - %s:\n", col.Name)
			fmt.Printf("      Indexes: %d\n", len(col.Indexes))
			if col.TTL != nil {
				fmt.Printf("      TTL field: %s\n", col.TTL.Field)
			}
		}
	}

	return nil
}
