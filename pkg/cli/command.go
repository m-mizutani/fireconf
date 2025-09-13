package cli

import (
	"context"
	"log/slog"

	"github.com/m-mizutani/clog"
	"github.com/urfave/cli/v3"
)

// NewApp creates a new CLI application
func NewApp() *cli.Command {
	return &cli.Command{
		Name:  "fireconf",
		Usage: "Firestore index and TTL configuration management tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "project",
				Aliases:  []string{"p"},
				Usage:    "Google Cloud project ID",
				Required: false,
				Sources:  cli.EnvVars("GOOGLE_CLOUD_PROJECT", "GCP_PROJECT"),
			},
			&cli.StringFlag{
				Name:    "database",
				Aliases: []string{"d"},
				Usage:   "Firestore database ID",
				Value:   "(default)",
				Sources: cli.EnvVars("FIRESTORE_DATABASE"),
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose logging",
			},
			&cli.StringFlag{
				Name:    "credentials",
				Usage:   "Path to service account key file",
				Sources: cli.EnvVars("GOOGLE_APPLICATION_CREDENTIALS"),
			},
		},
		Commands: []*cli.Command{
			newSyncCommand(),
			newImportCommand(),
			newValidateCommand(),
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			// Setup logger
			level := slog.LevelInfo
			if c.Bool("verbose") {
				level = slog.LevelDebug
			}

			handler := clog.New(
				clog.WithLevel(level),
				clog.WithColor(true),
				clog.WithSource(false),
			)
			logger := slog.New(handler)
			slog.SetDefault(logger)

			return ctx, nil
		},
	}
}

// Run executes the CLI application
func Run(args []string) error {
	app := NewApp()
	return app.Run(context.Background(), args)
}

// getLogger returns logger from context or default
func getLogger(ctx context.Context) *slog.Logger {
	return slog.Default()
}
