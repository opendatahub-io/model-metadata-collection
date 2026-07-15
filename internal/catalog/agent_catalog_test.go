package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestResolveReadmeLinks(t *testing.T) {
	base := "https://github.com/org/repo/tree/main/agents/test/"
	tests := []struct {
		name, input, want string
	}{
		{"relative path", "[guide](docs/setup.md)", "[guide](https://github.com/org/repo/tree/main/agents/test/docs/setup.md)"},
		{"relative parent", "[up](../README.md)", "[up](https://github.com/org/repo/tree/main/agents/README.md)"},
		{"relative with anchor", "[section](docs/setup.md#install)", "[section](https://github.com/org/repo/tree/main/agents/test/docs/setup.md#install)"},
		{"absolute https", "[site](https://example.com)", "[site](https://example.com)"},
		{"absolute http", "[site](http://example.com)", "[site](http://example.com)"},
		{"protocol-relative", "[site](//cdn.example.com/a.js)", "[site](//cdn.example.com/a.js)"},
		{"mailto", "[email](mailto:a@b.com)", "[email](mailto:a@b.com)"},
		{"image relative", "![logo](/images/logo.svg)", "logo"},
		{"image absolute", "![logo](https://example.com/logo.svg)", "![logo](https://example.com/logo.svg)"},
		{"mixed", "See [docs](docs/api.md) and [site](https://example.com).", "See [docs](https://github.com/org/repo/tree/main/agents/test/docs/api.md) and [site](https://example.com)."},
		{"no links", "plain text", "plain text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveReadmeLinks(tt.input, base)
			if got != tt.want {
				t.Errorf("resolveReadmeLinks(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildTemplateArtifacts(t *testing.T) {
	t.Run("generates JSON template from raw content", func(t *testing.T) {
		raw := map[string]interface{}{
			"name":        "test-agent",
			"displayName": "Test Agent",
			"framework":   "langgraph",
			"description": "A test agent.",
			"labels":      []interface{}{"tool-calling", "react"},
			"logo":        "data:image/svg+xml;base64,abc123",
		}
		templates := buildTemplateArtifacts(raw)
		if len(templates) != 1 {
			t.Fatalf("expected 1 template, got %d", len(templates))
		}
		if templates[0].Name != "agent.yaml" {
			t.Errorf("expected template name 'agent.yaml', got %q", templates[0].Name)
		}
		// Verify content is valid JSON containing the original fields
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(templates[0].Content), &parsed); err != nil {
			t.Fatalf("template content is not valid JSON: %v", err)
		}
		if parsed["name"] != "test-agent" {
			t.Errorf("expected name 'test-agent' in template content, got %v", parsed["name"])
		}
		if parsed["framework"] != "langgraph" {
			t.Errorf("expected framework 'langgraph' in template content, got %v", parsed["framework"])
		}
	})

	t.Run("returns nil for empty raw content", func(t *testing.T) {
		templates := buildTemplateArtifacts(nil)
		if templates != nil {
			t.Errorf("expected nil templates for nil input, got %v", templates)
		}
		templates = buildTemplateArtifacts(map[string]interface{}{})
		if templates != nil {
			t.Errorf("expected nil templates for empty map, got %v", templates)
		}
	})
}

func TestLabelsLogoMapping(t *testing.T) {
	tmpDir := t.TempDir()

	index := types.AgentsIndex{
		Source:     "Test",
		Repository: "org/repo",
		Branch:     "main",
		Agents: []types.AgentIndexEntry{
			{
				Path:        "agents/test",
				Name:        "test-agent",
				Framework:   "langgraph",
				Description: "A test agent.",
			},
		},
	}
	indexPath := filepath.Join(tmpDir, "index.yaml")
	writeYAML(t, indexPath, index)

	catalogPath := filepath.Join(tmpDir, "catalog.yaml")
	err := CreateAgentsCatalog(indexPath, catalogPath, "", true)
	if err != nil {
		t.Fatalf("CreateAgentsCatalog failed: %v", err)
	}

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog: %v", err)
	}
	var catalog types.AgentsCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("Failed to parse catalog: %v", err)
	}

	agent := catalog.Agents[0]
	// With skipEnrichment=true, labels/logo should be empty (no upstream fetch)
	if len(agent.Labels) != 0 {
		t.Errorf("expected empty labels with skipEnrichment, got %v", agent.Labels)
	}
	if agent.Logo != "" {
		t.Errorf("expected empty logo with skipEnrichment, got %q", agent.Logo)
	}
}

func TestLabelsLogoNotInCustomProperties(t *testing.T) {
	upstream := &types.UpstreamAgentYAML{
		Extra: map[string]interface{}{
			"deploymentModel": "flow-import",
		},
	}
	// Labels and logo should NOT appear in Extra because they are in KnownUpstreamFields
	if types.KnownUpstreamFields["labels"] != true {
		t.Error("expected 'labels' in KnownUpstreamFields")
	}
	if types.KnownUpstreamFields["logo"] != true {
		t.Error("expected 'logo' in KnownUpstreamFields")
	}

	agent := &types.AgentMetadata{}
	forwardExtraAsCustomProperties(agent, upstream.Extra)
	if _, found := agent.CustomProperties["labels"]; found {
		t.Error("labels should not be in customProperties")
	}
	if _, found := agent.CustomProperties["logo"]; found {
		t.Error("logo should not be in customProperties")
	}
	if _, found := agent.CustomProperties["deploymentModel"]; !found {
		t.Error("deploymentModel should be in customProperties")
	}
}

func TestTransformEnvVars(t *testing.T) {
	upstream := &types.UpstreamAgentYAML{}
	upstream.Env.Required = []string{"API_KEY", "BASE_URL", "MODEL_ID"}
	upstream.Env.Optional = []string{"PORT", "CONTAINER_IMAGE"}

	result := transformEnvVars(upstream)

	if len(result) != 5 {
		t.Fatalf("expected 5 env vars, got %d", len(result))
	}

	// Required vars come first
	for i, name := range []string{"API_KEY", "BASE_URL", "MODEL_ID"} {
		if result[i].Name != name || !result[i].Required {
			t.Errorf("env[%d]: expected {%s, true}, got {%s, %v}", i, name, result[i].Name, result[i].Required)
		}
	}

	// Optional vars follow
	for i, name := range []string{"PORT", "CONTAINER_IMAGE"} {
		idx := i + 3
		if result[idx].Name != name || result[idx].Required {
			t.Errorf("env[%d]: expected {%s, false}, got {%s, %v}", idx, name, result[idx].Name, result[idx].Required)
		}
	}
}

func TestTransformEnvVarsEmpty(t *testing.T) {
	upstream := &types.UpstreamAgentYAML{}
	result := transformEnvVars(upstream)
	if len(result) != 0 {
		t.Errorf("expected 0 env vars for empty upstream, got %d", len(result))
	}
}

func TestApplyIndexOverrides(t *testing.T) {
	t.Run("fills empty fields from index", func(t *testing.T) {
		agent := &types.AgentMetadata{}
		entry := types.AgentIndexEntry{
			Name:        "test-agent",
			DisplayName: "Test Agent",
			Framework:   "langgraph",
			Description: "A test agent",
		}
		applyIndexOverrides(agent, entry)

		if agent.Name != "test-agent" {
			t.Errorf("expected name 'test-agent', got %q", agent.Name)
		}
		if agent.DisplayName != "Test Agent" {
			t.Errorf("expected displayName 'Test Agent', got %q", agent.DisplayName)
		}
		if agent.Framework != "langgraph" {
			t.Errorf("expected framework 'langgraph', got %q", agent.Framework)
		}
		if agent.Description != "A test agent" {
			t.Errorf("expected description 'A test agent', got %q", agent.Description)
		}
	})

	t.Run("does not overwrite existing fields", func(t *testing.T) {
		agent := &types.AgentMetadata{
			Name:        "fetched-name",
			DisplayName: "Fetched Display Name",
			Framework:   "crewai",
			Description: "Fetched description",
		}
		entry := types.AgentIndexEntry{
			Name:        "override-name",
			DisplayName: "Override Display Name",
			Framework:   "autogen",
			Description: "Override description",
		}
		applyIndexOverrides(agent, entry)

		if agent.Name != "fetched-name" {
			t.Errorf("expected name 'fetched-name' (not overwritten), got %q", agent.Name)
		}
		if agent.Framework != "crewai" {
			t.Errorf("expected framework 'crewai' (not overwritten), got %q", agent.Framework)
		}
	})
}

func TestCreateAgentsCatalogSkipEnrichment(t *testing.T) {
	tmpDir := t.TempDir()

	// Index with only deployment-only agents (which have inline metadata).
	// With skipEnrichment=true, template agents without overrides will be skipped.
	index := types.AgentsIndex{
		Source:     "Red Hat Agents",
		Repository: "red-hat-data-services/agentic-starter-kits",
		Branch:     "main",
		Agents: []types.AgentIndexEntry{
			{
				Path:        "agents/claude-code",
				Name:        "claude-code",
				DisplayName: "Claude Code on OpenShift",
				Framework:   "claude-code",
				Description: "Deploy Claude Code on OpenShift.",
			},
			{
				Path:        "agents/codex/deployment",
				Name:        "codex",
				DisplayName: "Codex on OpenShift",
				Framework:   "codex",
				Description: "Run Codex in OpenShell sandbox.",
			},
		},
	}
	indexPath := filepath.Join(tmpDir, "index.yaml")
	writeYAML(t, indexPath, index)

	catalogPath := filepath.Join(tmpDir, "catalog.yaml")
	err := CreateAgentsCatalog(indexPath, catalogPath, "", true)
	if err != nil {
		t.Fatalf("CreateAgentsCatalog failed: %v", err)
	}

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog: %v", err)
	}

	var catalog types.AgentsCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("Failed to parse catalog: %v", err)
	}

	if catalog.Source != "Red Hat Agents" {
		t.Errorf("expected source 'Red Hat Agents', got %q", catalog.Source)
	}
	if len(catalog.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(catalog.Agents))
	}

	agent := catalog.Agents[0]
	if agent.Name != "claude-code" {
		t.Errorf("expected name 'claude-code', got %q", agent.Name)
	}
	if agent.RepositoryUrl != "https://github.com/red-hat-data-services/agentic-starter-kits/tree/main/agents/claude-code" {
		t.Errorf("unexpected repositoryUrl: %q", agent.RepositoryUrl)
	}
}

