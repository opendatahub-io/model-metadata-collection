package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/distribution/reference"
)

// RuntimeSBOMVersionResolver discovers upstream vLLM versions from SPDX SBOM
// attestations. Downloads and subject checks do not verify publisher signatures.
// One resolver caches tag resolutions and SBOMs across all controller sources.
type RuntimeSBOMVersionResolver struct {
	// SkipUnavailableImages also omits images when registry access is denied or
	// inspection returns HTTP 401/403/404. Transport and metadata errors remain fatal.
	SkipUnavailableImages bool
	authFile              string
	versions              map[string]string
	images                map[string]string
	missing               map[string]bool
	inspect               func(context.Context, string) (string, error)
	download              func(context.Context, string, string) ([]byte, error)
}

func NewRuntimeSBOMVersionResolver(authFile string) *RuntimeSBOMVersionResolver {
	return &RuntimeSBOMVersionResolver{
		authFile: authFile, versions: map[string]string{}, download: downloadRuntimeAttestations,
		images: map[string]string{}, missing: map[string]bool{},
		inspect: func(ctx context.Context, image string) (string, error) {
			return inspectRuntimeParameterManifest(ctx, authFile, image)
		},
	}
}

// ResolveParameters checks parameter images, freezes available tags, and fills
// missing <image-key>-upstream-version values. Explicit versions bypass SBOM
// discovery but still resolve image tags, so metadata and templates use one image.
func (r *RuntimeSBOMVersionResolver) ResolveParameters(ctx context.Context, source *RuntimeSourceConfig) error {
	if err := r.pinParameterImages(ctx, source); err != nil {
		return err
	}
	for _, key := range slices.Sorted(maps.Keys(source.Parameters)) {
		image := source.Parameters[key]
		if source.MissingImages[image] {
			continue
		}
		ref, err := reference.ParseNormalizedNamed(image)
		if err != nil || !isRedHatVLLMImage(ref) || source.Parameters[key+"-upstream-version"] != "" {
			continue
		}
		version, err := r.resolve(ctx, image, map[string]bool{})
		if err != nil {
			return fmt.Errorf("source %q: discover vLLM version for %s (or configure parameters.%s-upstream-version): %w", source.ID, key, key, err)
		}
		source.Parameters[key+"-upstream-version"] = version
	}
	return nil
}

func (r *RuntimeSBOMVersionResolver) pinParameterImages(ctx context.Context, source *RuntimeSourceConfig) error {
	parameters := maps.Clone(source.Parameters)
	resolved := map[string]string{}
	missing := map[string]bool{}
	for _, key := range slices.Sorted(maps.Keys(parameters)) {
		image := parameters[key]
		// params.env also contains versions, flags, and other non-image values.
		if !strings.Contains(strings.Split(image, "/")[0], ".") || !strings.Contains(image, "/") {
			continue
		}
		ref, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			continue
		}
		if reference.IsNameOnly(ref) {
			return fmt.Errorf("source %q parameter %s: image %q requires an explicit tag or digest", source.ID, key, image)
		}
		image = ref.String()
		if r.missing[image] {
			missing[parameters[key]] = true
			continue
		}
		pinned := r.images[image]
		if pinned == "" {
			digest, err := r.inspect(ctx, image)
			if err != nil {
				if ctx.Err() == nil && r.canSkipImageError(err) {
					r.missing[image] = true
					missing[parameters[key]] = true
					continue
				}
				return fmt.Errorf("source %q: resolve image parameter %s: %w", source.ID, key, err)
			}
			if !sourceBlobPattern.MatchString(digest) {
				return fmt.Errorf("source %q parameter %s: invalid runtime image digest %q", source.ID, key, digest)
			}
			// Keep the tag for release/support inference; the digest fixes the image
			// content, including the top-level manifest of multi-architecture images.
			pinned = image
			if _, hasDigest := ref.(reference.Digested); !hasDigest {
				pinned += "@" + digest
			}
			r.images[image] = pinned
			r.images[pinned] = pinned
		}
		resolved[parameters[key]] = pinned
		parameters[key] = pinned
	}
	source.Parameters = parameters
	source.MissingImages = missing
	// YAML aliases may also reuse a parameter image in explicit image overrides
	// or substitutions. Keep those references aligned with Kustomize's image.
	source.Replacements = replaceResolvedRuntimeImages(source.Replacements, resolved)
	templates := maps.Clone(source.Templates)
	for file, template := range templates {
		if image := resolved[template.Image]; image != "" {
			template.Image = image
		}
		template.Replacements = replaceResolvedRuntimeImages(template.Replacements, resolved)
		templates[file] = template
	}
	source.Templates = templates
	return nil
}

