# registry

The `registry` package handles container registry operations for OCI/Docker image access.

## Responsibilities

- Fetching OCI manifests from container registries
- Extracting layer information and annotations from manifests
- Parsing Docker/OCI image references into components (registry, repository, tag)
- Retrieving registry-level metadata (tags, creation dates)
- Providing manifest and layer data to the extraction pipeline

## Key Functions

- `FetchManifestSrcAndLayers()` - Fetches manifest metadata and layer info for an image
- `parseRegistryImageRef()` - Parses image references into structured components

## Dependencies

- `github.com/containers/image/v5` - OCI container image library
