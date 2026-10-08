package catalog

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

// ServingRuntimeSchemaRevision pins both upstream OpenAPI fragments. See the
// serving_runtime_schema README for provenance and the update procedure.
const ServingRuntimeSchemaRevision = "16f777698230d1ee76d1b5745336839d54edd82f"

//go:embed serving_runtime_schema/*.yaml
var servingRuntimeSchemas embed.FS

type ServingRuntimeSchemaValidation struct {
	Runtimes          int
	Versions          int
	CatalogExtensions []string
}

// ValidateServingRuntimeCatalogSchema validates catalog fields against the
// upstream ServingRuntime and ServingRuntimeVersion API schemas. The catalog
// wrapper and versions relationship are loader-specific; artifactType is supplied
// by the loader. Unknown catalog fields and our existing semantic constraints are
// checked too, because the API schemas allow additional properties.
func ValidateServingRuntimeCatalogSchema(ctx context.Context, input []byte) (*ServingRuntimeSchemaValidation, error) {
	var typed types.ServingRuntimeCatalog
	if err := decodeRuntimeYAML(input, &typed); err != nil {
		return nil, err
	}
	// Validate the original YAML values, rather than the Go structs: decoding can
	// coerce numbers into strings or discard nulls and zero values via omitempty.
	var node yaml.Node
	if err := decodeRuntimeYAML(input, &node); err != nil {
		return nil, err
	}
	// YAML timestamps must retain their original text. Decoding into interface{}
	// would turn a date-only value into time.Time and silently add an API-valid
	// midnight timestamp when marshaling JSON.
	preserveRuntimeTimestampStrings(&node)
	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("convert catalog YAML to JSON: %w", err)
	}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return nil, err
	}
	if source, ok := raw["source"].(string); !ok || strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("catalog source must be a nonempty string")
	}
	runtimes, ok := raw["serving_runtimes"].([]any)
	if !ok || len(runtimes) == 0 {
		return nil, fmt.Errorf("catalog requires a nonempty serving_runtimes array")
	}
	document, err := loadServingRuntimeSchemas(ctx)
	if err != nil {
		return nil, err
	}
	result := &ServingRuntimeSchemaValidation{Runtimes: len(runtimes)}
	minimumIsExtension := !runtimeAPIProperties(document.Components.Schemas["ServingRuntimeVersion"].Value)["minimumRHOAIVersion"]
	for index, entry := range runtimes {
		runtime, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("serving_runtimes[%d] must be an object", index)
		}
		versions, ok := runtime["versions"].([]any)
		if !ok || len(versions) == 0 {
			return nil, fmt.Errorf("runtime %q requires a nonempty versions array", typed.ServingRuntimes[index].Name)
		}
		result.Versions += len(versions)
		projected := maps.Clone(runtime)
		delete(projected, "versions")
		if err := validateRuntimeAPIObject(document, "ServingRuntime", projected); err != nil {
			return nil, fmt.Errorf("runtime %q: %w", typed.ServingRuntimes[index].Name, err)
		}
		for versionIndex, entry := range versions {
			version, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("runtime %q versions[%d] must be an object", typed.ServingRuntimes[index].Name, versionIndex)
			}
			projected := maps.Clone(version)
			projected["artifactType"] = "serving-runtime-version"
			if minimum, present := projected["minimumRHOAIVersion"]; present && minimumIsExtension {
				text, ok := minimum.(string)
				if !ok || text == "" {
					return nil, fmt.Errorf("runtime %q versions[%d]: minimumRHOAIVersion must be a nonempty release string", typed.ServingRuntimes[index].Name, versionIndex)
				}
				delete(projected, "minimumRHOAIVersion")
				result.CatalogExtensions = []string{"minimumRHOAIVersion"}
			}
			if err := validateRuntimeAPIObject(document, "ServingRuntimeVersion", projected); err != nil {
				return nil, fmt.Errorf("runtime %q versions[%d]: %w", typed.ServingRuntimes[index].Name, versionIndex, err)
			}
		}
	}
	if err := ValidateServingRuntimeCatalog(&typed); err != nil {
		return nil, err
	}
	return result, nil
}

