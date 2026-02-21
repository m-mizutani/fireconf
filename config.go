package fireconf

import (
	"os"

	"github.com/goccy/go-yaml"
	"github.com/m-mizutani/fireconf/internal/model"
	"github.com/m-mizutani/goerr/v2"
)

// Config represents Firestore configuration
type Config struct {
	Collections []Collection `yaml:"collections"`
}

// Collection represents a collection configuration
type Collection struct {
	Name    string  `yaml:"name"`
	Indexes []Index `yaml:"indexes"`
	TTL     *TTL    `yaml:"ttl,omitempty"`
}

// Index represents a composite index
type Index struct {
	Fields     []IndexField `yaml:"fields"`
	QueryScope QueryScope   `yaml:"queryScope,omitempty"`
}

// IndexField represents a field in an index
type IndexField struct {
	Path   string        `yaml:"path"`
	Order  Order         `yaml:"order,omitempty"`
	Array  ArrayConfig   `yaml:"arrayConfig,omitempty"`
	Vector *VectorConfig `yaml:"vectorConfig,omitempty"`
}

// VectorConfig represents vector configuration
type VectorConfig struct {
	Dimension int `yaml:"dimension"`
}

// TTL represents TTL configuration
type TTL struct {
	Field string `yaml:"field"`
}

// Order represents field ordering
type Order string

const (
	OrderAscending  Order = "ASCENDING"
	OrderDescending Order = "DESCENDING"
)

// ArrayConfig represents array configuration
type ArrayConfig string

const (
	ArrayConfigContains ArrayConfig = "CONTAINS"
)

// QueryScope represents query scope
type QueryScope string

const (
	QueryScopeCollection      QueryScope = "COLLECTION"
	QueryScopeCollectionGroup QueryScope = "COLLECTION_GROUP"
)

// LoadConfigFromYAML loads configuration from a YAML file
func LoadConfigFromYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 - path is provided by user as CLI argument
	if err != nil {
		return nil, goerr.Wrap(err, "failed to read config file")
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, goerr.Wrap(err, "failed to parse YAML")
	}

	return &config, nil
}

// SaveToYAML saves configuration to a YAML file
func (c *Config) SaveToYAML(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return goerr.Wrap(err, "failed to marshal config to YAML")
	}

	// #nosec G306 - YAML config files should be readable by others
	if err := os.WriteFile(path, data, 0644); err != nil {
		return goerr.Wrap(err, "failed to write config file")
	}

	return nil
}

// Validate validates configuration
func (c *Config) Validate() error {
	// Convert to internal model and validate
	internalConfig := convertToInternalConfig(c)

	for _, col := range internalConfig.Collections {
		if err := col.Validate(); err != nil {
			return &ValidationError{
				Field:   "collection." + col.Name,
				Message: err.Error(),
			}
		}
	}

	return nil
}

// convertFromInternalConfig converts internal model to public API
func convertFromInternalConfig(internal *model.Config) *Config {
	config := &Config{
		Collections: make([]Collection, len(internal.Collections)),
	}

	for i, col := range internal.Collections {
		collection := Collection{
			Name:    col.Name,
			Indexes: make([]Index, len(col.Indexes)),
		}

		for j, idx := range col.Indexes {
			index := Index{
				Fields: make([]IndexField, len(idx.Fields)),
			}

			if idx.QueryScope != "" {
				index.QueryScope = QueryScope(idx.QueryScope)
			}

			for k, field := range idx.Fields {
				indexField := IndexField{
					Path: field.Name,
				}

				if field.Order != "" {
					indexField.Order = Order(field.Order)
				}

				if field.ArrayConfig != "" {
					indexField.Array = ArrayConfig(field.ArrayConfig)
				}

				if field.VectorConfig != nil {
					indexField.Vector = &VectorConfig{
						Dimension: field.VectorConfig.Dimension,
					}
				}

				index.Fields[k] = indexField
			}

			collection.Indexes[j] = index
		}

		if col.TTL != nil {
			collection.TTL = &TTL{
				Field: col.TTL.Field,
			}
		}

		config.Collections[i] = collection
	}

	return config
}

// convertToInternalConfig converts public API to internal model
func convertToInternalConfig(config *Config) *model.Config {
	internal := &model.Config{
		Collections: make([]model.Collection, len(config.Collections)),
	}

	for i, col := range config.Collections {
		collection := model.Collection{
			Name:    col.Name,
			Indexes: make([]model.Index, len(col.Indexes)),
		}

		for j, idx := range col.Indexes {
			index := model.Index{
				Fields: make([]model.IndexField, len(idx.Fields)),
			}

			if idx.QueryScope != "" {
				index.QueryScope = string(idx.QueryScope)
			}

			for k, field := range idx.Fields {
				indexField := model.IndexField{
					Name: field.Path,
				}

				if field.Order != "" {
					indexField.Order = string(field.Order)
				}

				if field.Array != "" {
					indexField.ArrayConfig = string(field.Array)
				}

				if field.Vector != nil {
					indexField.VectorConfig = &model.VectorConfig{
						Dimension: field.Vector.Dimension,
					}
				}

				index.Fields[k] = indexField
			}

			collection.Indexes[j] = index
		}

		if col.TTL != nil {
			collection.TTL = &model.TTL{
				Field: col.TTL.Field,
			}
		}

		internal.Collections[i] = collection
	}

	return internal
}
