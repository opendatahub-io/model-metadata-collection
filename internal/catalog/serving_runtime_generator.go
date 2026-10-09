package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/distribution/reference"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

// ServingRuntimeGeneratorConfig separates template sources from deployment images.
// Support is derived from the target release and each template's runtime images.
type ServingRuntimeGeneratorConfig struct {
	Source   string                `yaml:"source"`
	Provider string                `yaml:"provider"`
	Sources  []RuntimeSourceConfig `yaml:"sources"`
}

type RuntimeSourceConfig struct {
	ID               string                           `yaml:"id"`
	Directory        string                           `yaml:"directory,omitempty"`
	TargetImage      string                           `yaml:"target_image,omitempty"`
	SourceImage      string                           `yaml:"source_image,omitempty"`
	TemplatePaths    []string                         `yaml:"template_paths,omitempty"`
	Include          []string                         `yaml:"include,omitempty"`
	Replacements     map[string]string                `yaml:"replacements,omitempty"`
	Templates        map[string]RuntimeTemplateConfig `yaml:"templates,omitempty"`
	KustomizeOverlay string                           `yaml:"kustomize_overlay,omitempty"`
	ParamsEnv        string                           `yaml:"params_env,omitempty"`
	Parameters       map[string]string                `yaml:"parameters,omitempty"`
	ImageVersions    map[string]string                `yaml:"-"`
	MissingImages    map[string]bool                  `yaml:"-"`
}

type RuntimeTemplateConfig struct {
	Name                string            `yaml:"name,omitempty"`
	Image               string            `yaml:"image,omitempty"`
	Version             string            `yaml:"version,omitempty"`
	MinimumRHOAIVersion string            `yaml:"minimum_rhoai_version,omitempty"`
	Replacements        map[string]string `yaml:"replacements,omitempty"`
}

type ServingRuntimeArtifacts struct {
	Index            []byte
	Catalog          []byte
	RuntimeFiles     map[string][]byte
	SkippedTemplates []SkippedRuntimeTemplate
}

type SkippedRuntimeTemplate struct {
	SourceID      string
	Path          string
	Name          string
	MissingImages []string
}

func ParseServingRuntimeGeneratorConfig(input []byte) (*ServingRuntimeGeneratorConfig, error) {
	var config ServingRuntimeGeneratorConfig
	if err := decodeRuntimeYAML(input, &config); err != nil {
		return nil, fmt.Errorf("serving runtime generator config: %w", err)
	}
	if strings.TrimSpace(config.Source) == "" || len(config.Sources) == 0 {
		return nil, fmt.Errorf("generator config requires source and at least one template source")
	}
	if config.Provider == "" {
		config.Provider = "Red Hat, Inc."
	}
	seen := map[string]bool{}
	for _, source := range config.Sources {
		if !runtimeNamePattern.MatchString(source.ID) || seen[source.ID] {
			return nil, fmt.Errorf("invalid or duplicate source id %q", source.ID)
		}
		seen[source.ID] = true
		locations := 0
		for _, value := range []string{source.Directory, source.TargetImage, source.SourceImage} {
			if value != "" {
				locations++
			}
		}
		if locations != 1 {
			return nil, fmt.Errorf("source %q requires exactly one of directory, target_image, source_image", source.ID)
		}
		for _, image := range []string{source.TargetImage, source.SourceImage} {
			if image != "" && !strings.HasPrefix(image, "dir:") {
				if err := validateRuntimeImage(image); err != nil {
					return nil, fmt.Errorf("source %q: %w", source.ID, err)
				}
			}
		}
		if strings.HasPrefix(source.TargetImage, "dir:") {
			return nil, fmt.Errorf("source %q: target_image must be a registry reference", source.ID)
		}
		for _, root := range source.TemplatePaths {
			if !fs.ValidPath(root) {
				return nil, fmt.Errorf("source %q: invalid template path %q", source.ID, root)
			}
		}
		for _, file := range []string{source.KustomizeOverlay, source.ParamsEnv} {
			if file != "" && !fs.ValidPath(file) {
				return nil, fmt.Errorf("source %q: invalid kustomize path %q", source.ID, file)
			}
		}
		for _, pattern := range source.Include {
			if _, err := path.Match(pattern, ""); err != nil {
				return nil, fmt.Errorf("source %q: invalid include pattern %q: %w", source.ID, pattern, err)
			}
		}
		for file := range source.Templates {
			if !fs.ValidPath(file) || file == "." {
				return nil, fmt.Errorf("source %q: invalid template override path %q", source.ID, file)
			}
		}
	}
	return &config, nil
}

