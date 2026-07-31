package metadata

import (
	"fmt"
	"os"
	"reflect"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// LoadExistingMetadata attempts to load existing metadata from processed models
func LoadExistingMetadata(registryModel, outputDir string) (*types.ExtractedMetadata, error) {
	// Create sanitized directory name for the model
	sanitizedName := utils.SanitizeManifestRef(registryModel)
	metadataPath := fmt.Sprintf("%s/%s/models/metadata.yaml", outputDir, sanitizedName)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err // File doesn't exist or can't read
	}

	// First try to unmarshal as new format
	var metadata types.ExtractedMetadata
	err = yaml.Unmarshal(data, &metadata)
	if err == nil {
		// Fix timestamp consistency and null handling for existing metadata
		fixTimestampConsistency(&metadata)
		return &metadata, nil
	}

	// Try mixed-type format (handles string timestamps in artifacts)
	var mixedMetadata types.MixedTypeExtractedMetadata
	err = yaml.Unmarshal(data, &mixedMetadata)
	if err == nil {
		// Convert mixed format to standard format
		convertedMetadata := convertMixedToStandard(&mixedMetadata)
		fixTimestampConsistency(convertedMetadata)
		return convertedMetadata, nil
	}

	// If that fails, try legacy format and migrate
	var legacyMetadata types.LegacyExtractedMetadata
	err = yaml.Unmarshal(data, &legacyMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata in all formats: %v", err)
	}

	// Migrate from legacy format
	migratedMetadata := migrateFromLegacyFormat(&legacyMetadata)
	fixTimestampConsistency(migratedMetadata)
	return migratedMetadata, nil
}

// migrateFromLegacyFormat converts legacy metadata to new format
func migrateFromLegacyFormat(legacy *types.LegacyExtractedMetadata) *types.ExtractedMetadata {
	new := &types.ExtractedMetadata{
		Name:                     legacy.Name,
		Provider:                 legacy.Provider,
		Description:              legacy.Description,
		Readme:                   legacy.Readme,
		Language:                 legacy.Language,
		License:                  legacy.License,
		LicenseLink:              legacy.LicenseLink,
		Tags:                     legacy.Tags,
		Tasks:                    legacy.Tasks,
		CreateTimeSinceEpoch:     legacy.CreateTimeSinceEpoch,
		LastUpdateTimeSinceEpoch: legacy.LastUpdateTimeSinceEpoch,
		Artifacts:                []types.OCIArtifact{}, // Will be populated later
	}
	return new
}

// fixTimestampConsistency ensures timestamp consistency and handles null values
func fixTimestampConsistency(metadata *types.ExtractedMetadata) {
	// Fix model-level null handling: if lastUpdateTimeSinceEpoch is null but createTimeSinceEpoch has a value,
	// set lastUpdateTimeSinceEpoch to the same value as createTimeSinceEpoch
	if metadata.LastUpdateTimeSinceEpoch == nil && metadata.CreateTimeSinceEpoch != nil {
		lastUpdate := *metadata.CreateTimeSinceEpoch
		metadata.LastUpdateTimeSinceEpoch = &lastUpdate
	}

	// Fix artifact-level timestamp consistency
	for i := range metadata.Artifacts {
		artifact := &metadata.Artifacts[i]

		// Convert string timestamps to int64 if needed
		fixArtifactTimestamp(&artifact.CreateTimeSinceEpoch)
		fixArtifactTimestamp(&artifact.LastUpdateTimeSinceEpoch)

		// Handle null lastUpdateTimeSinceEpoch: set it to createTimeSinceEpoch if createTimeSinceEpoch has a value
		if artifact.LastUpdateTimeSinceEpoch == nil && artifact.CreateTimeSinceEpoch != nil {
			lastUpdate := *artifact.CreateTimeSinceEpoch
			artifact.LastUpdateTimeSinceEpoch = &lastUpdate
		}
	}
}

// fixArtifactTimestamp converts string timestamps to int64 and handles type consistency
func fixArtifactTimestamp(timestamp **int64) {
	// This function handles cases where timestamps might be stored as strings in YAML
	// Note: Since Go's yaml package will already convert string numbers to int64 when unmarshaling
	// into *int64 fields, this function mainly serves as a placeholder for any future
	// timestamp format conversions that might be needed
}

// convertMixedToStandard converts MixedTypeExtractedMetadata to standard ExtractedMetadata
func convertMixedToStandard(mixed *types.MixedTypeExtractedMetadata) *types.ExtractedMetadata {
	standard := &types.ExtractedMetadata{
		Name:                     mixed.Name,
		Provider:                 mixed.Provider,
		Description:              mixed.Description,
		Readme:                   mixed.Readme,
		Language:                 mixed.Language,
		License:                  mixed.License,
		LicenseLink:              mixed.LicenseLink,
		Tags:                     mixed.Tags,
		Tasks:                    mixed.Tasks,
		CreateTimeSinceEpoch:     convertTimestamp(mixed.CreateTimeSinceEpoch),
		LastUpdateTimeSinceEpoch: convertTimestamp(mixed.LastUpdateTimeSinceEpoch),
		Artifacts:                make([]types.OCIArtifact, len(mixed.Artifacts)),
	}

	// Convert artifacts with mixed timestamp types
	for i, mixedArtifact := range mixed.Artifacts {
		standard.Artifacts[i] = types.OCIArtifact{
			URI:                      mixedArtifact.URI,
			CreateTimeSinceEpoch:     convertTimestamp(mixedArtifact.CreateTimeSinceEpoch),
			LastUpdateTimeSinceEpoch: convertTimestamp(mixedArtifact.LastUpdateTimeSinceEpoch),
			CustomProperties:         mixedArtifact.CustomProperties,
		}
	}

	return standard
}

// convertTimestamp converts interface{} timestamp to *int64
func convertTimestamp(timestamp any) *int64 {
	if timestamp == nil {
		return nil
	}

	switch v := timestamp.(type) {
	case string:
		// Convert string to int64
		if v == "" {
			return nil
		}
		// Try to parse as int64
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return &parsed
		}
		return nil
	case int:
		result := int64(v)
		return &result
	case int64:
		return &v
	case float64:
		result := int64(v)
		return &result
	default:
		return nil
	}
}

// createMetadataSource creates a MetadataSource with value and source tracking
func CreateMetadataSource(value any, source string) types.MetadataSource {
	if value == nil || (reflect.TypeOf(value).Kind() == reflect.String && value.(string) == "") {
		return types.MetadataSource{Value: nil, Source: "null"}
	}
	return types.MetadataSource{Value: value, Source: source}
}
