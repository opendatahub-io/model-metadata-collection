# Model Metadata Collection

A Go application for extracting, enriching, and cataloging metadata from Red Hat AI container images. This tool automatically discovers models from HuggingFace collections, processes OCI container images containing AI models, extracts model cards, enriches the metadata with information from HuggingFace, and generates comprehensive catalogs.

## Features

- **HuggingFace Collections Integration**: Automatically discovers and processes Red Hat AI validated model collections with version support (v1.0, v2.1, etc.)
- **OCI Container Analysis**: Extracts model cards from container image layers with annotation-based detection; creates skeleton metadata when extraction fails to ensure enrichment continuity
- **Metadata Enrichment**: Enriches model metadata with HuggingFace data including date-to-epoch conversion and quality validation, **with modelcard.md data taking priority over external sources**
- **Automated Tagging**: Automatically adds labels as tags based on model configuration with intelligent tag merging
- **Registry Integration**: Fetches OCI artifact metadata from container registries
- **Metadata Reporting**: Comprehensive analysis of metadata completeness, data source tracking, and quality metrics
- **Flexible CLI**: Configurable input/output paths and processing options with skip flags for individual components
- **Concurrent Processing**: Parallel processing of multiple models with configurable concurrency limits
- **Comprehensive Testing**: Unit tests for all major components
- **Structured Output**: Generates both individual model metadata and aggregated catalogs with quality-validated metadata

## Architecture

The project is organized into modular packages for maintainability and testability:

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

- Go 1.24 or later
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

This will create binaries at:
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

Run with default settings (automatically processes HuggingFace collections and falls back to `data/models-index.yaml`):

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

Generate comprehensive metadata completeness reports:

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
| `--help` | Show help message | `false` |

### Metadata Report CLI Options

| Option | Description | Default |
|--------|-------------|---------|
| `--catalog` | Path to models catalog YAML file | `data/models-catalog.yaml` |
| `--output-dir` | Directory containing model metadata | `output` |
| `--report-dir` | Directory for generated reports | `output` |
| `--help` | Show help message | `false` |

## Docker Build and Deployment

### Prerequisites for Docker Build

Before building the Docker image, you must authenticate with Red Hat's container registry:

```bash
# Login to Red Hat container registry (required for accessing ModelCar images)
docker login registry.redhat.io
# Enter your Red Hat account credentials when prompted
```

**Important**: This authentication step is required because the Docker build process needs to access Red Hat container images during the generation stage.

### Building the Container

The Docker build uses a multi-stage approach for optimal image size and security:

1. **Builder Stage**: Uses `registry.access.redhat.com/ubi9/go-toolset:1.24` to compile the Go application
2. **Generator Stage**: Uses `registry.access.redhat.com/ubi9-minimal` to run model extraction and generate `data/models-catalog.yaml`
3. **Runtime Stage**: Uses `registry.access.redhat.com/ubi9-micro` for the final minimal image containing only the catalog

```bash
# Build the Docker image (requires prior registry.redhat.io authentication)
make docker-build

# The build automatically uses your host Docker authentication
# No additional credential files needed
```

### Docker Authentication Details

- The build process automatically detects and uses your existing `~/.docker/config.json` authentication
- Registry credentials are securely mounted during build using Docker BuildKit secrets
- No credentials are stored in the final container image
- If no authentication is found, the build gracefully continues without registry access (with reduced functionality)

### Container Usage Examples

```bash
# Run container and access the generated catalog
docker run -d --name model-metadata-catalog model-metadata-collection:latest

# Copy catalog from container to host
docker cp model-metadata-catalog:/app/data/models-catalog.yaml ./models-catalog.yaml

# Mount catalog directory for external access
docker run -d -v $(pwd)/catalog-data:/app/data --name catalog model-metadata-collection:latest

# Remove container when done
docker rm -f model-metadata-catalog
```

### Custom Docker Build Options

```bash
# Build with custom image name and tag
DOCKER_IMAGE_NAME=my-model-catalog DOCKER_IMAGE_TAG=v1.0 make docker-build

# View image details after build
docker images model-metadata-collection
```

## Input Format

The tool can work with multiple input sources:

### Automatic HuggingFace Collections (Default)
Automatically discovers Red Hat AI validated model collections from HuggingFace and generates version-specific index files like `data/hugging-face-redhat-ai-validated-v1-0.yaml`.

### Manual YAML Input
You can also provide a YAML file with structured model entries supporting both OCI registry and HuggingFace model references:

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
- **labels**: Array of labels that will be added as tags to the model metadata
  - Common labels include: `"validated"`, `"featured"`, `"lab-teacher"`, `"lab-base"`
  - Labels are automatically converted to customProperties in the final model catalog
  - New labels can be added without code changes

### Version-Specific Index Files
Generated automatically from HuggingFace collections:

```yaml
# Example: data/hugging-face-redhat-ai-validated-v1-0.yaml
version: v1.0
models:
  - name: RedHatAI/Llama-4-Scout-17B-16E-Instruct
    url: https://huggingface.co/RedHatAI/Llama-4-Scout-17B-16E-Instruct
    readme_path: /RedHatAI/Llama-4-Scout-17B-16E-Instruct/README.md
```

