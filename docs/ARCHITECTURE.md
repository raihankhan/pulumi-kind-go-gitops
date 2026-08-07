# Architecture

This document explains *why* the pieces are arranged the way they are.

## Top-level goals

1. **Reproducible local environment** — a single `make up` brings up the entire platform.
2. **Production-shape defaults** — securityContext, resource limits, PDBs, AppProject scoping.
3. **Two-tier GitOps** — the platform itself is Pulumi-managed; the workloads on top are Argo CD-managed.
4. **Operator-friendly** — `cmd/pulumi/main.go` is a small, easy-to-read Go program; the heavy lifting is Helm and raw manifests.

## Layer model

```
┌──────────────────────────────────────────────────────────────────────┐
│  Layer 4: Workloads (GitOps, Argo CD source-of-truth = GitHub)       │
│  apps/nginx, etc.                                                     │
└──────────────────────────────────────────────────────────────────────┘
                              ▲
                              │  reconciles
                              │
┌──────────────────────────────────────────────────────────────────────┐
│  Layer 3: Continuous-delivery control plane                          │
│  Argo CD core + AppProject + App-of-Apps root Application            │
└──────────────────────────────────────────────────────────────────────┘
                              ▲
                              │  installed by Pulumi
                              │
┌──────────────────────────────────────────────────────────────────────┐
│  Layer 2: Infrastructure automation (Pulumi Kubernetes Operator)    │
│  Stack CRs, RBAC, secrets                                            │
└──────────────────────────────────────────────────────────────────────┘
                              ▲
                              │  installed by Pulumi
                              │
┌──────────────────────────────────────────────────────────────────────┐
│  Layer 1: Cluster + base services (Helm + raw manifests)             │
│  kind, ingress-nginx, cert-manager, Argo CD                          │
└──────────────────────────────────────────────────────────────────────┘
                              ▲
                              │  installed by Pulumi program
                              │
┌──────────────────────────────────────────────────────────────────────┐
│  Layer 0: Container runtime (kind -> Docker)                         │
└──────────────────────────────────────────────────────────────────────┘
```

## Why two IaC systems?

- **Pulumi** excels at *infrastructure provisioning*: Helm releases, raw
  manifests, secret creation, RBAC, and the Pulumi Operator. It is the only
  component that can install Helm charts across the cluster with full
  knowledge of dependencies and outputs.
- **Argo CD** excels at *continuous application reconciliation* — given a Git
  repo, it keeps the cluster in sync, prunes drift, and reports health.
  Putting it on top of Pulumi means platform engineers and application
  engineers each use the tool that fits their workflow.

## Why is the Pulumi Operator installed if we use Argo CD?

The Pulumi Operator is the gateway to running **production-grade infrastructure
pipelines from inside Kubernetes** — for example, if you later want to run
[CrossGuard](https://www.pulumi.com/docs/using-pulumi/crossguard/) policies
or drive external Pulumi stacks via Stack CRs. It also makes the platform
**extensible**: a Stack CR can deploy *additional* workloads (e.g. a database)
without depending on Helm or raw YAMLs.

If you don't need it, comment out the `installPulumiOperator` call in
`cmd/pulumi/main.go` and remove the `manifests/pulumi-operator/` directory.

## Why a single provider tied to `kind-<cluster>`?

We intentionally do **not** use Pulumi's default in-cluster provider. Tying the
provider to `kind-pulumi-gitops` (via `Context:`) means:

- `pulumi destroy` removes all the platform resources from the right cluster.
- The state file accurately reflects the cluster that owns the workloads.
- Anyone running `kubectl` locally will see the exact same view.

## Why are chart versions pinned?

Helm auto-upgrades break GitOps. If `argo-cd` jumps to a new major version
without you committing the bump to Git, your cluster silently moves away from
the source of truth. Pinning to chart versions and bumping via PR keeps the
state file and the GitOps tree aligned.

## Security posture

| Layer     | Control                                                                  |
| --------- | ------------------------------------------------------------------------ |
| Workload  | `runAsNonRoot`, `readOnlyRootFilesystem`, `seccomp: RuntimeDefault`, drop ALL caps |
| Workload  | PDB (minAvailable=2) + HPA                                              |
| Cluster   | Pod Security Admission labels on every namespace (`baseline`/`restricted`) |
| GitOps    | AppProject scoped to specific repos, namespaces, and resource kinds      |
| Secrets   | Pulumi `requireSecret` + Argo CD Repository Secrets (never in git)       |
| Network   | Ingress TLS is intentionally disabled in local; see `OPERATIONS.md` to add |

## Why app-of-apps?

The root `Application` recurses through `apps/` in the source repo, picking up
any subdirectory that contains a `kustomization.yaml` or a plain set of
manifests. To add a new workload:

```bash
mkdir apps/foo
echo "---" > apps/foo/kustomization.yaml
kubectl kustomize ... > apps/foo/all.yaml
git commit -am "feat: foo workload"
```

Argo CD picks it up on the next sync — no Pulumi re-run, no platform engineer
needed.
