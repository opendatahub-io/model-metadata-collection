package types

// MCPServerEntry represents an entry in the MCP servers index file
type MCPServerEntry struct {
	Name      string `yaml:"name"`
	InputPath string `yaml:"input_path"`
}

// MCPServersIndex represents the MCP servers index file structure
type MCPServersIndex struct {
	Source     string           `yaml:"source"`
	MCPServers []MCPServerEntry `yaml:"mcp_servers"`
}

// MCPServerMetadata represents the full metadata for a single MCP server
type MCPServerMetadata struct {
	Name                     string                   `yaml:"name"`
	Provider                 string                   `yaml:"provider"`
	License                  string                   `yaml:"license"`
	LicenseLink              string                   `yaml:"license_link"`
	Description              string                   `yaml:"description"`
	Readme                   string                   `yaml:"readme,omitempty"`
	Version                  string                   `yaml:"version"`
	Transports               []string                 `yaml:"transports,omitempty"`
	Logo                     string                   `yaml:"logo,omitempty"`
	DocumentationUrl         string                   `yaml:"documentationUrl,omitempty"`
	RepositoryUrl            string                   `yaml:"repositoryUrl,omitempty"`
	SourceCode               string                   `yaml:"sourceCode,omitempty"`
	PublishedDate            string                   `yaml:"publishedDate,omitempty"`
	Tools                    []MCPTool                `yaml:"tools,omitempty"`
	Artifacts                []MCPArtifact            `yaml:"artifacts,omitempty"`
	RuntimeMetadata          map[string]any           `yaml:"runtimeMetadata,omitempty"`
	SecurityIndicators       map[string]any           `yaml:"securityIndicators,omitempty"`
	CustomProperties         map[string]MetadataValue `yaml:"customProperties,omitempty"`
	CreateTimeSinceEpoch     string                   `yaml:"createTimeSinceEpoch,omitempty"`
	LastUpdateTimeSinceEpoch string                   `yaml:"lastUpdateTimeSinceEpoch,omitempty"`
}

// MCPTool represents a tool exposed by an MCP server
type MCPTool struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	AccessType  string         `yaml:"accessType"`
	Parameters  []MCPParameter `yaml:"parameters"`
}

// MCPParameter represents a parameter for an MCP tool
type MCPParameter struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// MCPArtifact represents a container artifact for an MCP server
type MCPArtifact struct {
	URI                      string `yaml:"uri"`
	CreateTimeSinceEpoch     string `yaml:"createTimeSinceEpoch,omitempty"`
	LastUpdateTimeSinceEpoch string `yaml:"lastUpdateTimeSinceEpoch,omitempty"`
}

// MCPServersCatalog represents the aggregated catalog of MCP servers
type MCPServersCatalog struct {
	Source     string              `yaml:"source"`
	MCPServers []MCPServerMetadata `yaml:"mcp_servers"`
}
