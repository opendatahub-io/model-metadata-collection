package catalog

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestCreateModelsCatalog(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	// Create test metadata files
	testModels := []struct {
		path     string
		metadata types.ExtractedMetadata
	}{
		{
			path: "model1/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name:        stringPtr("Test Model 1"),
				Provider:    stringPtr("Test Provider"),
				Description: stringPtr("A test model for unit testing"),
				License:     stringPtr("Apache-2.0"),
				Language:    []string{"en"},
				Tasks:       []string{"text-generation"},
				Tags:        []string{"validated", "featured", "test-tag"},
			},
		},
		{
			path: "model2/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name:        stringPtr("Test Model 2"),
				Provider:    stringPtr("Another Provider"),
				Description: stringPtr("Another test model"),
				License:     stringPtr("MIT"),
				Language:    []string{"en", "es"},
				Tasks:       []string{"text-classification"},
				Tags:        []string{"validated"},
			},
		},
		{
			path: "model3/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Test Model 3"),
				// Some fields intentionally nil to test handling
			},
		},
	}

	// Create the test directory structure and files
	for _, model := range testModels {
		fullPath := filepath.Join(outputDir, model.path)
		dir := filepath.Dir(fullPath)

		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}

		data, err := yaml.Marshal(model.metadata)
		if err != nil {
			t.Fatalf("Failed to marshal test metadata: %v", err)
		}

		err = os.WriteFile(fullPath, data, 0644)
		if err != nil {
			t.Fatalf("Failed to create test metadata file %s: %v", fullPath, err)
		}
	}

	// Change to the temp directory so CreateModelsCatalog can find the output directory
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

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create data directory for catalog output
	err = os.MkdirAll("data", 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Test CreateModelsCatalog
	testCatalogPath := filepath.Join("data", "test-models-catalog.yaml")
	err = CreateModelsCatalog("output", testCatalogPath)
	if err != nil {
		t.Fatalf("CreateModelsCatalog failed: %v", err)
	}

	// Verify the catalog file was created
	catalogPath := filepath.Join(tmpDir, "data", "test-models-catalog.yaml")
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		t.Fatal("Catalog file was not created")
	}

	// Read and parse the catalog file
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog file: %v", err)
	}

	var catalog types.ModelsCatalog
	err = yaml.Unmarshal(catalogData, &catalog)
	if err != nil {
		t.Fatalf("Failed to parse catalog YAML: %v", err)
	}

	// Verify catalog structure
	if catalog.Source != "Red Hat" {
		t.Errorf("Expected source 'Red Hat', got '%s'", catalog.Source)
	}

	if len(catalog.Models) != len(testModels) {
		t.Errorf("Expected %d models in catalog, got %d", len(testModels), len(catalog.Models))
	}

	// Verify models are sorted by name
	expectedOrder := []string{"Test Model 1", "Test Model 2", "Test Model 3"}
	for i, model := range catalog.Models {
		if model.Name == nil {
			if expectedOrder[i] != "" {
				t.Errorf("Expected model name '%s' at index %d, got nil", expectedOrder[i], i)
			}
		} else if *model.Name != expectedOrder[i] {
			t.Errorf("Expected model name '%s' at index %d, got '%s'", expectedOrder[i], i, *model.Name)
		}
	}

	// Verify specific model content
	model1 := catalog.Models[0]
	if model1.Name == nil || *model1.Name != "Test Model 1" {
		t.Error("First model should be 'Test Model 1'")
	}
	if model1.Provider == nil || *model1.Provider != "Test Provider" {
		t.Error("First model provider should be 'Test Provider'")
	}
	if len(model1.Language) != 1 || model1.Language[0] != "en" {
		t.Error("First model should have language 'en'")
	}
}

func TestCreateModelsCatalog_EmptyOutput(t *testing.T) {
	// Test with empty output directory
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create empty output directory: %v", err)
	}

	// Change to temp directory
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

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create data directory for catalog output
	err = os.MkdirAll("data", 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Test CreateModelsCatalog with empty directory
	testCatalogPath := filepath.Join("data", "test-models-catalog.yaml")
	err = CreateModelsCatalog("output", testCatalogPath)
	if err != nil {
		t.Fatalf("CreateModelsCatalog failed with empty directory: %v", err)
	}

	// Verify catalog file was created with empty models list
	catalogPath := filepath.Join(tmpDir, "data", "test-models-catalog.yaml")
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog file: %v", err)
	}

	var catalog types.ModelsCatalog
	err = yaml.Unmarshal(catalogData, &catalog)
	if err != nil {
		t.Fatalf("Failed to parse catalog YAML: %v", err)
	}

	if len(catalog.Models) != 0 {
		t.Errorf("Expected 0 models in empty catalog, got %d", len(catalog.Models))
	}
}