// GenerateServingRuntimeArtifacts recursively discovers supported manifest kinds.
// sources provides one filesystem per source ID, rooted at the checkout or source
// archive. It emits separate entries for each Template/LLMInferenceServiceConfig.
func GenerateServingRuntimeArtifacts(config *ServingRuntimeGeneratorConfig, sources map[string]fs.FS, runtimeOutputDir string) (*ServingRuntimeArtifacts, error) {
	if !fs.ValidPath(runtimeOutputDir) || runtimeOutputDir == "." {
		return nil, fmt.Errorf("runtime output directory must be relative to the repository root")
	}
	runtimes := map[string]types.ServingRuntime{}
	objectNames := map[string]bool{}
	artifacts := &ServingRuntimeArtifacts{RuntimeFiles: map[string][]byte{}}
	for _, source := range config.Sources {
		files, ok := sources[source.ID]
		if !ok {
			return nil, fmt.Errorf("source %q was not loaded", source.ID)
		}
		matched := map[string]bool{}
		matchedIncludes := map[string]bool{}
		count := 0
		skipped := 0
		roots := runtimeTemplateRoots(source, files)
		err := fs.WalkDir(files, ".", func(file string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !entry.Type().IsRegular() || (path.Ext(file) != ".yaml" && path.Ext(file) != ".yml") || !selectedRuntimeFile(file, roots) {
				return nil
			}
			if len(source.Include) != 0 {
				included := false
				for _, pattern := range source.Include {
					if yes, _ := path.Match(pattern, file); yes {
						matchedIncludes[pattern] = true
						included = true
					}
				}
				if !included {
					return nil
				}
			}
			data, err := fs.ReadFile(files, file)
			if err != nil {
				return err
			}
			decoder := yaml.NewDecoder(bytes.NewReader(data))
			for {
				var manifest map[string]any
				if err := decoder.Decode(&manifest); errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					return fmt.Errorf("%s: parse manifest: %w", file, err)
				}
				resource := servingRuntimeResource(manifest)
				kind, _ := manifest["kind"].(string)
				if kind != "LLMInferenceServiceConfig" && resource == nil {
					continue // Kustomizations and ClusterServingRuntime mirrors are not catalog templates.
				}
				matched[file] = true
				if missing := missingRuntimeManifestImages(manifest, source, source.Templates[file]); len(missing) != 0 {
					artifacts.SkippedTemplates = append(artifacts.SkippedTemplates, SkippedRuntimeTemplate{
						SourceID: source.ID, Path: file, Name: textAt(objectAt(manifest, "metadata"), "name"), MissingImages: missing,
					})
					skipped++
					continue
				}
				versionedName := kind == "Template" || (kind == "LLMInferenceServiceConfig" && strings.HasPrefix(textAt(objectAt(manifest, "metadata"), "name"), "kserve-"))
				runtime, err := runtimeFromManifest(manifest, resource, config.Provider, source, source.Templates[file])
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
				if versionedName {
					for _, object := range versionedRuntimeObjects(manifest) {
						metadata := objectAt(object, "metadata")
						name := textAt(metadata, "name")
						kind := textAt(object, "kind")
						key := kind + "/" + textAt(metadata, "namespace") + "/" + name
						if objectNames[key] {
							return fmt.Errorf("%s: duplicate versioned %s name %q", file, kind, name)
						}
						objectNames[key] = true
					}
				}
				if existing, ok := runtimes[runtime.Name]; ok {
					// The same template from another release contributes a version.
					existing.Versions = append(existing.Versions, runtime.Versions...)
					runtimes[runtime.Name] = existing
				} else {
					runtimes[runtime.Name] = runtime
				}
				count++
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", source.ID, err)
		}
		if count == 0 && skipped == 0 {
			return nil, fmt.Errorf("source %q: no runtime templates discovered", source.ID)
		}
		for file := range source.Templates {
			if !matched[file] {
				return nil, fmt.Errorf("source %q: template override %q did not match a runtime template", source.ID, file)
			}
		}
		for _, pattern := range source.Include {
			if !matchedIncludes[pattern] {
				return nil, fmt.Errorf("source %q: include pattern %q matched no files", source.ID, pattern)
			}
		}
	}
	if len(runtimes) == 0 {
		return nil, fmt.Errorf("no runtime versions have available parameter images; existing outputs were not changed")
	}
	result := types.ServingRuntimeCatalog{Source: config.Source}
	index := types.ServingRuntimeIndex{Source: config.Source}
	for _, name := range slices.Sorted(maps.Keys(runtimes)) {
		runtime := runtimes[name]
		slices.SortFunc(runtime.Versions, func(a, b types.ServingRuntimeVersion) int { return strings.Compare(a.Version, b.Version) })
		result.ServingRuntimes = append(result.ServingRuntimes, runtime)
		file := path.Join(runtimeOutputDir, name+".yaml")
		index.ServingRuntimes = append(index.ServingRuntimes, types.ServingRuntimeIndexEntry{Name: name, InputPath: file})
		data, err := marshalDiscoveredRuntime(runtime)
		if err != nil {
			return nil, err
		}
		artifacts.RuntimeFiles[file] = data
	}
	if err := ValidateServingRuntimeCatalog(&result); err != nil {
		return nil, err
	}
	var err error
	artifacts.Index, err = marshalServingRuntimeYAML(index)
	if err != nil {
		return nil, err
	}
	artifacts.Catalog, err = marshalServingRuntimeYAML(result)
	return artifacts, err
}

func selectedRuntimeFile(file string, roots []string) bool {
	if len(roots) == 0 {
		return true
	}
	for _, root := range roots {
		if root == "." || strings.HasPrefix(file, root+"/") {
			return true
		}
	}
	return false
}

func runtimeTemplateRoots(source RuntimeSourceConfig, files fs.FS) []string {
	if len(source.TemplatePaths) != 0 {
		return source.TemplatePaths
	}
	var roots []string
	for _, root := range defaultRuntimeTemplatePaths {
		if info, err := fs.Stat(files, root); err == nil && info.IsDir() {
			roots = append(roots, root)
		}
	}
	return roots // A directory pointing directly at templates is scanned in full.
}

func objectAt(object map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		object, _ = object[key].(map[string]any)
	}
	return object
}

