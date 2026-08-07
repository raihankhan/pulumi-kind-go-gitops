# Day-2 operations

## Re-running the bootstrap

The bootstrap script is idempotent:

```bash
make up
```

This will:
- Re-apply any drifted Helm releases.
- Reconcile the root Application.
- Print fresh credentials.

## Adding a new workload (GitOps path)

```bash
cd $APPS_REPO
mkdir apps/grafana
kubectl kustomize <(echo '---'; echo 'apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - github.com/grafana/grafana//manifests/base?ref=v11.1.0') > apps/grafana/all.yaml
git commit -am "feat: grafana"
```

Argo CD will pick it up automatically because the root Application uses
`directory.recurse: true`.

## Adding a new workload (Pulumi path)

For stateful workloads that should be managed by Pulumi instead of Argo CD
(e.g. databases, secrets, infrastructure):

```bash
mkdir -p apps/pulumi/<name>
cat > apps/pulumi/<name>/stack.yaml <<'EOF'
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: <name>
  namespace: pulumi-kubernetes-operator
spec:
  serviceAccountName: pulumi-kubernetes-operator
  programRefs:
    - name: <repo-name>
      repoURL: https://github.com/<org>/<repo>
      commit: main
  backend: file:///pulumi-operator/state
  config:
    secrets:
      pulumiConfig:
        foo:
          secure: REPLACE
EOF
kubectl apply -f apps/pulumi/<name>/stack.yaml
```

## Disabling self-heal temporarily

Sometimes you want to debug a workload by hand:

```bash
kubectl -n argocd patch application platform-root \
  --type merge \
  -p '{"spec":{"syncPolicy":{"automated":{"selfHeal":false}}}}'
```

Re-enable with:

```bash
kubectl -n argocd patch application platform-root \
  --type merge \
  -p '{"spec":{"syncPolicy":{"automated":{"selfHeal":true}}}}'
```

## Force a sync

```bash
kubectl -n argocd annotate application platform-root \
  argocd.argoproj.io/refresh=hard --overwrite
```

## Disasters

| Disaster                                       | Recovery                                                                       |
| ---------------------------------------------- | ------------------------------------------------------------------------------ |
| Argo CD crashed                                | `make destroy && make up` (state is local)                                     |
| kind cluster deleted                           | `make kind-delete && make up`                                                  |
| Pulumi state lost                              | Re-run `make up` with the same stack — Pulumi will reapply every resource      |
| apps repo unreachable from Argo CD             | Check `kubectl -n argocd get applications` and the Application's "Conditions"  |
| Apps in OutOfSync forever                      | `kubectl -n argocd annotate application ... argocd.argoproj.io/refresh=hard`   |
| Operator stuck in CrashLoopBackOff             | `kubectl -n pulumi-kubernetes-operator logs -l app=pulumi-kubernetes-operator` |

## Reset everything

```bash
make destroy
make kind-delete
make up
```

## Inspecting the cluster

```bash
# Top-level summary
kubectl get all,ing,pdb,hpa -A --context kind-pulumi-gitops

# Tree view of the platform stack
kubectl tree deploy -n pulumi-kubernetes-operator pulumi-kubernetes-operator 2>/dev/null \
  || kubectl describe deploy -n pulumi-kubernetes-operator pulumi-kubernetes-operator

# Argo CD app tree
argocd app resources platform-root --core   # if argocd CLI installed
```

## Adding TLS to the ingress

> **Local dev note**: TLS in kind typically requires `mkcert` or self-signed
> certs. For most local development the insecure NodePort is sufficient.

1. Install cert-manager via Pulumi (uncomment the cert-manager Helm release).
2. Create a `ClusterIssuer` for `selfsigned` in `manifests/platform/`.
3. Update `manifests/nginx/40-ingress.yaml` to use `tls` with `secretName: nginx-tls`.

## Upgrading pinned versions

1. Bump the version constant in `cmd/pulumi/main.go`.
2. Run `make build && make up` — Pulumi will reconcile the new chart release.
3. Commit the bump.
4. Verify Argo CD reconciles cleanly: `kubectl -n argocd get applications`.
