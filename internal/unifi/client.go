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

package unifi

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"time"

	"github.com/ubiquiti-community/go-unifi/unifi"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/poolutil"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

// Config holds the configuration for connecting to a Unifi controller.
type Config struct {
	Host       string
	APIKey     string
	Site       string
	Insecure   bool
	HTTPClient *http.Client
}

// Client wraps the Unifi API client with IPAM-specific operations.
type Client struct {
	client *unifi.Client
	site   string
}

// IPAllocation represents an allocated IP address.
type IPAllocation struct {
	IPAddress  string
	MacAddress string
	Hostname   string
	UseFixedIP bool
	Prefix     int32
	Gateway    string
}

// NewClient creates a new Unifi client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Site == "" {
		cfg.Site = "default"
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: cfg.Insecure, //nolint:gosec // G402: User-configurable for development/testing environments
				},
			},
		}
	}

	// Create the client.
	client := &unifi.Client{}

	// Set API key (required for authentication).
	client.SetAPIKey(cfg.APIKey)

	// Configure HTTP client.
	if err := client.SetHTTPClient(httpClient); err != nil {
		return nil, fmt.Errorf("failed to set HTTP client: %w", err)
	}

	// Set base URL.
	if err := client.SetBaseURL(cfg.Host); err != nil {
		return nil, fmt.Errorf("failed to set base URL: %w", err)
	}

	// Login to the controller (with API key, no user/pass needed).
	if err := client.Login(context.Background(), "", ""); err != nil {
		return nil, fmt.Errorf("failed to login to Unifi controller: %w", err)
	}

	return &Client{
		client: client,
		site:   cfg.Site,
	}, nil
}

// ValidateCredentials tests the connection and credentials.
func (c *Client) ValidateCredentials(ctx context.Context) error {
	// Try to list networks as a validation check.
	_, err := c.client.ListNetwork(ctx, c.site)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	return nil
}

// GetNetwork retrieves network information by ID.
func (c *Client) GetNetwork(ctx context.Context, networkID string) (*unifi.Network, error) {
	networks, err := c.client.ListNetwork(ctx, c.site)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	for i := range networks {
		if networks[i].ID == networkID {
			return &networks[i], nil
		}
	}

	return nil, fmt.Errorf("network %s not found", networkID)
}

// SyncNetworkToCIDR retrieves network configuration from Unifi and populates SubnetSpec.
// This syncs the CIDR, gateway, and optionally calculates prefix and exclude ranges based on DHCP settings.
//
//nolint:cyclop // Network configuration sync requires multiple conditional checks
func (c *Client) SyncNetworkToCIDR(ctx context.Context, networkID string) (*v1beta2.SubnetSpec, error) {
	network, err := c.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, err
	}

	// Validate that the network has required DHCP/IP configuration
	if network.IPSubnet == "" {
		return nil, fmt.Errorf("network %s has no IP subnet configured", networkID)
	}

	subnetSpec := &v1beta2.SubnetSpec{
		CIDR: network.IPSubnet,
	}

	// Extract gateway - prefer DHCPDGateway if set, otherwise calculate from CIDR
	if network.DHCPDGateway != "" && network.DHCPDGatewayEnabled {
		subnetSpec.Gateway = network.DHCPDGateway
	} else {
		// Calculate gateway from CIDR (typically .1 of the subnet)
		gateway, err := calculateGatewayFromCIDR(network.IPSubnet)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate gateway: %w", err)
		}
		subnetSpec.Gateway = gateway
	}

	// Calculate prefix from CIDR
	prefix, err := extractPrefixFromCIDR(network.IPSubnet)
	if err != nil {
		return nil, fmt.Errorf("failed to extract prefix: %w", err)
	}
	subnetSpec.Prefix = &prefix

	// Build exclude ranges from DHCP configuration
	excludeRanges := make([]string, 0)

	// If DHCP is enabled, exclude IPs outside the DHCP range
	if network.DHCPDEnabled && network.DHCPDStart != "" && network.DHCPDStop != "" {
		// Calculate exclude ranges for IPs before DHCP start and after DHCP stop
		beforeRange, afterRange, err := calculateExcludeRangesFromDHCP(network.IPSubnet, network.DHCPDStart, network.DHCPDStop)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate exclude ranges: %w", err)
		}
		if beforeRange != "" {
			excludeRanges = append(excludeRanges, beforeRange)
		}
		if afterRange != "" {
			excludeRanges = append(excludeRanges, afterRange)
		}
	}

	// Add DNS servers if configured
	if !network.DHCPDDNSEnabled {
		// DNS not enabled, skip DNS configuration
	} else {
		dnsServers := collectDNSServers(network)
		if len(dnsServers) > 0 {
			subnetSpec.DNSServers = dnsServers
		}
	}

	if len(excludeRanges) > 0 {
		subnetSpec.ExcludeRanges = excludeRanges
	}

	return subnetSpec, nil
}

