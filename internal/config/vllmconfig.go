package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

// VLLMConfigIndex maps model names (exact match) to their vLLM configs
type VLLMConfigIndex struct {
	configs map[string]*types.VLLMRecommendedConfig
}

// LoadVLLMConfigs reads all YAML files from the given directory and indexes by model.name.
// Returns an empty index (not an error) when configDir is empty or has no YAML files.
func LoadVLLMConfigs(configDir string) (*VLLMConfigIndex, error) {
	index := &VLLMConfigIndex{
		configs: make(map[string]*types.VLLMRecommendedConfig),
	}

	if configDir == "" {
		return index, nil
	}

	files, err := filepath.Glob(filepath.Join(configDir, "*.yaml"))
	if err != nil {
		return index, fmt.Errorf("failed to glob vllm config dir: %w", err)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Warning: failed to read vllm config %s: %v", file, err)
			continue
		}

		var cfg types.VLLMRecommendedConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Printf("Warning: failed to parse vllm config %s: %v", file, err)
			continue
		}

		if err := cfg.Validate(); err != nil {
			log.Printf("Warning: invalid vllm config %s: %v", file, err)
			continue
		}

		if _, exists := index.configs[cfg.Model.Name]; exists {
			log.Printf("Warning: duplicate vLLM config for model %s (from %s), overwriting previous", cfg.Model.Name, filepath.Base(file))
		}
		index.configs[cfg.Model.Name] = &cfg
		log.Printf("Loaded vLLM config for model: %s (from %s)", cfg.Model.Name, filepath.Base(file))
	}

	return index, nil
}

// GetConfig returns the vLLM config for the given model name (exact match)
func (idx *VLLMConfigIndex) GetConfig(modelName string) *types.VLLMRecommendedConfig {
	if idx == nil {
		return nil
	}
	return idx.configs[modelName]
}

// ModelCount returns the number of loaded configs
func (idx *VLLMConfigIndex) ModelCount() int {
	if idx == nil {
		return 0
	}
	return len(idx.configs)
}
