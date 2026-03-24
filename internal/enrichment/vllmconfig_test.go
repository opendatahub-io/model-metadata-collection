package enrichment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/internal/metadata"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

// newNullEnriched creates an EnrichedModelMetadata with all metadata sources set to "null"
// and the given registry/HuggingFace model names pre-populated.
func newNullEnriched(registryModel, hfModel string) *types.EnrichedModelMetadata {
	null := types.MetadataSource{Source: "null"}
	return &types.EnrichedModelMetadata{
		RegistryModel:        registryModel,
		HuggingFaceModel:     hfModel,
		Name:                 null,
		Provider:             null,
		Description:          null,
		License:              null,
		LicenseLink:          null,
		Language:             null,
		Tags:                 null,
		Tasks:                null,
		LastModified:         null,
		CreateTimeSinceEpoch: null,
		Downloads:            null,
		Likes:                null,
		ModelSize:            null,
		ValidatedOn:          null,
	}
}

func setupTestDir(t *testing.T, sanitizedName, modelcardContent string) string {
	t.Helper()
	tmpDir := t.TempDir()

	modelcardDir := filepath.Join(tmpDir, sanitizedName, "models")
	if err := os.MkdirAll(modelcardDir, 0755); err != nil {
		t.Fatalf("Failed to create modelcard dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(modelcardDir, "modelcard.md"), []byte(modelcardContent), 0644); err != nil {
		t.Fatalf("Failed to write modelcard: %v", err)
	}

	initialMetadata := types.ExtractedMetadata{
		Name: func() *string { s := "Test Model"; return &s }(),
	}
	metadataBytes, _ := yaml.Marshal(initialMetadata)
	if err := os.WriteFile(filepath.Join(modelcardDir, "metadata.yaml"), metadataBytes, 0644); err != nil {
		t.Fatalf("Failed to write initial metadata: %v", err)
	}

	return tmpDir
}

func TestVLLMConfigIntegration_WithConfig(t *testing.T) {
	registryModel := "registry.redhat.io/rhai/modelcar-llama-3-3-70b-fp8:1.0"
	sanitizedName := "registry.redhat.io_rhai_modelcar-llama-3-3-70b-fp8_1.0"

	tmpDir := setupTestDir(t, sanitizedName,
		"# Llama 3.3 70B\n\nThis is a test model.")

	enrichedData := newNullEnriched(registryModel, "RedHatAI/Llama-3.3-70B-Instruct-FP8-dynamic")
	enrichedData.VLLMConfig = &types.VLLMRecommendedConfig{
		Model: types.VLLMModelRef{Name: "RedHatAI/Llama-3.3-70B-Instruct-FP8-dynamic"},
		Presets: []types.VLLMPreset{
			{
				Mode: "online-serving",
				Optimizations: []types.VLLMOptimization{
					{
						Optimization: "low-latency",
						Hardware:     "H200",
						Description:  "Low latency for online serving",
						CLIArgs: []string{
							"--tensor-parallel-size 1",
							"--max-num-batched-tokens 1024",
							"--gpu-memory-utilization 0.92",
							"--kv-cache-dtype fp8",
						},
						Recommendations: []string{
							"--async-scheduling is on by default",
						},
					},
				},
			},
			{
				Mode: "offline-serving",
				Optimizations: []types.VLLMOptimization{
					{
						Optimization: "high-throughput",
						Hardware:     "H200",
						Description:  "High throughput for offline serving",
						CLIArgs: []string{
							"--tensor-parallel-size 1",
							"--max-num-batched-tokens 1024",
						},
					},
				},
			},
		},
	}

	err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir)
	if err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	if updatedMetadata.Readme == nil {
		t.Fatal("README is nil after update")
	}

	readme := *updatedMetadata.Readme

	expectedPhrases := []string{
		"## vLLM Recommended Configurations",
		"### Online Serving",
		"#### Low Latency",
		"- Hardware: **H200**",
		"- Description: Low latency for online serving",
		"--tensor-parallel-size 1",
		"--max-num-batched-tokens 1024",
		"--gpu-memory-utilization 0.92",
		"--kv-cache-dtype fp8",
		"- **Recommendations**:",
		"--async-scheduling is on by default",
		"### Offline Serving",
		"#### High Throughput",
		"High throughput for offline serving",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(readme, phrase) {
			t.Errorf("README missing expected phrase: %q", phrase)
		}
	}

	// Verify original content preserved
	if !strings.Contains(readme, "This is a test model") {
		t.Error("Original README content was not preserved")
	}
}

func TestVLLMConfigIntegration_WithoutConfig(t *testing.T) {
	registryModel := "registry.redhat.io/rhai/modelcar-granite-3b:1.0"
	sanitizedName := "registry.redhat.io_rhai_modelcar-granite-3b_1.0"

	tmpDir := setupTestDir(t, sanitizedName,
		"# Granite 3B\n\nThis model has no vLLM config.")

	enrichedData := newNullEnriched(registryModel, "RedHatAI/Granite-3B")

	err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir)
	if err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	if updatedMetadata.Readme != nil {
		readme := *updatedMetadata.Readme
		if strings.Contains(readme, "vLLM Recommended Configurations") {
			t.Error("README should NOT contain vLLM config section when config is nil")
		}
		if !strings.Contains(readme, "This model has no vLLM config") {
			t.Error("Original README content was not preserved")
		}
	}
}

