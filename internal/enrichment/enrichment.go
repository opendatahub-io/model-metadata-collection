package enrichment

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/internal/config"
	"github.com/opendatahub-io/model-metadata-collection/internal/huggingface"
	"github.com/opendatahub-io/model-metadata-collection/internal/metadata"
	"github.com/opendatahub-io/model-metadata-collection/internal/registry"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// isCompatibleModelFamily checks if a container registry model can be matched with a HuggingFace model
// This prevents cross-family matching (e.g., llama containers matching granite HF entries)
func isCompatibleModelFamily(regModel, hfModelName string) bool {
	regNorm := utils.NormalizeModelName(regModel)
	hfNorm := utils.NormalizeModelName(hfModelName)

	// Extract model family identifiers
	regFamily := extractModelFamily(regNorm)
	hfFamily := extractModelFamily(hfNorm)

	// Allow matching within the same family or when families are unknown
	return regFamily == "" || hfFamily == "" || regFamily == hfFamily
}

// extractModelFamily extracts the model family from a normalized model name
func extractModelFamily(normalizedName string) string {
	tokens := strings.FieldsFunc(normalizedName, func(r rune) bool {
		return r == '-' || r == '_' || r == '/' || r == '.'
	})

	for _, token := range tokens {
		switch token {
		case "llama", "granite", "mistral", "gemma", "qwen", "phi", "mixtral", "deepseek", "kimi":
			return token
		}
	}

	return ""
}

