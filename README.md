# Model Metadata Collection

A Go application that extracts, enriches, and catalogs metadata from Red Hat AI container images. The tool discovers models from HuggingFace collections, processes OCI container images, and generates structured catalogs.

## Features

- **HuggingFace Collections Integration**: Discovers and processes Red Hat AI validated model collections with version support (v1.0, v2.1, etc.)
- **OCI Container Analysis**: Extracts model cards from container image layers using annotation-based detection; creates skeleton metadata when extraction fails
- **Metadata Enrichment**: Enriches model metadata from HuggingFace, with modelcard.md data taking priority over external sources
- **Model Type Classification**: Classifies models as generative, predictive, or unknown with validation and configurable defaults
- **Automated Tagging**: Converts labels to tags and merges them from multiple sources without duplicates
- **Registry Integration**: Fetches OCI artifact metadata from container registries
- **Metadata Reporting**: Analyzes metadata completeness, data sources, and quality metrics
- **Static Catalog Support**: Merges static model catalogs with dynamically extracted metadata
- **Flexible CLI**: Supports configurable paths, output options, and per-component skip flags
- **Concurrent Processing**: Processes multiple models in parallel with configurable concurrency limits
- **Comprehensive Testing**: Includes unit tests for all major components
- **Structured Output**: Generates individual model metadata files and aggregated catalogs

## Architecture

The project is organized into modular packages:

```
├── cmd/
│   ├── model-extractor/          # Main CLI application for metadata extraction
│   └── metadata-report/          # CLI for generating metadata reports
├── internal/                     # Internal packages
│   ├── catalog/                  # Catalog generation services
│   ├── config/                   # Configuration management
│   ├── enrichment/               # Metadata enrichment services
│   ├── huggingface/             # HuggingFace API integration
│   ├── metadata/                # Metadata parsing and migration
│   ├── registry/                # Container registry services
│   └── report/                  # Metadata reporting and analysis
├── pkg/                         # Public packages
│   ├── types/                   # Shared type definitions
│   └── utils/                   # Utility functions
└── test/                        # Test files and test data
```

## Prerequisites

- Go 1.25 or later
- Access to container registries (registry.redhat.io)
- Internet access for HuggingFace API calls
- Docker (for containerized builds and deployment)

## Installation

### From Source

```bash
git clone https://github.com/opendatahub-io/model-metadata-collection.git
cd model-metadata-collection
make build
```

This creates binaries at:
- `build/model-extractor` - Main metadata extraction tool
- `build/metadata-report` - Metadata reporting and analysis tool

### Using Go Install

```bash
# Install the metadata extraction tool
go install github.com/opendatahub-io/model-metadata-collection/cmd/model-extractor@latest

# Install the metadata reporting tool
go install github.com/opendatahub-io/model-metadata-collection/cmd/metadata-report@latest
```

## Usage

### Basic Usage

Run with default settings (processes HuggingFace collections and falls back to `data/models-index.yaml`):

```bash
./build/model-extractor
```

### Custom Configuration

```bash
./build/model-extractor \
  --input custom-models.yaml \
  --output-dir /tmp/output \
  --catalog-output /tmp/catalog.yaml \
  --max-concurrent 10
```

### Metadata Reporting

Generate metadata completeness reports:

```bash
# Generate reports from existing output
./build/metadata-report --output-dir output --report-dir reports

# Use custom catalog file
./build/metadata-report \
  --catalog data/models-catalog.yaml \
  --output-dir output \
  --report-dir reports
```

### Skip Specific Processing Steps

```bash
# Skip HuggingFace processing and enrichment
./build/model-extractor --skip-huggingface --skip-enrichment

# Process only metadata extraction
./build/model-extractor --skip-huggingface --skip-enrichment --skip-catalog

# Include custom static catalog files
./build/model-extractor --static-catalog-files custom1.yaml,custom2.yaml

# Skip default static catalog but include custom ones
./build/model-extractor --skip-default-static-catalog --static-catalog-files custom.yaml

# Generate Red Hat MCP servers catalog only (no model processing)
./build/model-extractor \
    --mcp-index data/redhat-mcp-servers-index.yaml \
    --mcp-catalog-output data/redhat-mcp-servers-catalog.yaml \
    --skip-huggingface --skip-enrichment --skip-catalog

# Generate Partner MCP servers catalog only (no model processing)
./build/model-extractor \
    --mcp-index data/partner-mcp-servers-index.yaml \
    --mcp-catalog-output data/partner-mcp-servers-catalog.yaml \
    --skip-huggingface --skip-enrichment --skip-catalog

# Generate Community MCP servers catalog only (no model processing)
./build/model-extractor \
    --mcp-index data/community-mcp-servers-index.yaml \
    --mcp-catalog-output data/community-mcp-servers-catalog.yaml \
    --skip-huggingface --skip-enrichment --skip-catalog
```

