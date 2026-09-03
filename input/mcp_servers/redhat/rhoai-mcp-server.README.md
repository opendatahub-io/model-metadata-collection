# RHOAI MCP Server

MCP server for Red Hat OpenShift AI (RHOAI). Enables AI agents to manage Data Science Projects, Jupyter workbenches, model serving, pipelines, data connections, storage, training, and Model Registry.

- **Source**: https://github.com/opendatahub-io/rhoai-mcp
- **Container**: `registry.redhat.io/rhoai/odh-rhoai-mcp-rhel9:rhoai-3.6-ea.2` (upstream image: `quay.io/opendatahub/odh-rhoai-mcp:odh-stable`)
- **License**: MIT
- **Transports**: stdio, sse, streamable-http

## Deployment

Deploy using Kustomize:

```bash
kubectl apply -k 'https://github.com/opendatahub-io/rhoai-mcp//deploy/kustomize/overlays/openshift-oidc?ref=<version>'
```

Replace `<version>` with `main`, a release tag, or commit SHA (e.g. `?ref=v0.1.1`) to pin to a specific and reviewed version.

This creates the namespace, ServiceAccount, ClusterRole, Deployment, Service, ConfigMaps, and Route with OIDC authentication enabled.

## Required RBAC

The server authenticates users via OIDC/`TokenReview`, then forwards the caller's bearer token for all K8s API calls. The `ServiceAccount` has **no direct resource access** — resource access is evaluated against the authenticated user's own RBAC permissions. MCP tools are filtered per user via `SubjectAccessReview`.

### ClusterRole rules

| API Group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `authentication.k8s.io` | tokenreviews | create | Validate opaque bearer tokens via TokenReview API |
| `authorization.k8s.io` | subjectaccessreviews | create | RBAC-filter MCP tools per user |
| `user.openshift.io` | users | get | Fetch OCP group memberships for authenticated users |

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: rhoai-mcp
rules:
  # Validate opaque bearer tokens via TokenReview API
  - apiGroups: ["authentication.k8s.io"]
    resources: ["tokenreviews"]
    verbs: ["create"]
  # RBAC-filter MCP tools per user via SubjectAccessReview
  - apiGroups: ["authorization.k8s.io"]
    resources: ["subjectaccessreviews"]
    verbs: ["create"]
  # Fetch OCP group memberships for authenticated users
  - apiGroups: ["user.openshift.io"]
    resources: ["users"]
    verbs: ["get"]
```

Each user only sees the MCP tools they have permissions for. A user without `create` on `inferenceservices` will not see the model deployment tools.

> **Note:** An alternative `impersonation` strategy is also supported via `OIDC_KUBE_AUTH_STRATEGY=impersonation`, which uses ServiceAccount credentials with `Impersonate-*` headers instead of forwarding the user's token. This requires adding an `impersonate` rule to the ClusterRole.

### Read-only mode

Set `RHOAI_MCP_READ_ONLY_MODE=true` to restrict the server to read-only operations at the application level, in addition to the user's own RBAC.

## Configuration

Environment variables (all prefixed with `RHOAI_MCP_`):

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_MODE` | `auto` | Authentication mode: `auto` or `kubeconfig` |
| `TRANSPORT` | `stdio` | MCP transport: `stdio`, `sse`, or `streamable-http` |
| `READ_ONLY_MODE` | `false` | Disable all write operations |
| `ENABLE_DANGEROUS_OPERATIONS` | `false` | Enable delete operations |
| `KUBECONFIG_PATH` | `~/.kube/config` | Path to kubeconfig file |
| `KUBECONFIG_CONTEXT` | (current) | Kubeconfig context to use |

## Tools

88 tools across 8 domains and 4 composites (52 read-only, 36 read-write). See `rhoai-mcp-server.yaml` for the full tool listing with parameters.
