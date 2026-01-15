package huggingface

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestFetchCollections(t *testing.T) {
	// Test basic function structure - network calls will likely fail in test environment
	// but we can test that the function returns an appropriate error
	_, err := FetchCollections()
	if err != nil {
		// Expected to fail due to network unavailability in tests
		// Accept various types of network/API errors
		if !strings.Contains(err.Error(), "failed to fetch collections") &&
			!strings.Contains(err.Error(), "failed to parse collections") &&
			!strings.Contains(err.Error(), "json:") {
			t.Errorf("Expected a collections-related error, got: %v", err)
		}
	}
}

func TestFetchCollectionDetails(t *testing.T) {
	// Test with a test collection ID
	_, err := FetchCollectionDetails("test-collection-id")
	if err != nil {
		// Expected to fail due to network unavailability in tests
		if !strings.Contains(err.Error(), "failed to fetch collection details") {
			t.Errorf("Expected 'failed to fetch collection details' error, got: %v", err)
		}
	}
}

func TestDiscoverValidatedModelCollections(t *testing.T) {
	// Test basic function structure
	_, err := DiscoverValidatedModelCollections()
	if err != nil {
		// Expected to fail due to network unavailability in tests
		// Accept various types of network/API errors
		if !strings.Contains(err.Error(), "failed to fetch user collections") &&
			!strings.Contains(err.Error(), "failed to parse collections") &&
			!strings.Contains(err.Error(), "json:") {
			t.Errorf("Expected a collections-related error, got: %v", err)
		}
	}
}

func TestFetchModelDetails(t *testing.T) {
	// Test with a test model name
	_, err := FetchModelDetails("test/model")
	if err != nil {
		// Expected to fail due to network unavailability in tests
		// Accept various types of network/API errors
		if !strings.Contains(err.Error(), "failed to fetch model details") &&
			!strings.Contains(err.Error(), "API returned status") {
			t.Errorf("Expected a model details related error, got: %v", err)
		}
	}
}

func TestFetchReadme(t *testing.T) {
	// Test with a test model name
	_, err := FetchReadme("test/model")
	if err != nil {
		// Expected to fail due to network unavailability in tests
		// Accept various types of network/API errors
		if !strings.Contains(err.Error(), "failed to fetch README") &&
			!strings.Contains(err.Error(), "README not found") &&
			!strings.Contains(err.Error(), "status") {
			t.Errorf("Expected a README related error, got: %v", err)
		}
	}
}

