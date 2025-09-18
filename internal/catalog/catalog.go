package catalog

import (
	"embed"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

//go:embed assets/*.svg
var assetsFS embed.FS

// LoadStaticCatalogs loads and validates static catalog files from the provided file paths.
// It reads YAML files containing pre-defined model metadata and returns a consolidated
// slice of CatalogMetadata. Files that don't exist or fail validation are skipped with warnings.
//
// Parameters:
//   - filePaths: slice of file paths to static catalog YAML files
//
// Returns:
//   - []types.CatalogMetadata: consolidated models from all valid static catalogs
//   - error: only returns error for critical failures, individual file errors are logged
func LoadStaticCatalogs(filePaths []string) ([]types.CatalogMetadata, error) {
	var allStaticModels []types.CatalogMetadata

	for _, filePath := range filePaths {
		log.Printf("  Loading static catalog: %s", filePath)

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("  Warning: Static catalog file not found: %s", filePath)
			continue
		}

		// Read the file
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("  Error reading static catalog file %s: %v", filePath, err)
			continue
		}

		// Parse the YAML
		var staticCatalog types.ModelsCatalog
		err = yaml.Unmarshal(data, &staticCatalog)
		if err != nil {
			log.Printf("  Error parsing static catalog file %s: %v", filePath, err)
			continue
		}

		// Validate the catalog structure
		if err := validateStaticCatalog(&staticCatalog); err != nil {
			log.Printf("  Error validating static catalog file %s: %v", filePath, err)
			continue
		}

		// Add models from this catalog
		allStaticModels = append(allStaticModels, staticCatalog.Models...)
		log.Printf("  Successfully loaded %d models from %s", len(staticCatalog.Models), filePath)
	}

	log.Printf("Total static models loaded: %d", len(allStaticModels))
	return allStaticModels, nil
}

// validateStaticCatalog validates the structural integrity of a static catalog.
// It ensures required fields are present and properly formatted according to the
// ModelsCatalog schema requirements.
//
// Parameters:
//   - catalog: pointer to ModelsCatalog structure to validate
//
// Returns:
//   - error: validation error if structure is invalid, nil if valid
func validateStaticCatalog(catalog *types.ModelsCatalog) error {
	if catalog.Source == "" {
		return fmt.Errorf("static catalog missing required 'source' field")
	}

	for i, model := range catalog.Models {
		if model.Name == nil || *model.Name == "" {
			return fmt.Errorf("model at index %d missing required 'name' field", i)
		}

		if len(model.Artifacts) == 0 {
			return fmt.Errorf("model '%s' has no artifacts", *model.Name)
		}

		// Validate each artifact has a URI
		for j, artifact := range model.Artifacts {
			if artifact.URI == "" {
				return fmt.Errorf("model '%s' artifact at index %d missing required 'uri' field", *model.Name, j)
			}
		}
	}

	return nil
}

// CreateModelsCatalogWithStatic generates a comprehensive models catalog by merging
// dynamically extracted metadata with static model definitions. It walks the output
// directory to find metadata.yaml files, converts them to catalog format, merges
// static models (dynamic wins on name clashes), and emits a globally sorted catalog.
//
// Parameters:
//   - outputDir: directory containing extracted model metadata files
//   - catalogPath: output path for the generated models-catalog.yaml file
//   - staticModels: pre-defined static model metadata to include in catalog
//
// Returns:
//   - error: filesystem or marshaling errors, nil on success
func CreateModelsCatalogWithStatic(outputDir, catalogPath string, staticModels []types.CatalogMetadata) error {
	var allModels []types.ExtractedMetadata

	// Find all metadata.yaml files in the specified output directory
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "metadata.yaml" {
			log.Printf("  Processing: %s", path)

			// Read the metadata file
			data, err := os.ReadFile(path)
			if err != nil {
				log.Printf("  Error reading %s: %v", path, err)
				return nil // Continue with other files
			}

			// Parse the YAML
			var metadata types.ExtractedMetadata
			err = yaml.Unmarshal(data, &metadata)
			if err != nil {
				log.Printf("  Error parsing %s: %v", path, err)
				return nil // Continue with other files
			}

			// Add to collection
			allModels = append(allModels, metadata)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking directory: %v", err)
	}

	// Sort models by name for consistent output
	sort.Slice(allModels, func(i, j int) bool {
		nameI := ""
		nameJ := ""
		if allModels[i].Name != nil {
			nameI = *allModels[i].Name
		}
		if allModels[j].Name != nil {
			nameJ = *allModels[j].Name
		}
		return nameI < nameJ
	})

	// Convert dynamic models to catalog metadata (tags mapped to customProperties)
	var catalogModels []types.CatalogMetadata
	for _, model := range allModels {
		catalogModel := convertExtractedToCatalogMetadata(model)
		catalogModels = append(catalogModels, catalogModel)
	}

	// Merge static models with dynamic models using deduplication
	catalogModels = deduplicateModels(catalogModels, staticModels)

	// Globally stable ordering after deduplication
	sort.Slice(catalogModels, func(i, j int) bool {
		var a, b string
		if catalogModels[i].Name != nil {
			a = *catalogModels[i].Name
		}
		if catalogModels[j].Name != nil {
			b = *catalogModels[j].Name
		}
		return a < b
	})

	// Create the catalog structure
	catalog := types.ModelsCatalog{
		Source: "Red Hat",
		Models: catalogModels,
	}

	// Marshal to YAML
	output, err := yaml.Marshal(&catalog)
	if err != nil {
		return fmt.Errorf("error marshaling catalog: %v", err)
	}

	// Write to the specified catalog path
	err = os.WriteFile(catalogPath, output, 0644)
	if err != nil {
		return fmt.Errorf("error writing catalog file: %v", err)
	}

	log.Printf("Successfully created %s with %d dynamic models and %d static models", catalogPath, len(allModels), len(staticModels))
	return nil
}

