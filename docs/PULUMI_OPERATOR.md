# Pulumi Kubernetes Operator — usage notes

This is a short reference for how to use the Pulumi Operator once the
platform is up. It is **not** required for the local dev experience — the
nginx workload is already reconciled by Argo CD — but it is the primary
extension point for moving infra-as-code out of CI and into the cluster.

## What the operator does

The Pulumi Kubernetes Operator reconciles `Stack` CRDs. Each Stack CR points
at a Git repo + path containing a Pulumi program. The operator spawns a
runner pod that runs `pulumi up`, persists state to a backend, and reports
status back to the CR.

## Reference

- **Repo**: https://github.com/pulumi/pulumi-kubernetes-operator
- **Docs**: https://www.pulumi.com/docs/using-pulumi/kubernetes-operator/
- **API**: https://github.com/pulumi/pulumi-kubernetes-operator/blob/master/docs/api.md

## Pre-installed by the bootstrap

- **Namespace**: `pulumi-kubernetes-operator`
- **ServiceAccount**: `pulumi-kubernetes-operator` (cluster-admin binding)
- **Helm release**: `pulumi-operator` v2.5.0
- **CRDs**: `stacks.pulumi.com`, `programs.pulumi.com` (auto-installed by the chart)

## Adding a Stack CR

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: my-stack
  namespace: pulumi-kubernetes-operator
spec:
  serviceAccountName: pulumi-kubernetes-operator
  program:
    # Inline program OR
    programRefs:
      - name: my-stack-program
        repoURL: https://github.com/<org>/<repo>
        commit: master
  backend: file:///pulumi-operator/state
  config:
    secrets:
      pulumiConfig:
        my-secret:
          secure: REPLACE
```

## Backends

| Backend       | When to use                                  |
| ------------- | -------------------------------------------- |
| `file:///…`   | Local dev. State lives in the runner pod.    |
| `s3://…`      | Production. S3 bucket with state locking.    |
| Pulumi Cloud  | Easiest SaaS path. Requires access token.    |

For local dev the operator pod has an `emptyDir` mounted at
`/pulumi-operator/state`. State is **lost when the runner pod is deleted**, so
this is not for production.

## Debugging

```bash
# Operator pod
kubectl -n pulumi-kubernetes-operator get pods
kubectl -n pulumi-kubernetes-operator logs -l app.kubernetes.io/name=pulumi-kubernetes-operator -f

# Runner pods spawned by Stack CRs
kubectl -n pulumi-kubernetes-operator get pods -l app.kubernetes.io/component=stack-runner

# Stack status
kubectl -n pulumi-kubernetes-operator describe stack my-stack
```

## Limitations in local kind

- Runner pods need to `pull` the Pulumi image. If your network is slow, pre-load:
  `docker pull pulumi/pulumi-kubernetes-operator:v2.5.0 && kind load docker-image pulumi/pulumi-kubernetes-operator:v2.5.0 --name pulumi-gitops`
- State on `emptyDir` is ephemeral. Use a persistent volume claim or an S3 backend for anything beyond throwaway tests.
