package catalog

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestRuntimeKustomizeParametersAndAnnotations(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize executable is not installed")
	}
	root := t.TempDir()
	params := "# Preserve source defaults\nruntime-image=registry.redhat.io/rhoai/old:1.0\nruntime-version=0.21.0\n"
	files := map[string]string{
		"config/runtimes/vllm.yaml":          discoveredTemplate,
		"config/runtimes/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: [vllm.yaml]\n",
		"config/base/params.env":             params,
		"config/base/kustomization.yaml": `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../runtimes
configMapGenerator:
  - name: parameters
    envs: [params.env]
replacements:
  - source:
      kind: ConfigMap
      name: parameters
      fieldPath: data.runtime-image
    targets:
      - select:
          kind: Template
          name: vllm-cuda-runtime-template
        fieldPaths:
          - objects.0.spec.containers.0.image
          - objects.0.spec.workerSpec.containers.0.image
  - source:
      kind: ConfigMap
      name: parameters
      fieldPath: data.runtime-version
    targets:
      - select:
          kind: Template
          name: vllm-cuda-runtime-template
        fieldPaths:
          - objects.0.metadata.annotations.[opendatahub.io/runtime-version]
`,
	}
	for name, data := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	source := RuntimeSourceConfig{ID: "controller", Directory: root, Parameters: map[string]string{
		"runtime-image": "registry.redhat.io/rhoai/new:2.0", "runtime-version": "0.22.0",
		// This newer parameter is absent from the older source's params/overlay.
		"kserve-llm-d-cpu":                  "registry.redhat.io/rhaii-fast/vllm-cpu-rhel9:3.6.0-fast.1",
		"kserve-llm-d-cpu-upstream-version": "0.26.0+rhaiv.5",
	}}
	loaded, cleanup, err := LoadServingRuntimeSource(context.Background(), source, "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	stagedParams, err := fs.ReadFile(loaded, "config/base/params.env")
	if err != nil || !strings.Contains(string(stagedParams), "kserve-llm-d-cpu="+source.Parameters["kserve-llm-d-cpu"]+"\n") {
		t.Fatalf("new parameters were not supplied to the older overlay: %s: %v", stagedParams, err)
	}
	config := &ServingRuntimeGeneratorConfig{Source: "Red Hat Serving Runtimes", Provider: "Red Hat", Sources: []RuntimeSourceConfig{source}}
	artifacts, err := GenerateServingRuntimeArtifacts(config, map[string]fs.FS{source.ID: loaded}, "generated")
	if err != nil {
		t.Fatal(err)
	}
	var catalog types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(artifacts.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	version := catalog.ServingRuntimes[0].Versions[0]
	if version.Image != "registry.redhat.io/rhoai/new:2.0" || version.Version != "0.22.0" || strings.Contains(version.ServingRuntimeTemplate, "$(vllm-cuda-image)") {
		t.Fatalf("kustomize did not apply the source's mappings: %+v", version)
	}
	for _, test := range []struct {
		image string
		want  string
	}{
		{"registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6.0", "supported"},
		{"registry.redhat.io/rhaii-fast/vllm-cuda-rhel9:3.6.0-fast.1", "techPreview"},
	} {
		source.Parameters["runtime-image"] = test.image
		loaded, cleanup, err := LoadServingRuntimeSource(context.Background(), source, "")
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		// Use the fixture's rendered source tree as if it came from a GA target.
		config.Sources[0].Directory = ""
		config.Sources[0].TargetImage = "registry.redhat.io/rhoai/controller:v3.6.0-123"
		artifacts, err := GenerateServingRuntimeArtifacts(config, map[string]fs.FS{source.ID: loaded}, "generated")
		if err != nil {
			t.Fatal(err)
		}
		if err := yaml.Unmarshal(artifacts.Catalog, &catalog); err != nil {
			t.Fatal(err)
		}
		if got := catalog.ServingRuntimes[0].Versions[0].SupportLevel; got != test.want {
			t.Fatalf("parameter image %s: got %s, want %s", test.image, got, test.want)
		}
	}
	for file, expected := range files {
		actual, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil || string(actual) != expected {
			t.Fatalf("source checkout was modified: %s: %v", file, err)
		}
	}
}

func TestReplaceRuntimeParameters(t *testing.T) {
	input := []byte("# vllm-image=comment\nvllm-image=old\nstate=managed\n")
	output, err := replaceRuntimeParameters(input, map[string]string{"vllm-image": "registry.redhat.io/rhoai/vllm:1.0"})
	if err != nil || string(output) != "# vllm-image=comment\nvllm-image=registry.redhat.io/rhoai/vllm:1.0\nstate=managed\n" {
		t.Fatalf("unexpected params.env output %q: %v", output, err)
	}
	withNewKeys := map[string]string{"future-version": "0.26.0", "future-image": "registry.redhat.io/rhoai/future:1.0"}
	output, err = replaceRuntimeParameters(input, withNewKeys)
	if err != nil || string(output) != string(input)+"future-image=registry.redhat.io/rhoai/future:1.0\nfuture-version=0.26.0\n" {
		t.Fatalf("new parameters should be appended in deterministic order: %q: %v", output, err)
	}
	for _, parameters := range []map[string]string{
		{"vllm-image": "value\nstate=other"}, {"future-image": "value\nstate=other"},
		{"invalid=key": "value"}, {"": "value"}, {"#comment": "value"},
	} {
		if _, err := replaceRuntimeParameters(input, parameters); err == nil {
			t.Fatal("expected invalid parameter override")
		}
	}
}