### CLI Options

| Option | Description | Default |
|--------|-------------|---------|
| `--input` | Path to models index YAML file | `data/models-index.yaml` |
| `--output-dir` | Output directory for extracted metadata | `output` |
| `--catalog-output` | Path for the generated models catalog | `data/models-catalog.yaml` |
| `--max-concurrent` | Maximum concurrent model processing jobs | `5` |
| `--skip-huggingface` | Skip HuggingFace collection processing | `false` |
| `--skip-enrichment` | Skip metadata enrichment | `false` |
| `--skip-catalog` | Skip catalog generation | `false` |
| `--static-catalog-files` | Comma-separated list of static catalog files | `""` |
| `--skip-default-static-catalog` | Skip processing default input/supplemental-catalog.yaml | `false` |
| `--mcp-index` | Path to MCP servers index YAML file (enables MCP catalog generation) | `""` |
| `--mcp-catalog-output` | Path for the generated MCP servers catalog | `data/redhat-mcp-servers-catalog.yaml` |
| `--skip-mcp-enrichment` | Skip MCP server OCI image enrichment (architectures, timestamps) | `false` |
| `--help` | Show help message | `false` |

### Metadata Report CLI Options

| Option | Description | Default |
|--------|-------------|---------|
| `--catalog` | Path to models catalog YAML file | `data/models-catalog.yaml` |
| `--output-dir` | Directory containing model metadata | `output` |
| `--report-dir` | Directory for generated reports | `output` |
| `--help` | Show help message | `false` |

## Docker Build and Deployment

### Building the Container

The Docker build uses a single-stage approach based on `registry.access.redhat.com/ubi9-micro:latest`. It copies pre-generated catalog files and benchmark data into a minimal image.

```bash
make docker-build
```

The image exposes two volume mount points:
- `/app/data` — contains the pre-generated catalog and index YAML files
- `/app/benchmarks` — contains sample benchmark data

### Container Usage Examples

```bash
# Run container (stays alive for data access)
docker run -d --name model-metadata-catalog model-metadata-collection:latest

# Copy catalog files from container to host
docker cp model-metadata-catalog:/app/data/models-catalog.yaml ./models-catalog.yaml
docker cp model-metadata-catalog:/app/data/validated-models-catalog.yaml ./validated-models-catalog.yaml

# Mount data directory for external access
docker run -d -v $(pwd)/catalog-data:/app/data --name catalog model-metadata-collection:latest

# Remove container when done
docker rm -f catalog
```

### Custom Docker Build Options

```bash
# Build with custom image name and tag
DOCKER_IMAGE_NAME=my-model-catalog DOCKER_IMAGE_TAG=v1.0 make docker-build

# View image details after build
docker images model-metadata-collection
```

## Input Format

The tool accepts multiple input sources:

### Automatic HuggingFace Collections (Default)
Discovers Red Hat AI validated model collections from HuggingFace and generates version-specific index files such as `data/hugging-face-redhat-ai-validated-v1-0.yaml`.

### Static Model Catalogs
The tool merges static model catalogs with dynamically extracted metadata. By default, it reads `input/supplemental-catalog.yaml` automatically:

```yaml
source: Red Hat
models:
  - name: Static Model Example
    provider: Static Provider
    description: A model defined in static catalog
    language:
      - en
    license: MIT
    tasks:
      - text-generation
    artifacts:
      - uri: oci://example.com/static-model:1.0
```

### Manual YAML Input
Provide a YAML file with structured model entries supporting both OCI registry and HuggingFace model references:

```yaml
models:
  - type: "oci"
    uri: "registry.redhat.io/rhelai1/modelcar-granite-3-1-8b-base-quantized-w4a16:1.5"
    labels: ["validated"]
  - type: "oci"
    uri: "registry.redhat.io/rhelai1/modelcar-llama-3-3-70b-instruct:1.5"
    labels: ["validated", "featured"]
  - type: "hf"
    uri: "https://huggingface.co/microsoft/Phi-3.5-mini-instruct"
    labels: ["validated", "lab-teacher"]
```

Each model entry supports the following fields:
- **type**: `"oci"` for registry-based modelcar containers or `"hf"` for HuggingFace model links
- **uri**: The OCI registry reference or HuggingFace model URL
- **labels**: Array of labels added as tags to the model metadata
  - Common labels include: `"validated"`, `"featured"`, `"lab-teacher"`, `"lab-base"`
  - The tool converts labels to customProperties in the final model catalog
  - Add new labels without code changes
