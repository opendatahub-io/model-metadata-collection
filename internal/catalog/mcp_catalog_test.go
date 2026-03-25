package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestCreateMCPServersCatalog(t *testing.T) {
	t.Run("happy path with valid index and inputs", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create input files
		inputDir := filepath.Join(tmpDir, "input", "mcp_servers")
		if err := os.MkdirAll(inputDir, 0755); err != nil {
			t.Fatal(err)
		}

		server1 := types.MCPServerMetadata{
			Name:        "test-server-1",
			Provider:    "Red Hat",
			License:     "apache-2.0",
			Description: "Test server 1",
			Version:     "latest",
			Tools: []types.MCPTool{
				{Name: "list_items", Description: "List items", AccessType: "read_only", Parameters: []types.MCPParameter{}},
			},
			Artifacts: []types.MCPArtifact{
				{URI: "oci://registry.example.com/test-server-1:latest"},
			},
		}
		server2 := types.MCPServerMetadata{
			Name:        "test-server-2",
			Provider:    "Red Hat",
			License:     "apache-2.0",
			Description: "Test server 2",
			Version:     "1.0",
		}

		writeYAML(t, filepath.Join(inputDir, "test-server-1.yaml"), server1)
		writeYAML(t, filepath.Join(inputDir, "test-server-2.yaml"), server2)

		// Create index file
		index := types.MCPServersIndex{
			Source: "Red Hat MCP",
			MCPServers: []types.MCPServerEntry{
				{Name: "test-server-1", InputPath: filepath.Join(inputDir, "test-server-1.yaml")},
				{Name: "test-server-2", InputPath: filepath.Join(inputDir, "test-server-2.yaml")},
			},
		}
		indexPath := filepath.Join(tmpDir, "index.yaml")
		writeYAML(t, indexPath, index)

		// Generate catalog
		catalogPath := filepath.Join(tmpDir, "catalog.yaml")
		err := CreateMCPServersCatalog(indexPath, catalogPath)
		if err != nil {
			t.Fatalf("CreateMCPServersCatalog failed: %v", err)
		}

		// Verify catalog
		data, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatalf("Failed to read catalog: %v", err)
		}

		var catalog types.MCPServersCatalog
		if err := yaml.Unmarshal(data, &catalog); err != nil {
			t.Fatalf("Failed to parse catalog: %v", err)
		}

		if catalog.Source != "Red Hat MCP" {
			t.Errorf("expected source 'Red Hat MCP', got %q", catalog.Source)
		}
		if len(catalog.MCPServers) != 2 {
			t.Fatalf("expected 2 servers, got %d", len(catalog.MCPServers))
		}
		if catalog.MCPServers[0].Name != "test-server-1" {
			t.Errorf("expected first server 'test-server-1', got %q", catalog.MCPServers[0].Name)
		}
		if catalog.MCPServers[1].Name != "test-server-2" {
			t.Errorf("expected second server 'test-server-2', got %q", catalog.MCPServers[1].Name)
		}
		if len(catalog.MCPServers[0].Tools) != 1 {
			t.Errorf("expected 1 tool for server 1, got %d", len(catalog.MCPServers[0].Tools))
		}
		if len(catalog.MCPServers[0].Artifacts) != 1 {
			t.Errorf("expected 1 artifact for server 1, got %d", len(catalog.MCPServers[0].Artifacts))
		}
	})

	t.Run("missing input file is skipped", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create one valid input file
		inputDir := filepath.Join(tmpDir, "input")
		if err := os.MkdirAll(inputDir, 0755); err != nil {
			t.Fatal(err)
		}

		server := types.MCPServerMetadata{
			Name:        "good-server",
			Provider:    "Red Hat",
			Description: "Good test server",
			Version:     "latest",
		}
		writeYAML(t, filepath.Join(inputDir, "good-server.yaml"), server)

		// Index references one valid file and one missing file
		index := types.MCPServersIndex{
			Source: "Red Hat MCP",
			MCPServers: []types.MCPServerEntry{
				{Name: "good-server", InputPath: filepath.Join(inputDir, "good-server.yaml")},
				{Name: "missing-server", InputPath: filepath.Join(inputDir, "missing-server.yaml")},
			},
		}
		indexPath := filepath.Join(tmpDir, "index.yaml")
		writeYAML(t, indexPath, index)

		catalogPath := filepath.Join(tmpDir, "catalog.yaml")
		err := CreateMCPServersCatalog(indexPath, catalogPath)
		if err != nil {
			t.Fatalf("CreateMCPServersCatalog should not fail on missing input: %v", err)
		}

		// Verify only the valid server is in the catalog
		data, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatalf("Failed to read catalog: %v", err)
		}
		var catalog types.MCPServersCatalog
		if err := yaml.Unmarshal(data, &catalog); err != nil {
			t.Fatalf("Failed to parse catalog: %v", err)
		}

		if len(catalog.MCPServers) != 1 {
			t.Fatalf("expected 1 server (skipping missing), got %d", len(catalog.MCPServers))
		}
		if catalog.MCPServers[0].Name != "good-server" {
			t.Errorf("expected 'good-server', got %q", catalog.MCPServers[0].Name)
		}
	})

	t.Run("invalid YAML input is skipped", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Write invalid YAML
		inputDir := filepath.Join(tmpDir, "input")
		if err := os.MkdirAll(inputDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inputDir, "bad.yaml"), []byte("invalid: [yaml: {broken"), 0644); err != nil {
			t.Fatal(err)
		}

		index := types.MCPServersIndex{
			Source: "Test",
			MCPServers: []types.MCPServerEntry{
				{Name: "bad-server", InputPath: filepath.Join(inputDir, "bad.yaml")},
			},
		}
		indexPath := filepath.Join(tmpDir, "index.yaml")
		writeYAML(t, indexPath, index)

		catalogPath := filepath.Join(tmpDir, "catalog.yaml")
		err := CreateMCPServersCatalog(indexPath, catalogPath)
		if err != nil {
			t.Fatalf("CreateMCPServersCatalog should not fail on invalid input: %v", err)
		}

		data, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatalf("Failed to read catalog: %v", err)
		}
		var catalog types.MCPServersCatalog
		if err := yaml.Unmarshal(data, &catalog); err != nil {
			t.Fatalf("Failed to parse catalog: %v", err)
		}

		if len(catalog.MCPServers) != 0 {
			t.Errorf("expected 0 servers (invalid skipped), got %d", len(catalog.MCPServers))
		}
	})

	t.Run("empty index produces catalog with no servers", func(t *testing.T) {
		tmpDir := t.TempDir()

		index := types.MCPServersIndex{
			Source:     "Empty",
			MCPServers: []types.MCPServerEntry{},
		}
		indexPath := filepath.Join(tmpDir, "index.yaml")
		writeYAML(t, indexPath, index)

		catalogPath := filepath.Join(tmpDir, "catalog.yaml")
		err := CreateMCPServersCatalog(indexPath, catalogPath)
		if err != nil {
			t.Fatalf("CreateMCPServersCatalog failed: %v", err)
		}

		data, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatalf("Failed to read catalog: %v", err)
		}
		var catalog types.MCPServersCatalog
		if err := yaml.Unmarshal(data, &catalog); err != nil {
			t.Fatalf("Failed to parse catalog: %v", err)
		}

		if catalog.Source != "Empty" {
			t.Errorf("expected source 'Empty', got %q", catalog.Source)
		}
		if len(catalog.MCPServers) != 0 {
			t.Errorf("expected nil or empty servers, got %d", len(catalog.MCPServers))
		}
	})

	t.Run("missing index file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		catalogPath := filepath.Join(tmpDir, "catalog.yaml")

		err := CreateMCPServersCatalog(filepath.Join(tmpDir, "nonexistent.yaml"), catalogPath)
		if err == nil {
			t.Fatal("expected error for missing index file")
		}
	})

	t.Run("path traversal input_path is rejected", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a sentinel file OUTSIDE the fixture tree with unique identifiable content.
		// It is written as valid MCP server YAML so that if the traversal guard were removed
		// the file would parse successfully and appear in the catalog — making the regression
		// detectable both by server count and by sentinel content in the raw catalog bytes.
		sentinelDir := t.TempDir()
		const sentinelMarker = "sentinel-unique-xk3mq9"
		sentinelContent := "name: " + sentinelMarker + "\nprovider: sentinel\ndescription: sentinel\nversion: sentinel\n"
		sentinelPath := filepath.Join(sentinelDir, "sentinel.yaml")
		if err := os.WriteFile(sentinelPath, []byte(sentinelContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Compute a relative path from the index directory to the sentinel — it will start with "..".
		relPath, err := filepath.Rel(tmpDir, sentinelPath)
		if err != nil {
			t.Fatalf("failed to compute relative traversal path: %v", err)
		}

		index := types.MCPServersIndex{
			Source: "Test",
			MCPServers: []types.MCPServerEntry{
				{Name: "traversal-attempt", InputPath: relPath},
			},
		}
		indexPath := filepath.Join(tmpDir, "index.yaml")
		writeYAML(t, indexPath, index)

		catalogPath := filepath.Join(tmpDir, "catalog.yaml")
		if err := CreateMCPServersCatalog(indexPath, catalogPath); err != nil {
			t.Fatalf("CreateMCPServersCatalog should not fail on invalid input_path: %v", err)
		}

		data, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatalf("Failed to read catalog: %v", err)
		}

		// Sentinel content must not appear in the catalog output.
		if strings.Contains(string(data), sentinelMarker) {
			t.Error("catalog contains sentinel content — path traversal guard was bypassed")
		}

		var catalog types.MCPServersCatalog
		if err := yaml.Unmarshal(data, &catalog); err != nil {
			t.Fatalf("Failed to parse catalog: %v", err)
		}
		if len(catalog.MCPServers) != 0 {
			t.Errorf("expected 0 servers (path traversal rejected), got %d", len(catalog.MCPServers))
		}
	})

	t.Run("input with missing required fields is skipped", func(t *testing.T) {
		tmpDir := t.TempDir()

		inputDir := filepath.Join(tmpDir, "input")
		if err := os.MkdirAll(inputDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write a YAML with only name — missing provider, description, version
		if err := os.WriteFile(filepath.Join(inputDir, "minimal.yaml"), []byte("name: minimal-server\n"), 0644); err != nil {
			t.Fatal(err)
		}

		index := types.MCPServersIndex{
			Source: "Test",
			MCPServers: []types.MCPServerEntry{
				{Name: "minimal-server", InputPath: filepath.Join(inputDir, "minimal.yaml")},
			},
		}
		indexPath := filepath.Join(tmpDir, "index.yaml")
		writeYAML(t, indexPath, index)

		catalogPath := filepath.Join(tmpDir, "catalog.yaml")
		err := CreateMCPServersCatalog(indexPath, catalogPath)
		if err != nil {
			t.Fatalf("CreateMCPServersCatalog should not fail on incomplete input: %v", err)
		}

		data, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatalf("Failed to read catalog: %v", err)
		}
		var catalog types.MCPServersCatalog
		if err := yaml.Unmarshal(data, &catalog); err != nil {
			t.Fatalf("Failed to parse catalog: %v", err)
		}

		if len(catalog.MCPServers) != 0 {
			t.Errorf("expected 0 servers (missing required fields rejected), got %d", len(catalog.MCPServers))
		}
	})
}

func writeYAML(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := yaml.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal YAML: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}
