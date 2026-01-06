package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/containers/image/v5/docker"
	containertypes "github.com/containers/image/v5/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// HTTP client with timeout for registry API calls
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// RegistryManifest represents container registry manifest metadata
type RegistryManifest struct {
	Config struct {
		Created string `json:"created"`
	} `json:"config"`
	History []struct {
		Created string `json:"created"`
	} `json:"history"`
	Annotations map[string]string `json:"annotations"`
}

// parseRegistryImageRef extracts registry, repository, image name and tag from a registry reference
func parseRegistryImageRef(imageRef string) (registry, repository, imageName, tag string, err error) {
	parts := strings.Split(imageRef, "/")
	if len(parts) < 3 {
		return "", "", "", "", fmt.Errorf("invalid image reference format")
	}

	registry = parts[0]
	repository = parts[1]

	// Handle image name and tag
	imageWithTag := strings.Join(parts[2:], "/")
	if idx := strings.LastIndex(imageWithTag, ":"); idx != -1 {
		imageName = imageWithTag[:idx]
		tag = imageWithTag[idx+1:]
	} else {
		imageName = imageWithTag
		tag = "latest"
	}

	return registry, repository, imageName, tag, nil
}

// manifestListEntry represents an entry in a Docker/OCI manifest list
type manifestListEntry struct {
	Platform struct {
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
		Variant      string `json:"variant,omitempty"`
	} `json:"platform"`
}

// manifestListSchema represents a Docker/OCI manifest list
type manifestListSchema struct {
	SchemaVersion int                 `json:"schemaVersion"`
	MediaType     string              `json:"mediaType"`
	Manifests     []manifestListEntry `json:"manifests"`
}

// fetchImageArchitectures inspects an OCI image reference and returns all supported architectures
func fetchImageArchitectures(imageRef string) ([]string, error) {
	// Parse the image reference
	ref, err := docker.ParseReference("//" + imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %v", err)
	}

	// Create a system context
	sys := &containertypes.SystemContext{}

	// Create a context with timeout for registry operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create an image source to access the raw manifest
	src, err := ref.NewImageSource(ctx, sys)
	if err != nil {
		return nil, fmt.Errorf("failed to create image source: %v", err)
	}
	defer func() { _ = src.Close() }()

	// Get the raw manifest
	manifestBytes, manifestMIMEType, err := src.GetManifest(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %v", err)
	}

	var architectures []string

	// Check if it's a manifest list (indicated by specific MIME types)
	isManifestList := strings.Contains(manifestMIMEType, "manifest.list") ||
		strings.Contains(manifestMIMEType, "image.index")

	if isManifestList {
		// Parse as manifest list to extract platform information
		var manifestList manifestListSchema
		if err := json.Unmarshal(manifestBytes, &manifestList); err != nil {
			return nil, fmt.Errorf("failed to parse manifest list: %v", err)
		}

		// Collect unique architectures
		archSet := make(map[string]bool)
		for _, manifestEntry := range manifestList.Manifests {
			if manifestEntry.Platform.Architecture != "" {
				archSet[manifestEntry.Platform.Architecture] = true
			}
		}

		// Convert set to sorted slice for consistent output
		for arch := range archSet {
			architectures = append(architectures, arch)
		}
		sort.Strings(architectures)
	} else {
		// Single-arch image - need to get architecture from config
		img, err := ref.NewImage(ctx, sys)
		if err != nil {
			return nil, fmt.Errorf("failed to create image: %v", err)
		}
		defer func() { _ = img.Close() }()

		configBlob, err := img.ConfigBlob(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get config blob: %v", err)
		}

		// Parse config to extract architecture
		var config struct {
			Architecture string `json:"architecture"`
		}
		if err := json.Unmarshal(configBlob, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config: %v", err)
		}

		if config.Architecture != "" {
			architectures = []string{config.Architecture}
		}
	}

	if len(architectures) == 0 {
		return nil, fmt.Errorf("no architectures found in manifest")
	}

	return architectures, nil
}

// AddArchitectureToArtifactProps fetches architectures and adds them to artifact custom properties (exported)
// Returns true if architecture was successfully added, false otherwise.
func AddArchitectureToArtifactProps(imageRef string, customProps map[string]interface{}) bool {
	return addArchitectureToCustomProps(imageRef, customProps)
}

// addArchitectureToCustomProps fetches architectures and adds them to custom properties
// Returns true if architecture was successfully added, false otherwise.
func addArchitectureToCustomProps(imageRef string, customProps map[string]interface{}) bool {
	// Fetch architectures from the image
	architectures, err := fetchImageArchitectures(imageRef)
	if err != nil {
		log.Printf("Warning: Failed to fetch architectures for %s: %v", imageRef, err)
		return false
	}

	// Marshal architectures to JSON array format
	archJSON, err := json.Marshal(architectures)
	if err != nil {
		log.Printf("Warning: Failed to marshal architectures for %s: %v", imageRef, err)
		return false
	}

	// Add architecture to custom properties in the required format
	customProps["architecture"] = map[string]interface{}{
		"metadataType": "MetadataStringValue",
		"string_value": string(archJSON),
	}
	return true
}

