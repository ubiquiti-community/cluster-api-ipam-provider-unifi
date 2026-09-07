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
	"encoding/binary"
	"fmt"
	"net/netip"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
)

// GetIPAddress returns the IP address at the given index within a subnet.
// Supports both CIDR notation and Start/End ranges.
// Returns an error if the index is out of range for the subnet.
func GetIPAddress(subnet v1beta2.SubnetSpec, defaultPrefix int32, index int) (netip.Addr, error) {
	if subnet.CIDR != "" {
		return getIPFromCIDR(subnet.CIDR, index)
	}

	if subnet.Start != "" && subnet.End != "" {
		return getIPFromRange(subnet.Start, subnet.End, index)
	}

	return netip.Addr{}, fmt.Errorf("subnet must have either CIDR or Start/End")
}

// getIPFromCIDR returns the IP at the given index within a CIDR block.
func getIPFromCIDR(cidr string, index int) (netip.Addr, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	// Start from first usable IP (skip network address for IPv4)
	current := firstUsableIP(prefix)

	// Get the last usable IP
	last := lastUsableIP(prefix)

	// Advance by index
	for i := 0; i < index; i++ {
		current = current.Next()
		if !current.IsValid() || current.Compare(last) > 0 {
			return netip.Addr{}, fmt.Errorf("index %d out of range for CIDR %s", index, cidr)
		}
	}

	return current, nil
}

// getIPFromRange returns the IP at the given index between start and end IPs.
func getIPFromRange(start, end string, index int) (netip.Addr, error) {
	startIP, err := netip.ParseAddr(start)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid start IP %s: %w", start, err)
	}

	endIP, err := netip.ParseAddr(end)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid end IP %s: %w", end, err)
	}

	if startIP.Compare(endIP) > 0 {
		return netip.Addr{}, fmt.Errorf("start IP %s is greater than end IP %s", start, end)
	}

	// Advance by index
	current := startIP
	for i := 0; i < index; i++ {
		current = current.Next()
		if !current.IsValid() || current.Compare(endIP) > 0 {
			return netip.Addr{}, fmt.Errorf("index %d out of range for range %s-%s", index, start, end)
		}
	}

	return current, nil
}

// IPInSubnets checks if an IP address is within any of the configured subnets.
func IPInSubnets(ip string, subnets []v1beta2.SubnetSpec, defaultPrefix int32) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	for _, subnet := range subnets {
		if inSubnet(addr, subnet) {
			return true
		}
	}

	return false
}

// inSubnet checks if an IP is within a specific subnet.
func inSubnet(addr netip.Addr, subnet v1beta2.SubnetSpec) bool {
	if subnet.CIDR != "" {
		prefix, err := netip.ParsePrefix(subnet.CIDR)
		if err != nil {
			return false
		}
		return prefix.Contains(addr)
	}

	if subnet.Start != "" && subnet.End != "" {
		startIP, err := netip.ParseAddr(subnet.Start)
		if err != nil {
			return false
		}

		endIP, err := netip.ParseAddr(subnet.End)
		if err != nil {
			return false
		}

		return addr.Compare(startIP) >= 0 && addr.Compare(endIP) <= 0
	}

	return false
}

// firstUsableIP returns the first usable IP in a prefix.
// For IPv4, this skips the network address (first IP in the range).
// For IPv6, returns the first IP in the prefix.
func firstUsableIP(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr()

	// For IPv4, skip network address (e.g., 192.168.1.0 in 192.168.1.0/24)
	if addr.Is4() {
		return addr.Next()
	}

	// For IPv6, first IP is usable
	return addr
}

// lastUsableIP returns the last usable IP in a prefix.
// For IPv4, this skips the broadcast address (last IP in the range).
// For IPv6, returns the last IP in the prefix.
func lastUsableIP(prefix netip.Prefix) netip.Addr {
	// Use bit manipulation to calculate the last IP directly
	// instead of iterating through all addresses
	mask := prefix.Masked().Addr()
	bits := prefix.Bits()

	if mask.Is4() {
		// IPv4: Calculate last IP using bit operations
		maskBytes := mask.As4()
		hostBits := 32 - bits

		// Create a mask with all host bits set
		hostMask := uint32((1 << hostBits) - 1)

		// Convert IP to uint32
		ipUint := binary.BigEndian.Uint32(maskBytes[:])

		// Set all host bits (this gives broadcast address)
		broadcastUint := ipUint | hostMask

		// Subtract 1 to get last usable IP (skip broadcast)
		lastUsableUint := broadcastUint - 1

		// Convert back to netip.Addr
		var last [4]byte
		binary.BigEndian.PutUint32(last[:], lastUsableUint)
		return netip.AddrFrom4(last)
	}

	// IPv6: Calculate last IP using bit operations
	maskBytes := mask.As16()
	hostBits := 128 - bits

	// Convert to two uint64s (high and low 64 bits)
	high := binary.BigEndian.Uint64(maskBytes[0:8])
	low := binary.BigEndian.Uint64(maskBytes[8:16])

	// Set all host bits
	if hostBits <= 64 {
		// All host bits are in the low part
		hostMask := (uint64(1) << hostBits) - 1
		low |= hostMask
	} else {
		// Host bits span both parts
		lowBits := hostBits - 64
		high |= (uint64(1) << lowBits) - 1
		low = 0xFFFFFFFFFFFFFFFF
	}

	// For IPv6, we return the last IP (no broadcast address to skip)
	var last [16]byte
	binary.BigEndian.PutUint64(last[0:8], high)
	binary.BigEndian.PutUint64(last[8:16], low)
	return netip.AddrFrom16(last)
}

// GetPrefix returns the prefix to use for a subnet, considering overrides.
func GetPrefix(subnet v1beta2.SubnetSpec, defaultPrefix int32) int32 {
	if subnet.Prefix != nil {
		return *subnet.Prefix
	}
	if subnet.CIDR != "" {
		prefix, err := netip.ParsePrefix(subnet.CIDR)
		if err == nil {
			return int32(prefix.Bits()) // #nosec G115 - prefix bits are within safe range
		}
	}
	return defaultPrefix
}

// GetGateway returns the gateway to use for a subnet, considering overrides.
func GetGateway(subnet v1beta2.SubnetSpec, defaultGateway string) string {
	if subnet.Gateway != "" {
		return subnet.Gateway
	}
	return defaultGateway
}

// GetDNSServers returns the DNS servers to use for a subnet, considering overrides.
func GetDNSServers(subnet v1beta2.SubnetSpec, defaultDNS []string) []string {
	if len(subnet.DNSServers) > 0 {
		return subnet.DNSServers
	}
	return defaultDNS
}
