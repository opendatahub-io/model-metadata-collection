package enrichment

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/internal/huggingface"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

func TestEnrichMetadataFromHuggingFace_FilesNotExist(t *testing.T) {
	// Test with non-existent files
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

	// Test with missing HuggingFace index file
	err = EnrichMetadataFromHuggingFace("nonexistent-hf.yaml", "nonexistent-models.yaml", "output", "")
	if err == nil {
		t.Error("Expected error when HuggingFace index file doesn't exist")
	}
}

func TestEnrichMetadataFromHuggingFace_InvalidHFFile(t *testing.T) {
	// Test with invalid HuggingFace file
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

	// Create data directory and invalid HF file
	err = os.MkdirAll(huggingface.CollectionsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Create invalid YAML file
	invalidYAML := "invalid: yaml: content: ["
	err = os.WriteFile(huggingface.CollectionFilePath("v1-0"), []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid HF file: %v", err)
	}

	// Test with invalid HuggingFace file — must pass the prepared file so we
	// actually exercise the YAML parse path, not a file-not-found error.
	err = EnrichMetadataFromHuggingFace(huggingface.CollectionFilePath("v1-0"), "nonexistent-models.yaml", "output", "")
	if err == nil {
		t.Error("Expected error when HuggingFace index file is invalid")
	}
	if !strings.Contains(err.Error(), "failed to parse HuggingFace index") {
		t.Errorf("Expected YAML parse error, got: %v", err)
	}
}

func TestEnrichMetadataFromHuggingFace_MissingModelsIndex(t *testing.T) {
	// Test with valid HF file but missing models index
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

	// Create data directory and valid HF file
	err = os.MkdirAll(huggingface.CollectionsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Create valid HF index file
	hfIndex := types.VersionIndex{
		Version: "v1.0",
		Models: []types.ModelIndex{
			{
				Name:       "test/model",
				URL:        "https://huggingface.co/test/model",
				ReadmePath: "/test/model/README.md",
			},
		},
	}

	hfData, err := yaml.Marshal(hfIndex)
	if err != nil {
		t.Fatalf("Failed to marshal HF index: %v", err)
	}

	err = os.WriteFile(huggingface.CollectionFilePath("v1-0"), hfData, 0644)
	if err != nil {
		t.Fatalf("Failed to create HF file: %v", err)
	}

	// Test with missing models-index.yaml — must pass the prepared valid HF file
	// so we exercise the models index load path, not a file-not-found on the HF file.
	err = EnrichMetadataFromHuggingFace(huggingface.CollectionFilePath("v1-0"), "nonexistent-models.yaml", "output", "")
	if err == nil {
		t.Error("Expected error when models-index.yaml doesn't exist")
	}
	if !strings.Contains(err.Error(), "failed to load registry models") {
		t.Errorf("Expected registry models load error, got: %v", err)
	}
}

func TestEnrichMetadataFromHuggingFace_EmptyFiles(t *testing.T) {
	// Test with empty but valid files
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

	// Create collections directory and data directory
	err = os.MkdirAll(huggingface.CollectionsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create collections directory: %v", err)
	}
	err = os.MkdirAll("data", 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Create empty HF index file
	hfIndex := types.VersionIndex{
		Version: "v1.0",
		Models:  []types.ModelIndex{},
	}

	hfData, err := yaml.Marshal(hfIndex)
	if err != nil {
		t.Fatalf("Failed to marshal HF index: %v", err)
	}

	err = os.WriteFile(huggingface.CollectionFilePath("v1-0"), hfData, 0644)
	if err != nil {
		t.Fatalf("Failed to create HF file: %v", err)
	}

	// Create empty models config
	modelsConfig := types.ModelsConfig{
		Models: []types.ModelEntry{},
	}

	modelsData, err := yaml.Marshal(modelsConfig)
	if err != nil {
		t.Fatalf("Failed to marshal models config: %v", err)
	}

	err = os.WriteFile("data/models-index.yaml", modelsData, 0644)
	if err != nil {
		t.Fatalf("Failed to create models file: %v", err)
	}

	// Test with empty files - should succeed
	err = EnrichMetadataFromHuggingFace(huggingface.CollectionFilePath("v1-0"), "data/models-index.yaml", "output", "")
	if err != nil {
		t.Errorf("Unexpected error with empty files: %v", err)
	}
}

// TestEnrichMetadataFromHuggingFace_PinnedNameSkipsFuzzyMatch verifies that a
// model index entry with a pinned `name` (and no `hf_model`) never adopts a
// different model's identity via fuzzy matching, even when the HuggingFace
// collection contains a near-perfect lookalike. This is the fix for models
// like the "essential" Llama variant that have no correct HuggingFace page of
// their own and were previously mislabeled with another cataloged model's name.
func TestEnrichMetadataFromHuggingFace_PinnedNameSkipsFuzzyMatch(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	if err := os.MkdirAll(huggingface.CollectionsDir, 0755); err != nil {
		t.Fatalf("Failed to create collections directory: %v", err)
	}
	if err := os.MkdirAll("data", 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	const registryModel = "registry.redhat.io/rhai/modelcar-llama-3-1-8b-instruct-essential:3.0"
	const pinnedName = "RedHatAI/Llama-3.1-8B-Instruct-essential"

	// A lookalike entry that would otherwise win the fuzzy match by a wide margin.
	hfIndex := types.VersionIndex{
		Version: "v1.0",
		Models: []types.ModelIndex{
			{
				Name:       "RedHatAI/Llama-3.1-8B-Instruct",
				URL:        "https://huggingface.co/RedHatAI/Llama-3.1-8B-Instruct",
				ReadmePath: "/RedHatAI/Llama-3.1-8B-Instruct/README.md",
			},
		},
	}
	hfData, err := yaml.Marshal(hfIndex)
	if err != nil {
		t.Fatalf("Failed to marshal HF index: %v", err)
	}
	if err := os.WriteFile(huggingface.CollectionFilePath("v1-0"), hfData, 0644); err != nil {
		t.Fatalf("Failed to create HF file: %v", err)
	}

	modelsConfig := types.ModelsConfig{
		Models: []types.ModelEntry{
			{Type: "oci", URI: registryModel, Name: pinnedName},
		},
	}
	modelsData, err := yaml.Marshal(modelsConfig)
	if err != nil {
		t.Fatalf("Failed to marshal models config: %v", err)
	}
	if err := os.WriteFile("data/models-index.yaml", modelsData, 0644); err != nil {
		t.Fatalf("Failed to create models file: %v", err)
	}

	// Pre-create the output directory the way container extraction normally would,
	// so UpdateModelMetadataFile can write metadata.yaml.
	sanitized := utils.SanitizeManifestRef(registryModel)
	outputModelsDir := "output/" + sanitized + "/models"
	if err := os.MkdirAll(outputModelsDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	// This must not attempt any network calls: a pinned name with no hf_model
	// skips HuggingFace matching entirely (bestScore never reaches the 0.5
	// threshold), so FetchModelDetails/FetchReadme are never invoked.
	if err := EnrichMetadataFromHuggingFace(huggingface.CollectionFilePath("v1-0"), "data/models-index.yaml", "output", ""); err != nil {
		t.Fatalf("EnrichMetadataFromHuggingFace failed: %v", err)
	}

	metadataBytes, err := os.ReadFile(outputModelsDir + "/metadata.yaml")
	if err != nil {
		t.Fatalf("Failed to read written metadata.yaml: %v", err)
	}
	var written types.ExtractedMetadata
	if err := yaml.Unmarshal(metadataBytes, &written); err != nil {
		t.Fatalf("Failed to parse written metadata.yaml: %v", err)
	}

	if written.Name == nil || *written.Name != pinnedName {
		t.Errorf("Expected pinned name %q, got %v", pinnedName, written.Name)
	}
}

func TestUpdateModelMetadataFile_NoExistingFile(t *testing.T) {
	// Test updating metadata file when it doesn't exist yet
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

	// Test data
	registryModel := "registry.example.com/test/model:latest"
	enrichedData := &types.EnrichedModelMetadata{
		RegistryModel:    registryModel,
		EnrichmentStatus: "success",
		Name:             types.MetadataSource{Value: "Test Model", Source: "huggingface"},
		Provider:         types.MetadataSource{Value: "Test Provider", Source: "huggingface"},
		License:          types.MetadataSource{Value: "apache-2.0", Source: "huggingface"},
		Description:      types.MetadataSource{Value: "Test Description", Source: "huggingface"},
	}

	// Create output directory structure
	outputDir := "output/registry.example.com_test_model_latest/models"
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	// Call UpdateModelMetadataFile
	err = UpdateModelMetadataFile(registryModel, enrichedData, "output")
	if err != nil {
		t.Errorf("UpdateModelMetadataFile failed: %v", err)
	}

	// Verify enrichment.yaml was created
	enrichmentPath := "output/registry.example.com_test_model_latest/models/enrichment.yaml"
	if _, err := os.Stat(enrichmentPath); os.IsNotExist(err) {
		t.Errorf("Enrichment file was not created at %s", enrichmentPath)
	}
}

func TestUpdateModelMetadataFile_WithExistingFile(t *testing.T) {
	// Test updating metadata file when it already exists
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

	// Create output directory structure
	registryModel := "registry.example.com/test/model:latest"
	outputDir := "output/registry.example.com_test_model_latest/models"
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	// Create existing metadata file
	existingName := "Existing Model"
	existingProvider := "Existing Provider"
	existingDescription := "Existing description"
	existingMetadata := types.ExtractedMetadata{
		Name:        &existingName,
		Provider:    &existingProvider,
		Description: &existingDescription,
	}
	metadataData, err := yaml.Marshal(existingMetadata)
	if err != nil {
		t.Fatalf("Failed to marshal existing metadata: %v", err)
	}

	metadataPath := outputDir + "/metadata.yaml"
	err = os.WriteFile(metadataPath, metadataData, 0644)
	if err != nil {
		t.Fatalf("Failed to create existing metadata file: %v", err)
	}

	// Test data
	enrichedData := &types.EnrichedModelMetadata{
		RegistryModel:    registryModel,
		EnrichmentStatus: "success",
		Name:             types.MetadataSource{Value: "Enriched Model", Source: "huggingface"},
		Provider:         types.MetadataSource{Value: "Enriched Provider", Source: "huggingface"},
		License:          types.MetadataSource{Value: "mit", Source: "huggingface"},
		Description:      types.MetadataSource{Value: "Enriched Description", Source: "huggingface"},
	}

	// Call UpdateModelMetadataFile
	err = UpdateModelMetadataFile(registryModel, enrichedData, "output")
	if err != nil {
		t.Errorf("UpdateModelMetadataFile failed: %v", err)
	}

	// Verify files were created/updated
	enrichmentPath := outputDir + "/enrichment.yaml"
	if _, err := os.Stat(enrichmentPath); os.IsNotExist(err) {
		t.Errorf("Enrichment file was not created")
	}

	// Verify metadata file still exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Errorf("Metadata file should still exist")
	}
}

// TestUpdateModelMetadataFile_IndexPinnedNameOverridesExisting verifies that a
// name sourced from "index.pinned" (a hand-curated override in the model
// index) always wins over an existing container-extracted name, the same way
// "huggingface.yaml" already does. Without this, a pinned name would only take
// effect on models that don't already have a name written to metadata.yaml.
func TestUpdateModelMetadataFile_IndexPinnedNameOverridesExisting(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	registryModel := "registry.example.com/test/model:latest"
	outputDir := "output/registry.example.com_test_model_latest/models"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	existingName := "wrong-fuzzy-matched-name"
	existingMetadata := types.ExtractedMetadata{Name: &existingName}
	metadataData, err := yaml.Marshal(existingMetadata)
	if err != nil {
		t.Fatalf("Failed to marshal existing metadata: %v", err)
	}
	metadataPath := outputDir + "/metadata.yaml"
	if err := os.WriteFile(metadataPath, metadataData, 0644); err != nil {
		t.Fatalf("Failed to create existing metadata file: %v", err)
	}

	const pinnedName = "RedHatAI/correct-name"
	enrichedData := &types.EnrichedModelMetadata{
		RegistryModel:    registryModel,
		EnrichmentStatus: "name_pinned",
		Name:             types.MetadataSource{Value: pinnedName, Source: "index.pinned"},
		// Every other field must be explicitly "null" (the zero value's empty-string
		// Source is NOT "null" and would otherwise be misread as a real, unset value).
		Provider:    types.MetadataSource{Source: "null"},
		Description: types.MetadataSource{Source: "null"},
		License:     types.MetadataSource{Source: "null"},
		LicenseLink: types.MetadataSource{Source: "null"},
	}

	if err := UpdateModelMetadataFile(registryModel, enrichedData, "output"); err != nil {
		t.Fatalf("UpdateModelMetadataFile failed: %v", err)
	}

	updatedBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read updated metadata.yaml: %v", err)
	}
	var updated types.ExtractedMetadata
	if err := yaml.Unmarshal(updatedBytes, &updated); err != nil {
		t.Fatalf("Failed to parse updated metadata.yaml: %v", err)
	}

	if updated.Name == nil || *updated.Name != pinnedName {
		t.Errorf("Expected index.pinned name %q to override existing name %q, got %v", pinnedName, existingName, updated.Name)
	}
}

func TestUpdateAllModelsWithOCIArtifacts(t *testing.T) {
	// Test UpdateAllModelsWithOCIArtifacts function
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

	// Create directories
	err = os.MkdirAll(huggingface.CollectionsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create collections directory: %v", err)
	}
	err = os.MkdirAll("data", 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Create models config with test models
	modelsConfig := types.ModelsConfig{
		Models: []types.ModelEntry{
			{Type: "oci", URI: "registry.example.com/test/model1:latest", Labels: []string{"validated"}},
			{Type: "oci", URI: "registry.example.com/test/model2:latest", Labels: []string{"validated"}},
		},
	}

	modelsData, err := yaml.Marshal(modelsConfig)
	if err != nil {
		t.Fatalf("Failed to marshal models config: %v", err)
	}

	err = os.WriteFile("data/models-index.yaml", modelsData, 0644)
	if err != nil {
		t.Fatalf("Failed to create models file: %v", err)
	}

	// Call UpdateAllModelsWithOCIArtifacts
	err = UpdateAllModelsWithOCIArtifacts("data/models-index.yaml", "output")
	// This will likely fail due to network calls to registries, but we test that it doesn't panic
	// and that it attempts to process the models
	if err != nil {
		t.Logf("Expected error due to network calls: %v", err)
	}
}

func TestUpdateOCIArtifacts_InvalidModel(t *testing.T) {
	// Test UpdateOCIArtifacts with invalid model reference
	err := UpdateOCIArtifacts("invalid-model-reference", "output")
	if err == nil {
		t.Error("Expected error for invalid model reference")
	}
}

func TestIsLowQualityModelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Low quality names (should return true)
		{
			name:     "empty string",
			input:    "",
			expected: true,
		},
		{
			name:     "contains model card",
			input:    "Model Card - Test",
			expected: true,
		},
		{
			name:     "contains readme",
			input:    "README for the model",
			expected: true,
		},
		{
			name:     "contains documentation",
			input:    "Documentation page",
			expected: true,
		},
		{
			name:     "ends with card",
			input:    "Test Card",
			expected: true,
		},
		{
			name:     "contains modify (code comment artifact)",
			input:    "Modify OpenAI's API key in the code above",
			expected: true,
		},
		{
			name:     "contains api key",
			input:    "Set your API key here",
			expected: true,
		},
		{
			name:     "contains openai",
			input:    "OpenAI compatible setup",
			expected: true,
		},
		{
			name:     "contains example",
			input:    "Example usage instructions",
			expected: true,
		},
		{
			name:     "contains todo",
			input:    "TODO: add documentation",
			expected: true,
		},
		{
			name:     "contains note:",
			input:    "note: this is a test",
			expected: true,
		},
		{
			name:     "contains warning:",
			input:    "warning: do not use in production",
			expected: true,
		},
		{
			name:     "excessively long name",
			input:    "This is a very long model name that exceeds the maximum allowed length and should be considered low quality",
			expected: true,
		},

		// Good quality names (should return false)
		{
			name:     "simple model name",
			input:    "Llama-3.1-8B-Instruct",
			expected: false,
		},
		{
			name:     "huggingface format model name",
			input:    "RedHatAI/granite-3.1-8b-base",
			expected: false,
		},
		{
			name:     "quantized model name",
			input:    "Meta-Llama-3.1-8B-Instruct-quantized.w4a16",
			expected: false,
		},
		{
			name:     "fp8 dynamic model name",
			input:    "granite-3.1-8b-base-FP8-dynamic",
			expected: false,
		},
		{
			name:     "short reasonable name",
			input:    "Test Model v1.0",
			expected: false,
		},
		{
			name:     "name with version number",
			input:    "Phi-3.5-mini-instruct",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLowQualityModelName(tt.input)
			if result != tt.expected {
				t.Errorf("isLowQualityModelName(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}
