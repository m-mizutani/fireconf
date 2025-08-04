package usecase_test

import (
	"testing"

	"github.com/m-mizutani/fireconf/pkg/domain/interfaces"
	"github.com/m-mizutani/fireconf/pkg/domain/model"
	"github.com/m-mizutani/fireconf/pkg/usecase"
	"github.com/m-mizutani/gt"
)

func TestDiffIndexes(t *testing.T) {
	t.Run("No changes needed", func(t *testing.T) {
		desired := []model.Index{
			{
				Fields: []model.IndexField{
					{Name: "email", Order: "ASCENDING"},
					{Name: "createdAt", Order: "DESCENDING"},
				},
				QueryScope: "COLLECTION",
			},
		}

		existing := []interfaces.FirestoreIndex{
			{
				Fields: []interfaces.FirestoreIndexField{
					{FieldPath: "email", Order: "ASCENDING"},
					{FieldPath: "createdAt", Order: "DESCENDING"},
				},
				QueryScope: "COLLECTION",
			},
		}

		toCreate, toDelete := usecase.DiffIndexes(desired, existing)
		gt.Equal(t, len(toCreate), 0)
		gt.Equal(t, len(toDelete), 0)
	})

	t.Run("Create new index", func(t *testing.T) {
		desired := []model.Index{
			{
				Fields: []model.IndexField{
					{Name: "status", Order: "ASCENDING"},
					{Name: "updatedAt", Order: "DESCENDING"},
				},
				QueryScope: "COLLECTION",
			},
		}

		existing := []interfaces.FirestoreIndex{}

		toCreate, toDelete := usecase.DiffIndexes(desired, existing)
		gt.Equal(t, len(toCreate), 1)
		gt.Equal(t, len(toDelete), 0)
		gt.Equal(t, toCreate[0].Fields[0].FieldPath, "status")
		gt.Equal(t, toCreate[0].Fields[1].FieldPath, "updatedAt")
	})

	t.Run("Delete obsolete index", func(t *testing.T) {
		desired := []model.Index{}

		existing := []interfaces.FirestoreIndex{
			{
				Name: "projects/test/databases/default/collectionGroups/users/indexes/idx1",
				Fields: []interfaces.FirestoreIndexField{
					{FieldPath: "email", Order: "ASCENDING"},
				},
				QueryScope: "COLLECTION",
			},
		}

		toCreate, toDelete := usecase.DiffIndexes(desired, existing)
		gt.Equal(t, len(toCreate), 0)
		gt.Equal(t, len(toDelete), 1)
		gt.Equal(t, toDelete[0].Name, "projects/test/databases/default/collectionGroups/users/indexes/idx1")
	})

	t.Run("Replace index with different fields", func(t *testing.T) {
		desired := []model.Index{
			{
				Fields: []model.IndexField{
					{Name: "email", Order: "ASCENDING"},
					{Name: "name", Order: "ASCENDING"},
				},
				QueryScope: "COLLECTION",
			},
		}

		existing := []interfaces.FirestoreIndex{
			{
				Name: "projects/test/databases/default/collectionGroups/users/indexes/idx1",
				Fields: []interfaces.FirestoreIndexField{
					{FieldPath: "email", Order: "ASCENDING"},
					{FieldPath: "createdAt", Order: "DESCENDING"},
				},
				QueryScope: "COLLECTION",
			},
		}

		toCreate, toDelete := usecase.DiffIndexes(desired, existing)
		gt.Equal(t, len(toCreate), 1)
		gt.Equal(t, len(toDelete), 1)
	})

	t.Run("Handle array config fields", func(t *testing.T) {
		desired := []model.Index{
			{
				Fields: []model.IndexField{
					{Name: "tags", ArrayConfig: "CONTAINS"},
					{Name: "score", Order: "DESCENDING"},
				},
				QueryScope: "COLLECTION",
			},
		}

		existing := []interfaces.FirestoreIndex{}

		toCreate, _ := usecase.DiffIndexes(desired, existing)
		gt.Equal(t, len(toCreate), 1)
		gt.Equal(t, toCreate[0].Fields[0].ArrayConfig, "CONTAINS")
		gt.Equal(t, toCreate[0].Fields[1].Order, "DESCENDING")
	})

	t.Run("Handle vector config fields", func(t *testing.T) {
		desired := []model.Index{
			{
				Fields: []model.IndexField{
					{Name: "title", Order: "ASCENDING"},
					{Name: "__name__", Order: "ASCENDING"},
					{
						Name:         "embedding",
						VectorConfig: &model.VectorConfig{Dimension: 768},
					},
				},
				QueryScope: "COLLECTION",
			},
		}

		existing := []interfaces.FirestoreIndex{}

		toCreate, _ := usecase.DiffIndexes(desired, existing)
		gt.Equal(t, len(toCreate), 1)
		gt.NotEqual(t, toCreate[0].Fields[2].VectorConfig, nil)
		gt.Equal(t, toCreate[0].Fields[2].VectorConfig.Dimension, 768)
	})

	t.Run("Different query scopes", func(t *testing.T) {
		desired := []model.Index{
			{
				Fields: []model.IndexField{
					{Name: "status", Order: "ASCENDING"},
				},
				QueryScope: "COLLECTION_GROUP",
			},
		}

		existing := []interfaces.FirestoreIndex{
			{
				Name: "projects/test/databases/default/collectionGroups/users/indexes/idx1",
				Fields: []interfaces.FirestoreIndexField{
					{FieldPath: "status", Order: "ASCENDING"},
				},
				QueryScope: "COLLECTION",
			},
		}

		toCreate, toDelete := usecase.DiffIndexes(desired, existing)
		gt.Equal(t, len(toCreate), 1)
		gt.Equal(t, len(toDelete), 1)
		gt.Equal(t, toCreate[0].QueryScope, "COLLECTION_GROUP")
	})
}

