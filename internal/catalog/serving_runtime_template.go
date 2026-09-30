package catalog

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"gopkg.in/yaml.v3"
)

// Only fields used by the catalog adapter are decoded. Other Kubernetes fields
// remain in the original input; this adapter does not validate a deployment.
type runtimeTemplate struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Objects []struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name        string            `yaml:"name"`
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"metadata"`
		Spec struct {
			MultiModel            bool                         `yaml:"multiModel"`
			SupportedModelFormats []types.SupportedModelFormat `yaml:"supportedModelFormats"`
			Containers            []struct {
				Image string   `yaml:"image"`
				Args  []string `yaml:"args"`
				Env   []struct {
					Name      string     `yaml:"name"`
					Value     *string    `yaml:"value"`
					ValueFrom *yaml.Node `yaml:"valueFrom"`
				} `yaml:"env"`
				EnvFrom []yaml.Node `yaml:"envFrom"`
			} `yaml:"containers"`
		} `yaml:"spec"`
	} `yaml:"objects"`
}

func decodeServingRuntimeInput(data []byte, entry types.ServingRuntimeIndexEntry) (types.ServingRuntime, error) {
	var result types.ServingRuntime
	// Decode once as a map to reject duplicate keys and multiple documents before
	// detecting the format, without rejecting legitimate Kubernetes fields.
	var fields map[string]any
	if err := decodeRuntimeYAML(data, &fields); err != nil {
		return result, err
	}
	if _, ok := fields["kind"]; !ok {
		if err := decodeRuntimeYAML(data, &result); err != nil {
			return result, err
		}
		for i := range result.Versions {
			if entry.Image != "" {
				result.Versions[i].Image = entry.Image
			}
		}
		return result, nil
	}
	var template runtimeTemplate
	if err := yaml.Unmarshal(data, &template); err != nil {
		return result, fmt.Errorf("parse runtime template: %w", err)
	}
	if template.Kind != "Template" || template.APIVersion != "template.openshift.io/v1" {
		return result, fmt.Errorf("unsupported runtime input %s %s; expected template.openshift.io/v1 Template", template.APIVersion, template.Kind)
	}
	if len(template.Objects) != 1 {
		return result, fmt.Errorf("runtime template requires exactly one ServingRuntime object")
	}
	runtime := template.Objects[0]
	if runtime.Kind != "ServingRuntime" || runtime.APIVersion != "serving.kserve.io/v1alpha1" {
		return result, fmt.Errorf("template object must be serving.kserve.io/v1alpha1 ServingRuntime")
	}
	if len(runtime.Spec.Containers) != 1 {
		return result, fmt.Errorf("runtime template requires exactly one container")
	}
	ann := template.Metadata.Annotations
	ra := runtime.Metadata.Annotations
	result.Name = runtime.Metadata.Name
	result.DisplayName = ra["openshift.io/display-name"]
	if result.DisplayName == "" {
		result.DisplayName = ann["openshift.io/display-name"]
	}
	result.Provider = ann["openshift.io/provider-display-name"]
	result.Description = ann["description"]
	result.DocumentationURL = ann["template.openshift.io/documentation-url"]
	if tags := ann["tags"]; tags != "" {
		result.Tags = strings.Split(tags, ",")
	}
	result.SupportedModelFormats = runtime.Spec.SupportedModelFormats
	result.Capabilities = &types.RuntimeCapabilities{MultiModel: runtime.Spec.MultiModel}
	if accelerators := ra["opendatahub.io/recommended-accelerators"]; accelerators != "" {
		if err := json.Unmarshal([]byte(accelerators), &result.Capabilities.SupportedAccelerators); err != nil {
			return result, fmt.Errorf("invalid recommended-accelerators annotation: %w", err)
		}
	}
	container := runtime.Spec.Containers[0]
	version := types.ServingRuntimeVersion{Version: ra["opendatahub.io/runtime-version"], Image: container.Image, DefaultArgs: container.Args}
	if entry.Image != "" {
		version.Image = entry.Image
	}
	if strings.Contains(version.Image, "$(") {
		return result, fmt.Errorf("unresolved image %q; set image in the index entry to a fully qualified non-latest tag or digest", version.Image)
	}
	if len(container.EnvFrom) != 0 {
		return result, fmt.Errorf("envFrom cannot be represented in the runtime catalog")
	}
	for _, env := range container.Env {
		if env.ValueFrom != nil {
			return result, fmt.Errorf("environment variable %q: valueFrom cannot be represented in the runtime catalog", env.Name)
		}
		version.Env = append(version.Env, types.RuntimeEnvVar{Name: env.Name, DefaultValue: env.Value})
	}
	result.Versions = []types.ServingRuntimeVersion{version}
	return result, nil
}
