package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

func TestLoadExistingMetadataPreservesCustomProperties(t *testing.T) {
	for _, artifacts := range []string{
		"artifacts: []\n",
		"artifacts: [legacy-artifact]\n",
		"createTimeSinceEpoch: ''\nartifacts: []\n",
	} {
		t.Run(artifacts, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test-model", "models")
			if err := os.MkdirAll(path, 0700); err != nil {
				t.Fatal(err)
			}
			data := artifacts + "customProperties:\n  enabled:\n    metadataType: MetadataBoolValue\n    bool_value: false\n"
			if err := os.WriteFile(filepath.Join(path, "metadata.yaml"), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := LoadExistingMetadata("test-model", dir)
			if err != nil {
				t.Fatal(err)
			}
			want := types.MetadataValue{MetadataType: "MetadataBoolValue", BoolValue: false}
			if got, ok := result.CustomProperties["enabled"]; !ok || got != want {
				t.Fatalf("property lost in migration: %#v", result.CustomProperties)
			}
		})
	}
}
