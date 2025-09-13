package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/m-mizutani/clog"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/fireconf/cmd/fireconf/commands"
	"github.com/urfave/cli/v3"
)

var version = "dev"

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	app := &cli.Command{
		Name:    "fireconf",
		Usage:   "Firestore index and TTL configuration management tool",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "project",
				Aliases:  []string{"p"},
				Usage:    "GCP project ID",
				Sources:  cli.EnvVars("FIRECONF_PROJECT", "GCP_PROJECT"),
				Required: false, // Will be required for subcommands
			},
			&cli.StringFlag{
				Name:    "database",
				Aliases: []string{"d"},
				Usage:   "Firestore database ID",
				Value:   "(default)",
				Sources: cli.EnvVars("FIRECONF_DATABASE"),
			},
			&cli.StringFlag{
				Name:    "credentials",
				Usage:   "Service account key file path",
				Sources: cli.EnvVars("GOOGLE_APPLICATION_CREDENTIALS"),
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose logging",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug logging",
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			// Setup logger
			level := slog.LevelInfo
			if c.Bool("verbose") {
				level = slog.LevelDebug
			}
			if c.Bool("debug") {
				level = slog.LevelDebug - 1
			}

			logger := slog.New(clog.New(
				clog.WithWriter(os.Stderr),
				clog.WithLevel(level),
			))

			// Inject logger into context
			ctx = ctxlog.With(ctx, logger)

			return ctx, nil
		},
		Commands: []*cli.Command{
			commands.NewSyncCommand(),
			commands.NewImportCommand(),
			commands.NewValidateCommand(),
		},
	}

	ctx := context.Background()
	return app.Run(ctx, args)
}
