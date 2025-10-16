package types

import (
	"time"

	"gopkg.in/yaml.v3"
)

// ModelEntry represents a single model entry in the models index
type ModelEntry struct {
	Type   string   `yaml:"type"`   // "oci" for registry-based modelcar or "hf" for HuggingFace models
	URI    string   `yaml:"uri"`    // OCI link or HuggingFace link
	Labels []string `yaml:"labels"` // Labels for the model (e.g., "validated", "featured", "lab-teacher", "lab-base")
}

// ModelsConfig represents the configuration of models to process
type ModelsConfig struct {
	Models []ModelEntry `yaml:"models"`
}

// HuggingFace Collection structures
type HFCollection struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Items       []HFModel `json:"items"`
}

type HFModel struct {
	ID     string `json:"id"`
	Author string `json:"author"`
	Type   string `json:"type"`
	Gated  bool   `json:"gated"`
}

// HuggingFace model details
type HFModelDetails struct {
	ID           string    `json:"id"`
	Author       string    `json:"author"`
	Sha          string    `json:"sha"`
	Downloads    int       `json:"downloads"`
	Likes        int       `json:"likes"`
	Private      bool      `json:"private"`
	Gated        bool      `json:"gated"`
	Tags         []string  `json:"tags"`
	Description  string    `json:"description"`
	License      string    `json:"license"`
	CreatedAt    time.Time `json:"createdAt"`
	LastModified string    `json:"lastModified"`
}