var runtimeUnavailableHTTPStatus = regexp.MustCompile(`(?i)\bstatus(?:\s+code)?\s*:?\s*(401|403|404)\b`)

func (r *RuntimeSBOMVersionResolver) canSkipImageError(err error) bool {
	return runtimeImageUnavailable(err, r.SkipUnavailableImages)
}

func runtimeImageUnavailable(err error, includeDenied bool) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if runtimeImageMissing(err) {
		return true
	}
	if !includeDenied {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, unavailable := range []string{
		"unauthorized", "authentication required", "forbidden",
		"denied: requested access", "denied: access", "requested access to the resource is denied",
		"manifest not found", "image not found", "repository not found", "404 not found",
	} {
		if strings.Contains(message, unavailable) {
			return true
		}
	}
	return runtimeUnavailableHTTPStatus.MatchString(message)
}

func inspectRuntimeParameterManifest(ctx context.Context, authFile, image string) (string, error) {
	// Raw inspection avoids selecting a host architecture or downloading config
	// blobs. Hash the exact top-level manifest bytes, preserving image indexes.
	data, err := runRuntimeSkopeo(ctx, authFile, "inspect", "--raw", "docker://"+image)
	if err != nil {
		return "", err
	}
	var manifest struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("decode runtime image manifest: %w", err)
	}
	if manifest.SchemaVersion != 2 {
		return "", fmt.Errorf("runtime image manifest requires OCI/Docker schema version 2")
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data)), nil
}

// Skopeo renders the OCI distribution errors in stderr. Only explicit absence
// permits omission; authentication, transport and inspection failures are fatal.
func runtimeImageMissing(err error) bool {
	message := strings.ToLower(err.Error())
	for _, fatal := range []string{"unauthorized", "authentication", "denied", "forbidden"} {
		if strings.Contains(message, fatal) {
			return false
		}
	}
	for _, absent := range []string{"manifest unknown", "manifest_unknown", "name unknown", "name_unknown"} {
		if strings.Contains(message, absent) {
			return true
		}
	}
	return false
}

func replaceResolvedRuntimeImages(values, resolved map[string]string) map[string]string {
	result := maps.Clone(values)
	for key, value := range result {
		if image := resolved[value]; image != "" {
			result[key] = image
		}
	}
	return result
}

func isRedHatVLLMImage(ref reference.Named) bool {
	if reference.Domain(ref) != "registry.redhat.io" {
		return false
	}
	name := path.Base(reference.Path(ref))
	return strings.HasPrefix(name, "vllm-") || strings.HasPrefix(name, "odh-vllm-")
}

func (r *RuntimeSBOMVersionResolver) resolve(ctx context.Context, image string, visiting map[string]bool) (string, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", err
	}
	pinned, ok := ref.(reference.Digested)
	if !ok {
		return "", fmt.Errorf("SBOM discovery requires an image digest")
	}
	image = reference.TrimNamed(ref).Name() + "@" + pinned.Digest().String()
	if version := r.versions[image]; version != "" {
		return version, nil
	}
	if visiting[image] || len(visiting) >= 4 {
		return "", fmt.Errorf("cyclic or overly nested architecture SBOM references")
	}
	visiting[image] = true
	defer delete(visiting, image)
	data, err := r.download(ctx, r.authFile, image)
	if err != nil {
		return "", err
	}
	version, children, err := vllmVersionFromAttestations(data, pinned.Digest().String())
	if err != nil {
		return "", err
	}
	if version == "" {
		for _, digest := range children {
			candidate, err := r.resolve(ctx, reference.TrimNamed(ref).Name()+"@"+digest, visiting)
			if err != nil {
				return "", err
			}
			if version != "" && version != candidate {
				return "", fmt.Errorf("architecture SBOMs disagree on vLLM version: %s and %s", version, candidate)
			}
			version = candidate
		}
	}
	if version == "" {
		return "", fmt.Errorf("SPDX SBOM has no vllm package or architecture references")
	}
	r.versions[image] = version
	return version, nil
}

