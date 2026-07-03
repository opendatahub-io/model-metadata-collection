package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/internal/github"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// CreateAgentsCatalog reads an agents index file, fetches metadata from GitHub
// for each agent, and writes an aggregated catalog YAML.
func CreateAgentsCatalog(indexPath, catalogPath, branchOverride string, skipEnrichment bool) error {
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("error reading agents index file %s: %v", indexPath, err)
	}

	var index types.AgentsIndex
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		return fmt.Errorf("error parsing agents index file %s: %v", indexPath, err)
	}

	if index.Repository == "" {
		return fmt.Errorf("agents index file %s missing required field: repository", indexPath)
	}
	if branchOverride != "" {
		index.Branch = branchOverride
	} else if index.Branch == "" {
		index.Branch = "main"
	}

	log.Printf("Processing agents index: %s (%d entries, repo: %s, branch: %s)",
		indexPath, len(index.Agents), index.Repository, index.Branch)

	if !skipEnrichment {
		if err := github.ValidateBranch(index.Repository, index.Branch); err != nil {
			return fmt.Errorf("branch validation failed: %v", err)
		}
	}

	var agents []types.AgentMetadata
	for _, entry := range index.Agents {
		agent, err := buildAgentMetadata(index.Repository, index.Branch, entry, skipEnrichment)
		if err != nil {
			log.Printf("Warning: skipping agent at path %q: %v", entry.Path, err)
			continue
		}
		agents = append(agents, *agent)
		log.Printf("  Loaded agent: %s", agent.Name)
	}

	catalog := types.AgentsCatalog{
		Source: index.Source,
		Agents: agents,
	}

	catalogDir := filepath.Dir(catalogPath)
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		return fmt.Errorf("error creating catalog output directory: %v", err)
	}

	output, err := utils.MarshalYAMLWithNewline(&catalog)
	if err != nil {
		return fmt.Errorf("error marshaling agents catalog: %v", err)
	}

	if err := os.WriteFile(catalogPath, output, 0644); err != nil {
		return fmt.Errorf("error writing agents catalog file: %v", err)
	}

	log.Printf("Successfully created %s with %d agents", catalogPath, len(agents))
	return nil
}

// buildAgentMetadata constructs an AgentMetadata by fetching from GitHub or
// falling back to inline overrides from the index entry.
func buildAgentMetadata(repo, branch string, entry types.AgentIndexEntry, skipEnrichment bool) (*types.AgentMetadata, error) {
	agent := &types.AgentMetadata{}

	if !skipEnrichment {
		upstream, err := github.FetchAgentYAML(repo, branch, entry.Path)
		if errors.Is(err, github.ErrNotFound) {
			log.Printf("  No agent.yaml at %s, using index overrides", entry.Path)
		} else if err != nil {
			return nil, fmt.Errorf("failed to fetch agent.yaml for %s: %v", entry.Path, err)
		} else {
			agent.Name = upstream.Name
			agent.DisplayName = upstream.DisplayName
			agent.Framework = upstream.Framework
			agent.Description = upstream.Description
			agent.Env = transformEnvVars(upstream)
			forwardExtraAsCustomProperties(agent, upstream.Extra)
		}

		readme, err := github.FetchReadme(repo, branch, entry.Path)
		if err != nil {
			log.Printf("  Warning: failed to fetch README for %s: %v", entry.Path, err)
		} else if readme != "" {
			agent.Readme = readme
		}
	}

	applyIndexOverrides(agent, entry)

	agent.RepositoryUrl = fmt.Sprintf("https://github.com/%s/tree/%s/%s", repo, branch, entry.Path)

	if agent.Name == "" {
		return nil, fmt.Errorf("agent at path %q has no name (not in agent.yaml and no index override)", entry.Path)
	}
	if agent.Description == "" {
		return nil, fmt.Errorf("agent %q missing required field: description", agent.Name)
	}
	if agent.Framework == "" {
		return nil, fmt.Errorf("agent %q missing required field: framework", agent.Name)
	}

	return agent, nil
}

// applyIndexOverrides applies inline metadata from the index entry, filling in
// any fields that are still empty (index values don't overwrite fetched data).
func applyIndexOverrides(agent *types.AgentMetadata, entry types.AgentIndexEntry) {
	if agent.Name == "" && entry.Name != "" {
		agent.Name = entry.Name
	}
	if agent.DisplayName == "" && entry.DisplayName != "" {
		agent.DisplayName = entry.DisplayName
	}
	if agent.Framework == "" && entry.Framework != "" {
		agent.Framework = entry.Framework
	}
	if agent.Description == "" && entry.Description != "" {
		agent.Description = entry.Description
	}
}

// transformEnvVars converts the upstream agent.yaml env format (required/optional
// string lists) into the flat catalog format ([]AgentEnvVar with name+required).
func transformEnvVars(upstream *types.UpstreamAgentYAML) []types.AgentEnvVar {
	var envVars []types.AgentEnvVar
	for _, name := range upstream.Env.Required {
		envVars = append(envVars, types.AgentEnvVar{Name: name, Required: true})
	}
	for _, name := range upstream.Env.Optional {
		envVars = append(envVars, types.AgentEnvVar{Name: name, Required: false})
	}
	return envVars
}

// forwardExtraAsCustomProperties takes unknown fields from the upstream agent.yaml
// and adds them to the agent's customProperties as MetadataStringValue entries.
// Arrays and objects are JSON-encoded; scalars are converted to strings.
func forwardExtraAsCustomProperties(agent *types.AgentMetadata, extra map[string]interface{}) {
	if len(extra) == 0 {
		return
	}
	if agent.CustomProperties == nil {
		agent.CustomProperties = make(map[string]types.MetadataValue)
	}
	for key, val := range extra {
		var strVal string
		switch v := val.(type) {
		case string:
			strVal = v
		default:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				log.Printf("  Warning: could not serialize extra field %q: %v", key, err)
				continue
			}
			strVal = string(jsonBytes)
		}
		agent.CustomProperties[key] = types.MetadataValue{
			MetadataType: "MetadataStringValue",
			StringValue:  strVal,
		}
	}
}

// agentSupportTierFromSource maps an index Source value to a supportTier string.
// Currently unused — kept as a stub for future use if agent sources need tiered support.
//
//nolint:unused
func agentSupportTierFromSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "red hat agents":
		return "redHatSupported"
	case "partner agents":
		return "partnerSupported"
	case "community agents":
		return "communitySupported"
	default:
		return ""
	}
}
