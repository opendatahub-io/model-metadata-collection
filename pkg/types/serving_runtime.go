package types

// ServingRuntimeIndex lists the independently maintained runtime input files.
type ServingRuntimeIndex struct {
	Source          string                     `yaml:"source"`
	ServingRuntimes []ServingRuntimeIndexEntry `yaml:"serving_runtimes"`
}

type ServingRuntimeIndexEntry struct {
	Name      string `yaml:"name"`
	InputPath string `yaml:"input_path"`
}

// ServingRuntimeCatalog is the YAML contract consumed by the serving_runtime loader.
type ServingRuntimeCatalog struct {
	Source          string           `yaml:"source" json:"source"`
	ServingRuntimes []ServingRuntime `yaml:"serving_runtimes" json:"serving_runtimes"`
}

type ServingRuntime struct {
	Name                  string                  `yaml:"name" json:"name"`
	DisplayName           string                  `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Provider              string                  `yaml:"provider,omitempty" json:"provider,omitempty"`
	Description           string                  `yaml:"description,omitempty" json:"description,omitempty"`
	Tags                  []string                `yaml:"tags,omitempty" json:"tags,omitempty"`
	DocumentationURL      string                  `yaml:"documentationUrl,omitempty" json:"documentationUrl,omitempty"`
	RepositoryURL         string                  `yaml:"repositoryUrl,omitempty" json:"repositoryUrl,omitempty"`
	SupportedModelFormats []SupportedModelFormat  `yaml:"supportedModelFormats,omitempty" json:"supportedModelFormats,omitempty"`
	Capabilities          *RuntimeCapabilities    `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Versions              []ServingRuntimeVersion `yaml:"versions" json:"versions"`
}

type ServingRuntimeVersion struct {
	Version               string                         `yaml:"version" json:"version"`
	Image                 string                         `yaml:"image" json:"image"`
	SupportLevel          string                         `yaml:"supportLevel" json:"supportLevel"`
	SupportedModelFormats []SupportedModelFormat         `yaml:"supportedModelFormats,omitempty" json:"supportedModelFormats,omitempty"`
	ProtocolVersions      []string                       `yaml:"protocolVersions,omitempty" json:"protocolVersions,omitempty"`
	RecommendedResources  *RuntimeResourceRecommendation `yaml:"recommendedResources,omitempty" json:"recommendedResources,omitempty"`
	DefaultArgs           []string                       `yaml:"defaultArgs,omitempty" json:"defaultArgs,omitempty"`
	Env                   []RuntimeEnvVar                `yaml:"env,omitempty" json:"env,omitempty"`
	Deprecated            bool                           `yaml:"deprecated,omitempty" json:"deprecated,omitempty"`
}

type SupportedModelFormat struct {
	Name       string `yaml:"name" json:"name"`
	Version    string `yaml:"version,omitempty" json:"version,omitempty"`
	AutoSelect bool   `yaml:"autoSelect,omitempty" json:"autoSelect,omitempty"`
	Priority   *int32 `yaml:"priority,omitempty" json:"priority,omitempty"`
}

type RuntimeCapabilities struct {
	RequiresGPU           bool     `yaml:"requiresGPU,omitempty" json:"requiresGPU,omitempty"`
	SupportedAccelerators []string `yaml:"supportedAccelerators,omitempty" json:"supportedAccelerators,omitempty"`
	MultiModel            bool     `yaml:"multiModel,omitempty" json:"multiModel,omitempty"`
}

type RuntimeResourceRecommendation struct {
	Minimal     *RuntimeResourceTier `yaml:"minimal,omitempty" json:"minimal,omitempty"`
	Recommended *RuntimeResourceTier `yaml:"recommended,omitempty" json:"recommended,omitempty"`
	High        *RuntimeResourceTier `yaml:"high,omitempty" json:"high,omitempty"`
}

type RuntimeResourceTier struct {
	CPU         string            `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	Memory      string            `yaml:"memory,omitempty" json:"memory,omitempty"`
	Accelerator map[string]string `yaml:"accelerator,omitempty" json:"accelerator,omitempty"`
}

type RuntimeEnvVar struct {
	Name         string  `yaml:"name" json:"name"`
	Description  string  `yaml:"description,omitempty" json:"description,omitempty"`
	Required     bool    `yaml:"required,omitempty" json:"required,omitempty"`
	DefaultValue *string `yaml:"defaultValue,omitempty" json:"defaultValue,omitempty"`
	Secret       bool    `yaml:"secret,omitempty" json:"secret,omitempty"`
}