func textAt(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return value
}

func servingRuntimeResource(manifest map[string]any) map[string]any {
	if textAt(manifest, "kind") != "Template" {
		return nil
	}
	objects, _ := manifest["objects"].([]any)
	for _, object := range objects {
		resource, _ := object.(map[string]any)
		if textAt(resource, "kind") == "ServingRuntime" {
			return resource
		}
	}
	return nil
}

func runtimeFromManifest(manifest, resource map[string]any, provider string, source RuntimeSourceConfig, override RuntimeTemplateConfig) (types.ServingRuntime, error) {
	metadata := objectAt(manifest, "metadata")
	annotations := objectAt(metadata, "annotations")
	name := textAt(metadata, "name")
	if name == "" {
		return types.ServingRuntime{}, fmt.Errorf("template metadata.name is required")
	}
	if resource == nil {
		resource = manifest
	} else {
		name = strings.TrimSuffix(name, "-runtime-template")
	}
	if override.Name != "" {
		name = override.Name
	}
	resourceAnnotations := objectAt(resource, "metadata", "annotations")
	images, err := replaceRuntimeImages(manifest, runtimeImageReplacements(source, override))
	if err != nil {
		return types.ServingRuntime{}, err
	}
	if len(images) == 0 {
		return types.ServingRuntime{}, fmt.Errorf("template contains no container image")
	}
	image := primaryRuntimeImage(resource)
	if image == "" {
		image = images[0]
	}
	if override.Image != "" {
		image = override.Image
		if !slices.Contains(images, image) {
			return types.ServingRuntime{}, fmt.Errorf("configured image %q is absent from the rendered template", image)
		}
	}
	version := override.Version
	parameterVersion, err := runtimeParameterVersion(source.Parameters, image)
	if err != nil {
		return types.ServingRuntime{}, err
	}
	if version == "" {
		version = parameterVersion
	}
	if version == "" {
		version = textAt(resourceAnnotations, "opendatahub.io/runtime-version")
	}
	if version == "" {
		if ref, err := reference.ParseNormalizedNamed(image); err == nil {
			if tagged, ok := ref.(reference.Tagged); ok {
				version = tagged.Tag()
			}
		}
	}
	if version == "" {
		version = source.ImageVersions[image]
	}
	if version == "" {
		return types.ServingRuntime{}, fmt.Errorf("version is required for digest-pinned images without a runtime-version annotation")
	}
	if err := versionRuntimeManifestNames(manifest, version); err != nil {
		return types.ServingRuntime{}, err
	}
	// Empty annotations on KServe accelerator configs are populated with the runtime version.
	if resourceAnnotations != nil && (override.Version != "" || parameterVersion != "" || textAt(resourceAnnotations, "opendatahub.io/runtime-version") == "") {
		resourceAnnotations["opendatahub.io/runtime-version"] = version
	}
	supportLevel := inferServingRuntimeSupport(source.TargetImage, images)
	displayName := textAt(annotations, "openshift.io/display-name")
	if displayName == "" {
		displayName = textAt(resourceAnnotations, "openshift.io/display-name")
	}
	if displayName == "" {
		displayName = name
	}
	description := textAt(annotations, "description")
	if description == "" {
		description = textAt(annotations, "openshift.io/description")
	}
	if description == "" {
		description = displayName
	}
	if annotatedProvider := textAt(annotations, "openshift.io/provider-display-name"); annotatedProvider != "" {
		provider = annotatedProvider
	}
	runtime := types.ServingRuntime{
		Name: name, DisplayName: displayName, Provider: provider, Description: description,
		DocumentationURL: textAt(annotations, "template.openshift.io/documentation-url"),
	}
	if tags := textAt(annotations, "tags"); tags != "" {
		for _, tag := range strings.Split(tags, ",") {
			if tag = strings.TrimSpace(tag); tag != "" {
				runtime.Tags = append(runtime.Tags, tag)
			}
		}
	}
	capabilities := &types.RuntimeCapabilities{}
	if accelerators := textAt(resourceAnnotations, "opendatahub.io/recommended-accelerators"); accelerators != "" {
		if err := json.Unmarshal([]byte(accelerators), &capabilities.SupportedAccelerators); err != nil {
			return types.ServingRuntime{}, fmt.Errorf("invalid recommended-accelerators annotation: %w", err)
		}
	}
	capabilities.RequiresGPU = len(capabilities.SupportedAccelerators) > 0
	capabilities.MultiModel, _ = objectAt(resource, "spec")["multiModel"].(bool)
	runtime.Capabilities = capabilities
	if formats := objectAt(resource, "spec")["supportedModelFormats"]; formats != nil {
		data, err := json.Marshal(formats)
		if err != nil {
			return types.ServingRuntime{}, err
		}
		if err := json.Unmarshal(data, &runtime.SupportedModelFormats); err != nil {
			return types.ServingRuntime{}, err
		}
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return types.ServingRuntime{}, fmt.Errorf("encode template: %w", err)
	}
	runtimeVersion := types.ServingRuntimeVersion{
		Version: version, Image: image, SupportLevel: supportLevel, MinimumRHOAIVersion: override.MinimumRHOAIVersion,
	}
	if runtimeVersion.MinimumRHOAIVersion == "" {
		runtimeVersion.MinimumRHOAIVersion = inferMinimumRHOAIVersion(source.TargetImage)
	}
	if textAt(manifest, "kind") == "Template" {
		runtimeVersion.ServingRuntimeTemplate = string(encoded)
	} else {
		runtimeVersion.LLMInferenceServiceConfig = string(encoded)
	}
	runtime.Versions = []types.ServingRuntimeVersion{runtimeVersion}
	return runtime, nil
}

