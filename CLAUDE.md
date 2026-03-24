# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go application that extracts model metadata (specifically model cards) from Red Hat AI model container images. The tool has been enhanced to automatically consume data from HuggingFace collections and processes multiple ModelCar container images in parallel, scanning their layers for modelcard annotations, and extracts structured metadata with quality validation.

## Key Commands

### Build and Run
- `make build` - Build the model-extractor binary
- `make run` - Run the application to process HuggingFace collections and extract metadata from ModelCar container images
- `make clean` - Clean build artifacts and output
- `make fmt` - Format Go code
- `make vet` - Check for Go code issues
- `make test` - Run tests with coverage reporting
- `make docker-build` - Build Docker container image

### Development
- `make deps` - Download and tidy dependencies
- `make dev` - Quick development iteration (fmt, vet, test, build)
- `make ci` - Full CI pipeline
- `make lint` - Run linters (requires golangci-lint)

### Testing Notes
- Unit tests run with `make test` and skip integration tests that make network calls
- Integration tests in `internal/registry/registry_test.go` are currently skipped during normal test runs
- These tests can be run separately for integration testing with external registries

## Architecture

### Core Components

1. **HuggingFace Collections Processing** (`processHuggingFaceCollections`)
   - Automatically discovers Red Hat AI validated model collections
   - Supports semver version detection (v1.0, v2.1, etc.)
   - Generates version-specific index files in `data/hugging-face-redhat-ai-validated-v{version}.yaml`
   - Falls back to known collections if discovery fails

2. **Main Processing Flow** (`main.go:654+`)
   - First processes HuggingFace collections to generate/update index files
   - Uses latest version index file or falls back to legacy `models-index.yaml`
   - Processes ModelCar manifest references in parallel with semaphore limiting (max 5 concurrent processes)
   - Each goroutine handles one manifest reference from start to finish

3. **Container Image Processing** (`FetchManifestSrcAndLayers`)
   - Uses `github.com/containers/image/v5` library for OCI container operations
   - Parses Docker references and creates image sources
   - Extracts manifest metadata and layer information

4. **ModelCard Extraction and Metadata Processing** (`ScanLayersForModelCarD`)
   - Scans container layers for `io.opendatahub.modelcar.layer.type: "modelcard"` annotation
   - Handles both gzipped and uncompressed tar layers
   - Extracts `.md` files and processes metadata with quality validation
   - Converts dates to Unix epoch timestamps
   - Extracts OCI artifacts and model properties
   - Writes structured metadata alongside modelcard content

5. **Output Organization**
   - Creates sanitized directory names from manifest references
   - Generates structured YAML metadata files with extracted information
   - Output structure: `output/{sanitized-manifest-ref}/models/{modelcard.md,metadata.yaml}`
   - Creates summary `manifests.yaml` with metadata presence indicators

### Key Dependencies
- `github.com/containers/image/v5` - Container image manipulation and OCI registry access
- Standard library: `archive/tar`, `compress/gzip`, `sync` for concurrent processing

### Concurrency Model
- Uses `sync.WaitGroup` to coordinate parallel processing
- Semaphore pattern with buffered channel limits concurrent goroutines to 5
- Each manifest reference is processed independently in its own goroutine

### Data Flow
1. HuggingFace collections discovery → Version-specific index file generation
2. Load model references from latest index or legacy file → Parallel processing
3. Fetch container manifest and layers → Layer scanning
4. Find modelcard-annotated layers → Extract and validate metadata
5. Generate structured YAML metadata → Write organized output

## Enhanced Features (Recent Updates)

### HuggingFace Collections Integration
- **Automatic Discovery**: Scans HuggingFace for Red Hat AI validated model collections
- **Version Support**: Handles semver patterns like "Red Hat AI validated models - v1.0"
- **Index Generation**: Creates `data/hugging-face-redhat-ai-validated-v{version}.yaml` files with model names, URLs, and README paths
- **Flexible Processing**: Falls back gracefully when HuggingFace APIs are unavailable