func TestCreateModelsCatalog_NoOutputDirectory(t *testing.T) {
	// Test with no output directory - should create empty catalog
	tmpDir := t.TempDir()

	// Change to temp directory
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

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create data directory for catalog output
	err = os.MkdirAll("data", 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Test CreateModelsCatalog with no output directory - should not fail
	testCatalogPath := filepath.Join("data", "test-models-catalog.yaml")
	err = CreateModelsCatalog("output", testCatalogPath)
	if err != nil {
		// The function should handle missing output directory gracefully
		t.Logf("CreateModelsCatalog returned error (expected for missing output dir): %v", err)
		return
	}

	// If it succeeded, should create catalog with empty models list
	catalogPath := filepath.Join(tmpDir, "data", "test-models-catalog.yaml")
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		t.Fatal("Catalog file was not created")
	}
}

func TestCreateModelsCatalog_InvalidMetadata(t *testing.T) {
	// Test with invalid metadata file
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	metadataDir := filepath.Join(outputDir, "invalid-model", "models")

	err := os.MkdirAll(metadataDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create invalid YAML file
	invalidYAML := "invalid: yaml: content: ["
	err = os.WriteFile(filepath.Join(metadataDir, "metadata.yaml"), []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid metadata file: %v", err)
	}

	// Change to temp directory
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

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create data directory for catalog output
	err = os.MkdirAll("data", 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Test CreateModelsCatalog - should continue processing despite invalid file
	testCatalogPath := filepath.Join("data", "test-models-catalog.yaml")
	err = CreateModelsCatalog("output", testCatalogPath)
	if err != nil {
		t.Fatalf("CreateModelsCatalog failed: %v", err)
	}

	// Verify catalog was still created (invalid files are skipped)
	catalogPath := filepath.Join(tmpDir, "data", "test-models-catalog.yaml")
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		t.Fatal("Catalog file was not created")
	}

	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog file: %v", err)
	}

	var catalog types.ModelsCatalog
	err = yaml.Unmarshal(catalogData, &catalog)
	if err != nil {
		t.Fatalf("Failed to parse catalog YAML: %v", err)
	}

	// Should have 0 models since the invalid file was skipped
	if len(catalog.Models) != 0 {
		t.Errorf("Expected 0 models after skipping invalid metadata, got %d", len(catalog.Models))
	}
}

// TestLogoAssignment tests that logos are correctly assigned based on validation labels using embedded assets
func TestLogoAssignment(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "catalog_logo_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create output directory structure
	outputDir := filepath.Join(tmpDir, "output")
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	// Create data directory for catalog output
	dataDir := filepath.Join(tmpDir, "data")
	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Get expected logos from embedded assets using our determineLogo function
	validatedLogo := determineLogo([]string{"validated"})
	nonValidatedLogo := determineLogo([]string{"featured"})

	if validatedLogo == nil || nonValidatedLogo == nil {
		t.Fatalf("Failed to get logos from embedded assets")
	}

	// These should be different since one has validated tag and other doesn't
	if *validatedLogo == *nonValidatedLogo {
		t.Errorf("Expected different logos for validated vs non-validated models")
	}

	// Test models with different validation labels
	testModels := []struct {
		path              string
		metadata          types.ExtractedMetadata
		shouldBeValidated bool
	}{
		{
			path: "validated-model/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Validated Model"),
				Tags: []string{"validated", "featured"},
			},
			shouldBeValidated: true,
		},
		{
			path: "non-validated-model/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Non-Validated Model"),
				Tags: []string{"featured"},
			},
			shouldBeValidated: false,
		},
		{
			path: "model-with-only-validated/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Model With Only Validated"),
				Tags: []string{"validated"},
			},
			shouldBeValidated: true,
		},
		{
			path: "model-no-tags/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Model With No Tags"),
				Tags: []string{},
			},
			shouldBeValidated: false,
		},
	}

	// Create the test directory structure and files
	for _, model := range testModels {
		fullPath := filepath.Join(outputDir, model.path)
		dir := filepath.Dir(fullPath)

		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}

		data, err := yaml.Marshal(model.metadata)
		if err != nil {
			t.Fatalf("Failed to marshal test metadata: %v", err)
		}

		err = os.WriteFile(fullPath, data, 0644)
		if err != nil {
			t.Fatalf("Failed to create test metadata file %s: %v", fullPath, err)
		}
	}

	// Change to the temp directory so CreateModelsCatalog can find the output directory
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

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test CreateModelsCatalog
	testCatalogPath := filepath.Join("data", "test-models-catalog.yaml")
	err = CreateModelsCatalog("output", testCatalogPath)
	if err != nil {
		t.Fatalf("CreateModelsCatalog failed: %v", err)
	}

	// Verify catalog was created
	catalogPath := filepath.Join(tmpDir, "data", "test-models-catalog.yaml")
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		t.Fatal("Catalog file was not created")
	}

	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog file: %v", err)
	}

	var catalog types.ModelsCatalog
	err = yaml.Unmarshal(catalogData, &catalog)
	if err != nil {
		t.Fatalf("Failed to parse catalog YAML: %v", err)
	}

	// Should have 4 models
	if len(catalog.Models) != 4 {
		t.Fatalf("Expected 4 models in catalog, got %d", len(catalog.Models))
	}

	// Check logos for each model
	modelLogoMap := make(map[string]string)
	for _, model := range catalog.Models {
		if model.Name != nil && model.Logo != nil {
			modelLogoMap[*model.Name] = *model.Logo
		}
	}

	// Verify logos are assigned correctly based on validation status
	expectedValidationStatus := map[string]bool{
		"Validated Model":           true,
		"Non-Validated Model":       false,
		"Model With Only Validated": true,
		"Model With No Tags":        false,
	}

	for modelName, shouldBeValidated := range expectedValidationStatus {
		actualLogo, exists := modelLogoMap[modelName]
		if !exists {
			t.Errorf("Model %s not found in catalog", modelName)
			continue
		}

		// Verify logo exists and is properly formatted
		if !strings.HasPrefix(actualLogo, "data:image/svg+xml;base64,") {
			t.Errorf("Model %s: logo should be a base64 data URI, got: %s", modelName, actualLogo)
			continue
		}

		// Check that the correct logo type is assigned
		expectedLogo := nonValidatedLogo
		if shouldBeValidated {
			expectedLogo = validatedLogo
		}

		if actualLogo != *expectedLogo {
			t.Errorf("Model %s: expected %s validation logo, got different logo", modelName,
				map[bool]string{true: "validated", false: "non-validated"}[shouldBeValidated])
		}
	}
}