// GetOrAllocateIP gets an existing IP or allocates a new one.
func (c *Client) GetOrAllocateIP(ctx context.Context, pool *v1beta2.UnifiIPPool, claim *ipamv1beta2.IPAddressClaim, networkID, macAddress, hostname string, addressesInUse []ipamv1beta2.IPAddress) (*IPAllocation, error) {
	// First, check if this MAC already has a fixed IP assignment via User object.
	existingUser, err := c.client.GetUserByMAC(ctx, c.site, macAddress)
	if err == nil && existingUser != nil {
		// User exists - return existing allocation with Prefix and Gateway.
		// Need to determine prefix and gateway from pool config.
		defaultPrefix := int32(24)
		if pool.Spec.Prefix != nil && *pool.Spec.Prefix > 0 {
			defaultPrefix = *pool.Spec.Prefix
		}

		// Find subnet containing the existing IP to get accurate prefix/gateway
		prefix := defaultPrefix
		gateway := pool.Spec.Gateway
		addr, err := netip.ParseAddr(existingUser.FixedIP)
		if err == nil {
			for _, subnet := range pool.Spec.Subnets {
				// Check if IP is in this subnet
				var contains bool
				if subnet.CIDR != "" {
					if subnetPrefix, err := netip.ParsePrefix(subnet.CIDR); err == nil {
						contains = subnetPrefix.Contains(addr)
					}
				} else if subnet.Start != "" && subnet.End != "" {
					if startIP, err := netip.ParseAddr(subnet.Start); err == nil {
						if endIP, err := netip.ParseAddr(subnet.End); err == nil {
							contains = addr.Compare(startIP) >= 0 && addr.Compare(endIP) <= 0
						}
					}
				}
				if contains {
					prefix = poolutil.GetPrefix(subnet, defaultPrefix)
					gateway = poolutil.GetGateway(subnet, pool.Spec.Gateway)
					break
				}
			}
		}

		return &IPAllocation{
			IPAddress:  existingUser.FixedIP,
			MacAddress: existingUser.MAC,
			Hostname:   existingUser.Hostname,
			UseFixedIP: existingUser.UseFixedIP,
			Prefix:     prefix,
			Gateway:    gateway,
		}, nil
	}

	// If not found or error (other than NotFoundError), need to allocate new IP.
	if err != nil {
		// Check if it's a NotFoundError - that's expected, other errors should be returned.
		notFoundError := &unifi.NotFoundError{}
		if errors.As(err, &notFoundError) {
			return nil, fmt.Errorf("failed to check existing user: %w", err)
		}
	}

	// Get the network configuration.
	network, err := c.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, err
	}

	// Allocate the next available IP using 3-level priority algorithm.
	allocatedIP, prefix, gateway, err := c.allocateNextIP(ctx, pool, claim, network, addressesInUse)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Create a User object with fixed IP assignment.
	newUser := &unifi.User{
		MAC:        macAddress,
		FixedIP:    allocatedIP,
		Hostname:   hostname,
		UseFixedIP: true,
		NetworkID:  networkID,
	}

	// Create the user in Unifi controller.
	createdUser, err := c.client.CreateUser(ctx, c.site, newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to create user with fixed IP: %w", err)
	}

	// Return the allocation with metadata.
	return &IPAllocation{
		IPAddress:  createdUser.FixedIP,
		MacAddress: createdUser.MAC,
		Hostname:   createdUser.Hostname,
		UseFixedIP: createdUser.UseFixedIP,
		Prefix:     prefix,
		Gateway:    gateway,
	}, nil
}

