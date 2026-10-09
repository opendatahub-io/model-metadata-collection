package catalog

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/distribution/reference"
)

var rhoaiReleaseTag = regexp.MustCompile(`^v?([0-9]+\.[0-9]+(?:\.[0-9]+)?)(?:[-+].*)?$`)

// inferMinimumRHOAIVersion uses the controller release, not the runtime version.
// EA/build qualifiers and patch numbers do not affect the major.minor default.
// Digest-only references or non-release tags do not establish a minimum.
func inferMinimumRHOAIVersion(targetImage string) string {
	image, err := reference.ParseNormalizedNamed(targetImage)
	if err != nil {
		return ""
	}
	tagged, ok := image.(reference.Tagged)
	if !ok {
		return ""
	}
	parts := rhoaiReleaseTag.FindStringSubmatch(strings.TrimPrefix(tagged.Tag(), "rhoai-"))
	if len(parts) != 2 {
		return ""
	}
	release := parts[1]
	if strings.Count(release, ".") == 1 {
		release += ".0"
	}
	version, err := semver.StrictNewVersion(release)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d.%d", version.Major(), version.Minor())
}

func validateMinimumRHOAIVersion(value string) error {
	if value == "" {
		return nil
	}
	parts := rhoaiReleaseTag.FindStringSubmatch(value)
	if len(parts) != 2 {
		return fmt.Errorf("minimumRHOAIVersion %q must be a release version, such as 3.6", value)
	}
	if _, err := semver.NewVersion(value); err != nil {
		return fmt.Errorf("invalid minimumRHOAIVersion %q: %w", value, err)
	}
	return nil
}
