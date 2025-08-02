package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/m-mizutani/fireconf/pkg/adapter/firestore"
	"github.com/m-mizutani/fireconf/pkg/usecase"
	"github.com/m-mizutani/goerr/v2"
	"github.com/urfave/cli/v3"
)

func newImportCommand() *cli.Command {
	return &cli.Command{
		Name:      "import",
		Usage:     "Import existing Firestore configuration to YAML (imports all collections if none specified)",
		ArgsUsage: "[collection1] [collection2] ...]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output file path (use - for stdout)",
				Value:   "-",
			},
		},
		Action: runImport,
	}
}

func runImport(ctx context.Context, c *cli.Command) error {
	logger := getLogger(ctx)

	// Check required project flag
	if c.String("project") == "" {
		return goerr.New("project flag is required for import command")
	}

	// Create Firestore client first to check if we can list collections
	authConfig := firestore.AuthConfig{
		ProjectID:   c.String("project"),
		DatabaseID:  c.String("database"),
		Credentials: c.String("credentials"),
	}

	client, err := firestore.NewClient(ctx, authConfig)
	if err != nil {
		return goerr.Wrap(err, "failed to create Firestore client")
	}

	// Type assert to get Close method
	if closer, ok := client.(*firestore.Client); ok {
		defer closer.Close()
	}

	// Get collection names from arguments or discover all collections
	collections := c.Args().Slice()
	if len(collections) == 0 {
		logger.Info("No collections specified, discovering all collections...")
		discoveredCollections, err := client.ListCollections(ctx)
		if err != nil {
			return goerr.Wrap(err, "failed to discover collections. Please specify collection names explicitly.")
		}
		collections = discoveredCollections
		logger.Info("Discovered collections", "count", len(collections), "collections", collections)
	} else {
		logger.Info("Importing specified collections", "count", len(collections), "collections", collections)
	}

	// Create import use case
	imp := usecase.NewImport(client, logger)

	// Execute import
	config, err := imp.Execute(ctx, collections)
	if err != nil {
		return goerr.Wrap(err, "import failed")
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return goerr.Wrap(err, "failed to marshal configuration to YAML")
	}

	// Write output
	outputPath := c.String("output")
	if outputPath == "-" {
		// Write to stdout
		fmt.Print(string(data))
	} else {
		// Write to file
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return goerr.Wrap(err, "failed to write output file", goerr.V("path", outputPath))
		}
		fmt.Printf("✓ Configuration exported to %s\n", outputPath)
	}

	return nil
}