func TestGetLatestVersionIndexFile(t *testing.T) {
	// Test with no files
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		err := os.Chdir(originalDir)
		if err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	tmpDir := t.TempDir()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	err = os.MkdirAll("data", 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Test with no version files
	_, err = GetLatestVersionIndexFile()
	if err == nil {
		t.Error("Expected error when no version index files exist")
	}

	// Create some test version files
	testFiles := []string{
		"data/hugging-face-redhat-ai-validated-v1-0.yaml",
		"data/hugging-face-redhat-ai-validated-v2-0.yaml",
		"data/hugging-face-redhat-ai-validated-v1-5.yaml",
	}

	for _, file := range testFiles {
		err = os.WriteFile(file, []byte("version: test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	// Test getting latest version file
	latest, err := GetLatestVersionIndexFile()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return the last file alphabetically (v2-0)
	expected := "data/hugging-face-redhat-ai-validated-v2-0.yaml"
	if latest != expected {
		t.Errorf("Expected %s, got %s", expected, latest)
	}
}

func TestLoadModelsFromVersionIndex(t *testing.T) {
	// Create temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-version-index.yaml")

	tests := []struct {
		name        string
		fileContent types.VersionIndex
		expected    []string
		expectError bool
	}{
		{
			name: "valid version index",
			fileContent: types.VersionIndex{
				Version: "v1.0",
				Models: []types.ModelIndex{
					{
						Name:       "RedHatAI/granite-3.1-8b-base",
						URL:        "https://huggingface.co/RedHatAI/granite-3.1-8b-base",
						ReadmePath: "/RedHatAI/granite-3.1-8b-base/README.md",
					},
					{
						Name:       "microsoft/Phi-3.5-mini-instruct",
						URL:        "https://huggingface.co/microsoft/Phi-3.5-mini-instruct",
						ReadmePath: "/microsoft/Phi-3.5-mini-instruct/README.md",
					},
				},
			},
			expected: []string{
				"registry.redhat.io/rhelai1/modelcar-redhatai-granite-3.1-8b-base",
				"registry.redhat.io/rhelai1/modelcar-microsoft-phi-3.5-mini-instruct",
			},
			expectError: false,
		},
		{
			name: "empty models",
			fileContent: types.VersionIndex{
				Version: "v1.0",
				Models:  []types.ModelIndex{},
			},
			expected:    []string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test data to file
			data, err := yaml.Marshal(tt.fileContent)
			if err != nil {
				t.Fatalf("Failed to marshal test data: %v", err)
			}

			err = os.WriteFile(tmpFile, data, 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
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

func TestLoadModelsFromVersionIndex_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.yaml")

	// Create invalid YAML
	err := os.WriteFile(tmpFile, []byte("invalid: yaml: content: ["), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	_, err = LoadModelsFromVersionIndex(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestStringSliceUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name:     "single scalar value",
			yaml:     "value: en",
			expected: []string{"en"},
		},
		{
			name:     "sequence of values",
			yaml:     "value:\n  - en\n  - es\n  - fr",
			expected: []string{"en", "es", "fr"},
		},
		{
			name:     "empty scalar",
			yaml:     "value: ''",
			expected: nil,
		},
		{
			name:     "whitespace only scalar",
			yaml:     "value: '   '",
			expected: nil,
		},
		{
			name:     "sequence with duplicates",
			yaml:     "value:\n  - en\n  - en\n  - es",
			expected: []string{"en", "es"},
		},
		{
			name:     "sequence with empty strings",
			yaml:     "value:\n  - en\n  - ''\n  - es",
			expected: []string{"en", "es"},
		},
		{
			name:     "scalar with whitespace",
			yaml:     "value: '  en  '",
			expected: []string{"en"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				Value stringSlice `yaml:"value"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &result)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result.Value) != len(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result.Value)
				return
			}

			for i, v := range tt.expected {
				if result.Value[i] != v {
					t.Errorf("Expected value[%d] = %s, got %s", i, v, result.Value[i])
				}
			}
		})
	}
}

func TestExtractYAMLFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		checkFields func(t *testing.T, fm *YAMLFrontmatter)
	}{
		{
			name:        "empty content",
			content:     "",
			expectError: true,
		},
		{
			name:        "no frontmatter",
			content:     "# Model Name\nSome content",
			expectError: true,
		},
		{
			name:        "malformed frontmatter no closing",
			content:     "---\nlicense: apache-2.0\nno closing",
			expectError: true,
		},
		{
			name: "valid frontmatter with scalar language",
			content: `---
language: en
license: apache-2.0
---
# Model content`,
			expectError: false,
			checkFields: func(t *testing.T, fm *YAMLFrontmatter) {
				if len(fm.Language) != 1 || fm.Language[0] != "en" {
					t.Errorf("Expected language [en], got %v", fm.Language)
				}
				if fm.License != "apache-2.0" {
					t.Errorf("Expected license apache-2.0, got %s", fm.License)
				}
			},
		},
		{
			name: "valid frontmatter with sequence language",
			content: `---
language:
  - en
  - es
  - fr
provider: NVIDIA
---
# Model content`,
			expectError: false,
			checkFields: func(t *testing.T, fm *YAMLFrontmatter) {
				expectedLangs := []string{"en", "es", "fr"}
				if len(fm.Language) != len(expectedLangs) {
					t.Errorf("Expected %d languages, got %d", len(expectedLangs), len(fm.Language))
					return
				}
				for i, lang := range expectedLangs {
					if fm.Language[i] != lang {
						t.Errorf("Expected language[%d] = %s, got %s", i, lang, fm.Language[i])
					}
				}
				if fm.Provider != "NVIDIA" {
					t.Errorf("Expected provider NVIDIA, got %s", fm.Provider)
				}
			},
		},
		{
			name: "frontmatter with scalar base_model",
			content: `---
base_model: meta-llama/Llama-3.1-8B-Instruct
license: llama3
---
# Model content`,
			expectError: false,
			checkFields: func(t *testing.T, fm *YAMLFrontmatter) {
				if len(fm.BaseModel) != 1 || fm.BaseModel[0] != "meta-llama/Llama-3.1-8B-Instruct" {
					t.Errorf("Expected base_model [meta-llama/Llama-3.1-8B-Instruct], got %v", fm.BaseModel)
				}
			},
		},
		{
			name: "frontmatter with sequence base_model",
			content: `---
base_model:
  - meta-llama/Llama-3.1-8B-Instruct
  - RedHatAI/granite-3.1-8b
---
# Model content`,
			expectError: false,
			checkFields: func(t *testing.T, fm *YAMLFrontmatter) {
				expectedModels := []string{"meta-llama/Llama-3.1-8B-Instruct", "RedHatAI/granite-3.1-8b"}
				if len(fm.BaseModel) != len(expectedModels) {
					t.Errorf("Expected %d base models, got %d", len(expectedModels), len(fm.BaseModel))
					return
				}
				for i, expected := range expectedModels {
					if fm.BaseModel[i] != expected {
						t.Errorf("BaseModel[%d]: expected %q, got %q", i, expected, fm.BaseModel[i])
					}
				}
			},
		},
		{
			name: "frontmatter with validated_on",
			content: `---
license: apache-2.0
validated_on:
  - RHOAI 2.20
  - RHAIIS 3.0
---
# Model content`,
			expectError: false,
			checkFields: func(t *testing.T, fm *YAMLFrontmatter) {
				expectedValidated := []string{"RHOAI 2.20", "RHAIIS 3.0"}
				if len(fm.ValidatedOn) != len(expectedValidated) {
					t.Errorf("Expected %d validated_on entries, got %d", len(expectedValidated), len(fm.ValidatedOn))
					return
				}
				for i, val := range expectedValidated {
					if fm.ValidatedOn[i] != val {
						t.Errorf("Expected validated_on[%d] = %s, got %s", i, val, fm.ValidatedOn[i])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := ExtractYAMLFrontmatter(tt.content)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tt.checkFields != nil {
				tt.checkFields(t, fm)
			}
		})
	}
}

func TestExtractProviderFromReadme(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "model developers field",
			content: `# Test Model

**Model Developers:** Neural Magic

Some other content here.
`,
			expected: "Neural Magic",
		},
		{
			name: "author field",
			content: `# Test Model

**Author:** IBM Research

Some content.
`,
			expected: "IBM Research",
		},
		{
			name: "provider field",
			content: `# Test Model

**Provider:** Microsoft

Content here.
`,
			expected: "Microsoft",
		},
		{
			name: "no provider found",
			content: `# Test Model

Some content without any company information.
`,
			expected: "",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
		{
			name: "provider with special characters",
			content: `# Test Model

**Model Developers:** Red Hat, Inc.

Content.
`,
			expected: "Red Hat, Inc",
		},
		{
			name: "invalid provider (too short)",
			content: `# Test Model

**Author:** X

Content.
`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractProviderFromReadme(tt.content)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
