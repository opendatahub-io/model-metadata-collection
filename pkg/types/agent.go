package types

// AgentIndexEntry represents an entry in the agents index file.
// For agents with agent.yaml upstream, only Path is needed — metadata is fetched.
// For deployment-only agents without agent.yaml, metadata fields are provided inline.
type AgentIndexEntry struct {
	Path        string `yaml:"path"`
	Name        string `yaml:"name,omitempty"`
	DisplayName string `yaml:"displayName,omitempty"`
	Framework   string `yaml:"framework,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// AgentsIndex represents the agents index file structure.
type AgentsIndex struct {
	Source     string            `yaml:"source"`
	Repository string           `yaml:"repository"`
	Branch    string             `yaml:"branch"`
	Agents    []AgentIndexEntry  `yaml:"agents"`
}

// UpstreamAgentYAML represents the agent.yaml format in the agentic-starter-kits repo.
type UpstreamAgentYAML struct {
	Name        string `yaml:"name"`
	DisplayName string `yaml:"displayName"`
	Framework   string `yaml:"framework"`
	Description string `yaml:"description"`
	Env         struct {
		Required []string `yaml:"required"`
		Optional []string `yaml:"optional"`
	} `yaml:"env"`
}

// AgentEnvVar represents an environment variable for an agent.
type AgentEnvVar struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
}

// AgentArtifact represents a container artifact for an agent.
type AgentArtifact struct {
	URI                      string `yaml:"uri"`
	CreateTimeSinceEpoch     string `yaml:"createTimeSinceEpoch,omitempty"`
	LastUpdateTimeSinceEpoch string `yaml:"lastUpdateTimeSinceEpoch,omitempty"`
}

// AgentMetadata represents the full metadata for a single agent in the catalog.
type AgentMetadata struct {
	Name                     string                   `yaml:"name"`
	ExternalID               string                   `yaml:"externalId,omitempty"`
	DisplayName              string                   `yaml:"displayName"`
	Description              string                   `yaml:"description"`
	Readme                   string                   `yaml:"readme,omitempty"`
	RepositoryUrl            string                   `yaml:"repositoryUrl,omitempty"`
	PublishedDate            string                   `yaml:"publishedDate,omitempty"`
	Framework                string                   `yaml:"framework"`
	AgentType                string                   `yaml:"agentType"`
	Tags                     []string                 `yaml:"tags,omitempty"`
	Models                   []string                 `yaml:"models,omitempty"`
	Logo                     string                   `yaml:"logo,omitempty"`
	Env                      []AgentEnvVar            `yaml:"env,omitempty"`
	Artifacts                []AgentArtifact          `yaml:"artifacts,omitempty"`
	CustomProperties         map[string]MetadataValue `yaml:"customProperties,omitempty"`
	CreateTimeSinceEpoch     string                   `yaml:"createTimeSinceEpoch,omitempty"`
	LastUpdateTimeSinceEpoch string                   `yaml:"lastUpdateTimeSinceEpoch,omitempty"`
}

// AgentsCatalog represents the aggregated catalog of agents.
type AgentsCatalog struct {
	Source string          `yaml:"source"`
	Agents []AgentMetadata `yaml:"agents"`
}
