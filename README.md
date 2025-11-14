# Cluster API IPAM Provider for Unifi

A Kubernetes controller that provides IP Address Management (IPAM) for [Cluster API](https://cluster-api.sigs.k8s.io/) using Ubiquiti Unifi network controllers.

## Overview

This provider integrates Unifi network controllers with Cluster API to enable dynamic IP address allocation for cluster infrastructure. It implements the [Cluster API IPAM provider contract](https://cluster-api.sigs.k8s.io/tasks/experimental-features/ipam.html) and manages IP addresses through the Unifi API.

## Features

- **Dynamic IP Allocation**: Automatically allocate IP addresses from Unifi-managed networks
- **Multiple Network Support**: Configure multiple Unifi instances and network pools
- **Cluster API Integration**: Seamless integration with Cluster API IPAddressClaim resources
- **Automatic Cleanup**: Release IP addresses when resources are deleted
- **Subnet Management**: Support for multiple subnets and CIDR ranges

## Prerequisites

- Kubernetes cluster (v1.28+)
- Cluster API installed (v1.6+)
- Unifi Network Controller with API access
- Administrative credentials for Unifi API

## Installation

### Using clusterctl

The recommended way to install the provider is using `clusterctl`:

1. Create a `~/.cluster-api/clusterctl.yaml` file with the provider configuration:

```yaml
providers:
  - name: "unifi"
    url: "https://github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/releases/latest/metadata.yaml"
    type: "IPAMProvider"
```

2. Initialize the provider:

```bash
clusterctl init --ipam unifi
```

Alternatively, you can specify the provider version explicitly:

```yaml
apiVersion: clusterctl.cluster.x-k8s.io/v1alpha3
kind: Provider
metadata:
  name: cluster-api-ipam-provider-unifi
  namespace: ipam-system
spec:
  version: v0.1.0  # Replace with desired version
  type: IPAMProvider
  url: https://github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/releases/download/v0.1.0/install.yaml
```

### Using kubectl

```bash
kubectl apply -f https://github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/releases/latest/download/install.yaml
```

### Using kustomize

```bash
kustomize build config/default | kubectl apply -f -
```

## Configuration

### 1. Create a Unifi Instance

Define your Unifi controller connection:

```yaml
apiVersion: ipam.cluster.x-k8s.io/v1alpha1
kind: UnifiInstance
metadata:
  name: unifi-controller
  namespace: default
spec:
  host: "https://unifi.example.com:8443"
  # Reference to a secret containing credentials
  credentialsRef:
    name: unifi-credentials
  # Optional: Skip TLS verification
  insecure: false
  # Optional: Site name (default: "default")
  site: default
```

Create the credentials secret:

```bash
kubectl create secret generic unifi-credentials \
  --from-literal=username=admin \
  --from-literal=password=your-password
```

### 2. Create an IP Pool

Define an IP pool for allocation:

```yaml
apiVersion: ipam.cluster.x-k8s.io/v1alpha1
kind: UnifiIPPool
metadata:
  name: cluster-pool
  namespace: default
spec:
  # Reference to the Unifi instance
  instanceRef:
    name: unifi-controller
  # Network ID from Unifi
  networkId: "5f9a8b7c6d5e4f3a2b1c0d9e"
  # Subnets to allocate from
  subnets:
    - cidr: "192.168.1.0/24"
      gateway: "192.168.1.1"
      prefix: 24
      # Optional: Exclude specific IPs
      excludeRanges:
        - "192.168.1.1-192.168.1.10"
```

### 3. Request an IP Address

Cluster API will automatically create IPAddressClaim resources, but you can also create them manually:

```yaml
apiVersion: ipam.cluster.x-k8s.io/v1beta1
kind: IPAddressClaim
metadata:
  name: my-machine-ip
  namespace: default
spec:
  poolRef:
    apiGroup: ipam.cluster.x-k8s.io
    kind: UnifiIPPool
    name: cluster-pool
```

## Architecture

```
┌─────────────────┐
│  Cluster API    │
│   Controllers   │
└────────┬────────┘
         │ Creates
         ▼
┌─────────────────┐       ┌──────────────────┐
│ IPAddressClaim  │◄──────┤  Unifi IPAM      │
│   Resources     │       │  Provider        │
└────────┬────────┘       └────────┬─────────┘
         │                         │
         │ References              │ Manages
         ▼                         ▼
┌─────────────────┐       ┌──────────────────┐
│  UnifiIPPool    │       │ Unifi Controller │
│    (CRD)        │       │   (External)     │
└────────┬────────┘       └──────────────────┘
         │
         │ References
         ▼
┌─────────────────┐
│ UnifiInstance   │
│    (CRD)        │
└─────────────────┘
```

## Development

### Prerequisites

- Go 1.21+
- Docker or Podman
- kubectl
- kustomize

### Building

```bash
# Build the binary
make build

# Run tests
make test

# Generate CRDs and code
make manifests generate

# Build Docker image
make docker-build IMG=myregistry/cluster-api-ipam-provider-unifi:dev
```

### Running Locally

```bash
# Install CRDs
make install

# Run the controller locally
make run
```

### Testing

```bash
# Run unit tests
make test

# Run e2e tests (requires a running cluster)
make test-e2e
```

## Examples

See the [config/samples](config/samples) directory for complete examples.

## Troubleshooting

### Common Issues

**Issue**: Controller cannot connect to Unifi
- Verify the host URL is correct and accessible
- Check credentials in the secret
- Ensure firewall allows access to Unifi API port (default 8443)

**Issue**: IP addresses not allocated
- Verify the UnifiIPPool references a valid UnifiInstance
- Check that the network ID exists in Unifi
- Ensure subnet CIDR matches Unifi network configuration
- Review controller logs: `kubectl logs -n ipam-system deployment/unifi-ipam-controller`

**Issue**: Stale IP allocations
- The controller uses finalizers to clean up IPs
- If manual cleanup is needed, remove the finalizer after releasing the IP in Unifi

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

## References

- [Cluster API Documentation](https://cluster-api.sigs.k8s.io/)
- [Cluster API IPAM Provider Specification](https://cluster-api.sigs.k8s.io/tasks/experimental-features/ipam.html)
- [Unifi API Documentation](https://ubntwiki.com/products/software/unifi-controller/api)
- [go-unifi Client Library](https://github.com/ubiquiti-community/go-unifi)
