package catalog

import (
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
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// LoadStaticCatalogs loads static catalog files and returns their models
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

// validateStaticCatalog validates the structure of a static catalog
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

// CreateModelsCatalogWithStaticFromResults creates a models catalog from specific model results and static models
func CreateModelsCatalogWithStaticFromResults(outputDir, catalogPath string, modelRefs []string, staticModels []types.CatalogMetadata) error {
	var allModels []types.ExtractedMetadata

	// Process only metadata files for models that were processed in the current run
	for _, ref := range modelRefs {
		// Create sanitized directory name for the model (using same logic as main.go)
		sanitizedName := utils.SanitizeManifestRef(ref)
		metadataPath := filepath.Join(outputDir, sanitizedName, "models", "metadata.yaml")
		
		// Check if the metadata file exists
		if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
			log.Printf("  Warning: metadata file not found for %s: %s", ref, metadataPath)
			continue
		}
		
		log.Printf("  Processing: %s", metadataPath)

		// Read the metadata file
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			log.Printf("  Error reading %s: %v", metadataPath, err)
			continue
		}

		// Parse the YAML
		var metadata types.ExtractedMetadata
		err = yaml.Unmarshal(data, &metadata)
		if err != nil {
			log.Printf("  Error parsing %s: %v", metadataPath, err)
			continue
		}

		// Add to collection
		allModels = append(allModels, metadata)
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

	// Convert dynamic models to catalog metadata (excluding tags)
	var catalogModels []types.CatalogMetadata
	for _, model := range allModels {
		catalogModel := convertExtractedToCatalogMetadata(model)
		catalogModels = append(catalogModels, catalogModel)
	}

	// Deduplicate models by consolidating artifacts and merging metadata
	catalogModels = deduplicateAndMergeModels(catalogModels)

	// Merge static models with dynamic models (static models are appended at the end)
	catalogModels = append(catalogModels, staticModels...)

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

// CreateModelsCatalogWithStatic collects all metadata.yaml files, merges with static models, and creates a models-catalog.yaml (backward compatibility)
func CreateModelsCatalogWithStatic(outputDir, catalogPath string, staticModels []types.CatalogMetadata) error {
	var modelRefs []string

	// Find all metadata.yaml files in the specified output directory to maintain backward compatibility
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "metadata.yaml" {
			// Extract model reference from path for backward compatibility
			// Path format: outputDir/sanitized-ref/models/metadata.yaml
			relPath, _ := filepath.Rel(outputDir, path)
			pathParts := strings.Split(relPath, string(filepath.Separator))
			if len(pathParts) >= 2 {
				sanitizedRef := pathParts[0]
				// Convert back to original reference format (this is a best effort)
				// For exact matching, use CreateModelsCatalogWithStaticFromResults
				modelRefs = append(modelRefs, sanitizedRef)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking directory: %v", err)
	}

	// Use the new function with the found model references
	return CreateModelsCatalogWithStaticFromResults(outputDir, catalogPath, modelRefs, staticModels)
}

// CreateModelsCatalog collects all metadata.yaml files and creates a models-catalog.yaml (backward compatibility)
func CreateModelsCatalog(outputDir, catalogPath string) error {
	return CreateModelsCatalogWithStatic(outputDir, catalogPath, []types.CatalogMetadata{})
}

// convertExtractedToCatalogMetadata converts ExtractedMetadata to CatalogMetadata
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
			CustomProperties:         convertCustomPropertiesToMetadataValue(artifact.CustomProperties),
		}
		catalogArtifacts = append(catalogArtifacts, catalogArtifact)
	}

	// Convert tags to customProperties
	customProps := convertTagsToCustomProperties(model.Tags)

	// Add validated_on as customProperty if present
	if len(model.ValidatedOn) > 0 {
		validatedOnValue := strings.Join(model.ValidatedOn, ",")
		customProps["validated_on"] = createMetadataValue(validatedOnValue)
	}

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

// convertTimestampToString converts an int64 timestamp to a string, returning nil if input is nil
func convertTimestampToString(timestamp *int64) *string {
	if timestamp == nil {
		return nil
	}
	str := strconv.FormatInt(*timestamp, 10)
	return &str
}

// convertTagsToCustomProperties converts all tags to customProperties format
func convertTagsToCustomProperties(tags []string) map[string]types.MetadataValue {
	customProps := make(map[string]types.MetadataValue)

	for _, tag := range tags {
		if tag != "" { // Skip empty tags
			customProps[tag] = createMetadataValue("")
		}
	}

	return customProps
}

