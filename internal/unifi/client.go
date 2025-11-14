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
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ubiquiti-community/go-unifi/unifi"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/poolutil"

	ipamv1beta1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
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

// GetOrAllocateIP gets an existing IP or allocates a new one.
func (c *Client) GetOrAllocateIP(ctx context.Context, networkID, macAddress, hostname string, poolSpec *v1beta2.SubnetSpec, addressesInUse []ipamv1beta1.IPAddress) (*IPAllocation, error) {
	// First, check if this MAC already has a fixed IP assignment via User object.
	existingUser, err := c.client.GetUserByMAC(ctx, c.site, macAddress)
	if err == nil && existingUser != nil {
		// User exists - return existing allocation.
		return &IPAllocation{
			IPAddress:  existingUser.FixedIP,
			MacAddress: existingUser.MAC,
			Hostname:   existingUser.Hostname,
			UseFixedIP: existingUser.UseFixedIP,
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

	// Allocate the next available IP using poolutil.
	allocatedIP, err := c.allocateNextIP(network, poolSpec, addressesInUse)
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

	// Return the allocation.
	return &IPAllocation{
		IPAddress:  createdUser.FixedIP,
		MacAddress: createdUser.MAC,
		Hostname:   createdUser.Hostname,
		UseFixedIP: createdUser.UseFixedIP,
	}, nil
}

// allocateNextIP finds the next available IP using poolutil.
func (c *Client) allocateNextIP(network *unifi.Network, subnetSpec *v1beta2.SubnetSpec, addressesInUse []ipamv1beta1.IPAddress) (string, error) {
	if subnetSpec == nil {
		return "", fmt.Errorf("subnet spec is nil")
	}

	// Build list of in-use IPs including:
	// 1. IPs from Kubernetes IPAddress resources (CAPI-managed)
	// 2. Gateway
	// 3. Existing Unifi clients/leases (to avoid conflicts with existing network devices)
	inUseAddresses := make([]string, 0, len(addressesInUse)+1)

	// Add CAPI-managed IPs
	for _, addr := range addressesInUse {
		if addr.Spec.Address != "" {
			inUseAddresses = append(inUseAddresses, addr.Spec.Address)
		}
	}

	// Add gateway
	if subnetSpec.Gateway != "" {
		inUseAddresses = append(inUseAddresses, subnetSpec.Gateway)
	}

	// Add existing Unifi client IPs to avoid conflicts
	existingIPs, err := c.getExistingClientIPs(context.Background(), network.ID)
	if err != nil {
		// Log warning but continue - we'll at least avoid CAPI-managed conflicts
		// TODO: Add proper logging here
		_ = err
	} else {
		inUseAddresses = append(inUseAddresses, existingIPs...)
	}

	// Convert to IPSet.
	inUseIPSet, err := poolutil.AddressesToIPSet(inUseAddresses)
	if err != nil {
		return "", fmt.Errorf("failed to convert addresses to IPSet: %w", err)
	}

	// Build pool IPSet directly from subnet spec.
	poolIPSet, err := poolutil.PoolSpecToIPSet(subnetSpec)
	if err != nil {
		return "", fmt.Errorf("failed to convert pool spec to IPSet: %w", err)
	}

	// Find next available IP.
	availableIP, err := poolutil.FindNextAvailableIP(poolIPSet, inUseIPSet)
	if err != nil {
		return "", fmt.Errorf("failed to find available IP: %w", err)
	}

	return availableIP, nil
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
