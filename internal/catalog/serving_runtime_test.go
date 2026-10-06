package catalog

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

const validRuntimeIndex = `source: Red Hat Serving Runtimes
serving_runtimes:
  - name: vllm
    input_path: input/serving_runtimes/redhat/vllm.yaml
`

const validServingRuntimeTemplateInput = `    servingRuntimeTemplate:
      apiVersion: serving.kserve.io/v1alpha1
      kind: ServingRuntime
      metadata:
        name: vllm-runtime
      spec:
        containers:
          - name: kserve-container
            image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0
`

const validLLMInferenceServiceTemplateInput = `    llmInferenceServiceTemplate:
      apiVersion: serving.kserve.io/v1alpha1
      kind: LLMInferenceServiceConfig
      metadata:
        name: vllm-config
      spec:
        template:
          containers:
            - name: main
              image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0
`

const validRuntimeInput = `name: vllm
displayName: vLLM
provider: Red Hat
description: GPU inference runtime
supportedModelFormats:
  - name: safetensors
capabilities:
  requiresGPU: true
  supportedAccelerators: [nvidia.com/gpu]
versions:
  - version: "3.4.0"
    image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0
    minimumRHOAIVersion: "3.0"
    supportLevel: supported
` + validServingRuntimeTemplateInput + validLLMInferenceServiceTemplateInput + `    protocolVersions: [v2]
    recommendedResources:
      recommended:
        cpu: "4"
        memory: 16Gi
        accelerator:
          nvidia.com/gpu: "1"
    defaultArgs: ["--max-model-len", "4096"]
    env:
      - name: HF_TOKEN
        secret: true
`

func runtimeFiles(input string) fstest.MapFS {
	return fstest.MapFS{
		"input/serving_runtimes/redhat/vllm.yaml": &fstest.MapFile{Data: []byte(input)},
	}
}

func TestGenerateServingRuntimeCatalog(t *testing.T) {
	files := runtimeFiles(validRuntimeInput)
	first, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), files)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), files)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("generation not deterministic: %v", err)
	}
	if !bytes.Contains(first, []byte("serving_runtimes:")) || !bytes.Contains(first, []byte("supportLevel: supported")) || bytes.Contains(first, []byte("input_path")) {
		t.Fatalf("unexpected loader output: %s", first)
	}
	files["input/serving_runtimes/redhat/other.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(validRuntimeInput, "name: vllm", "name: other", 1))}
	index := strings.Replace(validRuntimeIndex, "serving_runtimes:\n", "serving_runtimes:\n  - name: other\n    input_path: input/serving_runtimes/redhat/other.yaml\n", 1)
	sorted, err := GenerateServingRuntimeCatalog([]byte(index), files)
	if err != nil || bytes.Index(sorted, []byte("name: other")) > bytes.Index(sorted, []byte("name: vllm")) {
		t.Fatalf("runtimes not sorted: %v", err)
	}
}

