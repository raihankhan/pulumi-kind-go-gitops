<div align="center">

# pulumi-kind-go-gitops

**Production-shaped local Kubernetes, on a single machine.**

`kind` → `Pulumi (Go)` → `Pulumi Operator` → `Argo CD` → `nginx` — reconciled from GitHub, written in Go, ready in one command.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](#)
[![Pulumi](https://img.shields.io/badge/Pulumi-Go%20IAC-8A33B6?logo=pulumi&logoColor=white)](#)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.30-326CE5?logo=kubernetes&logoColor=white)](#)
[![Argo%20CD](https://img.shields.io/badge/Argo%20CD-GitOps-EF7B4D?logo=argo&logoColor=white)](#)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://img.shields.io/badge/CI-master-success?logo=githubactions&logoColor=white)](#)

</div>

---

## Why this exists

Most "local Kubernetes" demos stop at `kind create cluster`. This project goes further: it bootstraps a **production-shaped platform** on `kind`, then runs a real **nginx workload** through a real **GitOps pipeline** — all from a single `make up`.

If you've ever wanted to prove out a GitOps flow on your laptop, debug an Argo CD sync, or hand a new hire a working platform in ten minutes — this is it.

---

## Architecture at a glance

### Platform layer — installed by Pulumi (this repo)

```
   Pulumi program (Go)                  kind cluster (3 control-plane + 3 worker)
   ──────────────────                  ─────────────────────────────────────────
   ┌───────────────────┐                ┌──────────────────────────────────────────┐
   │                   │     helm       │                                          │
   │   cmd/pulumi/     │    install     │   ┌─────────────────┐ ┌────────────────┐ │
   │   main.go         │ ─────────────▶ │   │ Pulumi          │ │ ingress-nginx  │ │
   │                   │                │   │ Kubernetes      │ │ class: nginx   │ │
   │   Argo CD · PKO   │                │   │ Operator        │ │ NodePort 30080 │ │
   │   ingress-nginx   │                │   │ leader election │ │ default class  │ │
   │                   │                │   └────────┬────────┘ └────────────────┘ │
   └───────────────────┘                │            │                             │
                                        │   ┌────────▼─────────┐ ┌────────────────┐ │
                                        │   │ Argo CD          │ │ platform-root  │ │
                                        │   │ server, repo,    │ │ Application    │ │
                                        │   │ controller       │ │ recurse=true   │ │
                                        │   └──────────────────┘ └────────────────┘ │
                                        │                                          │
                                        └──────────────────────────────────────────┘
```

### Workload layer — reconciled by Argo CD (apps repo)

```
   GitHub: apps repo                              Argo CD
   ────────────────                               ───────
   ┌──────────────────────────────┐   git pull   ┌──────────────────────────────┐
   │ pulumi-kind-go-gitops-apps   │ ───────────▶ │ platform-root Application     │
   │                              │              │ source.path: apps             │
   │ apps/                         │              │ directory.recurse: true       │
   │  ├── nginx/                   │              └─────────────┬────────────────┘
   │  │   ├── kustomization.yaml   │                            │
   │  │   └── 00..70.yaml (8)      │   creates 1 Application    │
   │  └── <future>/                │ ──────────▶  per directory │
   │                              │                            ▼
   │                              │              ┌──────────────────────────────┐
   │                              │              │ kind cluster                 │
   │                              │              │  - apps/nginx → nginx ns     │
   │                              │              │  - apps/<new>  → <new>  ns   │
   └──────────────────────────────┘              └──────────────────────────────┘
```

### Request flow

```
   Browser           ingress-nginx           Service               Pods
   ───────           ──────────────           ───────               ────
      │              (NodePort :80)         (ClusterIP :80)         (3x)
      │                  │                       │                   │
      │ GET nginx.local  │                       │                   │
      ├─────────────────▶│  HTTP :80             │                   │
      │                  ├──────────────────────▶│ ClusterIP         │
      │                  │                       ├──────────────────▶│
      │                  │                       │ endpoint selected │
      │                  │                       │ (iproute2 least-conn)
      │◀─────────────────┼───────────────────────┼───────────────────┤
      │ 200 OK           │                       │                   │
```

---

## What's in the box

| Layer | Path | What it does |
|---|---|---|
| Cluster | `kind/kind-config.yaml` | 6-node kind cluster (3 control-plane + 3 worker), port mappings, registry hook |
| IaC | `cmd/pulumi/main.go` | Go Pulumi program — installs Operator + ingress + Argo CD |
| Tooling | `cmd/kindctl/main.go` | Tiny CLI around `kind create/delete/status` |
| Abstraction | `internal/cluster/cluster.go` | Provider-agnostic cluster lifecycle with unit tests |
| Operator | `manifests/pulumi-operator/` | Namespace, RBAC, secret template, Helm values |
| GitOps | `manifests/argocd/` | AppProject, root Application, private-repo secret example |
| Workload | `manifests/nginx/` | Hardened nginx: Deployment, ConfigMap, Service, Ingress, HPA, PDB, ServiceMonitor |
| Scripts | `scripts/bootstrap.sh` | Idempotent end-to-end bootstrap |
| Scripts | `scripts/install-deps.sh` | Install kind, kubectl, helm, pulumi, Go |
| Build | `Makefile` | `build`, `test`, `up`, `destroy`, `kind-create/delete`, `argocd-pw` |
| Docs | `docs/SETUP.md` | Step-by-step from `kind` to platform |
| Docs | `docs/ARCHITECTURE.md` | Layer model, design decisions, security posture |
| Docs | `docs/OPERATIONS.md` | Day-2 ops, sync, debugging, disaster recovery |
| Docs | `docs/PULUMI_OPERATOR.md` | Stack CR usage, backends, debugging |
| CI | `.github/workflows/ci.yaml` | Build + vet + test on every push/PR to `master` |

---

## Quick start

> **Prereqs:** `kind`, `kubectl`, `helm`, `pulumi`, `go 1.25+`, and a container runtime (Docker / OrbStack / Colima).

```bash
# 1. Clone and build
git clone https://github.com/raihankhan/pulumi-kind-go-gitops.git
cd pulumi-kind-go-gitops
go mod download
make build

# 2. Bootstrap end-to-end
make up

# 3. Open Argo CD
make argocd-pw        # initial admin password
open http://localhost:30080   # insecure local mode, NodePort 30080

# 4. Reach nginx through ingress
echo "127.0.0.1 nginx.local argocd.local" | sudo tee -a /etc/hosts
curl http://nginx.local
```

Need more detail? See **[SETUP.md](docs/SETUP.md)** · **[OPERATIONS.md](docs/OPERATIONS.md)** · **[ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

---

## The apps repo (the GitOps source-of-truth)

Argo CD reconciles workloads from a separate repository. Mirror `manifests/nginx/` into `apps/nginx/` in your apps repo and Argo CD picks it up automatically.

```
apps/
└── nginx/
    ├── kustomization.yaml
    ├── 00-namespace.yaml
    ├── 10-deployment.yaml
    ├── 20-config.yaml
    ├── 30-service.yaml
    ├── 40-ingress.yaml
    ├── 50-hpa.yaml
    ├── 60-pdb.yaml
    └── 70-servicemonitor.yaml
```

`directory.recurse: true` on the root Application means **drop in a new directory, get a new reconciled workload** — no platform re-run needed.

---

## Pinned versions

| Component | Version | Why it matters |
|---|---|---|
| Kubernetes (kind node image) | `v1.30.0` | Stable, broadly supported by operators |
| Pulumi Kubernetes Provider | `v4.33.0` | Helm v4 API + native K8s types |
| Pulumi Go SDK | `v3.245.0` | LTS line, no breaking changes mid-stream |
| Pulumi Kubernetes Operator | `v2.5.0` | Watches `Stack` CRs, leader election on |
| Argo CD | `v7.6.12` | Server, repo-server, app-controller all metric-enabled |
| ingress-nginx | `v4.11.3` | Default ingress class, NodePort 30080/30443 |
| nginx (sample workload) | `1.27-alpine` | Minimal image, runs as non-root (uid 101) |

> Bumping a pinned version is a single-PR change: edit the constant in `cmd/pulumi/main.go`, commit, `make up`.

---

## Production-readiness checklist

A system-architect's eye, not a marketing checklist:

**Workload hardening** — the nginx pod runs with:
- `runAsNonRoot: true`, `runAsUser: 101`, `fsGroup: 101`
- `readOnlyRootFilesystem: true`, `tmpfs`/`emptyDir` for writable paths
- `allowPrivilegeEscalation: false`, drop ALL capabilities
- `seccompProfile: RuntimeDefault`
- `automountServiceAccountToken: false`
- PSA namespace label `enforce=baseline, audit=restricted`

**Reliability** — non-trivial for "just nginx":
- 3-replica Deployment with `RollingUpdate` (`maxUnavailable: 0`)
- `HorizontalPodAutoscaler` (3–10 pods, CPU 70%)
- `PodDisruptionBudget` (`minAvailable: 2`)
- `preStop` hook for graceful drain
- Liveness + readiness probes on both `/` and `/healthz`

**GitOps guardrails** — `AppProject` with no wildcards:
- Explicit `sourceRepos`, `destinations`, `namespaceResourceWhitelist`
- `automated.prune + selfHeal` on the root Application
- `ServerSideApply`, `PruneLast`, retry with exponential backoff
- `revisionHistoryLimit: 10` to bound history

**Secret hygiene** — never in git:
- Pulumi secrets via `pulumi config set --secret`
- Argo CD repo secrets via `kubectl create secret generic`
- Reference templates at `manifests/argocd/30-repo-secret.example.yaml`

**CI** — `.github/workflows/ci.yaml`:
- Single job, fast feedback: `make build` → `go vet ./...` → `make test`
- Concurrency cancel on re-push to `master`
- `permissions: contents: read` only

---

## When to use this, and when not to

| ✅ Use this when | ❌ Don't use this when |
|---|---|
| You're prototyping a GitOps pipeline | You need real multi-node HA (use EKS/GKE) |
| You want to onboard engineers to Argo CD + Pulumi | You're deploying to production (this is local-only) |
| You're rehearsing a disaster-recovery scenario | You need external L4 load balancing |
| You're building a Helm chart and want a fast test bed | You need network policies backed by Cilium (kind uses kindnet) |

---

## Contributing

1. Fork → branch off `master`
2. Edit code in `cmd/`, `internal/`, or manifests in `manifests/`
3. `make verify` (build + test + vet)
4. Push, open a PR
5. CI must pass: build + vet + tests on `master`

For new workloads, add them to the **apps repo**, not here. Argo CD owns that surface.

---

## License

MIT — see [LICENSE](LICENSE).