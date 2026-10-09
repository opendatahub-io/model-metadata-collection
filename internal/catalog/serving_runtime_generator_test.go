package catalog

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

const discoveredTemplate = `apiVersion: template.openshift.io/v1
kind: Template
metadata:
  name: vllm-cuda-runtime-template
  annotations:
    openshift.io/display-name: vLLM NVIDIA
    description: CUDA runtime
    openshift.io/provider-display-name: Red Hat, Inc.
    tags: llm, gpu
objects:
  - apiVersion: serving.kserve.io/v1alpha1
    kind: ServingRuntime
    metadata:
      name: vllm-cuda
      annotations:
        opendatahub.io/runtime-version: v0.21.0
        opendatahub.io/recommended-accelerators: '["nvidia.com/gpu"]'
    spec:
      supportedModelFormats:
        - name: vLLM
          autoSelect: true
      containers:
        - name: kserve-container
          image: $(vllm-cuda-image)
          args: ["--served-model-name={{.Name}}", "$(UNCHANGED)"]
      workerSpec:
        containers:
          - name: worker
            image: $(vllm-cuda-image)
`

const discoveredLLMConfig = `apiVersion: serving.kserve.io/v1alpha2
kind: LLMInferenceServiceConfig
metadata:
  name: cuda-single-node
  annotations:
    openshift.io/display-name: vLLM CUDA single node
    openshift.io/description: LLM service runtime
    opendatahub.io/runtime-version: ""
    opendatahub.io/recommended-accelerators: '["nvidia.com/gpu"]'
spec:
  template:
    containers:
      - name: main
        image: placeholder
    initContainers:
      - name: helper
        image: registry.redhat.io/rhoai/helper:2.0
  prefill:
    template:
      containers:
        - name: main
          image: placeholder
`

func discoveredSources() (ServingRuntimeGeneratorConfig, map[string]fs.FS) {
	config := ServingRuntimeGeneratorConfig{Source: "Red Hat Serving Runtimes", Provider: "Red Hat, Inc.", Sources: []RuntimeSourceConfig{{
		ID: "controller", Directory: "unused-in-unit-test",
		Replacements: map[string]string{"$(vllm-cuda-image)": "registry.redhat.io/rhoai/vllm:1.0", "placeholder": "registry.redhat.io/rhoai/vllm:1.0"},
	}}}
	files := fstest.MapFS{
		"config/runtimes/vllm/deep/cuda.yaml": {Data: []byte(discoveredTemplate)},
		"config/runtimes/config.yaml":         {Data: []byte(discoveredLLMConfig)},
		"config/runtimes/kustomization.yaml":  {Data: []byte("kind: Kustomization\nresources: [vllm]\n")},
		"config/runtimes/csr.yaml":            {Data: []byte("kind: ClusterServingRuntime\n")},
		"examples/ignored.yaml":               {Data: []byte("broken: [")},
	}
	return config, map[string]fs.FS{"controller": files}
}

func TestGeneratedRuntimeAcceleratorCapabilities(t *testing.T) {
	for _, test := range []struct {
		name        string
		annotation  string
		requiresGPU bool
	}{
		{name: "missing annotation"},
		{name: "empty list", annotation: "[]"},
		{name: "null list", annotation: "null"},
		{name: "Nvidia", annotation: `["nvidia.com/gpu"]`, requiresGPU: true},
		{name: "AMD", annotation: `["amd.com/gpu"]`, requiresGPU: true},
		{name: "Gaudi", annotation: `["habana.ai/gaudi"]`, requiresGPU: true},
		{name: "Spyre PF", annotation: `["ibm.com/spyre_pf"]`, requiresGPU: true},
		{name: "Spyre VF", annotation: `["ibm.com/spyre_vf"]`, requiresGPU: true},
		{name: "new accelerator", annotation: `["example.com/accelerator"]`, requiresGPU: true},
		{name: "multiple accelerators", annotation: `["habana.ai/gaudi", "ibm.com/spyre_pf"]`, requiresGPU: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			config, sources := discoveredSources()
			replacement := ""
			if test.annotation != "" {
				replacement = "opendatahub.io/recommended-accelerators: '" + test.annotation + "'"
			}
			for _, file := range sources["controller"].(fstest.MapFS) {
				file.Data = []byte(strings.ReplaceAll(string(file.Data),
					`opendatahub.io/recommended-accelerators: '["nvidia.com/gpu"]'`, replacement))
			}
			artifacts, err := GenerateServingRuntimeArtifacts(&config, sources, "input/generated")
			if err != nil {
				t.Fatal(err)
			}
			var catalog types.ServingRuntimeCatalog
			if err := yaml.Unmarshal(artifacts.Catalog, &catalog); err != nil {
				t.Fatal(err)
			}
			if len(catalog.ServingRuntimes) != 2 || len(artifacts.RuntimeFiles) != 2 {
				t.Fatal("expected both a ServingRuntime Template and an LLMInferenceServiceConfig")
			}
			for _, runtime := range catalog.ServingRuntimes {
				if runtime.Capabilities == nil || runtime.Capabilities.RequiresGPU != test.requiresGPU {
					t.Errorf("catalog runtime %q: capabilities = %+v, want requiresGPU = %t",
						runtime.Name, runtime.Capabilities, test.requiresGPU)
				}
			}
			for file, data := range artifacts.RuntimeFiles {
				var input map[string]any
				if err := yaml.Unmarshal(data, &input); err != nil {
					t.Fatal(err)
				}
				if requiresGPU := objectAt(input, "capabilities")["requiresGPU"] == true; requiresGPU != test.requiresGPU {
					t.Errorf("generated input %q: requiresGPU = %t, want %t", file, requiresGPU, test.requiresGPU)
				}
			}
		})
	}
}

