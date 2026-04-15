package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

// LoadServingRuntimeOverrideConfig reads a single serving runtime override
// configuration file. Returns nil (not error) when configPath is empty.
func LoadServingRuntimeOverrideConfig(configPath string) (*types.ServingRuntimeOverrideConfig, error) {
	if configPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read serving runtime override config %s: %w", configPath, err)
	}

	var cfg types.ServingRuntimeOverrideConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse serving runtime override config %s: %w", configPath, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid serving runtime override config %s: %w", configPath, err)
	}

	return &cfg, nil
}
