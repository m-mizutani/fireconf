package commands

import (
	"context"
	"log/slog"
	"os"

	"github.com/m-mizutani/clog"
	"github.com/m-mizutani/ctxlog"
)

// getLogger gets or creates a logger from context
func getLogger(ctx context.Context) *slog.Logger {
	if logger := ctxlog.From(ctx); logger != nil {
		return logger
	}
	return slog.New(clog.New(
		clog.WithWriter(os.Stderr),
		clog.WithLevel(slog.LevelInfo),
	))
}
