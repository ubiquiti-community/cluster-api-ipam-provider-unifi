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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnifiIPPoolSpec defines the desired state of UnifiIPPool.
type UnifiIPPoolSpec struct {
	// InstanceRef is a reference to the UnifiInstance to use
	// +kubebuilder:validation:Required
	InstanceRef corev1.ObjectReference `json:"instanceRef"`

	// NetworkID is the Unifi network ID to allocate from
	// DEPRECATED: Use auto-discovery instead. If set, skips auto-discovery.
	// This is the _id field from the Unifi network configuration
	// +optional
	NetworkID string `json:"networkId,omitempty"`

	// Subnets is the list of subnets to allocate from
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Subnets []SubnetSpec `json:"subnets"`

	// PreAllocations maps IPAddressClaim names to specific IP addresses
	// Used for static IP assignment and IP reuse across machine recreation
	// Takes priority over dynamic allocation
	// Example: {"cluster-control-plane-0": "10.1.40.10"}
	// +optional
	PreAllocations map[string]string `json:"preAllocations,omitempty"`

	// Prefix is the default network prefix length (e.g., 24 for /24)
	// Can be overridden per subnet
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=128
	// +optional
	Prefix *int32 `json:"prefix,omitempty"`

	// Gateway is the default gateway IP address
	// Can be overridden per subnet. This IP is never allocated.
	// +optional
	Gateway string `json:"gateway,omitempty"`

	// DNSServers is the default list of DNS servers
	// Can be overridden per subnet
	// +optional
	DNSServers []string `json:"dnsServers,omitempty"`
}