// allocateNextIP finds the next available IP using 3-level priority algorithm:
// 1. PreAllocations (static assignment or IP reuse)
// 2. Annotation request (claim specifies desired IP)
// 3. Dynamic allocation (iterate through subnets)
func (c *Client) allocateNextIP(ctx context.Context, pool *v1beta2.UnifiIPPool, claim *ipamv1beta2.IPAddressClaim, network *unifi.Network, addressesInUse []ipamv1beta2.IPAddress) (string, int32, string, error) {
	if pool == nil {
		return "", 0, "", fmt.Errorf("pool is nil")
	}
	if len(pool.Spec.Subnets) == 0 {
		return "", 0, "", fmt.Errorf("pool has no configured subnets")
	}

	// Get default prefix for validation
	defaultPrefix := int32(24) // fallback
	if pool.Spec.Prefix != nil && *pool.Spec.Prefix > 0 {
		defaultPrefix = *pool.Spec.Prefix
	}

	// PRIORITY 1: Check PreAllocations map
	if pool.Spec.PreAllocations != nil && claim != nil {
		if prealloc, exists := pool.Spec.PreAllocations[claim.Name]; exists {
			// Validate preallocated IP is in configured subnets
			if !poolutil.IPInSubnets(prealloc, pool.Spec.Subnets, defaultPrefix) {
				return "", 0, "", fmt.Errorf("preallocated IP %s for claim %s is not in configured subnets", prealloc, claim.Name)
			}

			// Check if preallocated IP is already assigned to a different claim
			for _, addr := range addressesInUse {
				if addr.Spec.Address == prealloc {
					// Check if it's assigned to the same claim (reuse scenario)
					if addr.Spec.ClaimRef.Name == claim.Name {
						// Same claim - this is IP reuse, allow it
						continue
					}
					return "", 0, "", fmt.Errorf("preallocated IP %s is already assigned to claim %s", prealloc, addr.Spec.ClaimRef.Name)
				}
			}

			// Check Unifi for conflicts
			staticAssignments, err := c.GetStaticAssignments(ctx, network.ID)
			if err != nil {
				return "", 0, "", fmt.Errorf("failed to check Unifi static assignments: %w", err)
			}
			for _, sa := range staticAssignments {
				if sa.IP == prealloc {
					// Check if it's the same MAC (reuse scenario)
					macAddress := generateMACForClaim(claim.Name)
					if sa.MAC == macAddress {
						// Same MAC - this is IP reuse from previous allocation
						continue
					}
					return "", 0, "", fmt.Errorf("preallocated IP %s has Unifi conflict with MAC %s", prealloc, sa.MAC)
				}
			}

			// Find which subnet contains this IP to get metadata
			for _, subnet := range pool.Spec.Subnets {
				prefix := poolutil.GetPrefix(subnet, defaultPrefix)
				gateway := poolutil.GetGateway(subnet, pool.Spec.Gateway)

				addr, err := netip.ParseAddr(prealloc)
				if err != nil {
					continue
				}

				// Check if IP is in this subnet
				var contains bool
				if subnet.CIDR != "" {
					if subnetPrefix, err := netip.ParsePrefix(subnet.CIDR); err == nil {
						contains = subnetPrefix.Contains(addr)
					}
				} else if subnet.Start != "" && subnet.End != "" {
					if startIP, err := netip.ParseAddr(subnet.Start); err == nil {
						if endIP, err := netip.ParseAddr(subnet.End); err == nil {
							contains = addr.Compare(startIP) >= 0 && addr.Compare(endIP) <= 0
						}
					}
				}
				if contains {
					return prealloc, prefix, gateway, nil
				}
			}

			// If we reach here, IP is valid but couldn't determine subnet metadata
			return prealloc, defaultPrefix, pool.Spec.Gateway, nil
		}
	}

	// PRIORITY 2: Check annotation for requested IP
	if claim != nil && claim.Annotations != nil {
		if requestedIP, exists := claim.Annotations["ipAddress"]; exists && requestedIP != "" {
			// Validate requested IP (similar to preallocated IP validation)
			if !poolutil.IPInSubnets(requestedIP, pool.Spec.Subnets, defaultPrefix) {
				return "", 0, "", fmt.Errorf("requested IP %s is not in configured subnets", requestedIP)
			}

			// Check if already assigned
			for _, addr := range addressesInUse {
				if addr.Spec.Address == requestedIP {
					return "", 0, "", fmt.Errorf("requested IP %s is already assigned", requestedIP)
				}
			}

			// Check Unifi for conflicts
			staticAssignments, err := c.GetStaticAssignments(ctx, network.ID)
			if err != nil {
				return "", 0, "", fmt.Errorf("failed to check Unifi static assignments: %w", err)
			}
			for _, sa := range staticAssignments {
				if sa.IP == requestedIP {
					return "", 0, "", fmt.Errorf("requested IP %s has Unifi conflict", requestedIP)
				}
			}

			// Find subnet metadata
			for _, subnet := range pool.Spec.Subnets {
				prefix := poolutil.GetPrefix(subnet, defaultPrefix)
				gateway := poolutil.GetGateway(subnet, pool.Spec.Gateway)

				addr, err := netip.ParseAddr(requestedIP)
				if err != nil {
					continue
				}

				// Check if IP is in this subnet
				var contains bool
				if subnet.CIDR != "" {
					if subnetPrefix, err := netip.ParsePrefix(subnet.CIDR); err == nil {
						contains = subnetPrefix.Contains(addr)
					}
				} else if subnet.Start != "" && subnet.End != "" {
					if startIP, err := netip.ParseAddr(subnet.Start); err == nil {
						if endIP, err := netip.ParseAddr(subnet.End); err == nil {
							contains = addr.Compare(startIP) >= 0 && addr.Compare(endIP) <= 0
						}
					}
				}
				if contains {
					return requestedIP, prefix, gateway, nil
				}
			}

			// If we reach here, IP is valid but couldn't determine subnet metadata
			return requestedIP, defaultPrefix, pool.Spec.Gateway, nil
		}
	}

	// PRIORITY 3: Dynamic allocation using iteration
	// Build map of allocated IPs (from both CAPI and Unifi)
	allocatedIPs := make(map[string]bool)
	for _, addr := range addressesInUse {
		allocatedIPs[addr.Spec.Address] = true
	}

	// Get Unifi static assignments
	staticAssignments, err := c.GetStaticAssignments(ctx, network.ID)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to get Unifi static assignments: %w", err)
	}
	for _, sa := range staticAssignments {
		allocatedIPs[sa.IP] = true
	}

	// Iterate through all subnets
	for _, subnet := range pool.Spec.Subnets {
		prefix := poolutil.GetPrefix(subnet, defaultPrefix)
		gateway := poolutil.GetGateway(subnet, pool.Spec.Gateway)

		// Iterate through IPs in this subnet
		index := 0
		for {
			ip, err := poolutil.GetIPAddress(subnet, defaultPrefix, index)
			if err != nil {
				// Out of range or error - try next subnet
				break
			}
			index++

			ipStr := ip.String()

			// Skip gateway
			if ipStr == gateway {
				continue
			}

			// Skip if already allocated
			if allocatedIPs[ipStr] {
				continue
			}

			// Found free IP!
			return ipStr, prefix, gateway, nil
		}
	}

	return "", 0, "", fmt.Errorf("exhausted IP pool: no free IPs available")
}

