package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadVLLMConfigs(t *testing.T) {
	t.Run("valid config files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Write a valid config file
		validYAML := `---
model:
  name: RedHatAI/gpt-oss-120b
presets:
  - mode: online-serving
    optimizations:
      - optimization: low-latency
        hardware: H200
        description: "Low latency for online serving"
        cli-args:
          - --tensor-parallel-size 1
          - --max-num-batched-tokens 8192
`
		if err := os.WriteFile(filepath.Join(tmpDir, "gpt-oss-120b.yaml"), []byte(validYAML), 0644); err != nil {
			t.Fatal(err)
		}

		index, err := LoadVLLMConfigs(tmpDir)
		if err != nil {
			t.Fatalf("LoadVLLMConfigs() error = %v", err)
		}

		if index.ModelCount() != 1 {
			t.Errorf("ModelCount() = %d, want 1", index.ModelCount())
		}

		cfg := index.GetConfig("RedHatAI/gpt-oss-120b")
		if cfg == nil {
			t.Fatal("GetConfig() returned nil for RedHatAI/gpt-oss-120b")
		}
		if cfg.Model.Name != "RedHatAI/gpt-oss-120b" {
			t.Errorf("Model.Name = %q, want %q", cfg.Model.Name, "RedHatAI/gpt-oss-120b")
		}
		if len(cfg.Presets) != 1 {
			t.Errorf("len(Presets) = %d, want 1", len(cfg.Presets))
		}
		if cfg.Presets[0].Mode != "online-serving" {
			t.Errorf("Presets[0].Mode = %q, want %q", cfg.Presets[0].Mode, "online-serving")
		}
	})

	t.Run("multiple config files", func(t *testing.T) {
		tmpDir := t.TempDir()

		yaml1 := `---
model:
  name: RedHatAI/model-a
presets:
  - mode: online-serving
    optimizations:
      - optimization: low-latency
        hardware: H200
        cli-args:
          - --tensor-parallel-size 1
`
		yaml2 := `---
model:
  name: RedHatAI/model-b
presets:
  - mode: offline-serving
    optimizations:
      - optimization: high-throughput
        hardware: H200
        cli-args:
          - --tensor-parallel-size 1
`
		os.WriteFile(filepath.Join(tmpDir, "model-a.yaml"), []byte(yaml1), 0644)
		os.WriteFile(filepath.Join(tmpDir, "model-b.yaml"), []byte(yaml2), 0644)

		index, err := LoadVLLMConfigs(tmpDir)
		if err != nil {
			t.Fatalf("LoadVLLMConfigs() error = %v", err)
		}
		if index.ModelCount() != 2 {
			t.Errorf("ModelCount() = %d, want 2", index.ModelCount())
		}
		if index.GetConfig("RedHatAI/model-a") == nil {
			t.Error("GetConfig('RedHatAI/model-a') returned nil")
		}
		if index.GetConfig("RedHatAI/model-b") == nil {
			t.Error("GetConfig('RedHatAI/model-b') returned nil")
		}
	})

	t.Run("invalid YAML is skipped", func(t *testing.T) {
		tmpDir := t.TempDir()

		os.WriteFile(filepath.Join(tmpDir, "bad.yaml"), []byte("not: [valid: yaml: {{"), 0644)

		index, err := LoadVLLMConfigs(tmpDir)
		if err != nil {
			t.Fatalf("LoadVLLMConfigs() error = %v", err)
		}
		if index.ModelCount() != 0 {
			t.Errorf("ModelCount() = %d, want 0", index.ModelCount())
		}
	})

	t.Run("invalid config is skipped", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Missing model name - should fail validation
		invalidYAML := `---
model:
  name: ""
presets:
  - mode: online-serving
    optimizations:
      - optimization: low-latency
        hardware: H200
`
		os.WriteFile(filepath.Join(tmpDir, "invalid.yaml"), []byte(invalidYAML), 0644)

		index, err := LoadVLLMConfigs(tmpDir)
		if err != nil {
			t.Fatalf("LoadVLLMConfigs() error = %v", err)
		}
		if index.ModelCount() != 0 {
			t.Errorf("ModelCount() = %d, want 0", index.ModelCount())
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		index, err := LoadVLLMConfigs(tmpDir)
		if err != nil {
			t.Fatalf("LoadVLLMConfigs() error = %v", err)
		}
		if index.ModelCount() != 0 {
			t.Errorf("ModelCount() = %d, want 0", index.ModelCount())
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		index, err := LoadVLLMConfigs("/nonexistent/path")
		if err != nil {
			t.Fatalf("LoadVLLMConfigs() error = %v", err)
		}
		if index.ModelCount() != 0 {
			t.Errorf("ModelCount() = %d, want 0", index.ModelCount())
		}
	})
}

func TestVLLMConfigIndex_GetConfig(t *testing.T) {
	llamaYAML := `---
model:
  name: RedHatAI/Llama-3.3-70B-Instruct-FP8-dynamic
presets:
  - mode: online-serving
    optimizations:
      - optimization: low-latency
        hardware: H200
        cli-args:
          - --tensor-parallel-size 1
`

	t.Run("exact match hit", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "llama.yaml"), []byte(llamaYAML), 0644)

		index, _ := LoadVLLMConfigs(tmpDir)
		cfg := index.GetConfig("RedHatAI/Llama-3.3-70B-Instruct-FP8-dynamic")
		if cfg == nil {
			t.Fatal("GetConfig() returned nil for exact match")
		}
	})

	t.Run("exact match miss", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "llama.yaml"), []byte(llamaYAML), 0644)

		index, _ := LoadVLLMConfigs(tmpDir)
		cfg := index.GetConfig("RedHatAI/Llama-3.3-70B-Instruct")
		if cfg != nil {
			t.Error("GetConfig() should return nil for non-exact match")
		}
	})

	t.Run("nil index", func(t *testing.T) {
		var index *VLLMConfigIndex
		cfg := index.GetConfig("anything")
		if cfg != nil {
			t.Error("GetConfig() on nil index should return nil")
		}
	})
}
