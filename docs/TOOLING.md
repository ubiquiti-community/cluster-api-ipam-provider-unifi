# Development Tooling and CI/CD Setup

## Summary of Changes

This document summarizes the comprehensive development tooling and CI/CD automation implemented for the cluster-api-ipam-provider-unifi project.

## 1. Updated Makefile

### Key Changes

#### Controller-gen Integration
- All code generation handled via `go generate ./...`
- Uses `go:generate` directives in `generate.go`
- Maintains consistency with Go toolchain patterns

#### Formatting with golangci-lint
- Replaced `go fmt` with `golangci-lint run --fix --fast`
- Added dedicated `lint` and `lint-fix` targets
- Uses official golangci-lint GitHub Action in CI

#### New Targets Added
- `lint` - Run golangci-lint against code
- `lint-fix` - Run golangci-lint with auto-fix
- `release` - Build release artifacts with goreleaser
- `release-snapshot` - Build snapshot release artifacts

#### Tool Management
All tools are now managed with `go tool` and don't require manual installation:
- `go tool controller-gen` - Code generation
- `go tool golangci-lint` - Linting and formatting
- `go tool goreleaser` - Release management
- `go tool setup-envtest` - Test environment setup

## 2. golangci-lint Configuration

**File**: `.golangci.yaml`

### Enabled Linters (40+)
- Code quality: gocritic, gocyclo, gocognit, goconst
- Error handling: errcheck, errchkjson, nilerr
- Code style: gofmt, gofumpt, goimports, gci
- Security: gosec
- Documentation: godot, goheader
- And many more...

### Key Configurations

#### Import Organization (gci)
```yaml
sections:
  - standard
  - default
  - prefix(github.com/ubiquiti-community/cluster-api-ipam-provider-unifi)
  - prefix(sigs.k8s.io/cluster-api)
```

#### Import Aliases (importas)
Enforces consistent import aliasing:
- `corev1` for k8s.io/api/core/v1
- `metav1` for k8s.io/apimachinery/pkg/apis/meta/v1
- `clusterv1` for cluster-api/api/v1beta2
- `ipamv1` for cluster-api/exp/ipam/api/v1beta2
- `ipamv1alpha1` for project's api/v1alpha1

#### License Header (goheader)
Enforces Apache 2.0 license header on all Go files.

#### Exclusions
- Disables certain linters for test files
- Excludes generated files (zz_generated.deepcopy.go)
- Special rules for webhook files

## 3. goreleaser Configuration

**File**: `.goreleaser.yaml`

### Build Configuration
- **Platforms**: Linux and Darwin
- **Architectures**: amd64 and arm64
- **Build flags**: `-trimpath` for reproducible builds
- **LDFlags**: Injects version, commit, and date

### Docker Images
- Multi-architecture support (amd64, arm64)
- Tags: `{version}`, `latest`, `{version}-{arch}`
- Registry: GitHub Container Registry (ghcr.io)
- Uses buildx for cross-platform builds

### Release Artifacts
Archives include:
- Manager binary
- LICENSE
- README.md

### Manifests
Before release hook generates:
- `install.yaml` - Complete installation manifest
- `metadata.yaml` - Cluster API provider metadata
- `clusterctl.yaml` - clusterctl configuration

## 4. GitHub Actions Workflows

### CI Workflow (`.github/workflows/ci.yaml`)

Runs on: Push to main/release branches, Pull requests

**Jobs**:
1. **Lint** - Runs golangci-lint
2. **Test** - Runs unit tests
3. **Build** - Builds manager binary
4. **Verify** - Ensures manifests and go.mod are up to date

### Docker Workflow (`.github/workflows/docker.yaml`)

Runs on: Push to main, Pull requests

**Features**:
- Multi-architecture builds (amd64, arm64)
- Uses GitHub Container Registry
- Smart tagging:
  - `latest` for main branch
  - `main-{sha}` for commits
  - `pr-{number}` for pull requests
- Build caching with GitHub Actions cache

### Release Workflow (`.github/workflows/release.yaml`)

Runs on: Version tags (v*.*.*)

**Steps**:
1. Checkout with full history
2. Setup Go and Docker buildx
3. Login to GHCR
4. Generate manifests
5. Run goreleaser
6. Upload artifacts

**Artifacts**:
- Binary archives (tar.gz)
- Docker images (multi-arch)
- Installation manifests
- Metadata files
- Checksums

### Pre-commit Workflow (`.github/workflows/pre-commit.yaml`)

Runs on: Pull requests

**Purpose**: Ensures code is properly formatted
- Runs `make lint-fix`
- Fails if formatting changes are detected
- Guides contributors to run linter locally

## 5. Supporting Scripts

### Release Manifest Generator

**File**: `hack/generate-release-manifests.sh`

**Purpose**: Generates Cluster API provider manifests for releases