func TestParameterVersionsReplaceOlderTemplateAnnotations(t *testing.T) {
	config, sources := discoveredSources()
	image := "registry.redhat.io/rhoai/vllm:1.0"
	config.Sources[0].Parameters = map[string]string{
		"vllm-cuda-image": image, "vllm-cuda-image-upstream-version": "0.26.0+rhaiv.8",
	}
	artifacts, err := GenerateServingRuntimeArtifacts(&config, sources, "input/generated")
	if err != nil {
		t.Fatal(err)
	}
	var result types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(artifacts.Catalog, &result); err != nil {
		t.Fatal(err)
	}
	for _, runtime := range result.ServingRuntimes {
		version := runtime.Versions[0]
		if version.Version != "0.26.0+rhaiv.8" {
			t.Fatalf("stale source annotation won over image parameter: %s", version.Version)
		}
		manifestText := version.ServingRuntimeTemplate
		if manifestText == "" {
			manifestText = version.LLMInferenceServiceConfig
		}
		var manifest map[string]any
		if err := json.Unmarshal([]byte(manifestText), &manifest); err != nil {
			t.Fatal(err)
		}
		resource := servingRuntimeResource(manifest)
		if resource == nil {
			resource = manifest
		}
		if textAt(objectAt(resource, "metadata", "annotations"), "opendatahub.io/runtime-version") != version.Version {
			t.Fatal("parameter version not stamped into generated manifest")
		}
	}
	config.Sources[0].Parameters["duplicate-image"] = image
	config.Sources[0].Parameters["duplicate-image-upstream-version"] = "conflicting"
	if _, err := GenerateServingRuntimeArtifacts(&config, sources, "input/generated"); err == nil {
		t.Fatal("conflicting parameters for the same runtime image accepted")
	}
}

func TestGenerateServingRuntimeArtifacts(t *testing.T) {
	config, sources := discoveredSources()
	first, err := GenerateServingRuntimeArtifacts(&config, sources, "input/generated")
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateServingRuntimeArtifacts(&config, sources, "input/generated")
	if err != nil || !bytes.Equal(first.Catalog, second.Catalog) || !bytes.Equal(first.Index, second.Index) {
		t.Fatalf("generation is not deterministic: %v", err)
	}
	var catalog types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(first.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.ServingRuntimes) != 2 {
		t.Fatalf("expected independent entries for both templates: %s", first.Catalog)
	}
	llm, serving := catalog.ServingRuntimes[0], catalog.ServingRuntimes[1]
	if llm.Versions[0].ServingRuntimeTemplate != "" || serving.Versions[0].LLMInferenceServiceConfig != "" {
		t.Fatal("unrelated manifests were paired")
	}
	if llm.Versions[0].Version != "1.0" || llm.Versions[0].Image != "registry.redhat.io/rhoai/vllm:1.0" {
		t.Fatalf("version/image should come from main container, not helper: %+v", llm.Versions[0])
	}
	if serving.Versions[0].Version != "v0.21.0" || serving.DisplayName != "vLLM NVIDIA" || !serving.Capabilities.RequiresGPU || len(serving.SupportedModelFormats) != 1 {
		t.Fatalf("metadata was not derived from the template: %+v", serving)
	}
	for _, runtime := range catalog.ServingRuntimes {
		version := runtime.Versions[0]
		manifest := version.ServingRuntimeTemplate + version.LLMInferenceServiceConfig
		if strings.Contains(manifest, "placeholder") || strings.Contains(manifest, "$(vllm-cuda-image)") {
			t.Fatal("an image placeholder was retained")
		}
		if strings.Count(manifest, "registry.redhat.io/rhoai/vllm:1.0") != 2 {
			t.Fatal("worker or prefill image was not replaced")
		}
	}
	if !strings.Contains(serving.Versions[0].ServingRuntimeTemplate, "{{.Name}}") || !strings.Contains(serving.Versions[0].ServingRuntimeTemplate, "$(UNCHANGED)") {
		t.Fatal("non-image expressions changed")
	}
	inputFiles := fstest.MapFS{}
	for file, data := range first.RuntimeFiles {
		if strings.Contains(string(data), "servingRuntimeTemplate: '{") {
			t.Fatal("generated input manifests should remain YAML objects")
		}
		inputFiles[file] = &fstest.MapFile{Data: data}
	}
	regenerated, err := GenerateServingRuntimeCatalog(first.Index, inputFiles)
	if err != nil || !bytes.Equal(first.Catalog, regenerated) {
		t.Fatalf("generated inputs/index cannot reproduce catalog: %v", err)
	}
}