// TestDetermineLogo tests the logo determination logic directly using embedded assets
func TestDetermineLogo(t *testing.T) {

	testCases := []struct {
		name              string
		tags              []string
		shouldBeValidated bool
	}{
		{
			name:              "validated tag present",
			tags:              []string{"validated", "featured"},
			shouldBeValidated: true,
		},
		{
			name:              "only validated tag",
			tags:              []string{"validated"},
			shouldBeValidated: true,
		},
		{
			name:              "no validated tag",
			tags:              []string{"featured", "popular"},
			shouldBeValidated: false,
		},
		{
			name:              "empty tags",
			tags:              []string{},
			shouldBeValidated: false,
		},
		{
			name:              "nil tags",
			tags:              nil,
			shouldBeValidated: false,
		},
	}

	// Get reference logos to compare against
	validatedLogo := determineLogo([]string{"validated"})
	nonValidatedLogo := determineLogo([]string{"featured"})

	if validatedLogo == nil || nonValidatedLogo == nil {
		t.Fatal("Failed to get reference logos")
	}

	// Validated and non-validated logos should be different
	if *validatedLogo == *nonValidatedLogo {
		t.Errorf("Expected different logos for validated vs non-validated models")
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logo := determineLogo(tc.tags)
			if logo == nil {
				t.Fatal("determineLogo returned nil")
			}

			// Verify it's a valid data URI
			if !strings.HasPrefix(*logo, "data:image/svg+xml;base64,") {
				t.Errorf("Expected valid data URI, got %s", *logo)
			}

			// Verify we get substantial content from embedded assets
			if len(*logo) < 100 {
				t.Errorf("Expected substantial logo content from embedded assets, got short string: %s", *logo)
			}

			// Verify correct logo type is returned based on validation status
			expectedLogo := nonValidatedLogo
			if tc.shouldBeValidated {
				expectedLogo = validatedLogo
			}

			if *logo != *expectedLogo {
				t.Errorf("Expected %s validation logo, got different logo",
					map[bool]string{true: "validated", false: "non-validated"}[tc.shouldBeValidated])
			}
		})
	}
}

// Helper function to create string pointers for testing
func stringPtr(s string) *string {
	return &s
}

