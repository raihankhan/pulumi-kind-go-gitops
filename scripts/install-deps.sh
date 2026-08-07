#!/usr/bin/env bash
set -Eeuo pipefail

# Install local dependencies on macOS (brew) or Linux (apt). Designed for
# local development only — production install is the operator's responsibility.

if [[ "$OSTYPE" == "darwin"* ]]; then
  if ! command -v brew >/dev/null 2>&1; then
    echo "Homebrew is required on macOS: https://brew.sh"
    exit 1
  fi
  brew update
  brew install kind kubectl helm pulumi go
elif command -v apt-get >/dev/null 2>&1; then
  curl -fsSL https://get.pulumi.com | sh
  curl -fsSL https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64 -o /usr/local/bin/kind
  chmod +x /usr/local/bin/kind
  curl -fsSL https://dl.k8s.io/release/v1.30.0/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl
  chmod +x /usr/local/bin/kubectl
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
else
  echo "Unsupported OS: $OSTYPE"
  exit 1
fi

echo "Dependencies installed."
