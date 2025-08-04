package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/m-mizutani/fireconf/pkg/domain/interfaces"
	"github.com/m-mizutani/fireconf/pkg/domain/model"
	"github.com/m-mizutani/goerr/v2"
	"golang.org/x/sync/errgroup"
)

// Sync handles synchronization of Firestore configuration
type Sync struct {
	client   interfaces.FirestoreClient
	logger   *slog.Logger
	dryRun   bool
	skipWait bool
}

// NewSync creates a new Sync use case
func NewSync(client interfaces.FirestoreClient, logger *slog.Logger, dryRun bool) *Sync {
	return &Sync{
		client:   client,
		logger:   logger,
		dryRun:   dryRun,
		skipWait: false,
	}
}

// NewSyncWithOptions creates a new Sync use case with options
func NewSyncWithOptions(client interfaces.FirestoreClient, logger *slog.Logger, dryRun bool, skipWait bool) *Sync {
	return &Sync{
		client:   client,
		logger:   logger,
		dryRun:   dryRun,
		skipWait: skipWait,
	}
}

// Execute synchronizes the configuration
func (s *Sync) Execute(ctx context.Context, config *model.Config) error {
	s.logger.Info("Starting sync operation", slog.Bool("dryRun", s.dryRun))

	// Process each collection
	for _, collection := range config.Collections {
		s.logger.Info("Processing collection", slog.String("name", collection.Name))

		// Validate collection
		if err := collection.Validate(); err != nil {
			return goerr.Wrap(err, "invalid collection configuration", goerr.V("collection", collection.Name))
		}

		// Ensure collection exists before processing indexes/TTL
		if err := s.ensureCollectionExists(ctx, collection.Name); err != nil {
			return goerr.Wrap(err, "failed to ensure collection exists", goerr.V("collection", collection.Name))
		}

		// Sync indexes
		if err := s.syncIndexes(ctx, collection); err != nil {
			return goerr.Wrap(err, "failed to sync indexes", goerr.V("collection", collection.Name))
		}

		// Sync TTL
		if err := s.syncTTL(ctx, collection); err != nil {
			return goerr.Wrap(err, "failed to sync TTL", goerr.V("collection", collection.Name))
		}
	}

	s.logger.Info("Sync operation completed successfully")
	return nil
}

