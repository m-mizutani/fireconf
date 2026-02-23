package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/m-mizutani/fireconf/internal/interfaces"
	"github.com/m-mizutani/fireconf/internal/model"
	"github.com/m-mizutani/goerr/v2"
	"golang.org/x/sync/errgroup"
)

// SyncOption configures a Sync use case
type SyncOption func(*Sync)

// SyncWithDryRun enables dry run mode (no actual changes are made)
func SyncWithDryRun() SyncOption {
	return func(s *Sync) { s.dryRun = true }
}

// SyncWithAsync skips waiting for indexes/operations to complete
func SyncWithAsync() SyncOption {
	return func(s *Sync) { s.async = true }
}

// Sync handles synchronization of Firestore configuration
type Sync struct {
	client interfaces.FirestoreClient
	logger *slog.Logger
	dryRun bool
	async  bool
}

// NewSync creates a new Sync use case
func NewSync(client interfaces.FirestoreClient, logger *slog.Logger, opts ...SyncOption) *Sync {
	s := &Sync{
		client: client,
		logger: logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Execute synchronizes the configuration
func (s *Sync) Execute(ctx context.Context, config *model.Config) error {
	s.logger.Info("Starting sync operation", slog.Bool("dryRun", s.dryRun))

	// Process collections in parallel
	g, ctx := errgroup.WithContext(ctx)

	// Limit concurrent collection processing
	sem := make(chan struct{}, 10) // Process up to 10 collections concurrently

	for _, collection := range config.Collections {
		collection := collection // capture

		g.Go(func() error {
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			s.logger.Info("Processing collection", slog.String("name", collection.Name))

			// Validate collection
			if err := collection.Validate(); err != nil {
				return goerr.Wrap(err, "invalid collection configuration", goerr.V("collection", collection.Name))
			}

			// Ensure collection exists before processing indexes/TTL
			if err := s.ensureCollectionExists(ctx, collection.Name); err != nil {
				return goerr.Wrap(err, "failed to ensure collection exists", goerr.V("collection", collection.Name))
			}

			// Sync indexes and TTL in parallel (they are independent)
			cg, cctx := errgroup.WithContext(ctx)
			cg.Go(func() error {
				if err := s.syncIndexes(cctx, collection); err != nil {
					return goerr.Wrap(err, "failed to sync indexes", goerr.V("collection", collection.Name))
				}
				return nil
			})
			cg.Go(func() error {
				if err := s.syncTTL(cctx, collection); err != nil {
					return goerr.Wrap(err, "failed to sync TTL", goerr.V("collection", collection.Name))
				}
				return nil
			})
			if err := cg.Wait(); err != nil {
				return err
			}

			s.logger.Info("Collection processing completed", slog.String("name", collection.Name))
			return nil
		})
	}

	// Wait for all collections to complete
	if err := g.Wait(); err != nil {
		return err
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

		if !s.async && op != nil {
			s.logger.Info("Waiting for index deletion to complete",
				slog.String("collection", collection.Name),
				slog.String("index", idx.Name))

			progressLogger := func(elapsed time.Duration) {
				s.logger.Info("Still waiting for index deletion...",
					slog.String("collection", collection.Name),
					slog.String("index", idx.Name),
					slog.Duration("elapsed", elapsed))
			}

			if err := s.waitForOperationWithProgress(ctx, op, progressLogger); err != nil {
				return goerr.Wrap(err, "failed to wait for index deletion", goerr.V("index", idx.Name))
			}
		}
	}

	// Create new indexes and collect the created index names
	var createdIndexNames []string
	if len(toCreate) > 0 {
		names, err := s.createIndexesConcurrently(ctx, collection.Name, toCreate)
		if err != nil {
			return err
		}
		createdIndexNames = names
	}

	// Wait for the newly created indexes to reach READY state by polling each by name
	if err := s.waitForIndexesReady(ctx, createdIndexNames); err != nil {
		return goerr.Wrap(err, "failed to wait for indexes to become ready")
	}

	return nil
}

// createIndexesConcurrently creates multiple indexes in parallel and returns their resource names.
// Index creation requests are submitted concurrently without waiting for each
// individual LRO to complete. The caller is responsible for waiting via
// waitForIndexesReady, which polls each index by name directly.
// This avoids potential hangs in the Firestore LRO polling mechanism.
func (s *Sync) createIndexesConcurrently(ctx context.Context, collectionName string, indexes []interfaces.FirestoreIndex) ([]string, error) {
	g, ctx := errgroup.WithContext(ctx)

	// Limit concurrent operations
	sem := make(chan struct{}, 5)

	var mu sync.Mutex
	var createdNames []string

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

			name, err := s.client.CreateIndex(ctx, collectionName, idx)
			if err != nil {
				return goerr.Wrap(err, "failed to create index",
					goerr.V("collection", collectionName),
					goerr.V("fields", idx.Fields))
			}

			if name != "" {
				mu.Lock()
				createdNames = append(createdNames, name)
				mu.Unlock()
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return createdNames, nil
}

// syncTTL synchronizes TTL policy for a collection.
// TTL operations are submitted as fire-and-forget since op.Wait() hangs indefinitely
// for Firestore UpdateField LROs. TTL changes are applied asynchronously by Firestore.
func (s *Sync) syncTTL(ctx context.Context, collection model.Collection) error {
	if collection.TTL == nil {
		if s.dryRun {
			s.logger.Info("Would check and disable TTL if exists",
				slog.String("collection", collection.Name))
			return nil
		}
		if _, err := s.client.DisableTTLPolicy(ctx, collection.Name); err != nil {
			return goerr.Wrap(err, "failed to disable TTL policy")
		}
		return nil
	}

	existing, err := s.client.GetTTLPolicy(ctx, collection.Name, collection.TTL.Field)
	if err != nil {
		return goerr.Wrap(err, "failed to get TTL policy")
	}

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

	switch action {
	case "enable":
		s.logger.Info("Enabling TTL policy",
			slog.String("collection", collection.Name),
			slog.String("field", collection.TTL.Field))
		if _, err := s.client.EnableTTLPolicy(ctx, collection.Name, collection.TTL.Field); err != nil {
			return goerr.Wrap(err, "failed to enable TTL policy")
		}

	case "change":
		s.logger.Info("Changing TTL field, disabling old policy",
			slog.String("collection", collection.Name))
		if _, err := s.client.DisableTTLPolicy(ctx, collection.Name); err != nil {
			return goerr.Wrap(err, "failed to disable old TTL policy")
		}
		s.logger.Info("Enabling new TTL policy",
			slog.String("collection", collection.Name),
			slog.String("field", collection.TTL.Field))
		if _, err := s.client.EnableTTLPolicy(ctx, collection.Name, collection.TTL.Field); err != nil {
			return goerr.Wrap(err, "failed to enable new TTL policy")
		}
	}

	return nil
}

// waitForIndexesReady waits for each created index to reach a stable state in parallel.
// Each index is polled individually via GetIndex, which avoids being blocked by
// unrelated indexes in the same collection.
func (s *Sync) waitForIndexesReady(ctx context.Context, indexNames []string) error {
	if s.async || s.dryRun || len(indexNames) == 0 {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, name := range indexNames {
		name := name
		g.Go(func() error {
			return s.waitForSingleIndexReady(ctx, name)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	s.logger.Info("All indexes are ready")
	return nil
}

// waitForSingleIndexReady polls a single index by name until it reaches READY state.
func (s *Sync) waitForSingleIndexReady(ctx context.Context, indexName string) error {
	backoff := time.Second
	maxBackoff := 10 * time.Second
	lastLog := time.Now()
	logInterval := 10 * time.Second

	for {
		idx, err := s.client.GetIndex(ctx, indexName)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Warn("Failed to get index while waiting, retrying",
				slog.String("index", indexName),
				slog.Any("error", err))
		} else {
			switch idx.State {
			case "READY":
				s.logger.Info("Index is ready", slog.String("index", indexName))
				return nil
			case "ERROR", "NEEDS_REPAIR":
				return goerr.New(fmt.Sprintf("index entered %s state", idx.State),
					goerr.V("index", indexName))
			default:
				// CREATING or other transitional state — keep waiting
			}
		}

		if time.Since(lastLog) >= logInterval {
			s.logger.Info("Waiting for index to become READY",
				slog.String("index", indexName))
			lastLog = time.Now()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// waitForOperationWithProgress is a helper method that wraps client wait with progress logging
func (s *Sync) waitForOperationWithProgress(ctx context.Context, operation interface{}, progressFunc func(time.Duration)) error {
	// Use custom wait logic with progress reporting
	start := time.Now()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	done := make(chan error, 1)

	// Start the wait operation in a goroutine
	go func() {
		done <- s.client.WaitForOperation(ctx, operation)
	}()

	// Log progress every 10 seconds
	for {
		select {
		case err := <-done:
			return err
		case <-ticker.C:
			if progressFunc != nil {
				progressFunc(time.Since(start))
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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
