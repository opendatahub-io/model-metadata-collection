package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadModelCustomProperties(t *testing.T) {
	for _, tc := range []struct {
		name, property string
		valid          bool
	}{
		{"empty string", `{metadataType: MetadataStringValue, string_value: ""}`, true},
		{"string", `{metadataType: MetadataStringValue, string_value: "team-a"}`, true},
		{"false", `{metadataType: MetadataBoolValue, bool_value: false}`, true},
		{"true", `{metadataType: MetadataBoolValue, bool_value: true}`, true},
		{"integer", `{metadataType: MetadataIntValue, int_value: "0"}`, true},
		{"minimum integer", `{metadataType: MetadataIntValue, int_value: "-9223372036854775808"}`, true},
		{"maximum integer", `{metadataType: MetadataIntValue, int_value: "9223372036854775807"}`, true},
		{"double", `{metadataType: MetadataDoubleValue, double_value: 0.95}`, true},
		{"zero double", `{metadataType: MetadataDoubleValue, double_value: 0}`, true},
		{"overflow", `{metadataType: MetadataIntValue, int_value: "9223372036854775808"}`, false},
		{"numeric integer", `{metadataType: MetadataIntValue, int_value: 10}`, false},
		{"fractional integer", `{metadataType: MetadataIntValue, int_value: "1.5"}`, false},
		{"nan", `{metadataType: MetadataDoubleValue, double_value: .nan}`, false},
		{"infinity", `{metadataType: MetadataDoubleValue, double_value: .inf}`, false},
		{"quoted boolean", `{metadataType: MetadataBoolValue, bool_value: "false"}`, false},
		{"boolean as string", `{metadataType: MetadataStringValue, string_value: false}`, false},
		{"missing type", `{bool_value: false}`, false},
		{"unsupported type", `{metadataType: MetadataStructValue, struct_value: ""}`, false},
		{"wrong field", `{metadataType: MetadataBoolValue, string_value: "false"}`, false},
		{"missing value", `{metadataType: MetadataBoolValue}`, false},
		{"null value", `{metadataType: MetadataBoolValue, bool_value: null}`, false},
		{"extra field", `{metadataType: MetadataBoolValue, bool_value: false, string_value: ""}`, false},
		{"null property", `null`, false},
		{"shorthand", `false`, false},
		{"duplicate field", `{metadataType: MetadataBoolValue, bool_value: false, bool_value: true}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "index.yaml")
			input := "models:\n  - type: oci\n    uri: test-model\n    customProperties:\n      example: " + tc.property + "\n"
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			entries, err := LoadModelsConfigFromYAML(path)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 1 || len(entries[0].CustomProperties) != 1 {
					t.Fatalf("properties not loaded: %#v", entries)
				}
			} else if err == nil || !strings.Contains(err.Error(), "test-model") || !strings.Contains(err.Error(), "example") {
				t.Fatalf("expected error identifying model and property, got %v", err)
			}
		})
	}
}