**Generates**:
1. `install.yaml` - Complete kustomize build
2. `metadata.yaml` - Provider metadata with contract version
3. `clusterctl.yaml` - Provider configuration for clusterctl

**Usage**:
```bash
./hack/generate-release-manifests.sh v0.1.0
```

## 6. Tool Dependencies

**File**: `tools.go`

Purpose: Declares tool dependencies for `go mod` tracking

**Tools**:
- golangci-lint/cmd/golangci-lint
- goreleaser/goreleaser/v2
- controller-tools/cmd/controller-gen
- controller-runtime/tools/setup-envtest
- kustomize/kustomize/v5

## 7. Documentation

### DEVELOPMENT.md

Comprehensive development guide covering:
- Prerequisites and setup
- Development workflow
- Code quality tools
- Building and testing
- Docker images
- Deployment
- Release process
- CI/CD workflows
- Project structure
- Debugging tips
- Contributing guidelines

## Usage Examples

### Daily Development

```bash
# Format and fix code
make lint-fix

# Run tests
make test

# Build
make build

# Run locally
make run
```

### Pre-commit Checklist

```bash
make lint-fix    # Format code
make test        # Run tests
make manifests   # Update manifests
make generate    # Update generated code
git add .
git commit -s -m "feat: add new feature"
```

### Creating a Release

```bash
# Tag the release
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0

# GitHub Actions automatically:
# - Runs CI checks
# - Builds multi-arch images
# - Generates manifests
# - Creates GitHub release
# - Publishes artifacts
```

### Testing Release Locally

```bash
# Build snapshot (doesn't publish)
make release-snapshot

# Check artifacts in dist/
ls -la dist/
```

## Benefits

### For Developers
- **Consistent formatting** - golangci-lint ensures uniform code style
- **Automated quality checks** - Linting catches issues early
- **Easy setup** - All tools auto-install to `./bin/`
- **Fast feedback** - CI runs on every PR

### For Maintainers
- **Automated releases** - Tag-based release workflow
- **Multi-arch support** - Native ARM64 and AMD64 images
- **Cluster API integration** - Proper metadata for clusterctl
- **Quality gates** - PRs must pass linting and tests

### For Users
- **Reliable releases** - Automated testing before release
- **Multiple platforms** - Pre-built binaries for all platforms
- **Easy installation** - Single manifest file (`install.yaml`)
- **Docker images** - Multi-arch images on GHCR

## CI/CD Pipeline Flow

```
┌─────────────────────────────────────────────────────────┐
│ Developer Workflow                                       │
├─────────────────────────────────────────────────────────┤
│ 1. make lint-fix (local formatting)                     │
│ 2. make test (local testing)                            │
│ 3. git commit & push                                     │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ Pull Request CI                                          │
├─────────────────────────────────────────────────────────┤
│ • Pre-commit: Format check                              │
│ • CI: Lint, Test, Build, Verify                        │
│ • Docker: Build multi-arch images                       │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ Merge to Main                                            │
├─────────────────────────────────────────────────────────┤
│ • CI runs again                                          │
│ • Docker images pushed with 'latest' tag                │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ Tag Release (v*.*.*)                                     │
├─────────────────────────────────────────────────────────┤
│ • Release workflow triggers                              │
│ • goreleaser builds binaries (4 arch combinations)      │
│ • Docker images built for amd64 & arm64                 │
│ • Manifests generated                                    │
│ • GitHub release created with all artifacts             │
└─────────────────────────────────────────────────────────┘
```

## Configuration Files Summary

| File | Purpose |
|------|---------|
| `.golangci.yaml` | golangci-lint configuration with 40+ linters |
| `.goreleaser.yaml` | Release automation with multi-arch builds |
| `.github/workflows/ci.yaml` | Continuous integration checks |
| `.github/workflows/docker.yaml` | Docker image builds |
| `.github/workflows/release.yaml` | Automated releases on tags |
| `.github/workflows/pre-commit.yaml` | Format checking on PRs |
| `hack/generate-release-manifests.sh` | Generate Cluster API manifests |
| `tools.go` | Tool dependency tracking |
| `DEVELOPMENT.md` | Comprehensive development guide |

## Migration Notes

### For Existing Developers

**Old workflow**:
```bash
go fmt ./...
go vet ./...
go build -o bin/manager cmd/manager/main.go
```

**New workflow**:
```bash
make lint-fix  # Replaces go fmt, includes many more checks
make build     # Still builds, but with quality checks
```

### Breaking Changes
None - all existing `make` targets still work, just enhanced with better tooling.

## Future Enhancements

Possible additions:
- [ ] E2E test workflow
- [ ] Code coverage reporting
- [ ] Security scanning (Trivy, Snyk)
- [ ] Dependency update automation (Dependabot)
- [ ] Release notes automation
- [ ] Benchmark tracking
- [ ] Performance regression testing