func TestGenerateServingRuntimeCatalogPreservesOptionalLoaderFields(t *testing.T) {
	input := `name: vllm
displayName: vLLM
provider: Red Hat
description: GPU inference runtime
readme: "# vLLM"
logo: https://example.com/logo.svg
tags: [llm, gpu]
license: apache-2.0
licenseLink: https://example.com/license
documentationUrl: https://example.com/docs
repositoryUrl: https://example.com/repository
supportedModelFormats:
  - name: safetensors
    version: "1"
    autoSelect: true
    priority: 10
capabilities:
  requiresGPU: true
  supportedAccelerators: [nvidia.com/gpu]
  multiModel: false
publishedDate: "2026-01-01T00:00:00Z"
lastUpdated: "2026-02-01T00:00:00Z"
externalId: runtime-1
customProperties:
  owner: example
versions:
  - version: "3.4.0"
    image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0
    minimumRHOAIVersion: "3.0"
    supportLevel: supported
    supportedModelFormats:
      - name: safetensors
        version: "1"
        autoSelect: true
        priority: 10
    protocolVersions: [v2]
    recommendedResources:
      minimal:
        cpu: "1"
        memory: 2Gi
      recommended:
        cpu: "4"
        memory: 16Gi
        accelerator:
          nvidia.com/gpu: "1"
      high:
        cpu: "8"
        memory: 32Gi
    defaultArgs: ["--max-model-len", "4096"]
    env:
      - name: LOG_LEVEL
        description: Logging verbosity
        required: false
        defaultValue: INFO
        secret: false
    servingRuntimeTemplate:
      apiVersion: serving.kserve.io/v1alpha1
      kind: ServingRuntime
      metadata:
        name: vllm-runtime
      spec:
        containers:
          - name: kserve-container
            image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0
    llmInferenceServiceTemplate:
      apiVersion: serving.kserve.io/v1alpha1
      kind: LLMInferenceServiceConfig
      metadata:
        name: vllm-config
      spec:
        template:
          containers:
            - name: main
              image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0
    deprecated: true
    publishedDate: "2026-02-01T00:00:00Z"
    externalId: version-1
`
	output, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		"readme:", "logo:", "tags:", "license:", "licenseLink:",
		"documentationUrl:", "repositoryUrl:", "autoSelect:", "priority:",
		"capabilities:", "publishedDate:", "lastUpdated:", "externalId:",
		"customProperties:", "protocolVersions:", "recommendedResources:",
		"minimal:", "recommended:", "high:", "defaultArgs:", "env:",
		"defaultValue:", "servingRuntimeTemplate:", "llmInferenceServiceTemplate:", "deprecated:",
		"minimumRHOAIVersion: \"3.0\"",
	} {
		if !bytes.Contains(output, []byte(field)) {
			t.Errorf("generated catalog lost %s: %s", field, output)
		}
	}
	for _, field := range []string{"publishedDate:", "externalId:", "supportedModelFormats:"} {
		if strings.Count(string(output), field) != 2 {
			t.Errorf("expected runtime and version %s in output: %s", field, output)
		}
	}
	var generated types.ServingRuntimeCatalog
	if err := yaml.Unmarshal(output, &generated); err != nil {
		t.Fatal(err)
	}
	version := generated.ServingRuntimes[0].Versions[0]
	if got, want := version.ServingRuntimeTemplate, `{"apiVersion":"serving.kserve.io/v1alpha1","kind":"ServingRuntime","metadata":{"name":"vllm-runtime"},"spec":{"containers":[{"image":"registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0","name":"kserve-container"}]}}`; got != want {
		t.Errorf("servingRuntimeTemplate changed during generation: got %q, want %q", got, want)
	}
	if got, want := version.LLMInferenceServiceTemplate, `{"apiVersion":"serving.kserve.io/v1alpha1","kind":"LLMInferenceServiceConfig","metadata":{"name":"vllm-config"},"spec":{"template":{"containers":[{"image":"registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0","name":"main"}]}}}`; got != want {
		t.Errorf("llmInferenceServiceTemplate changed during generation: got %q, want %q", got, want)
	}
}

