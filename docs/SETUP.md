# Setup guide

This guide takes you from **"only kind is installed"** to a working platform in about 10 minutes.

> These instructions assume macOS or Linux. If you're on Windows, use WSL2.

---

## 0. Prerequisites

| Tool       | Why                  | Install                                           |
| ---------- | -------------------- | ------------------------------------------------- |
| `kind`     | Local cluster        | `brew install kind` / [kind releases](https://kind.sigs.k8s.io/)        |
| `kubectl`  | Cluster client       | `brew install kubectl`                            |
| `helm`     | Argo CD + ingress    | `brew install helm`                               |
| `pulumi`   | IaC engine           | `curl -fsSL https://get.pulumi.com \| sh`         |
| `go 1.25+` | Build the Pulumi program | `brew install go` or https://go.dev |
| `docker`   | Runtime for kind     | Docker Desktop / OrbStack / Colima                |

Or run `./scripts/install-deps.sh` to install everything on macOS/Ubuntu.

---

## 1. Clone this repo

```bash
git clone https://github.com/raihankhan/pulumi-kind-go-gitops.git
cd pulumi-kind-go-gitops
go mod download
make build            # compiles the Pulumi program into ./bin/platform
```

---

## 2. Create the apps repository (the GitOps source of truth)

Argo CD will reconcile workloads from this repo. Initialize it **once** on GitHub:

```bash
APPS_REPO=pulumi-kind-go-gitops-apps
mkdir -p /tmp/$APPS_REPO && cd /tmp/$APPS_REPO
git init -b main
mkdir -p apps/nginx

# Copy the manifests from this repo
cp -R /path/to/pulumi-kind-go-gitops/manifests/nginx/* apps/nginx/

git add . && git commit -m "feat: nginx gitops workload"
gh repo create raihanraka/$APPS_REPO --public --source=. --remote=origin --push
```

> **Private repo?** See `manifests/argocd/30-repo-secret.example.yaml` and run the
> annotated `kubectl create secret` commands there before `make up`.

---

## 3. Configure Pulumi inputs

```bash
pulumi login --local   # or `pulumi login` if you use Pulumi Cloud

pulumi stack init dev  # if you don't already have one

pulumi config set clusterName "kind-pulumi-gitops"
pulumi config set githubRepo "https://github.com/raihankhan/pulumi-kind-go-gitops-apps.git"
pulumi config set githubBranch main
pulumi config set ingressClass nginx
pulumi config set argocdDomain argocd.local
```

---

## 4. Bootstrap

```bash
make up
```

This runs `scripts/bootstrap.sh` which:

1. Creates the kind cluster (`kind/kind-config.yaml`)
2. Initializes the Pulumi stack (if missing)
3. Runs `pulumi up` → installs Operator + ingress-nginx + Argo CD
4. Applies the AppProject + root Application
5. Prints connection info

---

## 5. Verify

```bash
# Cluster
kubectl get nodes --context kind-pulumi-gitops

# Operator
kubectl -n pulumi-kubernetes-operator get pods

# Argo CD apps
kubectl -n argocd get applications
# (within ~30s the root Application should report "Synced" once the apps repo is reachable)

# nginx
kubectl -n nginx get pods,svc,ingress
```

### Ingress access

For local development we expose two NodePorts on the control-plane container
(both mapped to the host):

| Port   | Purpose                                  |
| ------ | ---------------------------------------- |
| 30080  | Plain HTTP ingress                       |
| 30443  | HTTPS ingress / Argo CD insecure server |

Option 1 — hostnames (preferred):

```bash
sudo sh -c 'echo "127.0.0.1 nginx.local argocd.local" >> /etc/hosts'
curl -H "Host: nginx.local" http://localhost:30080
```

Option 2 — directly via host port:

```bash
curl http://localhost:30080
```

### Argo CD admin password

```bash
make argocd-pw
```

The initial secret is removed on first login. Argo CD is intentionally exposed
**insecure** (TLS is disabled) for local dev; do not reuse this configuration
outside kind.

---

## 6. Day-2

| Task                            | Command                                                             |
| ------------------------------- | ------------------------------------------------------------------- |
| Sync all apps                   | `kubectl -n argocd annotate application platform-root argocd.argoproj.io/refresh=hard` |
| Re-run Pulumi                   | `make up`                                                           |
| Tear down platform              | `make destroy`                                                      |
| Delete cluster                  | `make kind-delete`                                                  |
| Tail operator logs              | `kubectl -n pulumi-kubernetes-operator logs -l app.kubernetes.io/name=pulumi-kubernetes-operator --tail=100 -f` |
| Tail Argo CD logs               | `kubectl -n argocd logs -l app.kubernetes.io/name=argocd-application-controller --tail=100 -f` |

See `docs/OPERATIONS.md` for richer recovery procedures.
