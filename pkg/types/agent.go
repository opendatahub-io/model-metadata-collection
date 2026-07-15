package types

// AgentIndexEntry represents an entry in the agents index file.
// For agents with agent.yaml upstream, only Path is needed — metadata is fetched.
// For deployment-only agents without agent.yaml, metadata fields are provided inline.
type AgentIndexEntry struct {
	Path        string `yaml:"path"`
	ReadmePath  string `yaml:"readmePath,omitempty"`
	Name        string `yaml:"name,omitempty"`
	DisplayName string `yaml:"displayName,omitempty"`
	Framework   string `yaml:"framework,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// AgentsIndex represents the agents index file structure.
type AgentsIndex struct {
	Source     string            `yaml:"source"`
	Repository string            `yaml:"repository"`
	Branch     string            `yaml:"branch"`
	Agents     []AgentIndexEntry `yaml:"agents"`
}

// UpstreamAgentYAML represents the known fields from agent.yaml in the
// agentic-starter-kits repo. Any fields not listed here are captured in
// Extra and forwarded as customProperties in the catalog output.
type UpstreamAgentYAML struct {
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"displayName"`
	Framework   string   `yaml:"framework"`
	Description string   `yaml:"description"`
	Labels      []string `yaml:"labels"`
	Logo        string   `yaml:"logo"`
	Env         struct {
		Required []string `yaml:"required"`
		Optional []string `yaml:"optional"`
	} `yaml:"env"`
	Extra      map[string]interface{} `yaml:"-"`
	RawContent map[string]interface{} `yaml:"-"`
}

// KnownUpstreamFields lists the agent.yaml keys that are handled explicitly
// and should NOT be forwarded into customProperties.
var KnownUpstreamFields = map[string]bool{
	"name": true, "displayName": true, "framework": true,
	"description": true, "labels": true, "logo": true, "env": true,
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

// AgentTemplate represents a deployment template artifact for an agent.
// Content holds the full agent.yaml specification as a JSON-encoded string.
type AgentTemplate struct {
	Name    string `yaml:"name,omitempty"`
	Content string `yaml:"content"`
}

// AgentMetadata represents the full metadata for a single agent in the catalog.
type AgentMetadata struct {
	Name                     string                   `yaml:"name"`
	ExternalID               string                   `yaml:"externalId,omitempty"`
	DisplayName              string                   `yaml:"displayName"`
	Description              string                   `yaml:"description"`
	Readme                   string                   `yaml:"readme,omitempty"`
	RepositoryUrl            string                   `yaml:"repositoryUrl,omitempty"`
	Framework                string                   `yaml:"framework"`
	Labels                   []string                 `yaml:"labels,omitempty"`
	Logo                     string                   `yaml:"logo,omitempty"`
	Env                      []AgentEnvVar            `yaml:"env,omitempty"`
	Artifacts                []AgentArtifact          `yaml:"artifacts,omitempty"`
	Templates                []AgentTemplate          `yaml:"templates,omitempty"`
	CustomProperties         map[string]MetadataValue `yaml:"customProperties,omitempty"`
	CreateTimeSinceEpoch     string                   `yaml:"createTimeSinceEpoch,omitempty"`
	LastUpdateTimeSinceEpoch string                   `yaml:"lastUpdateTimeSinceEpoch,omitempty"`
}

// AgentsCatalog represents the aggregated catalog of agents.
type AgentsCatalog struct {
	Source string          `yaml:"source"`
	Agents []AgentMetadata `yaml:"agents"`
}
