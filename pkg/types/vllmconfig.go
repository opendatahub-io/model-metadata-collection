package types

import "fmt"

// VLLMRecommendedConfig represents the full vLLM recommended configuration for a model
type VLLMRecommendedConfig struct {
	Model   VLLMModelRef `yaml:"model"`
	Presets []VLLMPreset `yaml:"presets"`
}

// VLLMModelRef holds the model name used for exact-match lookup
type VLLMModelRef struct {
	Name string `yaml:"name"`
}

// VLLMPreset represents a serving mode preset (e.g., online-serving, offline-serving)
type VLLMPreset struct {
	Mode          string             `yaml:"mode"`
	Optimizations []VLLMOptimization `yaml:"optimizations"`
}

// VLLMOptimization represents an optimization configuration for specific hardware
type VLLMOptimization struct {
	Optimization    string           `yaml:"optimization"`
	Hardware        string           `yaml:"hardware"`
	Description     string           `yaml:"description,omitempty"`
	CLIArgs         []string         `yaml:"cli-args,omitempty"`
	EnvVars         []string         `yaml:"env-vars,omitempty"`
	Constraints     []VLLMConstraint `yaml:"constraints,omitempty"`
	Recommendations []string         `yaml:"recommendations,omitempty"`
}

// VLLMConstraint represents a performance constraint (e.g., TTFT <= 10ms)
type VLLMConstraint struct {
	Name     string `yaml:"name"`
	Value    string `yaml:"value"`
	Operator string `yaml:"operator"`
}

// HasPresets returns true if the config has any presets defined
func (vc *VLLMRecommendedConfig) HasPresets() bool {
	if vc == nil {
		return false
	}
	return len(vc.Presets) > 0
}

// Validate checks that the vLLM configuration is structurally valid.
// A nil config is considered valid (no-op) since callers use HasPresets() to
// guard rendering; Validate only applies to configs that will actually be used.
func (vc *VLLMRecommendedConfig) Validate() error {
	if vc == nil {
		return nil
	}
	if vc.Model.Name == "" {
		return fmt.Errorf("vllm config: model.name is required")
	}
	for i, preset := range vc.Presets {
		if preset.Mode == "" {
			return fmt.Errorf("vllm config: preset[%d].mode is required", i)
		}
		for j, opt := range preset.Optimizations {
			if opt.Optimization == "" {
				return fmt.Errorf("vllm config: preset[%d].optimizations[%d].optimization is required", i, j)
			}
			if opt.Hardware == "" {
				return fmt.Errorf("vllm config: preset[%d].optimizations[%d].hardware is required", i, j)
			}
			if len(opt.CLIArgs) == 0 {
				return fmt.Errorf("vllm config: preset[%d].optimizations[%d].cli-args is required", i, j)
			}
		}
	}
	return nil
}

// operatorText maps operators to human-readable text
var operatorText = map[string]string{
	"<=": "less than or equal to",
	">=": "greater than or equal to",
	"<":  "less than",
	">":  "greater than",
	"==": "equal to",
}

// FormatConstraint returns a human-readable string for a constraint
func (c VLLMConstraint) FormatConstraint() string {
	if text, ok := operatorText[c.Operator]; ok {
		return fmt.Sprintf("%s %s %s", c.Name, text, c.Value)
	}
	return fmt.Sprintf("%s %s %s", c.Name, c.Operator, c.Value)
}
