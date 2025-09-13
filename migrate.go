package fireconf

import (
	"context"
	"fmt"

	"github.com/m-mizutani/goerr/v2"
)

// MigrateOptions represents options for migration
type MigrateOptions struct {
	// DryRun if true, shows what would be changed without actually applying
	DryRun bool

	// Force if true, applies changes even if there are destructive operations
	Force bool

	// ProgressCallback is called for each operation
	ProgressCallback func(operation string, collection string)
}

// MigrateWithOptions applies configuration with options
func (c *Client) MigrateWithOptions(ctx context.Context, config *Config, opts MigrateOptions) error {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return goerr.Wrap(err, "invalid configuration")
	}

	if opts.DryRun {
		return c.dryRunMigrate(ctx, config, opts)
	}

	return c.Migrate(ctx, config)
}

// dryRunMigrate performs a dry run of migration
func (c *Client) dryRunMigrate(ctx context.Context, config *Config, opts MigrateOptions) error {
	// Import current configuration
	current, err := c.Import(ctx)
	if err != nil {
		return goerr.Wrap(err, "failed to get current configuration")
	}

	// Calculate diff
	diff := Diff(current, config)

	// Display what would be changed
	for _, colDiff := range diff.Collections {
		switch colDiff.Action {
		case ActionAdd:
			c.logger.Info("Would create collection",
				"collection", colDiff.Name,
				"indexes", len(colDiff.Indexes),
				"ttl", colDiff.TTL != nil)

		case ActionModify:
			if len(colDiff.IndexesToAdd) > 0 {
				c.logger.Info("Would add indexes",
					"collection", colDiff.Name,
					"count", len(colDiff.IndexesToAdd))
			}
			if len(colDiff.IndexesToDelete) > 0 {
				c.logger.Info("Would delete indexes",
					"collection", colDiff.Name,
					"count", len(colDiff.IndexesToDelete))
			}
			if colDiff.TTLAction != "" {
				c.logger.Info("Would modify TTL",
					"collection", colDiff.Name,
					"action", colDiff.TTLAction)
			}

		case ActionDelete:
			c.logger.Info("Would delete collection",
				"collection", colDiff.Name)
		}

		if opts.ProgressCallback != nil {
			opts.ProgressCallback(string(colDiff.Action), colDiff.Name)
		}
	}

	return nil
}

// MigrationPlan represents a plan for migration
type MigrationPlan struct {
	Steps []MigrationStep
}

// MigrationStep represents a single migration step
type MigrationStep struct {
	Collection  string
	Operation   string
	Description string
	Destructive bool
}

// GetMigrationPlan returns a migration plan without executing
func (c *Client) GetMigrationPlan(ctx context.Context, config *Config) (*MigrationPlan, error) {
	// Import current configuration
	current, err := c.Import(ctx)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get current configuration")
	}

	// Calculate diff
	diff := Diff(current, config)

	plan := &MigrationPlan{
		Steps: make([]MigrationStep, 0),
	}

	// Generate steps from diff
	for _, colDiff := range diff.Collections {
		switch colDiff.Action {
		case ActionAdd:
			plan.Steps = append(plan.Steps, MigrationStep{
				Collection:  colDiff.Name,
				Operation:   "CREATE_COLLECTION",
				Description: fmt.Sprintf("Create collection %s with %d indexes", colDiff.Name, len(colDiff.Indexes)),
				Destructive: false,
			})

		case ActionModify:
			for i := range colDiff.IndexesToAdd {
				plan.Steps = append(plan.Steps, MigrationStep{
					Collection:  colDiff.Name,
					Operation:   "CREATE_INDEX",
					Description: fmt.Sprintf("Create index #%d on collection %s", i+1, colDiff.Name),
					Destructive: false,
				})
			}

			for i := range colDiff.IndexesToDelete {
				plan.Steps = append(plan.Steps, MigrationStep{
					Collection:  colDiff.Name,
					Operation:   "DELETE_INDEX",
					Description: fmt.Sprintf("Delete index #%d from collection %s", i+1, colDiff.Name),
					Destructive: true,
				})
			}

			if colDiff.TTLAction == ActionAdd {
				plan.Steps = append(plan.Steps, MigrationStep{
					Collection:  colDiff.Name,
					Operation:   "ENABLE_TTL",
					Description: fmt.Sprintf("Enable TTL on field %s for collection %s", colDiff.TTL.Field, colDiff.Name),
					Destructive: false,
				})
			} else if colDiff.TTLAction == ActionDelete {
				plan.Steps = append(plan.Steps, MigrationStep{
					Collection:  colDiff.Name,
					Operation:   "DISABLE_TTL",
					Description: fmt.Sprintf("Disable TTL for collection %s", colDiff.Name),
					Destructive: true,
				})
			}

		case ActionDelete:
			plan.Steps = append(plan.Steps, MigrationStep{
				Collection:  colDiff.Name,
				Operation:   "DELETE_COLLECTION",
				Description: fmt.Sprintf("Delete collection %s", colDiff.Name),
				Destructive: true,
			})
		}
	}

	return plan, nil
}
