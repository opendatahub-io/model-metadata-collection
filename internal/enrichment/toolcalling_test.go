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

	// Verify README does NOT contain tool-calling section (structured data replaces it)
	if updatedMetadata.Readme != nil {
		readme := *updatedMetadata.Readme
		if strings.Contains(readme, "## vLLM Deployment with Tool Calling") {
			t.Error("README should not contain tool-calling Markdown section — structured servingConfig replaces it")
		}

		// Verify original content is preserved
		if !strings.Contains(readme, "This is a base model for testing") {
			t.Error("Original README content was not preserved")
		}
	}

	// Verify tool-calling config is persisted in metadata
	if updatedMetadata.ToolCallingConfig == nil {
		t.Fatal("ToolCallingConfig should be persisted in metadata")
	}
	tc := updatedMetadata.ToolCallingConfig
	if tc.ToolCallParser != "mistral" {
		t.Errorf("Expected ToolCallParser 'mistral', got %q", tc.ToolCallParser)
	}
	if len(tc.RequiredCLIArgs) != 2 {
		t.Errorf("Expected 2 RequiredCLIArgs, got %d", len(tc.RequiredCLIArgs))
	}
	if tc.GetProcessedTemplatePath() != "opt/app-root/template/chat_template.jinja" {
		t.Errorf("Expected processed template path, got %q", tc.GetProcessedTemplatePath())
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

	// CRITICAL: Verify tool-calling config is persisted in metadata even without validated_on
	if updatedMetadata.ToolCallingConfig == nil {
		t.Fatal("ToolCallingConfig should be persisted even without validated_on")
	}
	tc := updatedMetadata.ToolCallingConfig
	if tc.ToolCallParser != "hermes" {
		t.Errorf("Expected ToolCallParser 'hermes', got %q", tc.ToolCallParser)
	}
	if len(tc.RequiredCLIArgs) != 1 || tc.RequiredCLIArgs[0] != "--enable-prefix-caching" {
		t.Errorf("Expected RequiredCLIArgs ['--enable-prefix-caching'], got %v", tc.RequiredCLIArgs)
	}
	if tc.GetProcessedTemplatePath() != "opt/app-root/template/qwen_template.jinja" {
		t.Errorf("Expected processed template path, got %q", tc.GetProcessedTemplatePath())
	}

	// Verify README does NOT contain tool-calling section
	if updatedMetadata.Readme != nil {
		readme := *updatedMetadata.Readme
		if strings.Contains(readme, "## vLLM Deployment with Tool Calling") {
			t.Error("README should not contain tool-calling Markdown section")
		}
		// Verify original content is preserved
		if !strings.Contains(readme, "This model supports tool calling without validated_on field") {
			t.Error("Original README content was not preserved")
		}
	}
}

func TestToolCallingIntegration_WithValidatedTasks(t *testing.T) {
	tmpDir := t.TempDir()

	registryModel := "registry.redhat.io/rhai/modelcar-granite-4-0:3.0"
	enrichedData := &types.EnrichedModelMetadata{
		RegistryModel:    registryModel,
		HuggingFaceModel: "RedHatAI/Granite-4.0-H-Small",
		ToolCallingConfig: &types.ToolCallingConfig{
			Supported:      true,
			ToolCallParser: "granite",
			RequiredCLIArgs: []string{
				"--config_format granite",
			},
		},
		Name:                 types.MetadataSource{Source: "null"},
		Provider:             types.MetadataSource{Source: "null"},
		Description:          types.MetadataSource{Source: "null"},
		License:              types.MetadataSource{Source: "null"},
		LicenseLink:          types.MetadataSource{Source: "null"},
		Language:             types.MetadataSource{Source: "null"},
		Tags:                 types.MetadataSource{Source: "null"},
		Tasks:                metadata.CreateMetadataSource([]string{"text-generation", "tool-calling"}, "huggingface.yaml"),
		LastModified:         types.MetadataSource{Source: "null"},
		CreateTimeSinceEpoch: types.MetadataSource{Source: "null"},
		Downloads:            types.MetadataSource{Source: "null"},
		Likes:                types.MetadataSource{Source: "null"},
		ModelSize:            types.MetadataSource{Source: "null"},
		ValidatedOn:          types.MetadataSource{Source: "null"},
		ValidatedTasks:       metadata.CreateMetadataSource([]string{"tool-calling"}, "huggingface.yaml"),
	}

	sanitizedName := "registry.redhat.io_rhai_modelcar-granite-4-0_3.0"
	modelcardDir := filepath.Join(tmpDir, sanitizedName, "models")
	if err := os.MkdirAll(modelcardDir, 0755); err != nil {
		t.Fatalf("Failed to create modelcard dir: %v", err)
	}

	modelcardPath := filepath.Join(modelcardDir, "modelcard.md")
	if err := os.WriteFile(modelcardPath, []byte("# Granite 4.0\n\nTest model."), 0644); err != nil {
		t.Fatalf("Failed to write modelcard: %v", err)
	}

	initialMetadata := types.ExtractedMetadata{
		Name: func() *string { s := "Granite 4.0"; return &s }(),
	}
	metadataBytes, _ := yaml.Marshal(initialMetadata)
	metadataPath := filepath.Join(modelcardDir, "metadata.yaml")
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		t.Fatalf("Failed to write initial metadata: %v", err)
	}

	if err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir); err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	// Verify validatedTasks propagated
	if len(updatedMetadata.ValidatedTasks) != 1 || updatedMetadata.ValidatedTasks[0] != "tool-calling" {
		t.Errorf("Expected validatedTasks [tool-calling], got %v", updatedMetadata.ValidatedTasks)
	}

	// Verify tasks includes tool-calling
	hasToolCalling := false
	for _, task := range updatedMetadata.Tasks {
		if task == "tool-calling" {
			hasToolCalling = true
			break
		}
	}
	if !hasToolCalling {
		t.Errorf("Expected 'tool-calling' in tasks, got %v", updatedMetadata.Tasks)
	}

	// Verify tool-calling config persisted
	if updatedMetadata.ToolCallingConfig == nil {
		t.Fatal("Expected ToolCallingConfig to be persisted")
	}
	if updatedMetadata.ToolCallingConfig.ToolCallParser != "granite" {
		t.Errorf("Expected ToolCallParser 'granite', got %q", updatedMetadata.ToolCallingConfig.ToolCallParser)
	}

	// Verify no README tool-calling section
	if updatedMetadata.Readme != nil && strings.Contains(*updatedMetadata.Readme, "## vLLM Deployment with Tool Calling") {
		t.Error("README should not contain tool-calling Markdown section")
	}
}
