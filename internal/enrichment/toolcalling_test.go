package enrichment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/internal/metadata"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestToolCallingIntegration_WithToolCalling(t *testing.T) {
	// Create temp directory for test output
	tmpDir := t.TempDir()

	// Setup test data with tool-calling config
	registryModel := "registry.redhat.io/rhai/modelcar-mistral-7b:1.0"
	enrichedData := &types.EnrichedModelMetadata{
		RegistryModel:    registryModel,
		HuggingFaceModel: "RedHatAI/Mistral-7B-Instruct",
		ToolCallingConfig: &types.ToolCallingConfig{
			Supported: true,
			RequiredCLIArgs: []string{
				"--config_format mistral",
				"--load_format mistral",
			},
			ChatTemplatePath: "examples/chat_template.jinja",
			ToolCallParser:   "mistral",
		},
		// Initialize metadata sources with "null" to skip processing
		Name:                 types.MetadataSource{Source: "null"},
		Provider:             types.MetadataSource{Source: "null"},
		Description:          types.MetadataSource{Source: "null"},
		License:              types.MetadataSource{Source: "null"},
		LicenseLink:          types.MetadataSource{Source: "null"},
		Language:             types.MetadataSource{Source: "null"},
		Tags:                 types.MetadataSource{Source: "null"},
		Tasks:                types.MetadataSource{Source: "null"},
		LastModified:         types.MetadataSource{Source: "null"},
		CreateTimeSinceEpoch: types.MetadataSource{Source: "null"},
		Downloads:            types.MetadataSource{Source: "null"},
		Likes:                types.MetadataSource{Source: "null"},
		ModelSize:            types.MetadataSource{Source: "null"},
		ValidatedOn:          types.MetadataSource{Source: "null"},
	}

	// Create modelcard.md in expected location
	sanitizedName := "registry.redhat.io_rhai_modelcar-mistral-7b_1.0"
	modelcardDir := filepath.Join(tmpDir, sanitizedName, "models")
	if err := os.MkdirAll(modelcardDir, 0755); err != nil {
		t.Fatalf("Failed to create modelcard dir: %v", err)
	}

	modelcardContent := `---
name: Mistral 7B Instruct
---
# Mistral 7B Instruct

This is a base model for testing.`

	modelcardPath := filepath.Join(modelcardDir, "modelcard.md")
	if err := os.WriteFile(modelcardPath, []byte(modelcardContent), 0644); err != nil {
		t.Fatalf("Failed to write modelcard: %v", err)
	}

	// Create initial metadata.yaml so UpdateModelMetadataFile has something to work with
	initialMetadata := types.ExtractedMetadata{
		Name: func() *string { s := "Mistral 7B"; return &s }(),
	}
	metadataBytes, _ := yaml.Marshal(initialMetadata)
	metadataPath := filepath.Join(modelcardDir, "metadata.yaml")
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		t.Fatalf("Failed to write initial metadata: %v", err)
	}

	// Execute UpdateModelMetadataFile
	if err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir); err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	// Load updated metadata
	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	// Verify README contains tool-calling section
	if updatedMetadata.Readme == nil {
		t.Fatal("README is nil after update")
	}

	readme := *updatedMetadata.Readme

	// Check for expected content
	expectedPhrases := []string{
		"vLLM Deployment",
		"tool calling",
		"--config_format mistral",
		"--load_format mistral",
		"opt/app-root/template/chat_template.jinja",
		"mistral",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(readme, phrase) {
			t.Errorf("README missing expected phrase: %q", phrase)
		}
	}

	// Verify original content is preserved
	if !strings.Contains(readme, "This is a base model for testing") {
		t.Error("Original README content was not preserved")
	}

	// Verify path was converted from examples/ to opt/app-root/template/
	if strings.Contains(readme, "examples/") {
		t.Error("Path was not converted from examples/ to opt/app-root/template/")
	}
}

