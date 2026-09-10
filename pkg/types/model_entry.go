package types

import (
	"fmt"
	"math"
	"strconv"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML validates explicit index properties before decoding them. Catalog
// readers retain their existing permissive decoding for older metadata files.
func (entry *ModelEntry) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		URI        string    `yaml:"uri"`
		Properties yaml.Node `yaml:"customProperties"`
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if raw.Properties.Kind != 0 && raw.Properties.Tag != "!!null" {
		var properties map[string]yaml.Node
		if err := raw.Properties.Decode(&properties); err != nil {
			return fmt.Errorf("model %q customProperties: %w", raw.URI, err)
		}
		for key, value := range properties {
			if err := validateIndexProperty(&value); err != nil {
				return fmt.Errorf("model %q customProperties[%q]: %w", raw.URI, key, err)
			}
		}
	}
	type plain ModelEntry
	return node.Decode((*plain)(entry))
}

func validateIndexProperty(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return validateIndexProperty(node.Alias)
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected a typed metadata mapping")
	}
	var fields map[string]yaml.Node
	if err := node.Decode(&fields); err != nil {
		return err
	}
	typeNode := fields["metadataType"]
	if typeNode.Tag != "!!str" {
		return fmt.Errorf("metadataType must be a string")
	}
	var field, tag string
	switch typeNode.Value {
	case "MetadataStringValue":
		field, tag = "string_value", "!!str"
	case "MetadataBoolValue":
		field, tag = "bool_value", "!!bool"
	case "MetadataIntValue":
		field, tag = "int_value", "!!str"
	case "MetadataDoubleValue":
		field, tag = "double_value", "!!float"
	default:
		return fmt.Errorf("unsupported metadataType %q", typeNode.Value)
	}
	value, ok := fields[field]
	if !ok || len(fields) != 2 {
		return fmt.Errorf("%s requires exactly metadataType and %s", typeNode.Value, field)
	}
	if value.Tag != tag && (field != "double_value" || value.Tag != "!!int") {
		return fmt.Errorf("%s has an invalid value type (expected %s)", field, tag)
	}
	if field == "int_value" {
		if _, err := strconv.ParseInt(value.Value, 10, 64); err != nil {
			return fmt.Errorf("int_value must be a signed 64-bit decimal string: %w", err)
		}
	}
	if field == "double_value" {
		var number float64
		if err := value.Decode(&number); err != nil {
			return err
		}
		if math.IsInf(number, 0) || math.IsNaN(number) {
			return fmt.Errorf("double_value must be finite")
		}
	}
	return nil
}