// Version-specific index structures
type ModelIndex struct {
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`
	ReadmePath string `yaml:"readme_path"`
}

type VersionIndex struct {
	Version string       `yaml:"version"`
	Models  []ModelIndex `yaml:"models"`
}

// OCIArtifact represents a structured OCI artifact with metadata
type OCIArtifact struct {
	URI                      string                 `yaml:"uri"`
	CreateTimeSinceEpoch     *int64                 `yaml:"createTimeSinceEpoch"`
	LastUpdateTimeSinceEpoch *int64                 `yaml:"lastUpdateTimeSinceEpoch"`
	CustomProperties         map[string]interface{} `yaml:"customProperties,omitempty"`
}

// ExtractedMetadata represents the actual extracted values from the modelcard
type ExtractedMetadata struct {
	Name                     *string       `yaml:"name"`
	Provider                 *string       `yaml:"provider"`
	Description              *string       `yaml:"description"`
	Readme                   *string       `yaml:"readme"`
	Language                 []string      `yaml:"language"`
	License                  *string       `yaml:"license"`
	LicenseLink              *string       `yaml:"licenseLink"`
	Tags                     []string      `yaml:"tags"`
	Tasks                    []string      `yaml:"tasks"`
	CreateTimeSinceEpoch     *int64        `yaml:"createTimeSinceEpoch"`
	LastUpdateTimeSinceEpoch *int64        `yaml:"lastUpdateTimeSinceEpoch"`
	ValidatedOn              []string      `yaml:"validatedOn"`
	Artifacts                []OCIArtifact `yaml:"artifacts"`
}

// LegacyExtractedMetadata represents the old format with string artifacts
type LegacyExtractedMetadata struct {
	Name                     *string  `yaml:"name"`
	Provider                 *string  `yaml:"provider"`
	Description              *string  `yaml:"description"`
	Readme                   *string  `yaml:"readme"`
	Language                 []string `yaml:"language"`
	License                  *string  `yaml:"license"`
	LicenseLink              *string  `yaml:"licenseLink"`
	Tags                     []string `yaml:"tags"`
	Tasks                    []string `yaml:"tasks"`
	CreateTimeSinceEpoch     *int64   `yaml:"createTimeSinceEpoch"`
	LastUpdateTimeSinceEpoch *int64   `yaml:"lastUpdateTimeSinceEpoch"`
	ValidatedOn              []string `yaml:"validatedOn"`
	Artifacts                []string `yaml:"artifacts"`
}

// MixedTypeExtractedMetadata handles both string and int64 timestamps
type MixedTypeExtractedMetadata struct {
	Name                     *string       `yaml:"name"`
	Provider                 *string       `yaml:"provider"`
	Description              *string       `yaml:"description"`
	Readme                   *string       `yaml:"readme"`
	Language                 []string      `yaml:"language"`
	License                  *string       `yaml:"license"`
	LicenseLink              *string       `yaml:"licenseLink"`
	Tags                     []string      `yaml:"tags"`
	Tasks                    []string      `yaml:"tasks"`
	CreateTimeSinceEpoch     interface{}   `yaml:"createTimeSinceEpoch"`
	LastUpdateTimeSinceEpoch interface{}   `yaml:"lastUpdateTimeSinceEpoch"`
	ValidatedOn              []string      `yaml:"validatedOn"`
	Artifacts                []OCIArtifact `yaml:"artifacts"`
}

// MetadataSource represents where a piece of metadata came from
type MetadataSource struct {
	Value  interface{} `json:"value"`
	Source string      `json:"source"`
}

// EnrichedMetadata contains metadata with source tracking
type EnrichedMetadata struct {
	Name         MetadataSource `json:"name"`
	Provider     MetadataSource `json:"provider"`
	Description  MetadataSource `json:"description"`
	License      MetadataSource `json:"license"`
	LastModified MetadataSource `json:"lastModified"`
	Tags         MetadataSource `json:"tags"`
	Downloads    MetadataSource `json:"downloads"`
	Likes        MetadataSource `json:"likes"`
}

// EnrichedModelMetadata represents enriched metadata for a registry model
type EnrichedModelMetadata struct {
	RegistryModel    string `yaml:"registry_model"`
	HuggingFaceModel string `yaml:"huggingface_model,omitempty"`
	HuggingFaceURL   string `yaml:"huggingface_url,omitempty"`
	ReadmePath       string `yaml:"readme_path,omitempty"`
	MatchConfidence  string `yaml:"match_confidence,omitempty"`
	EnrichmentStatus string `yaml:"enrichment_status"`
	// Metadata with source tracking
	Name                 MetadataSource `yaml:"name"`
	Provider             MetadataSource `yaml:"provider"`
	Description          MetadataSource `yaml:"description"`
	License              MetadataSource `yaml:"license"`
	LicenseLink          MetadataSource `yaml:"license_link"`
	Language             MetadataSource `yaml:"language"`
	LastModified         MetadataSource `yaml:"last_modified"`
	CreateTimeSinceEpoch MetadataSource `yaml:"create_time_since_epoch"`
	Tags                 MetadataSource `yaml:"tags"`
	Tasks                MetadataSource `yaml:"tasks"`
	Downloads            MetadataSource `yaml:"downloads"`
	Likes                MetadataSource `yaml:"likes"`
	ModelSize            MetadataSource `yaml:"model_size"`
	ValidatedOn          MetadataSource `yaml:"validated_on"`
}

// EnrichmentInfo tracks data sources for metadata fields
type EnrichmentInfo struct {
	DataSources struct {
		Name                     string `json:"name"`
		Provider                 string `json:"provider"`
		Description              string `json:"description"`
		Readme                   string `json:"readme"`
		Language                 string `json:"language"`
		License                  string `json:"license"`
		LicenseLink              string `json:"licenseLink"`
		Tags                     string `json:"tags"`
		Tasks                    string `json:"tasks"`
		CreateTimeSinceEpoch     string `json:"createTimeSinceEpoch"`
		LastUpdateTimeSinceEpoch string `json:"lastUpdateTimeSinceEpoch"`
		Artifacts                string `json:"artifacts"`
		ValidatedOn              string `json:"validatedOn"`
	} `json:"dataSources"`
}

// MetadataValue represents a metadata value with type information
type MetadataValue struct {
	MetadataType string `yaml:"metadataType"`
	StringValue  string `yaml:"string_value"`
}

// MarshalYAML implements yaml.Marshaler to force string values to be quoted
func (mv MetadataValue) MarshalYAML() (interface{}, error) {
	// Create a map that will be marshaled with explicit string quoting for string_value
	result := map[string]interface{}{
		"metadataType": mv.MetadataType,
	}

	// Force string_value to be quoted by using a yaml.Node with style set to DoubleQuotedStyle
	if mv.StringValue != "" {
		stringNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: mv.StringValue,
			Style: yaml.DoubleQuotedStyle,
		}
		result["string_value"] = stringNode
	} else {
		result["string_value"] = mv.StringValue
	}

	return result, nil
}

// CatalogOCIArtifact represents an OCI artifact for catalog output with string timestamps
type CatalogOCIArtifact struct {
	URI                      string                 `yaml:"uri"`
	CreateTimeSinceEpoch     *string                `yaml:"createTimeSinceEpoch"`
	LastUpdateTimeSinceEpoch *string                `yaml:"lastUpdateTimeSinceEpoch"`
	CustomProperties         map[string]interface{} `yaml:"customProperties,omitempty"`
}

// CatalogMetadata represents metadata for the catalog output without tags field
type CatalogMetadata struct {
	Name                     *string                  `yaml:"name"`
	Provider                 *string                  `yaml:"provider"`
	Description              *string                  `yaml:"description"`
	Readme                   *string                  `yaml:"readme"`
	Language                 []string                 `yaml:"language"`
	License                  *string                  `yaml:"license"`
	LicenseLink              *string                  `yaml:"licenseLink"`
	Tasks                    []string                 `yaml:"tasks"`
	CreateTimeSinceEpoch     *string                  `yaml:"createTimeSinceEpoch"`
	LastUpdateTimeSinceEpoch *string                  `yaml:"lastUpdateTimeSinceEpoch"`
	CustomProperties         map[string]MetadataValue `yaml:"customProperties,omitempty"`
	Artifacts                []CatalogOCIArtifact     `yaml:"artifacts"`
	Logo                     *string                  `yaml:"logo,omitempty"`
}

// ModelsCatalog represents the aggregated catalog of all models
type ModelsCatalog struct {
	Source string            `yaml:"source"`
	Models []CatalogMetadata `yaml:"models"`
}

// Config represents the application configuration
type Config struct {
	ModelsIndexPath   string
	OutputDir         string
	CatalogOutputPath string
	MaxConcurrent     int
}

// ModelMetadata tracks metadata presence in modelcard
type ModelMetadata struct {
	Name                     bool `yaml:"name"`
	Provider                 bool `yaml:"provider"`
	Description              bool `yaml:"description"`
	Readme                   bool `yaml:"readme"`
	Language                 bool `yaml:"language"`
	License                  bool `yaml:"license"`
	LicenseLink              bool `yaml:"licenseLink"`
	Tags                     bool `yaml:"tags"`
	Tasks                    bool `yaml:"tasks"`
	CreateTimeSinceEpoch     bool `yaml:"createTimeSinceEpoch"`
	LastUpdateTimeSinceEpoch bool `yaml:"lastUpdateTimeSinceEpoch"`
	Artifacts                bool `yaml:"artifacts"`
}

// ModelCard represents a model card structure
type ModelCard struct {
	Present  bool          `yaml:"present"`
	Metadata ModelMetadata `yaml:"metadata"`
}

// ModelManifest represents a model manifest entry
type ModelManifest struct {
	Ref       string    `yaml:"ref"`
	ModelCard ModelCard `yaml:"modelcard"`
}

// ManifestsData represents the collection of all manifests
type ManifestsData struct {
	Models []ModelManifest `yaml:"models"`
}