// generateMACForClaim generates a deterministic MAC address for a claim name.
// Uses SHA256 to avoid collisions that would occur with simple length-based hashing.
func generateMACForClaim(claimName string) string {
	// Use SHA256 to generate a deterministic hash
	h := sha256.Sum256([]byte(claimName))

	// Use first 5 bytes from hash, with locally administered bit set
	// 02:xx:xx:xx:xx:xx format ensures it's a locally administered unicast MAC
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x", h[0], h[1], h[2], h[3], h[4])
}

// getExistingClientIPs retrieves all currently active/leased IPs from Unifi clients.
// This helps avoid allocating IPs that are already in use by existing network devices.
func (c *Client) getExistingClientIPs(ctx context.Context, networkID string) ([]string, error) {
	// List all active clients on the site (this includes both wired and wireless clients)
	clients, err := c.client.ListClientsActive(ctx, c.site)
	if err != nil {
		return nil, fmt.Errorf("failed to list active clients: %w", err)
	}

	// Collect IPs from clients - include both current IPs and fixed IP assignments
	existingIPs := make([]string, 0, len(clients))
	for i := range clients {
		client := &clients[i]

		// Add the client's current IP (active connection)
		if client.IP != "" {
			// Filter by network ID if specified
			if networkID == "" || client.NetworkId == networkID {
				existingIPs = append(existingIPs, client.IP)
			}
		}

		// Also add any fixed IP assignments from the User records
		if client.FixedIP != "" {
			if networkID == "" || client.NetworkId == networkID {
				existingIPs = append(existingIPs, client.FixedIP)
			}
		}
	}

	return existingIPs, nil
}