func TestLoadStaticCatalogs(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Test case 1: Valid static catalog file
	validCatalog := types.ModelsCatalog{
		Source: "Test Source",
		Models: []types.CatalogMetadata{
			{
				Name:        stringPtr("Static Model 1"),
				Provider:    stringPtr("Static Provider"),
				Description: stringPtr("A static test model"),
				License:     stringPtr("MIT"),
				Language:    []string{"en"},
				Tasks:       []string{"text-generation"},
				Artifacts: []types.CatalogOCIArtifact{
					{
						URI: "oci://example.com/model:1.0",
					},
				},
			},
			{
				Name:        stringPtr("Static Model 2"),
				Provider:    stringPtr("Another Provider"),
				Description: stringPtr("Another static model"),
				Artifacts: []types.CatalogOCIArtifact{
					{
						URI: "oci://example.com/model2:1.0",
					},
				},
			},
		},
	}

	validCatalogPath := filepath.Join(tmpDir, "valid-catalog.yaml")
	validData, err := yaml.Marshal(validCatalog)
	if err != nil {
		t.Fatalf("Failed to marshal valid catalog: %v", err)
	}
	err = os.WriteFile(validCatalogPath, validData, 0644)
	if err != nil {
		t.Fatalf("Failed to write valid catalog file: %v", err)
	}

	// Test case 2: Invalid YAML file
	invalidCatalogPath := filepath.Join(tmpDir, "invalid-catalog.yaml")
	invalidYAML := "invalid: yaml: content: ["
	err = os.WriteFile(invalidCatalogPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid catalog file: %v", err)
	}

	// Test case 3: Valid YAML but invalid structure (missing required fields)
	invalidStructurePath := filepath.Join(tmpDir, "invalid-structure.yaml")
	invalidStructure := types.ModelsCatalog{
		Source: "", // Missing source
		Models: []types.CatalogMetadata{
			{
				Name: stringPtr("Model Without Artifacts"),
				// Missing artifacts
			},
		},
	}
	invalidStructureData, err := yaml.Marshal(invalidStructure)
	if err != nil {
		t.Fatalf("Failed to marshal invalid structure catalog: %v", err)
	}
	err = os.WriteFile(invalidStructurePath, invalidStructureData, 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid structure catalog file: %v", err)
	}

	// Test successful loading of valid catalog
	t.Run("ValidCatalog", func(t *testing.T) {
		models, err := LoadStaticCatalogs([]string{validCatalogPath})
		if err != nil {
			t.Fatalf("LoadStaticCatalogs failed: %v", err)
		}

		if len(models) != 2 {
			t.Errorf("Expected 2 models, got %d", len(models))
		}

		if models[0].Name == nil || *models[0].Name != "Static Model 1" {
			t.Error("First model should be 'Static Model 1'")
		}
		if models[1].Name == nil || *models[1].Name != "Static Model 2" {
			t.Error("Second model should be 'Static Model 2'")
		}
	})

	// Test handling of missing files
	t.Run("MissingFile", func(t *testing.T) {
		missingFilePath := filepath.Join(tmpDir, "nonexistent.yaml")
		models, err := LoadStaticCatalogs([]string{missingFilePath})
		if err != nil {
			t.Fatalf("LoadStaticCatalogs failed: %v", err)
		}

		if len(models) != 0 {
			t.Errorf("Expected 0 models for missing file, got %d", len(models))
		}
	})

	// Test handling of invalid YAML
	t.Run("InvalidYAML", func(t *testing.T) {
		models, err := LoadStaticCatalogs([]string{invalidCatalogPath})
		if err != nil {
			t.Fatalf("LoadStaticCatalogs failed: %v", err)
		}

		if len(models) != 0 {
			t.Errorf("Expected 0 models for invalid YAML, got %d", len(models))
		}
	})

	// Test handling of invalid structure
	t.Run("InvalidStructure", func(t *testing.T) {
		models, err := LoadStaticCatalogs([]string{invalidStructurePath})
		if err != nil {
			t.Fatalf("LoadStaticCatalogs failed: %v", err)
		}

		if len(models) != 0 {
			t.Errorf("Expected 0 models for invalid structure, got %d", len(models))
		}
	})

	// Test loading multiple files
	t.Run("MultipleFiles", func(t *testing.T) {
		// Create second valid catalog
		validCatalog2 := types.ModelsCatalog{
			Source: "Second Source",
			Models: []types.CatalogMetadata{
				{
					Name:        stringPtr("Static Model 3"),
					Provider:    stringPtr("Third Provider"),
					Description: stringPtr("Third static model"),
					Artifacts: []types.CatalogOCIArtifact{
						{
							URI: "oci://example.com/model3:1.0",
						},
					},
				},
			},
		}

		validCatalog2Path := filepath.Join(tmpDir, "valid-catalog2.yaml")
		validData2, err := yaml.Marshal(validCatalog2)
		if err != nil {
			t.Fatalf("Failed to marshal second valid catalog: %v", err)
		}
		err = os.WriteFile(validCatalog2Path, validData2, 0644)
		if err != nil {
			t.Fatalf("Failed to write second valid catalog file: %v", err)
		}

		models, err := LoadStaticCatalogs([]string{validCatalogPath, validCatalog2Path})
		if err != nil {
			t.Fatalf("LoadStaticCatalogs failed: %v", err)
		}

		if len(models) != 3 {
			t.Errorf("Expected 3 models from two files, got %d", len(models))
		}
	})

	// Test empty file list
	t.Run("EmptyFileList", func(t *testing.T) {
		models, err := LoadStaticCatalogs([]string{})
		if err != nil {
			t.Fatalf("LoadStaticCatalogs failed: %v", err)
		}

		if len(models) != 0 {
			t.Errorf("Expected 0 models for empty file list, got %d", len(models))
		}
	})
}