func TestVLLMConfigIntegration_WithConstraintsAndEnvVars(t *testing.T) {
	registryModel := "registry.redhat.io/rhai/modelcar-test-constrained:1.0"
	sanitizedName := "registry.redhat.io_rhai_modelcar-test-constrained_1.0"

	tmpDir := setupTestDir(t, sanitizedName,
		"# Test Model\n\nBase content.")

	enrichedData := newNullEnriched(registryModel, "RedHatAI/Test-Constrained")
	enrichedData.VLLMConfig = &types.VLLMRecommendedConfig{
		Model: types.VLLMModelRef{Name: "RedHatAI/Test-Constrained"},
		Presets: []types.VLLMPreset{
			{
				Mode: "online-serving",
				Optimizations: []types.VLLMOptimization{
					{
						Optimization: "low-latency",
						Hardware:     "H200",
						Description:  "Low latency config",
						CLIArgs:      []string{"--tensor-parallel-size 1"},
						EnvVars: []string{
							"NVIDIA_VISIBLE_DEVICES=1",
							`VLLM_ATTENTION_BACKEND="FLASH_ATTN"`,
						},
						Constraints: []types.VLLMConstraint{
							{Name: "TTFT", Value: "10ms", Operator: "<="},
						},
					},
				},
			},
		},
	}

	err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir)
	if err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	if updatedMetadata.Readme == nil {
		t.Fatal("README is nil after update")
	}

	readme := *updatedMetadata.Readme

	// Verify constraints
	if !strings.Contains(readme, "TTFT less than or equal to 10ms") {
		t.Error("README missing constraint text")
	}

	// Verify env vars
	if !strings.Contains(readme, "NVIDIA_VISIBLE_DEVICES=1") {
		t.Error("README missing env var")
	}
	if !strings.Contains(readme, `VLLM_ATTENTION_BACKEND="FLASH_ATTN"`) {
		t.Error("README missing VLLM_ATTENTION_BACKEND env var")
	}
}

func TestVLLMConfigIntegration_BothToolCallingAndVLLMConfig(t *testing.T) {
	registryModel := "registry.redhat.io/rhai/modelcar-dual-config:1.0"
	sanitizedName := "registry.redhat.io_rhai_modelcar-dual-config_1.0"

	tmpDir := setupTestDir(t, sanitizedName,
		"# Dual Config Model\n\nBase content.")

	enrichedData := newNullEnriched(registryModel, "RedHatAI/Dual-Config-Model")
	enrichedData.ToolCallingConfig = &types.ToolCallingConfig{
		Supported:      true,
		ToolCallParser: "mistral",
	}
	enrichedData.VLLMConfig = &types.VLLMRecommendedConfig{
		Model: types.VLLMModelRef{Name: "RedHatAI/Dual-Config-Model"},
		Presets: []types.VLLMPreset{
			{
				Mode: "online-serving",
				Optimizations: []types.VLLMOptimization{
					{
						Optimization: "low-latency",
						Hardware:     "H200",
						CLIArgs:      []string{"--tensor-parallel-size 1"},
					},
				},
			},
		},
	}

	err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir)
	if err != nil {
		t.Fatalf("UpdateModelMetadataFile() failed: %v", err)
	}

	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	if updatedMetadata.Readme == nil {
		t.Fatal("README is nil after update")
	}

	readme := *updatedMetadata.Readme

	// Both sections should be present
	if !strings.Contains(readme, "vLLM Deployment with Tool Calling") {
		t.Error("README missing tool-calling section")
	}
	if !strings.Contains(readme, "vLLM Recommended Configurations") {
		t.Error("README missing vLLM config section")
	}

	// Tool-calling should appear before vLLM config
	tcIdx := strings.Index(readme, "vLLM Deployment with Tool Calling")
	vcIdx := strings.Index(readme, "vLLM Recommended Configurations")
	if tcIdx >= vcIdx {
		t.Error("Tool-calling section should appear before vLLM config section")
	}
}

func TestVLLMConfigIntegration_IdempotentReEnrichment(t *testing.T) {
	registryModel := "registry.redhat.io/rhai/modelcar-idempotent-test:1.0"
	sanitizedName := "registry.redhat.io_rhai_modelcar-idempotent-test_1.0"

	tmpDir := setupTestDir(t, sanitizedName,
		"# Idempotent Test Model\n\nBase content.")

	enrichedData := newNullEnriched(registryModel, "RedHatAI/Idempotent-Test")
	enrichedData.VLLMConfig = &types.VLLMRecommendedConfig{
		Model: types.VLLMModelRef{Name: "RedHatAI/Idempotent-Test"},
		Presets: []types.VLLMPreset{
			{
				Mode: "online-serving",
				Optimizations: []types.VLLMOptimization{
					{
						Optimization: "low-latency",
						Hardware:     "H200",
						CLIArgs:      []string{"--tensor-parallel-size 1"},
					},
				},
			},
		},
	}
	enrichedData.ToolCallingConfig = &types.ToolCallingConfig{
		Supported:      true,
		ToolCallParser: "mistral",
	}

	// Run enrichment twice
	for i := 0; i < 2; i++ {
		err := UpdateModelMetadataFile(registryModel, enrichedData, tmpDir)
		if err != nil {
			t.Fatalf("UpdateModelMetadataFile() run %d failed: %v", i+1, err)
		}
	}

	updatedMetadata, err := metadata.LoadExistingMetadata(registryModel, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load updated metadata: %v", err)
	}

	if updatedMetadata.Readme == nil {
		t.Fatal("README is nil after update")
	}

	readme := *updatedMetadata.Readme

	// Each section should appear exactly once
	tcCount := strings.Count(readme, "## vLLM Deployment with Tool Calling")
	if tcCount != 1 {
		t.Errorf("Tool-calling section should appear exactly once, found %d times", tcCount)
	}

	vcCount := strings.Count(readme, "## vLLM Recommended Configurations")
	if vcCount != 1 {
		t.Errorf("vLLM config section should appear exactly once, found %d times", vcCount)
	}
}