// ReleaseIP releases an allocated IP address.
func (c *Client) ReleaseIP(ctx context.Context, networkID, ipAddress, macAddress string) error {
	// Delete the User object which releases the fixed IP assignment.
	err := c.client.DeleteUserByMAC(ctx, c.site, macAddress)
	if err != nil {
		// If the user is not found, that's acceptable - already released.
		notFoundError := &unifi.NotFoundError{}
		if errors.As(err, &notFoundError) {
			return nil
		}
		return fmt.Errorf("failed to delete user with MAC %s: %w", macAddress, err)
	}
	return nil
}

// StaticAssignment represents a static DHCP assignment in Unifi.
type StaticAssignment struct {
	IP       string
	MAC      string
	Hostname string
}

// GetStaticAssignments retrieves all static DHCP assignments for a network.
// This queries all Unifi User objects with fixed IPs in the specified network.
func (c *Client) GetStaticAssignments(ctx context.Context, networkID string) ([]StaticAssignment, error) {
	// List all users with fixed IP assignments
	users, err := c.client.ListUser(ctx, c.site)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	assignments := make([]StaticAssignment, 0)
	for i := range users {
		user := &users[i]
		// Filter by network and fixed IP
		if user.NetworkID == networkID && user.UseFixedIP && user.FixedIP != "" {
			assignments = append(assignments, StaticAssignment{
				IP:       user.FixedIP,
				MAC:      user.MAC,
				Hostname: user.Hostname,
			})
		}
	}

	return assignments, nil
}

// CreateStaticAssignment creates a static DHCP assignment in Unifi.
func (c *Client) CreateStaticAssignment(ctx context.Context, networkID, ip, macAddress, hostname string) error {
	// Create or update User object with fixed IP
	user := &unifi.User{
		MAC:        macAddress,
		FixedIP:    ip,
		Hostname:   hostname,
		UseFixedIP: true,
		NetworkID:  networkID,
	}

	_, err := c.client.CreateUser(ctx, c.site, user)
	if err != nil {
		return fmt.Errorf("failed to create static assignment: %w", err)
	}

	return nil
}

// DeleteStaticAssignment removes a static DHCP assignment by MAC address.
func (c *Client) DeleteStaticAssignment(ctx context.Context, networkID, macAddress string) error {
	err := c.client.DeleteUserByMAC(ctx, c.site, macAddress)
	if err != nil {
		// If the user is not found, that's acceptable - already released.
		notFoundError := &unifi.NotFoundError{}
		if errors.As(err, &notFoundError) {
			return nil
		}
		return fmt.Errorf("failed to delete static assignment: %w", err)
	}
	return nil
}

