package model

import (
	"fmt"
	"strings"
)

// Index represents an index configuration
type Index struct {
	Fields     []IndexField `yaml:"fields"`
	QueryScope string       `yaml:"query_scope,omitempty"` // COLLECTION or COLLECTION_GROUP
}

// IndexField represents a field in an index
type IndexField struct {
	Name         string        `yaml:"name"`
	Order        string        `yaml:"order,omitempty"`        // ASCENDING or DESCENDING
	ArrayConfig  string        `yaml:"array_config,omitempty"` // CONTAINS
	VectorConfig *VectorConfig `yaml:"vector_config,omitempty"`
}

// VectorConfig represents vector search configuration
type VectorConfig struct {
	Dimension int `yaml:"dimension"`
}

// Validate validates the index configuration
func (i *Index) Validate() error {
	if len(i.Fields) == 0 {
		return fmt.Errorf("index must have at least one field")
	}

	// Validate query scope
	if i.QueryScope == "" {
		i.QueryScope = "COLLECTION" // default
	} else if i.QueryScope != "COLLECTION" && i.QueryScope != "COLLECTION_GROUP" {
		return fmt.Errorf("invalid queryScope: %s", i.QueryScope)
	}

	// Validate each field
	for _, field := range i.Fields {
		if err := field.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates the index field configuration
func (f *IndexField) Validate() error {
	if f.Name == "" {
		return fmt.Errorf("field name is required")
	}

	// Count how many field types are specified
	// Note: vector_config can coexist with order (order is ignored in that case)
	hasOrder := f.Order != ""
	hasArrayConfig := f.ArrayConfig != ""
	hasVectorConfig := f.VectorConfig != nil

	// Check for invalid combinations
	if hasArrayConfig && (hasOrder || hasVectorConfig) {
		return fmt.Errorf("field %s: array_config cannot be combined with order or vector_config", f.Name)
	}

	// If no type is specified, default to ASCENDING
	if !hasOrder && !hasArrayConfig && !hasVectorConfig {
		f.Order = "ASCENDING"
	}

	// Validate order
	if f.Order != "" && f.Order != "ASCENDING" && f.Order != "DESCENDING" {
		return fmt.Errorf("invalid order for field %s: %s", f.Name, f.Order)
	}

	// Validate array config
	if f.ArrayConfig != "" && f.ArrayConfig != "CONTAINS" {
		return fmt.Errorf("invalid array_config for field %s: %s", f.Name, f.ArrayConfig)
	}

	// Validate vector config
	if f.VectorConfig != nil {
		if f.VectorConfig.Dimension <= 0 {
			return fmt.Errorf("vector dimension must be positive for field %s", f.Name)
		}
	}

	return nil
}

// GetQueryScope returns normalized query scope
func (i *Index) GetQueryScope() string {
	if i.QueryScope == "" {
		return "COLLECTION"
	}
	return strings.ToUpper(i.QueryScope)
}

// IsComposite returns true if the index has multiple fields
func (i *Index) IsComposite() bool {
	return len(i.Fields) > 1
}
