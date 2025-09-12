package catalog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

// CreateModelsCatalog collects all metadata.yaml files and creates a models-catalog.yaml
func CreateModelsCatalog() error {
	var allModels []types.ExtractedMetadata

	// Find all metadata.yaml files in output directory
	err := filepath.Walk("output", func(path string, info os.FileInfo, err error) error {
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

	// Convert to catalog metadata (excluding tags)
	var catalogModels []types.CatalogMetadata
	for _, model := range allModels {
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

		catalogModel := types.CatalogMetadata{
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
		}
		catalogModels = append(catalogModels, catalogModel)
	}

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

	// Write to data/models-catalog.yaml
	catalogPath := "data/models-catalog.yaml"
	err = os.WriteFile(catalogPath, output, 0644)
	if err != nil {
		return fmt.Errorf("error writing catalog file: %v", err)
	}

	log.Printf("Successfully created %s with %d models", catalogPath, len(allModels))
	return nil
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
			customProps[tag] = types.MetadataValue{
				MetadataType: "MetadataStringValue",
				StringValue:  "",
			}
		}
	}

	return customProps
}
