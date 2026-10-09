package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opendatahub-io/model-metadata-collection/internal/catalog"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestGenerateFromConfigAndCheck(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.Mkdir("templates", 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `apiVersion: serving.kserve.io/v1alpha2
kind: LLMInferenceServiceConfig
metadata:
  name: cuda
  annotations:
    openshift.io/display-name: CUDA
    openshift.io/description: CUDA runtime
spec:
  template:
    containers:
      - name: main
        image: placeholder
`
	config := `source: Red Hat Serving Runtimes
sources:
  - id: templates
    directory: templates
    replacements:
      placeholder: registry.redhat.io/rhoai/vllm:1.0
`
	for file, data := range map[string]string{"templates/runtime.yaml": manifest, "generator.yaml": config} {
		if err := os.WriteFile(file, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	generate := func(check bool) error {
		return generateFromConfig(context.Background(), "generator.yaml", "", "input/generated", "data/index.yaml", "data/catalog.yaml", check, false)
	}
	if err := generate(false); err != nil {
		t.Fatal(err)
	}
	if err := generate(true); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile("data/index.yaml")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := catalog.GenerateServingRuntimeCatalog(index, os.DirFS("."))
	if err != nil {
		t.Fatal(err)
	}
	existing, err := os.ReadFile("data/catalog.yaml")
	if err != nil || !bytes.Equal(existing, generated) {
		t.Fatal("generated index/runtime inputs do not reproduce the catalog")
	}
	if err := os.WriteFile("data/index.yaml", []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := generate(true); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Fatalf("check did not detect stale index: %v", err)
	}
	data, err := os.ReadFile("data/index.yaml")
	if err != nil || string(data) != "stale" {
		t.Fatal("--check modified an output")
	}
	invalid := strings.Replace(config, "registry.redhat.io/rhoai/vllm:1.0", "registry.redhat.io/rhoai/vllm:latest", 1)
	if err := os.WriteFile("generator.yaml", []byte(invalid), 0644); err != nil {
		t.Fatal(err)
	}
	if err := generate(false); err == nil {
		t.Fatal("invalid images were accepted")
	}
	data, err = os.ReadFile("data/catalog.yaml")
	if err != nil || !bytes.Equal(data, existing) {
		t.Fatal("failed generation modified reviewed outputs")
	}
	staged, err := filepath.Glob("data/.serving-runtime-*")
	if err != nil || len(staged) != 0 {
		t.Fatal("temporary output files were retained")
	}
}

func TestGenerateFromMultipleControllerReleasesPreservesVersions(t *testing.T) {
	t.Chdir(t.TempDir())
	manifests := map[string]string{
		"llmisvc": `apiVersion: serving.kserve.io/v1alpha2
kind: LLMInferenceServiceConfig
metadata:
  name: kserve-config-llm-cuda
spec:
  template:
    containers:
      - name: main
        image: placeholder
`,
		"model-controller": `apiVersion: template.openshift.io/v1
kind: Template
metadata:
  name: vllm-cuda-runtime-template
objects:
  - apiVersion: serving.kserve.io/v1alpha1
    kind: ServingRuntime
    metadata:
      name: vllm-cuda-runtime
    spec:
      containers:
        - name: kserve-container
          image: placeholder
`,
	}
	var config strings.Builder
	config.WriteString("source: Red Hat Serving Runtimes\nsources:\n")
	for _, controller := range []string{"llmisvc", "model-controller"} {
		if err := os.Mkdir(controller, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(controller, "runtime.yaml"), []byte(manifests[controller]), 0644); err != nil {
			t.Fatal(err)
		}
		for i, version := range []string{"0.26.0+rhaiv.8", "0.27.0+rhaiv.1"} {
			fmt.Fprintf(&config, `  - id: %s-release-%d
    directory: %s
    replacements:
      placeholder: registry.redhat.io/rhoai/vllm:release-%d
    templates:
      runtime.yaml:
        version: %s
