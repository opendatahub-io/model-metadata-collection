package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestLoadModelsFromYAML(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		expected    []string
		expectError bool
	}{
		{
			name: "valid models config",
			fileContent: `models:
  - type: "oci"
    uri: "registry.redhat.io/rhelai1/modelcar-granite-3-1-8b-base:1.0"
    labels: ["validated"]
  - type: "oci"
    uri: "registry.redhat.io/rhelai1/modelcar-llama-3-2-1b-instruct:1.0"
    labels: ["validated"]
  - type: "hf"
    uri: "https://huggingface.co/microsoft/Phi-3.5-mini-instruct"
    labels: ["validated", "featured"]`,
			expected: []string{
				"registry.redhat.io/rhelai1/modelcar-granite-3-1-8b-base:1.0",
				"registry.redhat.io/rhelai1/modelcar-llama-3-2-1b-instruct:1.0",
				"https://huggingface.co/microsoft/Phi-3.5-mini-instruct",
			},
			expectError: false,
		},
		{
			name:        "empty models config",
			fileContent: `models: []`,
			expected:    []string{},
			expectError: false,
		},
		{
			name: "single model",
			fileContent: `models:
  - type: "oci"
    uri: "registry.redhat.io/rhelai1/modelcar-granite-3-1-8b-base:1.0"
    labels: ["validated"]`,
			expected: []string{
				"registry.redhat.io/rhelai1/modelcar-granite-3-1-8b-base:1.0",
			},
			expectError: false,
		},
		{
			name:        "invalid yaml",
			fileContent: `invalid: yaml: content: [`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "missing models field",
			fileContent: `other_field: value`,
			expected:    nil,
			expectError: false, // Should return empty slice
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test-config.yaml")

			err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test the function
			result, err := LoadModelsFromYAML(tmpFile)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d models, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected model[%d] = %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestLoadModelsFromYAML_FileNotFound(t *testing.T) {
	_, err := LoadModelsFromYAML("nonexistent-file.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLoadModelsFromVersionIndex(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		expected    []string
		expectError bool
	}{
		{
			name: "valid version index",
			fileContent: `version: "v1.0"
models:
  - name: "RedHatAI/granite-3.1-8b-base"
    url: "https://huggingface.co/RedHatAI/granite-3.1-8b-base"
    readme_path: "/RedHatAI/granite-3.1-8b-base/README.md"
  - name: "RedHatAI/Llama-3.2-1B-Instruct"
    url: "https://huggingface.co/RedHatAI/Llama-3.2-1B-Instruct"
    readme_path: "/RedHatAI/Llama-3.2-1B-Instruct/README.md"`,
			expected: []string{
				"registry.redhat.io/rhelai1/modelcar-redhatai-granite-3.1-8b-base",
				"registry.redhat.io/rhelai1/modelcar-redhatai-llama-3.2-1b-instruct",
			},
			expectError: false,
		},
		{
			name: "model name with slashes",
			fileContent: `version: "v1.0"
models:
  - name: "microsoft/Phi-3.5-mini-instruct"
    url: "https://huggingface.co/microsoft/Phi-3.5-mini-instruct"
    readme_path: "/microsoft/Phi-3.5-mini-instruct/README.md"`,
			expected: []string{
				"registry.redhat.io/rhelai1/modelcar-microsoft-phi-3.5-mini-instruct",
			},
			expectError: false,
		},
		{
			name: "empty models list",
			fileContent: `version: "v1.0"
models: []`,
			expected:    []string{},
			expectError: false,
		},
		{
			name:        "invalid yaml",
			fileContent: `invalid: yaml: content: [`,
			expected:    nil,
			expectError: true,
		},
		{
			name: "missing version field",
			fileContent: `models:
  - name: "test/model"
    url: "https://huggingface.co/test/model"`,
			expected: []string{
				"registry.redhat.io/rhelai1/modelcar-test-model",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test-version-index.yaml")

			err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test the function
			result, err := LoadModelsFromVersionIndex(tmpFile)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d models, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected model[%d] = %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestLoadModelsFromVersionIndex_FileNotFound(t *testing.T) {
	_, err := LoadModelsFromVersionIndex("nonexistent-file.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// Test helper to verify the conversion logic for model names
func TestModelNameConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "RedHatAI/granite-3.1-8b-base",
			expected: "registry.redhat.io/rhelai1/modelcar-redhatai-granite-3.1-8b-base",
		},
		{
			input:    "microsoft/Phi-3.5-mini-instruct",
			expected: "registry.redhat.io/rhelai1/modelcar-microsoft-phi-3.5-mini-instruct",
		},
		{
			input:    "simple-model",
			expected: "registry.redhat.io/rhelai1/modelcar-simple-model",
		},
		{
			input:    "Complex/Model/With/Many/Slashes",
			expected: "registry.redhat.io/rhelai1/modelcar-complex-model-with-many-slashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Create a temporary version index file with just this model
			versionIndex := types.VersionIndex{
				Version: "v1.0",
				Models: []types.ModelIndex{
					{
						Name:       tt.input,
						URL:        "https://huggingface.co/" + tt.input,
						ReadmePath: "/" + tt.input + "/README.md",
					},
				},
			}

			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test-conversion.yaml")

			data, err := yaml.Marshal(versionIndex)
			if err != nil {
				t.Fatalf("Failed to marshal test data: %v", err)
			}

			err = os.WriteFile(tmpFile, data, 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test the conversion
			result, err := LoadModelsFromVersionIndex(tmpFile)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != 1 {
				t.Fatalf("Expected 1 result, got %d", len(result))
			}

			if result[0] != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result[0])
			}
		})
	}
}
