package types

import (
	"testing"
)

func TestVLLMRecommendedConfig_HasPresets(t *testing.T) {
	tests := []struct {
		name     string
		config   *VLLMRecommendedConfig
		expected bool
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: false,
		},
		{
			name:     "empty presets",
			config:   &VLLMRecommendedConfig{},
			expected: false,
		},
		{
			name: "with presets",
			config: &VLLMRecommendedConfig{
				Presets: []VLLMPreset{
					{Mode: "online-serving"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.HasPresets()
			if got != tt.expected {
				t.Errorf("HasPresets() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVLLMRecommendedConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *VLLMRecommendedConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
		{
			name: "valid config",
			config: &VLLMRecommendedConfig{
				Model: VLLMModelRef{Name: "RedHatAI/gpt-oss-120b"},
				Presets: []VLLMPreset{
					{
						Mode: "online-serving",
						Optimizations: []VLLMOptimization{
							{
								Optimization: "low-latency",
								Hardware:     "H200",
								Description:  "Low latency for online serving",
								CLIArgs:      []string{"--tensor-parallel-size 1"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing model name",
			config: &VLLMRecommendedConfig{
				Presets: []VLLMPreset{
					{Mode: "online-serving", Optimizations: []VLLMOptimization{{Optimization: "low-latency", Hardware: "H200"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "missing mode",
			config: &VLLMRecommendedConfig{
				Model: VLLMModelRef{Name: "RedHatAI/gpt-oss-120b"},
				Presets: []VLLMPreset{
					{
						Optimizations: []VLLMOptimization{
							{Optimization: "low-latency", Hardware: "H200"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing optimization name",
			config: &VLLMRecommendedConfig{
				Model: VLLMModelRef{Name: "RedHatAI/gpt-oss-120b"},
				Presets: []VLLMPreset{
					{
						Mode: "online-serving",
						Optimizations: []VLLMOptimization{
							{Hardware: "H200"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing hardware",
			config: &VLLMRecommendedConfig{
				Model: VLLMModelRef{Name: "RedHatAI/gpt-oss-120b"},
				Presets: []VLLMPreset{
					{
						Mode: "online-serving",
						Optimizations: []VLLMOptimization{
							{Optimization: "low-latency"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing cli-args",
			config: &VLLMRecommendedConfig{
				Model: VLLMModelRef{Name: "RedHatAI/gpt-oss-120b"},
				Presets: []VLLMPreset{
					{
						Mode: "online-serving",
						Optimizations: []VLLMOptimization{
							{Optimization: "low-latency", Hardware: "H200"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "full config with all fields",
			config: &VLLMRecommendedConfig{
				Model: VLLMModelRef{Name: "RedHatAI/Llama-3.3-70B-Instruct-FP8-dynamic"},
				Presets: []VLLMPreset{
					{
						Mode: "online-serving",
						Optimizations: []VLLMOptimization{
							{
								Optimization: "low-latency",
								Hardware:     "H200",
								Description:  "Low latency for online serving",
								CLIArgs:      []string{"--tensor-parallel-size 1", "--max-num-batched-tokens 1024"},
								EnvVars:      []string{"NVIDIA_VISIBLE_DEVICES=1"},
								Constraints: []VLLMConstraint{
									{Name: "TTFT", Value: "10ms", Operator: "<="},
								},
								Recommendations: []string{"Use async scheduling for best results"},
							},
						},
					},
					{
						Mode: "offline-serving",
						Optimizations: []VLLMOptimization{
							{
								Optimization: "high-throughput",
								Hardware:     "H200",
								Description:  "High throughput for offline serving",
								CLIArgs:      []string{"--tensor-parallel-size 1"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVLLMConstraint_FormatConstraint(t *testing.T) {
	tests := []struct {
		name       string
		constraint VLLMConstraint
		expected   string
	}{
		{
			name:       "less than or equal",
			constraint: VLLMConstraint{Name: "TTFT", Value: "10ms", Operator: "<="},
			expected:   "TTFT less than or equal to 10ms",
		},
		{
			name:       "greater than or equal",
			constraint: VLLMConstraint{Name: "throughput", Value: "100rps", Operator: ">="},
			expected:   "throughput greater than or equal to 100rps",
		},
		{
			name:       "less than",
			constraint: VLLMConstraint{Name: "TTFT", Value: "5ms", Operator: "<"},
			expected:   "TTFT less than 5ms",
		},
		{
			name:       "greater than",
			constraint: VLLMConstraint{Name: "throughput", Value: "50rps", Operator: ">"},
			expected:   "throughput greater than 50rps",
		},
		{
			name:       "equal to",
			constraint: VLLMConstraint{Name: "batch_size", Value: "32", Operator: "=="},
			expected:   "batch_size equal to 32",
		},
		{
			name:       "unknown operator fallback",
			constraint: VLLMConstraint{Name: "metric", Value: "100", Operator: "~="},
			expected:   "metric ~= 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.constraint.FormatConstraint()
			if got != tt.expected {
				t.Errorf("FormatConstraint() = %q, want %q", got, tt.expected)
			}
		})
	}
}
