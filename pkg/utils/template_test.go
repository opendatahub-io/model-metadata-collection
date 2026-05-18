package utils

import (
	"strings"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestRenderVLLMConfigSection_NoConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *types.VLLMRecommendedConfig
	}{
		{name: "nil config", config: nil},
		{name: "empty presets", config: &types.VLLMRecommendedConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderVLLMConfigSection(tt.config)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if got != "" {
				t.Errorf("Expected empty string, got: %q", got)
			}
		})
	}
}

func TestRenderVLLMConfigSection_FullConfig(t *testing.T) {
	config := &types.VLLMRecommendedConfig{
		Model: types.VLLMModelRef{Name: "RedHatAI/Llama-3.3-70B-Instruct-FP8-dynamic"},
		Presets: []types.VLLMPreset{
			{
				Mode: "online-serving",
				Optimizations: []types.VLLMOptimization{
					{
						Optimization: "low-latency",
						Hardware:     "H200",
						Description:  "Low latency for online serving",
						CLIArgs: []string{
							"--tensor-parallel-size 1",
							"--max-num-batched-tokens 1024",
							"--gpu-memory-utilization 0.92",
							"--kv-cache-dtype fp8",
						},
						EnvVars: []string{
							"NVIDIA_VISIBLE_DEVICES=1",
							`VLLM_ATTENTION_BACKEND="FLASH_ATTN"`,
						},
						Constraints: []types.VLLMConstraint{
							{Name: "TTFT", Value: "10ms", Operator: "<="},
						},
						Recommendations: []string{
							"This is the default configuration for low latency online serving.",
						},
					},
				},
			},
			{
				Mode: "offline-serving",
				Optimizations: []types.VLLMOptimization{
					{
						Optimization: "high-throughput",
						Hardware:     "H200",
						Description:  "High throughput for offline serving",
						CLIArgs: []string{
							"--tensor-parallel-size 1",
							"--max-num-batched-tokens 65536",
						},
					},
				},
			},
		},
	}

	got, err := RenderVLLMConfigSection(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"## vLLM Recommended Configurations",
		"### Online Serving",
		"#### Low Latency",
		"- Hardware: **H200**",
		"- Description: Low latency for online serving",
		"--tensor-parallel-size 1",
		"--max-num-batched-tokens 1024",
		"--gpu-memory-utilization 0.92",
		"--kv-cache-dtype fp8",
		"NVIDIA_VISIBLE_DEVICES=1",
		`VLLM_ATTENTION_BACKEND="FLASH_ATTN"`,
		"TTFT less than or equal to 10ms",
		"- **Recommendations**:",
		"This is the default configuration for low latency online serving.",
		"### Offline Serving",
		"#### High Throughput",
		"High throughput for offline serving",
		"--max-num-batched-tokens 65536",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(got, phrase) {
			t.Errorf("Output missing expected phrase: %q\nFull output:\n%s", phrase, got)
		}
	}

	// Verify bash code blocks present
	if strings.Count(got, "```bash") < 2 {
		t.Errorf("Expected at least 2 bash code blocks, got %d", strings.Count(got, "```bash"))
	}
}

func TestRenderVLLMConfigSection_MinimalConfig(t *testing.T) {
	// Minimal valid config: only required fields (hardware + cli-args)
	config := &types.VLLMRecommendedConfig{
		Model: types.VLLMModelRef{Name: "RedHatAI/gpt-oss-120b"},
		Presets: []types.VLLMPreset{
			{
				Mode: "online-serving",
				Optimizations: []types.VLLMOptimization{
					{
						Optimization: "low-latency",
						Hardware:     "H200",
						CLIArgs:      []string{"--tensor-parallel-size 1"},
					},
				},
			},
		},
	}

	got, err := RenderVLLMConfigSection(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(got, "## vLLM Recommended Configurations") {
		t.Error("Missing title")
	}
	if !strings.Contains(got, "### Online Serving") {
		t.Error("Missing mode title")
	}
	if !strings.Contains(got, "- Hardware: **H200**") {
		t.Error("Missing hardware")
	}
	if !strings.Contains(got, "--tensor-parallel-size 1") {
		t.Error("Missing CLI args")
	}

	// Should not have optional sections
	if strings.Contains(got, "Env Vars") {
		t.Error("Should not have Env Vars section with no env vars")
	}
	if strings.Contains(got, "Constraints") {
		t.Error("Should not have Constraints section with no constraints")
	}
	if strings.Contains(got, "Recommendations") {
		t.Error("Should not have Recommendations section with no recommendations")
	}
	if strings.Contains(got, "Description") {
		t.Error("Should not have Description section with no description")
	}
}

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"online-serving", "Online Serving"},
		{"offline-serving", "Offline Serving"},
		{"low-latency", "Low Latency"},
		{"high-throughput", "High Throughput"},
		{"single", "Single"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := titleCase(tt.input)
			if got != tt.expected {
				t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