func TestCreateAgentsCatalogMissingIndex(t *testing.T) {
	tmpDir := t.TempDir()
	err := CreateAgentsCatalog(filepath.Join(tmpDir, "nonexistent.yaml"), filepath.Join(tmpDir, "catalog.yaml"), "", true)
	if err == nil {
		t.Fatal("expected error for missing index file")
	}
}

func TestCreateAgentsCatalogMissingRepository(t *testing.T) {
	tmpDir := t.TempDir()

	index := types.AgentsIndex{
		Source: "Test",
		Agents: []types.AgentIndexEntry{{Path: "agents/test"}},
	}
	indexPath := filepath.Join(tmpDir, "index.yaml")
	writeYAML(t, indexPath, index)

	err := CreateAgentsCatalog(indexPath, filepath.Join(tmpDir, "catalog.yaml"), "", true)
	if err == nil {
		t.Fatal("expected error for missing repository field")
	}
}

func TestCreateAgentsCatalogEmptyIndex(t *testing.T) {
	tmpDir := t.TempDir()

	index := types.AgentsIndex{
		Source:     "Empty",
		Repository: "org/repo",
		Branch:     "main",
		Agents:     []types.AgentIndexEntry{},
	}
	indexPath := filepath.Join(tmpDir, "index.yaml")
	writeYAML(t, indexPath, index)

	catalogPath := filepath.Join(tmpDir, "catalog.yaml")
	err := CreateAgentsCatalog(indexPath, catalogPath, "", true)
	if err != nil {
		t.Fatalf("CreateAgentsCatalog failed: %v", err)
	}

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog: %v", err)
	}
	var catalog types.AgentsCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("Failed to parse catalog: %v", err)
	}

	if catalog.Source != "Empty" {
		t.Errorf("expected source 'Empty', got %q", catalog.Source)
	}
	if len(catalog.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(catalog.Agents))
	}
}

