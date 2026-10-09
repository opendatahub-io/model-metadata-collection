package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestServingRuntimeCatalogImageNamespaces(t *testing.T) {
	digest := "@sha256:" + strings.Repeat("a", 64)
	for _, namespace := range []string{"rhoai", "rhaii", "rhaii-early-access", "rhaii-fast"} {
		for _, suffix := range []string{":3.6", digest, ":3.6" + digest, ":latest" + digest} {
			image := "registry.redhat.io/" + namespace + "/runtime" + suffix
			t.Run(image, func(t *testing.T) {
				input := strings.ReplaceAll(validRuntimeInput, "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0", image)
				if _, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(input)); err != nil {
					t.Fatalf("approved image rejected: %v", err)
				}
			})
		}
	}
	for _, image := range []string{
		"quay.io/rhoai/runtime:3.6",
		"docker.io/rhaii/runtime:3.6",
		"registry.access.redhat.com/rhoai/runtime:3.6",
		"registry.redhat.io/ubi9/runtime:3.6",
		"registry.redhat.io/rhaiis/runtime:3.6",
		"registry.redhat.io/rhoai-extra/runtime:3.6",
		"registry.redhat.io/rhaii-fast-extra/runtime:3.6",
		"registry.redhat.io/rhaii-early-access-extra/runtime:3.6",
		"registry.redhat.io.evil.example/rhoai/runtime:3.6",
		"evil.example/registry.redhat.io/rhoai/runtime:3.6",
		"registry.redhat.io:443/rhoai/runtime:3.6",
		"registry.redhat.io/rhoai:3.6",
	} {
		t.Run(image, func(t *testing.T) {
			// Only the top-level image is changed; the embedded images stay valid.
			input := strings.Replace(validRuntimeInput, "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0", image, 1)
			_, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(input))
			if err == nil || !strings.Contains(err.Error(), "image: image "+fmt.Sprintf("%q", image)+" must start with one of:") {
				t.Fatalf("expected image namespace rejection, got %v", err)
			}
		})
	}
}

func TestServingRuntimeSchemaValidatesEveryEmbeddedImage(t *testing.T) {
	const approved = `"registry.redhat.io/rhaii/runtime:3.6"`
	const rejected = `"quay.io/unapproved/runtime:3.6"`
	cases := []struct {
		name  string
		field string
		spec  string
		path  string
	}{
		{"runtime main", "servingRuntimeTemplate", `{"containers":[{"image":%s}]}`, "objects[0].spec.containers[0].image"},
		{"runtime sidecar", "servingRuntimeTemplate", `{"containers":[{"image":` + approved + `},{"image":%s}]}`, "objects[0].spec.containers[1].image"},
		{"runtime init", "servingRuntimeTemplate", `{"initContainers":[{"image":%s}]}`, "objects[0].spec.initContainers[0].image"},
		{"runtime worker", "servingRuntimeTemplate", `{"workerSpec":{"containers":[{"image":%s}]}}`, "objects[0].spec.workerSpec.containers[0].image"},
		{"llm main", "llmInferenceServiceConfig", `{"template":{"containers":[{"image":%s}]}}`, "spec.template.containers[0].image"},
		{"llm sidecar", "llmInferenceServiceConfig", `{"template":{"containers":[{"image":` + approved + `},{"image":%s}]}}`, "spec.template.containers[1].image"},
		{"llm init", "llmInferenceServiceConfig", `{"template":{"initContainers":[{"image":%s}]}}`, "spec.template.initContainers[0].image"},
		{"llm worker", "llmInferenceServiceConfig", `{"worker":{"template":{"containers":[{"image":%s}]}}}`, "spec.worker.template.containers[0].image"},
		{"llm prefill", "llmInferenceServiceConfig", `{"prefill":{"template":{"containers":[{"image":%s}]}}}`, "spec.prefill.template.containers[0].image"},
		{"llm prefill init", "llmInferenceServiceConfig", `{"prefill":{"template":{"initContainers":[{"image":%s}]}}}`, "spec.prefill.template.initContainers[0].image"},
	}
	for _, test := range cases {
		for _, image := range []string{approved, rejected, `"registry.redhat.io/rhaii/runtime:latest"`, `"$(runtime-image)"`, `null`, `42`} {
			t.Run(test.name+"/"+image, func(t *testing.T) {
				var catalog types.ServingRuntimeCatalog
				if err := yaml.Unmarshal(schemaCatalogFixture(t), &catalog); err != nil {
					t.Fatal(err)
				}
				version := &catalog.ServingRuntimes[0].Versions[0]
				if test.field == "servingRuntimeTemplate" {
					version.ServingRuntimeTemplate = fmt.Sprintf(`{"apiVersion":"template.openshift.io/v1","kind":"Template","metadata":{"name":"runtime"},"objects":[{"apiVersion":"serving.kserve.io/v1alpha1","kind":"ServingRuntime","metadata":{"name":"runtime"},"spec":%s}]}`, fmt.Sprintf(test.spec, image))
				} else {
					version.LLMInferenceServiceConfig = fmt.Sprintf(`{"apiVersion":"serving.kserve.io/v1alpha2","kind":"LLMInferenceServiceConfig","metadata":{"name":"runtime"},"spec":%s}`, fmt.Sprintf(test.spec, image))
				}
				input, err := json.Marshal(catalog)
				if err != nil {
					t.Fatal(err)
				}
				_, err = ValidateServingRuntimeCatalogSchema(context.Background(), input)
				if image == approved {
					if err != nil {
						t.Fatalf("approved embedded image rejected: %v", err)
					}
				} else if err == nil || !strings.Contains(err.Error(), test.field+"."+test.path) || !strings.Contains(err.Error(), `runtime "vllm" version "3.4.0"`) {
					t.Fatalf("expected runtime/version and embedded image path in error, got %v", err)
				}
			})
		}
	}
}

func TestServingRuntimeTemplateValidatesImagesInAdditionalObjects(t *testing.T) {
	input := strings.Replace(validRuntimeInput, validLLMInferenceServiceConfigInput, `        - apiVersion: v1
          kind: Pod
          metadata: {name: sidecar}
          spec:
            containers:
              - image: quay.io/unapproved/helper:3.6
`+validLLMInferenceServiceConfigInput, 1)
	_, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(input))
	if err == nil || !strings.Contains(err.Error(), "servingRuntimeTemplate.objects[1].spec.containers[0].image") {
		t.Fatalf("unapproved image in an additional Template object was not rejected: %v", err)
	}
}
