package github

import (
	"errors"
	"testing"
)

func TestRawGitHubURLConstruction(t *testing.T) {
	tests := []struct {
		repo       string
		branch     string
		agentPath  string
		wantYAML   string
		wantREADME string
	}{
		{
			repo:       "red-hat-data-services/agentic-starter-kits",
			branch:     "main",
			agentPath:  "agents/langgraph/templates/react_agent",
			wantYAML:   "https://raw.githubusercontent.com/red-hat-data-services/agentic-starter-kits/main/agents/langgraph/templates/react_agent/agent.yaml",
			wantREADME: "https://raw.githubusercontent.com/red-hat-data-services/agentic-starter-kits/main/agents/langgraph/templates/react_agent/README.md",
		},
		{
			repo:       "org/repo",
			branch:     "develop",
			agentPath:  "agents/test",
			wantYAML:   "https://raw.githubusercontent.com/org/repo/develop/agents/test/agent.yaml",
			wantREADME: "https://raw.githubusercontent.com/org/repo/develop/agents/test/README.md",
		},
	}

	for _, tc := range tests {
		yamlURL := buildRawURL(tc.repo, tc.branch, tc.agentPath, "agent.yaml")
		if yamlURL != tc.wantYAML {
			t.Errorf("YAML URL mismatch:\n  got:  %s\n  want: %s", yamlURL, tc.wantYAML)
		}

		readmeURL := buildRawURL(tc.repo, tc.branch, tc.agentPath, "README.md")
		if readmeURL != tc.wantREADME {
			t.Errorf("README URL mismatch:\n  got:  %s\n  want: %s", readmeURL, tc.wantREADME)
		}
	}
}

const testRepo = "red-hat-data-services/agentic-starter-kits"

func TestValidateBranchMain(t *testing.T) {

	err := ValidateBranch(testRepo, "main")
	if err != nil {
		t.Fatalf("expected main branch to be valid, got error: %v", err)
	}
}

func TestValidateBranchNonExistent(t *testing.T) {

	err := ValidateBranch(testRepo, "this-branch-does-not-exist-xyz-12345")
	if err == nil {
		t.Fatal("expected error for non-existent branch")
	}
}

func TestFetchAgentYAMLMainBranch(t *testing.T) {

	agent, err := FetchAgentYAML(testRepo, "main", "agents/langgraph/templates/react_agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.Name != "langgraph-react-agent" {
		t.Errorf("expected name 'langgraph-react-agent', got %q", agent.Name)
	}
	if agent.Framework != "langgraph" {
		t.Errorf("expected framework 'langgraph', got %q", agent.Framework)
	}
	if len(agent.Env.Required) == 0 {
		t.Error("expected at least one required env var")
	}
}

func TestFetchAgentYAMLNonExistentBranch(t *testing.T) {

	_, err := FetchAgentYAML(testRepo, "this-branch-does-not-exist-xyz-12345", "agents/langgraph/templates/react_agent")
	if err == nil {
		t.Fatal("expected error for non-existent branch")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestFetchAgentYAMLNonExistentPath(t *testing.T) {

	_, err := FetchAgentYAML(testRepo, "main", "agents/does-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestFetchReadmeMainBranch(t *testing.T) {

	readme, err := FetchReadme(testRepo, "main", "agents/langgraph/templates/react_agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readme == "" {
		t.Error("expected non-empty README content")
	}
}

func TestFetchReadmeNonExistentBranch(t *testing.T) {

	readme, err := FetchReadme(testRepo, "this-branch-does-not-exist-xyz-12345", "agents/langgraph/templates/react_agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readme != "" {
		t.Error("expected empty README for non-existent branch")
	}
}