// createMetadataValue creates a MetadataValue with the standard format
func createMetadataValue(value string) types.MetadataValue {
	return types.MetadataValue{
		MetadataType: "MetadataStringValue",
		StringValue:  value,
	}
}

// convertCustomPropertiesToMetadataValue converts CustomProperties from interface{} to MetadataValue format
func convertCustomPropertiesToMetadataValue(customProps map[string]interface{}) map[string]interface{} {
	if customProps == nil {
		return nil
	}

	result := make(map[string]interface{})
	for key, value := range customProps {
		result[key] = ensureMetadataValueFormat(value)
	}

	return result
}

// ensureMetadataValueFormat ensures a value is in the proper MetadataValue format with metadataType
func ensureMetadataValueFormat(value interface{}) map[string]interface{} {
	// Check if value is already in the correct MetadataValue format
	if valueMap, ok := value.(map[string]interface{}); ok {
		// Check if it already has metadataType
		if _, hasMetadataType := valueMap["metadataType"]; hasMetadataType {
			return valueMap
		} else {
			// Convert to proper MetadataValue format
			stringValue := ""
			if strVal, hasStringValue := valueMap["string_value"]; hasStringValue {
				if str, ok := strVal.(string); ok {
					stringValue = str
				}
			}
			return map[string]interface{}{
				"metadataType": "MetadataStringValue",
				"string_value": stringValue,
			}
		}
	} else {
		// Convert simple values to MetadataValue format
		stringValue := ""
		if str, ok := value.(string); ok {
			stringValue = str
		}
		return map[string]interface{}{
			"metadataType": "MetadataStringValue",
			"string_value": stringValue,
		}
	}
}

