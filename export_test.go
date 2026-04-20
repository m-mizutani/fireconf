package fireconf

// NewDiffTestClient constructs a Client suitable for testing DiffConfigs
// without establishing a Firestore connection. DiffConfigs is a pure
// computation over the provided desired/current configs, so the Firestore
// client field is intentionally left nil.
func NewDiffTestClient(desired *Config) *Client {
	return &Client{config: desired}
}
