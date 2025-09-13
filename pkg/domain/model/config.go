package model

// Config represents the YAML configuration
type Config struct {
	Collections []Collection `yaml:"collections"`
}

// Collection represents a Firestore collection configuration
type Collection struct {
	Name    string  `yaml:"name"`
	Indexes []Index `yaml:"indexes"`
	TTL     *TTL    `yaml:"ttl,omitempty"`
}

// Validate validates the collection configuration
func (c *Collection) Validate() error {
	if c.Name == "" {
		return &ConfigError{Field: "name", Message: "collection name is required"}
	}

	for i, idx := range c.Indexes {
		if err := idx.Validate(); err != nil {
			return &ConfigError{
				Field:   "indexes[" + string(rune(i)) + "]",
				Message: err.Error(),
			}
		}
	}

	if c.TTL != nil {
		if err := c.TTL.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "config error in " + e.Field + ": " + e.Message
}
