# Serving Runtime Catalog

This document describes the review process and lifecycle management for the serving runtime catalog. Packaging in the `odh-model-metadata-collection` image is planned.

## Overview

The serving runtime catalog provides a curated list of supported inference runtimes. The catalog is consumed by the model-registry-operator's serving runtime loader.

**Generator output path**: `data/redhat-serving-runtimes-catalog.yaml` (currently generated with dummy example images)

**Planned packaged location**: `/app/data/redhat-serving-runtimes-catalog.yaml` in the container image; the Dockerfile does not yet copy this artifact.

## Review Requirements

### Adding or Updating Runtimes

All pull requests modifying serving runtime data require:

1. **Maintainer approval** — Validates technical correctness of images, arguments, environment variables, and resource recommendations
2. **Standalone validation** — Run `make check-serving-runtimes` locally; CI integration is planned. Neither `make ci` nor the current CI workflows invoke this check.

### Support Level Definitions

| Level | Description |
|-------|-------------|
| `redHatSupported` | Derived from index source `Red Hat Serving Runtimes` |
| `supported` | Legacy explicit support level |
| `techPreview` | Technology Preview, limited support |
| `developerPreview` | Developer Preview, no production support |
| `community` | Community-contributed, best-effort |

### Image Requirements

- Images must be fully qualified with registry domain (e.g., `registry.redhat.io/...`, `quay.io/...`)
- Images must be pinned to a specific tag or digest (`:latest` is prohibited even when a digest is also present)

## File Structure

```
data/
  redhat-serving-runtimes-index.yaml    # Runtime inputs and catalog overrides
  redhat-serving-runtimes-catalog.yaml  # Generated catalog with dummy example images

input/serving_runtimes/redhat/
  runtime-template.yaml                 # Template for new runtimes
  vllm-cpu.yaml                         # Original OpenShift Template
  vllm-cpu-x86.yaml
  vllm-cuda.yaml
  vllm-gaudi.yaml
  vllm-rocm.yaml
  vllm-spyre-s390x.yaml
```

The six upstream templates retain image placeholders. The index currently supplies dummy `example.invalid` image overrides so generation and the standalone check can run. These images are not deployable; replace them with actual image references for deployment. Generate the catalog before running the standalone check.

## Adding a New Runtime

1. **Save the original OpenShift Template**

   Place the unchanged YAML under `input/serving_runtimes/redhat/`. The generator detects `apiVersion: template.openshift.io/v1` and `kind: Template`. It currently accepts exactly one `serving.kserve.io/v1alpha1` `ServingRuntime` object containing exactly one container. Other object kinds and ambiguous multi-object or multi-container templates are rejected.

   The adapter maps the embedded runtime name, display name, runtime version, model formats, `multiModel`, recommended accelerators, container arguments, and literal environment values. Provider, description, documentation URL, and comma-separated tags come from template annotations. Display name falls back to the template annotation if absent on the runtime. `REST` is not interpreted as a protocol version, and GPU requirements are not inferred.

   Original commands, ports, volumes, labels, and other unmapped fields remain in the input file; they are not included in generated output. `valueFrom` and `envFrom` are rejected because the current catalog cannot represent them. The adapter is not a Kubernetes manifest validator.

   Existing catalog-shaped inputs (such as `runtime-template.yaml`) remain supported with strict field validation.

2. **Add the index entry and overrides**

   ```yaml
   source: Red Hat Serving Runtimes
   serving_runtimes:
     - name: vllm-cpu-runtime
       input_path: input/serving_runtimes/redhat/vllm-cpu.yaml
       image: <fully-qualified-image-with-non-latest-tag-or-digest>
   ```

   The name must match the embedded runtime's `metadata.name`. The generator derives `supportLevel: redHatSupported` from source `Red Hat Serving Runtimes` (case-insensitive, ignoring surrounding whitespace), overriding any input version support level. No per-entry support level is needed. Unrecognized sources leave legacy input support levels unchanged; template inputs without a derived level fail support-level validation. Set `image` to resolve an upstream placeholder; omit it only when the template already contains a valid concrete image. Image overrides also apply to each version of legacy catalog-shaped inputs. Image overrides undergo the same validation as ordinary images.

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
2. Update the original template and its index overrides. For catalog-shaped inputs, add a new entry to the `versions` array.
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
| `latest` tag, with or without a digest | `must not use the latest tag` |
| Invalid support level | `invalid supportLevel` |
| Invalid protocol version | `invalid protocol version` (allowed: `v1`, `v2`, `grpc-v2`) |
| Invalid resource quantity | `resource tier requires valid cpu and memory quantities` |
| Secret env var with default value | `unsafe defaultValue for environment variable` |
| Env var named like a secret with default | `unsafe defaultValue for environment variable` |
| Unknown YAML fields | `field ... not found in type` |

## Standalone Check and Planned CI Integration

The standalone `make check-serving-runtimes` target invokes the generator with `--check` to verify:

1. The checked-in catalog matches what generation produces (determinism check)
2. All validation rules pass
3. No schema drift between index and input files

The check compares the existing catalog with freshly generated output without modifying it. If the catalog is missing or out of date, run `make process-serving-runtimes` and commit the generated catalog. Resolve any input validation errors before regenerating.

CI integration is planned: the `ci` target and current CI workflows do not invoke `check-serving-runtimes`.

## Example Input File

This illustrates the legacy catalog-shaped input format. The checked-in vLLM inputs use original OpenShift Templates instead.

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
    supportLevel: redHatSupported
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
