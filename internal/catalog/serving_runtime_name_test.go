package catalog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestVersionedLLMConfigName(t *testing.T) {
	for _, test := range []struct {
		name, version, want string
	}{
		{"kserve-config-llm-template-amd-rocm", "0.26.0+rhaiv.7", "0-26-0-config-llm-template-amd-rocm"},
		{"kserve-config", "0.26.0-rhaiv-7", "0-26-0-config"},
		{"kserve-config", "V1.2.3_RC+Build", "v1-2-3-rc-config"},
		{"kserve-config", "v3.6.0-ea.1+rhaiv.7", "v3-6-0-ea-1-config"},
		{"kserve-config", " .V1///β..2--+ ", "v1-2-config"},
		{"kserve-config", "3", "3-config"},
		{"custom-config", "1.2.3", "custom-config"},
	} {
		t.Run(test.version+"/"+test.name, func(t *testing.T) {
			got, err := versionedLLMConfigName(test.name, test.version)
			if err != nil || got != test.want {
				t.Fatalf("got %q, %v; want %q", got, err, test.want)
			}
		})
	}
	for _, version := range []string{"", "+-._", "β"} {
		if _, err := versionedLLMConfigName("kserve-config", version); err == nil {
			t.Fatalf("expected invalid version %q to fail", version)
		}
	}
	if _, err := versionedLLMConfigName("kserve-invalid_name", "1.0"); err == nil {
		t.Fatal("expected an invalid template suffix to fail")
	}
}

func TestVersionedRuntimeObjectName(t *testing.T) {
	for _, test := range []struct {
		name, version, want string
	}{
		{"vllm-cuda-runtime-template", "0.26.0+rhaiv.8", "0-26-0-vllm-cuda-runtime-template"},
		{"vllm-cuda-runtime", "0.26.0-rhaiv-8", "0-26-0-vllm-cuda-runtime"},
		{"kserve-ovms", "v2026.1.0", "v2026-1-0-ovms"},
		{"mlserver-runtime-template", "V1.7.1_RC+Build", "v1-7-1-rc-mlserver-runtime-template"},
	} {
		got, err := versionedRuntimeObjectName(test.name, test.version)
		if err != nil || got != test.want {
			t.Fatalf("got %q, %v; want %q", got, err, test.want)
		}
	}
}

func TestVersionedLLMConfigNameLength(t *testing.T) {
	withoutBuild, err := versionedLLMConfigName("kserve-cuda", "0.26.0+"+strings.Repeat("build", 100))
	if err != nil || withoutBuild != "0-26-0-cuda" {
		t.Fatalf("build metadata should be removed before truncation: %q, %v", withoutBuild, err)
	}
	version := strings.Repeat("v", 300)
	first, err := versionedLLMConfigName("kserve-cuda", version)
	if err != nil || len(first) != kubernetesObjectNameMaxLength || !kubernetesObjectNamePattern.MatchString(first) {
		t.Fatalf("long version did not produce a valid bounded name: %q, %v", first, err)
	}
	again, err := versionedLLMConfigName("kserve-cuda", version)
	if err != nil || again != first {
		t.Fatal("name shortening must be deterministic")
	}
	other, err := versionedLLMConfigName("kserve-rocm", version)
	if err != nil || other == first {
		t.Fatal("name shortening lost the template identity")
	}
	boundary := strings.Repeat("v", kubernetesObjectNameMaxLength-len("-cuda"))
	name, err := versionedLLMConfigName("kserve-cuda", boundary)
	if err != nil || name != boundary+"-cuda" {
		t.Fatal("a name at the length limit should not be shortened")
	}
}

func TestLLMConfigVersionedNamesAcrossReleases(t *testing.T) {
	config, sources := discoveredSources()
	config.Sources[0].Include = []string{"config/runtimes/config.yaml"}
	files := sources["controller"].(fstest.MapFS)
	files["config/runtimes/config.yaml"].Data = []byte(strings.Replace(discoveredLLMConfig, "name: cuda-single-node", "name: kserve-config-llm-cuda", 1))
	config.Sources[0].Templates = map[string]RuntimeTemplateConfig{"config/runtimes/config.yaml": {Version: "0.26.0+rhaiv.8"}}
	second := config.Sources[0]
	second.ID = "second-release"
	second.Templates = map[string]RuntimeTemplateConfig{"config/runtimes/config.yaml": {Version: "0.27.0+rhaiv.1"}}
	config.Sources = append(config.Sources, second)
	sources[second.ID] = files
	artifacts, err := GenerateServingRuntimeArtifacts(&config, sources, "generated")
	if err != nil {
		t.Fatal(err)
	}
	var catalog types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(artifacts.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.ServingRuntimes) != 1 || len(catalog.ServingRuntimes[0].Versions) != 2 {
		t.Fatal("versioned object names should preserve catalog version aggregation")
	}
	for i, want := range []string{"0-26-0-config-llm-cuda", "0-27-0-config-llm-cuda"} {
		var manifest map[string]any
		if err := json.Unmarshal([]byte(catalog.ServingRuntimes[0].Versions[i].LLMInferenceServiceConfig), &manifest); err != nil {
			t.Fatal(err)
		}
		if got := textAt(objectAt(manifest, "metadata"), "name"); got != want {
			t.Fatalf("version %d has object name %q; want %q", i, got, want)
		}
		fullVersion := catalog.ServingRuntimes[0].Versions[i].Version
		if !strings.Contains(fullVersion, "+rhaiv.") || textAt(objectAt(manifest, "metadata", "annotations"), "opendatahub.io/runtime-version") != fullVersion {
			t.Fatal("object-name shortening changed the full runtime version")
		}
	}
	inputs := fstest.MapFS{}
	for file, data := range artifacts.RuntimeFiles {
		inputs[file] = &fstest.MapFile{Data: data}
	}
	rebuilt, err := GenerateServingRuntimeCatalog(artifacts.Index, inputs)
	if err != nil || !bytes.Equal(artifacts.Catalog, rebuilt) {
		t.Fatalf("versioned manifests did not survive the index/input round trip: %v", err)
	}
	config.Sources[1].Templates["config/runtimes/config.yaml"] = RuntimeTemplateConfig{Version: "0.26.0+rhaiv.99"}
	if _, err := GenerateServingRuntimeArtifacts(&config, sources, "generated"); err == nil || !strings.Contains(err.Error(), "duplicate versioned") {
		t.Fatalf("expected colliding sanitized versions to fail: %v", err)
	}
}