func TestDiffTTL(t *testing.T) {
	t.Run("No TTL desired and none exists", func(t *testing.T) {
		needsUpdate, action := usecase.DiffTTL(nil, nil)
		gt.Equal(t, needsUpdate, false)
		gt.Equal(t, action, "")
	})

	t.Run("No TTL desired but exists", func(t *testing.T) {
		existing := &interfaces.FirestoreTTL{
			State:     "ACTIVE",
			FieldPath: "expireAt",
		}

		needsUpdate, action := usecase.DiffTTL(nil, existing)
		gt.Equal(t, needsUpdate, true)
		gt.Equal(t, action, "disable")
	})

	t.Run("TTL desired but none exists", func(t *testing.T) {
		desired := &model.TTL{
			Field: "expireAt",
		}

		needsUpdate, action := usecase.DiffTTL(desired, nil)
		gt.Equal(t, needsUpdate, true)
		gt.Equal(t, action, "enable")
	})

	t.Run("TTL exists in creating state", func(t *testing.T) {
		desired := &model.TTL{
			Field: "expireAt",
		}
		existing := &interfaces.FirestoreTTL{
			State:     "CREATING",
			FieldPath: "expireAt",
		}

		needsUpdate, action := usecase.DiffTTL(desired, existing)
		gt.Equal(t, needsUpdate, false)
		gt.Equal(t, action, "")
	})

	t.Run("TTL field changed", func(t *testing.T) {
		desired := &model.TTL{
			Field: "deletedAt",
		}
		existing := &interfaces.FirestoreTTL{
			State:     "ACTIVE",
			FieldPath: "expireAt",
		}

		needsUpdate, action := usecase.DiffTTL(desired, existing)
		gt.Equal(t, needsUpdate, true)
		gt.Equal(t, action, "change")
	})

	t.Run("TTL exists but inactive", func(t *testing.T) {
		desired := &model.TTL{
			Field: "expireAt",
		}
		existing := &interfaces.FirestoreTTL{
			State:     "INACTIVE",
			FieldPath: "expireAt",
		}

		needsUpdate, action := usecase.DiffTTL(desired, existing)
		gt.Equal(t, needsUpdate, true)
		gt.Equal(t, action, "enable")
	})

	t.Run("TTL matches exactly", func(t *testing.T) {
		desired := &model.TTL{
			Field: "expireAt",
		}
		existing := &interfaces.FirestoreTTL{
			State:     "ACTIVE",
			FieldPath: "expireAt",
		}

		needsUpdate, action := usecase.DiffTTL(desired, existing)
		gt.Equal(t, needsUpdate, false)
		gt.Equal(t, action, "")
	})
}

func TestConvertFirestoreToModelIndex(t *testing.T) {
	t.Run("Convert basic index", func(t *testing.T) {
		firestoreIdx := interfaces.FirestoreIndex{
			QueryScope: "COLLECTION",
			Fields: []interfaces.FirestoreIndexField{
				{FieldPath: "email", Order: "ASCENDING"},
				{FieldPath: "createdAt", Order: "DESCENDING"},
			},
		}

		modelIdx := usecase.ConvertFirestoreToModelIndex(firestoreIdx)
		gt.Equal(t, modelIdx.QueryScope, "COLLECTION")
		gt.Equal(t, len(modelIdx.Fields), 2)
		gt.Equal(t, modelIdx.Fields[0].Name, "email")
		gt.Equal(t, modelIdx.Fields[0].Order, "ASCENDING")
		gt.Equal(t, modelIdx.Fields[1].Name, "createdAt")
		gt.Equal(t, modelIdx.Fields[1].Order, "DESCENDING")
	})

	t.Run("Convert index with array config", func(t *testing.T) {
		firestoreIdx := interfaces.FirestoreIndex{
			QueryScope: "COLLECTION",
			Fields: []interfaces.FirestoreIndexField{
				{FieldPath: "tags", ArrayConfig: "CONTAINS"},
			},
		}

		modelIdx := usecase.ConvertFirestoreToModelIndex(firestoreIdx)
		gt.Equal(t, modelIdx.Fields[0].ArrayConfig, "CONTAINS")
		gt.Equal(t, modelIdx.Fields[0].Order, "")
	})

	t.Run("Convert index with vector config", func(t *testing.T) {
		firestoreIdx := interfaces.FirestoreIndex{
			QueryScope: "COLLECTION",
			Fields: []interfaces.FirestoreIndexField{
				{FieldPath: "title", Order: "ASCENDING"},
				{
					FieldPath: "embedding",
					VectorConfig: &interfaces.FirestoreVectorConfig{
						Dimension: 1536,
					},
				},
			},
		}

		modelIdx := usecase.ConvertFirestoreToModelIndex(firestoreIdx)
		gt.Equal(t, modelIdx.Fields[1].Name, "embedding")
		gt.NotEqual(t, modelIdx.Fields[1].VectorConfig, nil)
		gt.Equal(t, modelIdx.Fields[1].VectorConfig.Dimension, 1536)
		// Vector fields get ASCENDING order for convenience
		gt.Equal(t, modelIdx.Fields[1].Order, "ASCENDING")
	})
}
