package catalog

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sbomTestEnvelope(t *testing.T, digest, predicateType string, packages []map[string]any) []byte {
	t.Helper()
	statement, err := json.Marshal(map[string]any{
		"_type": "https://in-toto.io/Statement/v0.1", "predicateType": predicateType,
		"subject":   []any{map[string]any{"digest": map[string]string{"sha256": strings.TrimPrefix(digest, "sha256:")}}},
		"predicate": map[string]any{"packages": packages},
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(map[string]string{"payload": base64.StdEncoding.EncodeToString(statement)})
	if err != nil {
		t.Fatal(err)
	}
	return append(envelope, '\n')
}

func TestVLLMVersionFromAttestations(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	other := "sha256:" + strings.Repeat("b", 64)
	spdx := "https://spdx.dev/Document"
	vllm := func(version string) []map[string]any {
		return []map[string]any{{"name": "vllm", "versionInfo": version}}
	}
	valid := sbomTestEnvelope(t, digest, spdx, vllm("0.26.0+rhaiv.8"))
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"matching SPDX", valid, "0.26.0+rhaiv.8"},
		{"ignore provenance", append(sbomTestEnvelope(t, digest, "https://slsa.dev/provenance/v0.2", vllm("wrong")), valid...), "0.26.0+rhaiv.8"},
		{"ignore different subject", append(sbomTestEnvelope(t, other, spdx, vllm("wrong")), valid...), "0.26.0+rhaiv.8"},
		{"wrong subject only", sbomTestEnvelope(t, other, spdx, vllm("wrong")), ""},
		{"conflicting versions", append(valid, sbomTestEnvelope(t, digest, spdx, vllm("0.25.0"))...), ""},
		{"invalid payload", []byte(`{"payload":"invalid"}`), ""},
		{"empty download", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			version, _, err := vllmVersionFromAttestations(tc.data, digest)
			if tc.want == "" {
				if err == nil {
					t.Fatal("invalid attestation accepted")
				}
			} else if err != nil || version != tc.want {
				t.Fatalf("got %q, %v; want %q", version, err, tc.want)
			}
		})
	}
}

func TestResolveSBOMParametersArchitecturesAndCache(t *testing.T) {
	repository := "registry.redhat.io/rhaii/vllm-cuda-rhel9@"
	index := "sha256:" + strings.Repeat("a", 64)
	amd64 := "sha256:" + strings.Repeat("b", 64)
	arm64 := "sha256:" + strings.Repeat("c", 64)
	packages := []map[string]any{}
	for _, child := range []struct{ digest, arch string }{{amd64, "amd64"}, {arm64, "arm64"}} {
		packages = append(packages, map[string]any{
			"name":         "architecture",
			"externalRefs": []map[string]string{{"referenceType": "purl", "referenceLocator": "pkg:oci/vllm@" + child.digest + "?arch=" + child.arch + "&repository_url=quay.io/private/build"}},
		})
	}
	data := map[string][]byte{
		repository + index: sbomTestEnvelope(t, index, "https://spdx.dev/Document", packages),
		repository + amd64: sbomTestEnvelope(t, amd64, "https://spdx.dev/Document", []map[string]any{{"name": "vllm", "versionInfo": "0.26.0+rhaiv.8"}}),
		repository + arm64: sbomTestEnvelope(t, arm64, "https://spdx.dev/Document", []map[string]any{{"name": "vllm", "versionInfo": "0.26.0+rhaiv.8"}}),
	}
	resolver := NewRuntimeSBOMVersionResolver("")
	resolver.inspect = func(context.Context, string) (string, error) {
		return index, nil
	}
	calls := 0
	resolver.download = func(_ context.Context, _, image string) ([]byte, error) {
		calls++
		value, ok := data[image]
		if !ok {
			return nil, fmt.Errorf("unexpected image %s", image)
		}
		return value, nil
	}
	for _, key := range []string{"kserve-llm-d-nvidia-cuda", "vllm-cuda-image"} {
		source := RuntimeSourceConfig{ID: key, Parameters: map[string]string{key: repository + index}}
		if err := resolver.ResolveParameters(context.Background(), &source); err != nil {
			t.Fatal(err)
		}
		if source.Parameters[key+"-upstream-version"] != "0.26.0+rhaiv.8" {
			t.Fatal("upstream-version parameter not populated")
		}
	}
	if calls != 3 {
		t.Fatalf("shared image not cached: %d downloads", calls)
	}
	data[repository+arm64] = sbomTestEnvelope(t, arm64, "https://spdx.dev/Document", []map[string]any{{"name": "vllm", "versionInfo": "0.25.0"}})
	resolver.versions = map[string]string{}
	if _, err := resolver.resolve(context.Background(), repository+index, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "disagree") {
		t.Fatalf("architecture version conflict accepted: %v", err)
	}
}

