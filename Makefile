.PHONY: help build test tidy up destroy kind-create kind-delete kind-status argocd-pw port-forward ingress-hosts verify

ROOT_DIR := $(shell pwd)
CLUSTER_NAME ?= pulumi-gitops
PULUMI_STACK ?= dev

help:
	@echo "Targets:"
	@echo "  make build         Build the Pulumi Go program"
	@echo "  make test          Run Go tests"
	@echo "  make tidy          Run go mod tidy"
	@echo "  make up            Bootstrap the local platform end-to-end"
	@echo "  make destroy       Tear down Pulumi-deployed resources"
	@echo "  make kind-create   Create the kind cluster"
	@echo "  make kind-delete   Delete the kind cluster"
	@echo "  make kind-status   Show cluster status"
	@echo "  make argocd-pw     Print Argo CD initial admin password"
	@echo "  make port-forward  Port-forward the Argo CD server"
	@echo "  make ingress-hosts Print /etc/hosts lines to add"
	@echo "  make verify        Run build, test, and vet"

build:
	go build -o ./bin/platform ./cmd/pulumi

test:
	go test ./...

tidy:
	go mod tidy

up:
	./scripts/bootstrap.sh

destroy:
	pulumi destroy --yes --stack $(PULUMI_STACK)

kind-create:
	./cmd/kindctl create

kind-delete:
	./cmd/kindctl delete

kind-status:
	./cmd/kindctl status 2>/dev/null || kubectl get nodes --context kind-$(CLUSTER_NAME)

argocd-pw:
	@kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d; printf "\n"

port-forward:
	kubectl port-forward svc/argocd-server -n argocd 8080:443

ingress-hosts:
	@printf "127.0.0.1 nginx.local argocd.local\n"

verify: build test
	go vet ./...