func preserveRuntimeTimestampStrings(node *yaml.Node) {
	if node.Tag == "!!timestamp" {
		node.Tag = "!!str"
	}
	for _, child := range node.Content {
		preserveRuntimeTimestampStrings(child)
	}
}

func loadServingRuntimeSchemas(ctx context.Context) (*openapi3.T, error) {
	// These are fragments of a larger API. Merge only component schemas, keeping
	// upstream definitions intact and resolving their shared local references.
	schemas := map[string]any{}
	for _, file := range []string{"common.yaml", "serving_runtime-v1.yaml"} {
		data, err := servingRuntimeSchemas.ReadFile("serving_runtime_schema/" + file)
		if err != nil {
			return nil, err
		}
		var fragment struct {
			Components struct {
				Schemas map[string]any `yaml:"schemas"`
			} `yaml:"components"`
		}
		if err := yaml.Unmarshal(data, &fragment); err != nil {
			return nil, fmt.Errorf("parse upstream schema %s: %w", file, err)
		}
		for name, schema := range fragment.Components.Schemas {
			if _, exists := schemas[name]; exists {
				return nil, fmt.Errorf("duplicate upstream schema %q", name)
			}
			schemas[name] = schema
		}
	}
	data, err := json.Marshal(map[string]any{
		"openapi":    "3.0.3",
		"info":       map[string]string{"title": "Serving runtime catalog validation", "version": ServingRuntimeSchemaRevision},
		"paths":      map[string]any{},
		"components": map[string]any{"schemas": schemas},
	})
	if err != nil {
		return nil, err
	}
	loader := openapi3.NewLoader()
	loader.Context = ctx
	document, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("resolve upstream schemas: %w", err)
	}
	if err := document.Validate(ctx); err != nil {
		return nil, fmt.Errorf("validate upstream schemas: %w", err)
	}
	return document, nil
}

func validateRuntimeAPIObject(document *openapi3.T, name string, value map[string]any) error {
	ref := document.Components.Schemas[name]
	if ref == nil || ref.Value == nil {
		return fmt.Errorf("upstream schema %q is missing", name)
	}
	// Reject fields absent from the API even when upstream additionalProperties
	// permits them. Deliberate catalog extensions are handled by the projection.
	properties := runtimeAPIProperties(ref.Value)
	for _, field := range slices.Sorted(maps.Keys(value)) {
		if !properties[field] {
			return fmt.Errorf("field %q is absent from upstream %s schema at %s; the registry loader may ignore it", field, name, ServingRuntimeSchemaRevision)
		}
	}
	return ref.Value.VisitJSON(value,
		openapi3.EnableFormatValidation(),
		openapi3.WithStringFormatValidator("uri", openapi3.NewCallbackValidator(validateRuntimeURI)),
		openapi3.SetSchemaErrorMessageCustomizer(func(err *openapi3.SchemaError) string {
			// Avoid dumping the full catalog or embedded manifest in diagnostics.
			return fmt.Sprintf("%s at /%s: %s", name, strings.Join(err.JSONPointer(), "/"), err.Reason)
		}),
	)
}

func runtimeAPIProperties(schema *openapi3.Schema) map[string]bool {
	properties := map[string]bool{}
	var collect func(*openapi3.Schema)
	collect = func(schema *openapi3.Schema) {
		for field := range schema.Properties {
			properties[field] = true
		}
		for _, parent := range schema.AllOf {
			collect(parent.Value)
		}
	}
	collect(schema)
	return properties
}

func validateRuntimeURI(value string) error {
	if strings.IndexFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return fmt.Errorf("URI contains whitespace or control characters")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return err
	}
	if !parsed.IsAbs() {
		return fmt.Errorf("URI must include a scheme")
	}
	return nil
}
