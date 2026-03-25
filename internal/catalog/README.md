# catalog

The `catalog` package handles model catalog generation and management.

## Responsibilities

- Loading static catalog files from YAML
- Merging extracted model metadata into a unified catalog
- Deduplicating catalog entries by model URI
- Writing the final `models-catalog.yaml` output
- Encoding/decoding base64 README content for catalog entries

## Key Functions

- `LoadStaticCatalogs()` - Loads and parses static catalog YAML files
- `CreateModelsCatalog()` - Creates catalog from processed model output directory
- `CreateModelsCatalogWithStatic()` - Creates catalog merging dynamic and static model entries
- `CreateModelsCatalogWithStaticFromResults()` - Creates catalog from explicit model refs and static entries
- `CreateMCPServersCatalog()` - Reads MCP servers index, loads input files, and writes aggregated MCP catalog
