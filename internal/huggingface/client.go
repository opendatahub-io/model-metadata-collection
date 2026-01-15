package huggingface

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// FetchCollections fetches collections from HuggingFace
func FetchCollections() ([]types.HFCollection, error) {
	// Fetch collections list from RedHatAI
	resp, err := http.Get("https://huggingface.co/api/collections?search=red-hat-ai-validated-models")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch collections: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var collections []types.HFCollection
	err = json.Unmarshal(body, &collections)
	if err != nil {
		return nil, fmt.Errorf("failed to parse collections JSON: %v", err)
	}

	return collections, nil
}

// FetchCollectionDetails fetches detailed information for a specific collection
func FetchCollectionDetails(collectionID string) (*types.HFCollection, error) {
	url := fmt.Sprintf("https://huggingface.co/api/collections/%s", collectionID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch collection details: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var collection types.HFCollection
	err = json.Unmarshal(body, &collection)
	if err != nil {
		return nil, fmt.Errorf("failed to parse collection JSON: %v", err)
	}

	return &collection, nil
}

// DiscoverValidatedModelCollections finds all Red Hat AI validated model collections
func DiscoverValidatedModelCollections() ([]string, error) {
	// Fetch collections from RedHatAI user
	resp, err := http.Get("https://huggingface.co/api/users/RedHatAI/collections")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user collections: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var collections []types.HFCollection
	err = json.Unmarshal(body, &collections)
	if err != nil {
		return nil, fmt.Errorf("failed to parse collections JSON: %v", err)
	}

	var validatedModelCollections []string
	// Updated patterns to match both old and new collection naming schemes
	validatedPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)red.?hat.?ai.?validated.?models`),
		regexp.MustCompile(`(?i)red\s+hat\s+ai\s+validated\s+models\s+-\s+(january|february|march|april|may|june|july|august|september|october|november|december)\s+\d{4}`),
	}

	for _, collection := range collections {
		for _, pattern := range validatedPatterns {
			if pattern.MatchString(collection.Title) {
				validatedModelCollections = append(validatedModelCollections, collection.Slug)
				break // Avoid duplicates if multiple patterns match
			}
		}
	}

	return validatedModelCollections, nil
}

// FetchModelDetails fetches detailed metadata for a specific model
func FetchModelDetails(modelName string) (*types.HFModelDetails, error) {
	url := fmt.Sprintf("https://huggingface.co/api/models/%s", modelName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch model details: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var details types.HFModelDetails
	err = json.Unmarshal(body, &details)
	if err != nil {
		return nil, fmt.Errorf("failed to parse model details JSON: %v", err)
	}

	return &details, nil
}

// FetchReadme fetches the README content from HuggingFace
func FetchReadme(modelName string) (string, error) {
	url := fmt.Sprintf("https://huggingface.co/%s/raw/main/README.md", modelName)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch README: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("README not found, status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read README body: %v", err)
	}

	return string(body), nil
}

// GetLatestVersionIndexFile finds the latest version index file
func GetLatestVersionIndexFile() (string, error) {
	files, err := filepath.Glob("data/hugging-face-redhat-ai-validated-v*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to find version index files: %v", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no version index files found")
	}

	// Sort files to get the latest version
	sort.Strings(files)
	return files[len(files)-1], nil
}

// LoadModelsFromVersionIndex loads models from a version-specific index file
func LoadModelsFromVersionIndex(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var versionIndex types.VersionIndex
	err = yaml.Unmarshal(data, &versionIndex)
	if err != nil {
		return nil, err
	}

	var modelRefs []string
	for _, model := range versionIndex.Models {
		// Convert HuggingFace model to container registry format
		// This would need to be mapped to actual container registry URLs
		// For now, we'll use a placeholder format
		modelRef := fmt.Sprintf("registry.redhat.io/rhelai1/modelcar-%s", strings.ToLower(strings.ReplaceAll(model.Name, "/", "-")))
		modelRefs = append(modelRefs, modelRef)
	}

	return modelRefs, nil
}

// ExtractProviderFromReadme extracts provider/developer information from README content
func ExtractProviderFromReadme(readmeContent string) string {
	if readmeContent == "" {
		return ""
	}

	// Look for common provider patterns - updated to handle markdown formatting
	providerPatterns := []string{
		`(?i)\*?\*?Model developer[s]?\*?\*?:?\s*\*?\*?([^.\n\*]+)`,
		`(?i)\*?\*?Developer[s]?\*?\*?:?\s*\*?\*?([^.\n\*]+)`,
		`(?i)\*?\*?Created by\*?\*?:?\s*\*?\*?([^.\n\*]+)`,
		`(?i)\*?\*?Author[s]?\*?\*?:?\s*\*?\*?([^.\n\*]+)`,
		`(?i)\*?\*?Provider\*?\*?:?\s*\*?\*?([^.\n\*]+)`,
		`(?i)\*?\*?Model Developers?\*?\*?:?\s*\*?\*?([^.\n\*]+)`,
	}

	for _, pattern := range providerPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(readmeContent); len(matches) > 1 {
			provider := strings.TrimSpace(matches[1])
			provider = utils.CleanExtractedValue(provider)
			// Additional safety trimming to ensure no whitespace remains
			provider = strings.TrimSpace(provider)
			if utils.IsValidValue(provider, 2, 50, []string{`^[A-Za-z\s\\.&,()-]+$`}) {
				return provider
			}
		}
	}

	return ""
}

// stringSlice is a helper type that can unmarshal from either a YAML scalar or sequence
type stringSlice []string

func (s *stringSlice) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var str string
		if err := value.Decode(&str); err != nil {
			return err
		}
		trimmed := strings.TrimSpace(str)
		if trimmed == "" {
			*s = nil
		} else {
			*s = []string{trimmed}
		}
		return nil
	case yaml.SequenceNode:
		var arr []string
		if err := value.Decode(&arr); err != nil {
			return err
		}
		// trim empties and dedupe
		seen := map[string]struct{}{}
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			t := strings.TrimSpace(v)
			if t == "" {
				continue
			}
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			out = append(out, t)
		}
		*s = out
		return nil
	default:
		return fmt.Errorf("stringSlice: unsupported YAML node kind %v", value.Kind)
	}
}

// YAMLFrontmatter represents the YAML frontmatter in HuggingFace README files
type YAMLFrontmatter struct {
	Language    stringSlice `yaml:"language"`
	BaseModel   stringSlice `yaml:"base_model"`
	PipelineTag string      `yaml:"pipeline_tag"`
	License     string      `yaml:"license"`
	LicenseName string      `yaml:"license_name"`
	LicenseLink string      `yaml:"license_link"`
	Tags        []string    `yaml:"tags"`
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Tasks       []string    `yaml:"tasks"`
	Provider    string      `yaml:"provider"`
	ValidatedOn stringSlice `yaml:"validated_on"`
}

// ExtractYAMLFrontmatter extracts YAML frontmatter from README content
func ExtractYAMLFrontmatter(readmeContent string) (*YAMLFrontmatter, error) {
	if readmeContent == "" {
		return nil, fmt.Errorf("empty README content")
	}

	// Check if content starts with YAML frontmatter (---)
	if !strings.HasPrefix(readmeContent, "---") {
		return nil, fmt.Errorf("no YAML frontmatter found")
	}

	// Find the end of the frontmatter
	lines := strings.Split(readmeContent, "\n")
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		return nil, fmt.Errorf("malformed YAML frontmatter: no closing ---")
	}

	// Extract and parse YAML content
	yamlContent := strings.Join(lines[1:endIndex], "\n")
	var frontmatter YAMLFrontmatter
	err := yaml.Unmarshal([]byte(yamlContent), &frontmatter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %v", err)
	}

	return &frontmatter, nil
}

// ExtractReleaseDateFromReadme extracts release date information from README content
func ExtractReleaseDateFromReadme(readmeContent string) string {
	if readmeContent == "" {
		return ""
	}

	// Look for common release date patterns with more flexible matching
	datePatterns := []string{
		// Match any line containing "Release Date" followed by a date
		`(?i).*Release Date.*?([0-9]{1,2}[\/\-][0-9]{1,2}[\/\-][0-9]{4})`,
		`(?i).*Released.*?([0-9]{1,2}[\/\-][0-9]{1,2}[\/\-][0-9]{4})`,
		`(?i).*Launch Date.*?([0-9]{1,2}[\/\-][0-9]{1,2}[\/\-][0-9]{4})`,
		`(?i).*Date.*?([0-9]{1,2}[\/\-][0-9]{1,2}[\/\-][0-9]{4})`,
	}

	for _, pattern := range datePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(readmeContent); len(matches) > 1 {
			date := strings.TrimSpace(matches[1])
			date = utils.CleanExtractedValue(date)
			if date != "" && len(date) >= 8 { // Basic validation for date length
				return date
			}
		}
	}

	return ""
}
