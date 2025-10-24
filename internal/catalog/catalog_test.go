package catalog

import (
	"encoding/base64"
	"os"
	"path/filepath"
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
				Artifacts: []types.OCIArtifact{
					{
						URI: "oci://registry.example.com/test-model:1.0",
						CustomProperties: map[string]interface{}{
							"source": map[string]interface{}{
								"string_value": "registry.example.com",
							},
							"type": map[string]interface{}{
								"string_value": "modelcar",
							},
							"simple_prop": "simple_value",
						},
					},
				},
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

	// Verify that artifacts CustomProperties have metadataType
	if len(model1.Artifacts) > 0 {
		artifact := model1.Artifacts[0]
		if artifact.CustomProperties != nil {
			// Check source property
			if sourceVal, exists := artifact.CustomProperties["source"]; exists {
				sourceMap, ok := sourceVal.(map[string]interface{})
				if !ok {
					t.Error("Artifact source property should be a map")
				} else {
					if metadataType, hasType := sourceMap["metadataType"]; !hasType || metadataType != "MetadataStringValue" {
						t.Errorf("Artifact source property should have metadataType 'MetadataStringValue', got %v", metadataType)
					}
					if stringValue, hasValue := sourceMap["string_value"]; !hasValue || stringValue != "registry.example.com" {
						t.Errorf("Artifact source property should have string_value 'registry.example.com', got %v", stringValue)
					}
				}
			}

			// Check simple_prop (should be converted from simple string to MetadataValue format)
			if simplePropVal, exists := artifact.CustomProperties["simple_prop"]; exists {
				simplePropMap, ok := simplePropVal.(map[string]interface{})
				if !ok {
					t.Error("Artifact simple_prop property should be a map")
				} else {
					if metadataType, hasType := simplePropMap["metadataType"]; !hasType || metadataType != "MetadataStringValue" {
						t.Errorf("Artifact simple_prop property should have metadataType 'MetadataStringValue', got %v", metadataType)
					}
					if stringValue, hasValue := simplePropMap["string_value"]; !hasValue || stringValue != "simple_value" {
						t.Errorf("Artifact simple_prop property should have string_value 'simple_value', got %v", stringValue)
					}
				}
			}
		}
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

// TestLogoAssignment tests that logos are correctly assigned based on validation labels
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

	// Create assets directory and test SVG files
	assetsDir := filepath.Join(tmpDir, "assets")
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create assets directory: %v", err)
	}

	// Create test SVG content
	validatedSVG := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40" fill="green"/></svg>`
	modelSVG := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40" fill="blue"/></svg>`

	err = os.WriteFile(filepath.Join(assetsDir, "catalog-validated_model.svg"), []byte(validatedSVG), 0644)
	if err != nil {
		t.Fatalf("Failed to create validated SVG file: %v", err)
	}

	err = os.WriteFile(filepath.Join(assetsDir, "catalog-model.svg"), []byte(modelSVG), 0644)
	if err != nil {
		t.Fatalf("Failed to create model SVG file: %v", err)
	}

	// Calculate expected base64 data URIs
	validatedDataURI := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(validatedSVG))
	modelDataURI := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(modelSVG))

	// Test models with different validation labels
	testModels := []struct {
		path         string
		metadata     types.ExtractedMetadata
		expectedLogo string
	}{
		{
			path: "validated-model/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Validated Model"),
				Tags: []string{"validated", "featured"},
			},
			expectedLogo: validatedDataURI,
		},
		{
			path: "non-validated-model/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Non-Validated Model"),
				Tags: []string{"featured"},
			},
			expectedLogo: modelDataURI,
		},
		{
			path: "model-with-only-validated/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Model With Only Validated"),
				Tags: []string{"validated"},
			},
			expectedLogo: validatedDataURI,
		},
		{
			path: "model-no-tags/models/metadata.yaml",
			metadata: types.ExtractedMetadata{
				Name: stringPtr("Model With No Tags"),
				Tags: []string{},
			},
			expectedLogo: modelDataURI,
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

	// Verify expected logos
	expectedLogos := map[string]string{
		"Validated Model":           validatedDataURI,
		"Non-Validated Model":       modelDataURI,
		"Model With Only Validated": validatedDataURI,
		"Model With No Tags":        modelDataURI,
	}

	for modelName, expectedLogo := range expectedLogos {
		actualLogo, exists := modelLogoMap[modelName]
		if !exists {
			t.Errorf("Model %s not found in catalog", modelName)
			continue
		}
		if actualLogo != expectedLogo {
			t.Errorf("Model %s: expected logo %s, got %s", modelName, expectedLogo, actualLogo)
		}
	}
}

