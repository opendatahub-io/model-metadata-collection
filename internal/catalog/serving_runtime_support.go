package catalog

import (
	"regexp"
	"slices"
	"strings"

	"github.com/distribution/reference"
)

var earlyAccessRuntimeTag = regexp.MustCompile(`(?i)(^|[._-])(?:ea|fast)[0-9]*(?:[._-]|$)`)

// inferServingRuntimeSupport implements the release policy for generated entries:
// an EA controller release or a preview/unknown runtime makes the whole template
// techPreview. Only images actually used by this template contribute to its level.
func inferServingRuntimeSupport(targetImage string, runtimeImages []string) string {
	target, err := reference.ParseNormalizedNamed(targetImage)
	if err != nil || !supportedRuntimeRepository(target) {
		return "techPreview"
	}
	tagged, ok := target.(reference.Tagged)
	if !ok || earlyAccessRuntimeTag.MatchString(tagged.Tag()) {
		// A checkout or digest-only target does not establish a GA release tag.
		return "techPreview"
	}
	for _, image := range runtimeImages {
		runtime, err := reference.ParseNormalizedNamed(image)
		if err != nil || !supportedRuntimeRepository(runtime) {
			return "techPreview"
		}
		if tagged, ok := runtime.(reference.Tagged); ok && earlyAccessRuntimeTag.MatchString(tagged.Tag()) {
			return "techPreview"
		}
	}
	if len(runtimeImages) == 0 {
		return "techPreview"
	}
	return "supported"
}

func supportedRuntimeRepository(image reference.Named) bool {
	if !slices.Contains([]string{"registry.redhat.io", "registry.access.redhat.com"}, reference.Domain(image)) {
		return false
	}
	namespace, _, ok := strings.Cut(reference.Path(image), "/")
	// rhaiis is the previous namespace still used by published RHOAI templates.
	// Fast/preview namespaces do not match these stable product namespaces.
	return ok && slices.Contains([]string{"rhoai", "rhaii", "rhaiis"}, namespace)
}
