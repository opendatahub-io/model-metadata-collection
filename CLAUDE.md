# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go application that extracts model metadata (model cards) from Red Hat AI ModelCar container images. Automatically consumes HuggingFace collections, processes multiple ModelCar container images in parallel, scans layers for modelcard annotations, and extracts structured metadata with quality validation.

## Key Commands

- `make build` - Build the model-extractor binary
- `make test` - Run tests
- `make lint` - Run linters (requires golangci-lint)
- `make check` - Run all checks (fmt-check, vet, lint)
- `make dev` - Quick development iteration (fmt, vet, test, build)
- `make ci` - Full CI pipeline (deps, check, test, build)
- `make process` - Process all model indexes and MCP server catalogs
- `make process-models` - Process model indexes only (redhat, validated, other)
- `make process-redhat-mcp` - Process Red Hat MCP servers catalog only
- `make docker-build` - Build Docker container image

See [CONTRIBUTING.md](CONTRIBUTING.md) for full development setup, testing, and debugging instructions.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed architecture documentation with diagrams covering data flow, package structure, concurrency model, and dependencies.

### Testing Notes

- Unit tests run with `make test` and skip integration tests that make network calls
- Integration tests in `internal/registry/registry_test.go` are skipped during normal test runs
- Test fixtures are in `sample-data/` (also accessible via the `testdata/` symlink)

## Project Infrastructure

- **CI**: `.github/workflows/ci.yml` runs linting and tests on all PRs; `build-and-push-static-model-catalog-data.yml` handles Docker builds
- **Pre-commit**: `.pre-commit-config.yaml` provides local hooks for go-fmt, go-vet, and golangci-lint
- **Code Ownership**: `.github/CODEOWNERS` defines review assignment

## HuggingFace Collection Index File Naming Convention

**CRITICAL**: All index files MUST follow the glob pattern `data/hugging-face-redhat-ai-validated-v*.yaml` to be discoverable by `GetLatestVersionIndexFile()`.

Requirements:
1. Prefix: `data/hugging-face-redhat-ai-validated-`
2. **MUST** include `v` immediately after the prefix
3. Extension: `.yaml`

Examples:
- `data/hugging-face-redhat-ai-validated-v2026-02.yaml` (date-based)
- `data/hugging-face-redhat-ai-validated-v1-0-granite-quantized.yaml` (special collection)

When adding a new special collection type in `parseVersionFromTitle()` (`internal/huggingface/collections.go`), the return value MUST start with `v`:

```go
// CORRECT
return "v1.0-your-collection-name"
// INCORRECT - file won't be discovered
return "your-collection-name"
```

The `version` field inside the YAML file should match the `parseVersionFromTitle()` return value.

### Adding New Special Collections

1. Update `parseVersionFromTitle()` in `internal/huggingface/collections.go` — return must start with `v`
2. Update discovery patterns in `DiscoverValidatedModelCollections()` in `internal/huggingface/client.go`
3. Add to fallback list in `ProcessCollections()` in `internal/huggingface/collections.go`
4. Add tests in `internal/huggingface/collections_test.go`

## Docker Build and Deployment

Multi-stage Docker build: builder (UBI9 go-toolset) -> generator (UBI9-minimal) -> runtime (UBI9-micro).

```bash
# Requires prior registry.redhat.io authentication
docker login registry.redhat.io
make docker-build
```

Authentication is automatically detected from `~/.docker/config.json` and securely mounted via BuildKit secrets. No credentials are stored in the final image.

## Adding New Models and Model Families

### Adding a New HuggingFace Collection (Monthly/Dated)

1. Add collection slug to `internal/huggingface/collections.go` fallback list in `ProcessCollections()`
2. Run `make process` to generate index files and updated catalogs
3. Verify: `cat data/hugging-face-redhat-ai-validated-v{version}.yaml`
4. Ensure `data/validated-models-index.yaml` ends with a newline

### Adding a New Model Family

All model families are centrally defined in `internal/config/model_families.go`.

**Steps:**
1. Add family name to `SupportedModelFamilies` slice (alphabetically sorted)
2. Add normalization test cases in `pkg/utils/text_test.go`
3. Run `make test` — automated consistency checks verify alphabetical ordering, no duplicates, valid format, and regex pattern inclusion

The centralized config automatically propagates to:
- `internal/enrichment/enrichment.go` — cross-family matching prevention
- `pkg/utils/text.go` — version normalization regex

### Testing New Model Enrichment

Test enrichment in isolation before full processing:

```bash
make build
./build/model-extractor \
    --input test-model-index.yaml \
    --output-dir output/test-model \
    --skip-catalog \
    --skip-default-static-catalog
# Verify: grep "^name:" output/test-model/*/models/metadata.yaml
# Clean up: rm test-model-index.yaml && rm -rf output/test-model
```

### Common Pitfalls

