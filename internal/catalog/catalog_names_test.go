package catalog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// catalogNameList captures just the "name" field of each record in a
// data/*-catalog.yaml file, keyed by the record type's top-level list.
// It intentionally ignores every other field so that nested "name" keys
// (e.g. MCP tools, agent env vars, customProperties) never get picked up.
type catalogNameList struct {
	Models     []struct{ Name string } `yaml:"models"`
	MCPServers []struct{ Name string } `yaml:"mcp_servers"`
	Agents     []struct{ Name string } `yaml:"agents"`
}

// TestCatalogNamesAreUniquePerType ensures no record name is duplicated
// across the catalog files of the same type (models, mcp_servers, agents).
// Duplicate names across different types are allowed, since a model and an
// MCP server (for example) may legitimately share a name.
func TestCatalogNamesAreUniquePerType(t *testing.T) {
	files, err := filepath.Glob("../../data/*-catalog.yaml")
	if err != nil {
		t.Fatalf("failed to glob catalog files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no catalog files found matching data/*-catalog.yaml")
	}

	namesByType := map[string]map[string][]string{
		"models":      {},
		"mcp_servers": {},
		"agents":      {},
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}

		var list catalogNameList
		if err := yaml.Unmarshal(data, &list); err != nil {
			t.Fatalf("failed to parse %s: %v", file, err)
		}

		base := filepath.Base(file)
		record := func(typeKey, name string) {
			if name == "" {
				return
			}
			namesByType[typeKey][name] = append(namesByType[typeKey][name], base)
		}

		for _, m := range list.Models {
			record("models", m.Name)
		}
		for _, m := range list.MCPServers {
			record("mcp_servers", m.Name)
		}
		for _, a := range list.Agents {
			record("agents", a.Name)
		}
	}

	for _, typeKey := range []string{"models", "mcp_servers", "agents"} {
		t.Run(typeKey, func(t *testing.T) {
			var dupNames []string
			for name, files := range namesByType[typeKey] {
				if len(files) > 1 {
					dupNames = append(dupNames, name)
				}
			}
			sort.Strings(dupNames)

			for _, name := range dupNames {
				files := namesByType[typeKey][name]
				sort.Strings(files)
				t.Errorf("duplicate %s name %q appears in: %s", singularName(typeKey), name, strings.Join(files, ", "))
			}
		})
	}
}

// TestCatalogNamesAreNotNull ensures every record in a data/*-catalog.yaml file has a
// non-empty "name" field. A blank name typically means enrichment failed to resolve an
// identity for the record (e.g. a pinned HuggingFace lookup that couldn't be fetched)
// and the record was still written to the catalog.
func TestCatalogNamesAreNotNull(t *testing.T) {
	files, err := filepath.Glob("../../data/*-catalog.yaml")
	if err != nil {
		t.Fatalf("failed to glob catalog files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no catalog files found matching data/*-catalog.yaml")
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}

		var list catalogNameList
		if err := yaml.Unmarshal(data, &list); err != nil {
			t.Fatalf("failed to parse %s: %v", file, err)
		}

		base := filepath.Base(file)
		for i, m := range list.Models {
			if m.Name == "" {
				t.Errorf("%s: models[%d] has a null/empty name", base, i)
			}
		}
		for i, m := range list.MCPServers {
			if m.Name == "" {
				t.Errorf("%s: mcp_servers[%d] has a null/empty name", base, i)
			}
		}
		for i, a := range list.Agents {
			if a.Name == "" {
				t.Errorf("%s: agents[%d] has a null/empty name", base, i)
			}
		}
	}
}

func singularName(typeKey string) string {
	switch typeKey {
	case "models":
		return "model"
	case "mcp_servers":
		return "mcp_server"
	case "agents":
		return "agent"
	default:
		return typeKey
	}
}
