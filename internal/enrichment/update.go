package enrichment

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/internal/huggingface"
	"github.com/opendatahub-io/model-metadata-collection/internal/metadata"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// UpdateModelMetadataFile updates an existing metadata.yaml file with enriched data and creates separate enrichment.yaml
func UpdateModelMetadataFile(registryModel string, enrichedData *types.EnrichedModelMetadata, outputDir string) error {
	// Create sanitized directory name for the model
	sanitizedName := utils.SanitizeManifestRef(registryModel)
	metadataPath := fmt.Sprintf("%s/%s/models/metadata.yaml", outputDir, sanitizedName)
	enrichmentPath := fmt.Sprintf("%s/%s/models/enrichment.yaml", outputDir, sanitizedName)

	// Try to load existing metadata using migration logic
	existingMetadataPtr, err := metadata.LoadExistingMetadata(registryModel, outputDir)
	var existingMetadata types.ExtractedMetadata
	if err == nil && existingMetadataPtr != nil {
		existingMetadata = *existingMetadataPtr
	} else {
		// If loading fails, this could mean the metadata file doesn't exist yet
		// In this case, we start with an empty struct, which is correct
		log.Printf("  Warning: Could not load existing metadata for %s: %v", registryModel, err)
	}

	// Create enrichment structure with granular source tracking
	enrichmentInfo := struct {
		HuggingFaceModel string `yaml:"huggingface_model,omitempty"`
		HuggingFaceURL   string `yaml:"huggingface_url,omitempty"`
		MatchConfidence  string `yaml:"match_confidence,omitempty"`
		DataSources      struct {
			Name                 string `yaml:"name,omitempty"`
			Provider             string `yaml:"provider,omitempty"`
			Description          string `yaml:"description,omitempty"`
			License              string `yaml:"license,omitempty"`
			LicenseLink          string `yaml:"license_link,omitempty"`
			Language             string `yaml:"language,omitempty"`
			Tags                 string `yaml:"tags,omitempty"`
			Tasks                string `yaml:"tasks,omitempty"`
			LastModified         string `yaml:"last_modified,omitempty"`
			CreateTimeSinceEpoch string `yaml:"create_time_since_epoch,omitempty"`
			ValidatedOn          string `yaml:"validated_on,omitempty"`
			Readme               string `yaml:"readme,omitempty"`
		} `yaml:"data_sources"`
	}{}

	// Set enrichment info
	enrichmentInfo.HuggingFaceModel = enrichedData.HuggingFaceModel
	enrichmentInfo.HuggingFaceURL = enrichedData.HuggingFaceURL
	enrichmentInfo.MatchConfidence = enrichedData.MatchConfidence

	// Update metadata with enriched values and track sources in enrichment file
	if enrichedData.Name.Source != "null" {
		// Allow HuggingFace data to override modelcard extractions when we have high confidence
		shouldOverrideName := existingMetadata.Name == nil

		if existingMetadata.Name != nil {
			// Override based on HuggingFace match confidence
			switch enrichedData.MatchConfidence {
			case "high":
				shouldOverrideName = true
				log.Printf("  Overriding model name '%s' with high-confidence HuggingFace data", *existingMetadata.Name)
			case "medium":
				// For medium confidence, only override if the existing name looks like a document title
				name := strings.ToLower(*existingMetadata.Name)
				if strings.Contains(name, "model card") || strings.Contains(name, "readme") ||
					strings.Contains(name, "documentation") || strings.HasSuffix(name, " card") {
					shouldOverrideName = true
					log.Printf("  Overriding poor quality model name '%s' with medium-confidence HuggingFace data", *existingMetadata.Name)
				}
			}
		}

		if shouldOverrideName {
			nameStr := enrichedData.Name.Value.(string)
			existingMetadata.Name = &nameStr
		}
		enrichmentInfo.DataSources.Name = enrichedData.Name.Source
	}

	if enrichedData.Provider.Source != "null" {
		// Always override with HuggingFace YAML data (highest priority)
		shouldOverride := existingMetadata.Provider == nil || enrichedData.Provider.Source == "huggingface.yaml"
		if shouldOverride {
			providerStr := enrichedData.Provider.Value.(string)
			existingMetadata.Provider = &providerStr
		}
		enrichmentInfo.DataSources.Provider = enrichedData.Provider.Source
	}

	if enrichedData.Description.Source != "null" {
		// Always override with HuggingFace YAML data (highest priority)
		shouldOverride := existingMetadata.Description == nil || enrichedData.Description.Source == "huggingface.yaml"
		if shouldOverride {
			descStr := enrichedData.Description.Value.(string)
			existingMetadata.Description = &descStr
		}
		enrichmentInfo.DataSources.Description = enrichedData.Description.Source
	}

	if enrichedData.License.Source != "null" {
		// Always override with HuggingFace YAML data (highest priority)
		shouldOverride := existingMetadata.License == nil || enrichedData.License.Source == "huggingface.yaml"
		if shouldOverride {
			licenseStr := enrichedData.License.Value.(string)
			existingMetadata.License = &licenseStr
			// Automatically set license link if we have a well-known license
			if licenseURL := utils.GetLicenseURL(licenseStr); licenseURL != "" {
				existingMetadata.LicenseLink = &licenseURL
				enrichmentInfo.DataSources.LicenseLink = "generated"
			}
		}
		enrichmentInfo.DataSources.License = enrichedData.License.Source
	}

	if enrichedData.LicenseLink.Source != "null" {
		// Always override with HuggingFace YAML data (highest priority)
		shouldOverride := existingMetadata.LicenseLink == nil || enrichedData.LicenseLink.Source == "huggingface.yaml"
		if shouldOverride {
			licenseLinkStr := enrichedData.LicenseLink.Value.(string)
			existingMetadata.LicenseLink = &licenseLinkStr
		}
		enrichmentInfo.DataSources.LicenseLink = enrichedData.LicenseLink.Source
	}

	// Handle license from tags
	if enrichedData.Tags.Source == "huggingface.tags" && enrichedData.Tags.Value != nil {
		tags, ok := enrichedData.Tags.Value.([]string)
		if ok {
			_, tagLicense, _ := huggingface.ParseTagsForStructuredData(tags)
			if tagLicense != "" && existingMetadata.License == nil {
				existingMetadata.License = &tagLicense
				enrichmentInfo.DataSources.License = "huggingface.tags"
				// Automatically set license link if we have a well-known license
				if licenseURL := utils.GetLicenseURL(tagLicense); licenseURL != "" {
					existingMetadata.LicenseLink = &licenseURL
					enrichmentInfo.DataSources.LicenseLink = "generated"
				}
			}
		}
	}

	// Handle languages from enriched Language field
	if enrichedData.Language.Source != "null" && enrichedData.Language.Value != nil {
		if languages, ok := enrichedData.Language.Value.([]string); ok && len(languages) > 0 {
			// Always override with enriched language data (highest priority sources)
			shouldOverride := len(existingMetadata.Language) == 0 || enrichedData.Language.Source == "huggingface.yaml"
			if shouldOverride {
				existingMetadata.Language = languages
			}
			enrichmentInfo.DataSources.Language = enrichedData.Language.Source
		}
	}

	// Handle tags from enriched Tags field
	if enrichedData.Tags.Source != "null" && enrichedData.Tags.Value != nil {
		if newTags, ok := enrichedData.Tags.Value.([]string); ok && len(newTags) > 0 {
			// Always merge with existing tags to preserve "validated" and "featured" tags
			shouldMerge := len(existingMetadata.Tags) == 0 || enrichedData.Tags.Source == "huggingface.yaml" || enrichedData.Tags.Source == "huggingface.tags"
			if shouldMerge {
				// Preserve existing tags (like "validated", "featured") and merge with new ones
				mergedTags := make([]string, 0)

				// First, add existing tags to preserve "validated" and "featured"
				mergedTags = append(mergedTags, existingMetadata.Tags...)

				// Then add new tags, avoiding duplicates
				for _, newTag := range newTags {
					found := false
					for _, existingTag := range mergedTags {
						if existingTag == newTag {
							found = true
							break
						}
					}
					if !found {
						mergedTags = append(mergedTags, newTag)
					}
				}

				originalTags := make([]string, len(existingMetadata.Tags))
				copy(originalTags, existingMetadata.Tags)

				existingMetadata.Tags = mergedTags
				log.Printf("  Merged tags: existing %v + new %v = %v", originalTags, newTags, mergedTags)
			}
			enrichmentInfo.DataSources.Tags = enrichedData.Tags.Source
		}
	}

	// Handle tasks from enriched data first (highest priority)
	if enrichedData.Tasks.Source != "null" && enrichedData.Tasks.Value != nil {
		tasks, ok := enrichedData.Tasks.Value.([]string)
		if ok && len(tasks) > 0 {
			// Always override with HuggingFace YAML tasks (highest priority)
			shouldOverride := len(existingMetadata.Tasks) == 0 || enrichedData.Tasks.Source == "huggingface.yaml"
			if shouldOverride {
				log.Printf("  Debug: Using tasks from enrichedData.Tasks: %v", tasks)
				existingMetadata.Tasks = tasks
			}
			enrichmentInfo.DataSources.Tasks = enrichedData.Tasks.Source
		}
	} else if enrichedData.Tags.Source == "huggingface.tags" && enrichedData.Tags.Value != nil {
		// Fallback: parse tasks from tags if tasks field is not available
		tags, ok := enrichedData.Tags.Value.([]string)
		if ok {
			_, _, tasks := huggingface.ParseTagsForStructuredData(tags)
			log.Printf("  Debug: Parsed tasks from tags: %v", tasks)
			if len(tasks) > 0 && len(existingMetadata.Tasks) == 0 {
				existingMetadata.Tasks = tasks
				enrichmentInfo.DataSources.Tasks = "huggingface.tags"
			}
		}
	}

	// If still no tasks and we have a README, try to infer from model architecture (lowest priority)
	if len(existingMetadata.Tasks) == 0 && existingMetadata.Readme != nil {
		inferredTasks := huggingface.InferTasksFromReadme(*existingMetadata.Readme)
		if len(inferredTasks) > 0 {
			existingMetadata.Tasks = inferredTasks
			enrichmentInfo.DataSources.Tasks = "modelcard.inferred"
		}
	}

	// Handle enriched ValidatedOn data from HuggingFace YAML
	if enrichedData.ValidatedOn.Source != "null" && enrichedData.ValidatedOn.Value != nil {
		if raw, ok := enrichedData.ValidatedOn.Value.([]string); ok && len(raw) > 0 {
			// normalize: trim and dedupe
			seen := map[string]struct{}{}
			normalized := make([]string, 0, len(raw))
			for _, v := range raw {
				t := strings.TrimSpace(v)
				if t == "" {
					continue
				}
				if _, exists := seen[t]; exists {
					continue
				}
				seen[t] = struct{}{}
				normalized = append(normalized, t)
			}
			if len(normalized) > 0 {
				shouldOverride := len(existingMetadata.ValidatedOn) == 0 || enrichedData.ValidatedOn.Source == "huggingface.yaml"
				if shouldOverride {
					log.Printf("  Using validated_on from enrichedData: %v", normalized)
					existingMetadata.ValidatedOn = normalized
				}
				enrichmentInfo.DataSources.ValidatedOn = enrichedData.ValidatedOn.Source
			}
		}
	}

	// Handle enriched createTimeSinceEpoch data
	if enrichedData.CreateTimeSinceEpoch.Source != "null" && enrichedData.CreateTimeSinceEpoch.Value != nil {
		if createEpoch, ok := enrichedData.CreateTimeSinceEpoch.Value.(int64); ok {
			// Use enriched createTimeSinceEpoch if not already set or if existing value is null/zero
			if existingMetadata.CreateTimeSinceEpoch == nil || *existingMetadata.CreateTimeSinceEpoch == 0 {
				existingMetadata.CreateTimeSinceEpoch = &createEpoch
				enrichmentInfo.DataSources.CreateTimeSinceEpoch = enrichedData.CreateTimeSinceEpoch.Source
				log.Printf("  Set createTimeSinceEpoch from enriched data: %d", createEpoch)
			}
		}
	}

	// Handle enriched lastUpdateTimeSinceEpoch data from HuggingFace README
	if enrichedData.LastModified.Source != "null" && strings.HasPrefix(enrichedData.LastModified.Source, "huggingface") && enrichedData.LastModified.Value != nil {
		if releaseEpoch, ok := enrichedData.LastModified.Value.(int64); ok {
			// Use README release date for lastUpdateTimeSinceEpoch if not already set or if existing value is null/zero
			if existingMetadata.LastUpdateTimeSinceEpoch == nil || *existingMetadata.LastUpdateTimeSinceEpoch == 0 {
				existingMetadata.LastUpdateTimeSinceEpoch = &releaseEpoch
				enrichmentInfo.DataSources.LastModified = enrichedData.LastModified.Source
				log.Printf("  Set lastUpdateTimeSinceEpoch from HuggingFace README release date: %d", releaseEpoch)
			}
		}
	}

	// Final step: Set license link for any license that doesn't already have one
	if existingMetadata.License != nil && existingMetadata.LicenseLink == nil {
		if licenseURL := utils.GetLicenseURL(*existingMetadata.License); licenseURL != "" {
			existingMetadata.LicenseLink = &licenseURL
		}
	}

	// IMPORTANT: Preserve readme content if it's missing but modelcard file exists
	if existingMetadata.Readme == nil {
		modelcardPath := fmt.Sprintf("%s/%s/models/modelcard.md", outputDir, sanitizedName)
		if modelcardContent, err := os.ReadFile(modelcardPath); err == nil && len(modelcardContent) > 0 {
			readme := string(modelcardContent)
			existingMetadata.Readme = &readme
			log.Printf("  Restored readme content from modelcard.md for: %s", registryModel)
		}
	}

	// DESCRIPTION FALLBACK LOGIC: Generate description if missing
	if existingMetadata.Description == nil {
		var description string

		// First try to get description from HuggingFace data if available
		if enrichedData.HuggingFaceModel != "" {
			// Try to extract description from HuggingFace model name/path
			description = utils.GenerateDescriptionFromModelName(enrichedData.HuggingFaceModel)
		} else if existingMetadata.Name != nil {
			// Fallback to model name if no HuggingFace match
			description = utils.GenerateDescriptionFromModelName(*existingMetadata.Name)
		}

		if description != "" {
			existingMetadata.Description = &description
			log.Printf("  Generated description from model name for: %s", registryModel)
		}
	}

	// Write clean metadata to metadata.yaml (without enrichment section)
	updatedData, err := yaml.Marshal(existingMetadata)
	if err != nil {
		return fmt.Errorf("failed to marshal updated metadata: %v", err)
	}

	err = os.WriteFile(metadataPath, updatedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated metadata: %v", err)
	}

	// Write enrichment data to separate enrichment.yaml file
	enrichmentData, err := yaml.Marshal(enrichmentInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal enrichment data: %v", err)
	}

	err = os.WriteFile(enrichmentPath, enrichmentData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write enrichment file: %v", err)
	}

	return nil
}