func TestResolveSBOMParametersExplicitVersionsAndFailures(t *testing.T) {
	image := "registry.redhat.io/rhaii-fast/vllm-cuda-rhel9@sha256:" + strings.Repeat("a", 64)
	resolver := NewRuntimeSBOMVersionResolver("")
	resolver.inspect = func(context.Context, string) (string, error) {
		return "sha256:" + strings.Repeat("a", 64), nil
	}
	resolver.download = func(context.Context, string, string) ([]byte, error) {
		return nil, fmt.Errorf("SBOM attestation unavailable")
	}
	source := RuntimeSourceConfig{ID: "controller", Parameters: map[string]string{
		"vllm-image": image, "vllm-image-upstream-version": "explicit",
		"ovms-image": "registry.redhat.io/rhoai/odh-openvino-model-server-rhel9@sha256:" + strings.Repeat("b", 64),
	}}
	if err := resolver.ResolveParameters(context.Background(), &source); err != nil {
		t.Fatal(err)
	}
	delete(source.Parameters, "vllm-image-upstream-version")
	if err := resolver.ResolveParameters(context.Background(), &source); err == nil || !strings.Contains(err.Error(), "parameters.vllm-image-upstream-version") {
		t.Fatalf("missing attestation does not explain override: %v", err)
	}
	if _, err := resolver.resolve(context.Background(), image, map[string]bool{image: true}); err == nil {
		t.Fatal("cyclic architecture reference accepted")
	}
}

func TestResolveFloatingParameterImagesOnce(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	image := "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6"
	other := "quay.io/rhoai/odh-openvino-model-server-rhel9:rhoai-3.6"
	pinned := "registry.example.com/runtime@sha256:" + strings.Repeat("b", 64)
	resolver := NewRuntimeSBOMVersionResolver("")
	calls := map[string]int{}
	resolver.inspect = func(_ context.Context, value string) (string, error) {
		calls[value]++
		return digest, nil
	}
	resolver.download = func(context.Context, string, string) ([]byte, error) {
		t.Fatal("explicit version did not bypass SBOM discovery")
		return nil, nil
	}
	for _, id := range []string{"llmisvc", "model-controller"} {
		source := RuntimeSourceConfig{ID: id, Parameters: map[string]string{
			"runtime-image": image, "runtime-image-upstream-version": "0.26.0+rhaiv.8",
			"ovms-image": other, "ovms-alias": other, "pinned-image": pinned,
			"flag": "true", "path": "/opt/app-root/models", "url": "https://example.com/docs",
		}, Replacements: map[string]string{"placeholder": image}, Templates: map[string]RuntimeTemplateConfig{
			"runtime.yaml": {Image: image, Replacements: map[string]string{"placeholder": image}},
		}}
		if err := resolver.ResolveParameters(context.Background(), &source); err != nil {
			t.Fatal(err)
		}
		if source.Parameters["runtime-image"] != image+"@"+digest || source.Parameters["ovms-image"] != other+"@"+digest || source.Parameters["ovms-alias"] != other+"@"+digest {
			t.Fatalf("floating tags were not frozen consistently: %+v", source.Parameters)
		}
		if source.Parameters["pinned-image"] != pinned || source.Parameters["runtime-image-upstream-version"] != "0.26.0+rhaiv.8" || source.Parameters["url"] != "https://example.com/docs" {
			t.Fatal("digest pins or non-image parameters changed")
		}
		override := source.Templates["runtime.yaml"]
		if source.Replacements["placeholder"] != image+"@"+digest || override.Image != image+"@"+digest || override.Replacements["placeholder"] != image+"@"+digest {
			t.Fatal("image overrides no longer match the resolved parameter image")
		}
	}
	if len(calls) != 3 || calls[image] != 1 || calls[other] != 1 || calls[pinned] != 1 {
		t.Fatalf("tags were not cached across keys and sources: %+v", calls)
	}
}

