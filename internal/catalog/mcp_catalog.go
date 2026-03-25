package catalog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

// CreateMCPServersCatalog reads an MCP servers index file, loads each referenced
// input file, and writes an aggregated catalog YAML.
func CreateMCPServersCatalog(indexPath, catalogPath string) error {
	// Read and parse the index file
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("error reading MCP index file %s: %v", indexPath, err)
	}

	var index types.MCPServersIndex
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		return fmt.Errorf("error parsing MCP index file %s: %v", indexPath, err)
	}

	log.Printf("Processing MCP servers index: %s (%d entries)", indexPath, len(index.MCPServers))

	// Load each input file referenced by the index
	var servers []types.MCPServerMetadata
	for _, entry := range index.MCPServers {
		cleaned := filepath.Clean(entry.InputPath)
		if !filepath.IsAbs(cleaned) && strings.HasPrefix(cleaned, "..") {
			log.Printf("Warning: skipping MCP server %q: invalid input_path %q (relative path traversal not allowed)", entry.Name, entry.InputPath)
			continue
		}
		server, err := loadMCPServerInput(cleaned)
		if err != nil {
			log.Printf("Warning: skipping MCP server %q: %v", entry.Name, err)
			continue
		}
		servers = append(servers, *server)
		log.Printf("  Loaded MCP server: %s", entry.Name)
	}

	// Build the catalog
	catalog := types.MCPServersCatalog{
		Source:     index.Source,
		MCPServers: servers,
	}

	// Ensure output directory exists
	catalogDir := filepath.Dir(catalogPath)
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		return fmt.Errorf("error creating catalog output directory: %v", err)
	}

	// Marshal and write
	output, err := yaml.Marshal(&catalog)
	if err != nil {
		return fmt.Errorf("error marshaling MCP catalog: %v", err)
	}

	if err := os.WriteFile(catalogPath, output, 0644); err != nil {
		return fmt.Errorf("error writing MCP catalog file: %v", err)
	}

	log.Printf("Successfully created %s with %d MCP servers", catalogPath, len(servers))
	return nil
}

// loadMCPServerInput reads and parses a single MCP server input YAML file.
func loadMCPServerInput(inputPath string) (*types.MCPServerMetadata, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("error reading input file %s: %v", inputPath, err)
	}

	var server types.MCPServerMetadata
	if err := yaml.Unmarshal(data, &server); err != nil {
		return nil, fmt.Errorf("error parsing input file %s: %v", inputPath, err)
	}

	var missing []string
	if server.Name == "" {
		missing = append(missing, "name")
	}
	if server.Provider == "" {
		missing = append(missing, "provider")
	}
	if server.Description == "" {
		missing = append(missing, "description")
	}
	if server.Version == "" {
		missing = append(missing, "version")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("input file %s missing required field(s): %s", inputPath, strings.Join(missing, ", "))
	}

	return &server, nil
}
