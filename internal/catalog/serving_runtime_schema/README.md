# Model registry serving runtime schemas

These files are unmodified copies from
[`opendatahub-io/model-registry` at `16f777698230d1ee76d1b5745336839d54edd82f`](https://github.com/opendatahub-io/model-registry/tree/16f777698230d1ee76d1b5745336839d54edd82f):

- [`serving_runtime-v1.yaml`](https://github.com/opendatahub-io/model-registry/blob/16f777698230d1ee76d1b5745336839d54edd82f/api/openapi/src/plugins/serving_runtime-v1.yaml)
- [`common.yaml`](https://github.com/opendatahub-io/model-registry/blob/16f777698230d1ee76d1b5745336839d54edd82f/api/openapi/src/lib/common.yaml)
- [`LICENSE`](https://github.com/opendatahub-io/model-registry/blob/16f777698230d1ee76d1b5745336839d54edd82f/LICENSE)

The validator embeds both YAML files, merges their component schemas, and resolves
local `$ref` links. Validation needs no registry or GitHub access. Do not manually
recreate or edit their field definitions.

SHA-256 checksums:

```text
378005e44a5828321df33e690cac3b62eed24831d6ad03077898f916996ebb54  common.yaml
b5c219e91786b168dd2e99f0150d8d9ed5f4bde003366f6a322361768f359fce  serving_runtime-v1.yaml
```

To update, select an upstream commit and fetch both original paths and its license
at that commit. Update `ServingRuntimeSchemaRevision` in
`../serving_runtime_schema.go`, this provenance record, and the checksums together.
Run `make check-serving-runtimes` and the catalog tests before committing.

The API describes runtimes and versions separately, while the YAML catalog nests
`versions`. The validator projects each entry into its API shape and supplies the
loader's `artifactType: serving-runtime-version`. Our existing validator checks
the catalog wrapper, uniqueness, embedded JSON manifests, and other semantic
constraints. Unknown fields are rejected even though these API resource schemas
permit additional properties.

`minimumRHOAIVersion` is a deliberate catalog extension, validated locally and
reported separately. At this revision it is absent from both the API schema and
the YAML loader; publishing it in the catalog does not make it available through
the registry API or enforce a deployment restriction. If a future upstream schema
defines it, the validator will validate it against that definition automatically.

Manifest fields are strings in the API schema. Passing these checks does not
validate Kubernetes CRD schemas or prove a template can be deployed. The API
description of `servingRuntimeTemplate` refers to a ServingRuntime manifest;
this catalog carries OpenShift Template wrappers containing ServingRuntime
objects. Consumers need to process those wrappers.
