package catalog

import (
	"encoding/base64"
	"os"
	"path/filepath"
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
	err = CreateModelsCatalog()
	if err != nil {
		t.Fatalf("CreateModelsCatalog failed: %v", err)
	}

	// Verify the catalog file was created
	catalogPath := filepath.Join(tmpDir, "data", "models-catalog.yaml")
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
	err = CreateModelsCatalog()
	if err != nil {
		t.Fatalf("CreateModelsCatalog failed with empty directory: %v", err)
	}

	// Verify catalog file was created with empty models list
	catalogPath := filepath.Join(tmpDir, "data", "models-catalog.yaml")
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
	err = CreateModelsCatalog()
	if err != nil {
		// The function should handle missing output directory gracefully
		t.Logf("CreateModelsCatalog returned error (expected for missing output dir): %v", err)
		return
	}

	// If it succeeded, should create catalog with empty models list
	catalogPath := filepath.Join(tmpDir, "data", "models-catalog.yaml")
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
	err = CreateModelsCatalog()
	if err != nil {
		t.Fatalf("CreateModelsCatalog failed: %v", err)
	}

	// Verify catalog was still created (invalid files are skipped)
	catalogPath := filepath.Join(tmpDir, "data", "models-catalog.yaml")
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
	err = CreateModelsCatalog()
	if err != nil {
		t.Fatalf("CreateModelsCatalog failed: %v", err)
	}

	// Verify catalog was created
	catalogPath := filepath.Join(tmpDir, "data", "models-catalog.yaml")
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