func TestToolCallingIntegration_WithoutToolCalling(t *testing.T) {
	// Create temp directory for test output
	tmpDir := t.TempDir()

	// Setup test data WITHOUT tool-calling config
	registryModel := "registry.redhat.io/rhai/modelcar-granite-3b:1.0"
	enrichedData := &types.EnrichedModelMetadata{
		RegistryModel:     registryModel,
		HuggingFaceModel:  "RedHatAI/Granite-3B",
		ToolCallingConfig: nil, // No tool calling config
		// Initialize metadata sources with "null" to skip processing
		Name:                 types.MetadataSource{Source: "null"},
		Provider:             types.MetadataSource{Source: "null"},
		Description:          types.MetadataSource{Source: "null"},
		License:              types.MetadataSource{Source: "null"},
		LicenseLink:          types.MetadataSource{Source: "null"},
		Language:             types.MetadataSource{Source: "null"},
		Tags:                 types.MetadataSource{Source: "null"},
		Tasks:                types.MetadataSource{Source: "null"},
		LastModified:         types.MetadataSource{Source: "null"},
		CreateTimeSinceEpoch: types.MetadataSource{Source: "null"},
		Downloads:            types.MetadataSource{Source: "null"},
		Likes:                types.MetadataSource{Source: "null"},
		ModelSize:            types.MetadataSource{Source: "null"},
		ValidatedOn:          types.MetadataSource{Source: "null"},
	}

	// Create modelcard.md in expected location
	sanitizedName := "registry.redhat.io_rhai_modelcar-granite-3b_1.0"
	modelcardDir := filepath.Join(tmpDir, sanitizedName, "models")
	if err := os.MkdirAll(modelcardDir, 0755); err != nil {
		t.Fatalf("Failed to create modelcard dir: %v", err)
	}

	modelcardContent := `# Granite 3B

This is a model without tool calling support.`

	modelcardPath := filepath.Join(modelcardDir, "modelcard.md")
	if err := os.WriteFile(modelcardPath, []byte(modelcardContent), 0644); err != nil {
		t.Fatalf("Failed to write modelcard: %v", err)
	}

	// Create initial metadata.yaml
	initialMetadata := types.ExtractedMetadata{
		Name: func() *string { s := "Granite 3B"; return &s }(),
	}
	metadataBytes, _ := yaml.Marshal(initialMetadata)
	metadataPath := filepath.Join(modelcardDir, "metadata.yaml")
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		t.Fatalf("Failed to write initial metadata: %v", err)
	}

	// Execute UpdateModelMetadataFile
	if err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir); err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	// Load updated metadata
	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	// Verify README does NOT contain tool-calling section
	if updatedMetadata.Readme != nil {
		readme := *updatedMetadata.Readme

		// Should NOT have tool calling section
		if strings.Contains(readme, "vLLM Deployment") {
			t.Error("README should not contain tool-calling section when config is nil")
		}

		if strings.Contains(readme, "Tool Call Parser") {
			t.Error("README should not contain tool call parser section when config is nil")
		}

		// Original content should still be preserved
		if !strings.Contains(readme, "This is a model without tool calling support") {
			t.Error("Original README content was not preserved")
		}
	}
}

func TestToolCallingIntegration_EmptyConfig(t *testing.T) {
	// Test with empty config (no supported fields set)
	tmpDir := t.TempDir()

	registryModel := "registry.redhat.io/rhai/modelcar-test:1.0"
	enrichedData := &types.EnrichedModelMetadata{
		RegistryModel:     registryModel,
		HuggingFaceModel:  "RedHatAI/Test-Model",
		ToolCallingConfig: &types.ToolCallingConfig{
			// Empty config - no fields set
		},
		// Initialize metadata sources with "null" to skip processing
		Name:                 types.MetadataSource{Source: "null"},
		Provider:             types.MetadataSource{Source: "null"},
		Description:          types.MetadataSource{Source: "null"},
		License:              types.MetadataSource{Source: "null"},
		LicenseLink:          types.MetadataSource{Source: "null"},
		Language:             types.MetadataSource{Source: "null"},
		Tags:                 types.MetadataSource{Source: "null"},
		Tasks:                types.MetadataSource{Source: "null"},
		LastModified:         types.MetadataSource{Source: "null"},
		CreateTimeSinceEpoch: types.MetadataSource{Source: "null"},
		Downloads:            types.MetadataSource{Source: "null"},
		Likes:                types.MetadataSource{Source: "null"},
		ModelSize:            types.MetadataSource{Source: "null"},
		ValidatedOn:          types.MetadataSource{Source: "null"},
	}

	// Create modelcard.md
	sanitizedName := "registry.redhat.io_rhai_modelcar-test_1.0"
	modelcardDir := filepath.Join(tmpDir, sanitizedName, "models")
	if err := os.MkdirAll(modelcardDir, 0755); err != nil {
		t.Fatalf("Failed to create modelcard dir: %v", err)
	}

	modelcardContent := `# Test Model

Basic test model.`

	modelcardPath := filepath.Join(modelcardDir, "modelcard.md")
	if err := os.WriteFile(modelcardPath, []byte(modelcardContent), 0644); err != nil {
		t.Fatalf("Failed to write modelcard: %v", err)
	}

	// Create initial metadata.yaml
	initialMetadata := types.ExtractedMetadata{
		Name: func() *string { s := "Test Model"; return &s }(),
	}
	metadataBytes, _ := yaml.Marshal(initialMetadata)
	metadataPath := filepath.Join(modelcardDir, "metadata.yaml")
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		t.Fatalf("Failed to write initial metadata: %v", err)
	}

	// Execute
	if err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir); err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	// Load and verify
	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	// With empty config, HasToolCalling() should return false, so no section added
	if updatedMetadata.Readme != nil {
		readme := *updatedMetadata.Readme
		if strings.Contains(readme, "vLLM Deployment") {
			t.Error("Empty config should not trigger tool-calling section")
		}
	}
}

