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
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/ubiquiti-community/go-unifi/unifi"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/poolutil"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

// Config holds the configuration for connecting to a Unifi controller.
type Config struct {
	Host     string
	APIKey   string
	Site     string
	Insecure bool
}

// APIClient wraps the Unifi API client with IPAM-specific operations.
type APIClient struct {
	api  *unifi.ApiClient
	site string
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

// Request budget.
//
// go-unifi's TimeoutSeconds is PER ATTEMPT, not per call: it lands on
// retryablehttp's inner http.Client, and the retry loop wraps it. Left at
// go-unifi's defaults (30s, RetryMax 4) a single call could run 5 attempts plus
// ~15s of backoff -- about 165s of wall time, five and a half times the flat 30s
// whole-call timeout this package used before the migration, with a reconcile
// worker blocked for all of it.
//
// So bound both knobs explicitly. Two attempts of 15s with retryablehttp's 1s
// minimum backoff puts the worst case at ~31s, which keeps the old whole-call
// budget. One retry is kept rather than none because go-unifi retries a
// controller 429 and honors its Retry-After; beyond that, requeuing the
// reconcile is the right retry layer, not the HTTP client.
const (
	requestTimeoutSeconds = 15
	requestRetryMax       = 1
)

// NewAPIClient creates a new Unifi client.
func NewAPIClient(cfg Config) (*APIClient, error) {
	if cfg.Site == "" {
		cfg.Site = "default"
	}

	// go-unifi owns its transport: unifi.New builds a retrying HTTP client with
	// a cookie jar, applies InsecureSkipVerify when AllowInsecure is set, works
	// out the controller's API URL style and, for client/password auth, logs in.
	// With an API key there is no login step.
	timeoutSeconds := requestTimeoutSeconds
	retryMax := requestRetryMax
	client, err := unifi.New(context.Background(), &unifi.Config{
		BaseURL:        cfg.Host,
		APIKey:         cfg.APIKey,
		AllowInsecure:  cfg.Insecure,
		TimeoutSeconds: &timeoutSeconds,
		RetryMax:       &retryMax,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Unifi client: %w", err)
	}

	return &APIClient{
		api:  client,
		site: cfg.Site,
	}, nil
}

// ValidateCredentials tests the connection and credentials.
func (c *APIClient) ValidateCredentials(ctx context.Context) error {
	// Try to list networks as a validation check.
	_, err := c.api.ListNetwork(ctx, c.site)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	return nil
}

// GetNetwork retrieves network information by ID.
func (c *APIClient) GetNetwork(ctx context.Context, networkID string) (*unifi.Network, error) {
	networks, err := c.api.ListNetwork(ctx, c.site)
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
func (c *APIClient) SyncNetworkToCIDR(ctx context.Context, networkID string) (*v1beta2.SubnetSpec, error) {
	network, err := c.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, err
	}

	// go-unifi models these optional network settings as *string, where both a
	// nil pointer and a pointer to "" mean "not configured".
	ipSubnet := DerefString(network.IPSubnet)
	dhcpdGateway := DerefString(network.DHCPDGateway)
	dhcpdStart := DerefString(network.DHCPDStart)
	dhcpdStop := DerefString(network.DHCPDStop)

	// Validate that the network has required DHCP/IP configuration
	if ipSubnet == "" {
		return nil, fmt.Errorf("network %s has no IP subnet configured", networkID)
	}

	subnetSpec := &v1beta2.SubnetSpec{
		CIDR: ipSubnet,
	}

	// Extract gateway - prefer DHCPDGateway if set, otherwise calculate from CIDR
	if dhcpdGateway != "" && network.DHCPDGatewayEnabled {
		subnetSpec.Gateway = dhcpdGateway
	} else {
		// Calculate gateway from CIDR (typically .1 of the subnet)
		gateway, err := calculateGatewayFromCIDR(ipSubnet)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate gateway: %w", err)
		}
		subnetSpec.Gateway = gateway
	}

	// Calculate prefix from CIDR
	prefix, err := extractPrefixFromCIDR(ipSubnet)
	if err != nil {
		return nil, fmt.Errorf("failed to extract prefix: %w", err)
	}
	subnetSpec.Prefix = &prefix

	// Build exclude ranges from DHCP configuration
	excludeRanges := make([]string, 0)

	// If DHCP is enabled, exclude IPs outside the DHCP range
	if network.DHCPDEnabled && dhcpdStart != "" && dhcpdStop != "" {
		// Calculate exclude ranges for IPs before DHCP start and after DHCP stop
		beforeRange, afterRange, err := calculateExcludeRangesFromDHCP(ipSubnet, dhcpdStart, dhcpdStop)
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

// GetOrAllocateIP returns the pool address reserved for macAddress in UniFi,
// reserving one first if needed. A MAC UniFi already knows (a real device) whose
// reservation lies in the pool keeps it; one with no reservation, or one outside
// the pool, has a pool address written onto its existing record. Only an unknown
// MAC gets a new record: UniFi rejects a second record for a known MAC.
func (c *APIClient) GetOrAllocateIP(ctx context.Context, pool *v1beta2.IPPool, claim *ipamv1beta2.IPAddressClaim, networkID, macAddress, hostname string, addressesInUse []ipamv1beta2.IPAddress) (*IPAllocation, error) {
	existingClient, err := c.api.GetClientByMAC(ctx, c.site, macAddress)

	// A NotFoundError means this MAC simply has no record yet -- the normal
	// first-allocation case -- so fall through and allocate. Any other error
	// (auth, transport) says nothing about whether the MAC is assigned, so it
	// must be propagated rather than mistaken for "unassigned": allocating on
	// top of an unread assignment would hand out an IP that is already taken.
	//
	// errors.As, not errors.Is: NotFoundError carries fields and has no Is
	// method, so it is matched by type, and it may arrive wrapped.
	notFoundError := &unifi.NotFoundError{}
	if err != nil && !errors.As(err, &notFoundError) {
		return nil, fmt.Errorf("failed to check existing client: %w", err)
	}

	if existingClient != nil && clientHoldsPoolAddress(pool, existingClient) {
		return allocationFromExistingClient(pool, existingClient), nil
	}

	allocatedIP, prefix, gateway, err := c.allocateNextIP(ctx, pool, claim, macAddress, addressesInUse)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	var record *unifi.Client
	if existingClient != nil {
		existingClient.FixedIP = allocatedIP
		existingClient.UseFixedIP = true
		existingClient.NetworkID = networkID
		record, err = c.api.UpdateClient(ctx, c.site, existingClient)
		if err != nil {
			return nil, fmt.Errorf("failed to update client %s with fixed IP: %w", macAddress, err)
		}
	} else {
		record, err = c.api.CreateClient(ctx, c.site, &unifi.Client{
			MAC:        macAddress,
			FixedIP:    allocatedIP,
			Hostname:   hostname,
			UseFixedIP: true,
			NetworkID:  networkID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create client with fixed IP: %w", err)
		}
	}

	return &IPAllocation{
		IPAddress:  record.FixedIP,
		MacAddress: record.MAC,
		Hostname:   record.Hostname,
		UseFixedIP: record.UseFixedIP,
		Prefix:     prefix,
		Gateway:    gateway,
	}, nil
}

// clientHoldsPoolAddress reports whether client already has a fixed IP inside
// one of pool's subnets, i.e. a reservation this pool can simply hand back.
func clientHoldsPoolAddress(pool *v1beta2.IPPool, client *unifi.Client) bool {
	return client.UseFixedIP && client.FixedIP != "" &&
		poolutil.IPInSubnets(client.FixedIP, pool.Spec.Subnets, defaultPrefixFor(pool))
}

// allocateNextIP finds the next available IP using 3-level priority algorithm:
// 1. PreAllocations (static assignment or IP reuse)
// 2. Annotation request (claim specifies desired IP)
// 3. Dynamic allocation (iterate through subnets)
func (c *APIClient) allocateNextIP(ctx context.Context, pool *v1beta2.IPPool, claim *ipamv1beta2.IPAddressClaim, macAddress string, addressesInUse []ipamv1beta2.IPAddress) (string, int32, string, error) {
	if pool == nil {
		return "", 0, "", fmt.Errorf("pool is nil")
	}
	if len(pool.Spec.Subnets) == 0 {
		return "", 0, "", fmt.Errorf("pool has no configured subnets")
	}
	defaultPrefix := defaultPrefixFor(pool)

	ip, prefix, gateway, ok, err := c.allocateFromPreAllocation(ctx, pool, claim, macAddress, addressesInUse, defaultPrefix)
	if err != nil {
		return "", 0, "", err
	}
	if ok {
		return ip, prefix, gateway, nil
	}

	ip, prefix, gateway, ok, err = c.allocateFromAnnotation(ctx, pool, claim, addressesInUse, defaultPrefix)
	if err != nil {
		return "", 0, "", err
	}
	if ok {
		return ip, prefix, gateway, nil
	}

	return c.allocateDynamic(ctx, pool, addressesInUse, defaultPrefix)
}

// allocateFromPreAllocation honors an explicit pool.Spec.PreAllocations entry for
// claim, if one exists. ok is false (with a nil error) when no entry applies, so
// the caller falls through to the next allocation priority.
func (c *APIClient) allocateFromPreAllocation(ctx context.Context, pool *v1beta2.IPPool, claim *ipamv1beta2.IPAddressClaim, macAddress string, addressesInUse []ipamv1beta2.IPAddress, defaultPrefix int32) (ip string, prefix int32, gateway string, ok bool, err error) {
	if pool.Spec.PreAllocations == nil || claim == nil {
		return "", 0, "", false, nil
	}
	prealloc, exists := pool.Spec.PreAllocations[claim.Name]
	if !exists {
		return "", 0, "", false, nil
	}

	if !poolutil.IPInSubnets(prealloc, pool.Spec.Subnets, defaultPrefix) {
		return "", 0, "", false, fmt.Errorf("preallocated IP %s for claim %s is not in configured subnets", prealloc, claim.Name)
	}
	if err := preAllocationConflict(addressesInUse, prealloc, claim.Name); err != nil {
		return "", 0, "", false, err
	}
	if err := c.checkUnifiPreAllocationConflict(ctx, prealloc, macAddress); err != nil {
		return "", 0, "", false, err
	}

	prefix, gateway = subnetMetadataForIP(pool, prealloc, defaultPrefix)
	return prealloc, prefix, gateway, true, nil
}

// allocateFromAnnotation honors claim's "ipAddress" annotation, if set. ok is
// false (with a nil error) when the claim has no such request, so the caller
// falls through to dynamic allocation.
func (c *APIClient) allocateFromAnnotation(ctx context.Context, pool *v1beta2.IPPool, claim *ipamv1beta2.IPAddressClaim, addressesInUse []ipamv1beta2.IPAddress, defaultPrefix int32) (ip string, prefix int32, gateway string, ok bool, err error) {
	if claim == nil || claim.Annotations == nil {
		return "", 0, "", false, nil
	}
	requestedIP, exists := claim.Annotations["ipAddress"]
	if !exists || requestedIP == "" {
		return "", 0, "", false, nil
	}

	if !poolutil.IPInSubnets(requestedIP, pool.Spec.Subnets, defaultPrefix) {
		return "", 0, "", false, fmt.Errorf("requested IP %s is not in configured subnets", requestedIP)
	}
	if err := requestedIPConflict(addressesInUse, requestedIP); err != nil {
		return "", 0, "", false, err
	}
	if err := c.checkUnifiRequestedIPConflict(ctx, requestedIP); err != nil {
		return "", 0, "", false, err
	}

	prefix, gateway = subnetMetadataForIP(pool, requestedIP, defaultPrefix)
	return requestedIP, prefix, gateway, true, nil
}

// allocateDynamic picks the first free IP across pool's subnets, skipping each
// subnet's gateway, any IP already recorded in-use by CAPI or Unifi, and any IP
// pre-allocated to a claim (which is spoken for whether that claim exists yet or
// not).
func (c *APIClient) allocateDynamic(ctx context.Context, pool *v1beta2.IPPool, addressesInUse []ipamv1beta2.IPAddress, defaultPrefix int32) (string, int32, string, error) {
	allocatedIPs := make(map[string]bool, len(addressesInUse)+len(pool.Spec.PreAllocations))
	for _, addr := range addressesInUse {
		allocatedIPs[addr.Spec.Address] = true
	}
	for _, prealloc := range pool.Spec.PreAllocations {
		allocatedIPs[prealloc] = true
	}

	staticAssignments, err := c.GetStaticAssignments(ctx)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to get Unifi static assignments: %w", err)
	}
	for _, sa := range staticAssignments {
		allocatedIPs[sa.IP] = true
	}

	for _, subnet := range pool.Spec.Subnets {
		prefix := poolutil.GetPrefix(subnet, defaultPrefix)
		gateway := poolutil.GetGateway(subnet, pool.Spec.Gateway)

		if ip, ok := nextFreeIPInSubnet(subnet, defaultPrefix, gateway, allocatedIPs); ok {
			return ip, prefix, gateway, nil
		}
	}

	return "", 0, "", fmt.Errorf("exhausted IP pool: no free IPs available")
}

// nextFreeIPInSubnet walks subnet's addresses in order and returns the first one
// that is neither the gateway nor already recorded in allocatedIPs.
func nextFreeIPInSubnet(subnet v1beta2.SubnetSpec, defaultPrefix int32, gateway string, allocatedIPs map[string]bool) (string, bool) {
	for index := 0; ; index++ {
		ip, err := poolutil.GetIPAddress(subnet, defaultPrefix, index)
		if err != nil {
			// Out of range - try the next subnet.
			return "", false
		}

		ipStr := ip.String()
		if ipStr == gateway || allocatedIPs[ipStr] {
			continue
		}

		return ipStr, true
	}
}

// preAllocationConflict reports whether prealloc is already assigned, via a CAPI
// IPAddress, to a claim other than claimName. Reassigning the same claim's own
// address is allowed (reuse).
func preAllocationConflict(addressesInUse []ipamv1beta2.IPAddress, prealloc, claimName string) error {
	for _, addr := range addressesInUse {
		if addr.Spec.Address != prealloc || addr.Spec.ClaimRef.Name == claimName {
			continue
		}
		return fmt.Errorf("preallocated IP %s is already assigned to claim %s", prealloc, addr.Spec.ClaimRef.Name)
	}
	return nil
}

// requestedIPConflict reports whether requestedIP is already assigned to any
// claim via a CAPI IPAddress.
func requestedIPConflict(addressesInUse []ipamv1beta2.IPAddress, requestedIP string) error {
	for _, addr := range addressesInUse {
		if addr.Spec.Address == requestedIP {
			return fmt.Errorf("requested IP %s is already assigned", requestedIP)
		}
	}
	return nil
}

// checkUnifiPreAllocationConflict reports whether prealloc is already a static
// assignment in Unifi under a MAC other than the claim's own (its own would
// mean a previous allocation for the same claim being reused).
func (c *APIClient) checkUnifiPreAllocationConflict(ctx context.Context, prealloc, macAddress string) error {
	staticAssignments, err := c.GetStaticAssignments(ctx)
	if err != nil {
		return fmt.Errorf("failed to check Unifi static assignments: %w", err)
	}
	for _, sa := range staticAssignments {
		if sa.IP != prealloc || sa.MAC == macAddress {
			continue
		}
		return fmt.Errorf("preallocated IP %s has Unifi conflict with MAC %s", prealloc, sa.MAC)
	}
	return nil
}

// checkUnifiRequestedIPConflict reports whether requestedIP is already a static
// assignment in Unifi.
func (c *APIClient) checkUnifiRequestedIPConflict(ctx context.Context, requestedIP string) error {
	staticAssignments, err := c.GetStaticAssignments(ctx)
	if err != nil {
		return fmt.Errorf("failed to check Unifi static assignments: %w", err)
	}
	for _, sa := range staticAssignments {
		if sa.IP == requestedIP {
			return fmt.Errorf("requested IP %s has Unifi conflict", requestedIP)
		}
	}
	return nil
}

// allocationFromExistingClient builds the IPAllocation for a MAC that already has
// a fixed-IP Client in Unifi, resolving prefix/gateway from whichever pool subnet
// contains the address.
func allocationFromExistingClient(pool *v1beta2.IPPool, existingClient *unifi.Client) *IPAllocation {
	defaultPrefix := defaultPrefixFor(pool)
	prefix, gateway := defaultPrefix, pool.Spec.Gateway
	if _, err := netip.ParseAddr(existingClient.FixedIP); err == nil {
		prefix, gateway = subnetMetadataForIP(pool, existingClient.FixedIP, defaultPrefix)
	}

	return &IPAllocation{
		IPAddress:  existingClient.FixedIP,
		MacAddress: existingClient.MAC,
		Hostname:   existingClient.Hostname,
		UseFixedIP: existingClient.UseFixedIP,
		Prefix:     prefix,
		Gateway:    gateway,
	}
}

// defaultPrefixFor returns pool's configured default prefix, falling back to /24.
func defaultPrefixFor(pool *v1beta2.IPPool) int32 {
	if pool.Spec.Prefix != nil && *pool.Spec.Prefix > 0 {
		return *pool.Spec.Prefix
	}
	return 24
}

// subnetMetadataForIP finds which of pool's subnets contains ipStr and returns its
// prefix and gateway, falling back to defaultPrefix and the pool's own gateway
// when ipStr doesn't parse or no configured subnet claims it.
func subnetMetadataForIP(pool *v1beta2.IPPool, ipStr string, defaultPrefix int32) (prefix int32, gateway string) {
	prefix, gateway = defaultPrefix, pool.Spec.Gateway

	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return prefix, gateway
	}

	for _, subnet := range pool.Spec.Subnets {
		if subnetContainsAddr(subnet, addr) {
			return poolutil.GetPrefix(subnet, defaultPrefix), poolutil.GetGateway(subnet, pool.Spec.Gateway)
		}
	}

	return prefix, gateway
}

// subnetContainsAddr reports whether addr falls within subnet's CIDR or its
// Start/End range.
func subnetContainsAddr(subnet v1beta2.SubnetSpec, addr netip.Addr) bool {
	if subnet.CIDR != "" {
		subnetPrefix, err := netip.ParsePrefix(subnet.CIDR)
		return err == nil && subnetPrefix.Contains(addr)
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

// captMACAddressAnnotation is what cluster-api-provider-tinkerbell puts on the
// claims it creates, carrying the Hardware interface's MAC.
const captMACAddressAnnotation = "capt.tinkerbell.org/mac-address"

// MACForClaim returns the MAC the claim's UniFi reservation belongs on: the
// device's own MAC when the claim is annotated with one, otherwise a
// deterministic locally administered MAC derived from the claim name so the
// reservation still has a stable owner that cannot collide with another claim's.
func MACForClaim(claim *ipamv1beta2.IPAddressClaim) (string, error) {
	for _, key := range []string{v1beta2.MACAddressAnnotation, captMACAddressAnnotation} {
		raw := claim.Annotations[key]
		if raw == "" {
			continue
		}
		hw, err := net.ParseMAC(raw)
		if err != nil {
			return "", fmt.Errorf("annotation %s on claim %s is not a MAC address: %w", key, claim.Name, err)
		}
		if len(hw) != 6 {
			return "", fmt.Errorf("annotation %s on claim %s is not a 48-bit MAC address: %q", key, claim.Name, raw)
		}
		return hw.String(), nil
	}
	return generateMACForClaim(claim.Name), nil
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

// ReleaseIP releases an allocated IP address.
func (c *APIClient) ReleaseIP(ctx context.Context, networkID, ipAddress, macAddress string) error {
	// Delete the Client object which releases the fixed IP assignment.
	err := c.api.DeleteClientByMAC(ctx, c.site, macAddress)
	if err != nil {
		// If the client is not found, that's acceptable - already released.
		notFoundError := &unifi.NotFoundError{}
		if errors.As(err, &notFoundError) {
			return nil
		}
		return fmt.Errorf("failed to delete client with MAC %s: %w", macAddress, err)
	}
	return nil
}

// StaticAssignment represents a static DHCP assignment in Unifi.
type StaticAssignment struct {
	IP       string
	MAC      string
	Hostname string
}

// GetStaticAssignments retrieves every fixed-IP reservation on the site.
//
// It deliberately does not filter by network: a reservation made from the UniFi
// UI carries no network_id at all, yet the controller still enforces it
// site-wide (api.err.DuplicateFixedIP on any second record with the same IP).
// Reservations in other networks never match a pool address, so including
// them costs nothing; excluding them is what made the allocator hand out
// addresses UniFi then refused.
func (c *APIClient) GetStaticAssignments(ctx context.Context) ([]StaticAssignment, error) {
	clients, err := c.api.ListClient(ctx, c.site)
	if err != nil {
		return nil, fmt.Errorf("failed to list clients: %w", err)
	}

	assignments := make([]StaticAssignment, 0)
	for i := range clients {
		client := &clients[i]
		if client.UseFixedIP && client.FixedIP != "" {
			assignments = append(assignments, StaticAssignment{
				IP:       client.FixedIP,
				MAC:      client.MAC,
				Hostname: client.Hostname,
			})
		}
	}

	return assignments, nil
}

// CreateStaticAssignment creates a static DHCP assignment in Unifi.
func (c *APIClient) CreateStaticAssignment(ctx context.Context, networkID, ip, macAddress, hostname string) error {
	// Create or update Client object with fixed IP
	client := &unifi.Client{
		MAC:        macAddress,
		FixedIP:    ip,
		Hostname:   hostname,
		UseFixedIP: true,
		NetworkID:  networkID,
	}

	_, err := c.api.CreateClient(ctx, c.site, client)
	if err != nil {
		return fmt.Errorf("failed to create static assignment: %w", err)
	}

	return nil
}

// DeleteStaticAssignment removes a static DHCP assignment by MAC address.
func (c *APIClient) DeleteStaticAssignment(ctx context.Context, networkID, macAddress string) error {
	err := c.api.DeleteClientByMAC(ctx, c.site, macAddress)
	if err != nil {
		// If the client is not found, that's acceptable - already released.
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
func (c *APIClient) FindNetworkForSubnet(ctx context.Context, subnet string) (*unifi.Network, error) {
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
	networks, err := c.api.ListNetwork(ctx, c.site)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	// Find a network whose subnet contains the configured subnet
	for i := range networks {
		network := &networks[i]
		ipSubnet := DerefString(network.IPSubnet)
		if ipSubnet == "" {
			continue
		}

		// Parse network's subnet
		networkPrefix, err := netip.ParsePrefix(ipSubnet)
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
		ipInt := binary.BigEndian.Uint32(octets[:])

		// Add host mask
		lastInt := ipInt | hostMask

		var last [4]byte
		binary.BigEndian.PutUint32(last[:], lastInt)
		return netip.AddrFrom4(last)
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
	ipInt := binary.BigEndian.Uint32(addr[:])

	// OR with host mask to get broadcast
	broadcastInt := ipInt | hostMask

	// Convert back to addr
	var broadcast [4]byte
	binary.BigEndian.PutUint32(broadcast[:], broadcastInt)

	return netip.AddrFrom4(broadcast)
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

// DerefString reads an optional string field from a go-unifi struct. go-unifi
// models these as *string and uses both a nil pointer and a pointer to "" for
// "not configured", so both collapse to "" here. That keeps every caller's
// `!= ""` test meaning exactly what it meant when these fields were plain
// strings.
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// DerefInt64 is DerefString's counterpart for go-unifi's optional numeric
// fields: a nil pointer and a pointer to 0 both read as 0, which is the value
// callers already treated as "not configured".
func DerefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

// collectDNSServers gathers non-empty DNS server addresses from network configuration.
func collectDNSServers(network *unifi.Network) []string {
	dnsServers := make([]string, 0, 4)
	if dns := DerefString(network.DHCPDDNS1); dns != "" {
		dnsServers = append(dnsServers, dns)
	}
	if dns := DerefString(network.DHCPDDNS2); dns != "" {
		dnsServers = append(dnsServers, dns)
	}
	if dns := DerefString(network.DHCPDDNS3); dns != "" {
		dnsServers = append(dnsServers, dns)
	}
	if dns := DerefString(network.DHCPDDNS4); dns != "" {
		dnsServers = append(dnsServers, dns)
	}
	return dnsServers
}