// CreateModelsCatalog generates a models catalog from dynamically extracted metadata only.
// This function provides backward compatibility for workflows that don't use static catalogs.
// It internally calls CreateModelsCatalogWithStatic with an empty static models slice.
//
// Parameters:
//   - outputDir: directory containing extracted model metadata files
//   - catalogPath: output path for the generated models-catalog.yaml file
//
// Returns:
//   - error: filesystem or marshaling errors, nil on success
func CreateModelsCatalog(outputDir, catalogPath string) error {
	return CreateModelsCatalogWithStatic(outputDir, catalogPath, []types.CatalogMetadata{})
}

// convertExtractedToCatalogMetadata transforms ExtractedMetadata (internal format)
// to CatalogMetadata (public catalog format). It handles timestamp conversions,
// artifact format transformations, and tag-to-customProperties mapping.
//
// Parameters:
//   - model: ExtractedMetadata structure from modelcard processing
//
// Returns:
//   - types.CatalogMetadata: transformed metadata suitable for catalog output
func convertExtractedToCatalogMetadata(model types.ExtractedMetadata) types.CatalogMetadata {
	// Convert timestamps to strings and use artifact values when model values are null
	createTimeStr := convertTimestampToString(model.CreateTimeSinceEpoch)
	lastUpdateTimeStr := convertTimestampToString(model.LastUpdateTimeSinceEpoch)

	// If model timestamps are null, try to use values from the first artifact
	if createTimeStr == nil && len(model.Artifacts) > 0 {
		if model.Artifacts[0].CreateTimeSinceEpoch != nil {
			createTimeStr = convertTimestampToString(model.Artifacts[0].CreateTimeSinceEpoch)
		}
	}
	if lastUpdateTimeStr == nil && len(model.Artifacts) > 0 {
		if model.Artifacts[0].LastUpdateTimeSinceEpoch != nil {
			lastUpdateTimeStr = convertTimestampToString(model.Artifacts[0].LastUpdateTimeSinceEpoch)
		}
	}

	// Convert artifacts to catalog format with string timestamps
	var catalogArtifacts []types.CatalogOCIArtifact
	for _, artifact := range model.Artifacts {
		catalogArtifact := types.CatalogOCIArtifact{
			URI:                      artifact.URI,
			CreateTimeSinceEpoch:     convertTimestampToString(artifact.CreateTimeSinceEpoch),
			LastUpdateTimeSinceEpoch: convertTimestampToString(artifact.LastUpdateTimeSinceEpoch),
			CustomProperties:         artifact.CustomProperties,
		}
		catalogArtifacts = append(catalogArtifacts, catalogArtifact)
	}

	// Convert tags to customProperties
	customProps := convertTagsToCustomProperties(model.Tags)

	return types.CatalogMetadata{
		Name:                     model.Name,
		Provider:                 model.Provider,
		Description:              model.Description,
		Readme:                   model.Readme,
		Language:                 model.Language,
		License:                  model.License,
		LicenseLink:              model.LicenseLink,
		Tasks:                    model.Tasks,
		CreateTimeSinceEpoch:     createTimeStr,
		LastUpdateTimeSinceEpoch: lastUpdateTimeStr,
		CustomProperties:         customProps,
		Artifacts:                catalogArtifacts,
		Logo:                     determineLogo(model.Tags),
	}
}

// convertTimestampToString safely converts Unix epoch timestamps to string format.
// This function handles nil pointers gracefully and maintains type safety for
// optional timestamp fields in the catalog format.
//
// Parameters:
//   - timestamp: pointer to int64 Unix epoch timestamp, may be nil
//
// Returns:
//   - *string: string representation of timestamp, nil if input is nil
func convertTimestampToString(timestamp *int64) *string {
	if timestamp == nil {
		return nil
	}
	str := strconv.FormatInt(*timestamp, 10)
	return &str
}

