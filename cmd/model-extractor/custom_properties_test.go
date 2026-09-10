package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/opendatahub-io/model-metadata-collection/internal/catalog"
	"github.com/opendatahub-io/model-metadata-collection/internal/config"
	"github.com/opendatahub-io/model-metadata-collection/internal/enrichment"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestIndexCustomPropertiesPipeline(t *testing.T) {
	for _, withLabels := range []bool{false, true} {
		for _, enrich := range []bool{false, true} {
			t.Run(fmt.Sprintf("labels=%t/enrich=%t", withLabels, enrich), func(t *testing.T) {
				dir := t.TempDir()
				originalOutput := *outputDir
				*outputDir = dir
				t.Cleanup(func() { *outputDir = originalOutput })
				input := `models:
  - type: oci
    uri: test-model
    customProperties:
      featured:
        metadataType: MetadataBoolValue
        bool_value: false
      owner:
        metadataType: MetadataStringValue
        string_value: team-a
      priority:
        metadataType: MetadataIntValue
        int_value: "0"
      score:
        metadataType: MetadataDoubleValue
        double_value: 0.95
`
				if withLabels {
					input += "    labels: [featured, existing-label]\n"
				}
				indexPath := filepath.Join(dir, "index.yaml")
				if err := os.WriteFile(indexPath, []byte(input), 0600); err != nil {
					t.Fatal(err)
				}
				entries, err := config.LoadModelsConfigFromYAML(indexPath)
				if err != nil {
					t.Fatal(err)
				}
				metadataDir := filepath.Join(dir, "test-model", "models")
				if err := os.MkdirAll(metadataDir, 0700); err != nil {
					t.Fatal(err)
				}
				metadataPath := filepath.Join(metadataDir, "metadata.yaml")
				// A skeleton exercises the same path as a successful extraction.
				if err := os.WriteFile(metadataPath, []byte("name: Test Model\ntags: [upstream]\n"), 0600); err != nil {
					t.Fatal(err)
				}
				addModelLabelTags("test-model", entries[0])
				addModelLabelTags("test-model", entries[0]) // Reapplying is idempotent.
				if enrich {
					data := &types.EnrichedModelMetadata{
						Provider:    types.MetadataSource{Source: "null"},
						License:     types.MetadataSource{Source: "null"},
						LicenseLink: types.MetadataSource{Source: "null"},
						Description: types.MetadataSource{Value: "Enriched description", Source: "huggingface.yaml"},
					}
					if err := enrichment.UpdateModelMetadataFile("test-model", data, dir); err != nil {
						t.Fatal(err)
					}
				}
				bytes, err := os.ReadFile(metadataPath)
				if err != nil {
					t.Fatal(err)
				}
				var metadata types.ExtractedMetadata
				if err := yaml.Unmarshal(bytes, &metadata); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(metadata.CustomProperties, entries[0].CustomProperties) {
					t.Fatalf("intermediate properties changed: %#v", metadata.CustomProperties)
				}
				expectedTags := []string{"upstream"}
				if withLabels {
					expectedTags = append(expectedTags, "featured", "existing-label")
				}
				if !reflect.DeepEqual(metadata.Tags, expectedTags) {
					t.Fatalf("unexpected tags: %v", metadata.Tags)
				}
				catalogPath := filepath.Join(dir, "catalog.yaml")
				if err := catalog.CreateModelsCatalogWithStaticFromResults(dir, catalogPath, []string{"test-model"}, nil); err != nil {
					t.Fatal(err)
				}
				bytes, err = os.ReadFile(catalogPath)
				if err != nil {
					t.Fatal(err)
				}
				var result types.ModelsCatalog
				if err := yaml.Unmarshal(bytes, &result); err != nil {
					t.Fatal(err)
				}
				if len(result.Models) != 1 {
					t.Fatalf("expected one model, got %d", len(result.Models))
				}
				for key, want := range entries[0].CustomProperties {
					if got := result.Models[0].CustomProperties[key]; got != want {
						t.Errorf("%s: got %#v, want %#v", key, got, want)
					}
				}
			})
		}
	}
}