### Metadata Quality Improvements
- **Date Conversion**: Converts date strings to Unix epoch timestamps (e.g., "7/11/2024" → 1720656000)
- **OCI Artifact Extraction**: Focuses on relevant container registry patterns and model names
- **Validation Functions**: `isValidValue()` and `cleanExtractedValue()` ensure quality metadata
- **Structured Output**: Generates both `modelcard.md` and `metadata.yaml` for each model

### Version-Specific Index Files
```yaml
# Example: data/hugging-face-redhat-ai-validated-v1-0.yaml
version: v1.0
models:
  - name: RedHatAI/Llama-4-Scout-17B-16E-Instruct
    url: https://huggingface.co/RedHatAI/Llama-4-Scout-17B-16E-Instruct
    readme_path: /RedHatAI/Llama-4-Scout-17B-16E-Instruct/README.md
```

## HuggingFace Collection Index File Naming Convention

**CRITICAL**: All HuggingFace collection index files MUST follow a specific naming pattern to be discoverable by the system.

### Naming Requirements

The `GetLatestVersionIndexFile()` function uses the glob pattern:
```go
filepath.Glob("data/hugging-face-redhat-ai-validated-v*.yaml")
```

This requires ALL index files to:
1. Start with prefix: `data/hugging-face-redhat-ai-validated-`
2. **MUST** include a `v` immediately after the prefix (e.g., `v2026-02`, `v1-0-granite-quantized`)
3. End with `.yaml` extension

### File Naming Examples

**Date-based Collections:**
- ✅ `data/hugging-face-redhat-ai-validated-v2026-02.yaml` (February 2026)
- ✅ `data/hugging-face-redhat-ai-validated-v2025-09.yaml` (September 2025)
- ❌ `data/hugging-face-redhat-ai-validated-2026-02.yaml` (missing 'v' prefix)

**Special Collections (non-dated):**
- ✅ `data/hugging-face-redhat-ai-validated-v1-0-granite-quantized.yaml`
- ✅ `data/hugging-face-redhat-ai-validated-v1-0-embedding-models.yaml`
- ❌ `data/hugging-face-redhat-ai-validated-granite-quantized.yaml` (missing 'v' prefix)
- ❌ `data/hugging-face-redhat-ai-validated-embedding-models.yaml` (missing 'v' prefix)

### Version String Format in Code

When adding a new special collection type in `parseVersionFromTitle()` (internal/huggingface/collections.go):

```go
// CORRECT - includes v prefix for compatibility with GetLatestVersionIndexFile
if strings.Contains(lowerTitle, "embedding") {
    return "v1.0-embedding-models"  // Will generate: v1-0-embedding-models.yaml
}

// INCORRECT - missing v prefix, file won't be discovered
if strings.Contains(lowerTitle, "embedding") {
    return "embedding-models"  // Would generate: embedding-models.yaml (not found!)
}
```

### Version Field Inside Files

The `version` field inside the YAML file should match the version string returned by `parseVersionFromTitle()`:

```yaml
# File: data/hugging-face-redhat-ai-validated-v1-0-embedding-models.yaml
version: v1.0-embedding-models  # Matches the parseVersionFromTitle() return value
models:
  - name: RedHatAI/embeddinggemma-300m
    # ...
```

### Adding New Special Collections

When adding a new special collection (like granite-quantized or embedding-models):

1. **Update `parseVersionFromTitle()`** in `internal/huggingface/collections.go`:
   ```go
   if strings.Contains(lowerTitle, "your-collection-keyword") {
       return "v1.0-your-collection-name"  // MUST start with 'v'
   }
   ```

2. **Update discovery patterns** in `DiscoverValidatedModelCollections()` in `internal/huggingface/client.go`:
   ```go
   validatedPatterns := []*regexp.Regexp{
       // ... existing patterns ...
       regexp.MustCompile(`(?i)your.?collection.?keyword`),
   }
   ```

3. **Add to fallback list** in `ProcessCollections()` in `internal/huggingface/collections.go`:
   ```go
   collectionSlugs = []string{
       // ... existing collections ...
       "RedHatAI/your-collection-slug",
   }
   ```