// EnrichMetadataFromHuggingFace enriches registry model metadata using HuggingFace data
func EnrichMetadataFromHuggingFace(hfIndexPath, modelsIndexPath, outputDir string) error {
	log.Println("Enriching registry model metadata with HuggingFace data...")

	// Load HuggingFace models
	hfFilePath := hfIndexPath
	hfData, err := os.ReadFile(hfFilePath)
	if err != nil {
		return fmt.Errorf("failed to read HuggingFace index: %v", err)
	}

	var hfIndex types.VersionIndex
	err = yaml.Unmarshal(hfData, &hfIndex)
	if err != nil {
		return fmt.Errorf("failed to parse HuggingFace index: %v", err)
	}

	// Load registry models
	regModels, err := config.LoadModelsFromYAML(modelsIndexPath)
	if err != nil {
		return fmt.Errorf("failed to load registry models: %v", err)
	}

	matchCount := 0

	// For each registry model, find the best HuggingFace match and enrich metadata
	for _, regModel := range regModels {
		log.Printf("Processing model: %s", regModel)

		enriched := types.EnrichedModelMetadata{
			RegistryModel:    regModel,
			EnrichmentStatus: "no_match",
		}

		// Try to load existing modelcard metadata
		existingMetadata, err := metadata.LoadExistingMetadata(regModel, outputDir)
		if err != nil {
			log.Printf("  No existing metadata found for %s", regModel)
		}

		// Initialize metadata sources with existing data or nulls
		enriched.Name = metadata.CreateMetadataSource(nil, "null")
		enriched.Provider = metadata.CreateMetadataSource(nil, "null")
		enriched.Description = metadata.CreateMetadataSource(nil, "null")
		enriched.License = metadata.CreateMetadataSource(nil, "null")
		enriched.LicenseLink = metadata.CreateMetadataSource(nil, "null")
		enriched.Language = metadata.CreateMetadataSource(nil, "null")
		enriched.LastModified = metadata.CreateMetadataSource(nil, "null")
		enriched.CreateTimeSinceEpoch = metadata.CreateMetadataSource(nil, "null")
		enriched.Tags = metadata.CreateMetadataSource(nil, "null")
		enriched.Tasks = metadata.CreateMetadataSource(nil, "null")
		enriched.Downloads = metadata.CreateMetadataSource(nil, "null")
		enriched.Likes = metadata.CreateMetadataSource(nil, "null")
		enriched.ModelSize = metadata.CreateMetadataSource(nil, "null")
		enriched.ValidatedOn = metadata.CreateMetadataSource(nil, "null")

		// Populate from existing modelcard metadata if available (only for non-empty values)
		// We need to determine if the data came from YAML frontmatter or text parsing
		if existingMetadata != nil {
			// Try to load the modelcard.md file to analyze the source
			sanitizedName := utils.SanitizeManifestRef(regModel)
			modelcardPath := fmt.Sprintf("%s/%s/models/modelcard.md", outputDir, sanitizedName)

			var modelcardContent string
			var hasYAMLFrontmatter bool
			if content, err := os.ReadFile(modelcardPath); err == nil {
				modelcardContent = string(content)
				// Check if modelcard has YAML frontmatter
				if frontmatter, err := metadata.ExtractYAMLFrontmatterFromModelCard(modelcardContent); err == nil {
					hasYAMLFrontmatter = true
					// Determine sources based on YAML frontmatter presence
					// Note: Only check fields that exist in ModelCardYAMLFrontmatter struct

					// Name can come from YAML frontmatter
					if existingMetadata.Name != nil && *existingMetadata.Name != "" {
						source := "modelcard.regex"
						if frontmatter.Name != "" && frontmatter.Name == *existingMetadata.Name {
							source = "modelcard.yaml"
						}
						enriched.Name = metadata.CreateMetadataSource(*existingMetadata.Name, source)
					}

					// Provider can come from YAML frontmatter
					if existingMetadata.Provider != nil && *existingMetadata.Provider != "" {
						source := "modelcard.regex"
						if frontmatter.Provider != "" && frontmatter.Provider == *existingMetadata.Provider {
							source = "modelcard.yaml"
						}
						enriched.Provider = metadata.CreateMetadataSource(*existingMetadata.Provider, source)
					}

					// Description can come from YAML frontmatter
					if existingMetadata.Description != nil && *existingMetadata.Description != "" {
						source := "modelcard.regex"
						if frontmatter.Description != "" && frontmatter.Description == *existingMetadata.Description {
							source = "modelcard.yaml"
						}
						enriched.Description = metadata.CreateMetadataSource(*existingMetadata.Description, source)
					}

					// License can come from YAML frontmatter
					if existingMetadata.License != nil && *existingMetadata.License != "" {
						source := "modelcard.regex"
						if frontmatter.License != "" && frontmatter.License == *existingMetadata.License {
							source = "modelcard.yaml"
						} else if frontmatter.LicenseName != "" && frontmatter.LicenseName == *existingMetadata.License {
							source = "modelcard.yaml"
						}
						enriched.License = metadata.CreateMetadataSource(*existingMetadata.License, source)
					}

					// LicenseLink can come from YAML frontmatter
					if existingMetadata.LicenseLink != nil && *existingMetadata.LicenseLink != "" {
						source := "modelcard.regex"
						if frontmatter.LicenseLink != "" && frontmatter.LicenseLink == *existingMetadata.LicenseLink {
							source = "modelcard.yaml"
						}
						enriched.LicenseLink = metadata.CreateMetadataSource(*existingMetadata.LicenseLink, source)
					}

					// Tasks can come from YAML frontmatter (tasks field or pipeline_tag)
					if len(existingMetadata.Tasks) > 0 {
						source := "modelcard.regex"
						// Check if tasks match the tasks field or pipeline_tag
						if len(frontmatter.Tasks) > 0 && len(existingMetadata.Tasks) == len(frontmatter.Tasks) {
							allMatch := true
							for i, task := range existingMetadata.Tasks {
								if i >= len(frontmatter.Tasks) || task != frontmatter.Tasks[i] {
									allMatch = false
									break
								}
							}
							if allMatch {
								source = "modelcard.yaml"
							}
						} else if frontmatter.PipelineTag != "" && len(existingMetadata.Tasks) == 1 && existingMetadata.Tasks[0] == frontmatter.PipelineTag {
							source = "modelcard.yaml"
						}
						enriched.Tasks = metadata.CreateMetadataSource(existingMetadata.Tasks, source)
					}

					// Language can come from YAML frontmatter
					if len(existingMetadata.Language) > 0 {
						source := "modelcard.regex"
						if len(frontmatter.Language) > 0 && len(existingMetadata.Language) == len(frontmatter.Language) {
							allMatch := true
							for i, lang := range existingMetadata.Language {
								if i >= len(frontmatter.Language) || lang != frontmatter.Language[i] {
									allMatch = false
									break
								}
							}
							if allMatch {
								source = "modelcard.yaml"
							}
						}
						enriched.Language = metadata.CreateMetadataSource(existingMetadata.Language, source)
					}

					// Tags can come from YAML frontmatter
					if len(existingMetadata.Tags) > 0 {
						source := "modelcard.regex"
						if len(frontmatter.Tags) > 0 && len(existingMetadata.Tags) == len(frontmatter.Tags) {
							allMatch := true
							for i, tag := range existingMetadata.Tags {
								if i >= len(frontmatter.Tags) || tag != frontmatter.Tags[i] {
									allMatch = false
									break
								}
							}
							if allMatch {
								source = "modelcard.yaml"
							}
						}
						enriched.Tags = metadata.CreateMetadataSource(existingMetadata.Tags, source)
					}
				}
			}

			// If no YAML frontmatter analysis was possible, assume all modelcard data comes from regex/text parsing
			if !hasYAMLFrontmatter {
				if existingMetadata.Name != nil && *existingMetadata.Name != "" {
					enriched.Name = metadata.CreateMetadataSource(*existingMetadata.Name, "modelcard.regex")
				}
				if existingMetadata.Provider != nil && *existingMetadata.Provider != "" {
					enriched.Provider = metadata.CreateMetadataSource(*existingMetadata.Provider, "modelcard.regex")
				}
				if existingMetadata.Description != nil && *existingMetadata.Description != "" {
					enriched.Description = metadata.CreateMetadataSource(*existingMetadata.Description, "modelcard.regex")
				}
				if existingMetadata.License != nil && *existingMetadata.License != "" {
					enriched.License = metadata.CreateMetadataSource(*existingMetadata.License, "modelcard.regex")
				}
				if existingMetadata.LicenseLink != nil && *existingMetadata.LicenseLink != "" {
					enriched.LicenseLink = metadata.CreateMetadataSource(*existingMetadata.LicenseLink, "modelcard.regex")
				}
				if len(existingMetadata.Language) > 0 {
					enriched.Language = metadata.CreateMetadataSource(existingMetadata.Language, "modelcard.regex")
				}
				if len(existingMetadata.Tags) > 0 {
					enriched.Tags = metadata.CreateMetadataSource(existingMetadata.Tags, "modelcard.regex")
				}
				if len(existingMetadata.Tasks) > 0 {
					enriched.Tasks = metadata.CreateMetadataSource(existingMetadata.Tasks, "modelcard.regex")
				}
			}

			// Handle timestamps (these are typically from text parsing, not YAML)
			if existingMetadata.LastUpdateTimeSinceEpoch != nil {
				enriched.LastModified = metadata.CreateMetadataSource(*existingMetadata.LastUpdateTimeSinceEpoch, "modelcard.regex")
			}
			if existingMetadata.CreateTimeSinceEpoch != nil {
				enriched.CreateTimeSinceEpoch = metadata.CreateMetadataSource(*existingMetadata.CreateTimeSinceEpoch, "modelcard.regex")
			}
		}

		// Find best matching HuggingFace model
		bestMatch := types.ModelIndex{}
		bestScore := 0.0

		for _, hfModel := range hfIndex.Models {
			// Skip cross-family matches to prevent llama containers from matching granite HF entries
			if !isCompatibleModelFamily(regModel, hfModel.Name) {
				continue
			}

			score := utils.CalculateSimilarity(regModel, hfModel.Name)
			if score > bestScore {
				bestScore = score
				bestMatch = hfModel
			}
		}

		// Enrich with HuggingFace data if we found a good match
		threshold := 0.5
		if bestScore >= threshold {
			enriched.HuggingFaceModel = bestMatch.Name
			enriched.HuggingFaceURL = bestMatch.URL
			enriched.ReadmePath = bestMatch.ReadmePath
			enriched.EnrichmentStatus = "enriched"

			// Set confidence level
			if bestScore >= 0.8 {
				enriched.MatchConfidence = "high"
			} else {
				enriched.MatchConfidence = "medium"
			}

			// Try to fetch detailed HuggingFace metadata
			log.Printf("  Fetching HuggingFace details for: %s", bestMatch.Name)
			hfDetails, err := huggingface.FetchModelDetails(bestMatch.Name)
			if err != nil {
				log.Printf("  Warning: Failed to fetch HF details: %v", err)
			} else {
				// Always store HuggingFace name when available - the confidence-based override logic will decide whether to use it
				if hfDetails.ID != "" {
					// For high-confidence matches, always set the HuggingFace name so it can be used by confidence-based override logic
					if enriched.MatchConfidence == "high" {
						enriched.Name = metadata.CreateMetadataSource(hfDetails.ID, "huggingface.api")
					} else if enriched.Name.Source == "null" {
						// For medium/low confidence, only set if no existing name
						enriched.Name = metadata.CreateMetadataSource(hfDetails.ID, "huggingface.api")
					}
				}
				if enriched.License.Source == "null" && hfDetails.License != "" {
					enriched.License = metadata.CreateMetadataSource(hfDetails.License, "huggingface.api")
				}
				if enriched.LastModified.Source == "null" && hfDetails.LastModified != "" {
					enriched.LastModified = metadata.CreateMetadataSource(hfDetails.LastModified, "huggingface.api")
				}
				if len(hfDetails.Tags) > 0 {
					// Parse tags for structured data and potentially extract license
					languages, tagLicense, tasks := huggingface.ParseTagsForStructuredData(hfDetails.Tags)
					log.Printf("  Parsed from tags - Languages: %v, License: %s, Tasks: %v", languages, tagLicense, tasks)

					// NOTE: Do NOT store raw repository tags here - they will be used as fallback later
					// Raw repository tags contain language codes, arxiv refs, and other metadata that should be filtered

					// Store parsed languages (if no YAML frontmatter languages available)
					if enriched.Language.Source == "null" && len(languages) > 0 {
						enriched.Language = metadata.CreateMetadataSource(languages, "huggingface.tags")
					}

					// Use license from tags if not already set
					if enriched.License.Source == "null" && tagLicense != "" {
						enriched.License = metadata.CreateMetadataSource(tagLicense, "huggingface.tags")
					}

					// Store tasks if found
					if enriched.Tasks.Source == "null" && len(tasks) > 0 {
						enriched.Tasks = metadata.CreateMetadataSource(tasks, "huggingface.tags")
					}
				}
				if enriched.Downloads.Source == "null" && hfDetails.Downloads > 0 {
					enriched.Downloads = metadata.CreateMetadataSource(hfDetails.Downloads, "huggingface.api")
				}
				if enriched.Likes.Source == "null" && hfDetails.Likes > 0 {
					enriched.Likes = metadata.CreateMetadataSource(hfDetails.Likes, "huggingface.api")
				}
			}

			// Always fetch HuggingFace README to check for YAML frontmatter (highest priority)
			// Also extract release date and other metadata information as needed
			needsProvider := enriched.Provider.Source == "null"
			// Extract release date if we don't have a valid date yet (even from modelcard.regex with null value)
			needsReleaseDate := enriched.LastModified.Source == "null" ||
				(enriched.LastModified.Source == "modelcard.regex" && enriched.LastModified.Value == nil)

			log.Printf("  DEBUG: LastModified source='%s', value=%v, needsReleaseDate=%v",
				enriched.LastModified.Source, enriched.LastModified.Value, needsReleaseDate)
			log.Printf("  Fetching HuggingFace README for additional metadata: %s", bestMatch.Name)
			hfReadme, err := huggingface.FetchReadme(bestMatch.Name)
			if err != nil {
				log.Printf("  Warning: Failed to fetch HF README: %v", err)
			} else {
				// Try to extract YAML frontmatter first
				frontmatter, err := huggingface.ExtractYAMLFrontmatter(hfReadme)
				if err == nil {
					log.Printf("  Successfully extracted YAML frontmatter from HF README")

					// Always use name from HuggingFace YAML (highest priority)
					if frontmatter.Name != "" {
						enriched.Name = metadata.CreateMetadataSource(frontmatter.Name, "huggingface.yaml")
						log.Printf("  Found name in YAML frontmatter: %s", frontmatter.Name)
					}

					// Always use provider from HuggingFace YAML (highest priority)
					if frontmatter.Provider != "" {
						enriched.Provider = metadata.CreateMetadataSource(frontmatter.Provider, "huggingface.yaml")
						log.Printf("  Found provider in YAML frontmatter: %s", frontmatter.Provider)
					}

					// Always use description from HuggingFace YAML (highest priority)
					if frontmatter.Description != "" {
						enriched.Description = metadata.CreateMetadataSource(frontmatter.Description, "huggingface.yaml")
						log.Printf("  Found description in YAML frontmatter: %s", frontmatter.Description)
					}

					// Always use language from HuggingFace YAML frontmatter (highest priority)
					if len(frontmatter.Language) > 0 {
						// Convert to []string to ensure type compatibility
						enriched.Language = metadata.CreateMetadataSource([]string(frontmatter.Language), "huggingface.yaml")
						log.Printf("  Found languages in YAML frontmatter: %v", frontmatter.Language)
					}

					// Always use tags from HuggingFace YAML frontmatter (highest priority)
					if len(frontmatter.Tags) > 0 {
						enriched.Tags = metadata.CreateMetadataSource(frontmatter.Tags, "huggingface.yaml")
						log.Printf("  Found tags in YAML frontmatter: %v", frontmatter.Tags)
					}

					// Always use license from HuggingFace YAML frontmatter (highest priority)
					if frontmatter.License != "" {
						enriched.License = metadata.CreateMetadataSource(frontmatter.License, "huggingface.yaml")
						log.Printf("  Extracted license from YAML frontmatter: %s", frontmatter.License)
					}

					// Always use license_name if available and more specific (highest priority)
					if frontmatter.LicenseName != "" {
						enriched.License = metadata.CreateMetadataSource(frontmatter.LicenseName, "huggingface.yaml")
						log.Printf("  Extracted license_name from YAML frontmatter: %s", frontmatter.LicenseName)
					}

					// Always use license_link from HuggingFace YAML frontmatter (highest priority)
					if frontmatter.LicenseLink != "" {
						enriched.LicenseLink = metadata.CreateMetadataSource(frontmatter.LicenseLink, "huggingface.yaml")
						log.Printf("  Extracted license_link from YAML frontmatter: %s", frontmatter.LicenseLink)
					}

					// Always use tasks from HuggingFace YAML (highest priority)
					if len(frontmatter.Tasks) > 0 {
						enriched.Tasks = metadata.CreateMetadataSource(frontmatter.Tasks, "huggingface.yaml")
						log.Printf("  Extracted tasks from YAML frontmatter: %v", frontmatter.Tasks)
					} else if frontmatter.PipelineTag != "" {
						// Fallback to pipeline_tag for tasks if tasks field is not available
						tasks := []string{frontmatter.PipelineTag}
						enriched.Tasks = metadata.CreateMetadataSource(tasks, "huggingface.yaml")
						log.Printf("  Extracted pipeline_tag from YAML frontmatter: %s", frontmatter.PipelineTag)
					}
					// Always use validated_on from HuggingFace YAML (highest priority)
					if len(frontmatter.ValidatedOn) > 0 {
						enriched.ValidatedOn = metadata.CreateMetadataSource([]string(frontmatter.ValidatedOn), "huggingface.yaml")
						log.Printf("  Extracted validated_on from YAML frontmatter: %v", frontmatter.ValidatedOn)
					}
				} else {
					log.Printf("  No valid YAML frontmatter found in HF README: %v", err)
				}

				// Fallback to text parsing for provider if needed
				if needsProvider && enriched.Provider.Source == "null" {
					provider := huggingface.ExtractProviderFromReadme(hfReadme)
					if provider != "" {
						enriched.Provider = metadata.CreateMetadataSource(provider, "huggingface.regex")
						log.Printf("  Extracted provider from HF README text: %s", provider)
					}
				}

				// Try to extract explicit release date from README (high priority)
				releaseDate := huggingface.ExtractReleaseDateFromReadme(hfReadme)
				if releaseDate != "" {
					if epoch := utils.ParseDateToEpoch(releaseDate); epoch != nil {
						// Use this for createTimeSinceEpoch if we don't have it from modelcard
						if enriched.CreateTimeSinceEpoch.Source == "null" {
							enriched.CreateTimeSinceEpoch = metadata.CreateMetadataSource(*epoch, "huggingface.regex")
							log.Printf("  Extracted createTimeSinceEpoch from HF README release date: %s (epoch: %d)", releaseDate, *epoch)
						}
						// Also update lastModified if we don't have a more recent one
						if needsReleaseDate {
							enriched.LastModified = metadata.CreateMetadataSource(*epoch, "huggingface.regex")
							log.Printf("  Extracted lastModified from HF README release date: %s (epoch: %d)", releaseDate, *epoch)
						}
					}
				}
			}

			// Use repository tags as additional enrichment: Apply if no YAML frontmatter tags were found
			// This will merge with existing modelcard tags (like "validated"/"featured") during update phase
			if enriched.Tags.Source == "null" && len(hfDetails.Tags) > 0 {
				log.Printf("  No YAML frontmatter tags found, using filtered repository tags")
				// Filter out language codes, arxiv references, and other non-tag metadata
				filteredTags := huggingface.FilterTagsForCleanTagList(hfDetails.Tags)
				if len(filteredTags) > 0 {
					enriched.Tags = metadata.CreateMetadataSource(filteredTags, "huggingface.tags")
					log.Printf("  Using filtered repository tags: %v", filteredTags)
				}
			} else if enriched.Tags.Source == "modelcard.regex" && len(hfDetails.Tags) > 0 {
				log.Printf("  Found modelcard tags, merging with filtered repository tags")
				// Filter out language codes, arxiv references, and other non-tag metadata
				filteredTags := huggingface.FilterTagsForCleanTagList(hfDetails.Tags)
				if len(filteredTags) > 0 {
					// Merge existing modelcard tags with HuggingFace tags
					existingTags := enriched.Tags.Value.([]string)
					allTags := make([]string, 0)

					// First add existing tags
					allTags = append(allTags, existingTags...)

					// Then add new tags, avoiding duplicates
					for _, newTag := range filteredTags {
						found := false
						for _, existingTag := range allTags {
							if existingTag == newTag {
								found = true
								break
							}
						}
						if !found {
							allTags = append(allTags, newTag)
						}
					}

					enriched.Tags = metadata.CreateMetadataSource(allTags, "huggingface.tags")
					log.Printf("  Merged modelcard + repository tags: %v", allTags)
				}
			}

			// Update the model's metadata.yaml file with enriched data
			err = UpdateModelMetadataFile(regModel, &enriched, outputDir)
			if err != nil {
				log.Printf("  Warning: Failed to update metadata file for %s: %v", regModel, err)
			} else {
				log.Printf("  Successfully updated metadata file for: %s", regModel)

				// Also update artifacts with OCI metadata
				log.Printf("  Updating OCI artifacts for: %s", regModel)
				err = UpdateOCIArtifacts(regModel, outputDir)
				if err != nil {
					log.Printf("  Warning: Failed to update OCI artifacts for %s: %v", regModel, err)
				} else {
					log.Printf("  Successfully updated OCI artifacts for: %s", regModel)
				}
			}

			matchCount++
		}

	}

	// Clean up the old enriched metadata file if it exists
	_ = os.Remove("data/enriched-model-metadata.yaml")

	enrichmentRate := float64(matchCount) / float64(len(regModels)) * 100

	log.Printf("Metadata enrichment complete:")
	log.Printf("- Total registry models: %d", len(regModels))
	log.Printf("- Successfully enriched: %d (%.1f%%)", matchCount, enrichmentRate)
	log.Printf("- Individual metadata.yaml files have been updated with enriched data")

	return nil
}

