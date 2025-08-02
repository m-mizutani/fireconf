package model

import "fmt"

// TTL represents TTL configuration
type TTL struct {
	Field string `yaml:"field"`
}

// Validate validates the TTL configuration
func (t *TTL) Validate() error {
	if t.Field == "" {
		return fmt.Errorf("TTL field name is required")
	}
	return nil
}
