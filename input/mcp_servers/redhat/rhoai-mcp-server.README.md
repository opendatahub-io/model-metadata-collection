# RHOAI MCP Server

MCP server for Red Hat OpenShift AI (RHOAI). Enables AI agents to manage Data Science Projects, Jupyter workbenches, model serving, pipelines, data connections, storage, training, and Model Registry.

- **Source**: https://github.com/opendatahub-io/rhoai-mcp
- **Container**: `quay.io/opendatahub/odh-rhoai-mcp:odh-stable`
- **License**: MIT
- **Transports**: stdio, sse, streamable-http

## Deployment

Deploy using Kustomize:

```bash
# Base deployment (creates namespace, SA, ClusterRole, Deployment, Service)
kubectl apply -k https://github.com/opendatahub-io/rhoai-mcp/deploy/kustomize/base

# Or with OpenShift overlay (adds Route + OIDC auth)
kubectl apply -k https://github.com/opendatahub-io/rhoai-mcp/deploy/kustomize/overlays/openshift
```

## Required RBAC

The server needs a ServiceAccount with a ClusterRole granting access to RHOAI resources. The default ClusterRole (`rhoai-mcp`) is included in the Kustomize base.

### ClusterRole rules

| API Group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `""` (core) | namespaces | get, list, watch, create, patch, delete | Data Science Project management |
| `""` (core) | secrets, persistentvolumeclaims | get, list, watch, create, delete | Data connections and storage |
| `""` (core) | pods, pods/log, services, events, nodes | get, list, watch | Cluster exploration |
| `""` (core) | pods | create | PVC permission fix jobs |
| `""` (core) | persistentvolumes | create | NFS storage setup |
| `kubeflow.org` | notebooks | get, list, watch, create, patch, delete | Jupyter workbenches |
| `trainer.kubeflow.org` | trainjobs | get, list, watch, create, update, patch, delete | Model training |
| `trainer.kubeflow.org` | trainingruntimes | get, list, watch | Training runtime discovery |
| `trainer.kubeflow.org` | clustertrainingruntimes | get, list, watch, create, delete | Cluster training runtimes |
| `datasciencepipelinesapplications.opendatahub.io` | datasciencepipelinesapplications | get, list, watch, create, delete | Data Science Pipelines |
| `serving.kserve.io` | inferenceservices | get, list, watch, create, delete | Model serving |
| `serving.kserve.io` | servingruntimes | get, list, watch, create | Serving runtime management |
| `storage.k8s.io` | storageclasses | list | NFS storage discovery |

### Read-only mode

Set `RHOAI_MCP_READ_ONLY_MODE=true` to restrict the server to read-only operations. In this mode, all write verbs (create, patch, update, delete) are blocked at the application level regardless of RBAC permissions.

### Minimal read-only ClusterRole

For read-only deployments, a narrower ClusterRole suffices:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: rhoai-mcp-readonly
rules:
  - apiGroups: [""]
    resources: [namespaces, pods, pods/log, services, events, nodes, secrets, persistentvolumeclaims]
    verbs: [get, list, watch]
  - apiGroups: [kubeflow.org]
    resources: [notebooks]
    verbs: [get, list, watch]
  - apiGroups: [trainer.kubeflow.org]
    resources: [trainjobs, trainingruntimes, clustertrainingruntimes]
    verbs: [get, list, watch]
  - apiGroups: [datasciencepipelinesapplications.opendatahub.io]
    resources: [datasciencepipelinesapplications]
    verbs: [get, list, watch]
  - apiGroups: [serving.kserve.io]
    resources: [inferenceservices, servingruntimes]
    verbs: [get, list, watch]
  - apiGroups: [storage.k8s.io]
    resources: [storageclasses]
    verbs: [list]
```

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

46 tools across 8 domains and 3 composites (31 read-only, 15 write). See `rhoai-mcp-server.yaml` for the full tool listing with parameters.
