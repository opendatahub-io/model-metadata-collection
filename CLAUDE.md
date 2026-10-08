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
- `make process-models` - Process all model indexes (redhat, validated, other)
- `make process-redhat-models` - Process Red Hat models index only
- `make process-validated-models` - Process validated models index only
- `make process-other-models` - Process other models index only
- `make process-redhat-mcp` - Process Red Hat MCP servers catalog only
- `make install-serving-runtime-tools` - Install missing Skopeo, Cosign, and Kustomize CLIs with `go install` into `build/serving-runtime-tools/bin`; reuse tools already on `PATH`
- `make generate-serving-runtimes SERVING_RUNTIME_SKIP_UNAVAILABLE_IMAGES=true` - Generate runtime index, inputs, and catalog from configured controller source images, skipping unpublished GA sources; automatically install missing CLIs first
- `make process-serving-runtimes` - Rebuild the runtime catalog from committed index/input files, offline
- `make check-serving-runtimes` - Validate the runtime API schema, committed inputs, and catalog freshness, offline
- `make validate-serving-runtimes` - Validate the catalog against pinned model-registry OpenAPI schemas, offline
- `make lint-serving-runtimes` - Lint generated runtime YAML (requires yamllint; separate from Go linting)
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

**CRITICAL**: All index files MUST follow the glob pattern `input/models/collections/hugging-face-redhat-ai-validated-v*.yaml` to be discoverable by `GetLatestVersionIndexFile()`.

Path constants are centralized in `internal/huggingface/collections.go` (`CollectionsDir`, `CollectionFilePrefix`, helpers).

Requirements:
1. Prefix: `input/models/collections/hugging-face-redhat-ai-validated-`
2. **MUST** include `v` immediately after the prefix
3. Extension: `.yaml`

Examples:
- `input/models/collections/hugging-face-redhat-ai-validated-v2026-02.yaml` (date-based)
- `input/models/collections/hugging-face-redhat-ai-validated-v1-0-granite-quantized.yaml` (special collection)

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
3. Verify: `cat input/models/collections/hugging-face-redhat-ai-validated-v{version}.yaml`
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

**New models not in catalog**: Check index file generation (`ls input/models/collections/hugging-face-redhat-ai-validated-v*.yaml`), then check validated index (`grep -i "model-name" data/validated-models-index.yaml`), then test enrichment in isolation.

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