// SubnetSpec defines a subnet configuration.
// Supports either CIDR notation OR Start/End IP range (mutually exclusive).
type SubnetSpec struct {
	// CIDR is the subnet CIDR block (e.g., "10.1.40.0/24")
	// Mutually exclusive with Start/End
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`
	// +optional
	CIDR string `json:"cidr,omitempty"`

	// Start is the first IP address in the range (e.g., "10.1.40.10")
	// Requires End field. Mutually exclusive with CIDR.
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	// +optional
	Start string `json:"start,omitempty"`

	// End is the last IP address in the range (e.g., "10.1.40.50")
	// Requires Start field. Mutually exclusive with CIDR.
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	// +optional
	End string `json:"end,omitempty"`

	// Gateway overrides the pool-level gateway for this subnet
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	// +optional
	Gateway string `json:"gateway,omitempty"`

	// Prefix overrides the pool-level prefix for this subnet
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=32
	// +optional
	Prefix *int32 `json:"prefix,omitempty"`

	// ExcludeRanges is a list of IP ranges to exclude from allocation
	// Format: "start-end" (e.g., "192.168.1.1-192.168.1.10")
	// +optional
	ExcludeRanges []string `json:"excludeRanges,omitempty"`

	// DNSServers overrides the pool-level DNS servers for this subnet
	// +optional
	DNSServers []string `json:"dnsServers,omitempty"`
}

// UnifiIPPoolStatus defines the observed state of UnifiIPPool.
type UnifiIPPoolStatus struct {
	// Allocations tracks current IP assignments (claim name → IP address)
	// Automatically populated by watching IPAddress resources
	// Can be copied to Spec.PreAllocations before cluster upgrades for IP reuse
	// +optional
	Allocations map[string]string `json:"allocations,omitempty"`

	// DiscoveredNetworkID is the auto-discovered Unifi network ID
	// Populated by matching configured subnets to Unifi network ranges
	// +optional
	DiscoveredNetworkID string `json:"discoveredNetworkID,omitempty"`

	// Addresses provides summary statistics about address allocation
	// +optional
	Addresses *IPAddressStatusSummary `json:"addresses,omitempty"`

	// Capacity provides utilization metrics for the pool
	// +optional
	Capacity *PoolCapacity `json:"capacity,omitempty"`

	// NetworkInfo contains information about the Unifi network
	// +optional
	NetworkInfo *NetworkInfo `json:"networkInfo,omitempty"`

	// AllocationDetails tracks detailed allocation information
	// +optional
	AllocationDetails *AllocationDetails `json:"allocationDetails,omitempty"`

	// Conditions defines current state of the UnifiIPPool using metav1.Conditions
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastSyncTime is the last time the pool was successfully synced with Unifi
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedNetworkConfiguration contains the network config last observed from Unifi
	// Used to detect drift between Kubernetes and Unifi state
	// +optional
	ObservedNetworkConfiguration *ObservedNetworkConfig `json:"observedNetworkConfig,omitempty"`
}

// ObservedNetworkConfig represents the network configuration observed from Unifi.
// This is used to detect configuration drift.
type ObservedNetworkConfig struct {
	// CIDR is the subnet observed from Unifi
	// +optional.
	CIDR string `json:"cidr,omitempty"`

	// Gateway is the gateway IP observed from Unifi
	// +optional.
	Gateway string `json:"gateway,omitempty"`

	// DHCPEnabled indicates if DHCP is enabled on the network
	// +optional.
	DHCPEnabled *bool `json:"dhcpEnabled,omitempty"`

	// DHCPRange contains the DHCP start and stop IPs
	// +optional.
	DHCPRange *DHCPRangeConfig `json:"dhcpRange,omitempty"`
}

// DHCPRangeConfig represents DHCP range configuration.
type DHCPRangeConfig struct {
	// Start is the first IP in the DHCP range
	Start string `json:"start,omitempty"`

	// Stop is the last IP in the DHCP range
	Stop string `json:"stop,omitempty"`
}

// IPAddressStatusSummary provides summary statistics about IP address allocation.
type IPAddressStatusSummary struct {
	// Total is the total number of addresses in the pool.
	// +optional.
	Total *int32 `json:"total,omitempty"`

	// Used is the number of addresses currently allocated.
	// +optional.
	Used *int32 `json:"used,omitempty"`

	// Free is the number of addresses available for allocation.
	// +optional.
	Free *int32 `json:"free,omitempty"`

	// OutOfRange is the number of addresses allocated outside the pool's range.
	// +optional.
	OutOfRange *int32 `json:"outOfRange,omitempty"`
}

// PoolCapacity provides pool utilization metrics.
type PoolCapacity struct {
	// UtilizationPercent is the percentage of pool capacity in use (0-100)
	// +optional.
	UtilizationPercent *int32 `json:"utilizationPercent,omitempty"`

	// ExhaustedAt is the projected time when the pool will be exhausted
	// based on current allocation rate (if available)
	// +optional.
	ExhaustedAt *metav1.Time `json:"exhaustedAt,omitempty"`

	// HighUtilization indicates if the pool is nearing capacity (>80%)
	// +optional.
	HighUtilization *bool `json:"highUtilization,omitempty"`
}

// NetworkInfo contains details about the Unifi network.
type NetworkInfo struct {
	// Name is the human-readable name of the Unifi network
	// +optional.
	Name string `json:"name,omitempty"`

	// VLAN is the VLAN ID if configured
	// +optional.
	VLAN *int32 `json:"vlan,omitempty"`

	// Purpose describes the network purpose (corporate-guest, guest, etc)
	// +optional.
	Purpose string `json:"purpose,omitempty"`

	// NetworkGroup is the network group assignment (LAN, WAN, etc)
	// +optional.
	NetworkGroup string `json:"networkGroup,omitempty"`

	// DHCPLeaseTime is the DHCP lease duration in seconds
	// +optional.
	DHCPLeaseTime *int32 `json:"dhcpLeaseTime,omitempty"`
}

// AllocationDetails tracks detailed allocation information.
type AllocationDetails struct {
	// AllocatedIPs is a list of currently allocated IP addresses
	// +optional.
	AllocatedIPs []AllocatedIP `json:"allocatedIPs,omitempty"`

	// FirstAllocationTime is when the first IP was allocated from this pool
	// +optional.
	FirstAllocationTime *metav1.Time `json:"firstAllocationTime,omitempty"`

	// LastAllocationTime is when the most recent IP was allocated
	// +optional.
	LastAllocationTime *metav1.Time `json:"lastAllocationTime,omitempty"`
}

// AllocatedIP represents an allocated IP address with metadata.
type AllocatedIP struct {
	// Address is the allocated IP address
	Address string `json:"address,omitempty"`

	// ClaimName is the name of the IPAddressClaim that requested this IP
	// +optional.
	ClaimName string `json:"claimName,omitempty"`

	// ClusterName is the cluster that owns this allocation
	// +optional.
	ClusterName string `json:"clusterName,omitempty"`

	// MacAddress is the MAC address assigned in Unifi
	// +optional.
	MacAddress string `json:"macAddress,omitempty"`

	// AllocatedAt is when this IP was allocated
	// +optional.
	AllocatedAt *metav1.Time `json:"allocatedAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=unifiippools,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Network",type="string",JSONPath=".status.networkInfo.name",description="Unifi network name"
// +kubebuilder:printcolumn:name="CIDR",type="string",JSONPath=".spec.subnets[0].cidr",description="Network CIDR"
// +kubebuilder:printcolumn:name="Used",type="integer",JSONPath=".status.addresses.used",description="Allocated IPs"
// +kubebuilder:printcolumn:name="Free",type="integer",JSONPath=".status.addresses.free",description="Available IPs"
// +kubebuilder:printcolumn:name="Utilization",type="string",JSONPath=".status.capacity.utilizationPercent",description="Pool utilization %"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='NetworkSynced')].status",description="Network sync status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// UnifiIPPool is the Schema for the unifiippools API.
type UnifiIPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnifiIPPoolSpec   `json:"spec"`
	Status UnifiIPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UnifiIPPoolList contains a list of UnifiIPPool.
type UnifiIPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UnifiIPPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UnifiIPPool{}, &UnifiIPPoolList{})
}