// syncIndexes synchronizes indexes for a collection
func (s *Sync) syncIndexes(ctx context.Context, collection model.Collection) error {
	// Get existing indexes
	existing, err := s.client.ListIndexes(ctx, collection.Name)
	if err != nil {
		return goerr.Wrap(err, "failed to list existing indexes")
	}

	s.logger.Debug("Found existing indexes",
		slog.String("collection", collection.Name),
		slog.Int("count", len(existing)))

	// Calculate diff
	toCreate, toDelete := DiffIndexes(collection.Indexes, existing)

	s.logger.Info("Index diff calculated",
		slog.String("collection", collection.Name),
		slog.Int("desired", len(collection.Indexes)),
		slog.Int("existing", len(existing)),
		slog.Int("toCreate", len(toCreate)),
		slog.Int("toDelete", len(toDelete)))

	// Debug: Log detailed index information
	if s.logger.Enabled(context.Background(), slog.LevelDebug) {
		s.logger.Debug("Desired indexes",
			slog.String("collection", collection.Name))
		for i, idx := range collection.Indexes {
			s.logger.Debug("  Desired index",
				slog.Int("index", i),
				slog.Any("fields", convertModelToFirestoreIndex(idx).Fields),
				slog.String("queryScope", idx.QueryScope))
		}

		s.logger.Debug("Existing indexes",
			slog.String("collection", collection.Name))
		for i, idx := range existing {
			s.logger.Debug("  Existing index",
				slog.Int("index", i),
				slog.String("name", idx.Name),
				slog.String("state", idx.State),
				slog.Any("fields", idx.Fields),
				slog.String("queryScope", idx.QueryScope))
		}

		if len(toCreate) > 0 {
			s.logger.Debug("Indexes to create",
				slog.String("collection", collection.Name))
			for i, idx := range toCreate {
				s.logger.Debug("  Create index",
					slog.Int("index", i),
					slog.Any("fields", idx.Fields),
					slog.String("queryScope", idx.QueryScope))
			}
		}

		if len(toDelete) > 0 {
			s.logger.Debug("Indexes to delete",
				slog.String("collection", collection.Name))
			for i, idx := range toDelete {
				s.logger.Debug("  Delete index",
					slog.Int("index", i),
					slog.String("name", idx.Name),
					slog.Any("fields", idx.Fields),
					slog.String("queryScope", idx.QueryScope))
			}
		}
	}

	// Delete indexes that are no longer needed
	for _, idx := range toDelete {
		if s.dryRun {
			s.logger.Info("Would delete index",
				slog.String("collection", collection.Name),
				slog.String("index", idx.Name))
			continue
		}

		s.logger.Info("Deleting index",
			slog.String("collection", collection.Name),
			slog.String("index", idx.Name))

		op, err := s.client.DeleteIndex(ctx, idx.Name)
		if err != nil {
			return goerr.Wrap(err, "failed to delete index", goerr.V("index", idx.Name))
		}

		if !s.skipWait && op != nil {
			s.logger.Info("Waiting for index deletion to complete",
				slog.String("collection", collection.Name),
				slog.String("index", idx.Name))
			if err := s.client.WaitForOperation(ctx, op); err != nil {
				return goerr.Wrap(err, "failed to wait for index deletion", goerr.V("index", idx.Name))
			}
		}
	}

	// Create new indexes
	if len(toCreate) > 0 {
		if err := s.createIndexesConcurrently(ctx, collection.Name, toCreate); err != nil {
			return err
		}
	}

	return nil
}

// createIndexesConcurrently creates multiple indexes in parallel
func (s *Sync) createIndexesConcurrently(ctx context.Context, collectionName string, indexes []interfaces.FirestoreIndex) error {
	g, ctx := errgroup.WithContext(ctx)

	// Limit concurrent operations
	sem := make(chan struct{}, 5)

	for _, idx := range indexes {
		idx := idx // capture

		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			if s.dryRun {
				s.logger.Info("Would create index",
					slog.String("collection", collectionName),
					slog.Any("fields", idx.Fields),
					slog.String("queryScope", idx.QueryScope))
				return nil
			}

			s.logger.Info("Creating index",
				slog.String("collection", collectionName),
				slog.Any("fields", idx.Fields),
				slog.String("queryScope", idx.QueryScope))

			op, err := s.client.CreateIndex(ctx, collectionName, idx)
			if err != nil {
				return goerr.Wrap(err, "failed to create index",
					goerr.V("collection", collectionName),
					goerr.V("fields", idx.Fields))
			}

			if !s.skipWait && op != nil {
				s.logger.Info("Waiting for index creation to complete",
					slog.String("collection", collectionName),
					slog.Any("fields", idx.Fields))
				if err := s.client.WaitForOperation(ctx, op); err != nil {
					return goerr.Wrap(err, "failed to wait for index creation",
						goerr.V("collection", collectionName),
						goerr.V("fields", idx.Fields))
				}
			}

			return nil
		})
	}

	return g.Wait()
}

