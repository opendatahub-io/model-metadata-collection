package catalog

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/internal/registry"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// tsResult wraps timestamp values returned from registry inspection.
type tsResult struct {
	create *int64
	update *int64
}

// EnrichMCPServersFromRegistry reads the MCP servers index, inspects each
// server's container image artifacts via OCI registry, extracts architectures
// and timestamps, and writes enriched data back to the input YAML files.
func EnrichMCPServersFromRegistry(indexPath string) error {
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("error reading MCP index file %s: %v", indexPath, err)
	}

	var index types.MCPServersIndex
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		return fmt.Errorf("error parsing MCP index file %s: %v", indexPath, err)
	}

	log.Printf("Enriching MCP servers from registry: %s (%d entries)", indexPath, len(index.MCPServers))

	enrichedCount := 0
	for _, entry := range index.MCPServers {
		cleaned := filepath.Clean(entry.InputPath)
		if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
			log.Printf("Warning: skipping MCP server %q enrichment: invalid input_path %q", entry.Name, entry.InputPath)
			continue
		}

		server, err := loadMCPServerInput(cleaned)
		if err != nil {
			log.Printf("Warning: skipping MCP server %q enrichment: %v", entry.Name, err)
			continue
		}

		changed, err := enrichMCPServerArtifacts(server)
		if err != nil {
			log.Printf("Warning: skipping MCP server %q enrichment: %v", entry.Name, err)
			continue
		}

		if changed {
			if err := writeMCPServerInput(cleaned, server); err != nil {
				log.Printf("Warning: failed to write enriched data for MCP server %q: %v", entry.Name, err)
				continue
			}
			enrichedCount++
			log.Printf("  Enriched MCP server: %s", entry.Name)
		} else {
			log.Printf("  MCP server %s: no changes needed", entry.Name)
		}
	}

	log.Printf("MCP server enrichment complete: %d of %d servers enriched", enrichedCount, len(index.MCPServers))
	return nil
}

// enrichMCPServerArtifacts enriches a single MCP server's metadata with OCI
// registry data (architectures and timestamps). Returns true if changes were made.
func enrichMCPServerArtifacts(server *types.MCPServerMetadata) (bool, error) {
	changed := false

	// Validate all artifacts have URIs (if any exist)
	for i, artifact := range server.Artifacts {
		if artifact.URI == "" {
			return false, fmt.Errorf("artifact[%d] has empty URI — bad input data", i)
		}
	}

	// Collect architectures across all artifacts and track latest update time
	var allArchitectures []string
	var latestUpdateEpoch *int64

	for i := range server.Artifacts {
		artifact := &server.Artifacts[i]
		imageRef := strings.TrimPrefix(artifact.URI, "oci://")

		log.Printf("  DEBUG: inspecting artifact: %s", imageRef)

		// Fetch architectures with retry
		architectures, err := utils.RetryWithExponentialBackoff(
			utils.DefaultRetryConfig,
			func() ([]string, error) {
				return registry.FetchImageArchitectures(imageRef)
			},
			fmt.Sprintf("fetch architectures for %s", imageRef),
		)
		if err != nil {
			log.Printf("  Warning: failed to fetch architectures for %s after retries: %v", imageRef, err)
		} else {
			allArchitectures = append(allArchitectures, architectures...)
			log.Printf("  DEBUG: architectures for %s: %v", imageRef, architectures)
		}

		// Fetch timestamps with retry
		ts, err := utils.RetryWithExponentialBackoff(
			utils.DefaultRetryConfig,
			func() (tsResult, error) {
				c, u, e := registry.FetchImageTimestamps(imageRef)
				return tsResult{c, u}, e
			},
			fmt.Sprintf("fetch timestamps for %s", imageRef),
		)
		if err != nil {
			log.Printf("  Warning: failed to fetch timestamps for %s after retries: %v", imageRef, err)
		} else {
			// Update artifact timestamps
			if createStr := epochMillisToString(ts.create); createStr != "" && createStr != artifact.CreateTimeSinceEpoch {
				artifact.CreateTimeSinceEpoch = createStr
				changed = true
				log.Printf("  DEBUG: updated artifact create timestamp: %s", createStr)
			}
			if updateStr := epochMillisToString(ts.update); updateStr != "" && updateStr != artifact.LastUpdateTimeSinceEpoch {
				artifact.LastUpdateTimeSinceEpoch = updateStr
				changed = true
				log.Printf("  DEBUG: updated artifact update timestamp: %s", updateStr)
			}

			// Track latest update across all artifacts
			if ts.update != nil {
				if latestUpdateEpoch == nil || *ts.update > *latestUpdateEpoch {
					latestUpdateEpoch = ts.update
				}
			}
		}
	}

	// Store architectures in server-level custom properties
	if len(allArchitectures) > 0 {
		// Deduplicate and sort
		archSet := make(map[string]bool)
		for _, arch := range allArchitectures {
			archSet[arch] = true
		}
		var uniqueArchs []string
		for arch := range archSet {
			uniqueArchs = append(uniqueArchs, arch)
		}
		// Sort for deterministic output
		sort.Strings(uniqueArchs)

		archJSON, err := json.Marshal(uniqueArchs)
		if err != nil {
			log.Printf("  Warning: failed to marshal architectures: %v", err)
		} else {
			archValue := types.MetadataValue{
				MetadataType: "MetadataStringValue",
				StringValue:  string(archJSON),
			}

			if server.CustomProperties == nil {
				server.CustomProperties = make(map[string]types.MetadataValue)
			}

			existing, exists := server.CustomProperties["architecture"]
			if !exists || existing.StringValue != archValue.StringValue {
				server.CustomProperties["architecture"] = archValue
				changed = true
				log.Printf("  DEBUG: updated architecture custom property: %s", string(archJSON))
			}
		}
	}

	// Derive server createTimeSinceEpoch from publishedDate
	if server.PublishedDate != "" {
		if epochMs := utils.ParseTimeToEpochInt64(server.PublishedDate); epochMs != nil {
			createStr := strconv.FormatInt(*epochMs, 10)
			if createStr != server.CreateTimeSinceEpoch {
				server.CreateTimeSinceEpoch = createStr
				changed = true
				log.Printf("  DEBUG: derived server createTimeSinceEpoch from publishedDate: %s", createStr)
			}
		}
	}

	// Update server lastUpdateTimeSinceEpoch from latest artifact update
	if latestUpdateEpoch != nil {
		updateStr := strconv.FormatInt(*latestUpdateEpoch, 10)
		if updateStr != server.LastUpdateTimeSinceEpoch {
			server.LastUpdateTimeSinceEpoch = updateStr
			changed = true
			log.Printf("  DEBUG: updated server lastUpdateTimeSinceEpoch: %s", updateStr)
		}
	}

	return changed, nil
}

// writeMCPServerInput writes the enriched MCP server metadata back to its input YAML file.
func writeMCPServerInput(inputPath string, server *types.MCPServerMetadata) error {
	data, err := utils.MarshalYAMLWithNewline(server)
	if err != nil {
		return fmt.Errorf("error marshaling server %s: %v", server.Name, err)
	}
	return os.WriteFile(inputPath, data, 0644)
}

// epochMillisToString converts a pointer to epoch milliseconds to a string.
// Returns empty string if the pointer is nil.
func epochMillisToString(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}