// FetchRegistryMetadata fetches OCI artifact metadata from registry API
func FetchRegistryMetadata(imageRef string) (*types.OCIArtifact, error) {
	registry, repository, imageName, tag, err := parseRegistryImageRef(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %v", err)
	}

	// Create OCI URI format
	ociURI := fmt.Sprintf("oci://%s/%s/%s:%s", registry, repository, imageName, tag)

	// For Red Hat registry, we can try to fetch manifest metadata
	// This is a simplified implementation - in production you'd need proper authentication
	if strings.Contains(registry, "registry.redhat.io") {
		// Try to fetch manifest via registry API v2
		manifestURL := fmt.Sprintf("https://%s/v2/%s/%s/manifests/%s", registry, repository, imageName, tag)

		resp, err := httpClient.Get(manifestURL)
		if err != nil {
			// If we can't fetch from API, create artifact with nil timestamps
			customProps := map[string]interface{}{
				"source": map[string]interface{}{
					"string_value": "registry.redhat.io",
				},
				"type": map[string]interface{}{
					"string_value": "modelcar",
				},
			}
			// Add architecture information
			addArchitectureToCustomProps(imageRef, customProps)

			return &types.OCIArtifact{
				URI:                      ociURI,
				CreateTimeSinceEpoch:     nil,
				LastUpdateTimeSinceEpoch: nil,
				CustomProperties:         customProps,
			}, nil
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == 200 {
			// Parse manifest to extract timestamps
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				var manifest RegistryManifest
				if json.Unmarshal(body, &manifest) == nil {
					// Convert timestamps to epoch milliseconds
					createTime := utils.ParseTimeToEpochInt64(manifest.Config.Created)

					// Use the most recent history entry for update time
					updateTime := createTime
					if len(manifest.History) > 0 {
						lastHistoryTime := utils.ParseTimeToEpochInt64(manifest.History[len(manifest.History)-1].Created)
						if lastHistoryTime != nil {
							updateTime = lastHistoryTime
						}
					}

					customProps := map[string]interface{}{
						"source": map[string]interface{}{
							"string_value": "registry.redhat.io",
						},
						"type": map[string]interface{}{
							"string_value": "modelcar",
						},
					}

					// Add annotations as custom properties
					for key, value := range manifest.Annotations {
						customProps[key] = map[string]interface{}{
							"string_value": value,
						}
					}

					// Add architecture information
					addArchitectureToCustomProps(imageRef, customProps)

					return &types.OCIArtifact{
						URI:                      ociURI,
						CreateTimeSinceEpoch:     createTime,
						LastUpdateTimeSinceEpoch: updateTime,
						CustomProperties:         customProps,
					}, nil
				}
			}
		}
	}

	// Fallback: create artifact with nil timestamps when registry data unavailable
	customProps := map[string]interface{}{
		"source": map[string]interface{}{
			"string_value": registry,
		},
		"type": map[string]interface{}{
			"string_value": "modelcar",
		},
	}
	// Add architecture information
	addArchitectureToCustomProps(imageRef, customProps)

	return &types.OCIArtifact{
		URI:                      ociURI,
		CreateTimeSinceEpoch:     nil,
		LastUpdateTimeSinceEpoch: nil,
		CustomProperties:         customProps,
	}, nil
}

// ExtractOCIArtifactsFromRegistry creates structured OCI artifacts from registry references
func ExtractOCIArtifactsFromRegistry(manifestRef string) []types.OCIArtifact {
	var artifacts []types.OCIArtifact

	// The manifestRef itself is the primary OCI artifact
	if artifact, err := FetchRegistryMetadata(manifestRef); err == nil {
		artifacts = append(artifacts, *artifact)
	} else {
		log.Printf("Warning: Failed to fetch registry metadata for %s: %v", manifestRef, err)
		// Create basic artifact anyway with nil timestamps
		registry, repository, imageName, tag, parseErr := parseRegistryImageRef(manifestRef)
		if parseErr == nil {
			ociURI := fmt.Sprintf("oci://%s/%s/%s:%s", registry, repository, imageName, tag)
			artifacts = append(artifacts, types.OCIArtifact{
				URI:                      ociURI,
				CreateTimeSinceEpoch:     nil,
				LastUpdateTimeSinceEpoch: nil,
				CustomProperties: map[string]interface{}{
					"source": map[string]interface{}{
						"string_value": "unknown",
					},
					"error": map[string]interface{}{
						"string_value": err.Error(),
					},
				},
			})
		}
	}

	// Ensure we never return nil - always return at least an empty slice
	if artifacts == nil {
		artifacts = []types.OCIArtifact{}
	}
	return artifacts
}
