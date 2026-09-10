package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestExplicitCustomPropertiesOverrideGenerated(t *testing.T) {
	props := map[string]types.MetadataValue{
		"featured":     {MetadataType: "MetadataBoolValue", BoolValue: false},
		"model_type":   createMetadataValue("predictive"),
		"hardware_tag": createMetadataValue("custom-hardware"),
		"validated_on": createMetadataValue("custom-validation"),
	}
	model := types.ExtractedMetadata{
		Name:             stringPtr("Example"),
		Tags:             []string{"featured", "other"},
		HardwareTag:      []string{"generated-hardware"},
		ValidatedOn:      []string{"generated-validation"},
		CustomProperties: props,
	}
	result := convertExtractedToCatalogMetadata(model)
	for key, want := range props {
		if got := result.CustomProperties[key]; got != want {
			t.Errorf("%s: got %#v, want %#v", key, got, want)
		}
	}
	if got, ok := result.CustomProperties["other"]; !ok || got != createMetadataValue("") {
		t.Errorf("existing label behavior changed: %#v", got)
	}
}

func TestExplicitCustomPropertiesSurviveDeduplication(t *testing.T) {
	for _, refs := range [][]string{{"generated", "explicit", "conflict"}, {"conflict", "generated", "explicit"}} {
		t.Run(refs[0], func(t *testing.T) {
			dir := t.TempDir()
			models := map[string]types.ExtractedMetadata{
				"generated": {Name: stringPtr("Example"), Tags: []string{"featured"}},
				"explicit": {Name: stringPtr("EXAMPLE"), CustomProperties: map[string]types.MetadataValue{
					"featured":     {MetadataType: "MetadataBoolValue", BoolValue: false},
					"model_type":   createMetadataValue("predictive"),
					"hardware_tag": createMetadataValue("custom"),
					"validated_on": createMetadataValue("custom"),
				}},
				"conflict": {Name: stringPtr("Example"), CustomProperties: map[string]types.MetadataValue{
					"featured": {MetadataType: "MetadataBoolValue", BoolValue: true},
				}},
			}
			for ref, model := range models {
				model.HardwareTag = []string{"generated"}
				model.ValidatedOn = []string{"generated"}
				path := filepath.Join(dir, ref, "models")
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
				data, err := yaml.Marshal(model)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "metadata.yaml"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, "catalog.yaml")
			if err := CreateModelsCatalogWithStaticFromResults(dir, path, refs, nil); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var result types.ModelsCatalog
			if err := yaml.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Models) != 1 {
				t.Fatalf("expected deduplicated model, got %d", len(result.Models))
			}
			want := models["explicit"].CustomProperties
			if refs[0] == "conflict" {
				want["featured"] = models["conflict"].CustomProperties["featured"]
			}
			for key, value := range want {
				if got := result.Models[0].CustomProperties[key]; got != value {
					t.Errorf("%s: got %#v, want %#v", key, got, value)
				}
			}
		})
	}
}
