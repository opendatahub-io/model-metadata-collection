package catalog

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func schemaCatalogFixture(t *testing.T) []byte {
	t.Helper()
	data, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(validRuntimeInput))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestServingRuntimeCatalogSchema(t *testing.T) {
	result, err := ValidateServingRuntimeCatalogSchema(context.Background(), schemaCatalogFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.Runtimes != 1 || result.Versions != 1 || !slices.Equal(result.CatalogExtensions, []string{"minimumRHOAIVersion"}) {
		t.Fatalf("wrong validation report: %+v", result)
	}
}

func TestServingRuntimeCatalogSchemaRejectsInvalidData(t *testing.T) {
	fixture := schemaCatalogFixture(t)
	cases := map[string]func(map[string]any, map[string]any){
		"display name length":  func(runtime, _ map[string]any) { runtime["displayName"] = strings.Repeat("a", 256) },
		"numeric display name": func(runtime, _ map[string]any) { runtime["displayName"] = 42 },
		"URI format":           func(runtime, _ map[string]any) { runtime["documentationUrl"] = "relative/path" },
		"date format":          func(runtime, _ map[string]any) { runtime["publishedDate"] = "yesterday" },
		"null boolean": func(runtime, _ map[string]any) {
			runtime["capabilities"].(map[string]any)["requiresGPU"] = nil
		},
		"metadata property": func(runtime, _ map[string]any) {
			runtime["customProperties"] = map[string]any{"test": "untyped value"}
		},
		"support enum":  func(_, version map[string]any) { version["supportLevel"] = "preview" },
		"missing image": func(_, version map[string]any) { delete(version, "image") },
		"model format priority": func(_, version map[string]any) {
			version["supportedModelFormats"] = []any{map[string]any{"name": "vLLM", "priority": int64(2147483648)}}
		},
		"minimum version type":  func(_, version map[string]any) { version["minimumRHOAIVersion"] = 3.6 },
		"minimum version value": func(_, version map[string]any) { version["minimumRHOAIVersion"] = "latest" },
		"unknown version field": func(_, version map[string]any) { version["support_level"] = "supported" },
		"malformed manifest":    func(_, version map[string]any) { version["llmInferenceServiceConfig"] = "{" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var data map[string]any
			if err := yaml.Unmarshal(fixture, &data); err != nil {
				t.Fatal(err)
			}
			runtime := data["serving_runtimes"].([]any)[0].(map[string]any)
			version := runtime["versions"].([]any)[0].(map[string]any)
			mutate(runtime, version)
			input, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateServingRuntimeCatalogSchema(context.Background(), input); err == nil {
				t.Fatal("invalid catalog passed schema validation")
			}
		})
	}
}

func TestServingRuntimeCatalogSchemaWithMetadataAndMultipleVersions(t *testing.T) {
	var data map[string]any
	if err := yaml.Unmarshal(schemaCatalogFixture(t), &data); err != nil {
		t.Fatal(err)
	}
	runtime := data["serving_runtimes"].([]any)[0].(map[string]any)
	runtime["documentationUrl"] = "https://example.com/runtime"
	runtime["publishedDate"] = "2026-10-07T10:30:00Z"
	runtime["customProperties"] = map[string]any{"test": map[string]any{"metadataType": "MetadataStringValue", "string_value": "value"}}
	versions := runtime["versions"].([]any)
	version := versions[0].(map[string]any)
	delete(version, "minimumRHOAIVersion")
	second := map[string]any{}
	for key, value := range version {
		second[key] = value
	}
	second["version"] = "3.5.0"
	runtime["versions"] = append(versions, second)
	input, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ValidateServingRuntimeCatalogSchema(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Versions != 2 || len(result.CatalogExtensions) != 0 {
		t.Fatalf("wrong validation report: %+v", result)
	}
}

func TestServingRuntimeCatalogSchemaPreservesYAMLDateText(t *testing.T) {
	fixture := string(schemaCatalogFixture(t))
	for _, stamp := range []string{"2026-10-07", "2026-10-07T10:30:00Z"} {
		input := strings.Replace(fixture, "displayName: vLLM", "displayName: vLLM\n    publishedDate: "+stamp, 1)
		_, err := ValidateServingRuntimeCatalogSchema(context.Background(), []byte(input))
		if (err == nil) != strings.Contains(stamp, "T") {
			t.Fatalf("unquoted timestamp %q: %v", stamp, err)
		}
	}
}

func TestCommittedServingRuntimeCatalogSchema(t *testing.T) {
	data, err := os.ReadFile("../../data/redhat-serving-runtimes-catalog.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateServingRuntimeCatalogSchema(context.Background(), data); err != nil {
		t.Fatal(err)
	}
}
