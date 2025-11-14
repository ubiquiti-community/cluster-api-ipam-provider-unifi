# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/ubiquiti-community/cluster-api-ipam-provider-unifi:latest

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.34.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	go generate ./...

.PHONY: generate
generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	go generate ./...

GOLANGCI_LINT ?= $(shell which golangci-lint 2>/dev/null || echo "go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest")

.PHONY: fmt
fmt: ## Run golangci-lint fmt against code.
	$(GOLANGCI_LINT) fmt ./...
	$(GOLANGCI_LINT) run --fix

.PHONY: lint
lint: ## Run golangci-lint against code.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix against code.
	$(GOLANGCI_LINT) run --fix

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: generate vet ## Run tests.
	KUBEBUILDER_ASSETS="$(shell go tool setup-envtest use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

GORELEASER ?= $(shell which goreleaser 2>/dev/null || echo "go run github.com/goreleaser/goreleaser/v2@latest")

##@ Build

.PHONY: build
build: generate vet ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: generate vet ## Run a controller from your host.
	go run ./cmd/manager/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: release
release: ## Build release artifacts with goreleaser.
	$(GORELEASER) release --clean --skip=sign

.PHONY: release-snapshot
release-snapshot: ## Build snapshot release artifacts with goreleaser.
	$(GORELEASER) release --snapshot --clean --skip=sign

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: generate ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	go tool kustomize build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	go tool kustomize build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: generate ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && kubectl kustomize edit set image controller=${IMG}
	go tool kustomize build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	go tool kustomize build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

# All tools are now managed via 'go run' and don't require local installation