func TestMissingParameterImagesAreCachedWithoutSBOMDiscovery(t *testing.T) {
	image := "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6"
	pinned := "registry.example.com/runtime@sha256:" + strings.Repeat("b", 64)
	resolver := NewRuntimeSBOMVersionResolver("")
	calls := map[string]int{}
	resolver.inspect = func(_ context.Context, image string) (string, error) {
		calls[image]++
		return "", fmt.Errorf("reading manifest: manifest unknown")
	}
	resolver.download = func(context.Context, string, string) ([]byte, error) {
		t.Fatal("unpublished image queried for SBOMs")
		return nil, nil
	}
	for _, id := range []string{"llmisvc", "model-controller"} {
		source := RuntimeSourceConfig{ID: id, Parameters: map[string]string{
			"runtime-image": image, "alias-image": image, "pinned-image": pinned,
			"flag": "true", "runtime-image-upstream-version": "explicit",
		}}
		if err := resolver.ResolveParameters(context.Background(), &source); err != nil {
			t.Fatal(err)
		}
		if !source.MissingImages[image] || !source.MissingImages[pinned] || len(source.MissingImages) != 2 {
			t.Fatalf("missing images were not recorded: %+v", source.MissingImages)
		}
		if source.Parameters["runtime-image"] != image || source.Parameters["pinned-image"] != pinned {
			t.Fatal("missing image parameters changed")
		}
		delete(source.Parameters, "runtime-image-upstream-version")
		if err := resolver.ResolveParameters(context.Background(), &source); err != nil {
			t.Fatal(err)
		}
	}
	if calls[image] != 1 || calls[pinned] != 1 || len(calls) != 2 {
		t.Fatalf("missing images were not cached across parameters and sources: %+v", calls)
	}
}

func TestRuntimeImageMissingRejectsRegistryFailures(t *testing.T) {
	for message, want := range map[string]bool{
		"reading manifest 3.6: manifest unknown": true,
		"MANIFEST_UNKNOWN: tag missing":          true,
		"NAME_UNKNOWN: repository missing":       true,
		"name unknown":                           true,
		"unauthorized: manifest unknown":         false,
		"authentication required":                false,
		"denied: access forbidden":               false,
		"registry unavailable":                   false,
		"unexpected status 404 Not Found":        false,
		"dial tcp: connection refused":           false,
		"context deadline exceeded":              false,
	} {
		if got := runtimeImageMissing(fmt.Errorf("%s", message)); got != want {
			t.Errorf("%q: missing=%t, want %t", message, got, want)
		}
	}
}