func runtimeImageReplacements(source RuntimeSourceConfig, override RuntimeTemplateConfig) map[string]string {
	replacements := map[string]string{}
	maps.Copy(replacements, source.Replacements)
	maps.Copy(replacements, override.Replacements)
	if override.Image != "" {
		replacements["placeholder"] = override.Image
	}
	return replacements
}

// Consider every image field, including worker/prefill, init and sidecar images.
// Checking after configured substitutions excludes only manifests that actually
// use an unavailable parameter, including overrides that reuse YAML aliases.
func missingRuntimeManifestImages(manifest map[string]any, source RuntimeSourceConfig, override RuntimeTemplateConfig) []string {
	if len(source.MissingImages) == 0 {
		return nil
	}
	replacements := runtimeImageReplacements(source, override)
	missing := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch object := value.(type) {
		case map[string]any:
			for key, child := range object {
				if key == "image" {
					image, _ := child.(string)
					if replacement, ok := replacements[image]; ok {
						image = replacement
					}
					if source.MissingImages[image] {
						missing[image] = true
					}
					continue
				}
				visit(child)
			}
		case []any:
			for _, child := range object {
				visit(child)
			}
		}
	}
	visit(manifest)
	return slices.Sorted(maps.Keys(missing))
}

// Older controller sources lack version replacements. Pair an image parameter
// with its upstream-version parameter so their stale annotations are updated too.
func runtimeParameterVersion(parameters map[string]string, image string) (string, error) {
	version := ""
	for key, value := range parameters {
		if value != image {
			continue
		}
		candidate := parameters[key+"-upstream-version"]
		if candidate == "" {
			continue
		}
		if version != "" && version != candidate {
			return "", fmt.Errorf("conflicting upstream-version parameters for image %s", image)
		}
		version = candidate
	}
	return version, nil
}