## Output Structure

### Individual Model Metadata

For each model, the tool generates:

```
output/
└── registry.redhat.io_rhelai1_modelcar-granite-3-1-8b-base-quantized-w4a16_1.5/
    └── models/
        ├── modelcard.md          # Original model card content (when available)
        ├── metadata.yaml         # Structured metadata (always created)
        └── enrichment.yaml       # Data source tracking
```

**Note**: When modelcard extraction fails (e.g., no modelcard layer found in the container), the tool automatically creates a skeleton `metadata.yaml` file to ensure the enrichment process can still populate data from HuggingFace and other sources.

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

The metadata reporting functionality generates comprehensive analysis of field completeness and data source tracking:

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

This installs development tools including linters and security scanners.

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

# Run security scan
make security

# Run all checks
make check
```

### Development Workflow

```bash
# Quick development iteration
make dev
```

This runs formatting, vetting, testing, and building in sequence.

### Available Make Targets

| Target | Description |
|--------|-------------|
| `build` | Build the binary |
| `clean` | Clean build artifacts and output |
| `test` | Run tests |
| `test-coverage` | Run tests with coverage |
| `lint` | Run linters |
| `fmt` | Format code |
| `dev` | Quick development iteration |
| `ci` | Full CI pipeline |
| `release` | Create optimized release build |
| `run` | Run with default settings |
| `process` | Run with custom input/output paths |
| `report` | Generate metadata completeness reports |
| `docker-build` | Build Docker container image |

## API Integration

### HuggingFace Integration

The tool integrates with HuggingFace APIs to:

- Discover Red Hat AI validated model collections
- Fetch detailed model metadata
- Extract provider information from README files
- Parse structured data from model tags

**Data Prioritization**: The tool follows a strict data priority hierarchy:
1. **Primary**: Data extracted from `modelcard.md` files in container layers (highest priority)
2. **Secondary**: HuggingFace API data (used only when modelcard.md data is missing or empty)
3. **Fallback**: Registry metadata and generated defaults
4. **Skeleton Creation**: When modelcard extraction fails completely, creates minimal metadata structure for enrichment

**Tag Management**: The tool implements intelligent tag merging:
- Labels from the models-index.yaml configuration are automatically added as tags
- Existing modelcard tags are preserved and merged with HuggingFace enrichment tags
- Duplicate tags are automatically deduplicated
- Tag precedence follows the same hierarchy as data prioritization

This ensures that when non-empty data is present in the modelcard.md, it is always preserved and used over any external enrichment sources.

### Container Registry Integration

- Fetches OCI manifest metadata
- Extracts creation and update timestamps
- Processes custom annotations and properties
- Supports multiple registry formats

## Error Handling

The tool includes comprehensive error handling:

- **Network Failures**: Graceful degradation when APIs are unavailable
- **Malformed Data**: Robust parsing with fallback mechanisms
- **Missing Files**: Clear error messages and suggestions
- **Concurrent Processing**: Proper error isolation between goroutines
- **Failed Modelcard Extraction**: Automatically creates skeleton metadata files when modelcard layers are missing or corrupted, ensuring enrichment processes can continue
- **Tag Merging**: Intelligent tag merging that preserves existing tags while adding new ones from multiple sources

## Migration and Compatibility

The tool supports migration from legacy metadata formats:

- **Legacy String Artifacts**: Automatically migrated to structured OCI artifacts
- **Mixed Timestamp Types**: Consistent int64 timestamp handling
- **Backward Compatibility**: Reads existing metadata files in multiple formats

## Testing

The project includes comprehensive test coverage:

- **Unit Tests**: All utility functions and core logic
- **Integration Tests**: API interactions and file processing
- **Property-Based Tests**: Edge cases and data validation

Run tests with:

```bash
make test
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run the full test suite: `make ci`
5. Submit a pull request

### Code Standards

- Follow Go conventions and idioms
- Include unit tests for new functionality
- Update documentation as needed
- Run `make check` before submitting

## Security

- No hardcoded credentials or secrets
- Secure handling of external API responses
- Input validation and sanitization
- Regular security scanning with `make security`

## Performance

- Concurrent processing of multiple models
- Configurable concurrency limits
- Efficient memory usage for large files
- Optimized for large-scale batch processing

## Troubleshooting

### Common Issues

1. **Permission Errors**: Ensure output directories are writable
2. **Network Timeouts**: Check internet connectivity and registry access
3. **Memory Issues**: Reduce `--max-concurrent` for resource-constrained environments
4. **API Rate Limits**: The tool includes built-in rate limiting for HuggingFace APIs

### Debugging

Enable verbose logging:

```bash
./build/model-extractor --help  # Shows all available options
```

Check the logs for detailed processing information.

## License

This project is licensed under the terms specified in the LICENSE file.

## Support

For issues and questions:

1. Check the troubleshooting section above
2. Search existing [GitHub issues](https://github.com/opendatahub-io/model-metadata-collection/issues)
3. Create a new issue with detailed information
