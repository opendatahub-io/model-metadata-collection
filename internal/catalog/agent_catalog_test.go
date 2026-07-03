package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

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
		Branch:    "main",
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
		Branch:    "main",
		Agents:    []types.AgentIndexEntry{},
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
		Branch:    "main",
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
		Branch:    "main",
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
