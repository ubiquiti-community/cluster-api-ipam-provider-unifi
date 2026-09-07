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

## Migrating from UnifiIPPool/UnifiInstance

This provider's own kinds were renamed and moved to their own API group, to
match how [metal3-io/ip-address-manager](https://github.com/metal3-io/ip-address-manager)
is laid out:

| Before | After |
| --- | --- |
| `ipam.cluster.x-k8s.io/v1beta2`, kind `UnifiIPPool` | `unifi.ipam.cluster.x-k8s.io/v1beta2`, kind `IPPool` |
| `ipam.cluster.x-k8s.io/v1beta2`, kind `UnifiInstance` | `unifi.ipam.cluster.x-k8s.io/v1beta2`, kind `Instance` |

`ipam.cluster.x-k8s.io` belongs to Cluster API, and `IPAddress` and
`IPAddressClaim` still live there — only this provider's pool and controller
connection moved out. A claim reaches across the boundary through
`poolRef.apiGroup`, exactly as metal3's pool does. The version (`v1beta2`) is
unchanged.

**This is a breaking change with no conversion path.** There is no conversion
webhook and no storage migration: the old CRDs and any custom resources under
them are not converted, and existing `IPAddress` objects still carry
`poolRef.apiGroup: ipam.cluster.x-k8s.io` with `poolRef.kind: UnifiIPPool`,
neither of which the moved controllers match any more.

To migrate:

1. Record the current allocations, e.g.
   `kubectl get unifiippool <name> -o jsonpath='{.status.allocations}'`, so they
   can be carried over as `spec.preAllocations` on the new pool.
2. Recreate each `UnifiInstance` as an `Instance` and each `UnifiIPPool` as an
   `IPPool` under `apiVersion: unifi.ipam.cluster.x-k8s.io/v1beta2` (the spec
   fields are unchanged).
3. Re-point every `IPAddressClaim` at the new pool with
   `poolRef.apiGroup: unifi.ipam.cluster.x-k8s.io` and `poolRef.kind: IPPool`;
   existing `IPAddress` objects that reference the old group or kind must be
   deleted and re-claimed.
4. Remove the old `unifiippools.ipam.cluster.x-k8s.io` and
   `unifiinstances.ipam.cluster.x-k8s.io` CRDs once nothing references them.

See [config/samples/example.yaml](config/samples/example.yaml) for a complete
worked scenario.

The new CRDs keep `unifiippool`/`unifiinstance` (and their plurals) as short
names, but they only take effect once step 4 is done: while both the old and the
new CRDs are installed, `kubectl get unifiippool` still resolves to the old
CRD, whose singular name wins over another CRD's short name. Until then, address
the new kinds explicitly — `kubectl get ippool`, or
`kubectl get ippools.unifi.ipam.cluster.x-k8s.io`.

## Configuration

### 1. Create a Unifi Instance

Define your Unifi controller connection:

```yaml
apiVersion: unifi.ipam.cluster.x-k8s.io/v1beta2
kind: Instance
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
apiVersion: unifi.ipam.cluster.x-k8s.io/v1beta2
kind: IPPool
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
apiVersion: ipam.cluster.x-k8s.io/v1beta2
kind: IPAddressClaim
metadata:
  name: my-machine-ip
  namespace: default
  annotations:
    # Optional: the MAC of the machine this address is for.
    unifi.ipam.cluster.x-k8s.io/mac-address: "f4:4d:30:6f:a7:93"
spec:
  poolRef:
    apiGroup: unifi.ipam.cluster.x-k8s.io
    kind: IPPool
    name: cluster-pool
```

Each allocation is backed by a fixed-IP reservation in the Unifi controller.
Annotate the claim with the machine's MAC (`unifi.ipam.cluster.x-k8s.io/mac-address`;
the `capt.tinkerbell.org/mac-address` annotation that
cluster-api-provider-tinkerbell sets is also recognized) and the reservation is
made on that device's own client record, so Unifi DHCP hands the machine the same
address. A device Unifi already knows keeps its reservation if it lies in the
pool; otherwise a pool address is written onto its existing record. Without an
annotation the reservation is made on a deterministic, locally administered MAC
derived from the claim name.

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
│     IPPool      │       │ Unifi Controller │
│    (CRD)        │       │   (External)     │
└────────┬────────┘       └──────────────────┘
         │
         │ References
         ▼
┌─────────────────┐
│    Instance     │
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
- Verify the IPPool references a valid Instance
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