func TestServingRuntimeValidation(t *testing.T) {
	cases := map[string]string{
		"missing name":             strings.Replace(validRuntimeInput, "name: vllm", "name: ''", 1),
		"missing description":      strings.Replace(validRuntimeInput, "description: GPU inference runtime", "description: ''", 1),
		"empty versions":           strings.Split(validRuntimeInput, "versions:\n")[0] + "versions: []\n",
		"missing image":            strings.Replace(validRuntimeInput, "image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0", "image: ''", 1),
		"unqualified image":        strings.Replace(validRuntimeInput, "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0", "vllm:latest", 1),
		"unpinned image":           strings.Replace(validRuntimeInput, "image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0", "image: registry.redhat.io/rhaii/vllm-cuda-rhel9", 1),
		"invalid level":            strings.Replace(validRuntimeInput, "supportLevel: supported", "supportLevel: gold", 1),
		"bad protocol":             strings.Replace(validRuntimeInput, "[v2]", "[v3]", 1),
		"bad resource":             strings.Replace(validRuntimeInput, "memory: 16Gi", "memory: broken", 1),
		"unsafe secret":            strings.Replace(validRuntimeInput, "secret: true", "secret: true\n        defaultValue: password", 1),
		"secret by name":           strings.Replace(validRuntimeInput, "secret: true", "defaultValue: password", 1),
		"unknown field":            strings.Replace(validRuntimeInput, "provider: Red Hat", "provider: Red Hat\nsurprise: true", 1),
		"missing serving template": strings.Replace(validRuntimeInput, validServingRuntimeTemplateInput, "", 1),
		"missing llm template":     strings.Replace(validRuntimeInput, validLLMInferenceServiceTemplateInput, "", 1),
		"empty serving template":   strings.Replace(validRuntimeInput, validServingRuntimeTemplateInput, "    servingRuntimeTemplate: {}\n", 1),
		"empty llm template":       strings.Replace(validRuntimeInput, validLLMInferenceServiceTemplateInput, "    llmInferenceServiceTemplate: {}\n", 1),
		"openshift template":       strings.Replace(validRuntimeInput, "      kind: ServingRuntime", "      kind: Template", 1),
		"missing serving spec":     strings.Replace(validRuntimeInput, "      spec:\n        containers:", "      other:\n        containers:", 1),
		"string template":          strings.Replace(validRuntimeInput, validServingRuntimeTemplateInput, "    servingRuntimeTemplate: '{}'\n", 1),
		"string llm template":      strings.Replace(validRuntimeInput, validLLMInferenceServiceTemplateInput, "    llmInferenceServiceTemplate: '{}'\n", 1),
		"duplicate version":        strings.Replace(validRuntimeInput, "        secret: true\n", "        secret: true\n  - version: \"3.4.0\"\n", 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(input)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestServingRuntimeImageReferences(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	cases := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{name: "specific tag", image: "quay.io/org/runtime:1.2.3"},
		{name: "digest only", image: "quay.io/org/runtime@" + digest},
		{name: "specific tag and digest", image: "quay.io/org/runtime:1.2.3@" + digest},
		{name: "latest tag", image: "quay.io/org/runtime:latest", wantErr: true},
		{name: "latest tag and digest", image: "quay.io/org/runtime:latest@" + digest, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(validRuntimeInput, "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0", tc.image, 1)
			_, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(input))
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "must not use the latest tag") {
					t.Fatalf("expected latest tag rejection, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("expected valid image reference, got %v", err)
			}
		})
	}
}

func TestServingRuntimeIndexValidation(t *testing.T) {
	cases := map[string]string{
		"empty stub":        "source: \"\"\nserving_runtimes:\n  - name: \"\"\n    input_path: \"\"\n",
		"duplicate runtime": strings.Replace(validRuntimeIndex, "serving_runtimes:\n", "serving_runtimes:\n  - name: vllm\n    input_path: input/serving_runtimes/redhat/vllm.yaml\n", 1),
		"missing path":      strings.Replace(validRuntimeIndex, "input_path: input/serving_runtimes/redhat/vllm.yaml", "input_path: ''", 1),
		"traversal path":    strings.Replace(validRuntimeIndex, "input/serving_runtimes/redhat/vllm.yaml", "../outside.yaml", 1),
		"absolute path":     strings.Replace(validRuntimeIndex, "input/serving_runtimes/redhat/vllm.yaml", "/tmp/runtime.yaml", 1),
		"unknown path":      strings.Replace(validRuntimeIndex, "input/serving_runtimes/redhat/vllm.yaml", "input/missing.yaml", 1),
		"unknown field":     strings.Replace(validRuntimeIndex, "source: Red Hat Serving Runtimes", "source: Red Hat Serving Runtimes\nunknown: true", 1),
	}
	for name, index := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := GenerateServingRuntimeCatalog([]byte(index), runtimeFiles(validRuntimeInput)); err == nil {
				t.Fatal("expected index error")
			}
		})
	}
	if _, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(strings.Replace(validRuntimeInput, "name: vllm", "name: other", 1))); err == nil {
		t.Fatal("expected name mismatch error")
	}
}
