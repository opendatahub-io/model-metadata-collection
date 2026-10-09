# Serving Runtime Catalog

The serving runtime catalog is consumed by the model-registry-operator's serving
runtime loader. Template sources and deployment images are configured separately.

## Generate from source images or checkouts

The in-repository configuration is
[`input/serving_runtimes/generator-config.yaml`](../input/serving_runtimes/generator-config.yaml).
It explicitly sets runtime image parameters, replacing the controller source
defaults. EA controller and RHOAI component entries use floating `v3.6-ea.1`
aliases; vLLM uses the published `rhaii-fast/...:3.6.0-fast.1` aliases. These resolve
to the original matching operator bundle digests as of 2026-10-08 and may advance
to rebuilt images. The older `rhoai/odh-vllm-cpu-rhel9` image has no 3.6 EA alias
and retains its bundle digest pin. The two 3.6 GA controller entries use expected
`registry.redhat.io` tags:
`rhaii/...:3.6` for vLLM and `rhoai/...:v3.6` for RHOAI components. These floating
minor-release aliases follow existing `3.5`/`v3.5` tags; the 3.6 GA tags are not
published yet, and the stable vLLM Omni repository is unconfirmed. Generation
checks image availability and omits runtime versions using explicitly missing
images. It resolves published tags to SHA digests before rendering or SBOM
discovery. Authentication errors stop generation by default; the optional
`--skip-unavailable-images` flag also skips parameter images and controller
sources on authorization or not-found responses. Network errors remain fatal.
The checked-in generated files still contain only the EA sources. Configuration
population can be automated separately; the generator accepts the same YAML
contract regardless of its author.

