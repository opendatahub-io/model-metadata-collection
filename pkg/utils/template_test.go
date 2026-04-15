package utils

import (
	"strings"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestRenderToolCallingSection_NoToolCalling(t *testing.T) {
	tests := []struct {
		name   string
		config *types.ToolCallingConfig
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name:   "empty config",
			config: &types.ToolCallingConfig{},
		},
		{
			name: "only chat template path (not enough for tool calling)",
			config: &types.ToolCallingConfig{
				ChatTemplatePath: "examples/template.jinja",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderToolCallingSection(tt.config, "test-model")
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if got != "" {
				t.Errorf("Expected empty string for no tool calling, got: %q", got)
			}
		})
	}
}

func TestRenderToolCallingSection_FullConfig(t *testing.T) {
	config := &types.ToolCallingConfig{
		Supported: true,
		RequiredCLIArgs: []string{
			"--config_format mistral",
			"--load_format mistral",
			"--tokenizer_mode mistral",
		},
		ChatTemplateFile: "chat_template.jinja",
		ChatTemplatePath: "examples/chat_template.jinja",
		ToolCallParser:   "mistral",
	}

	got, err := RenderToolCallingSection(config, "RedHatAI/Ministral-3-14B-Instruct-2512")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify required content
	expectedPhrases := []string{
		"vLLM Deployment with Tool Calling",
		"RedHatAI/Ministral-3-14B-Instruct-2512",
		"--config_format mistral",
		"--load_format mistral",
		"--tokenizer_mode mistral",
		"--tool-call-parser mistral",
		"opt/app-root/template/chat_template.jinja",
		"--enable-auto-tool-choice",
		"Tool Call Parser",
		"Chat Template",
		"chat_template.jinja",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(got, phrase) {
			t.Errorf("Output missing expected phrase: %q", phrase)
		}
	}

	// Verify code block format
	if !strings.Contains(got, "```bash") {
		t.Error("Output missing bash code block")
	}

	// Verify path conversion happened
	if strings.Contains(got, "examples/") {
		t.Error("Path was not converted from examples/ to opt/app-root/template/")
	}
}

func TestRenderToolCallingSection_MinimalConfig(t *testing.T) {
	config := &types.ToolCallingConfig{
		Supported:      true,
		ToolCallParser: "mistral",
	}

	got, err := RenderToolCallingSection(config, "test-model")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have basic structure
	if !strings.Contains(got, "vLLM Deployment") {
		t.Error("Missing deployment section title")
	}

	if !strings.Contains(got, "mistral") {
		t.Error("Missing tool call parser value")
	}

	if !strings.Contains(got, "test-model") {
		t.Error("Missing model name")
	}

	// Should not have chat template section since none specified
	if strings.Contains(got, "Chat Template") {
		t.Error("Should not have chat template section")
	}
}

func TestRenderToolCallingSection_PathConversion(t *testing.T) {
	tests := []struct {
		name              string
		chatTemplatePath  string
		expectedInOutput  string
		notExpectedOutput string
	}{
		{
			name:              "examples path",
			chatTemplatePath:  "examples/template.jinja",
			expectedInOutput:  "opt/app-root/template/template.jinja",
			notExpectedOutput: "examples/",
		},
		{
			name:              "already converted path",
			chatTemplatePath:  "opt/app-root/template/template.jinja",
			expectedInOutput:  "opt/app-root/template/template.jinja",
			notExpectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.ToolCallingConfig{
				Supported:        true,
				ChatTemplatePath: tt.chatTemplatePath,
				ToolCallParser:   "test",
			}

			got, err := RenderToolCallingSection(config, "test-model")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !strings.Contains(got, tt.expectedInOutput) {
				t.Errorf("Expected output to contain %q", tt.expectedInOutput)
			}

			if tt.notExpectedOutput != "" && strings.Contains(got, tt.notExpectedOutput) {
				t.Errorf("Output should not contain %q", tt.notExpectedOutput)
			}
		})
	}
}

func TestRenderToolCallingSection_MistralExample(t *testing.T) {
	// Real Ministral-3-14B configuration
	config := &types.ToolCallingConfig{
		Supported: true,
		RequiredCLIArgs: []string{
			"--config_format mistral",
			"--load_format mistral",
			"--tokenizer_mode mistral",
		},
		ChatTemplateFile: "None",
		ChatTemplatePath: "None",
		ToolCallParser:   "mistral",
	}

	got, err := RenderToolCallingSection(config, "RedHatAI/Ministral-3-14B-Instruct-2512")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify all CLI args present
	cliArgs := []string{
		"--config_format mistral",
		"--load_format mistral",
		"--tokenizer_mode mistral",
		"--tool-call-parser mistral",
	}

	for _, arg := range cliArgs {
		if !strings.Contains(got, arg) {
			t.Errorf("Missing CLI arg: %q", arg)
		}
	}

	// Verify model name
	if !strings.Contains(got, "RedHatAI/Ministral-3-14B-Instruct-2512") {
		t.Error("Missing model name in output")
	}

	// Should not have chat template section since path is "None"
	if strings.Contains(got, "Chat Template") && strings.Contains(got, "Template path:") {
		t.Error("Should not have chat template path section when path is 'None'")
	}

	// Should have tool call parser section
	if !strings.Contains(got, "Tool Call Parser") {
		t.Error("Missing Tool Call Parser section")
	}
}

