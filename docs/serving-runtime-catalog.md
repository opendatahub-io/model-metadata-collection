# Serving Runtime Catalog

This document describes the review process and lifecycle management for the serving runtime catalog packaged in the `odh-model-metadata-collection` image.

## Overview

The serving runtime catalog provides a curated list of supported inference runtimes. The catalog is consumed by the model-registry-operator's serving runtime loader.

**Generated artifact**: `data/redhat-serving-runtimes-catalog.yaml`  
**Packaged location**: `/app/data/redhat-serving-runtimes-catalog.yaml` in the container image

## Review Requirements

### Adding or Updating Runtimes

All pull requests modifying serving runtime data require:

1. **Maintainer approval** — Validates technical correctness of images, arguments, environment variables, and resource recommendations
2. **CI validation** — `make check-serving-runtimes` must pass

### Support Level Definitions

| Level | Description |
|-------|-------------|
| `supported` | Fully supported in production |
| `techPreview` | Technology Preview, limited support |
| `developerPreview` | Developer Preview, no production support |
| `community` | Community-contributed, best-effort |

### Image Requirements

- Images must be fully qualified with registry domain (e.g., `registry.redhat.io/...`, `quay.io/...`)
- Images must be pinned to a specific tag or digest (no `:latest`)

## File Structure

```
data/
  redhat-serving-runtimes-index.yaml    # Index referencing input files
  redhat-serving-runtimes-catalog.yaml  # Generated catalog (do not edit)

input/serving_runtimes/redhat/
  runtime-template.yaml                 # Template for new runtimes
  vllm.yaml                             # Example: vLLM runtime
  openvino.yaml                         # Example: OpenVINO runtime
```

## Adding a New Runtime

1. **Create the input file**

   Copy the template and fill in all required fields:

   ```bash
   cp input/serving_runtimes/redhat/runtime-template.yaml \
      input/serving_runtimes/redhat/<runtime-name>.yaml
   ```

   Required fields:
   - `name` — Lowercase alphanumeric with hyphens (e.g., `vllm`, `openvino`)
   - `displayName` — Human-readable name
   - `provider` — Organization providing the runtime
   - `description` — Brief description of the runtime
   - `versions` — At least one version entry

   Each version requires:
   - `version` — Semantic version string
   - `image` — Fully qualified, pinned container image
   - `supportLevel` — One of: `supported`, `techPreview`, `developerPreview`, `community`

2. **Add the index entry**

   Edit `data/redhat-serving-runtimes-index.yaml`:

   ```yaml
   source: Red Hat Serving Runtimes
   serving_runtimes:
     - name: <runtime-name>
       input_path: input/serving_runtimes/redhat/<runtime-name>.yaml
   ```

   The `name` must exactly match the `name` field in the input file.

3. **Generate and validate**

   ```bash
   make process-serving-runtimes
   make check-serving-runtimes
   make test
   ```

4. **Submit for review**

   Open a pull request with:
   - The new input file
   - Updated index file
   - Regenerated catalog file
   - Justification for the new runtime

## Updating an Existing Runtime

### Adding a New Version

1. Edit the runtime's input file under `input/serving_runtimes/redhat/`
2. Add a new entry to the `versions` array
3. Regenerate: `make process-serving-runtimes`
4. Submit PR for review

### Updating Metadata

For non-version changes (description, tags, documentation URLs):

1. Edit the input file
2. Regenerate the catalog
3. Submit PR for review

### Updating Resource Recommendations

1. Update `recommendedResources` in the version entry
2. Include benchmark data or performance analysis in the PR description

## Deprecating a Runtime Version

To deprecate a version without removing it:

1. Set `deprecated: true` on the version entry:

   ```yaml
   versions:
     - version: "1.0.0"
       image: registry.redhat.io/example:1.0.0
       supportLevel: supported
       deprecated: true
   ```

2. Regenerate and submit PR
3. Downstream consumers should filter deprecated versions from default selection

## Removing a Runtime or Version

Removal requires careful coordination to avoid breaking deployments.

### Removing a Version

1. Verify no active deployments depend on the version
2. Remove the version entry from the input file
3. Regenerate and submit PR
4. Document the removal in release notes

### Removing an Entire Runtime

1. Mark all versions as `deprecated: true` in a prior release
2. Remove the input file
3. Remove the entry from the index file
4. Regenerate and submit PR
5. Document the removal in release notes

## Validation Rules

The catalog generator enforces these rules (generation fails on violations):

| Rule | Error |
|------|-------|
| Empty or invalid runtime name | `invalid or duplicate runtime name` |
| Missing displayName, provider, or description | `requires displayName, provider, description and versions` |
| No versions defined | `requires ... versions` |
| Duplicate version within runtime | `missing or duplicate version` |
| Unqualified image (no registry domain) | `must be a fully qualified pinned container reference` |
| Unpinned image (no tag/digest) | `must be a fully qualified pinned container reference` |
| Invalid support level | `invalid supportLevel` |
| Invalid protocol version | `invalid protocol version` (allowed: `v1`, `v2`, `grpc-v2`) |
| Invalid resource quantity | `resource tier requires valid cpu and memory quantities` |
| Secret env var with default value | `unsafe defaultValue for environment variable` |
| Env var named like a secret with default | `unsafe defaultValue for environment variable` |
| Unknown YAML fields | `field ... not found in type` |

## CI Integration

The CI pipeline runs `make check-serving-runtimes` to verify:

1. The checked-in catalog matches what generation produces (determinism check)
2. All validation rules pass
3. No schema drift between index and input files

If CI fails, regenerate locally with `make process-serving-runtimes` and commit the updated catalog.

## Example Input File

```yaml
name: vllm
displayName: vLLM
provider: Red Hat
description: High-throughput GPU inference runtime for large language models
documentationUrl: https://docs.redhat.com/en/documentation/red_hat_ai
repositoryUrl: https://github.com/vllm-project/vllm
tags:
  - gpu
  - llm
  - inference
supportedModelFormats:
  - name: safetensors
  - name: pytorch
capabilities:
  requiresGPU: true
  supportedAccelerators:
    - nvidia.com/gpu
    - amd.com/gpu
versions:
  - version: "3.4.0"
    image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.4.0
    supportLevel: supported
    protocolVersions:
      - v2
      - grpc-v2
    recommendedResources:
      recommended:
        cpu: "4"
        memory: 16Gi
        accelerator:
          nvidia.com/gpu: "1"
    defaultArgs:
      - "--max-model-len"
      - "4096"
    env:
      - name: HF_TOKEN
        description: HuggingFace API token for gated models
        secret: true
      - name: VLLM_LOGGING_LEVEL
        description: Logging verbosity
        defaultValue: INFO
```

## Related Documentation

- [CLAUDE.md](../CLAUDE.md) — Project overview and development guidelines
- [CONTRIBUTING.md](../CONTRIBUTING.md) — Contribution process
- [ARCHITECTURE.md](../ARCHITECTURE.md) — System architecture
