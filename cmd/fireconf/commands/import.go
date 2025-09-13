package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/m-mizutani/fireconf"
	"github.com/m-mizutani/goerr/v2"
	"github.com/urfave/cli/v3"
)

// NewImportCommand creates the import command
func NewImportCommand() *cli.Command {
	return &cli.Command{
		Name:  "import",
		Usage: "Import existing Firestore configuration",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "collections",
				Aliases: []string{"col"},
				Usage:   "Specific collections to import (imports all if not specified)",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output file path",
				Value:   "fireconf.yaml",
			},
			&cli.BoolFlag{
				Name:  "stdout",
				Usage: "Output to stdout instead of file",
			},
		},
		Action: runImport,
	}
}

func runImport(ctx context.Context, c *cli.Command) error {
	logger := getLogger(ctx)

	// Check required project flag
	projectID := c.String("project")
	if projectID == "" {
		return goerr.New("project flag is required for import command")
	}

	// Get database ID
	databaseID := c.String("database")
	if databaseID == "" {
		return goerr.New("database flag is required for import command")
	}

	// Create fireconf client
	opts := []fireconf.Option{
		fireconf.WithLogger(logger),
	}

	if credentials := c.String("credentials"); credentials != "" {
		opts = append(opts, fireconf.WithCredentialsFile(credentials))
	}

	client, err := fireconf.NewClient(ctx, projectID, databaseID, opts...)
	if err != nil {
		return goerr.Wrap(err, "failed to create client")
	}
	defer client.Close()

	// Get collections to import
	collections := c.StringSlice("collections")

	logger.Info("Importing Firestore configuration",
		"project", projectID,
		"database", databaseID,
		"collections", collections)

	// Execute import
	config, err := client.Import(ctx, collections...)
	if err != nil {
		return goerr.Wrap(err, "import failed")
	}

	// Convert to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return goerr.Wrap(err, "failed to marshal configuration")
	}

	// Output result
	if c.Bool("stdout") {
		fmt.Println(string(data))
	} else {
		outputPath := c.String("output")
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return goerr.Wrap(err, "failed to write output file")
		}
		logger.Info("Configuration imported successfully", "output", outputPath)
	}

	return nil
}