The checked-in catalog contains nine KServe accelerator templates from
`odh-kserve-llmisvc-controller-rhel9:v3.6.0-ea.1-1788535633` and fifteen ServingRuntime
templates from `odh-model-controller-rhel9:v3.6.0-ea.1-1788888197`. Runtime image pins
come from `rhods-operator.3.6.0-ea.1` in the
[RHOAI operator bundle](https://catalog.redhat.com/en/software/containers/rhoai/odh-operator-bundle/659803ca929f3c931af06f28):
`registry.redhat.io/rhoai/odh-operator-bundle@sha256:0f3f08fe3d76327b6df7259f5d6e5b4e06d0a8a7dc5c8bf0ba1e6d8330ca47e0`.
The currently resolved EA aliases match the bundle's related images. Generated
parameter image references retain both tag and digest, while the configuration
keeps floating aliases for subsequent generation. All twenty-four entries
are `techPreview` because the controllers are EA; the bundle also points the base
vLLM parameters to `rhaii-fast` images. Only base parameter keys are configured;
`*-fast-1` and `*-fast-2` slots are left unchanged.

Upstream vLLM versions are discovered from each configured runtime image's SPDX
SBOM attestations, then supplied as `<image-parameter>-upstream-version` parameters
before Kustomize renders the source. Each source lists its runtime image parameters
explicitly; versions and template paths need no manual mapping.
For older controller sources that lack version replacements, the generator also
stamps the version paired with
the rendered primary image into its runtime annotation.

Each configured controller image has a unique source ID that includes its RHOAI
release, such as `kserve-llmisvc-3-6-0-ea-1`.
Use an additional build or digest suffix when configuring different pins for the
same release. Upstream runtime versions are discovered from SBOMs. Source IDs do
not change stable catalog identifiers or generated filenames.

The parameter inventory also accounts for newer local KServe/model-controller
checkouts. For example, the CPU image and upstream-version parameters are already
configured, although this pinned KServe source has no CPU template yet. They are
passed to Kustomize and remain unused until a target source includes their
replacements and templates. The local checkouts are only a reference for maintaining
the configuration; generation still fetches the configured target sources.

```bash
# Optional: install tools before logging in. Generation also runs this target.
make install-serving-runtime-tools
# Authenticate to registry.redhat.io when needed (or use an existing auth file).
PATH="$PATH:$PWD/build/serving-runtime-tools/bin" skopeo login registry.redhat.io

# Skip the configured GA sources until their images are published.
make generate-serving-runtimes SERVING_RUNTIME_SKIP_UNAVAILABLE_IMAGES=true
make check-serving-runtimes
```

Generation reuses Skopeo, Cosign, and Kustomize already available on `PATH`.
Missing tools are installed with `go install` into
`build/serving-runtime-tools/bin`, which Make adds to generation's `PATH`.
Go must already be installed; module downloads and any Go toolchain upgrades
require network access on the first installation. Installation versions are
pinned in the Makefile and can be overridden with `SKOPEO_VERSION`,
`COSIGN_VERSION`, and `KUSTOMIZE_VERSION`; `SERVING_RUNTIME_TOOLS_BIN` overrides
the installation directory (use an absolute path, as required by `GOBIN`).
Existing executables are kept regardless of version.
The Skopeo installation uses `CGO_ENABLED=0` and `containers_image_openpgp`
to avoid native development-library prerequisites; the generator uses registry
and directory transports. Offline processing and validation targets do not
install these tools.

Generation writes all three parts of the existing index/input/catalog contract:

```text
input/serving_runtimes/generator-config.yaml        # Maintained generator configuration
input/serving_runtimes/generated/redhat/*.yaml      # Generated runtime inputs
data/redhat-serving-runtimes-index.yaml             # Generated name/input_path index
data/redhat-serving-runtimes-catalog.yaml           # Generated loader catalog
```

The catalog and index are packaged under `/app/data/` in the container image.
Generated runtime inputs preserve manifest YAML objects for review; catalog
manifests are JSON strings as required by the loader. Each OpenShift Template
containing a ServingRuntime and each LLMInferenceServiceConfig gets an independent
catalog entry. ClusterServingRuntime mirrors, kustomizations, and unrelated
resources are excluded. Versions require at least one of `servingRuntimeTemplate`
or `llmInferenceServiceConfig`; consumers must accept either manifest type.

A successful source generation replaces the index with references to generated
inputs. The legacy index/input command remains available for offline regeneration.
Generation does not delete unreferenced historical generated inputs.

## Minimal configuration

Paths are relative to the working repository root, not the configuration file.
The provider defaults to `Red Hat, Inc.`. For example:

```yaml
source: Red Hat Serving Runtimes
sources:
  - id: kserve-llmisvc
    target_image: registry.redhat.io/rhoai/odh-kserve-llmisvc-controller-rhel9:v3.6.0-ea.1-1788535633
    parameters:
      kserve-llm-d-nvidia-cuda: registry.redhat.io/rhaii-fast/vllm-cuda-rhel9:3.6.0-fast.1
      # Set other accelerator parameters to their approved runtime images too.

  - id: odh-model-controller
    target_image: registry.redhat.io/rhoai/odh-model-controller-rhel9:v3.6.0-ea.1-1788888197
    parameters:
      vllm-cuda-image: registry.redhat.io/rhaii-fast/vllm-cuda-rhel9:3.6.0-fast.1
      # Set other runtime parameters to approved images too.
```

The sample images above are illustrative. `parameters` replaces existing keys in
the source's `params.env` and appends newer configured keys in sorted order. This
lets one configuration cover older and newer target releases; unused parameters
do not add templates or change support classifications. Invalid parameter keys
and multiline values are rejected. The source checkout is copied to a temporary directory before any
parameter changes, so generation never modifies the checkout.

Parameter keys must match the selected controller source's replacements. For
example, the EA odh-model-controller source uses `vllm-cpu-image`, while newer
3.6 sources use `vllm-cpu-pz-image` for the ppc64le/s390x runtime. Configuring the
older key alone leaves the newer source's default image in place; the generator
does not infer parameter renames.

Before rendering, the generator checks every fully qualified image in
`parameters` with `skopeo inspect --raw`, including existing digest pins. Hashing
the exact manifest bytes resolves tags to their top-level SHA digest without
selecting an architecture; ARM-only images and multi-architecture indexes work.
Checks and floating-tag resolutions are cached across sources for one invocation. An explicit
registry `MANIFEST_UNKNOWN`/`NAME_UNKNOWN` response marks the image as absent;
authentication and non-OCI HTTP not-found responses remain fatal by default.
Controller/source availability is checked first, before parameters or SBOMs, so
skipped releases incur no parameter lookups. Successful source resolutions are
reused during loading. Malformed inspection and SBOM failures remain fatal. This
distinction follows the
[OCI registry error codes](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#error-codes).
Registries may return authorization errors for inaccessible repositories; these
do not prove an image is absent.

To omit runtime versions using parameter images that return authorization errors
or HTTP 401/403/404, and skip unavailable controller sources, enable
`--skip-unavailable-images`, or its Make override:

```bash
make generate-serving-runtimes SERVING_RUNTIME_SKIP_UNAVAILABLE_IMAGES=true
```

The flag also skips an entire source when its `target_image` or explicit
`source_image` inspection returns registry absence, authorization errors, or HTTP
401/403/404. If all source attachment candidates return these errors, that source
is skipped too. Without the flag, controller/source lookup errors remain fatal.
Denied images are treated as unavailable without claiming they are absent.
Source IDs and lookup errors are logged. Transient registry/network failures are
retried before applying availability rules. Exhausted retries, certificate errors,
extraction, Kustomize, malformed metadata, and SBOM parsing failures still fail generation. An empty
result fails without changing existing outputs. Parameter image checks remain
cached and skipped templates are reported.

Skopeo inspections/copies and Cosign attestation downloads use the same bounded
retry policy: four total command attempts, with 1, 2, and 4 second backoffs.
Recognized HTTP 408, 429, 500, 502, 503, and 504 responses, rate-limit errors,
connection resets/refusals, temporary DNS failures, and network timeouts/EOFs are
retryable. Cancellation stops commands and backoff immediately. Authorization,
not-found responses, invalid references, certificate verification, local credential
errors, and invalid metadata are not retried. The CLI tools generally do not expose
`Retry-After` headers to the generator; this wrapper uses bounded backoff and
does not replace any retries performed internally by the installed tools.

After Kustomize and configured image substitutions, only templates that actually
use a missing parameter image are omitted. All image fields are considered,
including worker/prefill, init and sidecar containers. Available versions from
other sources and unrelated templates still contribute to the catalog. Missing
unused parameters omit no entries. Diagnostics identify each unavailable image
and skipped source/template. If no runtime versions remain, generation fails
without replacing existing files. Offline index/catalog checks do not query
the registry.

The generator automatically discovers these conventions:

| Project | Recursive template root | Kustomize overlay | Parameters |
| --- | --- | --- | --- |
| KServe | `config/overlays/odh/accelerators` | `config/overlays/odh` | `config/overlays/odh/params.env` |
| odh-model-controller | `config/runtimes` | `config/base` | `config/base/params.env` |

Kustomize uses each project's own replacements, including placeholder images,
worker/prefill images, and runtime-version annotations. Catalog names, descriptions,
display names, tags, documentation URLs, model formats, and accelerator capabilities
come from the rendered templates. Image selection prefers the main inference
container over sidecars/init containers.

Versions come from an explicit template override, then an image parameter's paired
upstream-version value (explicit or SBOM-discovered), then the rendered
`opendatahub.io/runtime-version` annotation, then the runtime image tag. For an
image pinned only by digest without a runtime annotation, the CLI inspects its
`org.opencontainers.image.version` or `version` label. If neither exists, an explicit
version override is required. The controller image's version is never used as the
runtime version. Multiple configured releases of the same named template contribute
versions to one entry; duplicate versions are rejected. Runtime-level metadata
comes from the first configured source for that name.

Generated LLMInferenceServiceConfig object names replace the leading `kserve-`
with the resolved runtime version. OpenShift Template names and the names of all
nested ServingRuntime objects also receive a runtime-version prefix, replacing
`kserve-` when present. Other nested objects and OpenShift template parameters
retain their names. The version is lowercased, punctuation and other
non-alphanumeric runs become hyphens, and leading/trailing separators are removed.
Build metadata after `+` and already normalized `-rhaiv-…` suffixes are removed
from the name before truncation; release qualifiers such as `ea` are
retained. For example, `kserve-config-llm-template-amd-rocm` with version
`0.26.0+rhaiv.7` becomes `0-26-0-config-llm-template-amd-rocm`. The catalog version
and runtime-version annotation retain the full version including build metadata.
For a CUDA Template at `0.26.0+rhaiv.8`, `vllm-cuda-runtime-template` becomes
`0-26-0-vllm-cuda-runtime-template` and its nested `vllm-cuda-runtime` becomes
`0-26-0-vllm-cuda-runtime`. Names exceeding the [Kubernetes DNS subdomain limit](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)
of 253 characters are shortened with a deterministic hash suffix. Versions that
sanitize to the same object name fail generation. Catalog/index identifiers stay
stable so different runtime versions can share one entry; each version's manifest
has its own object name. Generation collects all configured sources before writing
files, so multiple controller releases contribute to the same runtime's `versions`
list rather than replacing earlier releases. Colliding names of the same kind and
namespace fail generation, including ServingRuntime names inside distinct Templates.

## Upstream vLLM versions and SBOM attestations

For vLLM repositories on `registry.redhat.io`, the CLI fills missing
`<image-key>-upstream-version` parameters automatically. It downloads attestations
by image digest using `cosign download attestation`, decodes their DSSE payloads,
selects the SPDX predicate type `https://spdx.dev/Document`, and checks that the
statement's subject digest matches the requested image. The version comes from
the package named exactly `vllm` and its SPDX `versionInfo`, preserving build
metadata such as `+rhaiv.8`. Product image labels are not upstream vLLM versions.

An index SBOM can list architecture images rather than the vLLM package itself.
The generator follows OCI package URLs with `arch` qualifiers and SHA-256 digests,
fetching those architecture attestations from the original Red Hat repository.
All architecture versions must agree; conflicting versions, cycles, missing
packages, or missing matching SPDX attestations fail generation. Downloads are
cached across sources during one generation. Tagged images are resolved to a
digest before querying attestations; digest pins remain preferable for repeatable
output.

The workflow in [RHOAI-Build-Config PR #18459](https://github.com/red-hat-data-services/RHOAI-Build-Config/pull/18459)
uses the same SPDX package/architecture discovery, but its `cosign download sbom`
command accesses standalone attachments. Cosign deprecated that attachment
workflow because its image relationship and separate verification/download steps
have weaknesses; an attestation includes the image subject and SBOM in a signed
statement. See [Cosign issue #2755](https://github.com/sigstore/cosign/issues/2755).
The generator uses the attestation command exclusively. If an older image only
publishes standalone attachments, supply its upstream version explicitly:

```yaml
parameters:
  vllm-cuda-image: registry.redhat.io/rhaii/vllm-cuda-rhel9@sha256:...
  vllm-cuda-image-upstream-version: 0.26.0+rhaiv.8
```

Attestation downloading and subject matching do **not** verify signatures or
establish publisher trust. This generator discovers catalog metadata; it does not
claim verified SBOM provenance. Cryptographic verification requires
`cosign verify-attestation` with the publisher's trusted public key or certificate
identity/issuer policy, as described in the
[Sigstore verification documentation](https://docs.sigstore.dev/cosign/verifying/attestation/).

Skopeo and Cosign use their normal registry credential discovery. When an explicit
`--authfile` is supplied, the generator provides Cosign a private temporary Docker
config containing those credentials and removes it after the command finishes.
Credentials are never passed in command arguments.

## Source-image resolution

Each source requires exactly one of:

- `target_image`: the binary/controller image whose matching sources are needed.
- `source_image`: an explicit source container reference, bypassing discovery.
- `directory`: an existing checkout or a directory containing templates directly.

For `target_image`, skopeo inspects the binary image without downloading its layers.
The generator tries the digest attachment tag `sha256-<digest>.src`, then the
binary tag with `-source`, then `<version>-<release>-source` from the image labels.
[Red Hat documents the source-tag convention and skopeo download workflow](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/epub/building_running_and_managing_containers/getting-ubi-container-image-source-code_adding-software-to-a-running-ubi-container).
The exact example image above was verified to publish both its `.src` attachment
and `v3.6.0-ea.1-1788535633-source` tag.

Skopeo copies the source image using its normal credential discovery, or an explicit
`--authfile` (`SERVING_RUNTIME_AUTHFILE` for make). Extraction handles gzip/plain tar
layers, nested source archives, and Red Hat `blobs/sha256/*` source payloads.
Only configuration YAML and `params.env` files are retained; archive links are
ignored and unsafe paths/conflicting templates fail generation. Containers are
never run. Kustomize renders the copied configuration tree.

Source manifests are resolved to SHA digests before copying. During one generation,
the loader caches the pristine extracted configuration by source-content digest
and selected extraction roots. Entries that resolve to identical content download
and extract it once, including different tag aliases or repositories. Each entry
renders its own temporary copy with its configured parameters. Failed extraction
does not create a cache entry. The cache is deleted at the end of the invocation;
there is no persistent cache, cache flag, expiry policy, or reuse across runs.
Distinct source images still require their own download on each run.

A previously downloaded skopeo directory can be reused offline:

```yaml
source_image: dir:/absolute/path/to/skopeo-source-directory
```

A source image may be large because it also contains dependency sources. An
explicit source image pinned by digest provides reproducible acquisition. Offline
catalog checks use the committed generated inputs and require neither registry
access nor skopeo/kustomize.

## Optional overrides

The standard conventions require no template mapping in config. For a different
layout or a release-specific exception, a source can provide:

```yaml
template_paths: [config/runtimes] # Overrides automatic directory discovery.
kustomize_overlay: config/base  # Overrides automatic overlay discovery.
params_env: config/base/params.env # Defaults to <overlay>/params.env.
include: [config/runtimes/vllm/vllm-cuda-template.yaml] # Optional path.Match patterns.
templates:
  config/runtimes/vllm/vllm-cuda-template.yaml:
    name: vllm-cuda
    version: v0.21.0
    minimum_rhoai_version: "3.6"
```

Template override paths are relative to the source root, after stripping the
repository/commit prefix in source archives. Include patterns match those paths;
unmatched patterns/overrides fail generation. Discovery is recursive without a
fixed directory depth. Kustomize transforms templates selected from the source
roots; extra resources and fast variants outside those roots are not indexed.

For a directory of raw templates without an upstream overlay, use `replacements`
to map exact image field values (for example, `$(vllm-cuda-image)`) to pinned images.
Per-template `image` replaces `placeholder` fields and selects the version's primary
image; per-template `replacements` can override source mappings. These fallback
replacements affect only image values, preserving arguments such as `{{.Name}}`
and unrelated template parameters. Every resulting image field must be pinned,
fully qualified. A reference using `latest` must also include a digest.

## Review and verification

Image selection requires maintainer/product review. Generator configuration has
no manual `support_level` field. Each generated version is classified using the
target release and the images actually present in its rendered template:

- A target tag containing an `ea` release token (for example,
  `v3.6.0-ea.1-1788535633`) produces `techPreview`.
- A runtime in `rhaii-fast` produces `techPreview`, including digest-pinned images.
- A stable target and stable runtime images under `rhoai`, `rhaii`, or the older
  `rhaiis` namespace on Red Hat registries produce `supported`.
- If any runtime image is a preview or unknown image, the entry is `techPreview`.
  Worker, prefill, and auxiliary container images contribute to this decision.
- Directory-only sources and digest-only targets without a release tag produce
  `techPreview`, since their target release cannot be classified from a tag.

The rule uses the final rendered image references, incorporating `params.env`
defaults, configured parameter overrides, and per-template image replacements.
An unused fast-image parameter does not downgrade an unrelated template. EA/fast
release tokens are matched as tag components, not arbitrary substrings. The
catalog generator implements this release classification without querying the
Ecosystem Catalog during generation. See the
[stable vLLM image](https://catalog.redhat.com/en/software/containers/rhaii/vllm-cuda-rhel9/69a57f8e94c1b4cc1eca015b)
and [fast vLLM images](https://catalog.redhat.com/en/search?q=rhaii-fast).

Existing manually curated input files remain compatible with all four loader
support levels (`supported`, `techPreview`, `developerPreview`, `community`).
The superseded dummy-image examples have been removed from the repository.

```bash
# Reproduce the catalog from committed index/runtime inputs, with no network:
make process-serving-runtimes
make check-serving-runtimes

# Validate catalog fields against the pinned model-registry OpenAPI schemas:
make validate-serving-runtimes

# Lint the generated index, inputs, and catalog YAML (requires yamllint):
make lint-serving-runtimes

# Check source-derived index, runtime inputs, and catalog against the config:
go run ./cmd/serving-runtime-catalog \
  --config input/serving_runtimes/generator-config.yaml --skip-unavailable-images --check

# Alternate config or output paths:
make generate-serving-runtimes SERVING_RUNTIME_GENERATOR_CONFIG_PATH=release-config.yaml
# Optionally omit unavailable controller sources and runtime parameter images:
make generate-serving-runtimes SERVING_RUNTIME_SKIP_UNAVAILABLE_IMAGES=true
```

Configuration and all generated content are validated before output files are
written. `--check` writes nothing and reports stale/missing artifacts. Existing
catalog validation also checks unique names/versions, manifest shape, model
formats, protocol versions, environment defaults, and resource quantities.
Every deployment image must start with `registry.redhat.io/rhoai/`,
`registry.redhat.io/rhaii/`, `registry.redhat.io/rhaii-early-access/`, or
`registry.redhat.io/rhaii-fast/`. The shared validator checks each version's
`image` and recursively checks every `image` field in its Template and
LLMInferenceServiceConfig, including worker, prefill, init, sidecar, and additional
Template objects. It rejects other registries/namespaces, unresolved image
parameters, and malformed references, reporting the runtime, version, and field
path. This rule runs during generation, offline checks, and API schema validation;
controller source images and unused source defaults are unaffected.
Submit the configuration, generated index/runtime inputs/catalog, and validation
results together for review. Avoid hand-editing generated files; change the source
configuration or upstream templates and regenerate.

### Minimum RHOAI version

Each runtime version defaults `minimumRHOAIVersion` to the `major.minor` release
in its source's controller `target_image` tag. For example,
`v3.6.0-ea.1-1788535633`, `v3.6.4-12345`, and `rhoai-3.6` all produce `"3.6"`.
This applies to both ServingRuntime Templates and LLMInferenceServiceConfigs.
Multiple controller releases sharing one runtime entry keep independent minimums.
The runtime container version and SBOM do not establish the RHOAI minimum.

Use a RHOAI release tag for `target_image`; a reference containing both the tag
and digest also works. Digest-only references and non-release tags cannot supply
this default and leave the field unset. A per-template `minimum_rhoai_version`
override takes precedence for the version generated by that source. The generator
config includes commented examples for both controller components.

The default is a release-based assumption. Use an override when compatibility
testing establishes a different minimum. `minimumRHOAIVersion` is currently a
catalog extension: the pinned model-registry API schema and YAML loader do not
expose it or enforce a deployment restriction.

### API schema validation

`make validate-serving-runtimes` checks the generated catalog against unmodified
copies of the [model-registry serving-runtime API schema](https://github.com/opendatahub-io/model-registry/blob/16f777698230d1ee76d1b5745336839d54edd82f/api/openapi/src/plugins/serving_runtime-v1.yaml)
and its shared definitions, pinned at commit
`16f777698230d1ee76d1b5745336839d54edd82f`. Both are embedded in the Go validator;
no GitHub or registry access is needed after Go dependencies are installed.
`make check-serving-runtimes` includes this validation before checking freshness,
and the Go tests validate the committed catalog too.

The schema describes runtime and version API objects, rather than the catalog
wrapper. The validator checks each runtime without its nested `versions`, then
each version with `artifactType: serving-runtime-version`, which the loader adds.
It checks original YAML types, required properties, support enums, nested object
schemas, string lengths, URI/date-time formats, and local catalog constraints.
Unknown fields fail even where the upstream schema permits additional properties.
`minimumRHOAIVersion` is validated locally as a release string and reported
separately; it is not claimed as an upstream API field.

The API schema treats both embedded manifest fields as strings. Existing local
checks validate their JSON structure and expected kinds, but these checks do not
validate Kubernetes CRD schemas or prove deployability. The API description of
`servingRuntimeTemplate` describes a ServingRuntime manifest; this catalog carries
OpenShift Template wrappers containing ServingRuntime objects, which consumers
must process.

See [the schema provenance and update instructions](../internal/catalog/serving_runtime_schema/README.md)
when refreshing the upstream revision. To validate another catalog:

```bash
go run ./cmd/serving-runtime-validate --catalog path/to/catalog.yaml
```

### Floating runtime image parameters

Image values in `sources[].parameters` may use floating tags, including `latest`.
Generation inspects each distinct image tag once, caches the result across all
controller sources, and substitutes `repository:tag@sha256:digest` into the
temporary `params.env`. Both Kustomize and SBOM discovery therefore use the same
image content. The digest comes from the top-level manifest, preserving
multi-architecture image indexes. Existing digest references remain unchanged.
Explicit upstream-version parameters bypass SBOM discovery but still resolve tags.
Image overrides and substitutions that reuse a configured parameter image are
updated to the same resolved reference too.

For example, a configuration value can be:

```yaml
parameters:
  vllm-cuda-image: registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6
```

The generated image becomes
`registry.redhat.io/rhaii/vllm-cuda-rhel9:3.6@sha256:<resolved-digest>`.
The tag remains visible for provenance and support inference; Kubernetes
[uses the digest for pulling](https://kubernetes.io/docs/concepts/containers/images/#image-names).
The generator configuration keeps its floating tag. Regenerating later may pick
up a different digest and version, while the committed YAML stays fixed until
regeneration. Offline catalog checks do not resolve tags again; a source-based
`--check` does and may report changed outputs.

Controller `target_image` references also accept floating release tags such as
`quay.io/rhoai/odh-kserve-llmisvc-controller-rhel9:rhoai-3.6`. Source discovery
uses the resolved controller digest for its preferred `.src` attachment lookup.
The `rhoai-3.6` tag also supplies the `"3.6"` minimum default. Each controller
source still needs its own runtime parameter overrides; target images do not
populate those deployment images automatically.
