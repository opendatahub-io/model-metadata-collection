package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/distribution/reference"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

var (
	runtimeNamePattern       = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	envNamePattern           = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	quantityPattern          = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:m|Ki|Mi|Gi|Ti|Pi|Ei|k|M|G|T|P|E)?$`)
	acceleratorPattern       = regexp.MustCompile(`^[a-z0-9.-]+/[a-z0-9.-]+$`)
	acceleratorAmountPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
	secretNamePattern        = regexp.MustCompile(`(?i)(token|password|passwd|secret|api.?key|credential)`)
)

func decodeRuntimeYAML(input []byte, value any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(input))
	decoder.KnownFields(true)
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("multiple YAML documents are not supported")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("parse trailing YAML: %w", err)
	}
	return nil
}

// GenerateServingRuntimeCatalog loads the index's individual input files and
// emits deterministic YAML in the serving_runtime loader's catalog shape.
// Input paths are relative to the supplied filesystem root, like MCP input_path.
func GenerateServingRuntimeCatalog(input []byte, inputFiles fs.FS) ([]byte, error) {
	var index types.ServingRuntimeIndex
	if err := decodeRuntimeYAML(input, &index); err != nil {
		return nil, fmt.Errorf("serving runtime index: %w", err)
	}
	if strings.TrimSpace(index.Source) == "" || len(index.ServingRuntimes) == 0 {
		return nil, fmt.Errorf("serving runtime index requires source and at least one entry")
	}
	result := types.ServingRuntimeCatalog{Source: index.Source}
	seen := make(map[string]bool)
	for _, entry := range index.ServingRuntimes {
		if !runtimeNamePattern.MatchString(entry.Name) || seen[entry.Name] {
			return nil, fmt.Errorf("invalid or duplicate serving runtime index name %q", entry.Name)
		}
		seen[entry.Name] = true
		if !fs.ValidPath(entry.InputPath) || entry.InputPath == "." {
			return nil, fmt.Errorf("runtime %q: invalid input_path %q", entry.Name, entry.InputPath)
		}
		data, err := fs.ReadFile(inputFiles, entry.InputPath)
		if err != nil {
			return nil, fmt.Errorf("runtime %q: read %s: %w", entry.Name, entry.InputPath, err)
		}
		var runtime types.ServingRuntime
		if err := decodeRuntimeYAML(data, &runtime); err != nil {
			return nil, fmt.Errorf("runtime %q (%s): %w", entry.Name, entry.InputPath, err)
		}
		if runtime.Name != entry.Name {
			return nil, fmt.Errorf("runtime name mismatch: index has %q but %s declares %q", entry.Name, entry.InputPath, runtime.Name)
		}
		result.ServingRuntimes = append(result.ServingRuntimes, runtime)
	}
	if err := ValidateServingRuntimeCatalog(&result); err != nil {
		return nil, err
	}
	slices.SortFunc(result.ServingRuntimes, func(left, right types.ServingRuntime) int {
		return strings.Compare(left.Name, right.Name)
	})
	for index := range result.ServingRuntimes {
		slices.SortFunc(result.ServingRuntimes[index].Versions, func(left, right types.ServingRuntimeVersion) int {
			return strings.Compare(left.Version, right.Version)
		})
	}
	output, err := yaml.Marshal(&result)
	if err != nil {
		return nil, fmt.Errorf("marshal serving runtime catalog: %w", err)
	}
	return output, nil
}

func ValidateServingRuntimeCatalog(catalog *types.ServingRuntimeCatalog) error {
	if catalog == nil || strings.TrimSpace(catalog.Source) == "" || len(catalog.ServingRuntimes) == 0 {
		return fmt.Errorf("source and at least one serving runtime are required")
	}
	seen := make(map[string]bool)
	for _, runtime := range catalog.ServingRuntimes {
		if !runtimeNamePattern.MatchString(runtime.Name) || seen[runtime.Name] {
			return fmt.Errorf("invalid or duplicate runtime name %q", runtime.Name)
		}
		seen[runtime.Name] = true
		if strings.TrimSpace(runtime.DisplayName) == "" || strings.TrimSpace(runtime.Provider) == "" || strings.TrimSpace(runtime.Description) == "" || len(runtime.Versions) == 0 {
			return fmt.Errorf("runtime %q requires displayName, provider, description and versions", runtime.Name)
		}
		if err := validateFormats(runtime.SupportedModelFormats); err != nil {
			return fmt.Errorf("runtime %q: %w", runtime.Name, err)
		}
		versions := make(map[string]bool)
		for _, version := range runtime.Versions {
			if strings.TrimSpace(version.Version) == "" || versions[version.Version] {
				return fmt.Errorf("runtime %q: missing or duplicate version %q", runtime.Name, version.Version)
			}
			versions[version.Version] = true
			if err := validateVersion(version); err != nil {
				return fmt.Errorf("runtime %q version %q: %w", runtime.Name, version.Version, err)
			}
		}
	}
	return nil
}

func validateFormats(formats []types.SupportedModelFormat) error {
	seen := make(map[string]bool)
	for _, format := range formats {
		if strings.TrimSpace(format.Name) == "" || seen[format.Name+":"+format.Version] {
			return fmt.Errorf("missing or duplicate model format %q", format.Name)
		}
		seen[format.Name+":"+format.Version] = true
	}
	return nil
}

func validateVersion(version types.ServingRuntimeVersion) error {
	image, err := reference.ParseNormalizedNamed(version.Image)
	if err != nil || !strings.Contains(strings.Split(version.Image, "/")[0], ".") || reference.IsNameOnly(image) {
		return fmt.Errorf("image %q must be a fully qualified pinned container reference", version.Image)
	}
	if !slices.Contains([]string{"supported", "techPreview", "developerPreview", "community"}, version.SupportLevel) {
		return fmt.Errorf("invalid supportLevel %q", version.SupportLevel)
	}
	if err := validateFormats(version.SupportedModelFormats); err != nil {
		return err
	}
	for _, protocol := range version.ProtocolVersions {
		if !slices.Contains([]string{"v1", "v2", "grpc-v2"}, protocol) {
			return fmt.Errorf("invalid protocol version %q", protocol)
		}
	}
	for _, arg := range version.DefaultArgs {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("defaultArgs cannot contain empty arguments")
		}
	}
	seen := make(map[string]bool)
	for _, env := range version.Env {
		if !envNamePattern.MatchString(env.Name) || seen[env.Name] {
			return fmt.Errorf("invalid or duplicate environment variable %q", env.Name)
		}
		seen[env.Name] = true
		if env.DefaultValue != nil && (env.Secret || env.Required || secretNamePattern.MatchString(env.Name)) {
			return fmt.Errorf("unsafe defaultValue for environment variable %q", env.Name)
		}
	}
	if resources := version.RecommendedResources; resources != nil {
		if resources.Minimal == nil && resources.Recommended == nil && resources.High == nil {
			return fmt.Errorf("recommendedResources requires at least one tier")
		}
		for _, tier := range []*types.RuntimeResourceTier{resources.Minimal, resources.Recommended, resources.High} {
			if tier == nil {
				continue
			}
			if !quantityPattern.MatchString(tier.CPU) || !quantityPattern.MatchString(tier.Memory) || tier.CPU == "0" || tier.Memory == "0" {
				return fmt.Errorf("resource tier requires valid cpu and memory quantities")
			}
			for name, amount := range tier.Accelerator {
				if !acceleratorPattern.MatchString(name) || !acceleratorAmountPattern.MatchString(amount) {
					return fmt.Errorf("invalid accelerator request %q=%q", name, amount)
				}
			}
		}
	}
	return nil
}