`, controller, i, controller, i, version)
		}
	}
	if err := os.WriteFile("generator.yaml", []byte(config.String()), 0644); err != nil {
		t.Fatal(err)
	}
	generate := func(check bool) error {
		return generateFromConfig(context.Background(), "generator.yaml", "", "input/generated", "data/index.yaml", "data/catalog.yaml", check, false)
	}
	if err := generate(false); err != nil {
		t.Fatal(err)
	}
	if err := generate(true); err != nil {
		t.Fatal(err)
	}
	indexData, err := os.ReadFile("data/index.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var index types.ServingRuntimeIndex
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.ServingRuntimes) != 2 {
		t.Fatalf("expected one file per runtime, got %d index entries", len(index.ServingRuntimes))
	}
	snapshot := map[string][]byte{"data/index.yaml": indexData}
	for _, entry := range index.ServingRuntimes {
		data, err := os.ReadFile(entry.InputPath)
		if err != nil {
			t.Fatal(err)
		}
		snapshot[entry.InputPath] = data
		var runtime map[string]any
		if err := yaml.Unmarshal(data, &runtime); err != nil {
			t.Fatal(err)
		}
		if runtime["name"] != entry.Name || len(runtime["versions"].([]any)) != 2 {
			t.Fatalf("release overwritten in %s", entry.InputPath)
		}
		for i, version := range runtime["versions"].([]any) {
			values := version.(map[string]any)
			prefix := []string{"0-26-0", "0-27-0"}[i]
			for _, field := range []string{"servingRuntimeTemplate", "llmInferenceServiceConfig"} {
				manifest, ok := values[field].(map[string]any)
				if !ok {
					continue
				}
				name := manifest["metadata"].(map[string]any)["name"].(string)
				if !strings.HasPrefix(name, prefix+"-") {
					t.Fatalf("version %d lost its object name: %s", i, name)
				}
			}
		}
	}
	catalogData, err := os.ReadFile("data/catalog.yaml")
	if err != nil {
		t.Fatal(err)
	}
	snapshot["data/catalog.yaml"] = catalogData
	rebuilt, err := catalog.GenerateServingRuntimeCatalog(indexData, os.DirFS("."))
	if err != nil || !bytes.Equal(rebuilt, catalogData) {
		t.Fatalf("multi-release inputs cannot reproduce the catalog: %v", err)
	}
	// A later release with a colliding name must leave every existing output intact.
	conflicting := strings.ReplaceAll(config.String(), "0.27.0+rhaiv.1", "0.26.0+rhaiv.99")
	if err := os.WriteFile("generator.yaml", []byte(conflicting), 0644); err != nil {
		t.Fatal(err)
	}
	if err := generate(false); err == nil || !strings.Contains(err.Error(), "duplicate versioned") {
		t.Fatalf("conflicting release accepted: %v", err)
	}
	for file, before := range snapshot {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("conflicting release overwrote %s", file)
		}
	}
}

func TestGenerateWithUnpublishedParameterImages(t *testing.T) {
	t.Chdir(t.TempDir())
	manifest := func(name string) string {
		return fmt.Sprintf(`apiVersion: serving.kserve.io/v1alpha2
kind: LLMInferenceServiceConfig
metadata:
  name: kserve-%s
spec:
  template:
    containers:
      - name: main
        image: placeholder
`, name)
	}
	config := `source: Red Hat Serving Runtimes
sources:
  - id: previous
    directory: controller
    parameters:
      runtime-image: registry.redhat.io/rhoai/vllm:3.5
      ovms-image: registry.redhat.io/rhoai/ovms:3.5
      runtime-version: "0.21.0"
  - id: ga
    directory: controller
    parameters:
      runtime-image: registry.redhat.io/rhoai/vllm:3.6
      ovms-image: registry.redhat.io/rhoai/ovms:3.6
      runtime-version: "0.22.0"
`
	skopeo := `#!/bin/sh
for arg; do image="$arg"; done
case "$REGISTRY_TEST_STATE" in
  auth) printf '%s' 'unauthorized: authentication required' >&2; exit 1 ;;
  absent) printf '%s' 'manifest unknown' >&2; exit 1 ;;
esac
case "$image" in
  docker://registry.redhat.io/rhoai/vllm:3.6)
    case "$REGISTRY_TEST_STATE" in
      auth-ga) printf '%s' 'unauthorized: access to the requested resource is not authorized' >&2 ;;
      not-found-ga) printf '%s' 'unexpected status 404 Not Found' >&2 ;;
      transport-ga) printf '%s' 'dial tcp: connection refused' >&2 ;;
      *) printf '%s' 'manifest unknown' >&2 ;;
    esac
    exit 1 ;;
esac
printf '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[]}'
`
	kustomize := `#!/bin/sh
while IFS='=' read -r key value; do
  case "$key" in
    runtime-image) runtime_image="$value" ;;
    ovms-image) ovms_image="$value" ;;
    runtime-version) runtime_version="$value" ;;
  esac
done < "$2/params.env"
for runtime in cuda ovms; do
  image="$runtime_image"
  if [ "$runtime" = ovms ]; then image="$ovms_image"; fi
  cat <<EOF