- **model_type**: Optional model type classification (defaults to `"generative"` if omitted)
  - Allowed values: `"generative"`, `"predictive"`, or `"unknown"`
  - Validated during catalog generation
  - Appears in the generated catalog as a customProperty

### Version-Specific Index Files
Generated automatically from HuggingFace collections.

```yaml
# Example: data/hugging-face-redhat-ai-validated-v1-0.yaml
version: v1.0
models:
  - name: RedHatAI/Llama-4-Scout-17B-16E-Instruct
    url: https://huggingface.co/RedHatAI/Llama-4-Scout-17B-16E-Instruct
    readme_path: /RedHatAI/Llama-4-Scout-17B-16E-Instruct/README.md
```

## Model Type Classification

The tool classifies models using `model_type`, which appears in the catalog's `customProperties`.

### Supported Model Types

- **`generative`**: Models that generate new content (text, images, etc.)
  - Examples: Large Language Models (LLMs), text-to-image models, code generators
  - This is the **default** when `model_type` is not specified

- **`predictive`**: Models that make predictions or classifications
  - Examples: Sentiment analysis, image classification, forecasting models

- **`unknown`**: Models with unclear or mixed purposes
  - Use when the model type cannot be determined

### Automatic Default Behavior

The tool defaults all models to `"generative"` in these cases:

1. **Static Catalogs**: Models in `input/supplemental-catalog.yaml` receive `model_type: "generative"`
2. **Dynamic Catalogs**: Models extracted from OCI containers default to `"generative"` unless specified otherwise
3. **Index Files**: Models in index YAML files without a `model_type` field receive the default

### Explicit Model Type Specification

To specify a different model type, add the `model_type` field to the index YAML:

```yaml
models:
  - type: "oci"
    uri: "registry.redhat.io/rhelai1/modelcar-example-predictive:1.0"
    labels: ["validated"]
    model_type: "predictive"  # Explicitly set as predictive model
```

### Validation

The tool validates `model_type` values during catalog generation:

- **Valid Values**: `"generative"`, `"predictive"`, and `"unknown"`
- **Invalid Values**: When the tool detects an invalid `model_type`, it:
  1. Logs a warning with the invalid value
  2. Falls back to `"generative"`
  3. Continues catalog generation

Example validation warning:
```
Warning: Invalid model_type "custom-type" for model "example-model", defaulting to "generative": invalid model_type: "custom-type" (allowed values: "generative", "predictive", "unknown")
```

### Output Format

In the generated catalog, `model_type` appears in the `customProperties` section in `MetadataStringValue` format:

```yaml
customProperties:
  model_type:
    metadataType: MetadataStringValue
    string_value: "generative"
```

This format ensures compatibility with downstream systems that consume catalog data.

## Output Structure

### Individual Model Metadata

The tool generates the following for each model:

```
output/
└── registry.redhat.io_rhelai1_modelcar-granite-3-1-8b-base-quantized-w4a16_1.5/
    └── models/
        ├── modelcard.md          # Original model card content (when available)
        ├── metadata.yaml         # Structured metadata (always created)
        └── enrichment.yaml       # Data source tracking
```

**Note**: When modelcard extraction fails, the tool creates a skeleton `metadata.yaml` so enrichment can still populate data from HuggingFace and other sources.

### Metadata Schema

```yaml
name: RedHatAI/granite-3.1-8b-base-quantized.w4a16
provider: Neural Magic (Red Hat)
description: Granite 3.1 8b Base (w4a16 quantized)
readme: |
  # granite-3.1-8b-base-quantized.w4a16
  ...
language:
  - en
license: apache-2.0
licenseLink: https://www.apache.org/licenses/LICENSE-2.0
tags:
  - validated                    # From labels array in models-index.yaml
  - featured                     # From labels array in models-index.yaml
  - lab-teacher                  # Additional custom labels from models-index.yaml
  - granite                      # Tags from HuggingFace enrichment
  - language                     # Additional tags merged from various sources
tasks:
  - text-generation
artifacts:
  - uri: oci://registry.redhat.io/rhelai1/modelcar-granite-3-1-8b-base-quantized-w4a16:1.5
    createTimeSinceEpoch: 1755612925000
    lastUpdateTimeSinceEpoch: 1755612925000
    customProperties:
      source:
        string_value: registry.redhat.io
      type:
        string_value: modelcar
customProperties:
  model_type:
    metadataType: MetadataStringValue
    string_value: "generative"
```

### Aggregated Catalog