4. **Update tests** in `internal/huggingface/collections_test.go`:
   ```go
   {
       name:     "your collection test",
       title:    "Your Collection Title",
       expected: "v1.0-your-collection-name",  // Must match parseVersionFromTitle()
   },
   ```

### Why the 'v' Prefix is Required

Without the 'v' prefix, the files will:
- ❌ Not be found by `GetLatestVersionIndexFile()`
- ❌ Not be included in automatic version detection
- ✅ Still be included in `generateMergedIndex()` (uses broader `*` pattern)
- ⚠️  Result in inconsistent behavior depending on code path

**Always use the 'v' prefix to ensure files are discoverable across all code paths.**

## Docker Build and Deployment

### Prerequisites
Before building the Docker image, you must authenticate with Red Hat's container registry:

```bash
# Login to Red Hat container registry (required for accessing ModelCar images)
docker login registry.redhat.io
# Enter your Red Hat account credentials when prompted
```

### Building the Container
The Docker build uses a multi-stage approach:

1. **Builder Stage**: Uses `registry.access.redhat.com/ubi9/go-toolset:1.24` to compile the Go application
2. **Generator Stage**: Uses `registry.access.redhat.com/ubi9-minimal` to run model extraction and generate `data/models-catalog.yaml`
3. **Runtime Stage**: Uses `registry.access.redhat.com/ubi9-micro` for the final minimal image containing only the catalog

```bash
# Build the Docker image (requires prior registry.redhat.io authentication)
make docker-build

# The build automatically uses your host Docker authentication
# No additional credential files needed
```

### Authentication Details
- The build process automatically detects and uses your existing `~/.docker/config.json` authentication
- Registry credentials are securely mounted during build using Docker BuildKit secrets
- No credentials are stored in the final container image
- If no authentication is found, the build gracefully continues without registry access

### Usage Examples
```bash
# Run container and access the generated catalog
docker run -d --name model-metadata-catalog model-metadata-collection:latest

# Copy catalog from container to host
docker cp model-metadata-catalog:/app/data/models-catalog.yaml ./models-catalog.yaml

# Mount catalog directory for external access
docker run -d -v $(pwd)/catalog-data:/app/data --name catalog model-metadata-collection:latest
```

## Adding New Models and Model Families

This section documents the process for adding new HuggingFace collections and ensuring proper metadata enrichment for new model families.

### Adding a New HuggingFace Collection (Monthly/Dated)

When a new monthly validated models collection is released (e.g., "March 2026"), follow these steps:

**1. Add Collection Slug to Fallback List**

Update `internal/huggingface/collections.go` in the `ProcessCollections()` function:

```go
collectionSlugs = []string{
    "RedHatAI/red-hat-ai-validated-models-may-2025-682613dc19c4a596dbac9437",
    // ... existing collections ...
    "RedHatAI/red-hat-ai-validated-models-march-2026-69b0697e7f157651f5c0f5ac", // NEW
    "RedHatAI/granite-quantized",
    "RedHatAI/embedding-models",
}
```

**2. Run `make process`**

This will:
- Generate the version-specific index file (e.g., `data/hugging-face-redhat-ai-validated-v2026-03.yaml`)
- Update `data/validated-models-index.yaml` with ModelCar references
- Generate updated catalogs

**3. Verify Generated Files**

```bash
# Check the generated index file
cat data/hugging-face-redhat-ai-validated-v2026-03.yaml

# Verify models are in the catalog
grep "name:" data/validated-models-catalog.yaml | tail -10
```

**4. Fix Any Missing Newlines**

Ensure `data/validated-models-index.yaml` ends with a newline (required by global standards).

### Adding a New Model Family

When adding models from a **new model family** (e.g., MiniMax, DeepSeek, etc.), you need to register the family in the centralized configuration.

**⚠️ CRITICAL: Centralized Model Family Registration**

**All model families are now centrally defined in `internal/config/model_families.go`**

**1. Add to SupportedModelFamilies** (`internal/config/model_families.go`)