// FindNetworkForSubnet auto-discovers a Unifi network that contains the given subnet.
// Returns the network if found, or an error if no matching network exists.
func (c *Client) FindNetworkForSubnet(ctx context.Context, subnet string) (*unifi.Network, error) {
	// Parse the subnet to check
	var subnetPrefix netip.Prefix
	var err error

	// Try parsing as CIDR
	subnetPrefix, err = netip.ParsePrefix(subnet)
	if err != nil {
		// Try parsing as IP range (we'll assume the whole range for simplicity)
		return nil, fmt.Errorf("subnet must be a valid CIDR: %w", err)
	}

	// List all networks
	networks, err := c.client.ListNetwork(ctx, c.site)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	// Find a network whose subnet contains the configured subnet
	for i := range networks {
		network := &networks[i]
		if network.IPSubnet == "" {
			continue
		}

		// Parse network's subnet
		networkPrefix, err := netip.ParsePrefix(network.IPSubnet)
		if err != nil {
			continue
		}

		// Check if network contains the pool subnet
		// For a network to contain a subnet, the subnet must be within the network's range
		if networkPrefix.Contains(subnetPrefix.Addr()) {
			// Also verify the subnet doesn't exceed the network range
			subnetEnd := lastAddrInPrefix(subnetPrefix)
			if networkPrefix.Contains(subnetEnd) {
				return network, nil
			}
		}
	}

	return nil, fmt.Errorf("no Unifi network found containing subnet %s", subnet)
}

// lastAddrInPrefix returns the last IP address in a prefix.
func lastAddrInPrefix(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr()
	bits := prefix.Bits()

	if addr.Is4() {
		// Calculate host bits
		hostBits := 32 - bits
		hostMask := uint32((1 << hostBits) - 1)

		// Get base IP as uint32
		octets := addr.As4()
		ipInt := uint32(octets[0])<<24 | uint32(octets[1])<<16 | uint32(octets[2])<<8 | uint32(octets[3])

		// Add host mask
		lastInt := ipInt | hostMask

		return netip.AddrFrom4([4]byte{
			byte(lastInt >> 24),
			byte(lastInt >> 16),
			byte(lastInt >> 8),
			byte(lastInt),
		})
	}

	// For IPv6, use simpler approach
	return addr
}

// Helper functions for CIDR and network calculations

// calculateGatewayFromCIDR extracts the first usable IP from a CIDR as the gateway.
// Typically this is .1 for the subnet.
func calculateGatewayFromCIDR(cidr string) (string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	// Get the network address and add 1 for the gateway
	netAddr := prefix.Addr()
	if !netAddr.Is4() {
		return "", fmt.Errorf("only IPv4 subnets are supported")
	}

	// Convert to 4-byte array and increment
	octets := netAddr.As4()
	octets[3]++ // Increment last octet for .1 address

	gateway := netip.AddrFrom4(octets)
	return gateway.String(), nil
}

// extractPrefixFromCIDR returns the prefix length from a CIDR string.
func extractPrefixFromCIDR(cidr string) (int32, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return 0, fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}
	// Prefix bits are always 0-32 for IPv4, 0-128 for IPv6
	return int32(prefix.Bits()), nil // #nosec G115 - prefix bits are within safe range
}

