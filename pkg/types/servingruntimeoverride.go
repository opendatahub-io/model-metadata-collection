package types

import "fmt"

// ServingRuntimeOverrideConfig holds configuration for models that require
// a custom (preview) serving runtime image instead of the default GA image.
type ServingRuntimeOverrideConfig struct {
	PreviewImage string `yaml:"preview_image"`
	Reason       string `yaml:"reason,omitempty"` // documentation-only: why this override is needed
	RuntimeName  string `yaml:"runtime_name"`
	DisplayName  string `yaml:"display_name"`
	Note         string `yaml:"note,omitempty"`
}

// Validate checks that all required fields are present.
func (c *ServingRuntimeOverrideConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.PreviewImage == "" {
		return fmt.Errorf("preview_image is required")
	}
	if c.RuntimeName == "" {
		return fmt.Errorf("runtime_name is required")
	}
	if c.DisplayName == "" {
		return fmt.Errorf("display_name is required")
	}
	return nil
}