func TestValidateStaticCatalog(t *testing.T) {
	// Test valid catalog
	t.Run("ValidCatalog", func(t *testing.T) {
		catalog := &types.ModelsCatalog{
			Source: "Test Source",
			Models: []types.CatalogMetadata{
				{
					Name: stringPtr("Valid Model"),
					Artifacts: []types.CatalogOCIArtifact{
						{
							URI: "oci://example.com/model:1.0",
						},
					},
				},
			},
		}

		err := validateStaticCatalog(catalog)
		if err != nil {
			t.Errorf("Valid catalog should not produce error: %v", err)
		}
	})

	// Test missing source
	t.Run("MissingSource", func(t *testing.T) {
		catalog := &types.ModelsCatalog{
			Source: "",
			Models: []types.CatalogMetadata{
				{
					Name: stringPtr("Model"),
					Artifacts: []types.CatalogOCIArtifact{
						{
							URI: "oci://example.com/model:1.0",
						},
					},
				},
			},
		}

		err := validateStaticCatalog(catalog)
		if err == nil {
			t.Error("Expected error for missing source")
		}
		if !strings.Contains(err.Error(), "missing required 'source' field") {
			t.Errorf("Error should mention missing source field: %v", err)
		}
	})

	// Test missing model name
	t.Run("MissingModelName", func(t *testing.T) {
		catalog := &types.ModelsCatalog{
			Source: "Test Source",
			Models: []types.CatalogMetadata{
				{
					Name: nil,
					Artifacts: []types.CatalogOCIArtifact{
						{
							URI: "oci://example.com/model:1.0",
						},
					},
				},
			},
		}

		err := validateStaticCatalog(catalog)
		if err == nil {
			t.Error("Expected error for missing model name")
		}
		if !strings.Contains(err.Error(), "missing required 'name' field") {
			t.Errorf("Error should mention missing name field: %v", err)
		}
	})

	// Test empty model name
	t.Run("EmptyModelName", func(t *testing.T) {
		catalog := &types.ModelsCatalog{
			Source: "Test Source",
			Models: []types.CatalogMetadata{
				{
					Name: stringPtr(""),
					Artifacts: []types.CatalogOCIArtifact{
						{
							URI: "oci://example.com/model:1.0",
						},
					},
				},
			},
		}

		err := validateStaticCatalog(catalog)
		if err == nil {
			t.Error("Expected error for empty model name")
		}
		if !strings.Contains(err.Error(), "missing required 'name' field") {
			t.Errorf("Error should mention missing name field: %v", err)
		}
	})

	// Test missing artifacts
	t.Run("MissingArtifacts", func(t *testing.T) {
		catalog := &types.ModelsCatalog{
			Source: "Test Source",
			Models: []types.CatalogMetadata{
				{
					Name:      stringPtr("Model Without Artifacts"),
					Artifacts: []types.CatalogOCIArtifact{},
				},
			},
		}

		err := validateStaticCatalog(catalog)
		if err == nil {
			t.Error("Expected error for missing artifacts")
		}
		if !strings.Contains(err.Error(), "has no artifacts") {
			t.Errorf("Error should mention missing artifacts: %v", err)
		}
	})

	// Test missing artifact URI
	t.Run("MissingArtifactURI", func(t *testing.T) {
		catalog := &types.ModelsCatalog{
			Source: "Test Source",
			Models: []types.CatalogMetadata{
				{
					Name: stringPtr("Model With Invalid Artifact"),
					Artifacts: []types.CatalogOCIArtifact{
						{
							URI: "",
						},
					},
				},
			},
		}

		err := validateStaticCatalog(catalog)
		if err == nil {
			t.Error("Expected error for missing artifact URI")
		}
		if !strings.Contains(err.Error(), "missing required 'uri' field") {
			t.Errorf("Error should mention missing URI field: %v", err)
		}
	})
}