// calculateExcludeRangesFromDHCP calculates IP ranges to exclude based on DHCP start/stop.
// Returns ranges before DHCP start and after DHCP stop (excluding network and broadcast).
func calculateExcludeRangesFromDHCP(cidr, dhcpStart, dhcpStop string) (beforeRange, afterRange string, err error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", "", fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	startIP, err := netip.ParseAddr(dhcpStart)
	if err != nil {
		return "", "", fmt.Errorf("invalid DHCP start IP %s: %w", dhcpStart, err)
	}

	stopIP, err := netip.ParseAddr(dhcpStop)
	if err != nil {
		return "", "", fmt.Errorf("invalid DHCP stop IP %s: %w", dhcpStop, err)
	}

	// Get network address (first IP) and broadcast (last IP)
	netAddr := prefix.Masked().Addr()
	broadcastAddr := calculateBroadcastAddr(prefix)

	// Calculate first usable IP (network + 1) and last usable IP (broadcast - 1)
	firstUsable := incrementIP(netAddr)
	lastUsable := decrementIP(broadcastAddr)

	// Build exclude range before DHCP start (if DHCP doesn't start at first usable)
	if startIP.Compare(firstUsable) > 0 {
		// Exclude from firstUsable to (startIP - 1)
		beforeEnd := decrementIP(startIP)
		beforeRange = formatIPRange(firstUsable, beforeEnd)
	}

	// Build exclude range after DHCP stop (if DHCP doesn't end at last usable)
	if stopIP.Compare(lastUsable) < 0 {
		// Exclude from (stopIP + 1) to lastUsable
		afterStart := incrementIP(stopIP)
		afterRange = formatIPRange(afterStart, lastUsable)
	}

	return beforeRange, afterRange, nil
}

// calculateBroadcastAddr calculates the broadcast address for a given prefix.
func calculateBroadcastAddr(prefix netip.Prefix) netip.Addr {
	if !prefix.Addr().Is4() {
		return netip.Addr{} // Only support IPv4 for now
	}

	addr := prefix.Addr().As4()
	maskBits := prefix.Bits()

	// Create host mask (inverse of network mask)
	hostMask := uint32((1 << (32 - maskBits)) - 1)

	// Convert address to uint32
	ipInt := uint32(addr[0])<<24 | uint32(addr[1])<<16 | uint32(addr[2])<<8 | uint32(addr[3])

	// OR with host mask to get broadcast
	broadcastInt := ipInt | hostMask

	// Convert back to addr
	broadcastAddr := netip.AddrFrom4([4]byte{
		byte(broadcastInt >> 24),
		byte(broadcastInt >> 16),
		byte(broadcastInt >> 8),
		byte(broadcastInt),
	})

	return broadcastAddr
}

// incrementIP returns the next IP address.
func incrementIP(ip netip.Addr) netip.Addr {
	if !ip.Is4() {
		return ip // Only support IPv4
	}

	octets := ip.As4()
	// Increment with carry
	for i := 3; i >= 0; i-- {
		if octets[i] < 255 {
			octets[i]++ // #nosec G602 - i is bounded by loop condition
			break
		}
		octets[i] = 0 // #nosec G602 - i is bounded by loop condition
	}

	return netip.AddrFrom4(octets)
}

// decrementIP returns the previous IP address.
func decrementIP(ip netip.Addr) netip.Addr {
	if !ip.Is4() {
		return ip // Only support IPv4
	}

	octets := ip.As4()
	// Decrement with borrow
	for i := 3; i >= 0; i-- {
		if octets[i] > 0 {
			octets[i]-- // #nosec G602 - i is bounded by loop condition
			break
		}
		octets[i] = 255 // #nosec G602 - i is bounded by loop condition
	}

	return netip.AddrFrom4(octets)
}

// formatIPRange formats two IP addresses as a CIDR or range string.
// If they form a valid CIDR block, returns CIDR notation, otherwise returns "start-end".
func formatIPRange(start, end netip.Addr) string {
	if !start.Is4() || !end.Is4() {
		return "" // Only support IPv4
	}

	// Try to express as CIDR if possible
	// For simplicity, just return as IP range format
	return fmt.Sprintf("%s-%s", start.String(), end.String())
}

// collectDNSServers gathers non-empty DNS server addresses from network configuration.
func collectDNSServers(network *unifi.Network) []string {
	dnsServers := make([]string, 0, 4)
	if network.DHCPDDNS1 != "" {
		dnsServers = append(dnsServers, network.DHCPDDNS1)
	}
	if network.DHCPDDNS2 != "" {
		dnsServers = append(dnsServers, network.DHCPDDNS2)
	}
	if network.DHCPDDNS3 != "" {
		dnsServers = append(dnsServers, network.DHCPDDNS3)
	}
	if network.DHCPDDNS4 != "" {
		dnsServers = append(dnsServers, network.DHCPDDNS4)
	}
	return dnsServers
}