apiVersion: serving.kserve.io/v1alpha2
kind: LLMInferenceServiceConfig
metadata:
  name: kserve-$runtime
  annotations:
    opendatahub.io/runtime-version: "$runtime_version"
spec:
  template:
    containers:
      - name: main
        image: $image
---
EOF
done
`
	for file, data := range map[string]string{
		"generator.yaml": config, "bin/skopeo": skopeo, "bin/kustomize": kustomize,
		"controller/config/base/params.env":         "runtime-image=placeholder\novms-image=placeholder\nruntime-version=\n",
		"controller/config/base/kustomization.yaml": "kind: Kustomization\n",
		"controller/config/runtimes/cuda.yaml":      manifest("cuda"),
		"controller/config/runtimes/ovms.yaml":      manifest("ovms"),
	} {
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0755); err != nil {
			t.Fatal(err)
		}
	}
	bin, err := filepath.Abs("bin")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("REGISTRY_TEST_STATE", "")
	generate := func(check bool) error {
		return generateFromConfig(context.Background(), "generator.yaml", "", "input/generated", "data/index.yaml", "data/catalog.yaml", check, false)
	}
	if err := generate(false); err != nil {
		t.Fatal(err)
	}
	if err := generate(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("data/catalog.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var result types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.ServingRuntimes) != 2 || len(result.ServingRuntimes[0].Versions) != 1 || len(result.ServingRuntimes[1].Versions) != 2 {
		t.Fatalf("missing GA image excluded other releases or runtimes: %+v", result.ServingRuntimes)
	}
	for _, runtime := range result.ServingRuntimes {
		for _, version := range runtime.Versions {
			if !strings.Contains(version.Image, "@sha256:") {
				t.Fatalf("available floating tag was not pinned: %s", version.Image)
			}
		}
	}
	snapshot := map[string][]byte{}
	for _, file := range []string{"data/index.yaml", "data/catalog.yaml", "input/generated/kserve-cuda.yaml", "input/generated/kserve-ovms.yaml"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		snapshot[file] = data
	}
	for _, state := range []string{"auth", "absent"} {
		t.Setenv("REGISTRY_TEST_STATE", state)
		for _, skip := range []bool{false, true} {
			if err := generateFromConfig(context.Background(), "generator.yaml", "", "input/generated", "data/index.yaml", "data/catalog.yaml", false, skip); err == nil {
				t.Fatalf("%s registry state with skip=%t overwrote outputs", state, skip)
			}
		}
		for file, before := range snapshot {
			after, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("%s registry state changed %s", state, file)
			}
		}
	}
	for _, state := range []string{"auth-ga", "not-found-ga", "transport-ga"} {
		t.Setenv("REGISTRY_TEST_STATE", state)
		if err := generate(false); err == nil {
			t.Fatalf("strict generation suppressed %s", state)
		}
		generateWithSkip := func(check bool) error {
			return generateFromConfig(context.Background(), "generator.yaml", "", "input/generated", "data/index.yaml", "data/catalog.yaml", check, true)
		}
		err := generateWithSkip(false)
		if state == "transport-ga" {
			if err == nil {
				t.Fatal("skip flag suppressed a transport failure")
			}
		} else {
			if err != nil {
				t.Fatalf("skip flag did not handle %s: %v", state, err)
			}
			if err := generateWithSkip(true); err != nil {
				t.Fatal(err)
			}
		}
		for file, before := range snapshot {
			after, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("%s changed available releases in %s", state, file)
			}
		}
	}
}

func TestGenerateWithUnavailableControllerSources(t *testing.T) {
	t.Chdir(t.TempDir())
	const available = `source: Red Hat Serving Runtimes
sources:
  - id: available
    directory: controller
`
	const target = `  - id: ga
    target_image: registry.redhat.io/rhoai/controller:v3.6
`
	const explicitSource = `  - id: ga
    source_image: registry.redhat.io/rhoai/controller:v3.6-source
`
	skopeo := `#!/bin/sh
case "$*" in
  *'inspect --no-tags'*)
    if [ "$CONTROLLER_TEST_MODE" = target ]; then
      printf '%s' "$CONTROLLER_TEST_ERROR" >&2; exit 2
    fi
    if [ "$CONTROLLER_TEST_MODE" = metadata ]; then printf '{'; exit 0; fi
    printf '{"Digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Labels":{}}'
    ;;
  *'inspect --raw'*)
    if [ "$CONTROLLER_TEST_MODE" = copy ]; then printf '{"schemaVersion":2}'; exit 0; fi
    if [ "$CONTROLLER_TEST_MODE" = mixed ]; then
      case "$*" in *'.src'*) printf '%s' 'dial tcp: connection refused' >&2; exit 2 ;; esac
    fi
    printf '%s' "$CONTROLLER_TEST_ERROR" >&2; exit 2
    ;;
  *'copy'*) printf '%s' "$CONTROLLER_TEST_ERROR" >&2; exit 2 ;;
  *) exit 3 ;;
