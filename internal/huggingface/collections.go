package huggingface

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

// parseVersionFromTitle extracts version from collection title using semver patterns and date patterns
func parseVersionFromTitle(title string) string {
	// Look for version patterns like "v1.0", "v2.1", "v1.0.0", etc.
	versionRegex := regexp.MustCompile(`v(\d+\.\d+(?:\.\d+)?)`)
	matches := versionRegex.FindStringSubmatch(strings.ToLower(title))
	if len(matches) > 1 {
		return "v" + matches[1]
	}

	// Look for date-based patterns like "May 2025", "September 2025", etc.
	dateRegex := regexp.MustCompile(`(?i)(january|february|march|april|may|june|july|august|september|october|november|december)\s+(\d{4})`)
	dateMatches := dateRegex.FindStringSubmatch(title)
	if len(dateMatches) > 2 {
		month := strings.ToLower(dateMatches[1])
		year := dateMatches[2]

		// Map months to numbers for version ordering
		monthMap := map[string]string{
			"january": "01", "february": "02", "march": "03", "april": "04",
			"may": "05", "june": "06", "july": "07", "august": "08",
			"september": "09", "october": "10", "november": "11", "december": "12",
		}

		if monthNum, exists := monthMap[month]; exists {
			return fmt.Sprintf("v%s.%s", year, monthNum)
		}
	}

	return ""
}

// generateVersionIndex creates an index file for a specific version
func generateVersionIndex(collection *types.HFCollection, version string) error {
	var models []types.ModelIndex

	for _, model := range collection.Items {
		// Create model index entry
		modelIndex := types.ModelIndex{
			Name:       model.ID,
			URL:        fmt.Sprintf("https://huggingface.co/%s", model.ID),
			ReadmePath: fmt.Sprintf("/%s/README.md", model.ID),
		}
		models = append(models, modelIndex)
	}

	versionIndex := types.VersionIndex{
		Version: version,
		Models:  models,
	}

	// Ensure data directory exists
	err := os.MkdirAll("data", 0755)
	if err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Generate filename
	filename := fmt.Sprintf("data/hugging-face-redhat-ai-validated-%s.yaml", strings.ReplaceAll(version, ".", "-"))

	// Marshal to YAML
	yamlData, err := yaml.Marshal(versionIndex)
	if err != nil {
		return fmt.Errorf("failed to marshal version index to YAML: %v", err)
	}

	// Write to file
	err = os.WriteFile(filename, yamlData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write version index file: %v", err)
	}

	log.Printf("Generated index file: %s with %d models", filename, len(models))
	return nil
}

// generateMergedIndex creates a merged index file from all processed collections
func generateMergedIndex() error {
	// Find all version index files
	files, err := filepath.Glob("data/hugging-face-redhat-ai-validated-v*.yaml")
	if err != nil {
		return fmt.Errorf("failed to find version index files: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no version index files found to merge")
	}

	// Collect all models from all versions, deduplicating by name
	allModels := make(map[string]types.ModelIndex)
	latestVersion := ""

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Failed to read %s: %v", file, err)
			continue
		}

		var versionIndex types.VersionIndex
		err = yaml.Unmarshal(data, &versionIndex)
		if err != nil {
			log.Printf("Failed to parse %s: %v", file, err)
			continue
		}

		// Track the latest version for the merged index
		if latestVersion == "" || versionIndex.Version > latestVersion {
			latestVersion = versionIndex.Version
		}

		// Add models, newer versions overwrite older ones for same model name
		for _, model := range versionIndex.Models {
			allModels[model.Name] = model
		}
	}

	// Convert map back to slice
	var mergedModels []types.ModelIndex
	for _, model := range allModels {
		mergedModels = append(mergedModels, model)
	}

	// Sort models by name for consistent output
	sort.Slice(mergedModels, func(i, j int) bool {
		return mergedModels[i].Name < mergedModels[j].Name
	})

	// Create merged index
	mergedIndex := types.VersionIndex{
		Version: latestVersion,
		Models:  mergedModels,
	}

	// Write merged index as the latest version file
	filename := fmt.Sprintf("data/hugging-face-redhat-ai-validated-%s.yaml", strings.ReplaceAll(latestVersion, ".", "-"))
	yamlData, err := yaml.Marshal(mergedIndex)
	if err != nil {
		return fmt.Errorf("failed to marshal merged index to YAML: %v", err)
	}

	err = os.WriteFile(filename, yamlData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write merged index file: %v", err)
	}

	log.Printf("Generated merged index file: %s with %d unique models", filename, len(mergedModels))
	return nil
}

// ProcessCollections processes all HuggingFace collections and generates index files
func ProcessCollections() error {
	log.Println("Discovering Red Hat AI validated model collections...")

	// Try to discover collections automatically
	collectionSlugs, err := DiscoverValidatedModelCollections()
	if err != nil {
		log.Printf("Failed to discover collections, using known collections: %v", err)
		// Fall back to known collections - include May, September, October 2025 and January 2026
		collectionSlugs = []string{
			"RedHatAI/red-hat-ai-validated-models-may-2025-682613dc19c4a596dbac9437",
			"RedHatAI/red-hat-ai-validated-models-september-2025-68cc3d7a8a272f6beae3e9a7",
			"RedHatAI/red-hat-ai-validated-models-october-2025-68ed0a23ec5ce4b0ffc4c60c",
			"RedHatAI/red-hat-ai-validated-models-january-2026-69652094dc3429e12c32ad49",
		}
	}

	if len(collectionSlugs) == 0 {
		return fmt.Errorf("no validated model collections found")
	}

	var processedCollections []string

	// Process each discovered collection
	for _, slug := range collectionSlugs {
		log.Printf("Processing collection: %s", slug)

		collection, err := FetchCollectionDetails(slug)
		if err != nil {
			log.Printf("Failed to fetch collection details for %s: %v", slug, err)
			continue
		}

		log.Printf("Found collection: %s", collection.Title)

		// Parse version from title
		version := parseVersionFromTitle(collection.Title)
		if version == "" {
			version = "v1.0" // Default fallback
		}

		log.Printf("Detected version: %s", version)

		// Generate index file for this version
		err = generateVersionIndex(collection, version)
		if err != nil {
			log.Printf("Failed to generate version index for %s: %v", version, err)
			continue
		}

		processedCollections = append(processedCollections, version)
	}

	// Generate merged index from all processed collections
	if len(processedCollections) > 1 {
		log.Println("Generating merged index from multiple collections...")
		err = generateMergedIndex()
		if err != nil {
			log.Printf("Warning: Failed to generate merged index: %v", err)
		}
	}

	return nil
}
