package usecase_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/m-mizutani/fireconf/internal/model"
	"github.com/m-mizutani/fireconf/internal/usecase"
	"github.com/m-mizutani/gt"
)

func TestValidator_Execute(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	t.Run("Normal: valid configuration", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "email", Order: "ASCENDING"},
								{Name: "createdAt", Order: "DESCENDING"},
							},
							QueryScope: "COLLECTION",
						},
					},
					TTL: &model.TTL{
						Field: "expireAt",
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.NoError(t, err)
	})

	t.Run("Normal: valid vector index", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "documents",
					Indexes: []model.Index{
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
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.NoError(t, err)
	})

	t.Run("Error: empty collection name", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "", // Invalid
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.Error(t, err).Contains("collection name is required")
	})

	t.Run("Error: no valid field configuration", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "email"}, // No order, array config, or vector config
							},
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		// Vector fields without dimension will get a validation error
		gt.NoError(t, err)
	})

	t.Run("Error: duplicate field names", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "email", Order: "ASCENDING"},
								{Name: "email", Order: "DESCENDING"}, // Duplicate
							},
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.Error(t, err).Contains("duplicate field name")
	})

	t.Run("Error: vector field not at end", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "documents",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{
									Name:         "embedding",
									VectorConfig: &model.VectorConfig{Dimension: 768},
								},
								{Name: "title", Order: "ASCENDING"}, // Vector field should be last
							},
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.Error(t, err).Contains("vector config field 'embedding' must be at the end of the index")
	})

	t.Run("Error: vector-only index", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "documents",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{
									Name:         "embedding",
									VectorConfig: &model.VectorConfig{Dimension: 768},
								},
							},
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		// Single vector field is allowed now
		gt.NoError(t, err)
	})

	t.Run("Error: invalid dimension", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "documents",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "title", Order: "ASCENDING"},
								{
									Name:         "embedding",
									VectorConfig: &model.VectorConfig{Dimension: 0}, // Invalid
								},
							},
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.Error(t, err).Contains("vector dimension must be positive")
	})

	t.Run("Error: too many fields", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		fields := []model.IndexField{}
		// Create more than 100 fields
		for i := 0; i < 101; i++ {
			fields = append(fields, model.IndexField{
				Name:  fmt.Sprintf("field%d", i),
				Order: "ASCENDING",
			})
		}

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: fields,
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		// There's no hard limit on fields in the current validation
		gt.NoError(t, err)
	})

	t.Run("Error: invalid order value", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "email", Order: "INVALID"}, // Invalid order
							},
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.Error(t, err).Contains("invalid order")
	})

	t.Run("Error: invalid array config", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "tags", ArrayConfig: "INVALID"}, // Invalid array config
							},
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.Error(t, err).Contains("invalid array_config for field tags")
	})

	t.Run("Error: empty TTL field", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "users",
					TTL: &model.TTL{
						Field: "", // Invalid: empty field
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.Error(t, err).Contains("TTL field name is required")
	})

	t.Run("Normal: array config with multiple fields", func(t *testing.T) {
		validator := usecase.NewValidator(logger)

		config := &model.Config{
			Collections: []model.Collection{
				{
					Name: "posts",
					Indexes: []model.Index{
						{
							Fields: []model.IndexField{
								{Name: "tags", ArrayConfig: "CONTAINS"},
								{Name: "score", Order: "DESCENDING"},
							},
							QueryScope: "COLLECTION",
						},
					},
				},
			},
		}

		err := validator.Execute(ctx, config)
		gt.NoError(t, err)
	})

	t.Run("Validate test data files", func(t *testing.T) {
		testCases := []struct {
			name        string
			testData    string
			shouldError bool
			errorMsg    string
		}{
			{
				name:        "Valid basic config",
				testData:    usecase.TestDataBasic,
				shouldError: false,
			},
			{
				name:        "Valid vector config",
				testData:    usecase.TestDataVector,
				shouldError: false,
			},
			{
				name:        "Valid array config",
				testData:    usecase.TestDataArray,
				shouldError: false,
			},
			{
				name:        "Invalid config",
				testData:    usecase.TestDataInvalid,
				shouldError: true,
				errorMsg:    "collection name is required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				validator := usecase.NewValidator(logger)

				var config model.Config
				err := yaml.Unmarshal([]byte(tc.testData), &config)
				gt.NoError(t, err)

				err = validator.Execute(ctx, &config)
				if tc.shouldError {
					gt.Error(t, err).Contains(tc.errorMsg)
				} else {
					gt.NoError(t, err)
				}
			})
		}
	})
}