func TestToolCallingIntegration_WithoutValidatedOn(t *testing.T) {
	// CRITICAL TEST: Verify tool-calling extraction works WITHOUT validated_on field
	// This tests the bug fix where tool-calling was incorrectly nested inside validated_on block
	tmpDir := t.TempDir()

	// Setup test data with tool-calling config but NO validated_on
	registryModel := "registry.redhat.io/rhai/modelcar-qwen-7b:1.0"
	enrichedData := &types.EnrichedModelMetadata{
		RegistryModel:    registryModel,
		HuggingFaceModel: "RedHatAI/Qwen-7B-Instruct",
		ToolCallingConfig: &types.ToolCallingConfig{
			Supported: true,
			RequiredCLIArgs: []string{
				"--enable-prefix-caching",
			},
			ChatTemplatePath: "examples/qwen_template.jinja",
			ToolCallParser:   "hermes",
		},
		// Initialize metadata sources with "null" to skip processing
		Name:                 types.MetadataSource{Source: "null"},
		Provider:             types.MetadataSource{Source: "null"},
		Description:          types.MetadataSource{Source: "null"},
		License:              types.MetadataSource{Source: "null"},
		LicenseLink:          types.MetadataSource{Source: "null"},
		Language:             types.MetadataSource{Source: "null"},
		Tags:                 types.MetadataSource{Source: "null"},
		Tasks:                types.MetadataSource{Source: "null"},
		LastModified:         types.MetadataSource{Source: "null"},
		CreateTimeSinceEpoch: types.MetadataSource{Source: "null"},
		Downloads:            types.MetadataSource{Source: "null"},
		Likes:                types.MetadataSource{Source: "null"},
		ModelSize:            types.MetadataSource{Source: "null"},
		ValidatedOn:          types.MetadataSource{Source: "null"}, // NO validated_on!
	}

	// Create modelcard.md
	sanitizedName := "registry.redhat.io_rhai_modelcar-qwen-7b_1.0"
	modelcardDir := filepath.Join(tmpDir, sanitizedName, "models")
	if err := os.MkdirAll(modelcardDir, 0755); err != nil {
		t.Fatalf("Failed to create modelcard dir: %v", err)
	}

	modelcardContent := `# Qwen 7B Instruct

This model supports tool calling without validated_on field.`

	modelcardPath := filepath.Join(modelcardDir, "modelcard.md")
	if err := os.WriteFile(modelcardPath, []byte(modelcardContent), 0644); err != nil {
		t.Fatalf("Failed to write modelcard: %v", err)
	}

	// Create initial metadata.yaml
	initialMetadata := types.ExtractedMetadata{
		Name: func() *string { s := "Qwen 7B"; return &s }(),
	}
	metadataBytes, _ := yaml.Marshal(initialMetadata)
	metadataPath := filepath.Join(modelcardDir, "metadata.yaml")
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		t.Fatalf("Failed to write initial metadata: %v", err)
	}

	// Execute UpdateModelMetadataFile
	if err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir); err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	// Load updated metadata
	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	// CRITICAL: Verify README contains tool-calling section even without validated_on
	if updatedMetadata.Readme == nil {
		t.Fatal("README is nil - tool-calling section was not added")
	}

	readme := *updatedMetadata.Readme

	// Check for expected content
	expectedPhrases := []string{
		"vLLM Deployment",
		"tool calling",
		"--enable-prefix-caching",
		"opt/app-root/template/qwen_template.jinja",
		"hermes",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(readme, phrase) {
			t.Errorf("README missing expected phrase: %q (tool-calling extraction may have failed)", phrase)
		}
	}

	// Verify original content is preserved
	if !strings.Contains(readme, "This model supports tool calling without validated_on field") {
		t.Error("Original README content was not preserved")
	}

	// Verify path conversion
	if strings.Contains(readme, "examples/") {
		t.Error("Path was not converted from examples/ to opt/app-root/template/")
	}
}
