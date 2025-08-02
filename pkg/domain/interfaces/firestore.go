package interfaces

import (
	"context"
)

//go:generate task mock

// FirestoreClient is the interface for Firestore Admin API operations
type FirestoreClient interface {
	// Collection operations
	ListCollections(ctx context.Context) ([]string, error)
	CollectionExists(ctx context.Context, collectionID string) (bool, error)
	CreateCollection(ctx context.Context, collectionID string) error

	// Index operations
	ListIndexes(ctx context.Context, collectionID string) ([]FirestoreIndex, error)
	CreateIndex(ctx context.Context, collectionID string, index FirestoreIndex) (interface{}, error)
	DeleteIndex(ctx context.Context, indexName string) (interface{}, error)

	// TTL operations
	GetTTLPolicy(ctx context.Context, collectionID string, fieldName string) (*FirestoreTTL, error)
	EnableTTLPolicy(ctx context.Context, collectionID string, fieldName string) (interface{}, error)
	DisableTTLPolicy(ctx context.Context, collectionID string) (interface{}, error)

	// Wait for operation to complete
	WaitForOperation(ctx context.Context, operation interface{}) error
}

// FirestoreIndex represents a Firestore index
type FirestoreIndex struct {
	Name       string
	Fields     []FirestoreIndexField
	QueryScope string
	State      string
}

// FirestoreIndexField represents a field in a Firestore index
type FirestoreIndexField struct {
	FieldPath    string
	Order        string
	ArrayConfig  string
	VectorConfig *FirestoreVectorConfig
}

// FirestoreVectorConfig represents vector configuration
type FirestoreVectorConfig struct {
	Dimension int
}

// FirestoreTTL represents a TTL policy
type FirestoreTTL struct {
	FieldPath string
	State     string // ENABLED or DISABLED
}