```yaml
source: Red Hat
models:
  - name: RedHatAI/granite-3.1-8b-base-quantized.w4a16
    provider: Neural Magic (Red Hat)
    # ... complete metadata for all models
```

### Metadata Reports

The reporting tool analyzes field completeness and data source tracking:

#### Report Structure

```
reports/
├── metadata-report.md         # Human-readable markdown report
└── metadata-report.yaml       # Machine-readable YAML report
```

#### Report Contents

- **Field Completeness**: Shows percentage completion for each metadata field across all models
- **Data Source Analysis**: Breaks down where metadata comes from (modelcard.md, HuggingFace, registry, etc.)
- **Individual Model Reports**: Detailed analysis for each model including missing fields and YAML health scores
- **Source Method Tracking**: Distinguishes between YAML frontmatter, regex extraction, API calls, and generated data

#### Example Report Output

```markdown
# Model Metadata Completeness Report

**Generated:** 2025-08-20 12:48:42 UTC

## Summary

**Total Models:** 39

### Field Completeness

| Field | Populated | Null | Percentage |
|-------|-----------|------|------------|
| tasks | 39 | 0 | 100.0% |
| artifacts | 39 | 0 | 100.0% |
| name | 39 | 0 | 100.0% |
| license | 39 | 0 | 100.0% |
| description | 39 | 0 | 100.0% |
| readme | 39 | 0 | 100.0% |
| provider | 38 | 1 | 97.4% |
| licenseLink | 37 | 2 | 94.9% |
| language | 35 | 4 | 89.7% |
| createTimeSinceEpoch | 26 | 13 | 66.7% |
| maturity | 0 | 39 | 0.0% |

### Data Sources

| Source | Count | Percentage |
|--------|-------|------------|
| modelcard.regex | 199 | 49.0% |
| huggingface.tags | 92 | 22.7% |
| registry | 39 | 9.6% |
| huggingface.yaml | 33 | 8.1% |
| generated | 30 | 7.4% |
```

## Development

### Setting Up Development Environment

```bash
make setup
```

This installs development tools: linters and security scanners.

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run benchmarks
make benchmark
```

### Code Quality

```bash
# Format code
make fmt

# Run linters
make lint

# Run all checks
make check
```

### Development Workflow

```bash
# Quick development iteration
make dev
```

Runs formatting, vetting, testing, and building in sequence.

### Available Make Targets

| Target | Description |
|--------|-------------|
| `build` | Build the binary |
| `clean` | Clean build artifacts and output |
| `test` | Run tests |
| `test-coverage` | Run tests with coverage |
| `lint` | Run linters |
| `fmt` | Format code |
| `check` | Run all checks (fmt-check, vet, lint) |
| `dev` | Quick development iteration |
| `ci` | Full CI pipeline |
| `release` | Create optimized release build |
| `run` | Run with default settings |
| `process` | Process all model indexes and MCP server catalogs |
| `report` | Generate metadata completeness reports |
| `docker-build` | Build Docker container image |

## API Integration

### HuggingFace Integration

The tool integrates with HuggingFace APIs to:

- Discover Red Hat AI validated model collections
- Fetch detailed model metadata
- Extract provider information from README files
- Parse structured data from model tags

**Data Prioritization**: The tool follows a strict priority hierarchy:
1. **Primary**: HuggingFace YAML frontmatter (highest priority, overrides all other sources)
2. **Secondary**: Data extracted from `modelcard.md` files in container layers
3. **Tertiary**: HuggingFace API data
4. **Fallback**: Registry metadata and generated defaults

When modelcard extraction fails, the tool creates a minimal metadata structure for enrichment.

**Tag Management**: The tool merges tags from multiple sources:
- Labels from `models-index.yaml` are added as tags
- Tags from modelcard.md and HuggingFace enrichment are merged and deduplicated

### Container Registry Integration

- Fetches OCI manifest metadata
- Extracts creation and update timestamps
- Processes custom annotations and properties
- Supports multiple registry formats

## Testing

The project includes:

- **Unit Tests**: Utility functions and core logic
- **Integration Tests**: API interactions and file processing
- **Property-Based Tests**: Edge cases and data validation

```bash
make test
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Include tests with your changes
4. Run the full test suite: `make ci`
5. Submit a pull request

## Troubleshooting

### Common Issues

1. **Permission Errors**: Ensure output directories are writable
2. **Network Timeouts**: Check internet connectivity and registry access
3. **Memory Issues**: Lower `--max-concurrent` in resource-constrained environments
4. **API Rate Limits**: HuggingFace requests use a 30-second timeout with no built-in rate limiting

## License

Licensed under the terms specified in the LICENSE file.
