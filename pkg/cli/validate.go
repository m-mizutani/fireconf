package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/m-mizutani/fireconf/pkg/domain/model"
	"github.com/m-mizutani/fireconf/pkg/usecase"
	"github.com/m-mizutani/goerr/v2"
	"github.com/urfave/cli/v3"
)

func newValidateCommand() *cli.Command {
	return &cli.Command{
		Name:  "validate",
		Usage: "Validate Firestore configuration file against constraints",
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

	// Create validator
	validator := usecase.NewValidator(logger)

	// Validate configuration
	if err := validator.Execute(ctx, &config); err != nil {
		return goerr.Wrap(err, "validation failed")
	}

	fmt.Println("✓ Configuration is valid.")
	return nil
}