// UpdateAllModelsWithOCIArtifacts updates all existing models with OCI artifact metadata
func UpdateAllModelsWithOCIArtifacts(modelsIndexPath, outputDir string) error {
	log.Println("Updating all existing models with OCI artifact metadata...")

	// Load all models from the index
	regModels, err := config.LoadModelsFromYAML(modelsIndexPath)
	if err != nil {
		return fmt.Errorf("failed to load registry models: %v", err)
	}

	updateCount := 0

	// Update each model that has existing metadata
	for _, regModel := range regModels {
		// Check if metadata file exists
		sanitizedName := utils.SanitizeManifestRef(regModel)
		metadataPath := fmt.Sprintf("%s/%s/models/metadata.yaml", outputDir, sanitizedName)

		if _, err := os.Stat(metadataPath); err == nil {
			log.Printf("  Updating OCI artifacts for: %s", regModel)
			err = UpdateOCIArtifacts(regModel, outputDir)
			if err != nil {
				log.Printf("  Warning: Failed to update OCI artifacts for %s: %v", regModel, err)
			} else {
				log.Printf("  Successfully updated OCI artifacts for: %s", regModel)
				updateCount++
			}
		} else {
			log.Printf("  No existing metadata found for: %s", regModel)
		}
	}

	log.Printf("OCI artifacts update complete:")
	log.Printf("- Total models checked: %d", len(regModels))
	log.Printf("- Successfully updated: %d", updateCount)

	return nil
}