func TestCreateAgentsCatalogDefaultBranch(t *testing.T) {
	tmpDir := t.TempDir()

	index := types.AgentsIndex{
		Source:     "Test",
		Repository: "org/repo",
		// Branch intentionally omitted — should default to "main"
		Agents: []types.AgentIndexEntry{
			{
				Path:        "agents/test",
				Name:        "test-agent",
				Framework:   "langgraph",
				Description: "A test agent.",
			},
		},
	}
	indexPath := filepath.Join(tmpDir, "index.yaml")
	writeYAML(t, indexPath, index)

	catalogPath := filepath.Join(tmpDir, "catalog.yaml")
	err := CreateAgentsCatalog(indexPath, catalogPath, "", true)
	if err != nil {
		t.Fatalf("CreateAgentsCatalog failed: %v", err)
	}

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog: %v", err)
	}
	var catalog types.AgentsCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("Failed to parse catalog: %v", err)
	}

	if len(catalog.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(catalog.Agents))
	}
	expected := "https://github.com/org/repo/tree/main/agents/test"
	if catalog.Agents[0].RepositoryUrl != expected {
		t.Errorf("expected repositoryUrl %q, got %q", expected, catalog.Agents[0].RepositoryUrl)
	}
}

func TestCreateAgentsCatalogBranchOverride(t *testing.T) {
	tmpDir := t.TempDir()

	index := types.AgentsIndex{
		Source:     "Test",
		Repository: "org/repo",
		Branch:     "main",
		Agents: []types.AgentIndexEntry{
			{
				Path:        "agents/test",
				Name:        "test-agent",
				Framework:   "langgraph",
				Description: "A test agent.",
			},
		},
	}
	indexPath := filepath.Join(tmpDir, "index.yaml")
	writeYAML(t, indexPath, index)

	catalogPath := filepath.Join(tmpDir, "catalog.yaml")
	err := CreateAgentsCatalog(indexPath, catalogPath, "release-3.5", true)
	if err != nil {
		t.Fatalf("CreateAgentsCatalog failed: %v", err)
	}

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("Failed to read catalog: %v", err)
	}
	var catalog types.AgentsCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("Failed to parse catalog: %v", err)
	}

	if len(catalog.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(catalog.Agents))
	}
	expected := "https://github.com/org/repo/tree/release-3.5/agents/test"
	if catalog.Agents[0].RepositoryUrl != expected {
		t.Errorf("expected repositoryUrl %q, got %q", expected, catalog.Agents[0].RepositoryUrl)
	}
}

func TestCreateAgentsCatalogNonExistentBranch(t *testing.T) {
	tmpDir := t.TempDir()

	index := types.AgentsIndex{
		Source:     "Test",
		Repository: "red-hat-data-services/agentic-starter-kits",
		Branch:     "main",
		Agents: []types.AgentIndexEntry{
			{Path: "agents/langgraph/templates/react_agent"},
		},
	}
	indexPath := filepath.Join(tmpDir, "index.yaml")
	writeYAML(t, indexPath, index)

	catalogPath := filepath.Join(tmpDir, "catalog.yaml")
	err := CreateAgentsCatalog(indexPath, catalogPath, "this-branch-does-not-exist-xyz-12345", false)
	if err == nil {
		t.Fatal("expected error for non-existent branch")
	}
}