func TestCreateModelsCatalogWithStatic(t *testing.T) {
	// Create temporary directory structure for testing
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	dataDir := filepath.Join(tmpDir, "data")

	// Create test dynamic model metadata files
	testDynamicModels := []struct {
		path     string
		metadata types.ExtractedMetadata
	}{
		{
			path: "dynamic-model1/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name:        stringPtr("Dynamic Model 1"),
				Provider:    stringPtr("Dynamic Provider"),
				Description: stringPtr("A dynamic test model"),
				License:     stringPtr("Apache-2.0"),
				Language:    []string{"en"},
				Tasks:       []string{"text-generation"},
				Tags:        []string{"dynamic"},
			},
		},
	}

	// Create dynamic model files
	for _, model := range testDynamicModels {
		fullPath := filepath.Join(outputDir, model.path)
		dir := filepath.Dir(fullPath)

		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}

		data, err := yaml.Marshal(model.metadata)
		if err != nil {
			t.Fatalf("Failed to marshal test metadata: %v", err)
		}

		err = os.WriteFile(fullPath, data, 0644)
		if err != nil {
			t.Fatalf("Failed to create test metadata file %s: %v", fullPath, err)
		}
	}

	// Create data directory for catalog output
	err := os.MkdirAll(dataDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Change to temp directory
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

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test with static models
	t.Run("WithStaticModels", func(t *testing.T) {
		staticModels := []types.CatalogMetadata{
			{
				Name:        stringPtr("Static Model 1"),
				Provider:    stringPtr("Static Provider"),
				Description: stringPtr("A static test model"),
				License:     stringPtr("MIT"),
				Language:    []string{"en"},
				Tasks:       []string{"text-classification"},
				Artifacts: []types.CatalogOCIArtifact{
					{
						URI: "oci://example.com/static-model:1.0",
					},
				},
			},
			{
				Name:        stringPtr("Static Model 2"),
				Provider:    stringPtr("Another Static Provider"),
				Description: stringPtr("Another static model"),
				Artifacts: []types.CatalogOCIArtifact{
					{
						URI: "oci://example.com/static-model2:1.0",
					},
				},
			},
		}

		testCatalogPath := filepath.Join("data", "test-catalog-with-static.yaml")
		err := CreateModelsCatalogWithStatic("output", testCatalogPath, staticModels)
		if err != nil {
			t.Fatalf("CreateModelsCatalogWithStatic failed: %v", err)
		}

		// Verify catalog was created
		catalogPath := filepath.Join(tmpDir, "data", "test-catalog-with-static.yaml")
		if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
			t.Fatal("Catalog file was not created")
		}

		catalogData, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatalf("Failed to read catalog file: %v", err)
		}

		var catalog types.ModelsCatalog
		err = yaml.Unmarshal(catalogData, &catalog)
		if err != nil {
			t.Fatalf("Failed to parse catalog YAML: %v", err)
		}

		// Should have 3 models total (1 dynamic + 2 static)
		if len(catalog.Models) != 3 {
			t.Errorf("Expected 3 models in catalog, got %d", len(catalog.Models))
		}

		// Check that all expected models are present
		modelNames := make([]string, len(catalog.Models))
		for i, model := range catalog.Models {
			if model.Name != nil {
				modelNames[i] = *model.Name
			}
		}

		// Sort collected names and compare with expected sorted list
		sort.Strings(modelNames)
		expected := []string{"Dynamic Model 1", "Static Model 1", "Static Model 2"}
		sort.Strings(expected)
		if !reflect.DeepEqual(modelNames, expected) {
			t.Errorf("Expected names %v, got %v", expected, modelNames)
		}
	})

	// Test with no static models (should work like CreateModelsCatalog)
	t.Run("WithoutStaticModels", func(t *testing.T) {
		testCatalogPath := filepath.Join("data", "test-catalog-no-static.yaml")
		err := CreateModelsCatalogWithStatic("output", testCatalogPath, []types.CatalogMetadata{})
		if err != nil {
			t.Fatalf("CreateModelsCatalogWithStatic failed: %v", err)
		}

		// Verify catalog was created
		catalogPath := filepath.Join(tmpDir, "data", "test-catalog-no-static.yaml")
		catalogData, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatalf("Failed to read catalog file: %v", err)
		}

		var catalog types.ModelsCatalog
		err = yaml.Unmarshal(catalogData, &catalog)
		if err != nil {
			t.Fatalf("Failed to parse catalog YAML: %v", err)
		}

		// Should have 1 model (just the dynamic model)
		if len(catalog.Models) != 1 {
			t.Errorf("Expected 1 model in catalog, got %d", len(catalog.Models))
		}

		if catalog.Models[0].Name == nil || *catalog.Models[0].Name != "Dynamic Model 1" {
			t.Error("Expected single model to be 'Dynamic Model 1'")
		}
	})
}

// TestDeduplicateModels tests the model deduplication logic
func TestDeduplicateModels(t *testing.T) {
	testCases := []struct {
		name          string
		dynamicModels []types.CatalogMetadata
		staticModels  []types.CatalogMetadata
		expectedNames []string
		description   string
	}{
		{
			name: "NoOverlap",
			dynamicModels: []types.CatalogMetadata{
				{Name: stringPtr("Dynamic Model A")},
				{Name: stringPtr("Dynamic Model B")},
			},
			staticModels: []types.CatalogMetadata{
				{Name: stringPtr("Static Model C")},
				{Name: stringPtr("Static Model D")},
			},
			expectedNames: []string{"Dynamic Model A", "Dynamic Model B", "Static Model C", "Static Model D"},
			description:   "All models should be included when there are no name conflicts",
		},
		{
			name: "WithOverlap",
			dynamicModels: []types.CatalogMetadata{
				{Name: stringPtr("Model A")},
				{Name: stringPtr("Model B")},
			},
			staticModels: []types.CatalogMetadata{
				{Name: stringPtr("Model B")}, // Duplicate of dynamic model
				{Name: stringPtr("Model C")},
			},
			expectedNames: []string{"Model A", "Model B", "Model C"},
			description:   "Dynamic models should take precedence over static models with same name",
		},
		{
			name: "DuplicateStaticModels",
			dynamicModels: []types.CatalogMetadata{
				{Name: stringPtr("Dynamic Model")},
			},
			staticModels: []types.CatalogMetadata{
				{Name: stringPtr("Static Model A")},
				{Name: stringPtr("Static Model A")}, // Duplicate within static
				{Name: stringPtr("Static Model B")},
			},
			expectedNames: []string{"Dynamic Model", "Static Model A", "Static Model B"},
			description:   "Duplicate static models should be filtered out",
		},
		{
			name:          "EmptyDynamicModels",
			dynamicModels: []types.CatalogMetadata{},
			staticModels: []types.CatalogMetadata{
				{Name: stringPtr("Static Only A")},
				{Name: stringPtr("Static Only B")},
			},
			expectedNames: []string{"Static Only A", "Static Only B"},
			description:   "Should handle case with no dynamic models",
		},
		{
			name: "EmptyStaticModels",
			dynamicModels: []types.CatalogMetadata{
				{Name: stringPtr("Dynamic Only A")},
				{Name: stringPtr("Dynamic Only B")},
			},
			staticModels:  []types.CatalogMetadata{},
			expectedNames: []string{"Dynamic Only A", "Dynamic Only B"},
			description:   "Should handle case with no static models",
		},
		{
			name: "NilNames",
			dynamicModels: []types.CatalogMetadata{
				{Name: stringPtr("Valid Dynamic")},
				{Name: nil}, // Invalid model with nil name
			},
			staticModels: []types.CatalogMetadata{
				{Name: nil}, // Invalid model with nil name
				{Name: stringPtr("Valid Static")},
			},
			expectedNames: []string{"Valid Dynamic", "Valid Static"},
			description:   "Should handle models with nil names gracefully",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := deduplicateModels(tc.dynamicModels, tc.staticModels)

			// Extract names from result for comparison
			var actualNames []string
			for _, model := range result {
				if model.Name != nil {
					actualNames = append(actualNames, *model.Name)
				}
			}

			// Verify expected number of models
			if len(actualNames) != len(tc.expectedNames) {
				t.Errorf("%s: Expected %d models, got %d", tc.description, len(tc.expectedNames), len(actualNames))
				t.Errorf("Expected names: %v", tc.expectedNames)
				t.Errorf("Actual names: %v", actualNames)
				return
			}

			// Verify all expected names are present in order
			for i, expectedName := range tc.expectedNames {
				if i >= len(actualNames) || actualNames[i] != expectedName {
					t.Errorf("%s: Expected model at index %d to be '%s', got '%s'", tc.description, i, expectedName, actualNames[i])
				}
			}

			// Verify no duplicate names in result
			nameCount := make(map[string]int)
			for _, name := range actualNames {
				nameCount[name]++
				if nameCount[name] > 1 {
					t.Errorf("%s: Found duplicate model name '%s' in result", tc.description, name)
				}
			}
		})
	}
}

