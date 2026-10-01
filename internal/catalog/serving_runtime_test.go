package catalog

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"
)

const validRuntimeIndex = `source: Red Hat Serving Runtimes
serving_runtimes:
  - name: vllm
    input_path: input/serving_runtimes/redhat/vllm.yaml
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
    supportLevel: supported
    protocolVersions: [v2]
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

func TestServingRuntimeValidation(t *testing.T) {
	cases := map[string]string{
		"missing name":        strings.Replace(validRuntimeInput, "name: vllm", "name: ''", 1),
		"missing description": strings.Replace(validRuntimeInput, "description: GPU inference runtime", "description: ''", 1),
		"empty versions":      strings.Split(validRuntimeInput, "versions:\n")[0] + "versions: []\n",
		"missing image":       strings.Replace(validRuntimeInput, "image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0", "image: ''", 1),
		"unqualified image":   strings.Replace(validRuntimeInput, "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0", "vllm:latest", 1),
		"unpinned image":      strings.Replace(validRuntimeInput, ":3.4.0\n    supportLevel", "\n    supportLevel", 1),
		"invalid level":       strings.Replace(validRuntimeInput, "supportLevel: supported", "supportLevel: gold", 1),
		"bad protocol":        strings.Replace(validRuntimeInput, "[v2]", "[v3]", 1),
		"bad resource":        strings.Replace(validRuntimeInput, "memory: 16Gi", "memory: broken", 1),
		"unsafe secret":       strings.Replace(validRuntimeInput, "secret: true", "secret: true\n        defaultValue: password", 1),
		"secret by name":      strings.Replace(validRuntimeInput, "secret: true", "defaultValue: password", 1),
		"unknown field":       strings.Replace(validRuntimeInput, "provider: Red Hat", "provider: Red Hat\nsurprise: true", 1),
		"duplicate version":   strings.Replace(validRuntimeInput, "  - version: \"3.4.0\"", "  - version: \"3.4.0\"\n    image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0\n    supportLevel: supported\n  - version: \"3.4.0\"", 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := GenerateServingRuntimeCatalog([]byte(validRuntimeIndex), runtimeFiles(input)); err == nil {
				t.Fatal("expected validation error")
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
