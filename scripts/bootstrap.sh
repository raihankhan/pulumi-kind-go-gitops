#!/usr/bin/env bash
set -Eeuo pipefail

# Bootstrap all local dependencies and deploy the platform.
# This script is intentionally idempotent: rerun it after changing Pulumi config.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

CLUSTER_NAME="${KIND_CLUSTER_NAME:-pulumi-gitops}"
KIND_CONFIG="${KIND_CONFIG:-kind/kind-config.yaml}"
PULUMI_PROJECT="${PULUMI_PROJECT:-pulumi-kind-go-gitops}"
PULUMI_STACK="${PULUMI_STACK:-dev}"

log() { printf '\n==> %s\n' "$*"; }
fail() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }

command -v kind >/dev/null 2>&1 || fail "kind is required; run scripts/install-deps.sh"
command -v kubectl >/dev/null 2>&1 || fail "kubectl is required; run scripts/install-deps.sh"
command -v pulumi >/dev/null 2>&1 || fail "pulumi is required; run scripts/install-deps.sh"
command -v helm >/dev/null 2>&1 || fail "helm is required; run scripts/install-deps.sh"

if ! kind get clusters | grep -Fxq "$CLUSTER_NAME"; then
  log "Creating kind cluster: $CLUSTER_NAME"
  kind create cluster --name "$CLUSTER_NAME" --config "$KIND_CONFIG" --wait 5m
else
  log "Using existing kind cluster: $CLUSTER_NAME"
fi

kubectl config use-context "kind-$CLUSTER_NAME" >/dev/null
kubectl wait --for=condition=Ready nodes --all --timeout=5m

log "Checking Pulumi project"
if [[ ! -f Pulumi.yaml ]]; then
  pulumi new kubernetes-go --name "$PULUMI_PROJECT" --stack "$PULUMI_STACK" --yes
fi

# Pulumi provider Context must be the actual kubeconfig context. The Pulumi
# program defaults to kind-pulumi-gitops, so explicitly set it here.
pulumi config set clusterName "kind-$CLUSTER_NAME"
pulumi config set githubRepo "${GITHUB_APPS_REPO:-https://github.com/raihankhan/pulumi-kind-go-gitops-apps.git}"
pulumi config set githubBranch "${GITHUB_APPS_BRANCH:-main}"
pulumi config set ingressClass "${INGRESS_CLASS:-nginx}"
pulumi config set argocdDomain "${ARGOCD_DOMAIN:-argocd.local}"

if [[ -n "${PULUMI_ACCESS_TOKEN:-}" ]]; then
  log "Configuring Pulumi Cloud access token from environment"
  pulumi config set --secret pulumi:accessToken "$PULUMI_ACCESS_TOKEN"
fi

log "Deploying Pulumi Operator, ingress-nginx, and Argo CD"
pulumi up --yes --stack "$PULUMI_STACK"

log "Applying local Argo CD AppProject and root Application"
# The Pulumi program applies these resources. Applying again is safe and makes
# recovery after a partial Pulumi run straightforward.
kubectl apply -f manifests/argocd/10-appproject.yaml
sed "s#https://github.com/raihankhan/pulumi-kind-go-gitops-apps.git#${GITHUB_APPS_REPO:-https://github.com/raihankhan/pulumi-kind-go-gitops-apps.git}#g" \
  manifests/argocd/20-root-application.yaml | kubectl apply -f -

log "Waiting for Argo CD server"
kubectl -n argocd rollout status deployment/argocd-server --timeout=5m

cat <<EOF

Bootstrap complete.

Cluster context: kind-$CLUSTER_NAME
Argo CD UI:      http://localhost:30080 (insecure local mode)
Argo CD admin password:
  kubectl -n argocd get secret argocd-initial-admin-secret \\
    -o jsonpath='{.data.password}' | base64 -d; printf '\\n'
Nginx:           http://nginx.local (add to /etc/hosts if needed)
Nginx direct:    http://localhost:30080 (if ingress is not used)

Next steps:
  1. Push manifests/nginx to the GitHub apps repo at apps/nginx.
  2. Configure GITHUB_APPS_REPO and rerun this script.
  3. Check: argocd app list --core OR kubectl -n argocd get applications
EOF