esac
`
	manifest := `apiVersion: serving.kserve.io/v1alpha2
kind: LLMInferenceServiceConfig
metadata:
  name: kserve-cuda
spec:
  template:
    containers:
      - name: main
        image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.5
`
	for file, data := range map[string]string{"controller/runtime.yaml": manifest, "bin/skopeo": skopeo, "generator.yaml": available} {
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0755); err != nil {
			t.Fatal(err)
		}
	}
	bin, err := filepath.Abs("bin")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	generate := func(check, skip bool) error {
		return generateFromConfig(context.Background(), "generator.yaml", "", "input/generated", "data/index.yaml", "data/catalog.yaml", check, skip)
	}
	if err := generate(false, false); err != nil {
		t.Fatal(err)
	}
	snapshot := map[string][]byte{}
	for _, file := range []string{"data/index.yaml", "data/catalog.yaml", "input/generated/kserve-cuda.yaml"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		snapshot[file] = data
	}
	for _, tc := range []struct {
		name string
		mode string
		err  string
		skip bool
	}{
		{"missing target", "target", "manifest unknown", true},
		{"denied target", "target", "unauthorized: access to the requested resource is not authorized", true},
		{"HTTP 403", "target", "unexpected status code 403", true},
		{"HTTP 404", "target", "unexpected status 404 Not Found", true},
		{"missing explicit source", "source", "manifest unknown", true},
		{"denied explicit source", "source", "unauthorized", true},
		{"missing attachments", "attachment", "manifest unknown", true},
		{"denied attachments", "attachment", "unauthorized", true},
		{"mixed attachment failures", "mixed", "manifest unknown", false},
		{"network", "target", "dial tcp: connection refused", false},
		{"TLS", "target", "x509: certificate signed by unknown authority", false},
		{"server error", "target", "unexpected status code 500", false},
		{"invalid metadata", "metadata", "manifest unknown", false},
		{"copy error", "copy", "manifest unknown", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CONTROLLER_TEST_MODE", tc.mode)
			t.Setenv("CONTROLLER_TEST_ERROR", tc.err)
			extra := target
			if tc.mode == "source" {
				extra = explicitSource
			}
			if tc.skip {
				extra += "    parameters:\n      unused-image: registry.redhat.io/rhoai/preflight-must-skip:3.6\n"
			}
			generate := generate
			if tc.mode == "mixed" || tc.name == "network" || tc.name == "server error" {
				generate = func(check, skip bool) error {
					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()
					return generateFromConfig(ctx, "generator.yaml", "", "input/generated", "data/index.yaml", "data/catalog.yaml", check, skip)
				}
			}
			if err := os.WriteFile("generator.yaml", []byte(available+extra), 0644); err != nil {
				t.Fatal(err)
			}
			if err := generate(false, false); err == nil {
				t.Fatal("strict mode ignored controller failure")
			}
			err := generate(false, true)
			if tc.skip {
				if err != nil {
					t.Fatalf("unavailable controller was not skipped: %v", err)
				}
				if err := generate(true, true); err != nil {
					t.Fatalf("source check failed after skipping controller: %v", err)
				}
			} else if err == nil {
				t.Fatal("skip flag suppressed a non-availability failure")
			}
			for file, before := range snapshot {
				after, err := os.ReadFile(file)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("controller failure changed available output %s", file)
				}
			}
		})
	}
	// An empty result must not overwrite an existing catalog or its inputs.
	t.Setenv("CONTROLLER_TEST_MODE", "target")
	t.Setenv("CONTROLLER_TEST_ERROR", "manifest unknown")
	if err := os.WriteFile("generator.yaml", []byte("source: Red Hat Serving Runtimes\nsources:\n"+target), 0644); err != nil {
		t.Fatal(err)
	}
	if err := generate(false, true); err == nil || !strings.Contains(err.Error(), "no controller sources are available") {
		t.Fatalf("all unavailable controllers did not fail generation: %v", err)
	}
	for file, before := range snapshot {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("all unavailable controllers changed %s", file)
		}
	}
}
