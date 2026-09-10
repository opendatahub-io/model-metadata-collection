package types

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMetadataValueRoundTrip(t *testing.T) {
	for _, input := range []string{
		`{metadataType: MetadataStringValue, string_value: ""}`,
		`{metadataType: MetadataStringValue, string_value: "true"}`,
		`{metadataType: MetadataBoolValue, bool_value: false}`,
		`{metadataType: MetadataBoolValue, bool_value: true}`,
		`{metadataType: MetadataIntValue, int_value: "0"}`,
		`{metadataType: MetadataIntValue, int_value: "-9223372036854775808"}`,
		`{metadataType: MetadataIntValue, int_value: "9223372036854775807"}`,
		`{metadataType: MetadataDoubleValue, double_value: 0.0}`,
		`{metadataType: MetadataDoubleValue, double_value: 0.95}`,
	} {
		t.Run(input, func(t *testing.T) {
			var value MetadataValue
			if err := yaml.Unmarshal([]byte(input), &value); err != nil {
				t.Fatal(err)
			}
			output, err := yaml.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var roundTrip MetadataValue
			if err := yaml.Unmarshal(output, &roundTrip); err != nil {
				t.Fatal(err)
			}
			if roundTrip != value {
				t.Fatalf("round trip changed value: %#v -> %#v", value, roundTrip)
			}
			var fields map[string]any
			if err := yaml.Unmarshal(output, &fields); err != nil {
				t.Fatal(err)
			}
			if len(fields) != 2 {
				t.Fatalf("expected only discriminator and active value, got %s", output)
			}
			switch value.MetadataType {
			case "MetadataStringValue":
				if fields["string_value"] != value.StringValue {
					t.Fatalf("string value was lost or coerced: %s", output)
				}
			case "MetadataBoolValue":
				if fields["bool_value"] != value.BoolValue {
					t.Fatalf("boolean was lost or coerced: %s", output)
				}
			case "MetadataIntValue":
				if fields["int_value"] != value.IntValue {
					t.Fatalf("integer must remain a string: %s", output)
				}
			case "MetadataDoubleValue":
				// YAML permits an integral double to be emitted as an integer.
				if fields["double_value"] != value.DoubleValue && !(value.DoubleValue == 0 && fields["double_value"] == 0) {
					t.Fatalf("double was lost or coerced: %s", output)
				}
			}
		})
	}
}