func TestTemplateVersionedNamesAcrossReleases(t *testing.T) {
	const file = "config/runtimes/vllm/deep/cuda.yaml"
	config, sources := discoveredSources()
	config.Sources[0].Include = []string{file}
	config.Sources[0].Templates = map[string]RuntimeTemplateConfig{file: {Version: "0.26.0+rhaiv.8"}}
	second := config.Sources[0]
	second.ID = "second-release"
	second.Templates = map[string]RuntimeTemplateConfig{file: {Version: "0.27.0+rhaiv.1"}}
	config.Sources = append(config.Sources, second)
	sources[second.ID] = sources["controller"]
	artifacts, err := GenerateServingRuntimeArtifacts(&config, sources, "generated")
	if err != nil {
		t.Fatal(err)
	}
	var result types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(artifacts.Catalog, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.ServingRuntimes) != 1 || result.ServingRuntimes[0].Name != "vllm-cuda" || len(result.ServingRuntimes[0].Versions) != 2 {
		t.Fatal("versioned Template names should preserve catalog version aggregation")
	}
	for i, prefix := range []string{"0-26-0", "0-27-0"} {
		version := result.ServingRuntimes[0].Versions[i]
		var manifest map[string]any
		if err := json.Unmarshal([]byte(version.ServingRuntimeTemplate), &manifest); err != nil {
			t.Fatal(err)
		}
		if got := textAt(objectAt(manifest, "metadata"), "name"); got != prefix+"-vllm-cuda-runtime-template" {
			t.Fatalf("incorrect Template name: %q", got)
		}
		resource := servingRuntimeResource(manifest)
		if got := textAt(objectAt(resource, "metadata"), "name"); got != prefix+"-vllm-cuda" {
			t.Fatalf("incorrect nested ServingRuntime name: %q", got)
		}
		if textAt(objectAt(resource, "metadata", "annotations"), "opendatahub.io/runtime-version") != version.Version {
			t.Fatal("object-name shortening changed the full runtime version")
		}
	}
	inputs := fstest.MapFS{}
	for path, data := range artifacts.RuntimeFiles {
		inputs[path] = &fstest.MapFile{Data: data}
	}
	if _, ok := inputs["generated/vllm-cuda.yaml"]; !ok {
		t.Fatal("generated filename should retain the catalog identifier")
	}
	rebuilt, err := GenerateServingRuntimeCatalog(artifacts.Index, inputs)
	if err != nil || !bytes.Equal(artifacts.Catalog, rebuilt) {
		t.Fatalf("versioned Templates did not survive the index/input round trip: %v", err)
	}
	config.Sources[1].Templates[file] = RuntimeTemplateConfig{Version: "0.26.0+rhaiv.99"}
	if _, err := GenerateServingRuntimeArtifacts(&config, sources, "generated"); err == nil || !strings.Contains(err.Error(), "duplicate versioned Template") {
		t.Fatalf("colliding sanitized Template versions accepted: %v", err)
	}
	// Distinct Templates can still collide on their nested ServingRuntime names.
	sources[second.ID] = fstest.MapFS{file: {Data: []byte(strings.Replace(discoveredTemplate, "name: vllm-cuda-runtime-template", "name: vllm-other-runtime-template", 1))}}
	if _, err := GenerateServingRuntimeArtifacts(&config, sources, "generated"); err == nil || !strings.Contains(err.Error(), "duplicate versioned ServingRuntime") {
		t.Fatalf("colliding nested ServingRuntime names accepted: %v", err)
	}
}

func TestTemplateNamesPreserveUnrelatedObjectsAndParameters(t *testing.T) {
	var manifest map[string]any
	if err := yaml.Unmarshal([]byte(discoveredTemplate), &manifest); err != nil {
		t.Fatal(err)
	}
	objects := manifest["objects"].([]any)
	objects = append(objects,
		map[string]any{"kind": "ServingRuntime", "metadata": map[string]any{"name": "second-runtime"}},
		map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "keep-this-name"}},
	)
	manifest["objects"] = objects
	manifest["parameters"] = []any{map[string]any{"name": "RUNTIME_NAME", "value": "custom-runtime"}}
	if err := versionRuntimeManifestNames(manifest, "0.26.0+rhaiv.8"); err != nil {
		t.Fatal(err)
	}
	if got := textAt(objectAt(objects[1].(map[string]any), "metadata"), "name"); got != "0-26-0-second-runtime" {
		t.Fatalf("additional ServingRuntime was not renamed: %s", got)
	}
	if got := textAt(objectAt(objects[2].(map[string]any), "metadata"), "name"); got != "keep-this-name" {
		t.Fatal("unrelated object renamed")
	}
	parameters := manifest["parameters"].([]any)[0].(map[string]any)
	if textAt(parameters, "name") != "RUNTIME_NAME" || textAt(parameters, "value") != "custom-runtime" {
		t.Fatal("OpenShift Template parameter modified")
	}
}
