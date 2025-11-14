/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package poolutil

import (
	"context"
	"fmt"
	"net/netip"

	"go4.org/netipx"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1alpha1"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
)

// ListAddressesInUse returns all IPAddresses that reference the given pool.
func ListAddressesInUse(ctx context.Context, c client.Client, namespace string, poolName string, poolKind string, poolAPIGroup string) ([]ipamv1.IPAddress, error) {
	addressList := &ipamv1.IPAddressList{}

	// List addresses in the same namespace as the pool (or cluster-wide if no namespace)
	if namespace != "" {
		if err := c.List(ctx, addressList, client.InNamespace(namespace)); err != nil {
			return nil, fmt.Errorf("failed to list addresses: %w", err)
		}
	} else {
		if err := c.List(ctx, addressList); err != nil {
			return nil, fmt.Errorf("failed to list addresses: %w", err)
		}
	}

	// Filter addresses that reference this pool
	inUse := make([]ipamv1.IPAddress, 0)
	for _, address := range addressList.Items {
		poolRefAPIGroup := ""
		if address.Spec.PoolRef.APIGroup != nil {
			poolRefAPIGroup = *address.Spec.PoolRef.APIGroup
		}
		if address.Spec.PoolRef.Name == poolName &&
			address.Spec.PoolRef.Kind == poolKind &&
			poolRefAPIGroup == poolAPIGroup {
			inUse = append(inUse, address)
		}
	}

	return inUse, nil
}

// AddressesToIPSet converts a slice of IP address strings to an IPSet.
func AddressesToIPSet(addresses []string) (*netipx.IPSet, error) {
	var builder netipx.IPSetBuilder

	for _, addr := range addresses {
		if addr == "" {
			continue
		}

		// Try parsing as a single IP
		if ip, err := netip.ParseAddr(addr); err == nil {
			builder.Add(ip)
			continue
		}

		// Try parsing as CIDR
		if prefix, err := netip.ParsePrefix(addr); err == nil {
			builder.AddPrefix(prefix)
			continue
		}

		return nil, fmt.Errorf("invalid address format: %s", addr)
	}

	ipSet, err := builder.IPSet()
	return ipSet, err
}

// PoolSpecToIPSet converts a SubnetSpec to an IPSet.
func PoolSpecToIPSet(poolSpec *ipamv1alpha1.SubnetSpec) (*netipx.IPSet, error) {
	if poolSpec == nil {
		return nil, fmt.Errorf("pool spec is nil")
	}

	var builder netipx.IPSetBuilder

	// Parse the CIDR
	prefix, err := netip.ParsePrefix(poolSpec.CIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CIDR %s: %w", poolSpec.CIDR, err)
	}

	builder.AddPrefix(prefix)

	// Remove gateway from the pool
	if poolSpec.Gateway != "" {
		gateway, err := netip.ParseAddr(poolSpec.Gateway)
		if err != nil {
			return nil, fmt.Errorf("failed to parse gateway %s: %w", poolSpec.Gateway, err)
		}
		builder.Remove(gateway)
	}

	// Remove excluded ranges
	for _, excludeRange := range poolSpec.ExcludeRanges {
		// Try parsing as CIDR first
		if prefix, err := netip.ParsePrefix(excludeRange); err == nil {
			builder.RemovePrefix(prefix)
			continue
		}

		// Try parsing as single IP
		if ip, err := netip.ParseAddr(excludeRange); err == nil {
			builder.Remove(ip)
			continue
		}

		// Try parsing as IP range (start-end format)
		// For simplicity, we'll skip range parsing for now
		// A more robust implementation would parse "10.0.0.1-10.0.0.10" format
	}

	// Remove network and broadcast addresses
	r := netipx.RangeOfPrefix(prefix)
	builder.Remove(r.From())
	builder.Remove(r.To())

	ipSet, err := builder.IPSet()
	return ipSet, err
}

// FindNextAvailableIP finds the next available IP address in the pool.
func FindNextAvailableIP(poolIPSet *netipx.IPSet, inUseIPSet *netipx.IPSet) (string, error) {
	if poolIPSet == nil || inUseIPSet == nil {
		return "", fmt.Errorf("IPSet is nil")
	}

	// Build available IPs by removing in-use IPs from the pool
	var builder netipx.IPSetBuilder
	for _, r := range poolIPSet.Ranges() {
		builder.AddRange(r)
	}
	for _, r := range inUseIPSet.Ranges() {
		builder.RemoveRange(r)
	}

	availableIPSet, err := builder.IPSet()
	if err != nil {
		return "", fmt.Errorf("failed to build available IP set: %w", err)
	}

	// Find the first available IP
	for _, r := range availableIPSet.Ranges() {
		from := r.From()
		if from.IsValid() {
			return from.String(), nil
		}
	}

	return "", fmt.Errorf("no available IP addresses in pool")
}

// ComputePoolStatus computes the status summary for a pool.
func ComputePoolStatus(poolIPSet *netipx.IPSet, addressesInUse []ipamv1.IPAddress, poolNamespace string) *ipamv1alpha1.IPAddressStatusSummary {
	if poolIPSet == nil {
		return &ipamv1alpha1.IPAddressStatusSummary{}
	}

	// Count total addresses in pool
	totalCount := 0
	for _, r := range poolIPSet.Ranges() {
		from := r.From()
		to := r.To()
		if from.Is4() {
			totalCount += int(to.As4()[3] - from.As4()[3] + 1)
		} else {
			// IPv6 - approximate
			totalCount += 1000
		}
	}

	// Count in-use and out-of-range addresses
	usedCount := 0
	outOfRangeCount := 0
	for _, addr := range addressesInUse {
		// Only count addresses in the same namespace (or cluster-wide)
		if poolNamespace != "" && addr.Namespace != poolNamespace {
			continue
		}

		if addr.Spec.Address == "" {
			continue
		}

		ip, err := netip.ParseAddr(addr.Spec.Address)
		if err != nil {
			continue
		}

		if poolIPSet.Contains(ip) {
			usedCount++
		} else {
			outOfRangeCount++
		}
	}

	freeCount := totalCount - usedCount
	if freeCount < 0 {
		freeCount = 0
	}

	return &ipamv1alpha1.IPAddressStatusSummary{
		Total:      totalCount,
		Used:       usedCount,
		Free:       freeCount,
		OutOfRange: outOfRangeCount,
	}
}