func TestGenerateServingRuntimeArtifactsFailures(t *testing.T) {
	cases := map[string]func(*ServingRuntimeGeneratorConfig){
		"unresolved image": func(c *ServingRuntimeGeneratorConfig) { delete(c.Sources[0].Replacements, "$(vllm-cuda-image)") },
		"latest image": func(c *ServingRuntimeGeneratorConfig) {
			c.Sources[0].Replacements["placeholder"] = "registry.redhat.io/rhoai/vllm:latest"
		},
		"unapproved primary image": func(c *ServingRuntimeGeneratorConfig) {
			c.Sources[0].Replacements["placeholder"] = "quay.io/unapproved/vllm:1.0"
		},
		"unapproved init image": func(c *ServingRuntimeGeneratorConfig) {
			c.Sources[0].Replacements["registry.redhat.io/rhoai/helper:2.0"] = "quay.io/unapproved/helper:2.0"
		},
		"unmatched include": func(c *ServingRuntimeGeneratorConfig) { c.Sources[0].Include = []string{"missing.yaml"} },
		"unmatched override": func(c *ServingRuntimeGeneratorConfig) {
			c.Sources[0].Templates = map[string]RuntimeTemplateConfig{"missing.yaml": {Version: "1.0"}}
		},
		"duplicate version": func(c *ServingRuntimeGeneratorConfig) { c.Sources = append(c.Sources, c.Sources[0]) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			config, sources := discoveredSources()
			mutate(&config)
			if _, err := GenerateServingRuntimeArtifacts(&config, sources, "input/generated"); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestGenerateServingRuntimeArtifactsSkipsOnlyAffectedVersions(t *testing.T) {
	missing := "registry.redhat.io/rhoai/unpublished:latest"
	for _, tc := range []struct {
		name   string
		mutate func(*ServingRuntimeGeneratorConfig, fstest.MapFS)
		want   string
	}{
		{"unused parameter", func(c *ServingRuntimeGeneratorConfig, _ fstest.MapFS) {
			c.Sources[0].Parameters = map[string]string{"future-image": missing}
		}, "both"},
		{"Template primary and worker", func(c *ServingRuntimeGeneratorConfig, _ fstest.MapFS) {
			c.Sources[0].Replacements["$(vllm-cuda-image)"] = missing
		}, "cuda-single-node"},
		{"LLM prefill image", func(_ *ServingRuntimeGeneratorConfig, files fstest.MapFS) {
			data := strings.Replace(discoveredLLMConfig, "  prefill:\n    template:\n      containers:\n        - name: main\n          image: placeholder", "  prefill:\n    template:\n      containers:\n        - name: main\n          image: "+missing, 1)
			files["config/runtimes/config.yaml"] = &fstest.MapFile{Data: []byte(data)}
		}, "vllm-cuda"},
		{"LLM init image", func(c *ServingRuntimeGeneratorConfig, _ fstest.MapFS) {
			c.Sources[0].Replacements["registry.redhat.io/rhoai/helper:2.0"] = missing
		}, "vllm-cuda"},
		{"explicit image override", func(c *ServingRuntimeGeneratorConfig, _ fstest.MapFS) {
			c.Sources[0].Templates = map[string]RuntimeTemplateConfig{"config/runtimes/config.yaml": {Image: missing}}
		}, "vllm-cuda"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, sources := discoveredSources()
			config.Sources[0].MissingImages = map[string]bool{missing: true}
			tc.mutate(&config, sources["controller"].(fstest.MapFS))
			artifacts, err := GenerateServingRuntimeArtifacts(&config, sources, "generated")
			if err != nil {
				t.Fatal(err)
			}
			var result types.ServingRuntimeCatalog
			if err := yaml.Unmarshal(artifacts.Catalog, &result); err != nil {
				t.Fatal(err)
			}
			if tc.want == "both" {
				if len(result.ServingRuntimes) != 2 || len(artifacts.SkippedTemplates) != 0 {
					t.Fatal("unused parameter excluded available templates")
				}
				return
			}
			if len(result.ServingRuntimes) != 1 || result.ServingRuntimes[0].Name != tc.want {
				t.Fatalf("wrong runtime omitted: %+v", result.ServingRuntimes)
			}
			if len(artifacts.SkippedTemplates) != 1 || len(artifacts.SkippedTemplates[0].MissingImages) != 1 || artifacts.SkippedTemplates[0].MissingImages[0] != missing {
				t.Fatalf("missing image diagnostic is incomplete: %+v", artifacts.SkippedTemplates)
			}
		})
	}
}

func TestMissingReleaseRetainsOtherVersions(t *testing.T) {
	config, sources := discoveredSources()
	config.Sources[0].Include = []string{"config/runtimes/config.yaml"}
	missing := "registry.redhat.io/rhoai/vllm:future"
	second := RuntimeSourceConfig{
		ID: "future", Directory: "unused", Include: config.Sources[0].Include,
		Replacements: map[string]string{"placeholder": missing}, MissingImages: map[string]bool{missing: true},
	}
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
	if len(result.ServingRuntimes) != 1 || len(result.ServingRuntimes[0].Versions) != 1 || result.ServingRuntimes[0].Versions[0].Version != "1.0" {
		t.Fatalf("available release was lost: %+v", result.ServingRuntimes)
	}
	if len(artifacts.SkippedTemplates) != 1 || artifacts.SkippedTemplates[0].SourceID != "future" {
		t.Fatal("missing source was not reported")
	}
	config.Sources = config.Sources[1:]
	if _, err := GenerateServingRuntimeArtifacts(&config, sources, "generated"); err == nil || !strings.Contains(err.Error(), "no runtime versions have available parameter images") {
		t.Fatalf("all-unavailable configuration accepted: %v", err)
	}
}

func TestGenerateServingRuntimeArtifactsVersionsAndOverrides(t *testing.T) {
	config, sources := discoveredSources()
	config.Sources[0].Include = []string{"config/runtimes/config.yaml"}
	digestImage := "registry.redhat.io/rhoai/vllm@sha256:" + strings.Repeat("a", 64)
	config.Sources[0].Replacements["placeholder"] = digestImage
	config.Sources[0].ImageVersions = map[string]string{digestImage: "0.21.0"}
	second := config.Sources[0]
	second.ID = "new-release"
	second.Templates = map[string]RuntimeTemplateConfig{"config/runtimes/config.yaml": {
		Version: "0.22.0", Image: "registry.redhat.io/rhoai/vllm:0.22.0", MinimumRHOAIVersion: "3.6",
	}}
	config.Sources = append(config.Sources, second)
	sources[second.ID] = sources["controller"]
	artifacts, err := GenerateServingRuntimeArtifacts(&config, sources, "generated")
	if err != nil {
		t.Fatal(err)
	}
	var catalog types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(artifacts.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	versions := catalog.ServingRuntimes[0].Versions
	if len(versions) != 2 || versions[1].Version != "0.22.0" || versions[1].SupportLevel != "techPreview" || versions[1].MinimumRHOAIVersion != "3.6" {
		t.Fatalf("overrides/version aggregation lost: %+v", versions)
	}
	var manifest map[string]any
	if err := json.Unmarshal([]byte(versions[1].LLMInferenceServiceConfig), &manifest); err != nil {
		t.Fatal(err)
	}
	if textAt(objectAt(manifest, "metadata", "annotations"), "opendatahub.io/runtime-version") != "0.22.0" {
		t.Fatal("version override was not reflected in the template")
	}
}

func TestParseServingRuntimeGeneratorConfig(t *testing.T) {
	valid := "source: Red Hat Serving Runtimes\nsources:\n  - id: controller\n    directory: ../checkout\n"
	config, err := ParseServingRuntimeGeneratorConfig([]byte(valid))
	if err != nil || config.Provider != "Red Hat, Inc." {
		t.Fatalf("cannot parse config/default provider: %v", err)
	}
	for name, input := range map[string]string{
		"unknown field":         valid + "    typo: true\n",
		"manual support policy": valid + "    support_level: supported\n",
		"two locations":         valid + "    target_image: registry.redhat.io/rhoai/controller:1.0\n",
		"traversal":             valid + "    template_paths: [../outside]\n",
		"unpinned target":       valid + "    source_image: registry.redhat.io/rhoai/controller\n",
		"multiple documents":    valid + "---\nsource: unexpected\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseServingRuntimeGeneratorConfig([]byte(input)); err == nil {
				t.Fatal("expected invalid configuration")
			}
		})
	}
}

func TestCommittedServingRuntimeGeneratorConfig(t *testing.T) {
	input, err := os.ReadFile("../../input/serving_runtimes/generator-config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseServingRuntimeGeneratorConfig(input); err != nil {
		t.Fatal(err)
	}
}