type runtimeSPDX struct {
	Packages []struct {
		Name         string `json:"name"`
		Version      string `json:"versionInfo"`
		ExternalRefs []struct {
			Type    string `json:"referenceType"`
			Locator string `json:"referenceLocator"`
		} `json:"externalRefs"`
	} `json:"packages"`
}

func vllmVersionFromAttestations(data []byte, digest string) (string, []string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	version := ""
	children := map[string]bool{}
	matched := false
	for {
		var envelope struct {
			Payload string `json:"payload"`
		}
		if err := decoder.Decode(&envelope); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return "", nil, fmt.Errorf("parse attestation envelope: %w", err)
		}
		payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
		if err != nil {
			return "", nil, fmt.Errorf("decode attestation payload: %w", err)
		}
		var statement struct {
			Type          string `json:"_type"`
			PredicateType string `json:"predicateType"`
			Subject       []struct {
				Digest map[string]string `json:"digest"`
			} `json:"subject"`
			Predicate json.RawMessage `json:"predicate"`
		}
		if err := json.Unmarshal(payload, &statement); err != nil {
			return "", nil, fmt.Errorf("parse attestation statement: %w", err)
		}
		if statement.PredicateType != "https://spdx.dev/Document" {
			continue // Provenance and other attestations are not SBOMs.
		}
		if statement.Type != "https://in-toto.io/Statement/v0.1" && statement.Type != "https://in-toto.io/Statement/v1" {
			return "", nil, fmt.Errorf("unsupported SBOM statement type %q", statement.Type)
		}
		subjectMatches := false
		for _, subject := range statement.Subject {
			if subject.Digest["sha256"] == strings.TrimPrefix(digest, "sha256:") {
				subjectMatches = true
			}
		}
		if !subjectMatches {
			continue
		}
		matched = true
		var sbom runtimeSPDX
		if err := json.Unmarshal(statement.Predicate, &sbom); err != nil {
			return "", nil, fmt.Errorf("parse SPDX predicate: %w", err)
		}
		for _, pkg := range sbom.Packages {
			if pkg.Name == "vllm" && pkg.Version != "" {
				if version != "" && version != pkg.Version {
					return "", nil, fmt.Errorf("SBOMs disagree on vLLM version: %s and %s", version, pkg.Version)
				}
				version = pkg.Version
			}
			for _, ref := range pkg.ExternalRefs {
				if ref.Type != "purl" || !strings.HasPrefix(ref.Locator, "pkg:oci/") {
					continue
				}
				purl, err := url.Parse(ref.Locator)
				if err != nil || purl.Query().Get("arch") == "" {
					continue
				}
				_, child, ok := strings.Cut(purl.Opaque, "@")
				if ok && sourceBlobPattern.MatchString(child) && child != digest {
					children[child] = true
				}
			}
		}
	}
	if !matched {
		return "", nil, fmt.Errorf("no SPDX SBOM attestation matches image digest %s", digest)
	}
	return version, slices.Sorted(maps.Keys(children)), nil
}

func downloadRuntimeAttestations(ctx context.Context, authFile, image string) ([]byte, error) {
	var commandEnv []string
	if authFile != "" {
		// Cosign uses Docker's keychain rather than skopeo's --authfile flag.
		// Keep credentials in a private temporary directory, never command args.
		data, err := os.ReadFile(authFile)
		if err != nil {
			return nil, err
		}
		directory, err := os.MkdirTemp("", "serving-runtime-registry-auth-")
		if err != nil {
			return nil, err
		}
		defer func() { _ = os.RemoveAll(directory) }()
		if err := os.WriteFile(filepath.Join(directory, "config.json"), data, 0600); err != nil {
			return nil, err
		}
		commandEnv = append(os.Environ(), "DOCKER_CONFIG="+directory)
	}
	return retryRuntimeRegistryOperation(ctx, "cosign download attestation", func() ([]byte, error) {
		command := exec.CommandContext(ctx, "cosign", "download", "attestation", image)
		command.Env = commandEnv
		var stderr bytes.Buffer
		command.Stderr = &stderr
		data, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("cosign download attestation: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return data, nil
	}, waitRuntimeRegistryRetry)
}