**Naming requirements**: `name` must be canonical `<reverse-dns-namespace>/<slug>`
(e.g. `com.redhat/openshift-mcp-server`, not `redhat/openshift-mcp-server`) and
`version` must be valid [semver](https://semver.org/) (e.g. `0.4.0`, not `0.4`
or `latest`). `display_name` is an optional human-facing label. Invalid values
are rejected by `MCPServerMetadata.Validate()` in `pkg/types/mcpserver.go` and
halt catalog generation — see [CONTRIBUTING.md](CONTRIBUTING.md#mcp-server-naming-conventions)
for details.

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

## Serving Runtime Catalog

Maintain controller targets and runtime image parameters in
`input/serving_runtimes/generator-config.yaml`. Generation fetches source images
matching each `target_image`, discovers templates recursively, and renders their
Kustomize overlays after replacing `params.env` values in a temporary copy. New
parameter keys can be appended for newer source releases. Local `../kserve` and
`../odh-model-controller` checkouts are references for the parameter inventory;
the release configuration uses target images rather than checkout overrides.

Runtime image parameters may use floating tags, including `latest`. Before
Kustomize or SBOM discovery, the resolver freezes each distinct tag to
`repository:tag@sha256:digest`, caching across controller sources. The digest
comes from the top-level manifest, retaining multi-architecture indexes. Existing
digest pins and non-image parameters remain unchanged; explicit runtime versions
still freeze image tags. Configuration keeps the floating tags, while generated
inputs/catalog retain the resolved digests. Offline checks do not revisit tags;
source-based checks and regeneration can discover newer images. Controller target
tags such as `rhoai-3.6` also work, but each source needs its runtime parameters.

EA controller targets and RHOAI image parameters use the published floating
`v3.6-ea.1` aliases; `rhaii-fast` parameters use `3.6.0-fast.1`. Their initial
resolved digests match the EA operator bundle. The older `odh-vllm-cpu-rhel9`
parameter retains its bundle digest because no 3.6 EA alias is published.
Generated parameter image references keep tag and digest; configuration aliases
can resolve to rebuilt images on subsequent runs.

The two 3.6 GA sources currently configure expected, unpublished Red Hat floating
aliases: `rhaii/...:3.6` and `rhoai/...:v3.6`. The stable vLLM Omni repository is
also unconfirmed. Parameter images, including digest pins, are checked before
rendering using `skopeo inspect --raw`; hashing the top-level manifest avoids
architecture selection and retains multi-architecture indexes. Explicit registry
manifest/repository absence excludes only template versions using those images;
unrelated templates and available releases remain.
All image fields count, including worker/prefill, init and sidecar images. Missing
unused parameters exclude nothing. Checks are cached across sources. Authentication
and HTTP not-found errors stop generation by default. `--skip-unavailable-images`
also skips parameter images on authorization or HTTP 401/403/404 errors; use
`make generate-serving-runtimes SERVING_RUNTIME_SKIP_UNAVAILABLE_IMAGES=true`.
With the flag, unavailable controller/source-image inspections skip the entire
source. This includes registry absence and authorization or HTTP 401/403/404
responses from target images, explicit source images, or all source attachment
candidates. Default controller lookup behavior remains strict. Skipped sources
and lookup errors are logged. Network/TLS, server, download, extraction, Kustomize,
malformed metadata, and SBOM failures remain fatal after applicable retries. An entirely
unavailable catalog fails without changing outputs. The committed generated files
currently contain only EA sources; offline checks do not query registries.

Generation preflights controller/source availability before resolving parameters
or SBOMs. Successful source resolutions are reused. A `RuntimeSourceLoader` caches
pristine extracted configuration by source manifest digest and extraction roots
for the invocation only; each parameter set renders a private copy. Close the loader
to remove cached files. Distinct sources are still downloaded each run; no persistent
cache or extra cache configuration is used.

Skopeo inspect/copy and Cosign attestation commands retry recognized HTTP 408,
429, 500, 502, 503, 504 and transient network failures, with four total attempts
and 1/2/4-second backoffs. Cancellation interrupts waits and commands. Authorization,
absence, invalid metadata, certificate verification, and local credential failures
are not retried. Exhausted transient failures remain fatal even with the skip flag.
Existing tool-internal retries may also apply.

Generated files are:

- `input/serving_runtimes/generated/redhat/*.yaml` - One input per runtime containing all configured versions
- `data/redhat-serving-runtimes-index.yaml` - Stable runtime identifiers and input paths
- `data/redhat-serving-runtimes-catalog.yaml` - Loader catalog with JSON manifest strings

Change the generator configuration or upstream templates and regenerate these
files together; do not hand-edit generated data. Each ServingRuntime Template and
LLMInferenceServiceConfig is an independent catalog entry. Multiple configured
controller releases contribute versions to the same runtime input. Generation
collects and validates every source before writing outputs; colliding object
names or duplicate versions fail generation.

Use unique, release-qualified source IDs, for example
`kserve-llmisvc-3-6-0-ea-1`; distinct builds of the same release need distinct IDs.
List runtime image parameters explicitly in each source.
Catalog identifiers and filenames stay stable across versions. Template names,
nested ServingRuntime names, and LLMInferenceServiceConfig names beginning with
`kserve-` receive a sanitized runtime-version prefix. Build metadata is removed
from object names; full versions remain in the catalog and annotations.

Upstream vLLM versions come from SPDX SBOM attestations downloaded with
`cosign download attestation`, including architecture SBOMs whose versions must
agree. Explicit `<image-key>-upstream-version` parameters bypass discovery. Do
not introduce the deprecated `cosign download sbom` workflow. Downloading and
matching image subjects do not verify publisher signatures. Support levels are
inferred from the target release and rendered images. Each generated version
defaults `minimumRHOAIVersion` to the controller target tag's `major.minor`, for
example `v3.6.0-ea.1-1788535633` or `rhoai-3.6` gives `"3.6"`. Use RHOAI release tags for
`target_image`; a tag plus digest retains this default, while digest-only targets
and non-release tags leave it unset. Optional
`templates.<source-relative-path>.minimum_rhoai_version` overrides take precedence
only for that source's version; examples for both manifest types are commented
in the configuration. This is a catalog extension: the pinned upstream API and
loader do not expose or enforce the minimum version.

`make validate-serving-runtimes` uses unmodified, embedded upstream API schema
fragments in `internal/catalog/serving_runtime_schema/`; its README records the
pinned revision and update procedure. It projects the catalog's runtime and
version entries separately and supplies the loader's artifact type. Validation
checks original YAML types, nested schemas, formats, and existing catalog
semantics. Unknown fields fail. Deliberate `minimumRHOAIVersion` extensions are
validated locally and reported separately. The API represents embedded manifests
as strings; this check does not validate Kubernetes CRD schemas or deployment.
Shared catalog validation requires every deployment image to start with
`registry.redhat.io/rhoai/`, `registry.redhat.io/rhaii/`,
`registry.redhat.io/rhaii-early-access/`, or `registry.redhat.io/rhaii-fast/`.
This includes the version's `image` and every embedded `image` field in Templates
and LLMInferenceServiceConfigs. Worker, prefill, init, sidecar, and additional
Template objects are checked recursively; failures identify the runtime version
and field path. Generation and offline/API checks enforce the same rule.

After regeneration, run `make check-serving-runtimes` and
`make lint-serving-runtimes`. YAML lint uses `.yamllint-serving-runtimes.yaml`
with two-space indentation and permits long image pins/JSON strings. Go lint
does not lint the generated YAML. Override `YAMLLINT` if needed, for example
`make lint-serving-runtimes YAMLLINT='uv tool run --from yamllint yamllint'`.

Generation accepts `SERVING_RUNTIME_GENERATOR_CONFIG_PATH`,
`SERVING_RUNTIME_AUTHFILE` and `SERVING_RUNTIME_SKIP_UNAVAILABLE_IMAGES` Make
overrides. To check all generated files directly
against the configured sources without writing them, run:

```bash
go run ./cmd/serving-runtime-catalog \
  --config input/serving_runtimes/generator-config.yaml --skip-unavailable-images --check
```

This source check requires registry access and the generation tools; the index
check above is offline. See [docs/serving-runtime-catalog.md](docs/serving-runtime-catalog.md)
for source resolution, parameter precedence, naming, and override details.

## Tool-Calling Metadata Extraction

Extracted from HuggingFace YAML frontmatter ONLY (not container modelcards). Fields: `tool_calling_supported`, `required_cli_args`, `chat_template_path`, `tool_call_parser`, `validated_tasks`.

Tool-calling configuration is output as structured YAML fields in the catalog, not rendered into README Markdown:

- **`servingConfig.toolCalling`** in `CatalogMetadata` — contains `toolCallParser`, `chatTemplate`, `enableAutoToolChoice`, and `requiredArgs` (camelCase keys matching the kubeflow/hub OpenAPI schema)
- **`validatedTasks`** in both `ExtractedMetadata` and `CatalogMetadata` — lists tasks that have been validated (e.g., `["tool-calling"]`)
- **`toolCallingConfig`** in `ExtractedMetadata` — intermediate extraction struct persisted for catalog generation

Chat template paths are auto-converted from `examples/` (HuggingFace) to `opt/app-root/template/` (RHOAI). When tool-calling config is present, `tool-calling` is automatically injected into the `tasks` array if not already present.

## vLLM Recommended Configurations

Optimized vLLM configurations from the PSAP team are stored as YAML files in `input/models/vllm-config/`. During enrichment, these are matched by exact `model.name` and rendered as a "vLLM Recommended Configurations" markdown section appended to the model's README. See `pkg/types/vllmconfig.go` for the YAML schema and `pkg/utils/templates/vllm-config.md.tmpl` for the template.