Add the new family name to the `SupportedModelFamilies` slice (keep alphabetically sorted):

```go
var SupportedModelFamilies = []string{
    "deepseek",
    "gemma",
    "granite",
    "kimi",
    "llama",
    "minimax",     // Example: Add your new family here
    "mistral",
    "mixtral",
    "phi",
    "qwen",
    "yournewfamily", // Add new family alphabetically
}
```

**That's it!** The centralized configuration automatically updates:
- ✅ `internal/enrichment/enrichment.go` - Model family extraction for cross-family matching
- ✅ `pkg/utils/text.go` - Version normalization regex pattern
- ✅ Build-time consistency checks ensure everything stays synchronized

**2. Run Tests to Verify**

The centralized approach includes automated consistency checks:

```bash
# Run tests to verify model family is properly registered
make test

# Tests will verify:
# - Alphabetical ordering
# - No duplicates
# - Valid family name format
# - Regex pattern includes all families
```

**3. Add Test Cases for New Family**

Update `pkg/utils/text_test.go` with normalization test cases:

```go
{
    name:     "yournewfamily standard version",
    input:    "registry.redhat.io/rhai/modelcar-yournewfamily-3-1:3.0",
    expected: "yournewfamily-3v1",
},
{
    name:     "yournewfamily from HuggingFace",
    input:    "RedHatAI/YourNewFamily-3.1",
    expected: "yournewfamily-3v1",
},
```

**Benefits of Centralized Approach:**
- ✅ Single source of truth prevents synchronization issues
- ✅ Automated tests catch missing families at build time
- ✅ Pre-compiled regex improves performance
- ✅ Easier to add new families (only one location to update)
- ✅ Self-documenting with comprehensive inline comments

### Testing New Model Enrichment

**⚠️ IMPORTANT**: Always test enrichment for new model families in isolation to identify issues quickly.

**1. Create a Test Index File**

```yaml
# test-model-index.yaml
models:
- type: oci
  uri: registry.redhat.io/rhai/modelcar-your-new-model:3.0
  labels:
  - validated
```

**2. Run Isolated Test**

```bash
make build

rm -rf output/test-model

./build/model-extractor \
    --input test-model-index.yaml \
    --output-dir output/test-model \
    --skip-catalog \
    --skip-default-static-catalog 2>&1 | grep -E "(Processing model:|Fetching|Found name|Found provider|Found description)"
```

**3. Verify Enrichment**

```bash
# Check the name field was populated
grep "^name:" output/test-model/registry.redhat.io_rhai_modelcar-your-new-model_3.0/models/metadata.yaml

# Should output something like:
# name: RedHatAI/YourNewModel-M2.5
```

**4. If Enrichment Fails (name: null)**

Debug the similarity score:
- Check that the model family is in `internal/config/model_families.go` `SupportedModelFamilies` slice
- Run `make test` to verify consistency checks pass
- Verify the HuggingFace model exists in `data/hugging-face-redhat-ai-validated-merged.yaml`

**5. Clean Up Test Files**

```bash
rm test-model-index.yaml
rm -rf output/test-model
```

### Common Pitfalls and Debugging

**Problem: Model name is `null` in catalog**

**Root Cause**: Model name normalization mismatch between registry and HuggingFace names results in low similarity score (< 0.5 threshold).

**Example (MiniMax case)**:
- Registry: `"minimax-m2-5"` → tokens: `["minimax", "m2", "5"]`
- HuggingFace: `"minimax-m2v5"` → tokens: `["minimax", "m2v5"]`
- Similarity: 0.33 (only 1 of 3 tokens match) → **NO MATCH**

**Solution**: Add model family to `internal/config/model_families.go` `SupportedModelFamilies` slice (alphabetically sorted)

**Problem: Collection index file not found by `GetLatestVersionIndexFile()`**

**Root Cause**: Index filename doesn't start with `v` prefix.

**Solution**: Ensure `parseVersionFromTitle()` returns version starting with `v` (e.g., `v2026.03`, `v1.0-embedding-models`).

**Problem: New models not showing up in catalog**