// UpdateOCIArtifacts updates the artifacts field with proper OCI metadata for existing models
func UpdateOCIArtifacts(registryModel, outputDir string) error {
	// Load existing metadata
	existingMetadata, err := metadata.LoadExistingMetadata(registryModel, outputDir)
	if err != nil {
		return fmt.Errorf("failed to load existing metadata: %v", err)
	}

	// Generate OCI artifacts from the registry model reference
	ociArtifacts := registry.ExtractOCIArtifactsFromRegistry(registryModel)

	// Preserve existing timestamp data when updating artifacts
	for i := range ociArtifacts {
		if i < len(existingMetadata.Artifacts) {
			// Preserve timestamps from existing artifacts if they exist
			if existingMetadata.Artifacts[i].CreateTimeSinceEpoch != nil {
				ociArtifacts[i].CreateTimeSinceEpoch = existingMetadata.Artifacts[i].CreateTimeSinceEpoch
			}
			if existingMetadata.Artifacts[i].LastUpdateTimeSinceEpoch != nil {
				ociArtifacts[i].LastUpdateTimeSinceEpoch = existingMetadata.Artifacts[i].LastUpdateTimeSinceEpoch
			}
		}
	}

	existingMetadata.Artifacts = ociArtifacts

	// Write updated metadata back to file
	sanitizedName := utils.SanitizeManifestRef(registryModel)
	metadataPath := fmt.Sprintf("%s/%s/models/metadata.yaml", outputDir, sanitizedName)

	updatedData, err := yaml.Marshal(existingMetadata)
	if err != nil {
		return fmt.Errorf("failed to marshal updated metadata: %v", err)
	}

	err = os.WriteFile(metadataPath, updatedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated metadata: %v", err)
	}

	return nil
}
