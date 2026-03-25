# registry

The `registry` package handles container registry operations for OCI/Docker image access.

## Responsibilities

- Fetching OCI manifests from container registries
- Extracting layer information and annotations from manifests
- Parsing Docker/OCI image references into components (registry, repository, tag)
- Retrieving registry-level metadata (tags, creation dates)
- Providing manifest and layer data to the extraction pipeline

## Key Functions

- `FetchRegistryMetadata()` - Fetches registry-level metadata (tags, creation dates) for an image
- `AddArchitectureToArtifactProps()` - Adds architecture info to OCI artifact properties
- `ExtractOCIArtifactsFromRegistry()` - Extracts OCI artifact metadata from a manifest reference

## Dependencies

- `github.com/containers/image/v5` - OCI container image library