**Debugging Steps**:
1. Check if index file was generated: `ls -la data/hugging-face-redhat-ai-validated-v*.yaml`
2. Check if models are in validated index: `grep -i "your-model" data/validated-models-index.yaml`
3. Test enrichment in isolation (see Testing section above)
4. Check enrichment logs for similarity scores: `grep "similarity\|score\|match" logs`

### Checklist for Adding New Model Collections

- [ ] Add collection slug to `internal/huggingface/collections.go` fallback list
- [ ] If new model family, add to `SupportedModelFamilies` in `internal/config/model_families.go` (alphabetically)
- [ ] Add test cases for new model family in `pkg/utils/text_test.go`
- [ ] Run `make test` to verify consistency checks pass
- [ ] Run `make build && make process`
- [ ] Verify generated index file exists with correct version prefix
- [ ] Test enrichment for at least one model in isolation
- [ ] Verify all models appear in catalog with proper names (not `null`)
- [ ] Fix any missing newlines in `data/validated-models-index.yaml`

## Current Capabilities
- ✅ HuggingFace collections integration with automatic discovery
- ✅ Semver version detection and handling
- ✅ High-quality metadata extraction with validation
- ✅ Date-to-epoch conversion
- ✅ OCI artifact extraction
- ✅ Parallel processing with concurrency limiting
- ✅ Graceful error handling and fallbacks
- ✅ Version-specific index file generation
- ✅ Multi-stage Docker builds with registry authentication
- ✅ Containerized catalog generation
- ✅ Centralized model family configuration with automated consistency checks
- ✅ Tool-calling metadata extraction and README enrichment

## Tool-Calling Metadata Extraction

**Source**: HuggingFace YAML frontmatter ONLY (not from container modelcards)

The pipeline automatically extracts tool-calling configuration from HuggingFace README YAML frontmatter and appends a formatted vLLM deployment section to the model's README in the catalog. This makes tool-calling configuration visible and easy for users to copy-paste into their deployments.

### Extracted Fields

The following fields are extracted from HuggingFace YAML frontmatter:
- `tool_calling_supported` (bool) - Indicates if the model supports tool calling
- `required_cli_args` (array of strings) - Required vLLM CLI arguments for tool calling
- `chat_template_path` (string) - Path to chat template file (auto-converted from `examples/` to `opt/app-root/template/`)
- `tool_call_parser` (string) - Tool call parser type (e.g., "mistral", "llama", etc.)

### Example YAML Frontmatter

From RedHatAI/Ministral-3-14B-Instruct-2512:
```yaml
---
tool_calling_supported: true
required_cli_args:
  - --config_format mistral
  - --load_format mistral
  - --tokenizer_mode mistral
chat_template_path: None
tool_call_parser: mistral
---
```

### Generated README Section

When tool-calling metadata is found, a vLLM deployment section is automatically appended to the model's README:

```markdown
## vLLM Deployment with Tool Calling

This model supports tool calling capabilities. Use the following configuration for vLLM deployment:

### Required CLI Arguments

\`\`\`bash
vllm serve RedHatAI/Ministral-3-14B-Instruct-2512 \
  --config_format mistral \
  --load_format mistral \
  --tokenizer_mode mistral \
  --tool-call-parser mistral \
  --enable-auto-tool-choice
\`\`\`

### Tool Call Parser

This model uses the `mistral` tool call parser.
```

### Chat Template Path Conversion

Chat template paths are automatically converted for RHOAI/OpenShift AI deployments:
- Input: `examples/chat_template.jinja` (HuggingFace format)
- Output: `opt/app-root/template/chat_template.jinja` (RHOAI format)

## vLLM Recommended Configurations

Optimized vLLM configurations from the PSAP team are stored as YAML files in `input/models/vllm-config/`. During enrichment, these are matched by exact `model.name` and rendered as a "vLLM Recommended Configurations" markdown section appended to the model's README. See `pkg/types/vllmconfig.go` for the YAML schema and `pkg/utils/templates/vllm-config.md.tmpl` for the template.

