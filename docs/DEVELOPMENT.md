# Development Guide

## Prerequisites

- Go 1.21 or later
- Docker (for building container images)
- kubectl
- Access to a Kubernetes cluster (for testing)
- make

## Getting Started

### Clone the repository

```bash
git clone https://github.com/ubiquiti-community/cluster-api-ipam-provider-unifi.git
cd cluster-api-ipam-provider-unifi
```

### Install dependencies

All development tools are managed with `go tool` and don't require manual installation:

- controller-gen (via `go generate` / `go tool controller-gen`)
- kustomize (via `kubectl kustomize`)
- setup-envtest (via `go tool setup-envtest`)
- golangci-lint (via official GitHub Action or `go tool golangci-lint`)
- goreleaser (via official GitHub Action or `go tool goreleaser`)

## Development Workflow

### Building

```bash
# Build the manager binary
make build

# Run the manager locally (requires kubeconfig access)
make run
```

### Code Quality

#### Formatting and Linting

We use `golangci-lint` for code formatting and linting:

```bash
# Run linter
make lint

# Run linter with auto-fix
make lint-fix

# Or run directly with go tool
go tool golangci-lint run --fix
```

#### Testing

```bash
# Run unit tests
make test

# Run tests with coverage
make test
open cover.out  # View coverage in browser
```

### Code Generation

When modifying APIs or adding/updating kubebuilder markers:

```bash
# Generate everything (CRDs, RBAC, webhooks, deepcopy methods)
go generate ./...

# Or use make targets
make generate
make manifests
```

All generation is handled by `go:generate` directives in `generate.go`.

### Working with CRDs

```bash
# Install CRDs to cluster
make install

# Uninstall CRDs from cluster
make uninstall
```

## Docker Images

### Building Images

```bash
# Build Docker image
make docker-build IMG=ghcr.io/ubiquiti-community/cluster-api-ipam-provider-unifi:dev

# Push Docker image
make docker-push IMG=ghcr.io/ubiquiti-community/cluster-api-ipam-provider-unifi:dev
```

### Multi-architecture Builds

The project uses goreleaser (via GitHub Actions) for multi-architecture builds:

```bash
# Build snapshot release locally (no push)
make release-snapshot

# Build and push release (requires tag, done by GitHub Actions)
make release

# Or run goreleaser directly
go tool goreleaser release --snapshot --clean
```

## Deployment

### Deploy to Cluster

```bash
# Deploy controller to cluster
make deploy IMG=ghcr.io/ubiquiti-community/cluster-api-ipam-provider-unifi:latest

# Undeploy from cluster
make undeploy
```

### Testing Locally

```bash
# 1. Install CRDs
make install

# 2. Run controller locally
make run

# 3. In another terminal, create test resources
kubectl apply -f config/samples/
```

## Release Process

### Creating a Release

Releases are automated via GitHub Actions. To create a new release:

1. **Ensure all changes are merged to main**

2. **Create and push a version tag:**
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

3. **GitHub Actions will automatically:**
   - Run tests and linting
   - Build multi-arch Docker images
   - Generate release manifests
   - Create GitHub release with artifacts
   - Push Docker images to GHCR

### Release Artifacts

Each release includes:
- **Binary archives** (Linux/Darwin, amd64/arm64)
- **Docker images** (multi-arch manifest)
- **install.yaml** - Complete installation manifest
- **metadata.yaml** - Cluster API metadata
- **clusterctl.yaml** - clusterctl provider configuration

## CI/CD Workflows

### Continuous Integration (`.github/workflows/ci.yaml`)

Runs on every push and PR:
- Linting with golangci-lint
- Unit tests
- Build verification
- Manifest generation verification
- Go module verification

### Docker Build (`.github/workflows/docker.yaml`)

Builds and pushes Docker images:
- On main branch: tags as `latest` and `main-<sha>`
- On PRs: builds but doesn't push
- Multi-architecture (amd64, arm64)

### Release (`.github/workflows/release.yaml`)

Runs when a version tag is pushed:
- Builds release artifacts with goreleaser
- Generates installation manifests
- Creates GitHub release
- Pushes Docker images with version tags

### Pre-commit (`.github/workflows/pre-commit.yaml`)

Runs on PRs to ensure code formatting:
- Runs `make lint-fix`
- Fails if there are uncommitted formatting changes

## Project Structure

```
.
├── api/                    # API definitions (CRDs)
│   └── v1alpha1/          # v1alpha1 API version
├── cmd/
│   └── manager/           # Controller manager entrypoint
├── config/                # Kubernetes manifests
│   ├── crd/              # CRD configurations
│   ├── default/          # Default deployment configs
│   ├── manager/          # Manager deployment
│   ├── rbac/             # RBAC configurations
│   ├── samples/          # Sample resources
│   └── webhook/          # Webhook configurations
├── hack/                  # Scripts and tools
├── internal/
│   ├── controllers/      # Controller implementations
│   ├── index/            # Indexers
│   ├── poolutil/         # Pool utilities
│   ├── unifi/            # Unifi API client
│   └── webhooks/         # Webhook implementations
├── pkg/
│   └── ipamutil/         # IPAM utilities
└── test/
    └── e2e/              # End-to-end tests
```

## Debugging

### Controller Logs

When running locally:
```bash
make run
```

When deployed to cluster:
```bash
kubectl logs -n ipam-system deployment/unifi-ipam-controller-manager -f
```

### Webhook Debugging

View webhook logs:
```bash
kubectl logs -n ipam-system deployment/unifi-ipam-controller-manager -c manager -f | grep webhook
```

View webhook configurations:
```bash
kubectl get mutatingwebhookconfigurations
kubectl get validatingwebhookconfigurations
```

### Common Issues

**Issue**: Webhook certificate errors
```bash
# Check certificate secret
kubectl get secret -n ipam-system webhook-server-cert

# Recreate if needed
kubectl delete secret -n ipam-system webhook-server-cert
# cert-manager will recreate it
```

**Issue**: CRD validation errors
```bash
# Reinstall CRDs
make uninstall
make install
```

**Issue**: Controller not reconciling
```bash
# Check controller logs
kubectl logs -n ipam-system deployment/unifi-ipam-controller-manager -f

# Check resource status
kubectl get unifiinstance -A -o yaml
kubectl get unifiippool -A -o yaml
```

## Contributing

### Code Style

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `make lint-fix` before committing
- Write tests for new functionality
- Update documentation as needed

### Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make lint-fix test`
5. Ensure all CI checks pass
6. Submit a pull request

### Commit Message Guidelines

Follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat: add new feature`
- `fix: resolve issue`
- `docs: update documentation`
- `test: add tests`
- `chore: update dependencies`

## Additional Resources

- [Cluster API Documentation](https://cluster-api.sigs.k8s.io/)
- [Kubebuilder Documentation](https://book.kubebuilder.io/)
- [Controller Runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [Unifi Controller API](https://ubntwiki.com/products/software/unifi-controller/api)
