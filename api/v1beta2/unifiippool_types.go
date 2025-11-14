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
	// This is the _id field from the Unifi network configuration
	// +kubebuilder:validation:Required
	NetworkID string `json:"networkId"`

	// Subnets is the list of subnets to allocate from
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Subnets []SubnetSpec `json:"subnets"`
}

// SubnetSpec defines a subnet configuration.
type SubnetSpec struct {
	// CIDR is the subnet CIDR block
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`
	CIDR string `json:"cidr"`

	// Gateway is the gateway IP address for this subnet
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	Gateway string `json:"gateway"`

	// Prefix is the network prefix length
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=32
	Prefix *int32 `json:"prefix"`

	// ExcludeRanges is a list of IP ranges to exclude from allocation
	// Format: "start-end" (e.g., "192.168.1.1-192.168.1.10")
	// +optional.
	ExcludeRanges []string `json:"excludeRanges,omitempty"`

	// DNS is the list of DNS servers for this subnet
	// +optional.
	DNS []string `json:"dns,omitempty"`
}

// UnifiIPPoolStatus defines the observed state of UnifiIPPool.
type UnifiIPPoolStatus struct {
	// Addresses provides summary statistics about address allocation
	// +optional.
	Addresses *IPAddressStatusSummary `json:"addresses,omitempty"`

	// Capacity provides utilization metrics for the pool
	// +optional.
	Capacity *PoolCapacity `json:"capacity,omitempty"`

	// NetworkInfo contains information about the Unifi network
	// +optional.
	NetworkInfo *NetworkInfo `json:"networkInfo,omitempty"`

	// AllocationDetails tracks detailed allocation information
	// +optional.
	AllocationDetails *AllocationDetails `json:"allocationDetails,omitempty"`

	// Conditions defines current state of the UnifiIPPool using metav1.Conditions
	// +optional.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastSyncTime is the last time the pool was successfully synced with Unifi
	// +optional.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedNetworkConfiguration contains the network config last observed from Unifi
	// Used to detect drift between Kubernetes and Unifi state
	// +optional.
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
	metav1.ObjectMeta `json:"metadata"`

	Spec   UnifiIPPoolSpec   `json:"spec"`
	Status UnifiIPPoolStatus `json:"status"`
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