// TestDetermineLogo tests the logo determination logic directly
func TestDetermineLogo(t *testing.T) {
	// Create temporary directory with SVG files for testing
	tmpDir := t.TempDir()

	// Create assets directory and test SVG files
	assetsDir := filepath.Join(tmpDir, "assets")
	err := os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create assets directory: %v", err)
	}

	// Create test SVG content
	validatedSVG := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40" fill="green"/></svg>`
	modelSVG := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40" fill="blue"/></svg>`

	err = os.WriteFile(filepath.Join(assetsDir, "catalog-validated_model.svg"), []byte(validatedSVG), 0644)
	if err != nil {
		t.Fatalf("Failed to create validated SVG file: %v", err)
	}

	err = os.WriteFile(filepath.Join(assetsDir, "catalog-model.svg"), []byte(modelSVG), 0644)
	if err != nil {
		t.Fatalf("Failed to create model SVG file: %v", err)
	}

	// Calculate expected base64 data URIs
	validatedDataURI := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(validatedSVG))
	modelDataURI := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(modelSVG))

	// Change to the temp directory so the function can find the assets
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

	testCases := []struct {
		name         string
		tags         []string
		expectedLogo string
	}{
		{
			name:         "validated tag present",
			tags:         []string{"validated", "featured"},
			expectedLogo: validatedDataURI,
		},
		{
			name:         "only validated tag",
			tags:         []string{"validated"},
			expectedLogo: validatedDataURI,
		},
		{
			name:         "no validated tag",
			tags:         []string{"featured", "popular"},
			expectedLogo: modelDataURI,
		},
		{
			name:         "empty tags",
			tags:         []string{},
			expectedLogo: modelDataURI,
		},
		{
			name:         "nil tags",
			tags:         nil,
			expectedLogo: modelDataURI,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logo := determineLogo(tc.tags)
			if logo == nil {
				t.Fatal("determineLogo returned nil")
			}
			if *logo != tc.expectedLogo {
				t.Errorf("Expected logo %s, got %s", tc.expectedLogo, *logo)
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

func TestConvertCustomPropertiesToMetadataValue(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "properties without metadataType",
			input: map[string]interface{}{
				"source": map[string]interface{}{
					"string_value": "registry.redhat.io",
				},
				"type": map[string]interface{}{
					"string_value": "modelcar",
				},
			},
			expected: map[string]interface{}{
				"source": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "registry.redhat.io",
				},
				"type": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "modelcar",
				},
			},
		},
		{
			name: "properties already with metadataType",
			input: map[string]interface{}{
				"source": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "registry.redhat.io",
				},
			},
			expected: map[string]interface{}{
				"source": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "registry.redhat.io",
				},
			},
		},
		{
			name: "simple string values",
			input: map[string]interface{}{
				"simple_key": "simple_value",
			},
			expected: map[string]interface{}{
				"simple_key": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "simple_value",
				},
			},
		},
		{
			name: "mixed format properties",
			input: map[string]interface{}{
				"with_metadata": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "existing",
				},
				"without_metadata": map[string]interface{}{
					"string_value": "needs_metadata",
				},
				"simple": "raw_string",
			},
			expected: map[string]interface{}{
				"with_metadata": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "existing",
				},
				"without_metadata": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "needs_metadata",
				},
				"simple": map[string]interface{}{
					"metadataType": "MetadataStringValue",
					"string_value": "raw_string",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := convertCustomPropertiesToMetadataValue(tc.input)

			// Deep comparison
			if tc.expected == nil {
				if result != nil {
					t.Errorf("Expected nil result, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil result, got nil")
				return
			}

			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d properties, got %d", len(tc.expected), len(result))
				return
			}

			for key, expectedValue := range tc.expected {
				actualValue, exists := result[key]
				if !exists {
					t.Errorf("Expected key '%s' not found in result", key)
					continue
				}

				// Compare the nested map values
				expectedMap, expectedIsMap := expectedValue.(map[string]interface{})
				actualMap, actualIsMap := actualValue.(map[string]interface{})

				if expectedIsMap != actualIsMap {
					t.Errorf("Key '%s': type mismatch - expected map: %v, actual map: %v", key, expectedIsMap, actualIsMap)
					continue
				}

				if expectedIsMap {
					for nestedKey, nestedExpected := range expectedMap {
						nestedActual, nestedExists := actualMap[nestedKey]
						if !nestedExists {
							t.Errorf("Key '%s.%s': not found in result", key, nestedKey)
							continue
						}
						if nestedActual != nestedExpected {
							t.Errorf("Key '%s.%s': expected '%v', got '%v'", key, nestedKey, nestedExpected, nestedActual)
						}
					}
					for nestedKey := range actualMap {
						if _, exists := expectedMap[nestedKey]; !exists {
							t.Errorf("Key '%s.%s': unexpected key in result", key, nestedKey)
						}
					}
				} else {
					if actualValue != expectedValue {
						t.Errorf("Key '%s': expected '%v', got '%v'", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
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

		// Check that static models are at the end
		modelNames := make([]string, len(catalog.Models))
		for i, model := range catalog.Models {
			if model.Name != nil {
				modelNames[i] = *model.Name
			}
		}

		// The first model should be the dynamic model (after sorting)
		if modelNames[0] != "Dynamic Model 1" {
			t.Errorf("Expected first model to be 'Dynamic Model 1', got '%s'", modelNames[0])
		}

		// Static models should be at the end
		staticFound := 0
		for _, name := range modelNames {
			if name == "Static Model 1" || name == "Static Model 2" {
				staticFound++
			}
		}
		if staticFound != 2 {
			t.Errorf("Expected 2 static models in catalog, found %d", staticFound)
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

func TestConvertExtractedToCatalogMetadata_ValidatedOn(t *testing.T) {
	// Test that ValidatedOn field is properly converted to customProperties
	metadata := types.ExtractedMetadata{
		Name:        stringPtr("Test Model"),
		Provider:    stringPtr("Red Hat"),
		Description: stringPtr("A test model"),
		ValidatedOn: []string{"RHOAI 2.24", "RHAIIS 3.2.1"},
		Tags:        []string{"validated", "featured"},
		Artifacts:   []types.OCIArtifact{},
	}

	result := convertExtractedToCatalogMetadata(metadata)

	// Check that validated_on is in customProperties
	if result.CustomProperties == nil {
		t.Error("Expected CustomProperties to be set")
	} else {
		validatedOnProp, exists := result.CustomProperties["validated_on"]
		if !exists {
			t.Error("Expected validated_on to be in CustomProperties")
		} else {
			expectedValue := types.MetadataValue{
				MetadataType: "MetadataStringValue",
				StringValue:  `["RHOAI 2.24","RHAIIS 3.2.1"]`,
			}
			if validatedOnProp != expectedValue {
				t.Errorf("Expected validated_on customProperty to be %+v, got %+v", expectedValue, validatedOnProp)
			}
		}

		// Check that tags are also converted
		validatedTagProp, exists := result.CustomProperties["validated"]
		if !exists {
			t.Error("Expected validated tag to be in CustomProperties")
		} else {
			expectedTagValue := types.MetadataValue{
				MetadataType: "MetadataStringValue",
				StringValue:  "",
			}
			if validatedTagProp != expectedTagValue {
				t.Errorf("Expected validated tag customProperty to be %+v, got %+v", expectedTagValue, validatedTagProp)
			}
		}
	}
}

func TestConvertExtractedToCatalogMetadata_NoValidatedOn(t *testing.T) {
	// Test that models without ValidatedOn don't have the customProperty
	metadata := types.ExtractedMetadata{
		Name:        stringPtr("Test Model"),
		Provider:    stringPtr("Red Hat"),
		Description: stringPtr("A test model"),
		ValidatedOn: []string{}, // Empty validated_on
		Tags:        []string{"featured"},
		Artifacts:   []types.OCIArtifact{},
	}

	result := convertExtractedToCatalogMetadata(metadata)

	// Check that validated_on is NOT in customProperties
	if result.CustomProperties != nil {
		if _, exists := result.CustomProperties["validated_on"]; exists {
			t.Error("Expected validated_on to NOT be in CustomProperties when ValidatedOn is empty")
		}
	}
}