func TestSkipUnavailableParameterImagesIsExplicit(t *testing.T) {
	image := "registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6"
	for _, tc := range []struct {
		name        string
		err         error
		defaultSkip bool
		flagSkip    bool
	}{
		{"OCI missing manifest", fmt.Errorf("manifest unknown"), true, true},
		{"unauthorized", fmt.Errorf("unauthorized: access to the requested resource is not authorized"), false, true},
		{"denied", fmt.Errorf("denied: requested access to the resource is denied"), false, true},
		{"authentication", fmt.Errorf("authentication required"), false, true},
		{"HTTP 401", fmt.Errorf("unexpected status code 401"), false, true},
		{"HTTP 403", fmt.Errorf("unexpected HTTP status: 403"), false, true},
		{"HTTP 404", fmt.Errorf("unexpected status 404 Not Found"), false, true},
		{"image not found", fmt.Errorf("image not found"), false, true},
		{"network", fmt.Errorf("dial tcp: connection refused"), false, false},
		{"TLS", fmt.Errorf("x509: certificate signed by unknown authority"), false, false},
		{"server error", fmt.Errorf("unexpected status 500"), false, false},
		{"missing tool", fmt.Errorf("exec: skopeo: executable file not found in PATH"), false, false},
		{"auth file unreadable", fmt.Errorf("open auth.json: permission denied"), false, false},
		{"canceled", fmt.Errorf("manifest unknown: %w", context.Canceled), false, false},
		{"deadline", fmt.Errorf("unauthorized: %w", context.DeadlineExceeded), false, false},
	} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/flag=%t", tc.name, enabled), func(t *testing.T) {
				resolver := NewRuntimeSBOMVersionResolver("")
				resolver.SkipUnavailableImages = enabled
				calls := 0
				resolver.inspect = func(context.Context, string) (string, error) {
					calls++
					return "", tc.err
				}
				resolver.download = func(context.Context, string, string) ([]byte, error) {
					t.Fatal("inaccessible parameter image queried for SBOM")
					return nil, nil
				}
				source := RuntimeSourceConfig{ID: "source", Parameters: map[string]string{"vllm-image": image}}
				wantSkip := tc.defaultSkip || (enabled && tc.flagSkip)
				err := resolver.ResolveParameters(context.Background(), &source)
				if wantSkip {
					if err != nil || !source.MissingImages[image] {
						t.Fatalf("unavailable image not skipped: %v, %+v", err, source.MissingImages)
					}
					if err := resolver.ResolveParameters(context.Background(), &source); err != nil || calls != 1 {
						t.Fatalf("unavailability not cached: %v, %d checks", err, calls)
					}
				} else if err == nil || len(source.MissingImages) != 0 {
					t.Fatalf("registry failure was suppressed: %v, %+v", err, source.MissingImages)
				}
				if source.Parameters["vllm-image"] != image || source.Parameters["vllm-image-upstream-version"] != "" {
					t.Fatal("unresolved image or version changed")
				}
			})
		}
	}
}

func TestFloatingVLLMImageMatchesDiscoveredSBOM(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	image := "registry.redhat.io/rhaii/vllm-cuda-rhel9:latest"
	resolver := NewRuntimeSBOMVersionResolver("")
	resolver.inspect = func(_ context.Context, value string) (string, error) {
		if value != image {
			t.Fatalf("unexpected tag inspection: %s", value)
		}
		return digest, nil
	}
	resolver.download = func(_ context.Context, _, value string) ([]byte, error) {
		if value != "registry.redhat.io/rhaii/vllm-cuda-rhel9@"+digest {
			t.Fatalf("SBOM download did not use the rendered image digest: %s", value)
		}
		return sbomTestEnvelope(t, digest, "https://spdx.dev/Document", []map[string]any{{"name": "vllm", "versionInfo": "0.26.0+rhaiv.8"}}), nil
	}
	source := RuntimeSourceConfig{ID: "llmisvc", Parameters: map[string]string{"vllm-image": image}}
	if err := resolver.ResolveParameters(context.Background(), &source); err != nil {
		t.Fatal(err)
	}
	if source.Parameters["vllm-image"] != image+"@"+digest || source.Parameters["vllm-image-upstream-version"] != "0.26.0+rhaiv.8" {
		t.Fatalf("image and version were not resolved together: %+v", source.Parameters)
	}
	if err := validateRuntimeImage(source.Parameters["vllm-image"]); err != nil {
		t.Fatal(err)
	}
}