func TestRenderToolCallingSection_OnlyParser(t *testing.T) {
	// Model with only parser specified (no CLI args)
	config := &types.ToolCallingConfig{
		ToolCallParser: "mistral",
	}

	got, err := RenderToolCallingSection(config, "test-model")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should use the else branch (no CLI args)
	if !strings.Contains(got, "vllm serve test-model") {
		t.Error("Missing vllm serve command")
	}

	if !strings.Contains(got, "--tool-call-parser mistral") {
		t.Error("Missing tool-call-parser flag")
	}

	if !strings.Contains(got, "--enable-auto-tool-choice") {
		t.Error("Missing enable-auto-tool-choice flag")
	}
}

func TestRenderToolCallingSection_SplitPathAndFilename(t *testing.T) {
	// Real Llama 3.1 configuration with split path and filename
	config := &types.ToolCallingConfig{
		Supported:        true,
		RequiredCLIArgs:  []string{}, // Empty array
		ChatTemplatePath: "examples/",
		ChatTemplateFile: "tool_chat_template_llama3.1_json.jinja",
		ToolCallParser:   "llama3_json",
	}

	got, err := RenderToolCallingSection(config, "RedHatAI/Meta-Llama-3.1-8B-Instruct-FP8-dynamic")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify the path was properly combined and converted
	expectedPath := "opt/app-root/template/tool_chat_template_llama3.1_json.jinja"
	if !strings.Contains(got, expectedPath) {
		t.Errorf("Output missing expected combined and converted path: %q", expectedPath)
	}

	// Should NOT contain the original examples/ path
	if strings.Contains(got, "examples/") {
		t.Error("Output should not contain 'examples/' - path should be converted")
	}

	// Verify tool call parser
	if !strings.Contains(got, "llama3_json") {
		t.Error("Missing llama3_json parser")
	}

	// Verify chat template file name is shown
	if !strings.Contains(got, "tool_chat_template_llama3.1_json.jinja") {
		t.Error("Missing chat template filename")
	}

	// Should have Chat Template section since we have both path and filename
	if !strings.Contains(got, "Chat Template") {
		t.Error("Missing Chat Template section")
	}
}

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

func TestRenderServingRuntimeOverrideSection_NoConfig(t *testing.T) {
	got, err := RenderServingRuntimeOverrideSection(nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if got != "" {
		t.Errorf("Expected empty string for nil config, got: %q", got)
	}
}

func TestRenderServingRuntimeOverrideSection_FullConfig(t *testing.T) {
	config := &types.ServingRuntimeOverrideConfig{
		PreviewImage: "registry.redhat.io/rhaiis-preview/vllm-cuda-rhel9:mistral-4-small",
		Reason:       "Requires transformers v5",
		RuntimeName:  "rhaiis-vllm-runtime-mistral4",
		DisplayName:  "RHAIIS vLLM Preview Runtime - Mistral 4",
	}

	got, err := RenderServingRuntimeOverrideSection(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"## Deploying with RHAIIS Preview Runtime",
		"Custom Serving Runtime",
		"registry.redhat.io",
		"Pull Secret",
		"NVIDIA GPU Operator",
		"OpenShift AI Dashboard",
		"Settings > Model resources and operations > Serving runtimes",
		"Add serving runtime",
		"REST or gRPC API",
		"Generative AI model",
		"rhaiis-vllm-runtime-mistral4",
		"RHAIIS vLLM Preview Runtime - Mistral 4",
		"registry.redhat.io/rhaiis-preview/vllm-cuda-rhel9:mistral-4-small",
		"vllm.entrypoints.openai.api_server",
		"--model=/mnt/models",
		"--port=8080",
		"HF_HOME",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(got, phrase) {
			t.Errorf("Output missing expected phrase: %q", phrase)
		}
	}

	// Verify YAML code block
	if !strings.Contains(got, "```yaml") {
		t.Error("Output missing YAML code block")
	}

	// Verify ServingRuntime kind
	if !strings.Contains(got, "kind: ServingRuntime") {
		t.Error("Output missing ServingRuntime kind")
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
