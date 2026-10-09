package catalog

import (
	"strings"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestInferMinimumRHOAIVersion(t *testing.T) {
	for tag, want := range map[string]string{
		"v3.6.0-ea.1-1788535633": "3.6",
		"v3.6.4-12345":           "3.6",
		"3.7.1":                  "3.7",
		"v4.0":                   "4.0",
		"rhoai-3.6":              "3.6",
		"rhoai-3.7.2":            "3.7",
		"latest":                 "",
		"build-3.6.0":            "",
		"v3.06.0-123":            "",
	} {
		t.Run(tag, func(t *testing.T) {
			if got := inferMinimumRHOAIVersion("registry.redhat.io/rhoai/controller:" + tag); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
	digest := "@sha256:" + strings.Repeat("a", 64)
	for image, want := range map[string]string{
		"registry.redhat.io/rhoai/controller" + digest:                 "",
		"registry.redhat.io/rhoai/controller:v3.6.0-ea.1-123" + digest: "3.6",
		"quay.io/rhoai/controller:rhoai-3.6" + digest:                  "3.6",
		"": "",
	} {
		if got := inferMinimumRHOAIVersion(image); got != want {
			t.Fatalf("%q: got %q, want %q", image, got, want)
		}
	}
}

func TestGeneratedMinimumVersionPerControllerRelease(t *testing.T) {
	config, sources := discoveredSources()
	config.Sources[0].Include = []string{"config/runtimes/config.yaml"}
	config.Sources[0].TargetImage = "registry.redhat.io/rhoai/controller:v3.6.0-ea.1-123"
	second := config.Sources[0]
	second.ID = "next-release"
	second.TargetImage = "registry.redhat.io/rhoai/controller:v3.7.2-456"
	second.Templates = map[string]RuntimeTemplateConfig{"config/runtimes/config.yaml": {Version: "2.0"}}
	config.Sources = append(config.Sources, second)
	sources[second.ID] = sources["controller"]
	for _, override := range []string{"", "3.5"} {
		config.Sources[1].Templates["config/runtimes/config.yaml"] = RuntimeTemplateConfig{Version: "2.0", MinimumRHOAIVersion: override}
		artifacts, err := GenerateServingRuntimeArtifacts(&config, sources, "generated")
		if err != nil {
			t.Fatal(err)
		}
		var result types.ServingRuntimeCatalog
		if err := yaml.Unmarshal(artifacts.Catalog, &result); err != nil {
			t.Fatal(err)
		}
		versions := result.ServingRuntimes[0].Versions
		want := "3.7"
		if override != "" {
			want = override
		}
		if len(versions) != 2 || versions[0].MinimumRHOAIVersion != "3.6" || versions[1].MinimumRHOAIVersion != want {
			t.Fatalf("per-source minimum defaults or overrides lost: %+v", versions)
		}
	}
}

func TestMinimumVersionDefaultForBothManifestKinds(t *testing.T) {
	config, sources := discoveredSources()
	config.Sources[0].TargetImage = "registry.redhat.io/rhoai/controller:v3.6.0-ea.1-123"
	artifacts, err := GenerateServingRuntimeArtifacts(&config, sources, "generated")
	if err != nil {
		t.Fatal(err)
	}
	var result types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(artifacts.Catalog, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.ServingRuntimes) != 2 {
		t.Fatalf("missing manifest kind: %+v", result.ServingRuntimes)
	}
	for _, runtime := range result.ServingRuntimes {
		if runtime.Versions[0].MinimumRHOAIVersion != "3.6" {
			t.Fatalf("runtime %s did not receive source default", runtime.Name)
		}
	}
}
