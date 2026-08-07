# pulumi-kind-go-gitops

Production-shaped local Kubernetes platform: **kind** + **Pulumi Kubernetes Operator** + **Argo CD**, with an **nginx** GitOps workload sourced from a **GitHub** repo.

This is what gets you from "kind is installed" to "I have an nginx pod reconciled by Argo CD, with the Pulumi Operator running alongside it" — all on a single local machine, all written in **Go (Pulumi IaC)**.

```
  ┌─────────────────────┐         ┌─────────────────────┐         ┌─────────────────────┐
  │   GitHub (source)   │ ──git──▶│  Argo CD            │ ──prune▶│  kind cluster       │
  │ pulumi-kind-…-apps  │         │  (App-of-Apps)       │         │  ┌───────────────┐  │
  └─────────────────────┘         └─────────────────────┘         │  │ nginx         │  │
                                                                     │  └───────────────┘  │
  ┌─────────────────────┐         ┌─────────────────────┐           │                     │
  │  Pulumi program     │ ──up──▶ │  Pulumi Kubernetes  │           │  ┌───────────────┐  │
  │  (this repo)        │         │  Operator           │ ◀─────────│  │ Stack CRs     │  │
  └─────────────────────┘         └─────────────────────┘           │  └───────────────┘  │
                                                                     └─────────────────────┘
```

## What's in here

| Path                                      | Purpose                                                                |
| ----------------------------------------- | ---------------------------------------------------------------------- |
| `kind/kind-config.yaml`                   | 6-node kind cluster (3 control-plane + 3 worker) with port mappings   |
| `cmd/pulumi/main.go`                      | Go Pulumi program — installs Operator + ingress-nginx + Argo CD       |
| `cmd/kindctl/main.go`                     | Tiny CLI around `kind create/delete/status`                            |
| `internal/cluster/cluster.go`             | Cluster lifecycle abstraction with a `Provider` interface              |
| `manifests/pulumi-operator/`              | Namespace + RBAC + secret + Helm values for the operator               |
| `manifests/argocd/`                       | AppProject + root Application + private-repo secret template            |
| `manifests/nginx/`                        | Workload manifests for the GitOps apps repo (`apps/nginx/`)            |
| `scripts/bootstrap.sh`                    | One-shot bootstrap: kind → pulumi up → Argo CD reconciliation           |
| `scripts/install-deps.sh`                 | Install kind, kubectl, helm, pulumi, Go                                |
| `Makefile`                                | Common dev targets                                                     |
| `docs/SETUP.md`                           | Step-by-step setup guide                                               |
| `docs/ARCHITECTURE.md`                    | Architecture deep-dive                                                 |
| `docs/OPERATIONS.md`                      | Day-2 operations: sync, debugging, disaster recovery                    |

## Quick start

```bash
# 1. Install deps
./scripts/install-deps.sh

# 2. Bootstrap end-to-end
make up

# 3. Open Argo CD (insecure local mode on NodePort 30080)
open http://localhost:30080

# Get the admin password
make argocd-pw

# Reach nginx through ingress
echo "127.0.0.1 nginx.local" | sudo tee -a /etc/hosts
curl http://nginx.local
```

See `docs/SETUP.md` for the GitHub-repo specifics and `docs/OPERATIONS.md` for day-2 tasks.

## Stack versions

| Component                          | Version          |
| ---------------------------------- | ---------------- |
| Kubernetes (kind node image)       | v1.30.0          |
| Pulumi Kubernetes Provider         | v4.33.0          |
| Pulumi Go SDK                      | v3.245.0         |
| Pulumi Kubernetes Operator         | v2.5.0           |
| Argo CD                            | v7.6.12          |
| ingress-nginx                      | v4.11.3          |
| nginx (sample workload)            | 1.27-alpine      |

## Production-readiness checklist

- ✅ **Hardened securityContext** on the nginx workload (non-root, read-only root FS, drop ALL caps, seccomp RuntimeDefault)
- ✅ **PodDisruptionBudget** (minAvailable=2) + **HorizontalPodAutoscaler**
- ✅ **Rolling update** strategy (`maxUnavailable: 0`) + **pre-stop hook** for graceful drain
- ✅ **Resource requests and limits** for every container
- ✅ **AppProject** scoped to specific repos, namespaces, and resource kinds (no wildcards)
- ✅ **GitOps auto-sync** with prune + selfHeal, plus an explicit retry policy
- ✅ **Namespace pod-security** admission labels (baseline/restricted)
- ✅ **No secrets in git** — Pulumi secrets and Argo CD repo Secrets are sourced from environment or kubectl out-of-band
- ✅ **Automated bootstrap** — single `make up` brings up the full platform
- ✅ **Unit tests** for the cluster lifecycle abstraction

## Layout (apps repo — separate)

The **apps repo** (e.g. `pulumi-kind-go-gitops-apps`) is where Argo CD's source-of-truth lives:

```
apps/
  nginx/
    00-namespace.yaml
    10-deployment.yaml
    20-config.yaml
    30-service.yaml
    40-ingress.yaml
    50-hpa.yaml
    60-pdb.yaml
    70-servicemonitor.yaml
```

The contents in this repo's `manifests/nginx/` are intentionally identical to that `apps/nginx/` path so you can simply copy them.

## License

MIT — see `LICENSE`.
