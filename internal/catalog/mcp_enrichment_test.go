package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestEnrichMCPServerArtifacts_EmptyURI(t *testing.T) {
	server := &types.MCPServerMetadata{
		Name:     "test-server",
		Provider: "Test",
		Artifacts: []types.MCPArtifact{
			{URI: ""},
		},
	}

	_, err := enrichMCPServerArtifacts(server)
	if err == nil {
		t.Fatal("expected error for empty URI artifact, got nil")
	}
	if !strings.Contains(err.Error(), "empty URI") {
		t.Errorf("expected 'empty URI' in error message, got: %v", err)
	}
}

func TestEnrichMCPServerArtifacts_NoArtifacts(t *testing.T) {
	server := &types.MCPServerMetadata{
		Name:     "test-server",
		Provider: "Test",
	}

	changed, err := enrichMCPServerArtifacts(server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected no changes for server with no artifacts")
	}
}

func TestEnrichMCPServerArtifacts_PublishedDateDerivation(t *testing.T) {
	server := &types.MCPServerMetadata{
		Name:          "test-server",
		Provider:      "Test",
		PublishedDate: "2025-07-23T00:00:00Z",
	}

	changed, err := enrichMCPServerArtifacts(server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// publishedDate should be converted to createTimeSinceEpoch
	if server.CreateTimeSinceEpoch == "" {
		t.Error("expected createTimeSinceEpoch to be set from publishedDate")
	}
	if !changed {
		t.Error("expected changes when deriving createTimeSinceEpoch from publishedDate")
	}

	// Verify the epoch value: 2025-07-23T00:00:00Z = 1753228800 seconds = 1753228800000 ms
	expected := "1753228800000"
	if server.CreateTimeSinceEpoch != expected {
		t.Errorf("expected createTimeSinceEpoch=%s, got=%s", expected, server.CreateTimeSinceEpoch)
	}
}

func TestEnrichMCPServerArtifacts_PublishedDateIdempotent(t *testing.T) {
	server := &types.MCPServerMetadata{
		Name:                 "test-server",
		Provider:             "Test",
		PublishedDate:        "2025-07-23T00:00:00Z",
		CreateTimeSinceEpoch: "1753228800000",
	}

	changed, err := enrichMCPServerArtifacts(server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected no changes when createTimeSinceEpoch already matches publishedDate")
	}
}

func TestEpochMillisToString(t *testing.T) {
	t.Run("nil returns empty string", func(t *testing.T) {
		result := epochMillisToString(nil)
		if result != "" {
			t.Errorf("expected empty string, got: %s", result)
		}
	})

	t.Run("valid value returns formatted string", func(t *testing.T) {
		val := int64(1753292581000)
		result := epochMillisToString(&val)
		if result != "1753292581000" {
			t.Errorf("expected '1753292581000', got: %s", result)
		}
	})
}

func TestWriteMCPServerInput(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-server.yaml")

	server := &types.MCPServerMetadata{
		Name:        "test-server",
		Provider:    "Test Provider",
		License:     "apache-2.0",
		Description: "A test server",
		Version:     "latest",
		Logo:        "data:image/svg+xml;base64," + strings.Repeat("ABCD", 200),
		CustomProperties: map[string]types.MetadataValue{
			"architecture": {
				MetadataType: "MetadataStringValue",
				StringValue:  `["amd64","arm64"]`,
			},
		},
	}

	err := writeMCPServerInput(outputPath, server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists and ends with newline
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("output file is empty")
	}
	if data[len(data)-1] != '\n' {
		t.Error("output file does not end with newline")
	}

	// Verify round-trip
	var roundTripped types.MCPServerMetadata
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if roundTripped.Name != server.Name {
		t.Errorf("name mismatch: expected %s, got %s", server.Name, roundTripped.Name)
	}
	if roundTripped.Logo != server.Logo {
		t.Errorf("logo was corrupted: original length=%d, got length=%d", len(server.Logo), len(roundTripped.Logo))
	}
	archProp, ok := roundTripped.CustomProperties["architecture"]
	if !ok {
		t.Fatal("architecture custom property missing after round-trip")
	}
	if archProp.StringValue != `["amd64","arm64"]` {
		t.Errorf("architecture value mismatch: expected %s, got %s", `["amd64","arm64"]`, archProp.StringValue)
	}
}

func TestEnrichMCPServersFromRegistry_MissingIndex(t *testing.T) {
	err := EnrichMCPServersFromRegistry("/nonexistent/index.yaml")
	if err == nil {
		t.Fatal("expected error for missing index file, got nil")
	}
}

func TestEnrichMCPServersFromRegistry_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "index.yaml")
	if err := os.WriteFile(indexPath, []byte("not: valid: yaml: ["), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err := EnrichMCPServersFromRegistry(indexPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestEnrichMCPServersFromRegistry_MissingInputFile(t *testing.T) {
	tmpDir := t.TempDir()
	indexContent := `source: Test
mcp_servers:
  - name: missing-server
    input_path: nonexistent.yaml
`
	indexPath := filepath.Join(tmpDir, "index.yaml")
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		t.Fatalf("failed to write index: %v", err)
	}

	// Should not return error — individual server failures are logged as warnings
	err := EnrichMCPServersFromRegistry(indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnrichMCPServersFromRegistry_EmptyIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexContent := `source: Test
mcp_servers: []
`
	indexPath := filepath.Join(tmpDir, "index.yaml")
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		t.Fatalf("failed to write index: %v", err)
	}

	err := EnrichMCPServersFromRegistry(indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
