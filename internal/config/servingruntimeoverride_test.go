package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServingRuntimeOverrideConfig(t *testing.T) {
	t.Run("empty path returns nil", func(t *testing.T) {
		cfg, err := LoadServingRuntimeOverrideConfig("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg != nil {
			t.Fatal("expected nil config for empty path")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := LoadServingRuntimeOverrideConfig("/nonexistent/file.yaml")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "bad.yaml")
		if err := os.WriteFile(path, []byte(":::not yaml"), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadServingRuntimeOverrideConfig(path)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("missing required fields returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "incomplete.yaml")
		content := []byte("preview_image: \"registry.example.com/image:tag\"\n")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadServingRuntimeOverrideConfig(path)
		if err == nil {
			t.Fatal("expected validation error for missing fields")
		}
	})

	t.Run("valid config loads successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "valid.yaml")
		content := []byte(`---
preview_image: "registry.example.com/vllm:preview"
reason: "Requires transformers v5"
runtime_name: "test-runtime"
display_name: "Test Preview Runtime"
note: "This model requires a Preview serving runtime"
`)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := LoadServingRuntimeOverrideConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if cfg.PreviewImage != "registry.example.com/vllm:preview" {
			t.Errorf("PreviewImage = %q, want %q", cfg.PreviewImage, "registry.example.com/vllm:preview")
		}
		if cfg.Reason != "Requires transformers v5" {
			t.Errorf("Reason = %q, want %q", cfg.Reason, "Requires transformers v5")
		}
		if cfg.RuntimeName != "test-runtime" {
			t.Errorf("RuntimeName = %q, want %q", cfg.RuntimeName, "test-runtime")
		}
		if cfg.DisplayName != "Test Preview Runtime" {
			t.Errorf("DisplayName = %q, want %q", cfg.DisplayName, "Test Preview Runtime")
		}
		if cfg.Note != "This model requires a Preview serving runtime" {
			t.Errorf("Note = %q, want %q", cfg.Note, "This model requires a Preview serving runtime")
		}
	})
}