func primaryRuntimeImage(resource map[string]any) string {
	spec := objectAt(resource, "spec")
	if textAt(resource, "kind") == "LLMInferenceServiceConfig" {
		spec = objectAt(spec, "template")
	}
	containers, _ := spec["containers"].([]any)
	for _, value := range containers {
		container, _ := value.(map[string]any)
		if name := textAt(container, "name"); name == "main" || name == "kserve-container" {
			return textAt(container, "image")
		}
	}
	if len(containers) != 0 {
		container, _ := containers[0].(map[string]any)
		return textAt(container, "image")
	}
	return ""
}

// Only image scalar values are replaced. Go template expressions, CLI args and
// unrelated OpenShift parameters retain their original meaning.
func replaceRuntimeImages(value any, replacements map[string]string) ([]string, error) {
	var images []string
	switch object := value.(type) {
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(object)) {
			child := object[key]
			if key == "image" {
				image, ok := child.(string)
				if !ok {
					return nil, fmt.Errorf("container image must be a string")
				}
				if replacement, ok := replacements[image]; ok {
					image = replacement
					object[key] = image
				}
				if err := validateRuntimeImage(image); err != nil {
					return nil, fmt.Errorf("unresolved or invalid template image: %w; configure a replacement", err)
				}
				images = append(images, image)
				continue
			}
			found, err := replaceRuntimeImages(child, replacements)
			if err != nil {
				return nil, err
			}
			images = append(images, found...)
		}
	case []any:
		for _, child := range object {
			found, err := replaceRuntimeImages(child, replacements)
			if err != nil {
				return nil, err
			}
			images = append(images, found...)
		}
	}
	return images, nil
}

func marshalDiscoveredRuntime(runtime types.ServingRuntime) ([]byte, error) {
	data, err := yaml.Marshal(runtime)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	versions, _ := document["versions"].([]any)
	for _, value := range versions {
		version, _ := value.(map[string]any)
		for _, field := range []string{"servingRuntimeTemplate", "llmInferenceServiceConfig"} {
			if text, ok := version[field].(string); ok {
				var manifest map[string]any
				if err := yaml.Unmarshal([]byte(text), &manifest); err != nil {
					return nil, err
				}
				version[field] = manifest
			}
		}
	}
	return marshalServingRuntimeYAML(document)
}
