package catalog

import (
	"strings"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestInferServingRuntimeSupport(t *testing.T) {
	ga := "registry.redhat.io/rhoai/odh-kserve-llmisvc-controller-rhel9:v3.6.0-1788535633"
	ea := "registry.redhat.io/rhoai/odh-kserve-llmisvc-controller-rhel9:v3.6.0-ea.1-1788535633"
	runtime := "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6.0"
	fast := "registry.redhat.io/rhaii-fast/vllm-cuda-rhel9:3.6.0-fast.1"
	digest := "@sha256:" + strings.Repeat("a", 64)
	for _, test := range []struct {
		name   string
		target string
		images []string
		want   string
	}{
		{"GA target and GA runtime", ga, []string{runtime}, "supported"},
		{"EA target and GA runtime", ea, []string{runtime}, "techPreview"},
		{"GA target and fast runtime", ga, []string{fast}, "techPreview"},
		{"EA target and fast runtime", ea, []string{fast}, "techPreview"},
		{"fast worker downgrades GA main", ga, []string{runtime, fast}, "techPreview"},
		{"digest pinned GA runtime", ga, []string{"registry.redhat.io/rhaii/vllm-cuda-rhel9" + digest}, "supported"},
		{"digest pinned fast runtime", ga, []string{"registry.redhat.io/rhaii-fast/vllm-cuda-rhel9" + digest}, "techPreview"},
		{"runtime EA tag in GA namespace", ga, []string{runtime + "-ea.1"}, "techPreview"},
		{"compact EA target tag", strings.Replace(ea, "ea.1", "ea1", 1), []string{runtime}, "techPreview"},
		{"unrelated EA substring", ga + "-seaside", []string{runtime}, "supported"},
		{"previous stable namespace", ga, []string{strings.Replace(runtime, "/rhaii/", "/rhaiis/", 1)}, "supported"},
		{"access registry", ga, []string{strings.Replace(runtime, "registry.redhat.io", "registry.access.redhat.com", 1)}, "supported"},
		{"unknown runtime", ga, []string{"quay.io/vllm/vllm:0.21.0"}, "techPreview"},
		{"similar namespace", ga, []string{strings.Replace(runtime, "/rhaii/", "/rhaii-other/", 1)}, "techPreview"},
		{"unknown registry", ga, []string{strings.Replace(runtime, "registry.redhat.io", "registry.example.com", 1)}, "techPreview"},
		{"directory has no target", "", []string{runtime}, "techPreview"},
		{"digest-only target has no release tag", "registry.redhat.io/rhoai/controller" + digest, []string{runtime}, "techPreview"},
		{"tag and digest preserve EA", ea + digest, []string{runtime}, "techPreview"},
		{"no runtime images", ga, nil, "techPreview"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := inferServingRuntimeSupport(test.target, test.images); got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestGenerateServingRuntimeSupportUsesRenderedTemplateImages(t *testing.T) {
	for _, test := range []struct {
		name   string
		worker string
		want   string
	}{
		{"unused fast parameter does not downgrade", "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6.0", "supported"},
		{"rendered fast worker downgrades", "registry.redhat.io/rhaii-fast/vllm-cuda-rhel9:3.6.0-fast.1", "techPreview"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config, files := discoveredSources()
			source := &config.Sources[0]
			source.Directory = ""
			source.TargetImage = "registry.redhat.io/rhoai/controller:v3.6.0-123"
			source.Include = []string{"config/runtimes/config.yaml"}
			source.Replacements["placeholder"] = "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6.0"
			source.Replacements["registry.redhat.io/rhoai/helper:2.0"] = test.worker
			source.Parameters = map[string]string{"unused-fast-image": "registry.redhat.io/rhaii-fast/vllm-cuda-rhel9:3.6.0-fast.1"}
			artifacts, err := GenerateServingRuntimeArtifacts(&config, files, "generated")
			if err != nil {
				t.Fatal(err)
			}
			var catalog types.ServingRuntimeCatalog
			if err := yaml.Unmarshal(artifacts.Catalog, &catalog); err != nil {
				t.Fatal(err)
			}
			if got := catalog.ServingRuntimes[0].Versions[0].SupportLevel; got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}