// convertTagsToCustomProperties transforms model tags into the catalog's customProperties
// format. Each tag becomes a MetadataStringValue entry in the customProperties map.
// Empty tags are filtered out to maintain data quality.
//
// Parameters:
//   - tags: slice of string tags from model metadata
//
// Returns:
//   - map[string]types.MetadataValue: customProperties map suitable for catalog format
func convertTagsToCustomProperties(tags []string) map[string]types.MetadataValue {
	customProps := make(map[string]types.MetadataValue)

	for _, tag := range tags {
		if tag != "" { // Skip empty tags
			customProps[tag] = types.MetadataValue{
				MetadataType: "MetadataStringValue",
				StringValue:  "",
			}
		}
	}

	return customProps
}

// determineLogo selects the appropriate logo based on model validation status.
// Models with "validated" tag receive the validated model logo, while others
// get the standard model logo. Returns a base64-encoded data URI for embedding.
//
// Parameters:
//   - tags: slice of model tags to examine for validation status
//
// Returns:
//   - *string: base64-encoded data URI of the selected logo, nil if encoding fails
func determineLogo(tags []string) *string {
	var svgPath string

	// Check if the model has the "validated" label
	for _, raw := range tags {
		tag := strings.TrimSpace(strings.ToLower(raw))
		if tag == "validated" {
			svgPath = "assets/catalog-validated_model.svg"
			break
		}
	}

	// Default logo for non-validated models
	if svgPath == "" {
		svgPath = "assets/catalog-model.svg"
	}

	// Read and encode the SVG file
	dataUri := encodeSVGToDataURI(svgPath)
	return dataUri
}

// encodeSVGToDataURI reads an SVG file from the embedded filesystem and converts it to
// a base64-encoded data URI suitable for embedding in web contexts. Uses go:embed for
// reliable asset access independent of working directory. Provides fallback logo if
// embedded assets fail to load.
//
// Parameters:
//   - svgPath: embedded filesystem path to the SVG file to encode
//
// Returns:
//   - *string: base64-encoded data URI, never nil (provides fallback on failures)
func encodeSVGToDataURI(svgPath string) *string {
	// Read the SVG file from embedded filesystem
	svgContent, err := assetsFS.ReadFile(svgPath)
	if err != nil {
		log.Printf("Warning: Failed to read embedded SVG file %s: %v", svgPath, err)
		log.Printf("Using fallback logo due to asset loading failure")
		return getFallbackLogo()
	}

	// Encode to base64
	base64Content := base64.StdEncoding.EncodeToString(svgContent)

	// Create data URI
	dataUri := "data:image/svg+xml;base64," + base64Content
	return &dataUri
}

// getFallbackLogo provides a minimal SVG logo when embedded assets fail to load.
// This ensures logo field is never nil and maintains consistent catalog structure.
//
// Returns:
//   - *string: base64-encoded fallback SVG data URI
func getFallbackLogo() *string {
	// Minimal fallback SVG - a simple gray circle with "M" text
	fallbackSVG := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">
		<circle cx="50" cy="50" r="40" fill="#888" stroke="#333" stroke-width="2"/>
		<text x="50" y="60" font-family="Arial, sans-serif" font-size="36" font-weight="bold" 
			  text-anchor="middle" fill="white">M</text>
	</svg>`

	base64Content := base64.StdEncoding.EncodeToString([]byte(fallbackSVG))
	dataUri := "data:image/svg+xml;base64," + base64Content
	return &dataUri
}

// deduplicateModels merges dynamic and static models while preventing duplicates.
// Dynamic models take precedence over static models when names match. This ensures
// that automatically extracted metadata is preferred over pre-defined static data.
//
// Parameters:
//   - dynamicModels: models extracted from container registries (higher precedence)
//   - staticModels: models from static catalog files (lower precedence)
//
// Returns:
//   - []types.CatalogMetadata: deduplicated list with dynamic models first, unique static models appended
func deduplicateModels(dynamicModels, staticModels []types.CatalogMetadata) []types.CatalogMetadata {
	// Create map of normalized dynamic model names for efficient lookup
	dynamicNameMap := make(map[string]bool)
	for _, model := range dynamicModels {
		if model.Name != nil {
			k := strings.ToLower(strings.TrimSpace(*model.Name))
			if k != "" {
				dynamicNameMap[k] = true
			}
		}
	}

	// Start with all dynamic models
	result := make([]types.CatalogMetadata, len(dynamicModels))
	copy(result, dynamicModels)

	// Add static models only if their name doesn't already exist in dynamic models
	for _, staticModel := range staticModels {
		if staticModel.Name != nil {
			k := strings.ToLower(strings.TrimSpace(*staticModel.Name))
			if k == "" {
				continue
			}
			if !dynamicNameMap[k] {
				result = append(result, staticModel)
				// Track this name to prevent duplicates within static models
				dynamicNameMap[k] = true
			} else {
				log.Printf("  Skipping duplicate static model: %s (dynamic version takes precedence)", *staticModel.Name)
			}
		}
	}

	return result
}
