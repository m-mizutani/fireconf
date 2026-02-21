package fireconf

import "log/slog"

// options represents client options
type options struct {
	// Logger for logging operations
	Logger *slog.Logger

	// CredentialsFile specifies the service account key file path (optional)
	CredentialsFile string

	// DryRun if true, shows what would be changed without actually applying
	DryRun bool
}

// Option is a function that configures options
type Option func(*options)

// WithLogger sets the logger
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.Logger = logger
	}
}

// WithCredentialsFile sets the credentials file path
func WithCredentialsFile(path string) Option {
	return func(o *options) {
		o.CredentialsFile = path
	}
}

// WithDryRun enables dry run mode
func WithDryRun(dryRun bool) Option {
	return func(o *options) {
		o.DryRun = dryRun
	}
}

// applyOptions applies option functions to options
func applyOptions(opts []Option) *options {
	o := &options{
		Logger: slog.New(slog.DiscardHandler),
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}
