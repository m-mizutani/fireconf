package fireconf

import "log/slog"

// Options represents client options
type Options struct {
	// Logger for logging operations
	Logger *slog.Logger

	// DatabaseID specifies the Firestore database ID (default: "(default)")
	DatabaseID string

	// CredentialsFile specifies the service account key file path (optional)
	CredentialsFile string

	// DryRun if true, shows what would be changed without actually applying
	DryRun bool

	// Verbose enables verbose logging
	Verbose bool
}

// Option is a function that configures Options
type Option func(*Options)

// WithLogger sets the logger
func WithLogger(logger *slog.Logger) Option {
	return func(o *Options) {
		o.Logger = logger
	}
}

// WithDatabaseID sets the database ID
func WithDatabaseID(databaseID string) Option {
	return func(o *Options) {
		o.DatabaseID = databaseID
	}
}

// WithCredentialsFile sets the credentials file path
func WithCredentialsFile(path string) Option {
	return func(o *Options) {
		o.CredentialsFile = path
	}
}

// WithDryRun enables dry run mode
func WithDryRun(dryRun bool) Option {
	return func(o *Options) {
		o.DryRun = dryRun
	}
}

// WithVerbose enables verbose logging
func WithVerbose(verbose bool) Option {
	return func(o *Options) {
		o.Verbose = verbose
	}
}

// applyOptions applies option functions to Options
func applyOptions(opts []Option) *Options {
	options := &Options{
		Logger:     slog.Default(),
		DatabaseID: "(default)",
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}