// syncTTL synchronizes TTL policy for a collection
func (s *Sync) syncTTL(ctx context.Context, collection model.Collection) error {
	// If no TTL is desired, check if we need to disable existing TTL
	if collection.TTL == nil {
		if s.dryRun {
			s.logger.Info("Would check and disable TTL if exists",
				slog.String("collection", collection.Name))
			return nil
		}

		// Disable any existing TTL
		op, err := s.client.DisableTTLPolicy(ctx, collection.Name)
		if err != nil {
			return goerr.Wrap(err, "failed to disable TTL policy")
		}

		if !s.skipWait && op != nil {
			s.logger.Info("Waiting for TTL policy disable to complete",
				slog.String("collection", collection.Name))
			if err := s.client.WaitForOperation(ctx, op); err != nil {
				return goerr.Wrap(err, "failed to wait for TTL policy disable")
			}
		}
		return nil
	}

	// Get current TTL policy
	existing, err := s.client.GetTTLPolicy(ctx, collection.Name, collection.TTL.Field)
	if err != nil {
		return goerr.Wrap(err, "failed to get TTL policy")
	}

	// Check if update is needed
	needsUpdate, action := DiffTTL(collection.TTL, existing)
	if !needsUpdate {
		s.logger.Debug("TTL policy is up to date",
			slog.String("collection", collection.Name),
			slog.String("field", collection.TTL.Field))
		return nil
	}

	if s.dryRun {
		s.logger.Info(fmt.Sprintf("Would %s TTL policy", action),
			slog.String("collection", collection.Name),
			slog.String("field", collection.TTL.Field))
		return nil
	}

	// Apply TTL policy
	switch action {
	case "enable":
		s.logger.Info("Enabling TTL policy",
			slog.String("collection", collection.Name),
			slog.String("field", collection.TTL.Field))

		op, err := s.client.EnableTTLPolicy(ctx, collection.Name, collection.TTL.Field)
		if err != nil {
			return goerr.Wrap(err, "failed to enable TTL policy")
		}

		if !s.skipWait && op != nil {
			s.logger.Info("Waiting for TTL policy enable to complete",
				slog.String("collection", collection.Name),
				slog.String("field", collection.TTL.Field))
			if err := s.client.WaitForOperation(ctx, op); err != nil {
				return goerr.Wrap(err, "failed to wait for TTL policy enable")
			}
		}

	case "change":
		// Disable old TTL first
		s.logger.Info("Changing TTL field, disabling old policy",
			slog.String("collection", collection.Name))

		op, err := s.client.DisableTTLPolicy(ctx, collection.Name)
		if err != nil {
			return goerr.Wrap(err, "failed to disable old TTL policy")
		}

		if !s.skipWait && op != nil {
			s.logger.Info("Waiting for old TTL policy disable to complete",
				slog.String("collection", collection.Name))
			if err := s.client.WaitForOperation(ctx, op); err != nil {
				return goerr.Wrap(err, "failed to wait for old TTL policy disable")
			}
		}

		// Enable new TTL
		s.logger.Info("Enabling new TTL policy",
			slog.String("collection", collection.Name),
			slog.String("field", collection.TTL.Field))

		op, err = s.client.EnableTTLPolicy(ctx, collection.Name, collection.TTL.Field)
		if err != nil {
			return goerr.Wrap(err, "failed to enable new TTL policy")
		}

		if !s.skipWait && op != nil {
			s.logger.Info("Waiting for new TTL policy enable to complete",
				slog.String("collection", collection.Name),
				slog.String("field", collection.TTL.Field))
			if err := s.client.WaitForOperation(ctx, op); err != nil {
				return goerr.Wrap(err, "failed to wait for new TTL policy enable")
			}
		}
	}

	return nil
}

// ensureCollectionExists ensures a collection exists before syncing
func (s *Sync) ensureCollectionExists(ctx context.Context, collectionName string) error {
	if s.dryRun {
		s.logger.Info("Would ensure collection exists", slog.String("collection", collectionName))
		return nil
	}

	exists, err := s.client.CollectionExists(ctx, collectionName)
	if err != nil {
		return goerr.Wrap(err, "failed to check collection existence")
	}

	if !exists {
		s.logger.Info("Collection does not exist, creating it", slog.String("collection", collectionName))
		if err := s.client.CreateCollection(ctx, collectionName); err != nil {
			return goerr.Wrap(err, "failed to create collection")
		}
		s.logger.Info("Collection created successfully", slog.String("collection", collectionName))
	} else {
		s.logger.Debug("Collection already exists", slog.String("collection", collectionName))
	}

	return nil
}