// determineLogo determines which logo to use based on model tags and returns base64-encoded data URI
func determineLogo(tags []string) *string {
	var svgPath string

	// Check if the model has the "validated" label
	for _, tag := range tags {
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

// encodeSVGToDataURI reads an SVG file and returns a base64-encoded data URI
func encodeSVGToDataURI(svgPath string) *string {
	// Read the SVG file
	svgContent, err := os.ReadFile(svgPath)
	if err != nil {
		log.Printf("Warning: Failed to read SVG file %s: %v", svgPath, err)
		// Return the file path as fallback
		fallback := svgPath
		return &fallback
	}

	// Encode to base64
	base64Content := base64.StdEncoding.EncodeToString(svgContent)

	// Create data URI
	dataUri := "data:image/svg+xml;base64," + base64Content
	return &dataUri
}

// deduplicateAndMergeModels consolidates duplicate models by merging their artifacts and metadata
func deduplicateAndMergeModels(models []types.CatalogMetadata) []types.CatalogMetadata {
	if len(models) <= 1 {
		return models
	}

	var unnamed []types.CatalogMetadata

	// Group models by name (case-insensitive)
	modelGroups := make(map[string][]types.CatalogMetadata)
	for _, model := range models {
		if model.Name == nil || strings.TrimSpace(*model.Name) == "" {
			unnamed = append(unnamed, model)
			continue
		}
		normalizedName := strings.ToLower(strings.TrimSpace(*model.Name))
		modelGroups[normalizedName] = append(modelGroups[normalizedName], model)
	}

	var result []types.CatalogMetadata
	duplicatesFound := 0

	for groupName, group := range modelGroups {
		if len(group) == 1 {
			// No duplicates, add as-is
			result = append(result, group[0])
		} else {
			// Merge duplicates
			log.Printf("Found %d duplicate models for '%s', consolidating...", len(group), groupName)
			duplicatesFound += len(group) - 1
			
			merged := mergeModelGroup(group)
			result = append(result, merged)
		}
	}

	if duplicatesFound > 0 {
		log.Printf("Successfully deduplicated %d models, consolidated %d duplicate entries", duplicatesFound, duplicatesFound)
	}

	// Sort result by name for consistent output
	sort.Slice(result, func(i, j int) bool {
		nameI := ""
		nameJ := ""
		if result[i].Name != nil {
			nameI = *result[i].Name
		}
		if result[j].Name != nil {
			nameJ = *result[j].Name
		}
		return nameI < nameJ
	})

	return append(result, unnamed...)
}

// mergeModelGroup merges a group of duplicate models into a single consolidated model
func mergeModelGroup(group []types.CatalogMetadata) types.CatalogMetadata {
	if len(group) == 0 {
		return types.CatalogMetadata{}
	}

	// Start with the first model as base
	merged := group[0]

	// Consolidate all artifacts from all models
	var allArtifacts []types.CatalogOCIArtifact
	artifactURIs := make(map[string]bool) // Track unique URIs

	for _, model := range group {
		for _, artifact := range model.Artifacts {
			// Only add if URI is unique
			if !artifactURIs[artifact.URI] {
				allArtifacts = append(allArtifacts, artifact)
				artifactURIs[artifact.URI] = true
			}
		}
	}
	merged.Artifacts = allArtifacts

	// Find earliest createTime and latest updateTime
	var earliestCreate *string
	var latestUpdate *string

	for _, model := range group {
		// Check model-level timestamps
		if model.CreateTimeSinceEpoch != nil {
			if earliestCreate == nil || compareTimestamps(*model.CreateTimeSinceEpoch, *earliestCreate) < 0 {
				earliestCreate = model.CreateTimeSinceEpoch
			}
		}
		if model.LastUpdateTimeSinceEpoch != nil {
			if latestUpdate == nil || compareTimestamps(*model.LastUpdateTimeSinceEpoch, *latestUpdate) > 0 {
				latestUpdate = model.LastUpdateTimeSinceEpoch
			}
		}

		// Check artifact-level timestamps
		for _, artifact := range model.Artifacts {
			if artifact.CreateTimeSinceEpoch != nil {
				if earliestCreate == nil || compareTimestamps(*artifact.CreateTimeSinceEpoch, *earliestCreate) < 0 {
					earliestCreate = artifact.CreateTimeSinceEpoch
				}
			}
			if artifact.LastUpdateTimeSinceEpoch != nil {
				if latestUpdate == nil || compareTimestamps(*artifact.LastUpdateTimeSinceEpoch, *latestUpdate) > 0 {
					latestUpdate = artifact.LastUpdateTimeSinceEpoch
				}
			}
		}
	}

	merged.CreateTimeSinceEpoch = earliestCreate
	merged.LastUpdateTimeSinceEpoch = latestUpdate

	// Merge metadata fields - prefer non-empty values, with priority to first model
	for i := 1; i < len(group); i++ {
		model := group[i]

		// Only override if current field is empty and new field has value
		if merged.Provider == nil && model.Provider != nil {
			merged.Provider = model.Provider
		}
		if merged.Description == nil && model.Description != nil {
			merged.Description = model.Description
		}
		if merged.Readme == nil && model.Readme != nil {
			merged.Readme = model.Readme
		}
		if merged.License == nil && model.License != nil {
			merged.License = model.License
		}
		if merged.LicenseLink == nil && model.LicenseLink != nil {
			merged.LicenseLink = model.LicenseLink
		}

		// Merge arrays by combining unique values
		if len(model.Language) > 0 {
			merged.Language = mergeUniqueStrings(merged.Language, model.Language)
		}
		if len(model.Tasks) > 0 {
			merged.Tasks = mergeUniqueStrings(merged.Tasks, model.Tasks)
		}

		// Merge custom properties
		if model.CustomProperties != nil {
			if merged.CustomProperties == nil {
				merged.CustomProperties = make(map[string]types.MetadataValue)
			}
			for key, value := range model.CustomProperties {
				if _, exists := merged.CustomProperties[key]; !exists {
					merged.CustomProperties[key] = value
				}
			}
		}
	}

	// Log the consolidation details
	log.Printf("  Consolidated %d models into '%s' with %d artifacts", len(group), *merged.Name, len(merged.Artifacts))
	for _, artifact := range merged.Artifacts {
		log.Printf("    - %s", artifact.URI)
	}

	return merged
}

// compareTimestamps compares two timestamp strings, returns -1 if a < b, 1 if a > b, 0 if equal
func compareTimestamps(a, b string) int {
	timestampA, errA := strconv.ParseInt(a, 10, 64)
	timestampB, errB := strconv.ParseInt(b, 10, 64)

	if errA != nil || errB != nil {
		// Fallback to string comparison if parsing fails
		return strings.Compare(a, b)
	}

	if timestampA < timestampB {
		return -1
	} else if timestampA > timestampB {
		return 1
	}
	return 0
}

// mergeUniqueStrings combines two string slices and removes duplicates
func mergeUniqueStrings(slice1, slice2 []string) []string {
	seen := make(map[string]bool)
	var result []string

	// Add from first slice
	for _, item := range slice1 {
		if !seen[item] && item != "" {
			result = append(result, item)
			seen[item] = true
		}
	}

	// Add from second slice
	for _, item := range slice2 {
		if !seen[item] && item != "" {
			result = append(result, item)
			seen[item] = true
		}
	}

	return result
}