func TestFloatingImageResolutionFailuresLeaveParametersUnchanged(t *testing.T) {
	image := "quay.io/rhoai/runtime:rhoai-3.6"
	for name, inspect := range map[string]func(context.Context, string) (string, error){
		"registry failure": func(context.Context, string) (string, error) { return "", fmt.Errorf("registry unavailable") },
		"bad digest":       func(context.Context, string) (string, error) { return "invalid", nil },
	} {
		t.Run(name, func(t *testing.T) {
			resolver := NewRuntimeSBOMVersionResolver("")
			resolver.inspect = inspect
			source := RuntimeSourceConfig{ID: "controller", Parameters: map[string]string{"runtime-image": image}}
			if err := resolver.ResolveParameters(context.Background(), &source); err == nil {
				t.Fatal("invalid image resolution succeeded")
			}
			if source.Parameters["runtime-image"] != image {
				t.Fatal("failed resolution changed parameters")
			}
		})
	}
}

func TestParameterPreflightUsesRawIndexWithoutArchitectureSelection(t *testing.T) {
	root := t.TempDir()
	script := `#!/bin/sh
case "$*" in
  *'inspect --raw docker://registry.example.com/runtime:3.6') printf '%s' "$REGISTRY_TEST_MANIFEST" ;;
  *) printf '%s' 'metadata inspection would require AMD64' >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "skopeo"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	// An index containing only ARM64 must be usable on an AMD64 generator host.
	index := `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","platform":{"os":"linux","architecture":"arm64"}}]}`
	t.Setenv("REGISTRY_TEST_MANIFEST", index)
	image := "registry.example.com/runtime:3.6"
	source := RuntimeSourceConfig{ID: "controller", Parameters: map[string]string{"runtime-image": image}}
	if err := NewRuntimeSBOMVersionResolver("").ResolveParameters(context.Background(), &source); err != nil {
		t.Fatal(err)
	}
	if got, want := source.Parameters["runtime-image"], image+"@sha256:eba5781392d6e8c143a59e60c761de2b0b8d46ef445ed742a2a662cbac7aeab7"; got != want {
		t.Fatalf("did not hash the exact top-level index: got %s, want %s", got, want)
	}
	for _, malformed := range []string{"{", `{"Digest":"sha256:wrong"}`, `{"schemaVersion":1}`} {
		t.Setenv("REGISTRY_TEST_MANIFEST", malformed)
		source := RuntimeSourceConfig{ID: "controller", Parameters: map[string]string{"runtime-image": image}}
		if err := NewRuntimeSBOMVersionResolver("").ResolveParameters(context.Background(), &source); err == nil {
			t.Fatalf("malformed image inspection accepted: %s", malformed)
		}
		if source.Parameters["runtime-image"] != image || len(source.MissingImages) != 0 {
			t.Fatal("malformed inspection changed parameters or was classified as absent")
		}
	}
}

func TestDownloadAttestationsUsesPrivateAuthConfig(t *testing.T) {
	dir := t.TempDir()
	authFile := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(authFile, []byte(`{"auths":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cosign := `#!/bin/sh
test "$1 $2" = 'download attestation' || exit 1
test -f "$DOCKER_CONFIG/config.json" || exit 2
test "$(stat -c %a "$DOCKER_CONFIG")" = 700 || exit 3
test "$(stat -c %a "$DOCKER_CONFIG/config.json")" = 600 || exit 4
printf '%s' "$DOCKER_CONFIG"
`
	if err := os.WriteFile(filepath.Join(dir, "cosign"), []byte(cosign), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	output, err := downloadRuntimeAttestations(context.Background(), authFile, "registry.example.com/image@sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(output)); !os.IsNotExist(err) {
		t.Fatal("temporary credential directory retained")
	}
}
