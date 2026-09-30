package catalog

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestRuntimeTemplateInputs(t *testing.T) {
	for _, variant := range []string{"cpu", "cpu-x86", "cuda", "gaudi", "rocm", "spyre-s390x"} {
		t.Run(variant, func(t *testing.T) {
			data, err := os.ReadFile("../../input/serving_runtimes/redhat/vllm-" + variant + ".yaml")
			if err != nil {
				t.Fatal(err)
			}
			name := "vllm-" + variant + "-runtime"
			index := fmt.Sprintf("source: Red Hat Serving Runtimes\nserving_runtimes:\n  - name: %s\n    input_path: runtime.yaml\n    image: quay.io/example/runtime:1.0\n", name)
			files := fstest.MapFS{"runtime.yaml": &fstest.MapFile{Data: data}}
			output, err := GenerateServingRuntimeCatalog([]byte(index), files)
			if err != nil {
				t.Fatal(err)
			}
			again, err := GenerateServingRuntimeCatalog([]byte(index), files)
			if err != nil || string(again) != string(output) {
				t.Fatalf("not deterministic: %v", err)
			}
			var catalog types.ServingRuntimeCatalog
			if err := yaml.Unmarshal(output, &catalog); err != nil {
				t.Fatal(err)
			}
			r := catalog.ServingRuntimes[0]
			v := r.Versions[0]
			if r.Name != name || r.Provider != "Red Hat, Inc." || v.Image != "quay.io/example/runtime:1.0" || v.SupportLevel != "redHatSupported" || v.Version != "v0.21.0" {
				t.Fatalf("unexpected mapping: %+v / %+v", r, v)
			}
			if len(v.DefaultArgs) != 3 || v.Env[0].Name != "HF_HOME" || *v.Env[0].DefaultValue != "/tmp/hf_home" {
				t.Fatalf("container mapping lost: %+v", v)
			}
			if r.SupportedModelFormats[0].AutoSelect != (variant != "gaudi") {
				t.Fatal("autoSelect changed")
			}
			if variant == "spyre-s390x" && len(v.Env) != 8 {
				t.Fatal("lost Spyre environment")
			}
		})
	}
}

func TestRuntimeTemplateErrors(t *testing.T) {
	data, err := os.ReadFile("../../input/serving_runtimes/redhat/vllm-cpu.yaml")
	if err != nil {
		t.Fatal(err)
	}
	base := string(data)
	entry := types.ServingRuntimeIndexEntry{Image: "quay.io/example/runtime:1.0"}
	cases := []struct {
		name, input, want string
		entry             types.ServingRuntimeIndexEntry
	}{
		{"unresolved image", base, "unresolved image", types.ServingRuntimeIndexEntry{}},
		{"wrong kind", strings.Replace(base, "kind: Template", "kind: List", 1), "unsupported runtime input", entry},
		{"wrong object", strings.Replace(base, "kind: ServingRuntime", "kind: Deployment", 1), "object must be", entry},
		{"multiple objects", base + "  - kind: ServingRuntime\n", "exactly one ServingRuntime", entry},
		{"multiple containers", strings.Replace(base, "      containers:\n", "      containers:\n        - image: quay.io/example/sidecar:1\n", 1), "exactly one container", entry},
		{"bad accelerators", strings.Replace(base, "        opendatahub.io/runtime-version:", "        opendatahub.io/recommended-accelerators: bad-json\n        opendatahub.io/runtime-version:", 1), "invalid recommended-accelerators", entry},
		{"valueFrom", strings.Replace(base, "              value: /tmp/hf_home", "              valueFrom:\n                secretKeyRef:\n                  name: secret\n                  key: token", 1), "valueFrom cannot", entry},
		{"envFrom", strings.Replace(base, "          env:\n", "          envFrom:\n            - secretRef:\n                name: secret\n          env:\n", 1), "envFrom cannot", entry},
		{"multiple documents", base + "\n---\nkind: Template\n", "multiple YAML documents", entry},
		{"duplicate keys", base + "kind: Template\n", "already defined", entry},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeServingRuntimeInput([]byte(tc.input), tc.entry)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
	// An override must still obey the existing image policy.
	index := []byte("source: Red Hat Serving Runtimes\nserving_runtimes:\n  - name: vllm-cpu-runtime\n    input_path: runtime.yaml\n    image: quay.io/example/runtime:latest\n")
	if _, err := GenerateServingRuntimeCatalog(index, fstest.MapFS{"runtime.yaml": &fstest.MapFile{Data: data}}); err == nil || !strings.Contains(err.Error(), "latest tag") {
		t.Fatalf("expected latest rejection: %v", err)
	}
}