// TestCreateModelsCatalogWithStaticDeduplication tests end-to-end deduplication
func TestCreateModelsCatalogWithStaticDeduplication(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	// Create dynamic model metadata that will conflict with static
	conflictingModel := types.ExtractedMetadata{
		Name:        stringPtr("Conflicting Model"),
		Provider:    stringPtr("Dynamic Provider"),
		Description: stringPtr("This model exists in both dynamic and static"),
		License:     stringPtr("Apache-2.0"),
		Language:    []string{"en"},
		Tasks:       []string{"text-generation"},
		Tags:        []string{"validated"},
	}

	uniqueDynamicModel := types.ExtractedMetadata{
		Name:        stringPtr("Unique Dynamic Model"),
		Provider:    stringPtr("Dynamic Provider"),
		Description: stringPtr("This model only exists in dynamic"),
		License:     stringPtr("MIT"),
		Language:    []string{"en"},
		Tasks:       []string{"text-classification"},
		Tags:        []string{},
	}

	// Create metadata files
	modelsToCreate := []struct {
		path     string
		metadata types.ExtractedMetadata
	}{
		{"conflicting-model/models/metadata.yaml", conflictingModel},
		{"unique-dynamic/models/metadata.yaml", uniqueDynamicModel},
	}

	for _, model := range modelsToCreate {
		fullPath := filepath.Join(outputDir, model.path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		yamlData, err := yaml.Marshal(&model.metadata)
		if err != nil {
			t.Fatalf("Failed to marshal metadata: %v", err)
		}

		err = os.WriteFile(fullPath, yamlData, 0644)
		if err != nil {
			t.Fatalf("Failed to write metadata file: %v", err)
		}
	}

	// Create static models with one conflict and one unique
	staticModels := []types.CatalogMetadata{
		{
			Name:        stringPtr("Conflicting Model"), // Same name as dynamic model
			Provider:    stringPtr("Static Provider"),
			Description: stringPtr("This should be ignored due to conflict"),
			License:     stringPtr("GPL-3.0"),
			Artifacts: []types.CatalogOCIArtifact{
				{URI: "registry.example.com/static/conflicting-model:latest"},
			},
		},
		{
			Name:        stringPtr("Unique Static Model"),
			Provider:    stringPtr("Static Provider"),
			Description: stringPtr("This model only exists in static"),
			License:     stringPtr("BSD-3-Clause"),
			Artifacts: []types.CatalogOCIArtifact{
				{URI: "registry.example.com/static/unique-model:latest"},
			},
		},
	}

	// Create catalog with deduplication
	catalogPath := filepath.Join(tmpDir, "models-catalog-dedup.yaml")
	err := CreateModelsCatalogWithStatic(outputDir, catalogPath, staticModels)
	if err != nil {
		t.Fatalf("Failed to create catalog with static models: %v", err)
	}

	// Read and verify the catalog
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog file: %v", err)
	}

	var catalog types.ModelsCatalog
	err = yaml.Unmarshal(catalogData, &catalog)
	if err != nil {
		t.Fatalf("Failed to parse catalog YAML: %v", err)
	}

	// Should have 3 models: 2 dynamic + 1 unique static (1 static model deduplicated)
	if len(catalog.Models) != 3 {
		t.Errorf("Expected 3 models in catalog after deduplication, got %d", len(catalog.Models))
	}

	// Verify the conflicting model has dynamic provider (not static)
	var conflictingModelFound bool
	for _, model := range catalog.Models {
		if model.Name != nil && *model.Name == "Conflicting Model" {
			conflictingModelFound = true
			if model.Provider == nil || *model.Provider != "Dynamic Provider" {
				t.Errorf("Expected conflicting model to have 'Dynamic Provider', got %v", model.Provider)
			}
		}
	}

	if !conflictingModelFound {
		t.Error("Conflicting model not found in catalog")
	}

	// Verify unique static model is present
	var uniqueStaticFound bool
	for _, model := range catalog.Models {
		if model.Name != nil && *model.Name == "Unique Static Model" {
			uniqueStaticFound = true
			break
		}
	}

	if !uniqueStaticFound {
		t.Error("Unique static model should be present in catalog")
	}
}