**Model name is `null` in catalog**: Normalization mismatch causes low similarity score (< 0.5 threshold). Fix by adding the model family to `SupportedModelFamilies`.

**Collection index file not found**: Missing `v` prefix in `parseVersionFromTitle()` return value.

**New models not in catalog**: Check index file generation (`ls data/hugging-face-redhat-ai-validated-v*.yaml`), then check validated index (`grep -i "model-name" data/validated-models-index.yaml`), then test enrichment in isolation.

### Checklist for Adding New Model Collections

- [ ] Add collection slug to `internal/huggingface/collections.go` fallback list
- [ ] If new model family, add to `SupportedModelFamilies` in `internal/config/model_families.go` (alphabetically)
- [ ] Add test cases for new model family in `pkg/utils/text_test.go`
- [ ] Run `make test` to verify consistency checks pass
- [ ] Run `make build && make process`
- [ ] Verify generated index file exists with correct version prefix
- [ ] Test enrichment for at least one model in isolation
- [ ] Verify all models appear in catalog with proper names (not `null`)

## MCP Server Metadata

Individual MCP server YAML files live in `input/mcp_servers/` organized into subdirectories:
- `input/mcp_servers/redhat/` - Red Hat MCP servers
- `input/mcp_servers/partner/` - Partner MCP servers
- `input/mcp_servers/community/` - Community MCP servers

Each catalog has its own index file that references servers by `input_path`:
- `data/redhat-mcp-servers-index.yaml` → `data/redhat-mcp-servers-catalog.yaml`
- `data/partner-mcp-servers-index.yaml` → `data/partner-mcp-servers-catalog.yaml`
- `data/community-mcp-servers-index.yaml` → `data/community-mcp-servers-catalog.yaml`

During `make process`, artifacts are enriched from OCI registries (architectures, timestamps) then aggregated into their respective catalog files. Types are in `pkg/types/mcpserver.go`, catalog in `internal/catalog/mcp_catalog.go`, enrichment in `internal/catalog/mcp_enrichment.go`. Use `--skip-mcp-enrichment` to bypass registry calls.

### Adding a New MCP Server

**For Red Hat MCP servers:**
1. Create `input/mcp_servers/redhat/<server-name>.yaml` (use an existing file as template)
2. Add an entry to `data/redhat-mcp-servers-index.yaml`
3. Run `make process` (or `make process-redhat-mcp` for MCP-only)
4. Verify: `grep "name:" data/redhat-mcp-servers-catalog.yaml`

**For Partner MCP servers:**
1. Create `input/mcp_servers/partner/<server-name>.yaml` (use an existing file as template)
2. Add an entry to `data/partner-mcp-servers-index.yaml`
3. Run `make process` (or `make process-partner-mcp` for MCP-only)
4. Verify: `grep "name:" data/partner-mcp-servers-catalog.yaml`

**For Community MCP servers:**
1. Create `input/mcp_servers/community/<server-name>.yaml` (use an existing file as template)
2. Add an entry to `data/community-mcp-servers-index.yaml`
3. Run `make process` (or `make process-community-mcp` for MCP-only)
4. Verify: `grep "name:" data/community-mcp-servers-catalog.yaml`

### MCP-Only Processing

```bash
# Process Red Hat MCP servers only
./build/model-extractor \
    --mcp-index data/redhat-mcp-servers-index.yaml \
    --mcp-catalog-output data/redhat-mcp-servers-catalog.yaml \
    --skip-huggingface --skip-enrichment --skip-catalog

# Process Partner MCP servers only
./build/model-extractor \
    --mcp-index data/partner-mcp-servers-index.yaml \
    --mcp-catalog-output data/partner-mcp-servers-catalog.yaml \
    --skip-huggingface --skip-enrichment --skip-catalog

# Process Community MCP servers only
./build/model-extractor \
    --mcp-index data/community-mcp-servers-index.yaml \
    --mcp-catalog-output data/community-mcp-servers-catalog.yaml \
    --skip-huggingface --skip-enrichment --skip-catalog
```

## Tool-Calling Metadata Extraction

Extracted from HuggingFace YAML frontmatter ONLY (not container modelcards). Fields: `tool_calling_supported`, `required_cli_args`, `chat_template_path`, `tool_call_parser`.

When found, a vLLM deployment section is automatically appended to the model's README. Chat template paths are auto-converted from `examples/` (HuggingFace) to `opt/app-root/template/` (RHOAI).

See `pkg/utils/templates/tool-calling.md.tmpl` for the generated template.

## vLLM Recommended Configurations

Optimized vLLM configurations from the PSAP team are stored as YAML files in `input/models/vllm-config/`. During enrichment, these are matched by exact `model.name` and rendered as a "vLLM Recommended Configurations" markdown section appended to the model's README. See `pkg/types/vllmconfig.go` for the YAML schema and `pkg/utils/templates/vllm-config.md.tmpl` for the template.
