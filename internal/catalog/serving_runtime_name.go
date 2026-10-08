package catalog

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

const kubernetesObjectNameMaxLength = 253

var kubernetesObjectNamePattern = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?(?:\.[a-z0-9](?:[-a-z0-9]*[a-z0-9])?)*$`)

// LLM configs without the upstream kserve- prefix keep their supplied names.
func versionedLLMConfigName(name, version string) (string, error) {
	if !strings.HasPrefix(name, "kserve-") {
		return name, nil
	}
	return versionedRuntimeObjectName(name, version)
}

// versionedRuntimeObjectName prefixes a DNS-safe runtime version without build
// metadata, replacing kserve- when present. A hash preserves identity when a
// long name must be shortened.
func versionedRuntimeObjectName(name, version string) (string, error) {
	// SemVer build metadata is not part of the runtime release identity.
	nameVersion, _, _ := strings.Cut(version, "+")
	var prefix strings.Builder
	separator := false
	for _, char := range strings.ToLower(nameVersion) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			if separator && prefix.Len() > 0 {
				prefix.WriteByte('-')
			}
			prefix.WriteRune(char)
			separator = false
		} else {
			separator = true
		}
	}
	// Also accept versions whose RHAI build suffix was already normalized.
	versionPrefix, _, _ := strings.Cut(prefix.String(), "-rhaiv-")
	if versionPrefix == "" {
		return "", fmt.Errorf("runtime version %q contains no characters usable in a Kubernetes object name", version)
	}
	result := versionPrefix + "-" + strings.TrimPrefix(name, "kserve-")
	if len(result) > kubernetesObjectNameMaxLength {
		digest := sha256.Sum256([]byte(result))
		result = strings.TrimRight(result[:kubernetesObjectNameMaxLength-13], "-.") + fmt.Sprintf("-%x", digest[:6])
	}
	if !kubernetesObjectNamePattern.MatchString(result) {
		return "", fmt.Errorf("versioned runtime object name %q is not a valid Kubernetes object name", result)
	}
	return result, nil
}

// Include every ServingRuntime inside a Template, while leaving unrelated
// objects and OpenShift template parameters untouched.
func versionedRuntimeObjects(manifest map[string]any) []map[string]any {
	objects := []map[string]any{manifest}
	if textAt(manifest, "kind") == "Template" {
		children, _ := manifest["objects"].([]any)
		for _, child := range children {
			object, _ := child.(map[string]any)
			if textAt(object, "kind") == "ServingRuntime" {
				objects = append(objects, object)
			}
		}
	}
	return objects
}

func versionRuntimeManifestNames(manifest map[string]any, version string) error {
	for _, object := range versionedRuntimeObjects(manifest) {
		metadata := objectAt(object, "metadata")
		name := textAt(metadata, "name")
		if name == "" {
			return fmt.Errorf("%s metadata.name is required", textAt(object, "kind"))
		}
		var versionedName string
		var err error
		if textAt(object, "kind") == "LLMInferenceServiceConfig" {
			versionedName, err = versionedLLMConfigName(name, version)
		} else {
			versionedName, err = versionedRuntimeObjectName(name, version)
		}
		if err != nil {
			return err
		}
		metadata["name"] = versionedName
	}
	return nil
}