// TestEmbeddedAssets tests that the go:embed assets work correctly
func TestEmbeddedAssets(t *testing.T) {
	testCases := []struct {
		name                string
		tags                []string
		shouldHaveValidated bool
	}{
		{
			name:                "validated tag present",
			tags:                []string{"validated", "featured"},
			shouldHaveValidated: true,
		},
		{
			name:                "no validated tag",
			tags:                []string{"featured", "popular"},
			shouldHaveValidated: false,
		},
		{
			name:                "empty tags",
			tags:                []string{},
			shouldHaveValidated: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logo := determineLogo(tc.tags)
			if logo == nil {
				t.Errorf("Expected logo, got nil")
				return
			}

			// Verify it's a proper data URI
			if !strings.HasPrefix(*logo, "data:image/svg+xml;base64,") {
				t.Errorf("Expected data URI to start with 'data:image/svg+xml;base64,', got %s", *logo)
			}

			// Verify we get substantial content (embedded assets should work)
			if len(*logo) < 100 {
				t.Errorf("Expected substantial logo content from embedded assets, got short string: %s", *logo)
			}

			// Verify the base64 content is valid
			base64Part := strings.TrimPrefix(*logo, "data:image/svg+xml;base64,")
			_, err := base64.StdEncoding.DecodeString(base64Part)
			if err != nil {
				t.Errorf("Failed to decode base64 content: %v", err)
			}
		})
	}
}

// TestEmbeddedAssetsDirectly tests reading assets directly from embedded filesystem
func TestEmbeddedAssetsDirectly(t *testing.T) {
	testFiles := []string{
		"assets/catalog-validated_model.svg",
		"assets/catalog-model.svg",
	}

	for _, filePath := range testFiles {
		t.Run(filePath, func(t *testing.T) {
			content, err := assetsFS.ReadFile(filePath)
			if err != nil {
				t.Errorf("Failed to read embedded file %s: %v", filePath, err)
				return
			}

			if len(content) == 0 {
				t.Errorf("Embedded file %s is empty", filePath)
			}

			// Verify it looks like SVG content
			contentStr := string(content)
			if !strings.Contains(contentStr, "<svg") {
				t.Errorf("Embedded file %s doesn't appear to contain SVG content", filePath)
			}
		})
	}
}

// TestGetFallbackLogo tests the fallback logo functionality
func TestGetFallbackLogo(t *testing.T) {
	logo := getFallbackLogo()

	if logo == nil {
		t.Fatal("getFallbackLogo returned nil")
	}

	// Verify it's a valid data URI
	if !strings.HasPrefix(*logo, "data:image/svg+xml;base64,") {
		t.Errorf("Expected valid data URI, got %s", *logo)
	}

	// Verify we get substantial content
	if len(*logo) < 100 {
		t.Errorf("Expected substantial fallback logo content, got short string: %s", *logo)
	}

	// Decode and verify the content contains SVG
	base64Part := strings.TrimPrefix(*logo, "data:image/svg+xml;base64,")
	svgContent, err := base64.StdEncoding.DecodeString(base64Part)
	if err != nil {
		t.Errorf("Failed to decode fallback logo base64: %v", err)
	}

	svgStr := string(svgContent)
	if !strings.Contains(svgStr, "<svg") || !strings.Contains(svgStr, "<circle") || !strings.Contains(svgStr, "<text") {
		t.Errorf("Fallback logo doesn't contain expected SVG elements")
	}

	// Verify it contains the "M" text for model
	if !strings.Contains(svgStr, ">M<") {
		t.Errorf("Fallback logo should contain 'M' text")
	}
}

// TestDetermineLogoResilience ensures determineLogo never returns nil and always yields a valid data URI.
func TestDetermineLogoResilience(t *testing.T) {
	// Test that the encodeSVGToDataURI function handles asset failures gracefully
	// by returning a fallback logo instead of nil

	// This tests the interface without relying on internal main package functions
	// We verify that determineLogo always returns a valid logo (never nil)
	testCases := [][]string{
		{"validated"},
		{"featured"},
		{},
		nil,
	}

	for _, tags := range testCases {
		logo := determineLogo(tags)
		if logo == nil {
			t.Errorf("determineLogo should never return nil for tags %v", tags)
			continue
		}

		if !strings.HasPrefix(*logo, "data:image/svg+xml;base64,") {
			t.Errorf("determineLogo should always return valid data URI for tags %v", tags)
		}
	}
}
