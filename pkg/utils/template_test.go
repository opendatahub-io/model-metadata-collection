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
